package test

import (
	"context"
	"database/sql"
	"os"
	"testing"

	_ "github.com/snowflakedb/gosnowflake/v2"

	"github.com/xo/dbmeta"
	_ "github.com/xo/dbmeta/models/snowflake"
	sffixture "github.com/xo/dbmeta/models/snowflake/fixture"
)

// openSnowflake returns a connection to the service named by DBMETA_SNOWFLAKE, which
// dbrun resolves from the places D117 names. The model was written before
// an account was provisioned, and these tests are what finish it (D144).
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
