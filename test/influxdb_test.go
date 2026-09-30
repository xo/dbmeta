package test

import (
	"database/sql"
	"os"
	"strings"
	"testing"

	_ "github.com/xo/dbimp/influxdb"

	"github.com/xo/dbmeta"
	_ "github.com/xo/dbmeta/models/influxdb"
	ixfixture "github.com/xo/dbmeta/models/influxdb/fixture"
)

// openInfluxDB returns a connection to the InfluxDB 3 server named by
// DBMETA_INFLUXDB, with dbimp's influxdb driver, which is what dburl's
// influxdb scheme opens and what usql uses.
func openInfluxDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("DBMETA_INFLUXDB")
	if dsn == "" {
		t.Skip("set DBMETA_INFLUXDB to run against a real server")
	}
	db, err := sql.Open("influxdb", dsn)
	if err != nil {
		t.Fatalf("opening: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.PingContext(t.Context()); err != nil {
		t.Fatalf("connecting: %v", err)
	}
	return db
}

// setupInfluxDB writes the fixture and returns the metadata for the server.
// InfluxDB 3 has no DROP in SQL, so there is no teardown, and writing the
// fixture again replaces its points.
func setupInfluxDB(t *testing.T, db *sql.DB) *dbmeta.Meta {
	t.Helper()
	versions, err := dbmeta.InfluxDB.Version(t.Context(), db)
	if err != nil {
		t.Fatalf("reading the version: %v", err)
	}
	m, err := dbmeta.New(dbmeta.InfluxDB, versions)
	if err != nil {
		t.Fatalf("building the meta: %v", err)
	}
	up, err := ixfixture.Everything.ResolveSetup(versions)
	if err != nil {
		t.Fatalf("resolving the setup: %v", err)
	}
	for _, s := range up {
		if _, err := db.ExecContext(t.Context(), s.Query); err != nil {
			t.Fatalf("%s: %v\n%s", s.Name, err, s.Query)
		}
	}
	t.Logf("server reports %s", versions)
	return m
}

// ixArgs is the filter the fixture's measurements sit behind.
func ixArgs() map[string]any {
	return dbmeta.Args{Schema: ixfixture.Everything.Schema}.Map()
}

// TestInfluxDBVersion reads the version, which is DataFusion's release.
func TestInfluxDBVersion(t *testing.T) {
	db := openInfluxDB(t)
	versions, err := dbmeta.InfluxDB.Version(t.Context(), db)
	if err != nil {
		t.Fatalf("reading the version: %v", err)
	}
	if versions.Main().Unknown || !strings.HasPrefix(versions.String(), "InfluxDB 3") {
		t.Errorf("expected InfluxDB 3 and a DataFusion release in %q", versions)
	}
	t.Logf("server reports %s", versions)
}

// TestInfluxDBScansEveryQuery reads every query through its own Scan.
func TestInfluxDBScansEveryQuery(t *testing.T) {
	db := openInfluxDB(t)
	scanEveryQuery(t, setupInfluxDB(t, db), db)
}

// TestInfluxDBFixtureObjects reads the fixture's measurements and a function
// with overloads back.
func TestInfluxDBFixtureObjects(t *testing.T) {
	db := openInfluxDB(t)
	ctx := t.Context()
	m := setupInfluxDB(t, db)

	tables := map[string]bool{}
	for v, err := range dbmeta.Tables.All(ctx, m, db, ixArgs()) {
		if err != nil {
			t.Fatalf("reading tables: %v", err)
		}
		tables[v.Name] = v.Type == "table"
	}
	for _, name := range []string{"author", "book", "region", "shipment"} {
		if !tables[name] {
			t.Errorf("expected the table %s in %v", name, tables)
		}
	}

	cols := map[string]dbmeta.Column{}
	for v, err := range dbmeta.Columns.All(ctx, m, db, dbmeta.Args{Schema: "iox", Parent: "shipment"}.Map()) {
		if err != nil {
			t.Fatalf("reading columns: %v", err)
		}
		cols[v.Name] = v
	}
	for name, want := range map[string]string{
		"shipment_id": "Dictionary(Int32, Utf8)", "amount": "Int64", "weight": "Float64",
		"fragile": "Boolean", "time": "Timestamp",
	} {
		if c, ok := cols[name]; !ok || !strings.HasPrefix(c.DataType, want) || c.Ordinal < 1 {
			t.Errorf("%s: expected the type %s from ordinal 1, got %+v", name, want, c)
		}
	}
	if cols["time"].Nullable || !cols["amount"].Nullable {
		t.Errorf("expected time alone NOT NULL, got time=%v amount=%v", cols["time"].Nullable, cols["amount"].Nullable)
	}

	// date_bin has overloads, and each one's parameters carry its id.
	ids := map[string]bool{}
	for v, err := range dbmeta.Functions.All(ctx, m, db, dbmeta.Args{Name: "date_bin", WithSystem: true}.Map()) {
		if err != nil {
			t.Fatalf("reading functions: %v", err)
		}
		if v.Kind != "func" || !v.ID.Valid || !v.ResultType.Valid {
			t.Errorf("expected a function with an id and a result type, got %+v", v)
		}
		ids[v.ID.V] = true
	}
	if len(ids) < 2 {
		t.Errorf("expected several overloads of date_bin, got %v", ids)
	}
	var params int
	for v, err := range dbmeta.RoutineParameters.All(ctx, m, db, dbmeta.Args{Parent: "date_bin"}.Map()) {
		if err != nil {
			t.Fatalf("reading routine parameters: %v", err)
		}
		if !ids[v.RoutineID.V] {
			t.Errorf("parameter %+v names no overload that Functions lists", v)
		}
		params++
	}
	if params == 0 {
		t.Error("expected the parameters of date_bin")
	}

	var aggs int
	for v, err := range dbmeta.Aggregates.All(ctx, m, db, dbmeta.Args{Name: "sum", WithSystem: true}.Map()) {
		if err != nil {
			t.Fatalf("reading aggregates: %v", err)
		}
		if v.Kind != "agg" {
			t.Errorf("expected agg, got %+v", v)
		}
		aggs++
	}
	if aggs == 0 {
		t.Error("expected sum among the aggregates")
	}

	s, ok, err := dbmeta.First(dbmeta.CurrentSchema.All(ctx, m, db, nil))
	if err != nil || !ok || s.Name != "iox" {
		t.Errorf("expected the current schema iox, got %+v, %v, %v", s, ok, err)
	}
}
