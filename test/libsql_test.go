package test

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"

	_ "github.com/xo/dbimp/libsql"

	"github.com/xo/dbmeta"
	_ "github.com/xo/dbmeta/models/libsql"
	lsfixture "github.com/xo/dbmeta/models/libsql/fixture"
	sqfixture "github.com/xo/dbmeta/models/sqlite3/fixture"
)

// openLibSQL returns a connection to the server named by DBMETA_LIBSQL, with
// dbimp's libSQL driver, which is what dburl names (D154). dbrun sets the
// variable to the libsql:// URL, because the driver takes no other form
// (D160).
func openLibSQL(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("DBMETA_LIBSQL")
	if dsn == "" {
		t.Skip("set DBMETA_LIBSQL to run against a real server")
	}
	return openLibSQLAt(t, dsn)
}

// openLibSQLAt opens dsn.
func openLibSQLAt(t *testing.T, dsn string) *sql.DB {
	t.Helper()
	db, err := sql.Open("libsql", dsn)
	if err != nil {
		t.Fatalf("opening: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.PingContext(t.Context()); err != nil {
		t.Fatalf("connecting: %v", err)
	}
	return db
}

// setupLibSQL builds the fixture and returns the metadata for the server. It
// tears down first, because a run that failed part way leaves the objects
// behind.
func setupLibSQL(t *testing.T, db *sql.DB) *dbmeta.Meta {
	t.Helper()
	versions, err := dbmeta.LibSQL.Version(t.Context(), db)
	if err != nil {
		t.Fatalf("reading the version: %v", err)
	}
	m, err := dbmeta.New(dbmeta.LibSQL, versions)
	if err != nil {
		t.Fatalf("building the meta: %v", err)
	}
	run := func(ctx context.Context, steps []sqfixture.Result, fatal bool) {
		for _, s := range steps {
			if _, err := db.ExecContext(ctx, s.Query); err != nil && fatal {
				t.Fatalf("%s: %v\n%s", s.Name, err, s.Query)
			}
		}
	}
	down, err := lsfixture.Everything.ResolveTeardown(versions)
	if err != nil {
		t.Fatalf("resolving the teardown: %v", err)
	}
	up, err := lsfixture.Everything.ResolveSetup(versions)
	if err != nil {
		t.Fatalf("resolving the setup: %v", err)
	}
	run(t.Context(), down, false)
	run(t.Context(), up, true)
	t.Cleanup(func() { run(context.WithoutCancel(t.Context()), down, false) })
	t.Logf("server reports %s", versions)
	return m
}

// TestLibSQLVersion reads the version, which is the release of SQLite the
// server runs.
func TestLibSQLVersion(t *testing.T) {
	db := openLibSQL(t)
	versions, err := dbmeta.LibSQL.Version(t.Context(), db)
	if err != nil {
		t.Fatalf("reading the version: %v", err)
	}
	if versions.Main().Compare(dbmeta.V(3, 37)) < 0 {
		t.Errorf("expected SQLite 3.37 or newer in %s", versions)
	}
	if !strings.HasPrefix(versions.String(), "libSQL") {
		t.Errorf("expected the product in %q", versions.String())
	}
	t.Logf("server reports %s", versions)
}

// TestLibSQLScansEveryQuery reads every query through its own Scan.
func TestLibSQLScansEveryQuery(t *testing.T) {
	db := openLibSQL(t)
	scanEveryQuery(t, setupLibSQL(t, db), db)
}

// TestLibSQLVectorIndex checks the one object libSQL adds that a query
// reads. The vector index reports as diskann, and the two tables libSQL keeps
// for it are system tables: a query lists them only when the caller asks for
// the system objects, and so are their columns, indexes and constraints
// (D160).
func TestLibSQLVectorIndex(t *testing.T) {
	db := openLibSQL(t)
	m := setupLibSQL(t, db)
	ctx := t.Context()
	internal := []string{"libsql_vector_meta_shadow", "embedding_vector_shadow"}

	tables := func(system bool) map[string]bool {
		found := make(map[string]bool)
		for v, err := range dbmeta.Tables.All(ctx, m, db, dbmeta.Args{Schema: "main", WithSystem: system}.Map()) {
			if err != nil {
				t.Fatalf("reading tables: %v", err)
			}
			found[v.Name] = true
		}
		return found
	}
	user, all := tables(false), tables(true)
	if !user["embedding"] {
		t.Error("expected the embedding table")
	}
	for _, name := range internal {
		if user[name] {
			t.Errorf("expected %s to be left out without the system objects", name)
		}
		if !all[name] {
			t.Errorf("expected %s with the system objects", name)
		}
	}

	var vector bool
	for v, err := range dbmeta.Indexes.All(ctx, m, db, dbmeta.Args{Schema: "main"}.Map()) {
		if err != nil {
			t.Fatalf("reading indexes: %v", err)
		}
		switch v.Name {
		case "embedding_vector":
			vector = true
			if v.Type != "diskann" || v.Unique || v.Primary {
				t.Errorf("expected a vector index, got %+v", v)
			}
		case "embedding_vector_shadow_idx":
			t.Errorf("expected the index on a shadow table to be left out, got %+v", v)
		case "book_published":
			if v.Type != "btree" {
				t.Errorf("expected a plain index to stay btree, got %+v", v)
			}
		}
	}
	if !vector {
		t.Error("expected the vector index")
	}

	// The index is on libsql_vector_idx(vector), which is an expression, so
	// its one column has no name.
	var columns int
	for v, err := range dbmeta.IndexColumns.All(ctx, m, db, dbmeta.Args{Schema: "main", Name: "embedding_vector%"}.Map()) {
		if err != nil {
			t.Fatalf("reading index columns: %v", err)
		}
		if v.Index != "embedding_vector" {
			t.Errorf("expected only the vector index, got %+v", v)
			continue
		}
		columns++
		if v.Name.Valid || v.Expression.V != "expression" {
			t.Errorf("expected an expression, got %+v", v)
		}
	}
	if columns != 1 {
		t.Errorf("expected one column of the vector index, got %d", columns)
	}

	// The declared type of a vector column is what the column reads.
	for v, err := range dbmeta.Columns.All(ctx, m, db, dbmeta.Args{Schema: "main", Parent: "embedding", Name: "vector"}.Map()) {
		if err != nil {
			t.Fatalf("reading columns: %v", err)
		}
		if v.DataType != "F32_BLOB(3)" {
			t.Errorf("expected F32_BLOB(3), got %q", v.DataType)
		}
	}
}
