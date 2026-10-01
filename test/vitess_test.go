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
	_ "github.com/xo/dbmeta/models/vitess"
	vtfixture "github.com/xo/dbmeta/models/vitess/fixture"
)

// openVitess returns a connection to the server named by DBMETA_VITESS, with
// the mysql driver, which is what dburl's vitess:// opens (D154).
func openVitess(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("DBMETA_VITESS")
	if dsn == "" {
		t.Skip("set DBMETA_VITESS to run against a real server")
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

// setupVitess builds the fixture and returns the metadata for the server. It
// tears down first, because a run that failed part way leaves the schema
// behind.
func setupVitess(t *testing.T, db *sql.DB) *dbmeta.Meta {
	t.Helper()
	versions, err := dbmeta.Vitess.Version(t.Context(), db)
	if err != nil {
		t.Fatalf("reading the version: %v", err)
	}
	m, err := dbmeta.New(dbmeta.Vitess, versions)
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
	down, err := vtfixture.Everything.ResolveTeardown(versions)
	if err != nil {
		t.Fatalf("resolving the teardown: %v", err)
	}
	up, err := vtfixture.Everything.ResolveSetup(versions)
	if err != nil {
		t.Fatalf("resolving the setup: %v", err)
	}
	run(t.Context(), down, false)
	run(t.Context(), up, true)
	t.Cleanup(func() { run(context.WithoutCancel(t.Context()), down, false) })
	t.Logf("server reports %s", versions)
	return m
}

// vtArgs is the filter the fixture's objects sit behind. It names the
// keyspace, which is what every query reports.
func vtArgs() map[string]any {
	return dbmeta.Args{Schema: vtfixture.Everything.Schema}.Map()
}

// TestVitessVersion reads the version. Vitess claims a MySQL release, and
// names its own only in @@version_comment, and the model keeps both.
func TestVitessVersion(t *testing.T) {
	db := openVitess(t)
	versions, err := dbmeta.Vitess.Version(t.Context(), db)
	if err != nil {
		t.Fatalf("reading the version: %v", err)
	}
	own := versions.Get("vitess")
	if own.Unknown || own.Compare(dbmeta.V(23)) < 0 {
		t.Errorf("expected Vitess 23 or newer in %s", versions)
	}
	if !versions.Has("mysql") || versions.Main().Unknown {
		t.Errorf("expected the MySQL release Vitess claims in %s", versions)
	}
	if !strings.HasPrefix(versions.String(), "Vitess ") {
		t.Errorf("expected the product in %q", versions.String())
	}
	t.Logf("server reports %s", versions)
}

// TestVitessSmoke runs each query and checks the columns match the fields.
func TestVitessSmoke(t *testing.T) {
	db := openVitess(t)
	m := setupVitess(t, db)
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

// TestVitessScansEveryQuery reads every query through its own Scan.
func TestVitessScansEveryQuery(t *testing.T) {
	db := openVitess(t)
	scanEveryQuery(t, setupVitess(t, db), db)
}

// TestVitessHidesItsSystemSchemas checks the one thing the shared statements
// do differently on Vitess. The database _vt holds the state of the tablet,
// and a filter written for MySQL listed its tables as a user's.
func TestVitessHidesItsSystemSchemas(t *testing.T) {
	db := openVitess(t)
	m := setupVitess(t, db)
	for v, err := range dbmeta.Tables.All(t.Context(), m, db, nil) {
		if err != nil {
			t.Fatalf("reading tables: %v", err)
		}
		switch strings.ToLower(v.Schema) {
		case "_vt", "information_schema", "performance_schema", "mysql", "sys":
			t.Errorf("%s.%s is listed without the system objects", v.Schema, v.Name)
		}
	}
}

// TestVitessNamesAKeyspace checks that every schema is named by its
// keyspace, and that vtgate accepts the name in a query. information_schema
// names the database of a shard, such as vt_dbmeta_0, which vtgate refuses.
// See D135.
func TestVitessNamesAKeyspace(t *testing.T) {
	db := openVitess(t)
	ctx := t.Context()
	m := setupVitess(t, db)

	schemas := map[string]bool{}
	for v, err := range dbmeta.Schemas.All(ctx, m, db, nil) {
		if err != nil {
			t.Fatalf("reading schemas: %v", err)
		}
		schemas[v.Name] = true
		if strings.HasPrefix(v.Name, "vt_") {
			t.Errorf("%s: expected a keyspace and got the database of a shard", v.Name)
		}
	}
	if !schemas["dbmeta"] || !schemas[vtfixture.Everything.Schema] {
		t.Errorf("expected the keyspaces dbmeta and %s, got %v", vtfixture.Everything.Schema, schemas)
	}

	current, ok, err := dbmeta.First(dbmeta.CurrentSchema.All(ctx, m, db, nil))
	if err != nil || !ok || current.Name != "dbmeta" {
		t.Errorf("expected the current schema dbmeta, got %+v, %v", current, err)
	}

	var n int
	for v, err := range dbmeta.Tables.All(ctx, m, db, vtArgs()) {
		if err != nil {
			t.Fatalf("reading tables: %v", err)
		}
		n++
		var count int
		//nolint:gosec // both names come from the catalog, and this is a test
		q := "SELECT COUNT(*) FROM `" + v.Schema + "`.`" + v.Name + "`"
		if err := db.QueryRowContext(ctx, q).Scan(&count); err != nil {
			t.Errorf("%s.%s: vtgate refuses the name: %v", v.Schema, v.Name, err)
		}
	}
	if n == 0 {
		t.Error("expected the fixture's tables")
	}
}

// TestVitessFixtureObjects reads the fixture back, under the name of its
// keyspace. The MySQL database that stores it is vt_dbmeta_fixture_0, and no
// query reports that name.
func TestVitessFixtureObjects(t *testing.T) {
	db := openVitess(t)
	ctx := t.Context()
	m := setupVitess(t, db)

	tables := map[string]string{}
	for v, err := range dbmeta.Tables.All(ctx, m, db, vtArgs()) {
		if err != nil {
			t.Fatalf("reading tables: %v", err)
		}
		tables[v.Name] = v.Type
	}
	for name, want := range map[string]string{"author": "table", "book": "table", "recent": "view"} {
		if tables[name] != want {
			t.Errorf("%s: expected %s, got %q", name, want, tables[name])
		}
	}

	var fk bool
	for v, err := range dbmeta.Constraints.All(ctx, m, db, vtArgs()) {
		if err != nil {
			t.Fatalf("reading constraints: %v", err)
		}
		fk = fk || v.Name == "book_author_fk" && v.Type == "foreign key"
	}
	if !fk {
		t.Error("expected the foreign key book_author_fk")
	}

	var addup bool
	for v, err := range dbmeta.RoutineParameters.All(ctx, m, db, vtArgs()) {
		if err != nil {
			t.Fatalf("reading routine parameters: %v", err)
		}
		addup = addup || v.Routine == "addup" && v.Name.V == "total" && v.Mode == "out"
	}
	if !addup {
		t.Error("expected the OUT parameter total of addup")
	}

	// A sequence is a table with the comment vitess_sequence, so it is
	// listed as a table too.
	var seq bool
	for v, err := range dbmeta.Sequences.All(ctx, m, db, vtArgs()) {
		if err != nil {
			t.Fatalf("reading sequences: %v", err)
		}
		seq = seq || v.Name == "counter" && v.DataType.V == "bigint" && v.Increment.V == "1" && !v.Start.Valid
	}
	if !seq {
		t.Error("expected the sequence counter, a bigint by 1 with no start")
	}
	if tables["counter"] != "table" {
		t.Errorf("counter: expected a table, got %q", tables["counter"])
	}

	var partitioned bool
	for v, err := range dbmeta.PartitionedTables.All(ctx, m, db, vtArgs()) {
		if err != nil {
			t.Fatalf("reading partitioned tables: %v", err)
		}
		partitioned = partitioned || v.Name == "sales" && v.Strategy == "range"
	}
	if !partitioned {
		t.Error("expected sales, partitioned by range")
	}
}

// TestVitessAnswersNoneOfThese checks that what Vitess does not have is
// refused rather than answered with no rows (D34).
func TestVitessAnswersNoneOfThese(t *testing.T) {
	db := openVitess(t)
	versions, err := dbmeta.Vitess.Version(t.Context(), db)
	if err != nil {
		t.Fatalf("reading the version: %v", err)
	}
	m, err := dbmeta.New(dbmeta.Vitess, versions)
	if err != nil {
		t.Fatalf("building the meta: %v", err)
	}
	for _, q := range []dbmeta.AnyQuery{
		dbmeta.Triggers, dbmeta.Roles, dbmeta.RoleGrants, dbmeta.Privileges,
		dbmeta.ForeignServers, dbmeta.UserMappings, dbmeta.ForeignTables,
		dbmeta.Aggregates, dbmeta.ColumnStats,
	} {
		if s := q.Support(m); s != dbmeta.NotSupported {
			t.Errorf("%s: expected %v, got %v", q.Name(), dbmeta.NotSupported, s)
		}
	}
}
