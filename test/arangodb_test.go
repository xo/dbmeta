package test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"

	_ "github.com/xo/dbimp/arangodb"

	"github.com/xo/dbmeta"
	_ "github.com/xo/dbmeta/models/arangodb"
	arfixture "github.com/xo/dbmeta/models/arangodb/fixture"
)

// openArangoDB returns a connection to the server named by DBMETA_ARANGODB,
// which is the arangodb:// URL that github.com/xo/dbimp/arangodb takes, and
// which dburl's arangodb scheme opens (D154).
func openArangoDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("DBMETA_ARANGODB")
	if dsn == "" {
		t.Skip("set DBMETA_ARANGODB to run against a real server")
	}
	db, err := sql.Open("arangodb", dsn)
	if err != nil {
		t.Fatalf("opening: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.PingContext(t.Context()); err != nil {
		t.Fatalf("connecting: %v", err)
	}
	return db
}

// arangoAPI sends one request to the HTTP API of a database, as the user dsn
// names. The database is the one dsn names when database is empty. AQL has
// no DDL, and the driver makes only collections and indexes, so the fixture
// makes everything else this way. A status of 400 or above is an error.
func arangoAPI(ctx context.Context, dsn, database string, r *arfixture.Request) error {
	u, err := url.Parse(dsn)
	if err != nil {
		return fmt.Errorf("parsing the dsn: %w", err)
	}
	if database == "" {
		database = strings.TrimPrefix(u.Path, "/")
	}
	base := url.URL{Scheme: "http", Host: u.Host, Path: "/_db/" + database}
	if u.Query().Get("tls") == "true" {
		base.Scheme = "https"
	}
	path, query, _ := strings.Cut(r.Path, "?")
	base.Path += path
	base.RawQuery = query
	req, err := http.NewRequestWithContext(ctx, r.Method, base.String(), strings.NewReader(r.Body))
	if err != nil {
		return fmt.Errorf("building %s %s: %w", r.Method, r.Path, err)
	}
	password, _ := u.User.Password()
	req.SetBasicAuth(u.User.Username(), password)
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("sending %s %s: %w", r.Method, r.Path, err)
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("reading the answer to %s %s: %w", r.Method, r.Path, err)
	}
	if res.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("%s %s answered %d: %s", r.Method, r.Path, res.StatusCode, body)
	}
	return nil
}

// setupArangoDB builds the fixture as the administrator and returns the
// metadata for the server.
func setupArangoDB(t *testing.T, db *sql.DB) *dbmeta.Meta {
	t.Helper()
	ctx := t.Context()
	dsn := os.Getenv("DBMETA_ARANGODB")
	versions, err := dbmeta.ArangoDB.Version(ctx, db)
	if err != nil {
		t.Fatalf("reading the version: %v", err)
	}
	down, err := arfixture.Everything.ResolveTeardown(versions)
	if err != nil {
		t.Fatalf("resolving the teardown: %v", err)
	}
	drop := func(c context.Context) {
		for _, step := range down {
			switch {
			case step.Skipped:
			case step.Request != nil:
				//nolint:errcheck // a teardown before setup is best effort
				arangoAPI(c, dsn, "", step.Request)
			default:
				//nolint:errcheck // a teardown before setup is best effort
				db.ExecContext(c, step.Query)
			}
		}
	}
	drop(ctx)
	up, err := arfixture.Everything.ResolveSetup(versions)
	if err != nil {
		t.Fatalf("resolving the setup: %v", err)
	}
	var ran, skipped int
	for _, step := range up {
		switch {
		case step.Skipped:
			skipped++
			continue
		case step.Request != nil:
			err = arangoAPI(ctx, dsn, "", step.Request)
		default:
			_, err = db.ExecContext(ctx, step.Query)
		}
		if err != nil {
			t.Fatalf("setup %s: %v", step.Name, err)
		}
		ran++
	}
	t.Cleanup(func() { drop(context.WithoutCancel(ctx)) })
	m, err := dbmeta.New(dbmeta.ArangoDB, versions)
	if err != nil {
		t.Fatalf("building the metadata: %v", err)
	}
	t.Logf("server reports %s, fixture ran %d steps and skipped %d", versions, ran, skipped)
	return m
}

// TestArangoDBVersion reads the version and checks what the model makes of
// it.
func TestArangoDBVersion(t *testing.T) {
	db := openArangoDB(t)
	versions, err := dbmeta.ArangoDB.Version(t.Context(), db)
	if err != nil {
		t.Fatalf("reading the version: %v", err)
	}
	if main := versions.Main(); main.Unknown || main.Parts[0] < 3 {
		t.Errorf("expected release 3 or newer, got %s", main)
	}
	if !strings.HasPrefix(versions.String(), "ArangoDB 3.") {
		t.Errorf("expected the display line to name ArangoDB, got %q", versions)
	}
	t.Logf("server reports %s", versions)
}

// TestArangoDBEveryQueryRuns runs every query the model answers and checks
// that the keys of each row are the declared fields, by name and in order.
// The driver takes the columns from the keys of the first row, so the order
// of the RETURN is the order of the columns.
func TestArangoDBEveryQueryRuns(t *testing.T) {
	db := openArangoDB(t)
	m := setupArangoDB(t, db)
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
		if len(cols) == 0 {
			t.Errorf("%s: no row, so the columns cannot be checked", q.Name())
		} else if strings.Join(cols, ",") != strings.Join(names, ",") {
			t.Errorf("%s: returns %v and declares %v", q.Name(), cols, names)
		}
		ran++
	}
	if ran != 5 {
		t.Errorf("%d queries ran, and the model answers 5: change this test with the model", ran)
	}
}

// TestArangoDBScansEveryQuery reads every query through its own Scan, with
// the system collections included.
func TestArangoDBScansEveryQuery(t *testing.T) {
	db := openArangoDB(t)
	scanEveryQuery(t, setupArangoDB(t, db), db)
}

// TestArangoDBFixtureObjects reads the fixture back through the typed API.
func TestArangoDBFixtureObjects(t *testing.T) {
	db := openArangoDB(t)
	m := setupArangoDB(t, db)
	ctx := t.Context()
	fx := arfixture.Everything

	// Every collection is a table of the type collection, the edge
	// collection wrote too, and the view recent is not a table.
	tables := map[string]string{}
	for v, err := range dbmeta.Tables.All(ctx, m, db, nil) {
		if err != nil {
			t.Fatalf("reading tables: %v", err)
		}
		if v.Catalog != fx.Catalog || v.Schema != "" {
			t.Errorf("table %s: catalog %q and schema %q, want %q and none", v.Name, v.Catalog, v.Schema, fx.Catalog)
		}
		tables[v.Name] = v.Type
	}
	for _, name := range []string{"author", "book", "region", "shipment", "note", "loose", "wrote"} {
		if tables[name] != "collection" {
			t.Errorf("expected %s as a collection, got %q", name, tables[name])
		}
	}
	if _, ok := tables["recent"]; ok {
		t.Error("the view recent is a table: AQL was not known to list views")
	}
	if _, ok := tables["_graphs"]; ok {
		t.Error("a system collection is listed without with_system")
	}

	// The columns are the properties of each rule, in its order.
	cols := map[string][]dbmeta.Column{}
	for v, err := range dbmeta.Columns.All(ctx, m, db, nil) {
		if err != nil {
			t.Fatalf("reading columns: %v", err)
		}
		cols[v.Table] = append(cols[v.Table], v)
	}
	names := func(table string) string {
		var out []string
		for _, c := range cols[table] {
			out = append(out, fmt.Sprintf("%d:%s", c.Ordinal, c.Name))
		}
		return strings.Join(out, ",")
	}
	if got := names("book"); got != "1:book_id,2:author_id,3:title,4:published" {
		t.Errorf("book: columns %s", got)
	}
	if got := names("loose"); got != "" {
		t.Errorf("loose has no rule and has the columns %s", got)
	}
	byName := map[string]dbmeta.Column{}
	for _, c := range cols["note"] {
		byName[c.Name] = c
	}
	for name, want := range map[string]struct {
		dataType string
		nullable bool
		pk       bool
	}{
		"_key": {"string", true, true},
		"body": {"", true, false},
		"isbn": {"string, null", true, false},
		"tags": {"array", true, false},
	} {
		c, ok := byName[name]
		if !ok || c.DataType != want.dataType || c.Nullable != want.nullable || c.PrimaryKey != want.pk {
			t.Errorf("note.%s: want %+v, got %+v", name, want, c)
		}
	}
	for _, c := range cols["author"] {
		if want := c.Name == "rating" || c.Name == "shade"; c.Nullable != want {
			t.Errorf("author.%s: nullable %v, want %v", c.Name, c.Nullable, want)
		}
	}

	// One check constraint for each rule, holding the rule and its level.
	checks := map[string]string{}
	for v, err := range dbmeta.Constraints.All(ctx, m, db, nil) {
		if err != nil {
			t.Fatalf("reading constraints: %v", err)
		}
		if v.Type != "check" || v.Name != "" || !v.Definition.Valid {
			t.Errorf("%s: want an unnamed check with a definition, got %+v", v.Table, v)
		}
		checks[v.Table] = v.Definition.V
	}
	if len(checks) != 5 || !strings.Contains(checks["note"], `"level":"new"`) {
		t.Errorf("expected a check on each of the five collections with a rule, got %v", checks)
	}
	if _, ok := checks["loose"]; ok {
		t.Error("loose has no rule and has a check")
	}

	// The two functions, one of them deterministic.
	fns := map[string]dbmeta.Function{}
	for v, err := range dbmeta.Functions.All(ctx, m, db, nil) {
		if err != nil {
			t.Fatalf("reading functions: %v", err)
		}
		fns[v.Name] = v
	}
	if f := fns["dbmeta::full_title"]; f.Volatility != "immutable" || !strings.Contains(f.Source.V, "subtitle") {
		t.Errorf("dbmeta::full_title: got %+v", f)
	}
	if f := fns["dbmeta::roll"]; f.Volatility != "volatile" || f.Language != "javascript" {
		t.Errorf("dbmeta::roll: got %+v", f)
	}

	user, ok, err := dbmeta.First(dbmeta.CurrentUser.All(ctx, m, db, nil))
	if err != nil || !ok || user.Name != "root" {
		t.Errorf("current user: got %+v, %v, %v", user, ok, err)
	}
}

// TestArangoDBLeavesOut checks the kinds the model does not answer, although
// the fixture makes an object of each: AQL lists no index, no view, no
// database and no user, and there is no schema to be current.
func TestArangoDBLeavesOut(t *testing.T) {
	m, err := dbmeta.New(dbmeta.ArangoDB, dbmeta.VersionSet{})
	if err != nil {
		t.Fatal(err)
	}
	for _, q := range []dbmeta.AnyQuery{
		dbmeta.Indexes, dbmeta.IndexColumns, dbmeta.Views, dbmeta.Databases,
		dbmeta.Schemas, dbmeta.CurrentSchema, dbmeta.Roles, dbmeta.Privileges,
		dbmeta.TextSearchDictionaries, dbmeta.RoutineParameters,
	} {
		if s := q.Support(m); s != dbmeta.NotSupported {
			t.Errorf("%s: %v, want NotSupported", q.Name(), s)
		}
	}
	if _, _, err := dbmeta.Indexes.Build(m, nil); !errors.Is(err, dbmeta.ErrNotSupported) {
		t.Errorf("indexes: expected ErrNotSupported, got %v", err)
	}
}
