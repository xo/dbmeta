package test

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	mysqldriver "github.com/go-sql-driver/mysql"

	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/dump"
)

// runScript runs the statements of a script in order, and fails the test on the
// first error, with the statement.
func runScript(t *testing.T, db *sql.DB, stmts []dump.Statement) {
	t.Helper()
	for _, st := range stmts {
		if st.SQL == "" {
			continue
		}
		if _, err := db.ExecContext(t.Context(), st.SQL); err != nil {
			t.Fatalf("running the %s %s: %v\n%s", st.Kind, st.Name, err, st.SQL)
		}
	}
}

// scriptOf collects the statements of a script.
func scriptOf(t *testing.T, snap *dump.Snapshot, d dbmeta.Dialect) []dump.Statement {
	t.Helper()
	var out []dump.Statement
	for st, err := range dump.Script(snap, d, dump.Policy{}) {
		if err != nil {
			t.Fatalf("writing the script: %v", err)
		}
		out = append(out, st)
	}
	return out
}

// roundTripSchema reads a snapshot, writes its script, runs it on a database
// that has none of the objects, reads the snapshot again, and requires the same
// bytes. reset drops what the first read found, and again drops it at the end.
func roundTripSchema(t *testing.T, db *sql.DB, m *dbmeta.Meta, filter dump.Filter, reset func()) {
	t.Helper()
	first, err := dump.Read(t.Context(), m, db, filter)
	if err != nil {
		t.Fatalf("reading: %v", err)
	}
	for _, sk := range first.Skipped {
		if sk.Kind == "sequence" {
			t.Errorf("the sequence %s.%s is skipped, and it is read with its schema", sk.Schema, sk.Name)
		}
	}
	want, err := first.JSON()
	if err != nil {
		t.Fatal(err)
	}
	stmts := scriptOf(t, first, m.Dialect())
	reset()
	t.Cleanup(reset)
	runScript(t, db, stmts)
	second, err := dump.Read(t.Context(), m, db, filter)
	if err != nil {
		t.Fatalf("reading the copy: %v", err)
	}
	got, err := second.JSON()
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Errorf("the copy differs from the source:\n%s", lineDiff(string(want), string(got)))
		var script strings.Builder
		for _, st := range stmts {
			script.WriteString(st.SQL + ";\n")
		}
		t.Logf("script:\n%s", script.String())
	}
}

// postgresRoundTripSQL is the schema of the round trip, for a release. An identity
// column came in 10, a generated column in 12, and EXECUTE FUNCTION in 11, so an
// older release gets the older forms.
func postgresRoundTripSQL(release int) []string {
	steps := slices.Clone(postgresRoundTrip)
	for i, s := range steps {
		if release < 10 {
			s = strings.ReplaceAll(s, "AS bigint START", "START")
			s = strings.ReplaceAll(s, "id integer GENERATED ALWAYS AS IDENTITY PRIMARY KEY", "id serial PRIMARY KEY")
		}
		if release < 12 {
			s = strings.ReplaceAll(s, "\t\ttotal numeric GENERATED ALWAYS AS (price * qty) STORED,\n", "")
		}
		if release < 11 {
			s = strings.ReplaceAll(s, "EXECUTE FUNCTION", "EXECUTE PROCEDURE")
		}
		steps[i] = s
	}
	// A key on a partitioned table came in 11.
	if release >= 11 {
		steps = append(steps, postgresPartitioned...)
	}
	return steps
}

// postgresPartitioned is a partitioned table with a key, an index, a trigger and
// two partitions, one of them partitioned again, and a materialized view with an
// index that reads it. The script must make the keys of the partitions from the
// key of the table and not twice.
var postgresPartitioned = []string{
	`CREATE TABLE dump_rt.events (
		event_id integer NOT NULL,
		happened date NOT NULL,
		kind text NOT NULL,
		PRIMARY KEY (event_id, happened, kind)) PARTITION BY RANGE (happened)`,
	"CREATE INDEX events_kind ON dump_rt.events (kind)",
	"CREATE TABLE dump_rt.events_2025 PARTITION OF dump_rt.events FOR VALUES FROM ('2025-01-01') TO ('2026-01-01')",
	"CREATE TABLE dump_rt.events_2026 PARTITION OF dump_rt.events FOR VALUES FROM ('2026-01-01') TO ('2027-01-01') PARTITION BY LIST (kind)",
	"CREATE TABLE dump_rt.events_2026_a PARTITION OF dump_rt.events_2026 FOR VALUES IN ('a')",
	"CREATE TABLE dump_rt.events_2026_rest PARTITION OF dump_rt.events_2026 DEFAULT",
	"COMMENT ON TABLE dump_rt.events IS 'an audit log'",
	"CREATE MATERIALIZED VIEW dump_rt.event_counts AS SELECT kind, count(*) AS n FROM dump_rt.events GROUP BY kind",
	"CREATE UNIQUE INDEX event_counts_kind ON dump_rt.event_counts (kind)",
}

var postgresRoundTrip = []string{
	"CREATE SCHEMA dump_rt",
	"CREATE SCHEMA dump_rt2",
	// An extension makes its own type and functions, which the script leaves to it.
	"CREATE EXTENSION citext WITH SCHEMA dump_rt",
	// A domain on a domain, and a composite type with an attribute of a domain, so
	// that the order of the names is not the order of the statements.
	"CREATE DOMAIN dump_rt.positive AS integer DEFAULT 1 NOT NULL CHECK (VALUE > 0)",
	"CREATE DOMAIN dump_rt.small AS dump_rt.positive CHECK (VALUE < 100)",
	"CREATE DOMAIN dump_rt.email AS dump_rt.citext CHECK (VALUE ~ '@')",
	"COMMENT ON DOMAIN dump_rt.positive IS 'above zero'",
	"CREATE TYPE dump_rt.address AS (street text, city text, level dump_rt.small)",
	"COMMENT ON TYPE dump_rt.address IS 'a place'",
	"CREATE TYPE dump_rt.mood AS ENUM ('sad', 'it''s ok', 'back\\slash')",
	"CREATE SEQUENCE dump_rt.counter AS bigint START 10 INCREMENT 5 MINVALUE 10 MAXVALUE 1000 CYCLE",
	`CREATE TABLE dump_rt2.owner (
		id integer GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
		name text NOT NULL COLLATE "C",
		UNIQUE (name))`,
	`CREATE TABLE dump_rt.item (
		id serial PRIMARY KEY,
		owner_id integer NOT NULL REFERENCES dump_rt2.owner(id) ON DELETE CASCADE,
		mood dump_rt.mood DEFAULT 'sad',
		price numeric(10,2) CHECK (price >= 0),
		qty integer DEFAULT 1,
		total numeric GENERATED ALWAYS AS (price * qty) STORED,
		tags text[],
		made timestamp(3) with time zone DEFAULT now(),
		note text)`,
	"ALTER TABLE dump_rt.item ADD COLUMN home dump_rt.address, ADD COLUMN contact dump_rt.email DEFAULT 'a@b'",
	"CREATE INDEX item_note ON dump_rt.item (lower(note)) WHERE note IS NOT NULL",
	"CREATE INDEX item_price ON dump_rt.item (price DESC, qty)",
	"CREATE UNIQUE INDEX item_mood_owner ON dump_rt.item (mood, owner_id)",
	"CREATE VIEW dump_rt.cheap AS SELECT id, price FROM dump_rt.item WHERE price < 10",
	// a_top sorts before cheap and reads it, so a script in order of name fails.
	"CREATE VIEW dump_rt.a_top AS SELECT id FROM dump_rt.cheap WITH CASCADED CHECK OPTION",
	"COMMENT ON TABLE dump_rt.item IS 'it''s a table'",
	"COMMENT ON COLUMN dump_rt.item.note IS 'a note'",
	`CREATE FUNCTION dump_rt.touch() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN NEW.note := 'x'; RETURN NEW; END $$`,
	"CREATE TRIGGER item_touch BEFORE INSERT ON dump_rt.item FOR EACH ROW EXECUTE FUNCTION dump_rt.touch()",
}

// TestPostgreSQLScriptRoundTrips writes a script for a schema that has one of
// each object, runs it on an empty database, and compares the two snapshots.
func TestPostgreSQLScriptRoundTrips(t *testing.T) {
	for _, driver := range postgresDrivers {
		t.Run(driver, func(t *testing.T) {
			db := openPostgresWith(t, driver)
			m := metaFor(t, dbmeta.PostgreSQL, db)
			reset := func() {
				_, _ = db.ExecContext(context.Background(), "DROP SCHEMA IF EXISTS dump_rt CASCADE")
				_, _ = db.ExecContext(context.Background(), "DROP SCHEMA IF EXISTS dump_rt2 CASCADE")
			}
			reset()
			t.Cleanup(reset)
			for _, s := range postgresRoundTripSQL(pgRelease(m)) {
				mustExec(t, db, s)
			}
			filter := dump.Filter{Schemas: []string{"dump_rt", "dump_rt2"}}
			checkPostgresTypes(t, m, db, filter)
			if pgRelease(m) >= 11 {
				checkPostgresPartitions(t, m, db, filter)
			}
			roundTripSchema(t, db, m, filter, reset)
		})
	}
}

// checkPostgresTypes checks that the snapshot has the extension, the domains and
// the composite type, and has none of what the extension makes.
func checkPostgresTypes(t *testing.T, m *dbmeta.Meta, db *sql.DB, filter dump.Filter) {
	t.Helper()
	snap, err := dump.Read(t.Context(), m, db, filter)
	if err != nil {
		t.Fatalf("reading: %v", err)
	}
	if len(snap.Extensions) != 1 || snap.Extensions[0].Name != "citext" || snap.Extensions[0].Schema != "dump_rt" {
		t.Errorf("the extensions are %+v, and the expected one is citext in dump_rt", snap.Extensions)
	}
	for _, sc := range snap.Schemas {
		if sc.Name != "dump_rt" {
			continue
		}
		domains := map[string]dump.Domain{}
		for _, d := range sc.Domains {
			domains[d.Name] = d
		}
		if d := domains["positive"]; d.Type != "integer" || !d.NotNull || d.Default == nil || *d.Default != "1" ||
			!strings.Contains(d.Constraints, "VALUE > 0") || d.Comment != "above zero" {
			t.Errorf("the domain positive is %+v", d)
		}
		if d := domains["small"]; d.Type != "dump_rt.positive" || len(domains) != 3 {
			t.Errorf("the domains are %+v", domains)
		}
		if len(sc.Composites) != 1 || sc.Composites[0].Name != "address" || len(sc.Composites[0].Columns) != 3 ||
			sc.Composites[0].Comment != "a place" {
			t.Errorf("the composite types are %+v", sc.Composites)
		}
		for _, r := range sc.Routines {
			if strings.HasPrefix(r.Name, "citext") || strings.Contains(r.Name, "regex") {
				t.Errorf("the routine %s belongs to the extension, and the script does not write it", r.Name)
			}
		}
		for _, tb := range sc.Tables {
			if tb.Name != "item" && !strings.HasPrefix(tb.Name, "events") {
				t.Errorf("the table %s is not one of the fixture", tb.Name)
			}
		}
	}
}

// checkPostgresPartitions checks that the snapshot has the partitioned table, its
// partitions with their bounds, and the materialized view with its index.
func checkPostgresPartitions(t *testing.T, m *dbmeta.Meta, db *sql.DB, filter dump.Filter) {
	t.Helper()
	snap, err := dump.Read(t.Context(), m, db, filter)
	if err != nil {
		t.Fatalf("reading: %v", err)
	}
	tables := map[string]dump.Table{}
	var views map[string]dump.View
	for _, sc := range snap.Schemas {
		if sc.Name != "dump_rt" {
			continue
		}
		for _, tb := range sc.Tables {
			tables[tb.Name] = tb
		}
		views = map[string]dump.View{}
		for _, v := range sc.Views {
			views[v.Name] = v
		}
	}
	if p := tables["events"].Partitioning; p == nil || p.Strategy != "range" || p.Expression != "happened" {
		t.Errorf("the partitioning of events is %+v", p)
	}
	if p := tables["events_2026"].PartitionOf; p == nil || p.Table != "events" || !strings.HasPrefix(p.Bound, "FOR VALUES FROM") {
		t.Errorf("events_2026 is a partition of %+v", p)
	}
	if !tables["events_2026"].IsPartition() || tables["events_2026"].Partitioning == nil {
		t.Errorf("events_2026 is %+v, and it is a partition that is split again", tables["events_2026"])
	}
	if p := tables["events_2026_rest"].PartitionOf; p == nil || p.Bound != "DEFAULT" {
		t.Errorf("events_2026_rest is a partition of %+v", p)
	}
	if v := views["event_counts"]; !v.Materialized || len(v.Indexes) != 1 {
		t.Errorf("event_counts is %+v, and it is a materialized view with one index", v)
	}
	if v := views["cheap"]; v.Materialized {
		t.Errorf("cheap is a plain view, and it reads as %+v", v)
	}
}

// lineDiff lists the lines that differ, with the line number, which is enough to
// find the difference between two renderings of one snapshot.
func lineDiff(want, got string) string {
	w, g := strings.Split(want, "\n"), strings.Split(got, "\n")
	var b strings.Builder
	n := 0
	for i := 0; i < len(w) || i < len(g); i++ {
		var x, y string
		if i < len(w) {
			x = w[i]
		}
		if i < len(g) {
			y = g[i]
		}
		if x != y {
			b.WriteString("line " + strconv.Itoa(i+1) + "\n  source: " + x + "\n  copy:   " + y + "\n")
			if n++; n >= 12 {
				b.WriteString("...\n")
				break
			}
		}
	}
	return b.String()
}

var embeddedRoundTrip = []string{
	"CREATE TABLE dump_owner (id integer PRIMARY KEY, name varchar(20) NOT NULL UNIQUE)",
	`CREATE TABLE dump_item (
		id integer PRIMARY KEY,
		owner_id integer NOT NULL REFERENCES dump_owner(id) ON DELETE CASCADE,
		price decimal(10,2),
		qty integer DEFAULT 1,
		note varchar(40) DEFAULT 'it''s',
		UNIQUE (owner_id, price))`,
	"CREATE INDEX dump_item_note ON dump_item (note)",
	"CREATE INDEX dump_item_qty ON dump_item (qty DESC, price)",
	"CREATE VIEW dump_cheap AS SELECT id, price FROM dump_item WHERE price < 10",
}

// TestSQLiteScriptRoundTrips runs the script of a snapshot on a second, empty
// database of the same kind.
func TestSQLiteScriptRoundTrips(t *testing.T) {
	for _, driver := range sqliteDrivers {
		t.Run(driver, func(t *testing.T) {
			source := openSQLiteAt(t, driver)
			m := metaFor(t, dbmeta.SQLite3, source)
			for _, s := range embeddedRoundTrip {
				mustExec(t, source, s)
			}
			copyViaScript(t, source, m, dump.Filter{Schemas: []string{"main"}}, func() *sql.DB { return openSQLiteAt(t, driver) })
		})
	}
}

// TestDuckDBScriptRoundTrips runs the script of a snapshot on a second, empty
// database of the same kind.
func TestDuckDBScriptRoundTrips(t *testing.T) {
	source := openDuckDBAt(t)
	m := metaFor(t, dbmeta.DuckDB, source)
	for _, s := range embeddedRoundTrip {
		// DuckDB refuses ON DELETE CASCADE.
		mustExec(t, source, strings.ReplaceAll(s, " ON DELETE CASCADE", ""))
	}
	copyViaScript(t, source, m, dump.Filter{Schemas: []string{"main"}}, func() *sql.DB { return openDuckDBAt(t) })
}

// copyViaScript reads a snapshot of a database, writes its script, runs it on
// the second database that fresh opens, and requires the same snapshot.
func copyViaScript(t *testing.T, source *sql.DB, m *dbmeta.Meta, filter dump.Filter, fresh func() *sql.DB) {
	t.Helper()
	first, err := dump.Read(t.Context(), m, source, filter)
	if err != nil {
		t.Fatalf("reading: %v", err)
	}
	want, err := first.JSON()
	if err != nil {
		t.Fatal(err)
	}
	stmts := scriptOf(t, first, m.Dialect())
	target := fresh()
	runScript(t, target, stmts)
	second, err := dump.Read(t.Context(), m, target, filter)
	if err != nil {
		t.Fatalf("reading the copy: %v", err)
	}
	got, err := second.JSON()
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		var script strings.Builder
		for _, st := range stmts {
			script.WriteString(st.SQL + ";\n")
		}
		t.Errorf("the copy differs from the source:\n%s\nscript:\n%s", lineDiff(string(want), string(got)), script.String())
	}
}

// openSQLiteAt opens a second, empty SQLite database.
func openSQLiteAt(t *testing.T, driver string) *sql.DB {
	t.Helper()
	db, err := sql.Open(driver, filepath.Join(t.TempDir(), "copy.db"))
	if err != nil {
		t.Fatalf("opening the copy: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// openDuckDBAt opens a second, empty DuckDB database.
func openDuckDBAt(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("duckdb", filepath.Join(t.TempDir(), "copy.duckdb"))
	if err != nil {
		t.Fatalf("opening the copy: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

var mysqlRoundTrip = []string{
	"CREATE TABLE rt_owner (id int AUTO_INCREMENT PRIMARY KEY, name varchar(20) NOT NULL UNIQUE) COMMENT='the owners'",
	`CREATE TABLE rt_item (
		id int AUTO_INCREMENT PRIMARY KEY,
		owner_id int NOT NULL,
		label varchar(30) DEFAULT 'it''s a \\ label',
		qty int DEFAULT 5,
		code varchar(5) DEFAULT (concat('a', 'b')),
		made timestamp NULL DEFAULT CURRENT_TIMESTAMP,
		price decimal(10,2) COMMENT 'the price',
		twice int GENERATED ALWAYS AS (qty * 2) STORED,
		half int GENERATED ALWAYS AS (qty / 2) VIRTUAL,
		CONSTRAINT rt_fk_owner FOREIGN KEY (owner_id) REFERENCES rt_owner(id) ON DELETE RESTRICT ON UPDATE CASCADE,
		CONSTRAINT rt_chk CHECK (qty > 0),
		KEY rt_idx_label (label, price),
		UNIQUE KEY rt_uq (owner_id, code))`,
	"CREATE VIEW rt_cheap AS SELECT id, price FROM rt_item WHERE price < 10",
	"CREATE TRIGGER rt_touch BEFORE INSERT ON rt_item FOR EACH ROW SET NEW.qty = 1",
	// One table for each way that MySQL splits a table, so that each is written back
	// as it was read.
	"CREATE TABLE rt_range (id int NOT NULL, happened date NOT NULL, PRIMARY KEY (id, happened)) PARTITION BY RANGE (YEAR(happened)) (PARTITION p2025 VALUES LESS THAN (2026), PARTITION pmax VALUES LESS THAN MAXVALUE)",
	"CREATE TABLE rt_cols (id int NOT NULL, happened date NOT NULL, PRIMARY KEY (id, happened)) PARTITION BY RANGE COLUMNS (happened) (PARTITION p2025 VALUES LESS THAN ('2026-01-01'), PARTITION pmax VALUES LESS THAN (MAXVALUE))",
	"CREATE TABLE rt_list (id int NOT NULL, kind int NOT NULL, PRIMARY KEY (id, kind)) PARTITION BY LIST (kind) (PARTITION pa VALUES IN (1, 2), PARTITION pb VALUES IN (3))",
	"CREATE TABLE rt_hash (id int NOT NULL PRIMARY KEY) PARTITION BY HASH (id) PARTITIONS 3",
	"CREATE TABLE rt_key (id int NOT NULL PRIMARY KEY) PARTITION BY KEY () PARTITIONS 2",
}

// TestMySQLScriptRoundTrips writes the script of a schema that has one of each
// object, runs it on a database that has none of them, and compares the two
// snapshots. It runs on MySQL and on MariaDB, whichever DBMETA_MYSQL names.
func TestMySQLScriptRoundTrips(t *testing.T) {
	db := openMySQLDump(t)
	m := metaFor(t, dbmeta.MySQL, db)
	var schema string
	if err := db.QueryRowContext(t.Context(), "SELECT DATABASE()").Scan(&schema); err != nil || schema == "" {
		t.Skipf("the connection names no database: %v", err)
	}
	reset := func() {
		for _, s := range []string{"DROP VIEW IF EXISTS rt_cheap", "DROP TABLE IF EXISTS rt_item", "DROP TABLE IF EXISTS rt_owner",
			"DROP TABLE IF EXISTS rt_range", "DROP TABLE IF EXISTS rt_cols", "DROP TABLE IF EXISTS rt_list",
			"DROP TABLE IF EXISTS rt_hash", "DROP TABLE IF EXISTS rt_key"} {
			_, _ = db.ExecContext(context.Background(), s)
		}
	}
	reset()
	t.Cleanup(reset)
	for _, s := range mysqlRoundTrip {
		mustExec(t, db, s)
	}
	filter := dump.Filter{Schemas: []string{schema}, Tables: []string{`rt\_%`}}
	snap, err := dump.Read(t.Context(), m, db, filter)
	if err != nil {
		t.Fatalf("reading: %v", err)
	}
	want := map[string]struct {
		strategy string
		parts    int
	}{"rt_range": {"range", 2}, "rt_cols": {"range columns", 2}, "rt_list": {"list", 2}, "rt_hash": {"hash", 3}, "rt_key": {"key", 2}}
	for _, tb := range snap.Schemas[0].Tables {
		w, ok := want[tb.Name]
		if !ok {
			continue
		}
		if tb.Partitioning == nil || tb.Partitioning.Strategy != w.strategy || len(tb.Partitioning.Parts) != w.parts {
			t.Errorf("the partitioning of %s is %+v, and the expected strategy is %s with %d parts", tb.Name, tb.Partitioning, w.strategy, w.parts)
		}
		delete(want, tb.Name)
	}
	if len(want) > 0 {
		t.Errorf("the snapshot has no partitioned table for %v", want)
	}
	roundTripSchema(t, db, m, filter, reset)
}

// openMySQLDump opens a connection to a database of its own on the server that
// DBMETA_MYSQL names, and drops the database at the end, so that the dump tests
// share no table with another test. The DSN of CI names no database.
func openMySQLDump(t *testing.T) *sql.DB {
	t.Helper()
	return openMySQLNamed(t, "dbmeta_dump")
}

// openMySQLNamed opens a connection to a database of the given name, made empty
// and dropped at the end.
func openMySQLNamed(t *testing.T, name string) *sql.DB {
	t.Helper()
	admin := openMySQL(t)
	for _, s := range []string{"DROP DATABASE IF EXISTS " + name, "CREATE DATABASE " + name} {
		mustExec(t, admin, s)
	}
	t.Cleanup(func() { _, _ = admin.ExecContext(context.Background(), "DROP DATABASE IF EXISTS "+name) })
	cfg, err := mysqldriver.ParseDSN(os.Getenv("DBMETA_MYSQL"))
	if err != nil {
		t.Fatalf("reading DBMETA_MYSQL: %v", err)
	}
	cfg.DBName = name
	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		t.Fatalf("opening %s: %v", name, err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// sqlServerRoundTrip is a schema with one of each object that SQL Server has and
// the dump reads: an identity column, a computed column, a check, a default, a
// unique key, a foreign key, an index with an included column and a filter, a
// sequence and two views, one reading the other.
var sqlServerRoundTrip = []string{
	"CREATE SCHEMA dump_rt",
	"CREATE SEQUENCE dump_rt.counter AS bigint START WITH 10 INCREMENT BY 5 MINVALUE 10 MAXVALUE 1000 CYCLE",
	`CREATE TABLE dump_rt.owner (
		id int IDENTITY(1,1) PRIMARY KEY,
		name nvarchar(40) NOT NULL UNIQUE)`,
	`CREATE TABLE dump_rt.item (
		id int NOT NULL PRIMARY KEY,
		owner_id int NOT NULL REFERENCES dump_rt.owner (id) ON DELETE CASCADE,
		price decimal(10,2) CHECK (price >= 0),
		qty int NULL DEFAULT 1,
		total AS (price * qty) PERSISTED,
		made datetime2(3) NULL DEFAULT (getdate()),
		active bit NOT NULL DEFAULT 1,
		note nvarchar(100) NULL,
		data varbinary(max) NULL)`,
	"CREATE INDEX item_note ON dump_rt.item (note) INCLUDE (qty) WHERE note IS NOT NULL",
	"CREATE UNIQUE INDEX item_owner_note ON dump_rt.item (owner_id, note DESC)",
	"CREATE VIEW [dump_rt].[cheap] AS SELECT id, price FROM dump_rt.item WHERE price < 10",
	// a_top sorts before cheap and reads it, so a script in order of name fails.
	"CREATE VIEW [dump_rt].[a_top] AS SELECT id FROM dump_rt.cheap",
}

// TestSQLServerScriptRoundTrips writes a script for a schema that has one of each
// object, runs it on an empty database, and compares the two snapshots.
func TestSQLServerScriptRoundTrips(t *testing.T) {
	db := openSQLServerNamed(t, "dbmeta_dump")
	m := metaFor(t, dbmeta.SQLServer, db)
	for _, s := range sqlServerRoundTrip {
		mustExec(t, db, s)
	}
	filter := dump.Filter{Schemas: []string{"dump_rt"}}
	first, err := dump.Read(t.Context(), m, db, filter)
	if err != nil {
		t.Fatalf("reading: %v", err)
	}
	want, err := first.JSON()
	if err != nil {
		t.Fatal(err)
	}
	stmts := scriptOf(t, first, dbmeta.SQLServer)
	// The copy goes in a database of its own, so the same names are free.
	copyDB := openSQLServerNamed(t, "dbmeta_dump_copy")
	runScript(t, copyDB, stmts)
	second, err := dump.Read(t.Context(), metaFor(t, dbmeta.SQLServer, copyDB), copyDB, filter)
	if err != nil {
		t.Fatalf("reading the copy: %v", err)
	}
	got, err := second.JSON()
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Errorf("the copy differs from the source:\n%s", lineDiff(string(want), string(got)))
		var script strings.Builder
		for _, st := range stmts {
			script.WriteString(st.SQL + ";\n")
		}
		t.Logf("script:\n%s", script.String())
	}
}

// TestMySQLReadsTheCurrentDatabase reads with no filter and requires only the
// database of the connection, though the server holds others.
func TestMySQLReadsTheCurrentDatabase(t *testing.T) {
	db := openMySQLNamed(t, "dbmeta_scope_a")
	other := openMySQLNamed(t, "dbmeta_scope_b")
	mustExec(t, db, "CREATE TABLE a (id int PRIMARY KEY)")
	mustExec(t, other, "CREATE TABLE b (id int PRIMARY KEY)")
	mustExec(t, db, "CREATE VIEW v AS SELECT id FROM dbmeta_scope_a.a")
	snap, err := dump.Read(t.Context(), metaFor(t, dbmeta.MySQL, db), db, dump.Filter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Schemas) != 1 || snap.Schemas[0].Name != "dbmeta_scope_a" {
		t.Fatalf("the schemas are %+v, want only dbmeta_scope_a", snap.Schemas)
	}
	var script strings.Builder
	for _, st := range scriptOf(t, snap, dbmeta.MySQL) {
		script.WriteString(st.SQL + ";\n")
	}
	if strings.Contains(script.String(), "dbmeta_scope_a`.") {
		t.Errorf("the view keeps the name of its database:\n%s", script.String())
	}
}
