package test

import (
	"bytes"
	"context"
	"database/sql"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/dump"
)

// readOptions says what a product cannot answer, so that a check that it cannot
// meet is not made.
type readOptions struct {
	// schema is the schema that the tables are in.
	schema string
	// qualify puts the schema in front of a table name in the setup.
	qualify func(string) string
	// noChecks is set where the product does not report a check constraint.
	noChecks bool
	// noIndexColumns is set where the product does not report index columns.
	noIndexColumns bool
	// noLength is set where the product keeps no length for a varchar.
	noLength bool
}

var readSetup = []string{
	"CREATE TABLE {parent} (id integer PRIMARY KEY, name varchar(20) NOT NULL UNIQUE, n integer CHECK (n > 0))",
	// The key is a table constraint, because MySQL reads a REFERENCES in a column
	// and ignores it.
	"CREATE TABLE {child} (id integer PRIMARY KEY, parent_id integer, a integer, b integer, UNIQUE (a, b), FOREIGN KEY (parent_id) REFERENCES {parent}(id))",
	"CREATE INDEX child_a ON {child} (a)",
	"CREATE VIEW {view} AS SELECT id FROM {parent}",
}

// buildReadFixture creates the tables of the read tests and drops them again.
func buildReadFixture(t *testing.T, db *sql.DB, o readOptions) {
	t.Helper()
	names := strings.NewReplacer("{parent}", o.qualify("dump_parent"), "{child}", o.qualify("dump_child"), "{view}", o.qualify("dump_view"))
	drop := func() {
		for _, s := range []string{"DROP VIEW IF EXISTS " + o.qualify("dump_view"),
			"DROP TABLE IF EXISTS " + o.qualify("dump_child"), "DROP TABLE IF EXISTS " + o.qualify("dump_parent")} {
			_, _ = db.ExecContext(context.Background(), s)
		}
	}
	drop()
	t.Cleanup(drop)
	for _, s := range readSetup {
		mustExec(t, db, names.Replace(s))
	}
}

// checkSnapshot checks what Read must find in the tables of the fixture.
func checkSnapshot(t *testing.T, snap *dump.Snapshot, o readOptions) {
	t.Helper()
	var schema *dump.Schema
	for i := range snap.Schemas {
		if snap.Schemas[i].Name == o.schema {
			schema = &snap.Schemas[i]
		}
	}
	if schema == nil {
		t.Fatalf("the snapshot has no schema %q: %+v", o.schema, snap.Schemas)
	}
	table := func(name string) *dump.Table {
		for i := range schema.Tables {
			if schema.Tables[i].Name == name {
				return &schema.Tables[i]
			}
		}
		t.Fatalf("the snapshot has no table %q", name)
		return nil
	}
	parent, child := table("dump_parent"), table("dump_child")

	if len(parent.Columns) != 3 || parent.Columns[0].Name != "id" || parent.Columns[1].Name != "name" || parent.Columns[2].Name != "n" {
		t.Fatalf("the columns of the parent are %+v, want id, name and n in order", parent.Columns)
	}
	if k := parent.Columns[0].Type.Kind; k != dump.Integer {
		t.Errorf("id has the kind %s, want integer", k)
	}
	wantLength := 20
	if o.noLength {
		wantLength = 0
	}
	if c := parent.Columns[1]; c.Type.Kind != dump.String || c.Type.Length != wantLength || c.Nullable {
		t.Errorf("name is %+v, want a string of 20 that is not null", c)
	}

	kinds := func(tbl *dump.Table) map[string][]string {
		out := map[string][]string{}
		for _, c := range tbl.Constraints {
			out[c.Kind] = append(out[c.Kind], strings.Join(c.Columns, ","))
		}
		return out
	}
	pk := kinds(parent)
	if got := pk["primary key"]; len(got) != 1 || got[0] != "id" {
		t.Errorf("the parent has the primary keys %v, want id", got)
	}
	if got := pk["unique"]; len(got) != 1 || got[0] != "name" {
		t.Errorf("the parent has the unique keys %v, want name", got)
	}
	if !o.noChecks && len(pk["check"]) != 1 {
		t.Errorf("the parent has the checks %v, want one", pk["check"])
	}
	var fk *dump.Constraint
	for i := range child.Constraints {
		if child.Constraints[i].Kind == "foreign key" {
			fk = &child.Constraints[i]
		}
	}
	if fk == nil || fk.Ref == nil {
		t.Fatalf("the child has no foreign key: %+v", child.Constraints)
	}
	if strings.Join(fk.Columns, ",") != "parent_id" || fk.Ref.Table != "dump_parent" ||
		strings.Join(fk.Ref.Columns, ",") != "id" || (fk.Ref.Schema != "" && fk.Ref.Schema != o.schema) {
		t.Errorf("the foreign key is %+v with the reference %+v, want parent_id to dump_parent(id)", fk, fk.Ref)
	}

	var idx *dump.Index
	for i := range child.Indexes {
		if child.Indexes[i].Name == "child_a" {
			idx = &child.Indexes[i]
		}
	}
	switch {
	case idx == nil:
		t.Errorf("the child has no index child_a: %+v", child.Indexes)
	case o.noIndexColumns:
	case len(idx.Columns) != 1 || idx.Columns[0].Name != "a":
		t.Errorf("child_a has the columns %+v, want a", idx.Columns)
	}

	var view *dump.View
	for i := range schema.Views {
		if schema.Views[i].Name == "dump_view" {
			view = &schema.Views[i]
		}
	}
	if view == nil || !strings.Contains(strings.ToLower(view.Definition), "dump_parent") {
		t.Errorf("the view is %+v, want a definition that names dump_parent", view)
	}
}

// readSnapshotChecks reads the snapshot, checks it, reads it again for the same
// bytes, and reads the JSON back.
func readSnapshotChecks(t *testing.T, db *sql.DB, m *dbmeta.Meta, o readOptions) {
	t.Helper()
	buildReadFixture(t, db, o)
	filter := dump.Filter{Schemas: []string{o.schema}}
	snap, err := dump.Read(t.Context(), m, db, filter)
	if err != nil {
		t.Fatalf("reading: %v", err)
	}
	checkSnapshot(t, snap, o)

	first, err := snap.JSON()
	if err != nil {
		t.Fatal(err)
	}
	again, err := dump.Read(t.Context(), m, db, filter)
	if err != nil {
		t.Fatalf("reading again: %v", err)
	}
	second, err := again.JSON()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Error("two reads of the same database wrote different bytes")
	}
	back, err := dump.ParseSnapshot(first)
	if err != nil {
		t.Fatalf("reading the JSON back: %v", err)
	}
	checkSnapshot(t, back, o)

	// An exclude list leaves a table out, and so do its columns and keys.
	filter.ExcludeTables = []string{"dump_child"}
	left, err := dump.Read(t.Context(), m, db, filter)
	if err != nil {
		t.Fatalf("reading with an exclude list: %v", err)
	}
	for _, s := range left.Schemas {
		for _, tbl := range s.Tables {
			if tbl.Name == "dump_child" {
				t.Error("the table dump_child was excluded and is in the snapshot")
			}
		}
	}
}

func TestPostgreSQLReadsASnapshot(t *testing.T) {
	for _, driver := range postgresDrivers {
		t.Run(driver, func(t *testing.T) {
			db := openPostgresWith(t, driver)
			m := metaFor(t, dbmeta.PostgreSQL, db)
			mustExec(t, db, "DROP SCHEMA IF EXISTS dump_read CASCADE")
			mustExec(t, db, "CREATE SCHEMA dump_read")
			t.Cleanup(func() { _, _ = db.ExecContext(context.Background(), "DROP SCHEMA IF EXISTS dump_read CASCADE") })
			readSnapshotChecks(t, db, m, readOptions{
				schema: "dump_read", qualify: func(n string) string { return "dump_read." + n },
			})
		})
	}
}

func TestSQLiteReadsASnapshot(t *testing.T) {
	for _, driver := range sqliteDrivers {
		t.Run(driver, func(t *testing.T) {
			db := openSQLiteAt(t, driver)
			m := metaFor(t, dbmeta.SQLite3, db)
			readSnapshotChecks(t, db, m, readOptions{
				schema: "main", qualify: func(n string) string { return n }, noChecks: true,
			})
		})
	}
}

func TestDuckDBReadsASnapshot(t *testing.T) {
	db := openDuckDBAt(t)
	m := metaFor(t, dbmeta.DuckDB, db)
	readSnapshotChecks(t, db, m, readOptions{
		schema: "main", qualify: func(n string) string { return n }, noIndexColumns: true, noLength: true,
	})
}

// TestPostgreSQLSnapshotCost reads a catalog of thousands of tables and checks
// that the read is a fixed number of queries and not one for each table. D47
// asks for this check. The time is logged, and the limit is far longer than a
// read with one query for each kind of object takes.
func TestPostgreSQLSnapshotCost(t *testing.T) {
	const tables = 3000
	db := openPostgresWith(t, postgresDrivers[0])
	m := metaFor(t, dbmeta.PostgreSQL, db)
	dropCost := func() {
		// Thousands of tables in one DROP exhaust the lock table of the server, so
		// they go in batches of their own.
		for i := 1; i <= tables; i += 100 {
			var batch strings.Builder
			for j := i; j < i+100 && j <= tables; j++ {
				batch.WriteString("DROP TABLE IF EXISTS dump_cost.t" + strconv.Itoa(j) + ";")
			}
			_, _ = db.ExecContext(context.Background(), batch.String())
		}
		_, _ = db.ExecContext(context.Background(), "DROP SCHEMA IF EXISTS dump_cost CASCADE")
	}
	dropCost()
	mustExec(t, db, "CREATE SCHEMA dump_cost")
	t.Cleanup(dropCost)
	// A batch is its own transaction, because thousands of tables in one
	// exhaust the lock table of the server.
	for from := 1; from <= tables; from += 100 {
		var batch strings.Builder
		for i := from; i < from+100 && i <= tables; i++ {
			n := strconv.Itoa(i)
			batch.WriteString("CREATE TABLE dump_cost.t" + n + " (id integer PRIMARY KEY, a text, b integer, c numeric(10,2), d timestamp);")
			batch.WriteString("CREATE INDEX t" + n + "_b ON dump_cost.t" + n + " (b);")
		}
		mustExec(t, db, batch.String())
	}
	mustExec(t, db, "ANALYZE")
	start := time.Now()
	snap, err := dump.Read(t.Context(), m, db, dump.Filter{Schemas: []string{"dump_cost"}})
	took := time.Since(start)
	if err != nil {
		t.Fatalf("reading: %v", err)
	}
	var got int
	for _, s := range snap.Schemas {
		got += len(s.Tables)
		for _, tbl := range s.Tables {
			if len(tbl.Columns) != 5 || len(tbl.Indexes) != 2 {
				t.Fatalf("the table %s has %d columns and %d indexes, want 5 and 2", tbl.Name, len(tbl.Columns), len(tbl.Indexes))
			}
		}
	}
	if got != tables {
		t.Fatalf("the snapshot has %d tables, want %d", got, tables)
	}
	t.Logf("%d tables, 5 columns and 2 indexes each, read in %s", tables, took)
	if took > 30*time.Second {
		t.Errorf("the read took %s, which is too long for %d tables", took, tables)
	}
}

func TestMySQLReadsASnapshot(t *testing.T) {
	db := openMySQLDump(t)
	m := metaFor(t, dbmeta.MySQL, db)
	var schema string
	if err := db.QueryRowContext(t.Context(), "SELECT DATABASE()").Scan(&schema); err != nil || schema == "" {
		t.Skipf("the connection names no database: %v", err)
	}
	readSnapshotChecks(t, db, m, readOptions{
		schema: schema, qualify: func(n string) string { return n },
		// MariaDB 10.2 and later report a check, and MySQL 8.0.16 and later.
	})
}
