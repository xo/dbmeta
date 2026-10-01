package test

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	_ "github.com/go-sql-driver/mysql"

	"github.com/xo/dbmeta"
	_ "github.com/xo/dbmeta/models/singlestore"
	ssfixture "github.com/xo/dbmeta/models/singlestore/fixture"
)

// openSingleStore returns a connection to the server named by DBMETA_MEMSQL,
// with the mysql driver, which is what dburl's memsql:// opens (D154).
func openSingleStore(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("DBMETA_MEMSQL")
	if dsn == "" {
		t.Skip("set DBMETA_MEMSQL to run against a real server")
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

// setupSingleStore builds the fixture and returns the metadata for the server. It
// tears down first, because a run that failed part way leaves the schema
// behind.
func setupSingleStore(t *testing.T, db *sql.DB) *dbmeta.Meta {
	t.Helper()
	versions, err := dbmeta.MemSQL.Version(t.Context(), db)
	if err != nil {
		t.Fatalf("reading the version: %v", err)
	}
	m, err := dbmeta.New(dbmeta.MemSQL, versions)
	if err != nil {
		t.Fatalf("building the meta: %v", err)
	}
	// The steps run on one connection, because the fixture selects its
	// database with USE before a step that needs one. The connection is
	// discarded afterwards, so that no other query inherits the database.
	run := func(ctx context.Context, steps []ssfixture.Result, fatal bool) {
		conn, err := db.Conn(ctx)
		if err != nil {
			t.Fatalf("taking a connection: %v", err)
		}
		defer func() {
			// Raw returning ErrBadConn closes the connection and discards it
			// rather than returning it to the pool, so no Close follows.
			if err := conn.Raw(func(any) error { return driver.ErrBadConn }); !errors.Is(err, driver.ErrBadConn) {
				t.Errorf("discarding the fixture connection: %v", err)
			}
		}()
		for _, s := range steps {
			if s.Skipped {
				continue
			}
			if _, err := conn.ExecContext(ctx, s.Query); err != nil && fatal {
				t.Fatalf("%s: %v\n%s", s.Name, err, s.Query)
			}
		}
	}
	down, err := ssfixture.Everything.ResolveTeardown(versions)
	if err != nil {
		t.Fatalf("resolving the teardown: %v", err)
	}
	up, err := ssfixture.Everything.ResolveSetup(versions)
	if err != nil {
		t.Fatalf("resolving the setup: %v", err)
	}
	run(t.Context(), down, false)
	run(t.Context(), up, true)
	t.Cleanup(func() { run(context.WithoutCancel(t.Context()), down, false) })
	t.Logf("server reports %s", versions)
	return m
}

// ssArgs is the filter the fixture's objects sit behind.
func ssArgs() map[string]any {
	return dbmeta.Args{Schema: ssfixture.Everything.Schema}.Map()
}

// TestSingleStoreVersion reads the version. SingleStore claims a MySQL
// release and names its own in @@memsql_version, and the model keeps both.
func TestSingleStoreVersion(t *testing.T) {
	db := openSingleStore(t)
	versions, err := dbmeta.MemSQL.Version(t.Context(), db)
	if err != nil {
		t.Fatalf("reading the version: %v", err)
	}
	own := versions.Get("memsql")
	if own.Unknown || own.Compare(dbmeta.V(9)) < 0 {
		t.Errorf("expected SingleStore 9 or newer in %s", versions)
	}
	if !versions.Has("mysql") || versions.Main().Unknown {
		t.Errorf("expected the MySQL release SingleStore claims in %s", versions)
	}
	if !strings.HasPrefix(versions.String(), "SingleStore ") {
		t.Errorf("expected the product in %q", versions.String())
	}
	t.Logf("server reports %s", versions)
}

// TestSingleStoreSmoke runs each query and checks the columns match the
// fields.
func TestSingleStoreSmoke(t *testing.T) {
	db := openSingleStore(t)
	m := setupSingleStore(t, db)
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

// TestSingleStoreScansEveryQuery reads every query through its own Scan.
func TestSingleStoreScansEveryQuery(t *testing.T) {
	db := openSingleStore(t)
	scanEveryQuery(t, setupSingleStore(t, db), db)
}

// TestSingleStoreHidesItsSystemSchemas checks the schema filter the shared
// statements take on SingleStore. memsql and cluster are its own.
func TestSingleStoreHidesItsSystemSchemas(t *testing.T) {
	db := openSingleStore(t)
	m := setupSingleStore(t, db)
	for v, err := range dbmeta.Tables.All(t.Context(), m, db, nil) {
		if err != nil {
			t.Fatalf("reading tables: %v", err)
		}
		switch strings.ToLower(v.Schema) {
		case "information_schema", "memsql", "cluster":
			t.Errorf("%s.%s is listed without the system objects", v.Schema, v.Name)
		}
	}
}

// TestSingleStoreFixtureObjects reads back what is SingleStore's own.
func TestSingleStoreFixtureObjects(t *testing.T) {
	db := openSingleStore(t)
	ctx := t.Context()
	m := setupSingleStore(t, db)

	// SingleStore reports the primary key of a columnstore table as UNIQUE,
	// and the model reports it as the primary key it is.
	types := map[string]string{}
	for v, err := range dbmeta.Constraints.All(ctx, m, db, ssArgs()) {
		if err != nil {
			t.Fatalf("reading constraints: %v", err)
		}
		types[v.Table+"."+v.Name] = v.Type
	}
	for name, want := range map[string]string{
		"author.PRIMARY": "primary key", "region.PRIMARY": "primary key",
		"book.book_title_unique": "unique",
	} {
		if types[name] != want {
			t.Errorf("%s: expected %s, got %q", name, want, types[name])
		}
	}

	// region's key is also its shard key, and is listed once.
	var keys []string
	for v, err := range dbmeta.IndexColumns.All(ctx, m, db, dbmeta.Args{Schema: ssfixture.Everything.Schema, Parent: "region"}.Map()) {
		if err != nil {
			t.Fatalf("reading index columns: %v", err)
		}
		keys = append(keys, fmt.Sprintf("%s.%s:%d", v.Index, v.Name.V, v.Ordinal))
	}
	if strings.Join(keys, ", ") != "PRIMARY.country:1, PRIMARY.area:2" {
		t.Errorf("expected the key of region once, got %v", keys)
	}

	agg, ok, err := dbmeta.First(dbmeta.Aggregates.All(ctx, m, db, ssArgs()))
	if err != nil || !ok || agg.Name != "total" || !strings.Contains(agg.Source.V, "total_iter") {
		t.Errorf("expected the aggregate total, got %+v, %v", agg, err)
	}

	var addup int
	for v, err := range dbmeta.RoutineParameters.All(ctx, m, db, dbmeta.Args{Schema: ssfixture.Everything.Schema, Parent: "addup"}.Map()) {
		if err != nil {
			t.Fatalf("reading routine parameters: %v", err)
		}
		addup++
		_ = v
	}
	if addup != 2 {
		t.Errorf("expected the two parameters of addup, got %d", addup)
	}

	grant, ok, err := dbmeta.First(dbmeta.RoleGrants.All(ctx, m, db, dbmeta.Args{Name: "dbmeta_fixture_reader"}.Map()))
	if err != nil || !ok || grant.Role != "dbmeta_fixture_group" {
		t.Errorf("expected the reader granted to the group, got %+v, %v", grant, err)
	}

	var privilege bool
	for v, err := range dbmeta.Privileges.All(ctx, m, db, ssArgs()) {
		if err != nil {
			t.Fatalf("reading privileges: %v", err)
		}
		privilege = privilege || v.Type == "schema" && v.Access.V == "dbmeta_fixture_reader=SELECT"
	}
	if !privilege {
		t.Error("expected the reader's SELECT on the fixture's database")
	}

	var rating bool
	for v, err := range dbmeta.ColumnStats.All(ctx, m, db, dbmeta.Args{Schema: ssfixture.Everything.Schema, Parent: "author", Name: "rating"}.Map()) {
		if err != nil {
			t.Fatalf("reading column statistics: %v", err)
		}
		rating = v.NullFrac.Valid
	}
	if !rating {
		t.Error("expected the statistics of rating")
	}

	var settings int
	for _, err := range dbmeta.Settings.All(ctx, m, db, dbmeta.Args{Name: "sync_permissions"}.Map()) {
		if err != nil {
			t.Fatalf("reading settings: %v", err)
		}
		settings++
	}
	if settings != 1 {
		t.Errorf("expected the variable sync_permissions, got %d", settings)
	}
}

// TestSingleStoreAnswersNoneOfThese checks that what SingleStore does not
// have is refused rather than answered with no rows (D34).
func TestSingleStoreAnswersNoneOfThese(t *testing.T) {
	db := openSingleStore(t)
	versions, err := dbmeta.MemSQL.Version(t.Context(), db)
	if err != nil {
		t.Fatalf("reading the version: %v", err)
	}
	m, err := dbmeta.New(dbmeta.MemSQL, versions)
	if err != nil {
		t.Fatalf("building the meta: %v", err)
	}
	for _, q := range []dbmeta.AnyQuery{
		dbmeta.Triggers, dbmeta.Sequences, dbmeta.ForeignServers, dbmeta.UserMappings,
		dbmeta.PartitionedTables, dbmeta.EnumValues, dbmeta.Extensions,
	} {
		if s := q.Support(m); s != dbmeta.NotSupported {
			t.Errorf("%s: expected %v, got %v", q.Name(), dbmeta.NotSupported, s)
		}
	}
}

// TestSingleStoreExtendedStats reads the fixture's correlation back, which is
// SingleStore's functional dependency statistic and has no name (D149).
func TestSingleStoreExtendedStats(t *testing.T) {
	db := openSingleStore(t)
	m := setupSingleStore(t, db)
	var found bool
	for v, err := range dbmeta.ExtendedStats.All(t.Context(), m, db, ssArgs()) {
		if err != nil {
			t.Fatalf("reading extended stats: %v", err)
		}
		if v.Table != "author" {
			continue
		}
		found = true
		if v.Definition.V != "name, rating FROM dbmeta_fixture.author" {
			t.Errorf("expected the two columns and the table, got %q", v.Definition.V)
		}
		if v.Kinds != "f" || !v.Dependencies || v.Ndistinct || v.MCV || v.Name.Valid {
			t.Errorf("expected a nameless functional dependency, got %+v", v)
		}
	}
	if !found {
		t.Error("expected the correlation on author")
	}
}
