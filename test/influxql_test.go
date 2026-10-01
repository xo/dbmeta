package test

import (
	"database/sql"
	"net/url"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/xo/dbimp/influxdb"

	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/container"
	_ "github.com/xo/dbmeta/models/influxql"
	iqfixture "github.com/xo/dbmeta/models/influxql/fixture"
)

// influxQL makes dsn speak InfluxQL, as dburl's influxql scheme does: it
// adds sqlmode=disable where the URL names no sqlmode. dbrun gives the
// influxdb:// URL of the server, and InfluxDB 3 speaks SQL to it by default.
func influxQL(t *testing.T, dsn string) string {
	t.Helper()
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("parsing %s: %v", dsn, err)
	}
	q := u.Query()
	if !q.Has("sqlmode") {
		q.Set("sqlmode", "disable")
	}
	u.RawQuery = q.Encode()
	return u.String()
}

// openInfluxQL returns a connection that speaks InfluxQL to the server named
// by DBMETA_INFLUXQL, with dbimp's influxdb driver, which is what dburl's
// influxql scheme opens (D154).
func openInfluxQL(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("DBMETA_INFLUXQL")
	if dsn == "" {
		t.Skip("set DBMETA_INFLUXQL to run against a real server")
	}
	db, err := sql.Open("influxdb", influxQL(t, dsn))
	if err != nil {
		t.Fatalf("opening: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.PingContext(t.Context()); err != nil {
		t.Fatalf("connecting: %v", err)
	}
	return db
}

// influxRelease reads the release from GET /ping through the driver, which
// is the only place it is, and parses it as a caller does. No InfluxQL
// statement names it (D165).
func influxRelease(t *testing.T, db *sql.DB) dbmeta.VersionSet {
	t.Helper()
	conn, err := db.Conn(t.Context())
	if err != nil {
		t.Fatalf("taking a connection: %v", err)
	}
	var release string
	err = conn.Raw(func(dc any) error {
		var err error
		release, err = influxdb.Version(t.Context(), dc)
		return err
	})
	if cerr := conn.Close(); cerr != nil {
		t.Fatalf("returning the connection: %v", cerr)
	}
	if err != nil {
		t.Fatalf("reading the release: %v", err)
	}
	versions, err := dbmeta.InfluxQL.ParseVersion([]string{release})
	if err != nil {
		t.Fatalf("parsing the release %q: %v", release, err)
	}
	return versions
}

// influxMajor is the major release of the server, 1, 2 or 3.
func influxMajor(m *dbmeta.Meta) uint32 {
	return m.Version().Main().Parts[0]
}

// setupInfluxQL writes the fixture and returns the metadata for the server,
// with the release that GET /ping reports. A point with the same tags and
// time replaces the one before it, so there is no teardown.
func setupInfluxQL(t *testing.T, db *sql.DB) *dbmeta.Meta {
	t.Helper()
	versions := influxRelease(t, db)
	m, err := dbmeta.New(dbmeta.InfluxQL, versions)
	if err != nil {
		t.Fatalf("building the meta: %v", err)
	}
	up, err := iqfixture.Everything.ResolveSetup(versions)
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

// iqRefused are the kinds that only InfluxDB 1 answers. InfluxDB 2 answers
// "not implemented" for SHOW USERS, SHOW GRANTS and SHOW DIAGNOSTICS, and
// InfluxDB 3 does not parse them.
var iqRefused = []string{dbmeta.Roles.Name(), dbmeta.Privileges.Name(), dbmeta.Settings.Name()}

// TestInfluxQLVersion checks that no statement reads the version, and that
// the release of GET /ping parses.
func TestInfluxQLVersion(t *testing.T) {
	db := openInfluxQL(t)
	versions, err := dbmeta.InfluxQL.Version(t.Context(), db)
	if err != nil {
		t.Fatalf("reading the version: %v", err)
	}
	if !versions.Main().Unknown {
		t.Errorf("expected an unknown version, because no InfluxQL statement names it, got %s", versions)
	}
	release := influxRelease(t, db)
	if major := release.Main().Parts[0]; major < 1 || major > 3 || !strings.HasPrefix(release.String(), "InfluxDB ") {
		t.Errorf("expected InfluxDB 1, 2 or 3, got %s", release)
	}
	t.Logf("server reports %s", release)
}

// TestInfluxQLScansEveryQuery reads every query through its own Scan. On
// InfluxDB 2 and 3, the server refuses the three kinds that only InfluxDB 1
// has, and the test checks that each one is refused.
func TestInfluxQLScansEveryQuery(t *testing.T) {
	db := openInfluxQL(t)
	m := setupInfluxQL(t, db)
	if influxMajor(m) == 1 {
		scanEveryQuery(t, m, db)
		return
	}
	all := dbmeta.Args{WithSystem: true}.Map()
	for name, drain := range map[string]func() (int, error){
		dbmeta.Schemas.Name():    scanOne(t, all, dbmeta.Schemas, m, db),
		dbmeta.Databases.Name():  scanOne(t, all, dbmeta.Databases, m, db),
		dbmeta.Tables.Name():     scanOne(t, all, dbmeta.Tables, m, db),
		dbmeta.Columns.Name():    scanOne(t, all, dbmeta.Columns, m, db),
		dbmeta.Roles.Name():      scanOne(t, all, dbmeta.Roles, m, db),
		dbmeta.Privileges.Name(): scanOne(t, all, dbmeta.Privileges, m, db),
		dbmeta.Settings.Name():   scanOne(t, all, dbmeta.Settings, m, db),
	} {
		n, err := drain()
		switch refused := slices.Contains(iqRefused, name); {
		case refused && err == nil:
			t.Errorf("%s: expected InfluxDB %d to refuse it, got %d rows", name, influxMajor(m), n)
		case refused:
			t.Logf("%-12s refused: %v", name, err)
		case err != nil:
			t.Errorf("%s: %v", name, err)
		default:
			t.Logf("%-12s %d rows", name, n)
		}
	}
	checkTypes(t, m, db, true)
	checkFold(t, m, db)
}

// TestInfluxQLFixtureObjects reads the fixture's measurements and their
// columns back, and on InfluxDB 1 the users, the grants and the settings.
func TestInfluxQLFixtureObjects(t *testing.T) {
	db := openInfluxQL(t)
	ctx := t.Context()
	m := setupInfluxQL(t, db)
	schema := iqfixture.Everything.Schema

	schemas := map[string]bool{}
	for v, err := range dbmeta.Schemas.All(ctx, m, db, nil) {
		if err != nil {
			t.Fatalf("reading schemas: %v", err)
		}
		schemas[v.Name] = true
	}
	if !schemas[schema] || schemas["_internal"] || schemas["_monitoring"] {
		t.Errorf("expected %s and no database InfluxDB keeps for itself, got %v", schema, schemas)
	}

	tables := map[string]string{}
	for v, err := range dbmeta.Tables.All(ctx, m, db, dbmeta.Args{Schema: schema}.Map()) {
		if err != nil {
			t.Fatalf("reading tables: %v", err)
		}
		tables[v.Name] = v.Type
	}
	for _, name := range []string{"author", "book", "region", "shipment"} {
		if tables[name] != "table" {
			t.Errorf("expected the table %s in %v", name, tables)
		}
	}

	var got []string
	for v, err := range dbmeta.Columns.All(ctx, m, db, dbmeta.Args{Schema: schema, Parent: "shipment"}.Map()) {
		if err != nil {
			t.Fatalf("reading columns: %v", err)
		}
		if v.Nullable == (v.Name == "time") {
			t.Errorf("%s: expected time alone NOT NULL, got nullable=%v", v.Name, v.Nullable)
		}
		got = append(got, v.Name+" "+v.DataType)
		if v.Ordinal != len(got) {
			t.Errorf("%s: expected the ordinal %d, got %d", v.Name, len(got), v.Ordinal)
		}
	}
	// The order of SELECT *, measured on 1.13.1 and 3.11.5.
	want := []string{
		"time timestamp", "amount integer", "area tag", "country tag",
		"fragile boolean", "shipment_id tag", "weight float",
	}
	if !slices.Equal(got, want) {
		t.Errorf("expected the columns of shipment\n%v\ngot\n%v", want, got)
	}

	if influxMajor(m) != 1 {
		for _, name := range iqRefused {
			t.Logf("%s: InfluxDB %d has no such statement", name, influxMajor(m))
		}
		return
	}
	roles := map[string]bool{}
	for v, err := range dbmeta.Roles.All(ctx, m, db, nil) {
		if err != nil {
			t.Fatalf("reading roles: %v", err)
		}
		roles[v.Name] = v.Superuser
	}
	if super, ok := roles[container.InfluxDBUser]; !ok || super || !roles["admin"] {
		t.Errorf("expected the administrator admin and the user %s, got %v", container.InfluxDBUser, roles)
	}
	var access string
	for v, err := range dbmeta.Privileges.All(ctx, m, db, dbmeta.Args{Schema: schema}.Map()) {
		if err != nil {
			t.Fatalf("reading privileges: %v", err)
		}
		access = v.Access.V
	}
	if want := container.InfluxDBUser + "=READ"; access != want {
		t.Errorf("expected the grant %s on %s, got %q", want, schema, access)
	}
	settings := map[string]string{}
	for v, err := range dbmeta.Settings.All(ctx, m, db, dbmeta.Args{Name: "data.%"}.Map()) {
		if err != nil {
			t.Fatalf("reading settings: %v", err)
		}
		settings[v.Name] = v.Value.V
	}
	if settings["data.dir"] != "/var/lib/influxdb/data" {
		t.Errorf("expected the setting data.dir, got %v", settings)
	}
}

// TestInfluxQLWalkStops checks that a walk ends when its caller stops, and
// that a query a walk answers has no one statement to build (D146).
func TestInfluxQLWalkStops(t *testing.T) {
	db := openInfluxQL(t)
	m := setupInfluxQL(t, db)
	if _, _, err := dbmeta.Columns.Build(m, nil); err == nil {
		t.Error("columns: expected no one statement to build")
	}
	n := 0
	for _, err := range dbmeta.Columns.All(t.Context(), m, db, nil) {
		if err != nil {
			t.Fatalf("reading columns: %v", err)
		}
		n++
		break
	}
	if n != 1 {
		t.Errorf("expected one column before the break, got %d", n)
	}
}

// makeInfluxQLUser connects as the ordinary user the dbrun entry makes.
//
// InfluxDB 2 refuses CREATE USER and GRANT, so the entry makes the user on
// both releases, and becoming it is a change to the credentials of the URL.
// On InfluxDB 1 the user can read dbmeta, and on InfluxDB 2 it can read the
// bucket dbmeta. InfluxDB 3 Core has no user with fewer rights than the
// administrator, so there is no principal to be (D152).
func makeInfluxQLUser(t *testing.T, scene *sql.DB, dsn, _ string) string {
	t.Helper()
	if major := influxRelease(t, scene).Main().Parts[0]; major > 2 {
		t.Skipf("InfluxDB %d has no user with fewer rights than the administrator", major)
	}
	u, err := url.Parse(influxQL(t, dsn))
	if err != nil {
		t.Fatalf("parsing %s: %v", dsn, err)
	}
	u.User = url.UserPassword(container.InfluxDBUser, container.Password)
	return u.String()
}
