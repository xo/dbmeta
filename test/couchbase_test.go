package test

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/xo/dbimp/couchbase"

	"github.com/xo/dbmeta"
	_ "github.com/xo/dbmeta/models/couchbase"
	cbfixture "github.com/xo/dbmeta/models/couchbase/fixture"
)

// openCouchbase returns a connection to the server named by DBMETA_COUCHBASE,
// which is the couchbase:// URL that github.com/xo/dbimp/couchbase takes.
func openCouchbase(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("DBMETA_COUCHBASE")
	if dsn == "" {
		t.Skip("set DBMETA_COUCHBASE to run against a real server")
	}
	db, err := sql.Open("couchbase", dsn)
	if err != nil {
		t.Fatalf("opening: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.PingContext(t.Context()); err != nil {
		t.Fatalf("connecting: %v", err)
	}
	return db
}

// cbFloor is the floor of the Couchbase model. Below it every query is too old,
// because 7.2 returns the columns of a result in name order. See D104.
var cbFloor = dbmeta.V(7, 6)

// setupCouchbase builds the fixture and returns the metadata for the server.
//
// A collection is not visible to the next statement for a moment after it is
// created, so a step that fails with "not found" is tried again for up to
// thirty seconds, rather than the fixture waiting a fixed time.
func setupCouchbase(t *testing.T, db *sql.DB) *dbmeta.Meta {
	t.Helper()
	ctx := t.Context()
	versions, err := dbmeta.Couchbase.Version(ctx, db)
	if err != nil {
		t.Fatalf("reading the version: %v", err)
	}
	down, err := cbfixture.Everything.ResolveTeardown(versions)
	if err != nil {
		t.Fatalf("resolving the teardown: %v", err)
	}
	drop := func(c context.Context) {
		for _, step := range down {
			if !step.Skipped {
				//nolint:errcheck // a teardown before setup is best effort
				db.ExecContext(c, step.Query)
			}
		}
	}
	drop(ctx)
	up, err := cbfixture.Everything.ResolveSetup(versions)
	if err != nil {
		t.Fatalf("resolving the setup: %v", err)
	}
	var ran, skipped int
	for _, step := range up {
		if step.Skipped {
			skipped++
			continue
		}
		if err := cbExec(ctx, db, step.Query); err != nil {
			t.Fatalf("setup %s: %v\n%s", step.Name, err, step.Query)
		}
		ran++
	}
	t.Cleanup(func() { drop(context.WithoutCancel(ctx)) })
	m, err := dbmeta.New(dbmeta.Couchbase, versions)
	if err != nil {
		t.Fatalf("building the metadata: %v", err)
	}
	t.Logf("server reports %s, fixture ran %d steps and skipped %d", versions, ran, skipped)
	return m
}

// cbExec runs one fixture statement, and tries it again while the keyspace
// it names is not visible yet.
func cbExec(ctx context.Context, db *sql.DB, query string) error {
	deadline := time.Now().Add(30 * time.Second)
	for {
		_, err := db.ExecContext(ctx, query)
		if err == nil || !strings.Contains(err.Error(), "not found") || time.Now().After(deadline) {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}
}

// skipBelowFloor skips a test on a release the model does not read.
func skipBelowFloor(t *testing.T, m *dbmeta.Meta) {
	t.Helper()
	if !m.Version().Main().AtLeast(cbFloor) {
		t.Skipf("the Couchbase model starts at 7.6, and the server is %s", m.Version())
	}
}

// TestCouchbaseVersion reads the version the way usql does and checks what
// the model makes of it.
func TestCouchbaseVersion(t *testing.T) {
	db := openCouchbase(t)
	versions, err := dbmeta.Couchbase.Version(t.Context(), db)
	if err != nil {
		t.Fatalf("reading the version: %v", err)
	}
	if main := versions.Main(); main.Unknown || main.Parts[0] < 7 {
		t.Errorf("expected release 7 or newer, got %s", main)
	}
	if !strings.HasPrefix(versions.String(), "Couchbase ") {
		t.Errorf("expected the display line to name Couchbase, got %q", versions)
	}
	t.Logf("server reports %s", versions)
}

// TestCouchbaseEveryQueryRuns runs every query the model answers and checks
// that the columns are the declared fields, by name and in order. Order is
// the fault that sets the floor at 7.6, so it is checked here rather than
// only counted.
func TestCouchbaseEveryQueryRuns(t *testing.T) {
	db := openCouchbase(t)
	m := setupCouchbase(t, db)
	var ran, tooOld int
	for _, q := range dbmeta.Queries() {
		switch q.Support(m) {
		case dbmeta.NotSupported, dbmeta.NotBuilt:
			continue
		case dbmeta.TooOld:
			tooOld++
			continue
		case dbmeta.Supported:
		}
		query, vals, err := q.Build(m, nil)
		if err != nil {
			t.Errorf("%s: building: %v", q.Name(), err)
			continue
		}
		cols, err := columnsOf(t, db, query, vals)
		if err != nil {
			t.Errorf("%s: %v\n%s", q.Name(), err, query)
			continue
		}
		fields, err := q.Fields(m)
		if err != nil {
			t.Fatalf("%s: reading fields: %v", q.Name(), err)
		}
		names := make([]string, len(fields))
		for i, f := range fields {
			names[i] = f.Name
		}
		if strings.Join(cols, ",") != strings.Join(names, ",") && len(cols) != 0 {
			t.Errorf("%s: returns %v and declares %v", q.Name(), cols, names)
		}
		ran++
	}
	t.Logf("%d queries ran, %d too old", ran, tooOld)
	if m.Version().Main().AtLeast(cbFloor) && tooOld != 0 {
		t.Errorf("%d queries are too old on %s, which is at the floor or above", tooOld, m.Version())
	}
	if !m.Version().Main().AtLeast(cbFloor) && ran != 0 {
		t.Errorf("%d queries ran on %s, which is below the floor", ran, m.Version())
	}
}

// TestCouchbaseScansEveryQuery reads every query the model answers through
// its Scan, row by row. The column check above does not reach Scan.
func TestCouchbaseScansEveryQuery(t *testing.T) {
	db := openCouchbase(t)
	m := setupCouchbase(t, db)
	skipBelowFloor(t, m)
	check := func(name string, n int, err error) {
		t.Helper()
		if err != nil {
			t.Errorf("%s: %v", name, err)
			return
		}
		t.Logf("%-18s %d rows", name, n)
	}
	n, err := drain(t, dbmeta.Databases, m, db)
	check("databases", n, err)
	n, err = drain(t, dbmeta.Schemas, m, db)
	check("schemas", n, err)
	n, err = drain(t, dbmeta.Tables, m, db)
	check("tables", n, err)
	n, err = drain(t, dbmeta.Indexes, m, db)
	check("indexes", n, err)
	n, err = drain(t, dbmeta.IndexColumns, m, db)
	check("index columns", n, err)
	n, err = drain(t, dbmeta.Functions, m, db)
	check("functions", n, err)
	n, err = drain(t, dbmeta.RoutineParameters, m, db)
	check("routine parameters", n, err)
	n, err = drain(t, dbmeta.Sequences, m, db)
	check("sequences", n, err)
	n, err = drain(t, dbmeta.Roles, m, db)
	check("roles", n, err)
	n, err = drain(t, dbmeta.RoleGrants, m, db)
	check("role grants", n, err)
	n, err = drain(t, dbmeta.Privileges, m, db)
	check("privileges", n, err)
	n, err = drain(t, dbmeta.CurrentUser, m, db)
	check("current user", n, err)
	var answered int
	for _, q := range dbmeta.Queries() {
		if q.Support(m) == dbmeta.Supported {
			answered++
		}
	}
	if answered != 12 {
		t.Errorf("the model answers %d queries and this test reads 12: add the new one here", answered)
	}
}

// TestCouchbaseFixtureObjects reads the fixture back through the typed API.
func TestCouchbaseFixtureObjects(t *testing.T) {
	db := openCouchbase(t)
	m := setupCouchbase(t, db)
	skipBelowFloor(t, m)
	ctx := t.Context()
	fx := cbfixture.Everything
	args := dbmeta.Args{Schema: fx.Schema}.Map()

	var tables []string
	for v, err := range dbmeta.Tables.All(ctx, m, db, args) {
		if err != nil {
			t.Fatalf("reading tables: %v", err)
		}
		if v.Catalog != fx.Catalog {
			t.Errorf("table %s: catalog %q, want the bucket %q", v.Name, v.Catalog, fx.Catalog)
		}
		tables = append(tables, v.Name)
	}
	if strings.Join(tables, ",") != "author,book,region,shipment" {
		t.Errorf("tables: got %v", tables)
	}

	keys := map[string][]dbmeta.IndexColumn{}
	for v, err := range dbmeta.IndexColumns.All(ctx, m, db, args) {
		if err != nil {
			t.Fatalf("reading index columns: %v", err)
		}
		keys[v.Index] = append(keys[v.Index], v)
	}
	if k := keys["book_author"]; len(k) != 2 || k[0].Name.V != "author_id" || k[1].Name.V != "title" ||
		k[0].Descending || !k[1].Descending {
		t.Errorf("book_author: expected author_id, then title DESC, got %+v", k)
	}
	if k := keys["book_tags"]; len(k) != 1 || k[0].Name.Valid || !k[0].Expression.Valid {
		t.Errorf("book_tags: expected one expression key, got %+v", k)
	}
	if k := keys["book_published"]; len(k) != 1 || k[0].Name.V != "published" {
		t.Errorf("book_published: expected the key published, got %+v", k)
	}

	params := map[string][]string{}
	for v, err := range dbmeta.RoutineParameters.All(ctx, m, db, args) {
		if err != nil {
			t.Fatalf("reading routine parameters: %v", err)
		}
		params[v.Routine] = append(params[v.Routine], v.Name.V)
	}
	if got := strings.Join(params["full_title"], ","); got != "title,subtitle" {
		t.Errorf("full_title: parameters %q", got)
	}
	if got := strings.Join(params["total"], ","); got != "..." {
		t.Errorf("total: parameters %q, want the variadic ...", got)
	}

	var seq bool
	for v, err := range dbmeta.Sequences.All(ctx, m, db, args) {
		if err != nil {
			t.Fatalf("reading sequences: %v", err)
		}
		if v.Name == "book_seq" {
			seq = v.Increment.Valid && v.Increment.V == "1" && v.Maximum.Valid
		}
	}
	if !seq {
		t.Error("the sequence book_seq is missing or has no increment and maximum")
	}

	var grant bool
	for v, err := range dbmeta.Privileges.All(ctx, m, db, dbmeta.Args{Schema: fx.Catalog}.Map()) {
		if err != nil {
			t.Fatalf("reading privileges: %v", err)
		}
		if v.Type == "collection" && v.Name == fx.Schema+".book" &&
			strings.Contains(v.Access.V, "dbmeta_user=select") {
			grant = true
		}
	}
	if !grant {
		t.Error("the grant on dbmeta_fixture.book is missing")
	}

	user, ok, err := dbmeta.First(dbmeta.CurrentUser.All(ctx, m, db, nil))
	if err != nil || !ok || user.Name != "Administrator" {
		t.Errorf("current user: got %+v, %v, %v", user, ok, err)
	}
}

// TestCouchbaseTooOldBelowTheFloor checks that 7.2 is refused as too old
// rather than answered wrongly, because it sends the columns of a result in
// name order.
func TestCouchbaseTooOldBelowTheFloor(t *testing.T) {
	db := openCouchbase(t)
	versions, err := dbmeta.Couchbase.Version(t.Context(), db)
	if err != nil {
		t.Fatalf("reading the version: %v", err)
	}
	if versions.Main().AtLeast(cbFloor) {
		t.Skipf("the server is %s, at the floor or above", versions)
	}
	m, err := dbmeta.New(dbmeta.Couchbase, versions)
	if err != nil {
		t.Fatal(err)
	}
	for v, err := range dbmeta.Tables.All(t.Context(), m, db, nil) {
		if !errors.Is(err, dbmeta.ErrVersionTooOld) {
			t.Fatalf("expected ErrVersionTooOld, got a row %+v and %v", v, err)
		}
	}
	_, _, err = dbmeta.Tables.Build(m, nil)
	if !errors.Is(err, dbmeta.ErrVersionTooOld) {
		t.Errorf("expected ErrVersionTooOld, got %v", err)
	}
}
