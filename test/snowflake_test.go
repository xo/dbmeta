package test

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"

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
