package test

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"

	_ "github.com/rqlite/gorqlite/stdlib"

	"github.com/xo/dbmeta"
	_ "github.com/xo/dbmeta/models/rqlite"
	rqfixture "github.com/xo/dbmeta/models/rqlite/fixture"
	sqfixture "github.com/xo/dbmeta/models/sqlite3/fixture"
)

// openRqlite returns a connection to the server named by DBMETA_RQLITE, with
// the database/sql driver of github.com/rqlite/gorqlite. dbimp has no rqlite
// driver yet, and Ken chose this one until it does.
func openRqlite(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("DBMETA_RQLITE")
	if dsn == "" {
		t.Skip("set DBMETA_RQLITE to run against a real server")
	}
	return openRqliteAt(t, dsn)
}

// openRqliteAt opens dsn. gorqlite reads /status on open to find the other
// nodes, which an ordinary user may not read, so discovery is turned off.
func openRqliteAt(t *testing.T, dsn string) *sql.DB {
	t.Helper()
	if !strings.Contains(dsn, "disableClusterDiscovery") {
		sep := "?"
		if strings.Contains(dsn, "?") {
			sep = "&"
		}
		dsn += sep + "disableClusterDiscovery=true"
	}
	db, err := sql.Open("rqlite", dsn)
	if err != nil {
		t.Fatalf("opening: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.PingContext(t.Context()); err != nil {
		t.Fatalf("connecting: %v", err)
	}
	return db
}

// setupRqlite builds the fixture and returns the metadata for the server. It
// tears down first, because a run that failed part way leaves the objects
// behind.
func setupRqlite(t *testing.T, db *sql.DB) *dbmeta.Meta {
	t.Helper()
	versions, err := dbmeta.Rqlite.Version(t.Context(), db)
	if err != nil {
		t.Fatalf("reading the version: %v", err)
	}
	m, err := dbmeta.New(dbmeta.Rqlite, versions)
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
	down, err := rqfixture.Everything.ResolveTeardown(versions)
	if err != nil {
		t.Fatalf("resolving the teardown: %v", err)
	}
	up, err := rqfixture.Everything.ResolveSetup(versions)
	if err != nil {
		t.Fatalf("resolving the setup: %v", err)
	}
	run(t.Context(), down, false)
	run(t.Context(), up, true)
	t.Cleanup(func() { run(context.WithoutCancel(t.Context()), down, false) })
	t.Logf("server reports %s", versions)
	return m
}

// TestRqliteVersion reads the version, which is the release of SQLite the
// server runs.
func TestRqliteVersion(t *testing.T) {
	db := openRqlite(t)
	versions, err := dbmeta.Rqlite.Version(t.Context(), db)
	if err != nil {
		t.Fatalf("reading the version: %v", err)
	}
	if versions.Main().Compare(dbmeta.V(3, 37)) < 0 {
		t.Errorf("expected SQLite 3.37 or newer in %s", versions)
	}
	if !strings.HasPrefix(versions.String(), "rqlite") {
		t.Errorf("expected the product in %q", versions.String())
	}
	t.Logf("server reports %s", versions)
}

// TestRqliteScansEveryQuery reads every query through its own Scan.
func TestRqliteScansEveryQuery(t *testing.T) {
	db := openRqlite(t)
	scanEveryQuery(t, setupRqlite(t, db), db)
}
