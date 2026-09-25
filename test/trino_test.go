package test

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"testing"

	_ "github.com/trinodb/trino-go-client/trino"

	"github.com/xo/dbmeta"
	_ "github.com/xo/dbmeta/models/trino"
	trfixture "github.com/xo/dbmeta/models/trino/fixture"
)

// openTrino returns a connection to the server named by DBMETA_TRINO.
func openTrino(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("DBMETA_TRINO")
	if dsn == "" {
		t.Skip("set DBMETA_TRINO to run against a real server")
	}
	db, err := sql.Open("trino", dsn)
	if err != nil {
		t.Fatalf("opening: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.PingContext(t.Context()); err != nil {
		t.Fatalf("connecting: %v", err)
	}
	return db
}

// setupTrino builds the fixture and returns the metadata for the server.
//
// It tears down first, because a run that failed part way leaves the schema
// behind and CREATE SCHEMA then fails rather than the test reporting what
// actually went wrong.
func setupTrino(t *testing.T, db *sql.DB) *dbmeta.Meta {
	t.Helper()
	ctx := t.Context()
	versions, err := dbmeta.Trino.Version(ctx, db)
	if err != nil {
		t.Fatalf("reading the version: %v", err)
	}

	down, err := trfixture.Everything.ResolveTeardown(versions)
	if err != nil {
		t.Fatalf("resolving the teardown: %v", err)
	}
	for _, step := range down {
		if !step.Skipped {
			//nolint:errcheck // a teardown before setup is best effort
			db.ExecContext(ctx, step.Query)
		}
	}

	up, err := trfixture.Everything.ResolveSetup(versions)
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

	m, err := dbmeta.New(dbmeta.Trino, versions)
	if err != nil {
		t.Fatalf("building the metadata: %v", err)
	}
	t.Logf("server reports %s, fixture ran %d steps and skipped %d", versions, ran, skipped)
	return m
}

// trArgs is the filter the fixture's objects sit behind.
func trArgs() map[string]any {
	return dbmeta.Args{Schema: trfixture.Everything.Schema}.Map()
}

// TestTrinoVersion reads the version and checks what the model makes of it.
//
// Trino numbers a release with one integer and has no major or minor, which
// is unlike every other product here.
func TestTrinoVersion(t *testing.T) {
	db := openTrino(t)
	versions, err := dbmeta.Trino.Version(t.Context(), db)
	if err != nil {
		t.Fatalf("reading the version: %v", err)
	}
	main := versions.Main()
	if main.Unknown || len(main.Parts) == 0 {
		t.Fatalf("expected a parsed version, got %v", versions)
	}
	if main.Parts[0] < 350 {
		t.Errorf("expected a release number, got %v", main)
	}
	t.Logf("server reports %s", versions)
}

// TestTrinoEveryQueryRuns executes every query Trino answers and checks the
// columns match the declared fields.
func TestTrinoEveryQueryRuns(t *testing.T) {
	db := openTrino(t)
	m := setupTrino(t, db)

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
	t.Logf("%d queries ran, %d not supported by Trino", ran, unsupported)
	if ran == 0 {
		t.Fatal("expected at least one query to run")
	}
}

// TestTrinoFindsTheFixture reads rows through the typed API and checks the
// objects the fixture built are there.
func TestTrinoFindsTheFixture(t *testing.T) {
	db := openTrino(t)
	m := setupTrino(t, db)
	ctx := t.Context()

	want := map[string]string{
		"author": "table", "book": "table", "region": "table",
		"shipment": "table", "recent": "view",
	}
	got := map[string]string{}
	var withComment int
	for v, err := range dbmeta.Tables.All(ctx, m, db, trArgs()) {
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
	// Every fixture relation carries a comment, which is what proves the join
	// to system.metadata.table_comments rather than the empty remarks column
	// in system.jdbc.tables.
	if withComment != len(want) {
		t.Errorf("expected %d commented relations, got %d", len(want), withComment)
	}
}

// TestTrinoColumnsCarryTheComment covers the one thing system.jdbc.columns has
// and information_schema.columns does not.
func TestTrinoColumnsCarryTheComment(t *testing.T) {
	db := openTrino(t)
	m := setupTrino(t, db)
	ctx := t.Context()

	a := dbmeta.Args{Schema: trfixture.Everything.Schema, Parent: "author"}.Map()
	var cols, commented int
	nullable := map[string]bool{}
	for v, err := range dbmeta.Columns.All(ctx, m, db, a) {
		if err != nil {
			t.Fatalf("reading columns: %v", err)
		}
		cols++
		nullable[v.Name] = v.Nullable
		if v.Name == "name" {
			commented++
			if !v.Comment.Valid || v.Comment.V != "the author name" {
				t.Errorf("expected the column comment, got %v", v.Comment)
			}
			if v.DataType != "varchar(128)" {
				t.Errorf("expected varchar(128), got %q", v.DataType)
			}
		}
	}
	if cols != 4 {
		t.Errorf("expected 4 columns on author, got %d", cols)
	}
	if commented != 1 {
		t.Errorf("expected one commented column, got %d", commented)
	}
	// NOT NULL is the one column property Trino keeps, so it is the one that
	// has to survive the JDBC nullable code.
	if nullable["author_id"] || nullable["name"] {
		t.Error("expected author_id and name to be NOT NULL")
	}
	if !nullable["rating"] {
		t.Error("expected rating to be nullable")
	}
}

// TestTrinoCatalogFilters covers the level no other model has.
//
// Trino is the only product here with three levels, so it is the only model
// that answers a catalog filter, and a filter that quietly matched nothing
// would look the same as an empty catalog.
func TestTrinoCatalogFilters(t *testing.T) {
	db := openTrino(t)
	m := setupTrino(t, db)
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
	if got := count(trArgs()); got != 5 {
		t.Errorf("expected the five fixture relations, got %d", got)
	}
}

// TestTrinoCatalogsAreDatabases checks the kind that maps a Trino catalog onto
// what psql calls a database, and the connector onto an access method.
func TestTrinoCatalogsAreDatabases(t *testing.T) {
	db := openTrino(t)
	m := setupTrino(t, db)
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

// TestTrinoViewsCarryTheirDefinition covers the kind dbtpl needs, and the one
// query that reads the session catalog rather than spanning every catalog.
func TestTrinoViewsCarryTheirDefinition(t *testing.T) {
	db := openTrino(t)
	m := setupTrino(t, db)
	ctx := t.Context()

	var views int
	for v, err := range dbmeta.Views.All(ctx, m, db, trArgs()) {
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
		if !v.Comment.Valid || v.Comment.V != "the newest books" {
			t.Errorf("expected the view comment, got %v", v.Comment)
		}
	}
	if views != 1 {
		t.Errorf("expected the one fixture view, got %d", views)
	}
}

// TestTrinoRolesAreEmptyOnMemory pins what the memory connector cannot do.
//
// Roles, role grants and privileges are real queries against the standard
// views, and the memory connector implements no role management, so they run
// and return nothing. A connector with sql-standard security populates them.
// Without this test that is indistinguishable from a query that is broken.
func TestTrinoRolesAreEmptyOnMemory(t *testing.T) {
	db := openTrino(t)
	m := setupTrino(t, db)
	ctx := t.Context()

	for _, q := range []struct {
		name string
		run  func() error
	}{
		{"roles", func() error {
			for _, err := range dbmeta.Roles.All(ctx, m, db, nil) {
				if err != nil {
					return err
				}
				return errors.New("expected no row")
			}
			return nil
		}},
		{"role_grants", func() error {
			for _, err := range dbmeta.RoleGrants.All(ctx, m, db, nil) {
				if err != nil {
					return err
				}
				return errors.New("expected no row")
			}
			return nil
		}},
		{"privileges", func() error {
			for _, err := range dbmeta.Privileges.All(ctx, m, db, trArgs()) {
				if err != nil {
					return err
				}
				return errors.New("expected no row")
			}
			return nil
		}},
	} {
		if err := q.run(); err != nil {
			t.Errorf("%s: %v", q.name, err)
		}
	}
}

// TestTrinoCurrentSessionReadsTheCatalog checks the two connection questions.
func TestTrinoCurrentSessionReadsTheCatalog(t *testing.T) {
	db := openTrino(t)
	m := setupTrino(t, db)
	ctx := t.Context()

	var schemas int
	for v, err := range dbmeta.CurrentSchema.All(ctx, m, db, nil) {
		if err != nil {
			t.Fatalf("reading the current schema: %v", err)
		}
		schemas++
		if v.Catalog != "memory" {
			t.Errorf("expected the memory catalog, got %q", v.Catalog)
		}
	}
	if schemas != 1 {
		t.Errorf("expected one current schema, got %d", schemas)
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
