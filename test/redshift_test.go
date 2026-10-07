package test

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/xo/dbmeta"
	_ "github.com/xo/dbmeta/models/redshift"
	rsfixture "github.com/xo/dbmeta/models/redshift/fixture"
)

// pgxDSN rewrites the scheme that dburl names for Redshift, which is redshift,
// to the one pgx reads, which is postgres. Any other address is unchanged.
func pgxDSN(dsn string) string {
	if rest, ok := strings.CutPrefix(dsn, "redshift://"); ok {
		return "postgres://" + rest
	}
	return dsn
}

// openRedshift returns a connection to the service named by DBMETA_REDSHIFT, which
// dbrun resolves from the places D117 names. The model was written before
// an account was provisioned, and these tests are what finish it (D144).
func openRedshift(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("DBMETA_REDSHIFT")
	if dsn == "" {
		t.Skip("set DBMETA_REDSHIFT to run against the service")
	}
	db, err := sql.Open("pgx", pgxDSN(dsn))
	if err != nil {
		t.Fatalf("opening: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.PingContext(t.Context()); err != nil {
		t.Fatalf("connecting: %v", err)
	}
	return db
}

// setupRedshift builds the fixture and returns the metadata for the service.
func setupRedshift(t *testing.T, db *sql.DB) *dbmeta.Meta {
	t.Helper()
	ctx := t.Context()
	versions, err := dbmeta.Redshift.Version(ctx, db)
	if err != nil {
		t.Fatalf("reading the version: %v", err)
	}
	down, err := rsfixture.Everything.ResolveTeardown(versions)
	if err != nil {
		t.Fatalf("resolving the teardown: %v", err)
	}
	for _, step := range down {
		if !step.Skipped {
			//nolint:errcheck // a teardown before setup is best effort
			db.ExecContext(ctx, step.Query)
		}
	}
	up, err := rsfixture.Everything.ResolveSetup(versions)
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
	m, err := dbmeta.New(dbmeta.Redshift, versions)
	if err != nil {
		t.Fatalf("building the metadata: %v", err)
	}
	t.Logf("service reports %s", versions)
	return m
}

// TestRedshiftSmoke runs each query and checks the columns match the fields.
func TestRedshiftSmoke(t *testing.T) {
	db := openRedshift(t)
	m := setupRedshift(t, db)
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

// TestRedshiftScansEveryQuery reads every query through its own Scan.
func TestRedshiftScansEveryQuery(t *testing.T) {
	db := openRedshift(t)
	scanEveryQuery(t, setupRedshift(t, db), db)
}
