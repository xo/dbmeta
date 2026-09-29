package test

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"strings"
	"testing"

	_ "github.com/sclgo/impala-go"

	"github.com/xo/dbmeta"
	_ "github.com/xo/dbmeta/models/impala"
	imfixture "github.com/xo/dbmeta/models/impala/fixture"
)

// openImpala returns a connection to the server named by DBMETA_IMPALA, with
// sclgo/impala-go, which is what usql uses (D52).
func openImpala(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("DBMETA_IMPALA")
	if dsn == "" {
		t.Skip("set DBMETA_IMPALA to run against a real server")
	}
	db, err := sql.Open("impala", dsn)
	if err != nil {
		t.Fatalf("opening: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.PingContext(t.Context()); err != nil {
		t.Fatalf("connecting: %v", err)
	}
	return db
}

// setupImpala builds the fixture and returns the metadata for the server.
func setupImpala(t *testing.T, db *sql.DB) *dbmeta.Meta {
	t.Helper()
	ctx := t.Context()
	versions, err := dbmeta.Impala.Version(ctx, db)
	if err != nil {
		t.Fatalf("reading the version: %v", err)
	}
	down, err := imfixture.Everything.ResolveTeardown(versions)
	if err != nil {
		t.Fatalf("resolving the teardown: %v", err)
	}
	for _, step := range down {
		if !step.Skipped {
			//nolint:errcheck // a teardown before setup is best effort
			db.ExecContext(ctx, step.Query)
		}
	}
	up, err := imfixture.Everything.ResolveSetup(versions)
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
	m, err := dbmeta.New(dbmeta.Impala, versions)
	if err != nil {
		t.Fatalf("building the metadata: %v", err)
	}
	t.Logf("server reports %s", versions)
	return m
}

// imArgs is the filter the fixture's objects sit behind.
func imArgs() map[string]any {
	return dbmeta.Args{Schema: imfixture.Everything.Schema}.Map()
}

// TestImpalaVersion reads the banner.
func TestImpalaVersion(t *testing.T) {
	db := openImpala(t)
	versions, err := dbmeta.Impala.Version(t.Context(), db)
	if err != nil {
		t.Fatalf("reading the version: %v", err)
	}
	if main := versions.Main(); main.Unknown || main.Compare(dbmeta.V(4, 4)) < 0 {
		t.Errorf("expected 4.4 or newer in %s", versions)
	}
	if !strings.HasPrefix(versions.String(), "Apache Impala ") {
		t.Errorf("expected the product in %q", versions.String())
	}
}

// TestImpalaScansEveryQuery reads every query through its own Scan, or its
// walk.
func TestImpalaScansEveryQuery(t *testing.T) {
	db := openImpala(t)
	scanEveryQuery(t, setupImpala(t, db), db)
}

// TestImpalaWalkHasNoOneStatement checks that a query a walk answers says
// so rather than building a statement that answers nothing. See D146.
func TestImpalaWalkHasNoOneStatement(t *testing.T) {
	m, err := dbmeta.New(dbmeta.Impala, dbmeta.VersionSet{})
	if err != nil {
		t.Fatalf("building the metadata: %v", err)
	}
	if _, _, err := dbmeta.Tables.Build(m, nil); !errors.Is(err, dbmeta.ErrSeveralStatements) {
		t.Errorf("tables: expected %v, got %v", dbmeta.ErrSeveralStatements, err)
	}
	if _, _, err := dbmeta.CurrentSchema.Build(m, nil); err != nil {
		t.Errorf("current_schema: expected one statement, got %v", err)
	}
}

// TestImpalaFixtureObjects reads the fixture back through the walks.
func TestImpalaFixtureObjects(t *testing.T) {
	db := openImpala(t)
	ctx := t.Context()
	m := setupImpala(t, db)

	// The metastore makes a new table external, so DESCRIBE FORMATTED
	// reports EXTERNAL_TABLE, measured on 4.4.1 and 4.5.2.
	types := map[string]string{}
	comments := map[string]string{}
	for v, err := range dbmeta.Tables.All(ctx, m, db, imArgs()) {
		if err != nil {
			t.Fatalf("reading tables: %v", err)
		}
		types[v.Name] = v.Type
		comments[v.Name] = v.Comment.V
	}
	for name, want := range map[string]string{"author": "external table", "book": "external table", "recent": "view"} {
		if types[name] != want {
			t.Errorf("%s: expected %s, got %q", name, want, types[name])
		}
	}
	if comments["author"] != "people who write" {
		t.Errorf("author: expected its comment, got %q", comments["author"])
	}

	want := []string{"author_id", "name", "rating", "shade"}
	for v, err := range dbmeta.Columns.All(ctx, m, db, dbmeta.Args{Schema: imfixture.Everything.Schema, Parent: "author"}.Map()) {
		if err != nil {
			t.Fatalf("reading columns: %v", err)
		}
		if v.Ordinal < 1 || v.Ordinal > len(want) || want[v.Ordinal-1] != v.Name {
			t.Errorf("%s: unexpected ordinal %d", v.Name, v.Ordinal)
		}
		if v.Name == "author_id" && (v.DataType != "int" || v.Comment.V != "surrogate key") {
			t.Errorf("author_id: expected int with its comment, got %+v", v)
		}
	}

	view, ok, err := dbmeta.First(dbmeta.Views.All(ctx, m, db, imArgs()))
	if err != nil || !ok || view.Name != "recent" || !strings.Contains(view.Definition.V, "book_id") {
		t.Errorf("expected the view recent with its definition, got %+v, %v", view, err)
	}

	var rating bool
	for v, err := range dbmeta.ColumnStats.All(ctx, m, db, dbmeta.Args{Schema: imfixture.Everything.Schema, Parent: "author", Name: "rating"}.Map()) {
		if err != nil {
			t.Fatalf("reading column statistics: %v", err)
		}
		rating = v.Distinct.Valid && v.Distinct.V == 2
	}
	if !rating {
		t.Error("expected two distinct values of rating")
	}

	sum, ok, err := dbmeta.First(dbmeta.Aggregates.All(ctx, m, db, dbmeta.Args{WithSystem: true, Name: "sum"}.Map()))
	if err != nil || !ok || sum.Schema != "_impala_builtins" || sum.Kind != "agg" {
		t.Errorf("expected the built in sum, got %+v, %v", sum, err)
	}
}

// TestImpalaAnswersNoneOfThese checks that what Impala does not list is
// refused rather than answered with no rows (D34).
func TestImpalaAnswersNoneOfThese(t *testing.T) {
	m, err := dbmeta.New(dbmeta.Impala, dbmeta.VersionSet{})
	if err != nil {
		t.Fatalf("building the metadata: %v", err)
	}
	for _, q := range []dbmeta.AnyQuery{
		dbmeta.Indexes, dbmeta.Constraints, dbmeta.Triggers, dbmeta.Sequences, dbmeta.Roles,
	} {
		if s := q.Support(m); s != dbmeta.NotSupported {
			t.Errorf("%s: expected %v, got %v", q.Name(), dbmeta.NotSupported, s)
		}
	}
}
