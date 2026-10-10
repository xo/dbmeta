package test

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/xo/dbmeta"
	_ "github.com/xo/dbmeta/models/questdb"
	qdfixture "github.com/xo/dbmeta/models/questdb/fixture"
)

// openQuestDB returns a connection to the server named by DBMETA_QUESTDB.
//
// QuestDB is reached on its PostgreSQL interface with pgx, which is what
// dburl's questdb:// opens (D154).
func openQuestDB(t *testing.T) *sql.DB {
	t.Helper()
	return openQuestDBAt(t, os.Getenv("DBMETA_QUESTDB"))
}

// openQuestDBAt returns a connection to dsn, or skips when dsn is empty.
func openQuestDBAt(t *testing.T, dsn string) *sql.DB {
	t.Helper()
	if dsn == "" {
		t.Skip("set DBMETA_QUESTDB to run against a real server")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("opening: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.PingContext(t.Context()); err != nil {
		t.Fatalf("connecting: %v", err)
	}
	return db
}

// setupQuestDB builds the fixture and returns the metadata for the server.
//
// It tears down first, because a run that failed part way leaves the tables
// behind and CREATE TABLE then fails rather than the test reporting what
// actually went wrong.
func setupQuestDB(t *testing.T, db *sql.DB) *dbmeta.Meta {
	t.Helper()
	ctx := t.Context()
	versions, err := dbmeta.QuestDB.Version(ctx, db)
	if err != nil {
		t.Fatalf("reading the version: %v", err)
	}

	down, err := qdfixture.Everything.ResolveTeardown(versions)
	if err != nil {
		t.Fatalf("resolving the teardown: %v", err)
	}
	for _, step := range down {
		if !step.Skipped {
			//nolint:errcheck // a teardown before setup is best effort
			db.ExecContext(ctx, step.Query)
		}
	}

	up, err := qdfixture.Everything.ResolveSetup(versions)
	if err != nil {
		t.Fatalf("resolving the setup: %v", err)
	}
	var ran, skipped int
	for _, step := range up {
		if step.Skipped {
			skipped++
			continue
		}
		if _, err := db.ExecContext(ctx, step.Query); err != nil {
			t.Fatalf("setup %s: %v\n%s", step.Name, err, step.Query)
		}
		ran++
	}
	t.Cleanup(func() {
		for _, step := range down {
			if !step.Skipped {
				//nolint:errcheck // the test has already reported what matters
				db.ExecContext(context.WithoutCancel(ctx), step.Query)
			}
		}
	})

	// A WAL table applies an insert after it returns. Wait for the row of
	// book. Table.Rows is NULL until then, so two reads of the same table
	// can differ (D207).
	// Every WAL table must have applied what was written to it, not only
	// book, because the parity test reads the row count of each one.
	deadline := time.Now().Add(30 * time.Second)
	for {
		var rows sql.NullInt64
		err := db.QueryRowContext(ctx, `SELECT table_row_count FROM tables() WHERE table_name = 'book'`).Scan(&rows)
		var lagging int64
		if err == nil {
			err = db.QueryRowContext(ctx, `SELECT count(*) FROM wal_tables() WHERE sequencerTxn > writerTxn`).Scan(&lagging)
		}
		if err == nil && rows.Valid && lagging == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("waiting for the WAL tables to apply their writes: %v", err)
		}
		time.Sleep(200 * time.Millisecond)
	}

	m, err := dbmeta.New(dbmeta.QuestDB, versions)
	if err != nil {
		t.Fatalf("building the metadata: %v", err)
	}
	t.Logf("server reports %s, fixture ran %d steps and skipped %d", versions, ran, skipped)
	return m
}

// qdArgs is the filter the fixture's objects sit behind.
func qdArgs() map[string]any {
	return dbmeta.Args{Schema: qdfixture.Everything.Schema}.Map()
}

// TestQuestDBVersion reads the version and checks what the model makes of
// it. QuestDB claims PostgreSQL 12.3 everywhere else, and the model reads its
// own release from build().
func TestQuestDBVersion(t *testing.T) {
	db := openQuestDB(t)
	versions, err := dbmeta.QuestDB.Version(t.Context(), db)
	if err != nil {
		t.Fatalf("reading the version: %v", err)
	}
	main := versions.Main()
	if main.Unknown || len(main.Parts) == 0 {
		t.Fatalf("expected a release in %s", versions)
	}
	// 9.4 is the floor, which is the newest release of the older line.
	if main.Compare(dbmeta.V(9, 4)) < 0 {
		t.Errorf("expected release 9.4 or newer, got %s", main)
	}
	if display := versions.String(); !strings.HasPrefix(display, "QuestDB ") {
		t.Errorf("expected the product in %q", display)
	}
	t.Logf("server reports %s", versions)
}

// TestQuestDBSmoke runs each registered query against the fixture and checks
// that it returns the columns it declares. This is the hard rule 9 check: a
// query that has never run is not finished.
func TestQuestDBSmoke(t *testing.T) {
	db := openQuestDB(t)
	m := setupQuestDB(t, db)
	var ran, unsupported int
	for _, q := range dbmeta.Queries() {
		if q.Support(m) != dbmeta.Supported {
			unsupported++
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
			t.Errorf("%s: declares %d fields and returns %d columns",
				q.Name(), len(fields), len(cs))
		}
		ran++
	}
	t.Logf("%d queries ran, %d not supported", ran, unsupported)
}

// TestQuestDBScansEveryQuery reads every query through its own Scan.
func TestQuestDBScansEveryQuery(t *testing.T) {
	db := openQuestDB(t)
	scanEveryQuery(t, setupQuestDB(t, db), db)
}

// TestQuestDBFixtureObjects reads the fixture back through the typed API and
// checks the values that are QuestDB's own.
func TestQuestDBFixtureObjects(t *testing.T) {
	db := openQuestDB(t)
	ctx := t.Context()
	m := setupQuestDB(t, db)

	kinds := map[string]string{}
	for v, err := range dbmeta.Tables.All(ctx, m, db, qdArgs()) {
		if err != nil {
			t.Fatalf("reading tables: %v", err)
		}
		kinds[v.Name] = v.Type
		if v.Schema != "public" {
			t.Errorf("%s: expected the schema public, got %q", v.Name, v.Schema)
		}
	}
	for name, want := range map[string]string{
		"author": "table", "book": "table", "recent": "view", "book_count": "materialized view",
	} {
		if kinds[name] != want {
			t.Errorf("%s: expected %s, got %q", name, want, kinds[name])
		}
	}

	// The position counts from 1, and every column is nullable, because
	// QuestDB has no NOT NULL.
	for v, err := range dbmeta.Columns.All(ctx, m, db, dbmeta.Args{Schema: "public", Parent: "author"}.Map()) {
		if err != nil {
			t.Fatalf("reading columns: %v", err)
		}
		if v.Name == "author_id" && v.Ordinal != 1 {
			t.Errorf("author_id: expected the position 1, got %d", v.Ordinal)
		}
		if !v.Nullable || v.PrimaryKey || v.Default.Valid {
			t.Errorf("%s: expected nullable, not a key and no default, got %+v", v.Name, v)
		}
	}

	views := map[string]string{}
	for v, err := range dbmeta.Views.All(ctx, m, db, qdArgs()) {
		if err != nil {
			t.Fatalf("reading views: %v", err)
		}
		views[v.Name] = v.Definition.V
	}
	if !strings.Contains(views["recent"], "book_id") || !strings.Contains(strings.ToLower(views["book_count"]), "sample by") {
		t.Errorf("expected both views with their definitions, got %v", views)
	}

	parts := map[string]string{}
	for v, err := range dbmeta.PartitionedTables.All(ctx, m, db, qdArgs()) {
		if err != nil {
			t.Fatalf("reading partitioned tables: %v", err)
		}
		parts[v.Name] = v.Strategy + " " + v.Expression
	}
	if parts["book"] != "RANGE YEAR (published)" {
		t.Errorf("book: expected RANGE YEAR (published), got %q", parts["book"])
	}

	// Every function is built in, so none is listed without the system
	// objects, and sum is an aggregate with them.
	if n, err := drain(t, dbmeta.Functions, m, db); err != nil || n != 0 {
		t.Errorf("expected no function without the system objects, got %d, %v", n, err)
	}
	var sum bool
	for v, err := range dbmeta.Aggregates.All(ctx, m, db, dbmeta.Args{WithSystem: true, Name: "sum"}.Map()) {
		if err != nil {
			t.Fatalf("reading aggregates: %v", err)
		}
		sum = sum || v.Kind == "agg" && v.ArgTypes.V != ""
	}
	if !sum {
		t.Error("expected sum among the aggregates, with its argument types")
	}

	var settings int
	for v, err := range dbmeta.Settings.All(ctx, m, db, nil) {
		if err != nil {
			t.Fatalf("reading settings: %v", err)
		}
		if v.Context.V != "reloadable" && v.Context.V != "restart" {
			t.Errorf("%s: unexpected context %q", v.Name, v.Context.V)
		}
		settings++
	}
	if settings == 0 {
		t.Error("expected settings")
	}

	user, ok, err := dbmeta.First(dbmeta.CurrentUser.All(ctx, m, db, nil))
	if err != nil || !ok || user.Name != "admin" {
		t.Errorf("expected the user admin, got %+v, %v", user, err)
	}
}

// TestQuestDBAnswersNoneOfThese checks that the objects QuestDB does not have
// are refused rather than answered with no rows (D34). A symbol index is the
// one that exists and is still refused: its flag is only in table_columns,
// which reads one table at a time.
func TestQuestDBAnswersNoneOfThese(t *testing.T) {
	db := openQuestDB(t)
	versions, err := dbmeta.QuestDB.Version(t.Context(), db)
	if err != nil {
		t.Fatalf("reading the version: %v", err)
	}
	m, err := dbmeta.New(dbmeta.QuestDB, versions)
	if err != nil {
		t.Fatalf("building the metadata: %v", err)
	}
	for _, q := range []dbmeta.AnyQuery{
		dbmeta.Indexes, dbmeta.IndexColumns, dbmeta.Constraints, dbmeta.ConstraintColumns,
		dbmeta.Roles, dbmeta.Privileges, dbmeta.Comments, dbmeta.Sequences, dbmeta.Triggers,
		dbmeta.Types, dbmeta.Extensions,
		// D207: table_partitions() takes a constant name, so no one
		// statement reads the partitions of the tables.
		dbmeta.Partitions, dbmeta.Policies, dbmeta.Inherits, dbmeta.Rules, dbmeta.NotNulls,
	} {
		if s := q.Support(m); s != dbmeta.NotSupported {
			t.Errorf("%s: expected %v, got %v", q.Name(), dbmeta.NotSupported, s)
		}
	}
}

// TestQuestDBTableFields reads the fields that D198 and D199 added to Tables
// (D207). QuestDB has no owner, no persistence choice and no access method.
// It has a row count and the settings of a table. It has no size that a
// filter can narrow.
func TestQuestDBTableFields(t *testing.T) {
	db := openQuestDB(t)
	ctx := t.Context()
	m := setupQuestDB(t, db)
	byName := map[string]dbmeta.Table{}
	for v, err := range dbmeta.Tables.All(ctx, m, db, qdArgs()) {
		if err != nil {
			t.Fatalf("reading tables: %v", err)
		}
		byName[v.Name] = v
		if v.Owner.Valid || v.Persistence.Valid || v.AccessMethod.Valid || v.Size.Valid {
			t.Errorf("%s: expected no owner, persistence, access method or size, got %+v", v.Name, v)
		}
	}
	want := "wal=true, dedup=false, maxUncommittedRows=500000, o3MaxLag=600000000us"
	if got := byName["book"].Options.V; got != want {
		t.Errorf("book: expected the options %q, got %q", want, got)
	}
	if got := byName["author"].Options.V; !strings.HasPrefix(got, "wal=false") {
		t.Errorf("author: expected the options of a table that is not a WAL table, got %q", got)
	}
	if byName["recent"].Options.Valid {
		t.Errorf("recent: a view has no options, got %q", byName["recent"].Options.V)
	}
	if got := byName["book"].Rows; !got.Valid || got.V != 1 {
		t.Errorf("book: expected 1 row, got %+v", got)
	}
}
