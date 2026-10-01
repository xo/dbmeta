package test

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"

	_ "github.com/go-sql-driver/mysql"

	"github.com/xo/dbmeta"
	myfixture "github.com/xo/dbmeta/models/mysql/fixture"
	_ "github.com/xo/dbmeta/models/tidb"
	tdfixture "github.com/xo/dbmeta/models/tidb/fixture"
)

// openTiDB returns a connection to the server named by DBMETA_TIDB, with the
// mysql driver, which is what dburl's tidb:// opens (D154).
func openTiDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("DBMETA_TIDB")
	if dsn == "" {
		t.Skip("set DBMETA_TIDB to run against a real server")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("opening: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.PingContext(t.Context()); err != nil {
		t.Fatalf("connecting: %v", err)
	}
	return db
}

// setupTiDB builds the fixture and returns the metadata for the server. It
// tears down first, because a run that failed part way leaves the schema
// behind.
func setupTiDB(t *testing.T, db *sql.DB) *dbmeta.Meta {
	t.Helper()
	versions, err := dbmeta.TiDB.Version(t.Context(), db)
	if err != nil {
		t.Fatalf("reading the version: %v", err)
	}
	m, err := dbmeta.New(dbmeta.TiDB, versions)
	if err != nil {
		t.Fatalf("building the meta: %v", err)
	}
	run := func(ctx context.Context, steps []myfixture.Result, fatal bool) {
		for _, s := range steps {
			if s.Skipped {
				continue
			}
			if _, err := db.ExecContext(ctx, s.Query); err != nil && fatal {
				t.Fatalf("%s: %v\n%s", s.Name, err, s.Query)
			}
		}
	}
	down, err := tdfixture.Everything.ResolveTeardown(versions)
	if err != nil {
		t.Fatalf("resolving the teardown: %v", err)
	}
	up, err := tdfixture.Everything.ResolveSetup(versions)
	if err != nil {
		t.Fatalf("resolving the setup: %v", err)
	}
	run(t.Context(), down, false)
	run(t.Context(), up, true)
	t.Cleanup(func() { run(context.WithoutCancel(t.Context()), down, false) })
	t.Logf("server reports %s", versions)
	return m
}

// tdArgs is the filter the fixture's objects sit behind.
func tdArgs() map[string]any {
	return dbmeta.Args{Schema: tdfixture.Everything.Schema}.Map()
}

// TestTiDBVersion reads the version. TiDB claims a MySQL release before its
// own, and the model keeps both.
func TestTiDBVersion(t *testing.T) {
	db := openTiDB(t)
	versions, err := dbmeta.TiDB.Version(t.Context(), db)
	if err != nil {
		t.Fatalf("reading the version: %v", err)
	}
	own := versions.Get("tidb")
	if own.Unknown || own.Compare(dbmeta.V(7, 5)) < 0 {
		t.Errorf("expected TiDB 7.5 or newer in %s", versions)
	}
	if !versions.Has("mysql") || versions.Main().Unknown {
		t.Errorf("expected the MySQL release TiDB claims in %s", versions)
	}
	if !strings.HasPrefix(versions.String(), "TiDB ") {
		t.Errorf("expected the product in %q", versions.String())
	}
	t.Logf("server reports %s", versions)
}

// TestTiDBSmoke runs each query and checks the columns match the fields.
func TestTiDBSmoke(t *testing.T) {
	db := openTiDB(t)
	m := setupTiDB(t, db)
	var ran int
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
		ran++
	}
	t.Logf("%d queries ran", ran)
}

// TestTiDBScansEveryQuery reads every query through its own Scan.
func TestTiDBScansEveryQuery(t *testing.T) {
	db := openTiDB(t)
	scanEveryQuery(t, setupTiDB(t, db), db)
}

// TestTiDBHidesItsSystemSchemas checks the one thing the shared statements
// do differently on TiDB. TiDB spells three system schemas in capitals and
// compares a name with its case, and METRICS_SCHEMA is its own, so a filter
// written for MySQL listed all four as a user's.
func TestTiDBHidesItsSystemSchemas(t *testing.T) {
	db := openTiDB(t)
	m := setupTiDB(t, db)
	for v, err := range dbmeta.Tables.All(t.Context(), m, db, nil) {
		if err != nil {
			t.Fatalf("reading tables: %v", err)
		}
		switch strings.ToLower(v.Schema) {
		case "information_schema", "performance_schema", "metrics_schema", "mysql", "sys":
			t.Errorf("%s.%s is listed without the system objects", v.Schema, v.Name)
		}
	}
}

// TestTiDBFixtureObjects reads back what TiDB has and the MySQL fixture does
// not build on MySQL: a sequence, a role granted to a user, and a privilege.
func TestTiDBFixtureObjects(t *testing.T) {
	db := openTiDB(t)
	ctx := t.Context()
	m := setupTiDB(t, db)

	var seq bool
	for v, err := range dbmeta.Sequences.All(ctx, m, db, tdArgs()) {
		if err != nil {
			t.Fatalf("reading sequences: %v", err)
		}
		seq = seq || v.Name == "counter" && v.Start.V == "10" && v.Increment.V == "2"
	}
	if !seq {
		t.Error("expected the sequence counter, starting at 10 by 2")
	}

	var granted bool
	for v, err := range dbmeta.RoleGrants.All(ctx, m, db, nil) {
		if err != nil {
			t.Fatalf("reading role grants: %v", err)
		}
		granted = granted || strings.HasPrefix(v.Role, "dbmeta_member") && strings.HasPrefix(v.MemberOf, "dbmeta_reader")
	}
	if !granted {
		t.Error("expected dbmeta_reader granted to dbmeta_member")
	}

	// Before 8.5 the view that lists a grant is empty, so the query is too
	// old there.
	if dbmeta.Privileges.Support(m) == dbmeta.TooOld {
		t.Logf("privileges: too old on %s", m.Version())
		return
	}
	var privilege bool
	for v, err := range dbmeta.Privileges.All(ctx, m, db, tdArgs()) {
		if err != nil {
			t.Fatalf("reading privileges: %v", err)
		}
		privilege = privilege || v.Name == "book" && strings.Contains(v.Access.V, "dbmeta_member")
	}
	if !privilege {
		t.Error("expected the privilege on book")
	}

	var partitioned bool
	for v, err := range dbmeta.PartitionedTables.All(ctx, m, db, tdArgs()) {
		if err != nil {
			t.Fatalf("reading partitioned tables: %v", err)
		}
		partitioned = partitioned || v.Name == "sales" && v.Strategy == "range"
	}
	if !partitioned {
		t.Error("expected sales, partitioned by range")
	}

	var settings int
	for _, err := range dbmeta.Settings.All(ctx, m, db, dbmeta.Args{Name: "tidb_%"}.Map()) {
		if err != nil {
			t.Fatalf("reading settings: %v", err)
		}
		settings++
	}
	if settings == 0 {
		t.Error("expected the tidb_ variables among the settings")
	}
}

// TestTiDBAnswersNoneOfThese checks that what TiDB does not have is refused
// rather than answered with no rows (D34).
func TestTiDBAnswersNoneOfThese(t *testing.T) {
	db := openTiDB(t)
	versions, err := dbmeta.TiDB.Version(t.Context(), db)
	if err != nil {
		t.Fatalf("reading the version: %v", err)
	}
	m, err := dbmeta.New(dbmeta.TiDB, versions)
	if err != nil {
		t.Fatalf("building the meta: %v", err)
	}
	for _, q := range []dbmeta.AnyQuery{
		dbmeta.Functions, dbmeta.Aggregates, dbmeta.RoutineParameters, dbmeta.Triggers,
		dbmeta.ForeignServers, dbmeta.UserMappings, dbmeta.ForeignTables, dbmeta.AccessMethods,
		dbmeta.Extensions, dbmeta.ColumnStats,
	} {
		if s := q.Support(m); s != dbmeta.NotSupported {
			t.Errorf("%s: expected %v, got %v", q.Name(), dbmeta.NotSupported, s)
		}
	}
}
