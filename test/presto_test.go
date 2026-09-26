package test

import (
	"context"
	"database/sql"
	"os"
	"testing"

	_ "github.com/prestodb/presto-go-client/v2"

	"github.com/xo/dbmeta"
	_ "github.com/xo/dbmeta/models/presto"
	prfixture "github.com/xo/dbmeta/models/presto/fixture"
)

// openPresto returns a connection to the server named by DBMETA_PRESTO.
func openPresto(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("DBMETA_PRESTO")
	if dsn == "" {
		t.Skip("set DBMETA_PRESTO to run against a real server")
	}
	db, err := sql.Open("presto", dsn)
	if err != nil {
		t.Fatalf("opening: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.PingContext(t.Context()); err != nil {
		t.Fatalf("connecting: %v", err)
	}
	return db
}

// setupPresto builds the fixture and returns the metadata for the server.
//
// It tears down first, because a run that failed part way leaves the schema
// behind and CREATE SCHEMA then fails rather than the test reporting what
// actually went wrong.
func setupPresto(t *testing.T, db *sql.DB) *dbmeta.Meta {
	t.Helper()
	ctx := t.Context()
	versions, err := dbmeta.Presto.Version(ctx, db)
	if err != nil {
		t.Fatalf("reading the version: %v", err)
	}

	down, err := prfixture.Everything.ResolveTeardown(versions)
	if err != nil {
		t.Fatalf("resolving the teardown: %v", err)
	}
	for _, step := range down {
		if !step.Skipped {
			//nolint:errcheck // a teardown before setup is best effort
			db.ExecContext(ctx, step.Query)
		}
	}

	up, err := prfixture.Everything.ResolveSetup(versions)
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

	m, err := dbmeta.New(dbmeta.Presto, versions)
	if err != nil {
		t.Fatalf("building the metadata: %v", err)
	}
	t.Logf("server reports %s, fixture ran %d steps and skipped %d", versions, ran, skipped)
	return m
}

// prArgs is the filter the fixture's objects sit behind.
func prArgs() map[string]any {
	return dbmeta.Args{Schema: prfixture.Everything.Schema}.Map()
}

// TestPrestoVersion reads the version and checks what the model makes of it.
//
// Presto numbers every release 0.x and appends the build it was cut from, so
// 0.299-7d50721 is release 299. Trino, which forked from it, numbers releases
// with a bare integer. The two cannot be compared and D73 is why they are
// separate dialects rather than one with a version gate.
func TestPrestoVersion(t *testing.T) {
	db := openPresto(t)
	versions, err := dbmeta.Presto.Version(t.Context(), db)
	if err != nil {
		t.Fatalf("reading the version: %v", err)
	}
	main := versions.Main()
	if main.Unknown || len(main.Parts) == 0 {
		t.Fatalf("expected a parsed version, got %v", versions)
	}
	if main.Parts[0] != 0 || len(main.Parts) < 2 {
		t.Errorf("expected a 0.x release number, got %v", main)
	}
	t.Logf("server reports %s", versions)
}

// TestPrestoEveryQueryRuns executes every query Presto answers and checks the
// columns match the declared fields.
func TestPrestoEveryQueryRuns(t *testing.T) {
	db := openPresto(t)
	m := setupPresto(t, db)

	var ran, unsupported int
	for _, q := range dbmeta.Queries() {
		switch q.Support(m) {
		case dbmeta.NotSupported, dbmeta.NotBuilt, dbmeta.TooOld:
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
	}
	t.Logf("%d queries ran, %d not supported by Presto", ran, unsupported)
	if ran == 0 {
		t.Fatal("expected at least one query to run")
	}
}

// TestPrestoFindsTheFixture reads rows through the typed API and checks the
// objects the fixture built are there.
func TestPrestoFindsTheFixture(t *testing.T) {
	db := openPresto(t)
	m := setupPresto(t, db)
	ctx := t.Context()

	want := map[string]string{
		"author": "table", "book": "table", "region": "table",
		"shipment": "table", "recent": "view",
	}
	got := map[string]string{}
	var withComment int
	for v, err := range dbmeta.Tables.All(ctx, m, db, prArgs()) {
		if err != nil {
			t.Fatalf("reading tables: %v", err)
		}
		got[v.Name] = v.Type
		if v.Comment.Valid {
			withComment++
		}
		if v.Catalog != "memory" {
			t.Errorf("%s: expected the memory catalog, got %q", v.Name, v.Catalog)
		}
	}
	for name, kind := range want {
		if got[name] != kind {
			t.Errorf("expected %s to be a %s, got %q", name, kind, got[name])
		}
	}
	// No relation carries a comment, and that is the product. Presto accepts
	// a COMMENT clause on CREATE TABLE, keeps nothing readable, and has no
	// system.metadata.table_comments for the query to reach. Pinned here so
	// that an absence stays a decision rather than becoming a bug.
	if withComment != 0 {
		t.Errorf("expected no commented relation, got %d", withComment)
	}
}

// TestPrestoColumnsAreAllNullable pins the two column properties Presto does
// not keep, both of which models/trino does.
//
// Presto has no COMMENT ON statement at all, so a column comment cannot be set
// and system.jdbc.columns.remarks is always NULL. Its memory connector refuses
// NOT NULL on the newest release there is, so every column is nullable. Trino
// answers both, which is why each has its own conformance section.
func TestPrestoColumnsAreAllNullable(t *testing.T) {
	db := openPresto(t)
	m := setupPresto(t, db)
	ctx := t.Context()

	a := dbmeta.Args{Schema: prfixture.Everything.Schema, Parent: "author"}.Map()
	var cols, commented int
	nullable := map[string]bool{}
	for v, err := range dbmeta.Columns.All(ctx, m, db, a) {
		if err != nil {
			t.Fatalf("reading columns: %v", err)
		}
		cols++
		nullable[v.Name] = v.Nullable
		if v.Comment.Valid {
			commented++
		}
		if v.Name == "name" && v.DataType != "varchar(128)" {
			t.Errorf("expected varchar(128), got %q", v.DataType)
		}
	}
	if cols != 4 {
		t.Errorf("expected 4 columns on author, got %d", cols)
	}
	if commented != 0 {
		t.Errorf("expected no commented column, got %d", commented)
	}
	for name, isNull := range nullable {
		if !isNull {
			t.Errorf("expected %s to be nullable: Presto keeps no NOT NULL", name)
		}
	}
}

// TestPrestoCatalogFilters covers the level no other model has.
//
// Presto is the only product here with three levels, so it is the only model
// that answers a catalog filter, and a filter that quietly matched nothing
// would look the same as an empty catalog.
func TestPrestoCatalogFilters(t *testing.T) {
	db := openPresto(t)
	m := setupPresto(t, db)
	ctx := t.Context()

	count := func(a map[string]any) int {
		t.Helper()
		var n int
		for _, err := range dbmeta.Tables.All(ctx, m, db, a) {
			if err != nil {
				t.Fatalf("reading tables: %v", err)
			}
			n++
		}
		return n
	}
	inMemory := count(dbmeta.Args{Catalog: "memory"}.Map())
	if inMemory == 0 {
		t.Fatal("expected the memory catalog to hold the fixture")
	}
	if got := count(dbmeta.Args{Catalog: "no_such_catalog"}.Map()); got != 0 {
		t.Errorf("expected no rows for an unknown catalog, got %d", got)
	}
	// The fixture builds a second schema so a schema filter is asked
	// something it can answer wrongly.
	if got := count(dbmeta.Args{Catalog: "memory", Schema: "dbmeta_other"}.Map()); got != 1 {
		t.Errorf("expected the one table in dbmeta_other, got %d", got)
	}
	if got := count(prArgs()); got != 5 {
		t.Errorf("expected the five fixture relations, got %d", got)
	}
}

// TestPrestoCatalogsAreDatabases checks the kind that maps a Presto catalog onto
// what psql calls a database, and the connector onto an access method.
func TestPrestoCatalogsAreDatabases(t *testing.T) {
	db := openPresto(t)
	m := setupPresto(t, db)
	ctx := t.Context()

	var found bool
	for v, err := range dbmeta.Databases.All(ctx, m, db, nil) {
		if err != nil {
			t.Fatalf("reading databases: %v", err)
		}
		if v.Name == "memory" {
			found = true
			if !v.Comment.Valid || v.Comment.V != "memory" {
				t.Errorf("expected the connector name, got %v", v.Comment)
			}
		}
	}
	if !found {
		t.Error("expected the memory catalog")
	}

	var methods int
	for v, err := range dbmeta.AccessMethods.All(ctx, m, db, nil) {
		if err != nil {
			t.Fatalf("reading access methods: %v", err)
		}
		methods++
		if v.Type != "table" {
			t.Errorf("expected a connector to report as a table method, got %q", v.Type)
		}
	}
	if methods == 0 {
		t.Error("expected the connectors to answer for access methods")
	}
}

// TestPrestoViewsCarryTheirDefinition covers the kind dbtpl needs, and the one
// query that reads the session catalog rather than spanning every catalog.
func TestPrestoViewsCarryTheirDefinition(t *testing.T) {
	db := openPresto(t)
	m := setupPresto(t, db)
	ctx := t.Context()

	var views int
	for v, err := range dbmeta.Views.All(ctx, m, db, prArgs()) {
		if err != nil {
			t.Fatalf("reading views: %v", err)
		}
		views++
		if v.Name != "recent" {
			continue
		}
		if v.Definition == "" {
			t.Error("expected the view definition")
		}
		if v.Comment.Valid {
			t.Errorf("expected no view comment, got %v", v.Comment)
		}
	}
	if views != 1 {
		t.Errorf("expected the one fixture view, got %d", views)
	}
}

// TestPrestoRolesAreNotOffered pins the difference from models/trino.
//
// Trino's memory connector answers the standard role views with no rows.
// Presto's raises NOT_SUPPORTED, so this model does not register Roles or
// RoleGrants at all: D34 says a query dbmeta offers must run. Privileges reads
// table_privileges, which does answer, and returns nothing on this connector.
func TestPrestoRolesAreNotOffered(t *testing.T) {
	db := openPresto(t)
	m := setupPresto(t, db)
	ctx := t.Context()

	for _, q := range []dbmeta.AnyQuery{dbmeta.Roles, dbmeta.RoleGrants} {
		if got := q.Support(m); got != dbmeta.NotSupported {
			t.Errorf("%s: expected NotSupported, got %v", q.Name(), got)
		}
	}
	// Privileges does answer, and returns nothing on this connector.
	for _, err := range dbmeta.Privileges.All(ctx, m, db, prArgs()) {
		if err != nil {
			t.Fatalf("reading privileges: %v", err)
		}
		t.Error("expected no privilege row on the memory connector")
	}
}

// TestPrestoCurrentUserAnswersAndSchemaDoesNot checks the two connection
// questions, of which Presto answers one.
//
// current_user resolves. Neither current_catalog nor current_schema does, and
// nothing in system.runtime carries the session, so CurrentSchema is not
// registered. models/trino answers both.
func TestPrestoCurrentUserAnswersAndSchemaDoesNot(t *testing.T) {
	db := openPresto(t)
	m := setupPresto(t, db)
	ctx := t.Context()

	if got := dbmeta.CurrentSchema.Support(m); got != dbmeta.NotSupported {
		t.Errorf("expected CurrentSchema to be NotSupported, got %v", got)
	}

	var users int
	for v, err := range dbmeta.CurrentUser.All(ctx, m, db, nil) {
		if err != nil {
			t.Fatalf("reading the current user: %v", err)
		}
		users++
		if v.Name == "" {
			t.Error("expected a principal name")
		}
		if !v.Session.Valid || v.Session.V != v.Name {
			t.Errorf("expected the session user to equal the name, got %v", v.Session)
		}
	}
	if users != 1 {
		t.Errorf("expected one current user, got %d", users)
	}
}
