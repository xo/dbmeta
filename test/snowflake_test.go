package test

import (
	"context"
	"database/sql"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	_ "github.com/snowflakedb/gosnowflake/v2"

	"github.com/xo/dbmeta"
	_ "github.com/xo/dbmeta/models/snowflake"
	sffixture "github.com/xo/dbmeta/models/snowflake/fixture"
)

// openSnowflake returns a connection to the service named by DBMETA_SNOWFLAKE, which
// dbrun resolves from the places D117 names. The model was written before
// an account was provisioned, and D190 holds what the first run found.
func openSnowflake(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("DBMETA_SNOWFLAKE")
	if dsn == "" {
		t.Skip("set DBMETA_SNOWFLAKE to run against the service")
	}
	db, err := sql.Open("snowflake", dsn)
	if err != nil {
		t.Fatalf("opening: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.PingContext(t.Context()); err != nil {
		t.Fatalf("connecting: %v", err)
	}
	return db
}

// setupSnowflake builds the fixture and returns the metadata for the service.
func setupSnowflake(t *testing.T, db *sql.DB) *dbmeta.Meta {
	t.Helper()
	ctx := t.Context()
	versions, err := dbmeta.Snowflake.Version(ctx, db)
	if err != nil {
		t.Fatalf("reading the version: %v", err)
	}
	down, err := sffixture.Everything.ResolveTeardown(versions)
	if err != nil {
		t.Fatalf("resolving the teardown: %v", err)
	}
	for _, step := range down {
		if !step.Skipped {
			//nolint:errcheck // a teardown before setup is best effort
			db.ExecContext(ctx, step.Query)
		}
	}
	up, err := sffixture.Everything.ResolveSetup(versions)
	if err != nil {
		t.Fatalf("resolving the setup: %v", err)
	}
	for _, step := range up {
		if step.Skipped {
			continue
		}
		if _, err := db.ExecContext(ctx, step.Query); err != nil {
			t.Fatalf("setup %s: %v\n%s", step.Name, err, step.Query)
		}
	}
	t.Cleanup(func() {
		for _, step := range down {
			if !step.Skipped {
				//nolint:errcheck // the test has already reported what matters
				db.ExecContext(context.WithoutCancel(ctx), step.Query)
			}
		}
	})
	m, err := dbmeta.New(dbmeta.Snowflake, versions)
	if err != nil {
		t.Fatalf("building the metadata: %v", err)
	}
	t.Logf("service reports %s", versions)
	return m
}

// TestSnowflakeSmoke runs each query and checks the columns match the fields.
func TestSnowflakeSmoke(t *testing.T) {
	db := openSnowflake(t)
	m := setupSnowflake(t, db)
	for _, q := range dbmeta.Queries() {
		if q.Support(m) != dbmeta.Supported {
			continue
		}
		query, vals, err := q.Build(m, nil)
		if err != nil {
			t.Errorf("%s: building: %v", q.Name(), err)
			continue
		}
		cs, err := columnsOf(t, db, query, vals)
		if err != nil {
			t.Errorf("%s: %v\n%s", q.Name(), err, query)
			continue
		}
		fields, err := q.Fields(m)
		if err != nil {
			t.Errorf("%s: reading fields: %v", q.Name(), err)
			continue
		}
		if len(cs) != len(fields) {
			t.Errorf("%s: declares %d fields and returns %d columns", q.Name(), len(fields), len(cs))
		}
	}
}

// TestSnowflakeScansEveryQuery reads every query through its own Scan.
func TestSnowflakeScansEveryQuery(t *testing.T) {
	db := openSnowflake(t)
	scanEveryQuery(t, setupSnowflake(t, db), db)
}

// sfArgs reads the fixture schema, which Snowflake folds to upper case.
func sfArgs() map[string]any {
	return dbmeta.Args{Schema: sffixture.Everything.Schema}.Map()
}

// TestSnowflakeFixtureObjects reads the fixture back through the typed API and
// checks the values that are Snowflake's own.
func TestSnowflakeFixtureObjects(t *testing.T) {
	db := openSnowflake(t)
	ctx := t.Context()
	m := setupSnowflake(t, db)

	types := map[string]string{}
	for v, err := range dbmeta.Tables.All(ctx, m, db, sfArgs()) {
		if err != nil {
			t.Fatalf("reading tables: %v", err)
		}
		types[v.Name] = v.Type
		if v.Name == "AUTHOR" && v.Comment.V != "people who write" {
			t.Errorf("AUTHOR: expected its comment, got %+v", v.Comment)
		}
	}
	for name, want := range map[string]string{"AUTHOR": "table", "BOOK": "table", "RECENT": "view"} {
		if types[name] != want {
			t.Errorf("%s: expected %s, got %q", name, want, types[name])
		}
	}

	for v, err := range dbmeta.Columns.All(ctx, m, db, sfArgs()) {
		if err != nil {
			t.Fatalf("reading columns: %v", err)
		}
		wantKey := map[string]bool{
			"AUTHOR.AUTHOR_ID": true, "BOOK.BOOK_ID": true, "REGION.COUNTRY": true,
			"REGION.AREA": true, "SHIPMENT.SHIPMENT_ID": true,
		}[v.Table+"."+v.Name]
		if v.PrimaryKey != wantKey {
			t.Errorf("%s.%s: expected primary_key=%v, got %v", v.Table, v.Name, wantKey, v.PrimaryKey)
		}
		switch v.Table + "." + v.Name {
		case "AUTHOR.AUTHOR_ID":
			if v.Nullable || v.Identity.V != "by default" || v.Comment.V != "surrogate key" {
				t.Errorf("AUTHOR_ID: expected not nullable, an identity and its comment, got %+v", v)
			}
		case "AUTHOR.SHADE":
			if v.Default.V != "'red'" {
				t.Errorf("SHADE: expected the default 'red', got %q", v.Default.V)
			}
		case "BOOK.TITLE":
			if v.Collation.V != "en-ci" {
				t.Errorf("TITLE: expected the collation en-ci, got %q", v.Collation.V)
			}
		}
	}

	kinds := map[string]int{}
	for v, err := range dbmeta.Constraints.All(ctx, m, db, sfArgs()) {
		if err != nil {
			t.Fatalf("reading constraints: %v", err)
		}
		kinds[v.Type]++
		// Snowflake records a key and checks none of them.
		if !v.Enforced.Valid || v.Enforced.V {
			t.Errorf("%s: expected enforced to be a real false, got %+v", v.Name, v.Enforced)
		}
	}
	if kinds["primary key"] != 4 || kinds["foreign key"] != 2 || kinds["unique"] != 1 {
		t.Errorf("expected 4 primary keys, 2 foreign keys and 1 unique key, got %v", kinds)
	}

	for v, err := range dbmeta.Sequences.All(ctx, m, db, sfArgs()) {
		if err != nil {
			t.Fatalf("reading sequences: %v", err)
		}
		if v.Name != "COUNTER" || v.Start.V != "10" || v.Increment.V != "2" {
			t.Errorf("expected COUNTER from 10 by 2, got %+v", v)
		}
	}

	routines := map[string]string{}
	for v, err := range dbmeta.Functions.All(ctx, m, db, sfArgs()) {
		if err != nil {
			t.Fatalf("reading functions: %v", err)
		}
		routines[v.Name] = v.Kind
	}
	for v, err := range dbmeta.Functions.All(ctx, m, db, sfArgs()) {
		if err != nil {
			t.Fatalf("reading functions: %v", err)
		}
		if v.Name == "SHOUT" && v.Volatility != "volatile" {
			t.Errorf("SHOUT: expected volatile, got %q", v.Volatility)
		}
	}
	if routines["SHOUT"] != "func" || routines["ADDUP"] != "proc" {
		t.Errorf("expected SHOUT as a function and ADDUP as a procedure, got %v", routines)
	}

	// The type of a grant on a view is view, and not table.
	var granted bool
	for v, err := range dbmeta.Privileges.All(ctx, m, db, sfArgs()) {
		if err != nil {
			t.Fatalf("reading privileges: %v", err)
		}
		if v.Name == "RECENT" {
			granted = true
			if v.Type != "view" || !strings.HasPrefix(v.Access.V, "DBMETA_ROLE=OWNERSHIP/") {
				t.Errorf("RECENT: expected an ownership grant on a view, got %+v", v)
			}
		}
	}
	if !granted {
		t.Error("RECENT: expected a grant")
	}
}

// TestSnowflakeConstraintColumns reads the columns of every key, and the
// column that each foreign key points at, through one statement for each
// (D203).
func TestSnowflakeConstraintColumns(t *testing.T) {
	db := openSnowflake(t)
	ctx := t.Context()
	m := setupSnowflake(t, db)
	got := map[string]string{}
	for v, err := range dbmeta.ConstraintColumns.All(ctx, m, db, sfArgs()) {
		if err != nil {
			t.Fatalf("reading constraint columns: %v", err)
		}
		got[v.Table+"."+v.Constraint+"."+strconv.FormatInt(v.Ordinal, 10)] =
			v.Name + ">" + v.ForeignSchema.V + "." + v.ForeignTable.V + "." + v.ForeignName.V
	}
	want := map[string]string{
		"BOOK.BOOK_AUTHOR_FK.1":         "AUTHOR_ID>DBMETA_FIXTURE.AUTHOR.AUTHOR_ID",
		"BOOK.BOOK_TITLE_UNIQUE.1":      "TITLE>..",
		"SHIPMENT.SHIPMENT_REGION_FK.1": "COUNTRY>DBMETA_FIXTURE.REGION.COUNTRY",
		"SHIPMENT.SHIPMENT_REGION_FK.2": "AREA>DBMETA_FIXTURE.REGION.AREA",
	}
	for key, w := range want {
		if got[key] != w {
			t.Errorf("%s: expected %q, got %q", key, w, got[key])
		}
	}
	// A primary key has no name the test knows, and a composite one has two
	// rows.
	var region int
	for key := range got {
		if strings.HasPrefix(key, "REGION.") && strings.HasSuffix(key, ".2") {
			region++
		}
	}
	if region != 1 {
		t.Errorf("expected the composite key of REGION to have a second column, got %v", got)
	}
	if len(got) != 9 {
		t.Errorf("expected 9 key columns, got %d: %v", len(got), got)
	}
	// Keep narrows the rows, since the statement takes no filter.
	only := 0
	args := dbmeta.Args{Schema: sffixture.Everything.Schema, Parent: "SHIPMENT"}.Map()
	for v, err := range dbmeta.ConstraintColumns.All(ctx, m, db, args) {
		if err != nil {
			t.Fatalf("reading the constraint columns of SHIPMENT: %v", err)
		}
		if v.Table != "SHIPMENT" {
			t.Errorf("expected only SHIPMENT, got %s", v.Table)
		}
		only++
	}
	if only != 3 {
		t.Errorf("expected 3 key columns of SHIPMENT, got %d", only)
	}
}

// TestSnowflakeDynamicTable checks that a dynamic table is not reported as an
// ordinary one, and that the settings read.
func TestSnowflakeDynamicTable(t *testing.T) {
	db := openSnowflake(t)
	ctx := t.Context()
	m := setupSnowflake(t, db)
	exec(t, db, `CREATE DYNAMIC TABLE `+sffixture.Everything.Schema+`.RECENT_BOOKS`+
		` TARGET_LAG = '1 day' WAREHOUSE = DBMETA_WH AS SELECT book_id, title FROM `+
		sffixture.Everything.Schema+`.book`)
	var got string
	for v, err := range dbmeta.Tables.All(ctx, m, db, dbmeta.Args{Schema: sffixture.Everything.Schema, Name: "RECENT_BOOKS"}.Map()) {
		if err != nil {
			t.Fatalf("reading tables: %v", err)
		}
		got = v.Type
	}
	if got != "dynamic table" {
		t.Errorf("RECENT_BOOKS: expected a dynamic table, got %q", got)
	}
	var tz bool
	for v, err := range dbmeta.Settings.All(ctx, m, db, dbmeta.Args{Name: "TIMEZONE"}.Map()) {
		if err != nil {
			t.Fatalf("reading settings: %v", err)
		}
		tz = v.Name == "TIMEZONE" && v.Value.Valid
	}
	if !tz {
		t.Error("expected the setting TIMEZONE")
	}
}

// TestSnowflakeTableFields reads the fields that D198 and D199 added to
// Tables (D207): the owner, the persistence, the size, the rows and the
// options. A temporary table lives as long as its session, so the test keeps
// one connection.
func TestSnowflakeTableFields(t *testing.T) {
	db := openSnowflake(t)
	db.SetMaxOpenConns(1)
	ctx := t.Context()
	m := setupSnowflake(t, db)
	exec(t, db, `CREATE TEMPORARY TABLE `+sffixture.Everything.Schema+`.SCRATCH (id INTEGER)`)

	// The row count of a table that was just written can lag, so the test
	// waits for it.
	var tables map[string]dbmeta.Table
	for range 30 {
		tables = map[string]dbmeta.Table{}
		for v, err := range dbmeta.Tables.All(ctx, m, db, sfArgs()) {
			if err != nil {
				t.Fatalf("reading tables: %v", err)
			}
			tables[v.Name] = v
		}
		if tables["AUTHOR"].Rows.V == 3 {
			break
		}
		time.Sleep(2 * time.Second)
	}

	author := tables["AUTHOR"]
	if !author.Owner.Valid || author.Owner.V == "" {
		t.Errorf("AUTHOR: expected an owner, got %+v", author.Owner)
	}
	if author.Rows.V != 3 || !author.Size.Valid || author.Size.V <= 0 {
		t.Errorf("AUTHOR: expected 3 rows and a size, got %+v and %+v", author.Rows, author.Size)
	}
	if author.Persistence.V != "permanent" {
		t.Errorf("AUTHOR: expected permanent, got %+v", author.Persistence)
	}
	if !strings.HasPrefix(author.Options.V, "retention_time=") || strings.Contains(author.Options.V, "cluster_by") {
		t.Errorf("AUTHOR: expected a retention time and no clustering key, got %+v", author.Options)
	}
	events := tables["EVENTS"]
	if events.Persistence.V != "transient" || events.Options.V != "retention_time=0, cluster_by=LINEAR(event_id)" {
		t.Errorf("EVENTS: expected a transient table with its key, got %+v and %+v", events.Persistence, events.Options)
	}
	if got := tables["SCRATCH"].Persistence.V; got != "temporary" {
		t.Errorf("SCRATCH: expected temporary, got %q", got)
	}
	if recent := tables["RECENT"]; recent.Persistence.Valid || recent.Options.Valid || recent.Rows.Valid || recent.Size.Valid {
		t.Errorf("RECENT: a view has no persistence, options, rows or size, got %+v", recent)
	}
}

// TestSnowflakeKeysIgnoreTheCurrentSchema moves the session to another schema
// and reads the keys again. SHOW with no scope reads the current schema, so a
// statement with no scope finds no key here. See D203.
func TestSnowflakeKeysIgnoreTheCurrentSchema(t *testing.T) {
	db := openSnowflake(t)
	// One connection, so that the USE below is the session every read has.
	db.SetMaxOpenConns(1)
	ctx := t.Context()
	m := setupSnowflake(t, db)
	var database string
	if err := db.QueryRowContext(ctx, `SELECT CURRENT_DATABASE()`).Scan(&database); err != nil {
		t.Fatalf("reading the database: %v", err)
	}
	exec(t, db, `USE SCHEMA `+database+`.INFORMATION_SCHEMA`)
	args := dbmeta.Args{Schema: sffixture.Everything.Schema, Parent: "AUTHOR"}.Map()
	var key bool
	for v, err := range dbmeta.Columns.All(ctx, m, db, args) {
		if err != nil {
			t.Fatalf("reading columns: %v", err)
		}
		if v.Name == "AUTHOR_ID" {
			key = v.PrimaryKey
		}
	}
	if !key {
		t.Error("AUTHOR_ID: expected a primary key from another current schema")
	}
	var n int
	for _, err := range dbmeta.ConstraintColumns.All(ctx, m, db, args) {
		if err != nil {
			t.Fatalf("reading constraint columns: %v", err)
		}
		n++
	}
	if n != 1 {
		t.Errorf("expected 1 key column of AUTHOR from another current schema, got %d", n)
	}
}

// TestSnowflakeKeyScopes reads keys at each scope the statements take, from a
// schema whose name holds a double quote and a semicolon, to prove that a name
// stays inside its quotes. The names have no underscore, because an underscore
// is a wildcard and keeps the scope at the database. See D203.
func TestSnowflakeKeyScopes(t *testing.T) {
	db := openSnowflake(t)
	ctx := t.Context()
	m := setupSnowflake(t, db)
	const hostile = `DMKEYS"; DROP TABLE T; --`
	var database string
	if err := db.QueryRowContext(ctx, `SELECT CURRENT_DATABASE()`).Scan(&database); err != nil {
		t.Fatalf("reading the database: %v", err)
	}
	quoted := dbmeta.QuoteIdentifier(hostile, `"`, `"`)
	cleanup(t, db, `DROP SCHEMA IF EXISTS `+quoted+` CASCADE`)
	t.Cleanup(func() { cleanup(t, db, `DROP SCHEMA IF EXISTS `+quoted+` CASCADE`) })
	exec(t, db, `CREATE SCHEMA `+quoted)
	exec(t, db, `CREATE TABLE `+quoted+`.PARENT (A INT, B INT, PRIMARY KEY (A, B))`)
	exec(t, db, `CREATE TABLE `+quoted+`.CHILD (X INT PRIMARY KEY, A INT, B INT,`+
		` FOREIGN KEY (A, B) REFERENCES `+quoted+`.PARENT (A, B))`)
	for _, c := range []struct {
		name       string
		catalog    string
		schema     string
		table      string
		columns    int
		keyColumns int
	}{
		{"one table", "", hostile, "CHILD", 3, 3},
		{"one table with its database", database, hostile, "CHILD", 3, 3},
		{"one schema", database, hostile, "", 5, 5},
		{"one schema without its database", "", hostile, "", 5, 5},
		{"a wildcard table", database, hostile, "CH%", 3, 3},
		{"a wildcard schema", "", "DMKEYS%", "CHILD", 3, 3},
		{"a table that does not exist, with a wildcard", database, hostile, "NOSUCH%", 0, 0},
	} {
		t.Run(c.name, func(t *testing.T) {
			args := dbmeta.Args{Catalog: c.catalog, Schema: c.schema, Parent: c.table}.Map()
			var columns int
			for _, err := range dbmeta.Columns.All(ctx, m, db, args) {
				if err != nil {
					t.Fatalf("reading columns: %v", err)
				}
				columns++
			}
			if columns != c.columns {
				t.Errorf("expected %d columns, got %d", c.columns, columns)
			}
			var n int
			for v, err := range dbmeta.ConstraintColumns.All(ctx, m, db, args) {
				if err != nil {
					t.Fatalf("reading constraint columns: %v", err)
				}
				if v.Schema != hostile {
					t.Errorf("expected the schema %q, got %q", hostile, v.Schema)
				}
				n++
			}
			if n != c.keyColumns {
				t.Errorf("expected %d key columns, got %d", c.keyColumns, n)
			}
		})
	}
}
