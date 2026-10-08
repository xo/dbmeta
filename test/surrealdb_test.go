package test

import (
	"context"
	"database/sql"
	"errors"
	"net/url"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/xo/dbimp/surrealdb"

	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/container"
	_ "github.com/xo/dbmeta/models/surrealdb"
	srfixture "github.com/xo/dbmeta/models/surrealdb/fixture"
)

// openSurrealDB returns a connection to the server named by DBMETA_SURREALDB,
// which is the surrealdb:// URL that github.com/xo/dbimp/surrealdb takes. Its
// path names the namespace dbmeta and the database dbmeta, which every
// statement of the model reads.
func openSurrealDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("DBMETA_SURREALDB")
	if dsn == "" {
		t.Skip("set DBMETA_SURREALDB to run against a real server")
	}
	db, err := sql.Open(surrealdb.Name, dsn)
	if err != nil {
		t.Fatalf("opening: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.PingContext(t.Context()); err != nil {
		t.Fatalf("connecting: %v", err)
	}
	return db
}

// srINFO is the release from which INFO is a value that a SELECT reads, which
// every kind but the current schema needs.
var srINFO = dbmeta.V(3)

// srRelease reads the release through the RPC method version, the way usql
// does, and parses it with the model. The model reads it with SELECT
// version(), and TestSurrealDBVersion checks that the two agree.
func srRelease(t *testing.T, db *sql.DB) dbmeta.VersionSet {
	t.Helper()
	ctx := t.Context()
	conn, err := db.Conn(ctx)
	if err != nil {
		t.Fatalf("taking a connection: %v", err)
	}
	var raw string
	err = conn.Raw(func(dc any) error {
		raw, err = surrealdb.Version(ctx, dc)
		return err
	})
	if cerr := conn.Close(); cerr != nil {
		t.Errorf("returning the connection: %v", cerr)
	}
	if err != nil {
		t.Fatalf("reading the version through the driver: %v", err)
	}
	versions, err := dbmeta.SurrealDB.ParseVersion([]string{raw})
	if err != nil {
		t.Fatalf("parsing %q: %v", raw, err)
	}
	return versions
}

// setupSurrealDB builds the fixture and returns the metadata for the server.
// The metadata takes the release that SELECT version() reads.
func setupSurrealDB(t *testing.T, db *sql.DB) *dbmeta.Meta {
	t.Helper()
	ctx := t.Context()
	versions, err := dbmeta.SurrealDB.Version(ctx, db)
	if err != nil {
		t.Fatalf("reading the version: %v", err)
	}
	down, err := srfixture.Everything.ResolveTeardown(versions)
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
	up, err := srfixture.Everything.ResolveSetup(versions)
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
	t.Cleanup(func() { drop(context.WithoutCancel(ctx)) })
	m, err := dbmeta.New(dbmeta.SurrealDB, versions)
	if err != nil {
		t.Fatalf("building the metadata: %v", err)
	}
	t.Logf("server reports %s, fixture ran %d steps and skipped %d", versions, ran, skipped)
	return m
}

// TestSurrealDBVersion checks that SELECT version() gives the full release,
// and that it is the release that the RPC method gives, which usql reads.
func TestSurrealDBVersion(t *testing.T) {
	db := openSurrealDB(t)
	got, err := dbmeta.SurrealDB.Version(t.Context(), db)
	if err != nil {
		t.Fatalf("reading the version: %v", err)
	}
	if main := got.Main(); main.Unknown || len(main.Parts) < 3 {
		t.Errorf("expected the full release, got %s", main)
	}
	if !strings.HasPrefix(got.String(), "SurrealDB ") {
		t.Errorf("expected the display line to name SurrealDB, got %q", got)
	}
	if rpc := srRelease(t, db); rpc.String() != got.String() {
		t.Errorf("SELECT version() says %s and the RPC method says %s", got, rpc)
	}
	t.Logf("the server reports %s", got)
}

// TestSurrealDBVersionForAnOrdinaryUser checks that the user of the entry
// reads the same release as the administrator.
func TestSurrealDBVersionForAnOrdinaryUser(t *testing.T) {
	dsn := os.Getenv("DBMETA_SURREALDB")
	if dsn == "" {
		t.Skip("set DBMETA_SURREALDB to run against a real server")
	}
	admin, err := dbmeta.SurrealDB.Version(t.Context(), openSurrealDB(t))
	if err != nil {
		t.Fatalf("reading the version as the administrator: %v", err)
	}
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("parsing %s: %v", dsn, err)
	}
	u.User = url.UserPassword(container.SurrealDBUser, container.Password)
	u.RawQuery = url.Values{"auth": {"database"}}.Encode()
	t.Setenv("DBMETA_SURREALDB", u.String())
	got, err := dbmeta.SurrealDB.Version(t.Context(), openSurrealDB(t))
	if err != nil {
		t.Fatalf("reading the version as the ordinary user on %s: %v", admin, err)
	}
	if got.String() != admin.String() {
		t.Errorf("expected the ordinary user to read %s, got %s", admin, got)
	}
}

// TestSurrealDBEveryQueryRuns runs every query the model answers and checks
// that the columns are the declared fields, by name and in order, and that a
// 2.x server is too old for every kind but the current schema.
func TestSurrealDBEveryQueryRuns(t *testing.T) {
	db := openSurrealDB(t)
	m := setupSurrealDB(t, db)
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
		// An empty result has no row to take the columns from.
		if len(cols) != 0 && strings.Join(cols, ",") != strings.Join(names, ",") {
			t.Errorf("%s: returns %v and declares %v", q.Name(), cols, names)
		}
		ran++
	}
	t.Logf("%d queries ran, %d too old", ran, tooOld)
	want := 18
	if !m.Version().Main().AtLeast(srINFO) {
		want = 1
	}
	if ran != want {
		t.Errorf("%d queries ran on %s, want %d", ran, m.Version(), want)
	}
}

// TestSurrealDBScansEveryQuery reads every query the model answers through
// its Scan, row by row.
func TestSurrealDBScansEveryQuery(t *testing.T) {
	db := openSurrealDB(t)
	m := setupSurrealDB(t, db)
	scanEveryQuery(t, m, db)
}

// TestSurrealDBTooOld checks that 2.x reports a kind that needs INFO as a
// value as too old, rather than sending a statement it cannot parse.
func TestSurrealDBTooOld(t *testing.T) {
	db := openSurrealDB(t)
	versions, err := dbmeta.SurrealDB.Version(t.Context(), db)
	if err != nil {
		t.Fatalf("reading the version: %v", err)
	}
	if versions.Main().AtLeast(srINFO) {
		t.Skipf("the server is %s, which reads INFO as a value", versions)
	}
	m, err := dbmeta.New(dbmeta.SurrealDB, versions)
	if err != nil {
		t.Fatal(err)
	}
	for v, err := range dbmeta.Tables.All(t.Context(), m, db, nil) {
		if !errors.Is(err, dbmeta.ErrVersionTooOld) {
			t.Fatalf("expected ErrVersionTooOld, got a row %+v and %v", v, err)
		}
	}
	schema, ok, err := dbmeta.First(dbmeta.CurrentSchema.All(t.Context(), m, db, nil))
	if err != nil || !ok || schema.Catalog != "dbmeta" || schema.Name != "dbmeta" {
		t.Errorf("current schema: got %+v, %v, %v", schema, ok, err)
	}
}

// TestSurrealDBCurrentUserIsNotSupported checks that the current user stays
// unanswered. No function names the system user a session signed in as, and
// $auth is NONE for one (D164).
func TestSurrealDBCurrentUserIsNotSupported(t *testing.T) {
	m, err := dbmeta.New(dbmeta.SurrealDB, dbmeta.VersionSet{})
	if err != nil {
		t.Fatal(err)
	}
	if s := dbmeta.CurrentUser.Support(m); s != dbmeta.NotSupported {
		t.Errorf("current user is %v, want not supported", s)
	}
}

// TestSurrealDBFixtureObjects reads the fixture back through the typed API.
func TestSurrealDBFixtureObjects(t *testing.T) {
	db := openSurrealDB(t)
	m := setupSurrealDB(t, db)
	if !m.Version().Main().AtLeast(srINFO) {
		t.Skipf("the server is %s, which answers the current schema alone", m.Version())
	}
	ctx := t.Context()
	fx := srfixture.Everything
	args := dbmeta.Args{Schema: fx.Schema}.Map()

	tables := map[string]string{}
	for v, err := range dbmeta.Tables.All(ctx, m, db, args) {
		if err != nil {
			t.Fatalf("reading tables: %v", err)
		}
		if v.Catalog != fx.Catalog || v.Schema != fx.Schema {
			t.Errorf("table %s: catalog %q and schema %q, want %q and %q", v.Name, v.Catalog, v.Schema, fx.Catalog, fx.Schema)
		}
		tables[v.Name] = v.Type
	}
	for name, typ := range map[string]string{
		"author": "table", "book": "table", "region": "table", "shipment": "table",
		"recent": "view", "wrote": "relation",
	} {
		if tables[name] != typ {
			t.Errorf("table %s: type %q, want %q", name, tables[name], typ)
		}
	}

	cols := map[string]dbmeta.Column{}
	for v, err := range dbmeta.Columns.All(ctx, m, db, dbmeta.Args{Parent: "author"}.Map()) {
		if err != nil {
			t.Fatalf("reading columns: %v", err)
		}
		cols[v.Name] = v
	}
	if c := cols["name"]; c.DataType != "string" || c.Nullable {
		t.Errorf("author.name: got %+v, want a string that is not null", c)
	}
	if c := cols["bio"]; !c.Nullable {
		t.Errorf("author.bio: got %+v, want a nullable field", c)
	}
	if c := cols["rating"]; c.Default.V != "3" || c.Comment.V != "out of five" {
		t.Errorf("author.rating: got %+v, want the default 3 and a comment", c)
	}
	if c := cols["changed"]; c.Generated.V != "s" || c.DataType != "any" || !c.Nullable {
		t.Errorf("author.changed: got %+v, want a stored field of any type", c)
	}
	if c := cols["shout"]; c.Generated.V != "v" {
		t.Errorf("author.shout: got %+v, want a computed field", c)
	}

	keys := map[string][]string{}
	for v, err := range dbmeta.IndexColumns.All(ctx, m, db, args) {
		if err != nil {
			t.Fatalf("reading index columns: %v", err)
		}
		keys[v.Index] = append(keys[v.Index], v.Name.V)
	}
	if got := strings.Join(keys["book_author"], ","); got != "author_id,title" {
		t.Errorf("book_author: fields %q", got)
	}

	constraints := map[string]dbmeta.Constraint{}
	for v, err := range dbmeta.Constraints.All(ctx, m, db, args) {
		if err != nil {
			t.Fatalf("reading constraints: %v", err)
		}
		constraints[v.Table+"."+v.Name] = v
	}
	if c := constraints["book.title"]; c.Type != "check" || c.Definition.V != "string::len($value) > 0" {
		t.Errorf("the check on book.title: got %+v", c)
	}
	if c := constraints["region.region_key"]; c.Type != "unique" {
		t.Errorf("the unique index region_key: got %+v", c)
	}

	var trigger bool
	for v, err := range dbmeta.Triggers.All(ctx, m, db, args) {
		if err != nil {
			t.Fatalf("reading triggers: %v", err)
		}
		if v.Name == "book_created" && v.Table == "book" && strings.HasPrefix(v.Definition, "DEFINE EVENT book_created") {
			trigger = true
		}
	}
	if !trigger {
		t.Error("the event book_created is missing or has no definition")
	}

	params := map[string][]string{}
	for v, err := range dbmeta.RoutineParameters.All(ctx, m, db, args) {
		if err != nil {
			t.Fatalf("reading routine parameters: %v", err)
		}
		params[v.Routine] = append(params[v.Routine], v.Name.V+" "+v.DataType)
	}
	if got := strings.Join(params["full_title"], ","); got != "title string,subtitle string" {
		t.Errorf("full_title: parameters %q", got)
	}

	seq, ok, err := dbmeta.First(dbmeta.Sequences.All(ctx, m, db, args))
	if err != nil || !ok || seq.Name != "book_seq" || seq.Start.V != "100" {
		t.Errorf("sequence: got %+v, %v, %v", seq, ok, err)
	}

	var editor bool
	for v, err := range dbmeta.RoleGrants.All(ctx, m, db, nil) {
		if err != nil {
			t.Fatalf("reading role grants: %v", err)
		}
		if v.Role == "dbmeta_user" && v.MemberOf == "EDITOR" {
			editor = true
		}
	}
	if !editor {
		t.Error("the role EDITOR of dbmeta_user is missing")
	}

	priv, ok, err := dbmeta.First(dbmeta.Privileges.All(ctx, m, db, dbmeta.Args{Name: "region"}.Map()))
	if err != nil || !ok || priv.Access.V != "select=FULL, create=WHERE $auth.admin = true, update=NONE, delete=NONE" ||
		!strings.Contains(priv.ColumnAccess.V, "area: select=FULL, create=WHERE $auth.admin = true") {
		t.Errorf("privileges of region: got %+v, %v, %v", priv, ok, err)
	}

	comments := map[string]string{}
	for v, err := range dbmeta.Comments.All(ctx, m, db, args) {
		if err != nil {
			t.Fatalf("reading comments: %v", err)
		}
		comments[v.Type+" "+v.Name] = v.Comment
	}
	for key, want := range map[string]string{
		"table author":         "people who write books",
		"column author.rating": "out of five",
		"index book_title":     "one book for each title",
		"trigger book_created": "logs a new book",
		"function full_title":  "joins two titles",
		"param max_rating":     "the best rating",
	} {
		if comments[key] != want {
			t.Errorf("comment on %s: got %q, want %q", key, comments[key], want)
		}
	}
}

// TestSurrealDBPatterns checks that a pattern matches what dbmeta.Like says it
// matches, for the characters a regular expression gives a meaning to.
func TestSurrealDBPatterns(t *testing.T) {
	db := openSurrealDB(t)
	m := setupSurrealDB(t, db)
	if !m.Version().Main().AtLeast(srINFO) {
		t.Skipf("the server is %s, which answers the current schema alone", m.Version())
	}
	names := func(pattern string) []string {
		var out []string
		for v, err := range dbmeta.Columns.All(t.Context(), m, db, dbmeta.Args{Name: pattern}.Map()) {
			if err != nil {
				t.Fatalf("reading columns named %q: %v", pattern, err)
			}
			out = append(out, v.Table+"."+v.Name)
		}
		slices.Sort(out)
		return out
	}
	every := names("")
	for _, pattern := range []string{
		"name", "a%", "%_id", "book\\_id", "b_o", "t%s", "%", "tags.*", "tags._", "tags\\.\\*",
		".", ".%", "name\\", "\\%", "(name)", "[a-z]%", "a b", "#", "<", "$", "^%",
	} {
		var want []string
		for _, n := range every {
			_, name, _ := strings.Cut(n, ".")
			if dbmeta.Like(pattern, name) {
				want = append(want, n)
			}
		}
		if got := names(pattern); !slices.Equal(got, want) {
			t.Errorf("pattern %q: got %v, want %v", pattern, got, want)
		}
	}
}

// TestSurrealDBSchemasIsTheConnectedDatabase holds D168. Every kind below a
// schema reads only the database of the connection, so Schemas lists that
// one alone, even when the namespace holds another.
func TestSurrealDBSchemasIsTheConnectedDatabase(t *testing.T) {
	db := openSurrealDB(t)
	m := setupSurrealDB(t, db)
	if !m.Version().Main().AtLeast(srINFO) {
		t.Skipf("the server is %s, which answers the current schema alone", m.Version())
	}
	ctx := t.Context()
	const other = "dbmeta_other"
	if _, err := db.ExecContext(ctx, "DEFINE DATABASE IF NOT EXISTS "+other); err != nil {
		t.Fatalf("making the database %s: %v", other, err)
	}
	t.Cleanup(func() {
		//nolint:errcheck // the test has already reported what matters
		db.ExecContext(context.WithoutCancel(ctx), "REMOVE DATABASE IF EXISTS "+other)
	})
	var names []string
	for v, err := range dbmeta.Schemas.All(ctx, m, db, nil) {
		if err != nil {
			t.Fatalf("reading schemas: %v", err)
		}
		names = append(names, v.Name)
	}
	if want := srfixture.Everything.Schema; len(names) != 1 || names[0] != want {
		t.Errorf("schemas: got %v, want only %s", names, want)
	}
}
