package test

import (
	"context"
	"database/sql"
	"os"
	"slices"
	"strings"
	"testing"

	_ "github.com/apache/arrow-go/v18/arrow/flight/flightsql/driver"

	"github.com/xo/dbmeta"
	dkfixture "github.com/xo/dbmeta/models/duckdb/fixture"
	_ "github.com/xo/dbmeta/models/gizmosql"
	gzfixture "github.com/xo/dbmeta/models/gizmosql/fixture"
	"github.com/xo/dbmeta/test/internal/gizmosql"
)

// gizmoSystem is the database that GizmoSQL attaches for its own views.
const gizmoSystem = "_gizmosql_system"

// openGizmoSQL returns a connection to the server named by DBMETA_GIZMOSQL, with
// the Arrow Flight SQL driver that dburl names for gizmosql, which is
// github.com/apache/arrow-go (D154). dbrun sets the variable to the DSN of the
// driver.
//
// The driver is given a session token and not the password. The driver never
// makes the handshake that opens a session, and GizmoSQL refuses every call
// without one, so the test makes the handshake itself and hands the driver
// what it issued. See D187 and internal/gizmosql.
func openGizmoSQL(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("DBMETA_GIZMOSQL")
	if dsn == "" {
		t.Skip("set DBMETA_GIZMOSQL to run against a real server")
	}
	return openGizmoSQLAt(t, dsn)
}

// openGizmoSQLAt opens dsn after the handshake.
func openGizmoSQLAt(t *testing.T, dsn string) *sql.DB {
	t.Helper()
	dsn, err := gizmosql.DSN(t.Context(), dsn)
	if err != nil {
		t.Fatalf("opening a session: %v", err)
	}
	db, err := sql.Open("flightsql", dsn)
	if err != nil {
		t.Fatalf("opening: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.PingContext(t.Context()); err != nil {
		t.Fatalf("connecting: %v", err)
	}
	return db
}

// setupGizmoSQL builds the fixture, which is the DuckDB fixture, and returns
// the metadata for the server. It tears down first, because a run that failed
// part way leaves the objects behind.
func setupGizmoSQL(t *testing.T, db *sql.DB) *dbmeta.Meta {
	t.Helper()
	versions, err := dbmeta.GizmoSQL.Version(t.Context(), db)
	if err != nil {
		t.Fatalf("reading the version: %v", err)
	}
	m, err := dbmeta.New(dbmeta.GizmoSQL, versions)
	if err != nil {
		t.Fatalf("building the meta: %v", err)
	}
	run := func(ctx context.Context, steps []dkfixture.Result, fatal bool) {
		for _, s := range steps {
			if _, err := db.ExecContext(ctx, s.Query); err != nil && fatal {
				t.Fatalf("%s: %v\n%s", s.Name, err, s.Query)
			}
		}
	}
	down, err := gzfixture.Everything.ResolveTeardown(versions)
	if err != nil {
		t.Fatalf("resolving the teardown: %v", err)
	}
	up, err := gzfixture.Everything.ResolveSetup(versions)
	if err != nil {
		t.Fatalf("resolving the setup: %v", err)
	}
	run(t.Context(), down, false)
	run(t.Context(), up, true)
	t.Cleanup(func() { run(context.WithoutCancel(t.Context()), down, false) })
	t.Logf("server reports %s, fixture ran %d steps", versions, len(up))
	return m
}

func gzArgs() map[string]any {
	return dbmeta.Args{Schema: gzfixture.Everything.Schema}.Map()
}

// TestGizmoSQLVersion reads the version, which is the release of DuckDB that
// the server runs. No SQL statement names the GizmoSQL release (D187).
func TestGizmoSQLVersion(t *testing.T) {
	db := openGizmoSQL(t)
	versions, err := dbmeta.GizmoSQL.Version(t.Context(), db)
	if err != nil {
		t.Fatalf("reading the version: %v", err)
	}
	if versions.Main().Compare(dbmeta.V(1, 4)) < 0 {
		t.Errorf("expected DuckDB 1.4 or newer in %s", versions)
	}
	if !strings.HasPrefix(versions.String(), "GizmoSQL, which runs DuckDB v") {
		t.Errorf("expected the product and its engine in %q", versions.String())
	}
	t.Logf("server reports %s", versions)
}

// TestGizmoSQLEveryQueryRuns executes every query GizmoSQL answers and checks
// the columns match the declared fields.
func TestGizmoSQLEveryQueryRuns(t *testing.T) {
	db := openGizmoSQL(t)
	m := setupGizmoSQL(t, db)

	var ran, unsupported int
	var names []string
	for _, q := range dbmeta.Queries() {
		if q.Support(m) != dbmeta.Supported {
			unsupported++
			continue
		}
		query, vals, err := q.Build(m, nil)
		if err != nil {
			t.Errorf("%s: rendering: %v", q.Name(), err)
			continue
		}
		cols, err := columnsOf(t, db, query, vals)
		if err != nil {
			t.Errorf("%s: executing: %v\n%s", q.Name(), err, query)
			continue
		}
		fields, err := q.Fields(m)
		if err != nil {
			t.Errorf("%s: reading fields: %v", q.Name(), err)
			continue
		}
		if len(cols) != len(fields) {
			t.Errorf("%s: declares %d fields and returns %d columns", q.Name(), len(fields), len(cols))
			continue
		}
		for i := range cols {
			if cols[i] != fields[i].Name {
				t.Errorf("%s: column %d is %q and the field is %q", q.Name(), i, cols[i], fields[i].Name)
			}
		}
		ran++
		names = append(names, q.Name())
	}
	t.Logf("%d of %d queries ran: %s", ran, ran+unsupported, strings.Join(names, " "))
	if ran != 20 {
		t.Errorf("expected 20 queries to run, got %d", ran)
	}
}

// TestGizmoSQLScansEveryQuery reads every query through its own Scan, with
// the system objects and without them. The Flight SQL driver reads a column
// into a Go value by its Arrow type, and it refuses a list, a map and a
// struct, so this finds a statement that returns one.
func TestGizmoSQLScansEveryQuery(t *testing.T) {
	db := openGizmoSQL(t)
	m := setupGizmoSQL(t, db)
	scanEveryQuery(t, m, db)
	scanEveryQueryWith(t, m, db, false)
}

// TestGizmoSQLReadsTheFixture reads the fixture back through the typed API,
// which the rendering test cannot check. The statements are the duckdb
// model's, so this checks that the Flight SQL driver does not change an
// answer.
func TestGizmoSQLReadsTheFixture(t *testing.T) {
	db := openGizmoSQL(t)
	m := setupGizmoSQL(t, db)
	ctx := t.Context()

	want := map[string]string{
		"author": "table", "book": "table", "region": "table",
		"shipment": "table", "recent": "view",
	}
	found := make(map[string]string)
	for v, err := range dbmeta.Tables.All(ctx, m, db, gzArgs()) {
		if err != nil {
			t.Fatalf("reading tables: %v", err)
		}
		found[v.Name] = v.Type
	}
	for name, kind := range want {
		if found[name] != kind {
			t.Errorf("expected %q to be a %s, got %q", name, kind, found[name])
		}
	}

	cols := make(map[string]dbmeta.Column)
	for v, err := range dbmeta.Columns.All(ctx, m, db, gzArgs()) {
		if err != nil {
			t.Fatalf("reading columns: %v", err)
		}
		cols[v.Table+"."+v.Name] = v
	}
	if got := cols["author.author_id"]; !got.PrimaryKey || got.Nullable {
		t.Errorf("expected author_id to be a non null primary key, got %+v", got)
	}
	if got := cols["author.rating"]; got.PrimaryKey || !got.Nullable {
		t.Errorf("expected rating to be nullable and not a key, got %+v", got)
	}
	if got := cols["author.author_id"]; !got.Comment.Valid || got.Comment.V != "surrogate key" {
		t.Errorf("expected the column comment, got %v", got.Comment)
	}
	if got := cols["author.name"]; got.Comment.Valid {
		t.Errorf("expected no comment on an uncommented column, got %v", got.Comment)
	}

	// The kinds that read a list on the server and return rows.
	var labels []string
	for v, err := range dbmeta.EnumValues.All(ctx, m, db, gzArgs()) {
		if err != nil {
			t.Fatalf("reading enum values: %v", err)
		}
		if v.Enum == "colour" {
			labels = append(labels, v.Label)
		}
	}
	if want := []string{"red", "green", "blue"}; !slices.Equal(labels, want) {
		t.Errorf("expected %v in declaration order, got %v", want, labels)
	}
	var params []string
	a := dbmeta.Args{Schema: gzfixture.Everything.Schema, Parent: "addup"}.Map()
	for v, err := range dbmeta.RoutineParameters.All(ctx, m, db, a) {
		if err != nil {
			t.Fatalf("reading routine parameters: %v", err)
		}
		params = append(params, v.Name.V)
	}
	if want := []string{"a", "b"}; !slices.Equal(params, want) {
		t.Errorf("expected %v, got %v", want, params)
	}

	var fk []string
	for v, err := range dbmeta.ConstraintColumns.All(ctx, m, db, gzArgs()) {
		if err != nil {
			t.Fatalf("reading constraint columns: %v", err)
		}
		if v.Table == "shipment" && v.ForeignTable.V == "region" {
			fk = append(fk, v.Name+">"+v.ForeignName.V)
		}
	}
	if want := []string{"country>country", "area>area"}; !slices.Equal(fk, want) {
		t.Errorf("expected the composite foreign key %v, got %v", want, fk)
	}
}

// TestGizmoSQLSystemDatabase checks the one object GizmoSQL adds. It attaches
// a database named _gizmosql_system, with two views for its Flight SQL
// metadata calls, and DuckDB does not flag any of it internal. The model
// leaves it out unless the caller asks for the system objects, in every kind
// that reads a catalog function (D187).
func TestGizmoSQLSystemDatabase(t *testing.T) {
	db := openGizmoSQL(t)
	m := setupGizmoSQL(t, db)
	ctx := t.Context()

	names := func(system bool) ([]string, []string) {
		var databases, views []string
		a := dbmeta.Args{WithSystem: system}.Map()
		for v, err := range dbmeta.Databases.All(ctx, m, db, a) {
			if err != nil {
				t.Fatalf("reading databases: %v", err)
			}
			databases = append(databases, v.Name)
		}
		for v, err := range dbmeta.Views.All(ctx, m, db, a) {
			if err != nil {
				t.Fatalf("reading views: %v", err)
			}
			if v.Catalog == gizmoSystem {
				views = append(views, v.Name)
			}
		}
		return databases, views
	}

	databases, views := names(false)
	if slices.Contains(databases, gizmoSystem) || len(views) != 0 {
		t.Errorf("expected %s to be left out, got databases %v and views %v", gizmoSystem, databases, views)
	}
	if !slices.Contains(databases, "dbmeta") {
		t.Errorf("expected the database of the file, got %v", databases)
	}
	databases, views = names(true)
	if !slices.Contains(databases, gizmoSystem) {
		t.Errorf("expected %s with the system objects, got %v", gizmoSystem, databases)
	}
	for _, want := range []string{"gizmosql_index_info", "gizmosql_view_definition"} {
		if !slices.Contains(views, want) {
			t.Errorf("expected the view %s with the system objects, got %v", want, views)
		}
	}
}

// TestGizmoSQLUnsupported checks that what the engine does not have says so
// rather than answering with an empty result. D34 requires that difference.
func TestGizmoSQLUnsupported(t *testing.T) {
	db := openGizmoSQL(t)
	m := setupGizmoSQL(t, db)
	for _, q := range []dbmeta.AnyQuery{
		// The core has one user, and DuckDB has no users, grants or triggers.
		dbmeta.Roles, dbmeta.RoleGrants, dbmeta.Privileges, dbmeta.RoleSettings,
		dbmeta.DefaultACLs, dbmeta.Triggers, dbmeta.Tablespaces,
		dbmeta.ColumnStats, dbmeta.IndexColumns,
	} {
		if got := q.Support(m); got != dbmeta.NotSupported {
			t.Errorf("%s: expected it to be reported unsupported, got %v", q.Name(), got)
		}
	}
}

// TestGizmoSQLTerminator checks that the server takes a trailing semicolon, so
// the model does not set TerminatorStripped. Measured on 1.40.0 and 1.41.0.
func TestGizmoSQLTerminator(t *testing.T) {
	db := openGizmoSQL(t)
	for _, q := range []string{"SELECT 1;", "SELECT 1; ", "SELECT 1 -- c"} {
		var n int64
		if err := db.QueryRowContext(t.Context(), q).Scan(&n); err != nil {
			t.Errorf("expected %q to run, got: %v", q, err)
		}
	}
	info, ok := dbmeta.GizmoSQL.Info()
	if !ok || info.Terminator != dbmeta.TerminatorKept {
		t.Errorf("expected a server that takes a semicolon to leave TerminatorStripped off")
	}
}
