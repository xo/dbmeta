package test

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"

	_ "github.com/ydb-platform/ydb-go-sdk/v3"

	"github.com/xo/dbmeta"
	_ "github.com/xo/dbmeta/models/ydb"
	ydfixture "github.com/xo/dbmeta/models/ydb/fixture"
)

// openYDB returns a connection to the server named by DBMETA_YDB.
//
// The connection string is the one dburl's GenYDB writes for a ydb:// URL,
// grpc://user:password@host:port/database, and the driver is the one
// ydb-go-sdk registers as ydb, so the test opens the server the way a
// consumer does. go_balancer=disable keeps the address the driver was
// given, because the server answers with an address inside the container.
func openYDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("DBMETA_YDB")
	if dsn == "" {
		t.Skip("set DBMETA_YDB to run against a real server")
	}
	return openAt(t, "ydb", dsn)
}

// setupYDB builds the fixture and returns the metadata for the server.
//
// It tears down first, because a run that failed part way leaves the tables
// behind and CREATE TABLE then fails rather than the test reporting what
// actually went wrong.
func setupYDB(t *testing.T, db *sql.DB) *dbmeta.Meta {
	t.Helper()
	ctx := t.Context()
	versions, err := dbmeta.YDB.Version(ctx, db)
	if err != nil {
		t.Fatalf("reading the version: %v", err)
	}

	down, err := ydfixture.Everything.ResolveTeardown(versions)
	if err != nil {
		t.Fatalf("resolving the teardown: %v", err)
	}
	for _, step := range down {
		if !step.Skipped {
			//nolint:errcheck // a teardown before setup is best effort
			db.ExecContext(ctx, step.Query)
		}
	}

	up, err := ydfixture.Everything.ResolveSetup(versions)
	if err != nil {
		t.Fatalf("resolving the setup: %v", err)
	}
	var ran, skipped int
	for _, step := range up {
		if step.Skipped {
			skipped++
			continue
		}
		if _, err := db.ExecContext(ctx, step.Query); err != nil {
			t.Fatalf("setup %s: %v\n%s", step.Name, err, step.Query)
		}
		ran++
	}
	t.Cleanup(func() {
		for _, step := range down {
			if !step.Skipped {
				//nolint:errcheck // the test has already reported what matters
				db.ExecContext(context.WithoutCancel(ctx), step.Query)
			}
		}
	})

	m, err := dbmeta.New(dbmeta.YDB, versions)
	if err != nil {
		t.Fatalf("building the metadata: %v", err)
	}
	t.Logf("server reports %s, fixture ran %d steps and skipped %d", versions, ran, skipped)
	return m
}

// ydbArgs is the filter the fixture's objects sit behind.
func ydbArgs() map[string]any {
	return dbmeta.Args{Schema: ydfixture.Everything.Schema}.Map()
}

// TestYDBVersion reads the version and checks what the model makes of it.
func TestYDBVersion(t *testing.T) {
	db := openYDB(t)
	versions, err := dbmeta.YDB.Version(t.Context(), db)
	if err != nil {
		t.Fatalf("reading the version: %v", err)
	}
	main := versions.Main()
	if main.Unknown || len(main.Parts) != 4 {
		t.Fatalf("expected a version of four numbers, got %s", versions)
	}
	// 26.2 is the floor, which is the older of the two lines dbrun starts.
	if main.Compare(dbmeta.V(26, 2)) < 0 {
		t.Errorf("expected release 26.2 or newer, got %s", main)
	}
	if display := versions.String(); !strings.HasPrefix(display, "YDB ") {
		t.Errorf("expected the product in %q", display)
	}
	t.Logf("server reports %s", versions)
}

// TestYDBSmoke runs each registered query against the fixture and checks
// that it returns the columns it declares.
func TestYDBSmoke(t *testing.T) {
	db := openYDB(t)
	m := setupYDB(t, db)

	var ran, unsupported int
	for _, q := range dbmeta.Queries() {
		switch q.Support(m) {
		case dbmeta.NotBuilt, dbmeta.NotSupported, dbmeta.TooOld:
			unsupported++
			continue
		case dbmeta.Supported:
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
			t.Errorf("%s: declares %d fields and returns %d columns",
				q.Name(), len(fields), len(cs))
		}
		for i, c := range cs {
			if i < len(fields) && c != fields[i].Name {
				t.Errorf("%s: column %d is %q and the field is %q", q.Name(), i, c, fields[i].Name)
			}
		}
		ran++
	}
	if ran != 7 {
		t.Errorf("expected the 7 queries the package comment names to run, %d ran", ran)
	}
	t.Logf("%d queries ran, %d not supported", ran, unsupported)
}

// TestYDBFixtureObjects reads the fixture back through the typed API, and
// checks each object the model leaves out on purpose, so that an absence
// stays a decision rather than becoming a fault. See D161.
func TestYDBFixtureObjects(t *testing.T) {
	db := openYDB(t)
	ctx := t.Context()
	m := setupYDB(t, db)
	schema := ydfixture.Everything.Schema

	// Tables holds the row tables and the column table, both as a table, and
	// neither the view nor the tables that implement an index.
	tables := map[string]string{}
	for v, err := range dbmeta.Tables.All(ctx, m, db, ydbArgs()) {
		if err != nil {
			t.Fatalf("reading tables: %v", err)
		}
		if v.Schema == schema {
			tables[v.Name] = v.Type
		}
		if v.Catalog != "/local" {
			t.Errorf("table %s: expected the catalog /local, got %q", v.Name, v.Catalog)
		}
	}
	want := map[string]string{
		"author": "table", "book": "table", "region": "table", "shipment": "table",
		"ledger": "table",
	}
	for name, typ := range want {
		if tables[name] != typ {
			t.Errorf("table %s: expected type %q, got %q", name, typ, tables[name])
		}
	}
	if len(tables) != len(want) {
		t.Errorf("expected exactly the tables %v, got %v", want, tables)
	}

	// Schemas holds the directories that hold something, and not the empty
	// one, which nothing tells from a view.
	schemas := map[string]string{}
	for v, err := range dbmeta.Schemas.All(ctx, m, db, nil) {
		if err != nil {
			t.Fatalf("reading schemas: %v", err)
		}
		schemas[v.Name] = v.Owner
	}
	for _, name := range []string{"dbmeta", schema} {
		if _, ok := schemas[name]; !ok {
			t.Errorf("expected the directory %s in %v", name, schemas)
		}
	}
	for _, name := range []string{schema + "/empty", ".sys", ".metadata"} {
		if _, ok := schemas[name]; ok {
			t.Errorf("expected no schema %s, because D161 leaves it out", name)
		}
	}

	// Privileges types every path it can and calls the rest an object.
	privileges := map[string]dbmeta.Privilege{}
	for v, err := range dbmeta.Privileges.All(ctx, m, db, ydbArgs()) {
		if err != nil {
			t.Fatalf("reading privileges: %v", err)
		}
		if v.Schema.V == schema {
			privileges[v.Name] = v
		}
	}
	for name, typ := range map[string]string{
		"author": "table", "ledger": "table", "recent": "object",
		"events": "object", "empty": "object",
	} {
		if got := privileges[name].Type; got != typ {
			t.Errorf("privilege on %s: expected type %q, got %q", name, typ, got)
		}
	}
	if got := privileges["author"].Access.V; !strings.Contains(got, ydfixture.Readers+"=ydb.generic.read") {
		t.Errorf("privilege on author: expected the grant to %s, got %q", ydfixture.Readers, got)
	}
	if got := privileges["book"].Access; got.Valid {
		t.Errorf("privilege on book: expected no explicit grant, got %q", got.V)
	}

	// Roles and role grants hold the user, the group and the membership.
	roles := map[string]dbmeta.Role{}
	for v, err := range dbmeta.Roles.All(ctx, m, db, nil) {
		if err != nil {
			t.Fatalf("reading roles: %v", err)
		}
		roles[v.Name] = v
	}
	if r, ok := roles[ydfixture.Grantee]; !ok || !r.CanLogin || !strings.Contains(r.MemberOf, ydfixture.Readers) {
		t.Errorf("expected the user %s, who can log in and is in %s, got %+v",
			ydfixture.Grantee, ydfixture.Readers, r)
	}
	if r, ok := roles[ydfixture.Readers]; !ok || r.CanLogin {
		t.Errorf("expected the group %s, which cannot log in, got %+v", ydfixture.Readers, r)
	}
	var member bool
	for v, err := range dbmeta.RoleGrants.All(ctx, m, db, dbmeta.Args{Name: ydfixture.Readers}.Map()) {
		if err != nil {
			t.Fatalf("reading role grants: %v", err)
		}
		member = member || v.Role == ydfixture.Grantee
	}
	if !member {
		t.Errorf("expected %s to belong to %s", ydfixture.Grantee, ydfixture.Readers)
	}

	// One database, and at least one storage pool.
	database, ok, err := dbmeta.First(dbmeta.Databases.All(ctx, m, db, nil))
	if err != nil || !ok || database.Name != "/local" {
		t.Errorf("expected the database /local, got %+v, %v, %v", database, ok, err)
	}
	pool, ok, err := dbmeta.First(dbmeta.Tablespaces.All(ctx, m, db, nil))
	if err != nil || !ok || !strings.HasPrefix(pool.Options.V, "kind=") {
		t.Errorf("expected a storage pool with its kind, got %+v, %v, %v", pool, ok, err)
	}
}

// TestYDBRefusesAnOrdinaryUser checks the fact that parity records for
// every query: the .sys views refuse a user that is not an administrator,
// although the user can read and describe the directory dbmeta.
func TestYDBRefusesAnOrdinaryUser(t *testing.T) {
	admin := openYDB(t)
	m := setupYDB(t, admin)
	db := openAt(t, "ydb", makeYDBUser(t, admin, dsnOf(t, "DBMETA_YDB"), ""))
	for _, err := range dbmeta.Tables.All(t.Context(), m, db, ydbArgs()) {
		if err == nil {
			t.Fatal("expected the ordinary user to be refused .sys/partition_stats")
		}
		t.Logf("refused: %.120s", err)
		return
	}
	t.Fatal("expected the ordinary user to be refused, and no error came")
}
