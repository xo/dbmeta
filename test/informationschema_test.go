package test

import (
	"context"
	"database/sql"
	"strconv"
	"testing"

	"github.com/xo/dbmeta"
	is "github.com/xo/dbmeta/models/informationschema"
	isfixture "github.com/xo/dbmeta/models/informationschema/fixture"
)

// The shared information_schema model, registered under a dialect of its own
// so it can be exercised against a PostgreSQL server without colliding with
// the native PostgreSQL model.
//
// PostgreSQL has an information_schema and follows the standard closely, which
// makes it the cheapest server to prove the shared queries on. The queries are
// the same ones MySQL, SQL Server, DuckDB and the rest will use, so running
// them here catches a fault before any of those models exist.
const isDialect dbmeta.Dialect = "infoschema_over_postgres"

func init() {
	is.Register(isDialect, is.Profile{
		Placeholder:    func(n int) string { return "$" + strconv.Itoa(n) },
		VersionQuery:   `SHOW server_version`,
		VersionColumns: 1,
		ParseVersion: func(cols []string) (dbmeta.VersionSet, error) {
			var s dbmeta.VersionSet
			s.Set("", dbmeta.ParseVersion(cols[0]))
			s.Display = "PostgreSQL " + cols[0]
			return s, nil
		},
		SystemSchemas: []string{"information_schema", "pg_catalog", "pg_toast"},
	})
}

// setupIS builds the shared fixture and returns the meta for it.
func setupIS(t *testing.T, db *sql.DB) (*dbmeta.Meta, isfixture.Fixture) {
	t.Helper()
	versions, err := isDialect.Version(t.Context(), db)
	if err != nil {
		t.Fatalf("reading the version: %v", err)
	}
	m, err := dbmeta.New(isDialect, versions)
	if err != nil {
		t.Fatalf("building the meta: %v", err)
	}
	f := isfixture.Everything(isfixture.PostgreSQL())

	run := func(ctx context.Context, steps []isfixture.Step, fatal bool) {
		for _, s := range steps {
			if _, err := db.ExecContext(ctx, s.Query); err != nil && fatal {
				t.Fatalf("%s: %v\n%s", s.Name, err, s.Query)
			}
		}
	}
	run(t.Context(), f.Teardown, false)
	run(t.Context(), f.Setup, true)
	t.Cleanup(func() { run(context.Background(), f.Teardown, false) })
	return m, f
}

// TestInformationSchemaQueriesRun executes every query the shared model
// registers, against a real server, and checks the columns match the declared
// fields. These are the queries every database without a native model will
// use, so a fault here is a fault for all of them.
func TestInformationSchemaQueriesRun(t *testing.T) {
	db := open(t)
	m, f := setupIS(t, db)

	var ran int
	for _, q := range dbmeta.Queries() {
		if q.Support(m) != dbmeta.Supported {
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
	t.Logf("the shared model answered %d queries against %s, fixture schema %s", ran, m, f.Schema)
	if ran == 0 {
		t.Fatal("expected the shared model to answer something")
	}
}

// TestInformationSchemaFindsTheFixture checks the queries return the objects
// the fixture built, rather than running cleanly and finding nothing.
func TestInformationSchemaFindsTheFixture(t *testing.T) {
	db := open(t)
	m, f := setupIS(t, db)
	ctx := t.Context()
	a := dbmeta.Args{Schema: f.Schema}.Map()

	want := map[string]bool{"author": false, "book": false, "recent": false}
	for v, err := range dbmeta.Tables.All(ctx, m, db, a) {
		if err != nil {
			t.Fatalf("reading tables: %v", err)
		}
		if _, ok := want[v.Name]; ok {
			want[v.Name] = true
		}
	}
	for name, found := range want {
		if !found {
			t.Errorf("expected the shared model to find %q", name)
		}
	}

	var cols int
	for v, err := range dbmeta.Columns.All(ctx, m, db, a) {
		if err != nil {
			t.Fatalf("reading columns: %v", err)
		}
		cols++
		if v.Schema != f.Schema {
			t.Errorf("expected a column of %s, got one of %s", f.Schema, v.Schema)
		}
	}
	if cols == 0 {
		t.Error("expected the shared model to find columns")
	}

	var constraints int
	for v, err := range dbmeta.Constraints.All(ctx, m, db, a) {
		if err != nil {
			t.Fatalf("reading constraints: %v", err)
		}
		constraints++
		if v.Type == "" {
			t.Errorf("expected a constraint type for %s", v.Name)
		}
	}
	if constraints == 0 {
		t.Error("expected the fixture's primary, unique, foreign key and check constraints")
	}
}

// TestSharedAndNativeAgree compares the two models on the same server and the
// same objects. They will not agree on everything, because the native model
// reads more, but they must agree on the object names, which is the part both
// claim to answer.
func TestSharedAndNativeAgree(t *testing.T) {
	db := open(t)
	shared, f := setupIS(t, db)
	native := setup(t, db)

	names := func(m *dbmeta.Meta, schema string) map[string]bool {
		out := map[string]bool{}
		for v, err := range dbmeta.Tables.All(t.Context(), m, db, dbmeta.Args{Schema: schema}.Map()) {
			if err != nil {
				t.Fatalf("reading tables: %v", err)
			}
			out[v.Name] = true
		}
		return out
	}
	// The comparison runs one way only. Everything the shared model finds,
	// the native model must find too, because the native one reads the
	// catalog directly. The reverse does not hold and must not be asserted:
	// information_schema.tables has no row for a sequence or a materialized
	// view, so the native model legitimately finds more. That is the gap
	// docs/QUERIES.md describes, not a fault.
	a, b := names(shared, f.Schema), names(native, f.Schema)
	for name := range a {
		if !b[name] {
			t.Errorf("the shared model found %q and the native model did not", name)
		}
	}
	var onlyNative []string
	for name := range b {
		if !a[name] {
			onlyNative = append(onlyNative, name)
		}
	}
	t.Logf("both models found %d relations, and the native model found %d more that information_schema does not list: %v",
		len(a), len(onlyNative), onlyNative)
}
