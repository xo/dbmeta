package test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strings"
	"testing"

	_ "github.com/xo/dbimp/elasticsearch"

	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/container"
	_ "github.com/xo/dbmeta/models/elasticsearch"
	esfixture "github.com/xo/dbmeta/models/elasticsearch/fixture"
)

// openElasticsearch returns a connection to the server named by
// DBMETA_ELASTICSEARCH, which is the elasticsearch:// URL that
// github.com/xo/dbimp/elasticsearch takes, and which dburl's elasticsearch
// scheme opens (D154, D177).
func openElasticsearch(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("DBMETA_ELASTICSEARCH")
	if dsn == "" {
		t.Skip("set DBMETA_ELASTICSEARCH to run against a real server")
	}
	return openElasticsearchAs(t, dsn)
}

// openElasticsearchAs opens a connection with the DSN dsn.
func openElasticsearchAs(t *testing.T, dsn string) *sql.DB {
	t.Helper()
	db, err := sql.Open("elasticsearch", dsn)
	if err != nil {
		t.Fatalf("opening: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.PingContext(t.Context()); err != nil {
		t.Fatalf("connecting: %v", err)
	}
	return db
}

// esCall sends one request to the HTTP interface, as the user the DSN names,
// and returns the status and the body. The DSN is
// elasticsearch://user:password@host:port, and the interface answers HTTP on
// the same port.
func esCall(ctx context.Context, dsn string, r esfixture.Request) (int, []byte, error) {
	u, err := url.Parse(dsn)
	if err != nil {
		return 0, nil, fmt.Errorf("parsing the dsn: %w", err)
	}
	scheme := "http"
	if u.Query().Get("tls") == "true" {
		scheme = "https"
	}
	req, err := http.NewRequestWithContext(ctx, r.Method, scheme+"://"+u.Host+r.Path, strings.NewReader(r.Body))
	if err != nil {
		return 0, nil, fmt.Errorf("building %s %s: %w", r.Method, r.Path, err)
	}
	password, _ := u.User.Password()
	req.SetBasicAuth(u.User.Username(), password)
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("sending %s %s: %w", r.Method, r.Path, err)
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return 0, nil, fmt.Errorf("reading the answer to %s %s: %w", r.Method, r.Path, err)
	}
	return res.StatusCode, body, nil
}

// esRun sends the steps in order, and returns the first refusal. A step that
// the server refuses with a status of 400 or above is an error, unless ignore
// is true.
func esRun(ctx context.Context, dsn string, steps []esfixture.Step, ignore bool) error {
	for _, step := range steps {
		status, body, err := esCall(ctx, dsn, step.Request)
		switch {
		case err != nil:
			return fmt.Errorf("%s: %w", step.Name, err)
		case status >= http.StatusBadRequest && !ignore:
			return fmt.Errorf("%s: %s %s answered %d: %s", step.Name, step.Request.Method, step.Request.Path, status, body)
		}
	}
	return nil
}

// setupElasticsearch builds the fixture as the administrator and returns the
// metadata for the server. It removes what an earlier run left, and removes
// what it made when the test ends.
func setupElasticsearch(t *testing.T, db *sql.DB) *dbmeta.Meta {
	t.Helper()
	ctx := t.Context()
	dsn := os.Getenv("DBMETA_ELASTICSEARCH")
	versions, err := dbmeta.Elasticsearch.Version(ctx, db)
	if err != nil {
		t.Fatalf("reading the version: %v", err)
	}
	fx := esfixture.Everything
	if err := esRun(ctx, dsn, fx.Teardown, true); err != nil {
		t.Fatalf("removing an earlier fixture: %v", err)
	}
	if err := esRun(ctx, dsn, fx.Setup, false); err != nil {
		t.Fatalf("building the fixture: %v", err)
	}
	t.Cleanup(func() {
		//nolint:errcheck // a teardown is best effort
		esRun(context.WithoutCancel(ctx), dsn, fx.Teardown, true)
	})
	m, err := dbmeta.New(dbmeta.Elasticsearch, versions)
	if err != nil {
		t.Fatalf("building the metadata: %v", err)
	}
	t.Logf("server reports %s", versions)
	return m
}

// TestElasticsearchVersion checks that SELECT version() gives the release to
// the administrator and that the model parses it.
func TestElasticsearchVersion(t *testing.T) {
	db := openElasticsearch(t)
	versions, err := dbmeta.Elasticsearch.Version(t.Context(), db)
	if err != nil {
		t.Fatalf("reading the version: %v", err)
	}
	if main := versions.Main(); main.Unknown || main.Parts[0] < 8 || !strings.HasPrefix(versions.String(), "Elasticsearch ") {
		t.Errorf("expected Elasticsearch 8 or newer, got %s", versions)
	}
	t.Logf("server reports %s", versions)
}

// TestElasticsearchVersionRefusedToAnOrdinaryUser checks that the user of the
// entry cannot read the release. SELECT version() reads GET /, which needs
// the cluster privilege cluster:monitor/main. The role of the user does not
// hold it, and the server answers HTTP 403 with a security_exception, which the caller
// gets as the error and not as an unknown version. D191 records that no other way to read
// the release is in the model.
func TestElasticsearchVersionRefusedToAnOrdinaryUser(t *testing.T) {
	dsn := os.Getenv("DBMETA_ELASTICSEARCH")
	if dsn == "" {
		t.Skip("set DBMETA_ELASTICSEARCH to run against a real server")
	}
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("parsing %s: %v", dsn, err)
	}
	u.User = url.UserPassword(container.ElasticsearchUser, container.Password)
	db, err := sql.Open("elasticsearch", u.String())
	if err != nil {
		t.Fatalf("opening: %v", err)
	}
	defer db.Close()
	_, err = dbmeta.Elasticsearch.Version(t.Context(), db)
	if err == nil || !strings.Contains(err.Error(), "security_exception") {
		t.Errorf("expected the ordinary user to be refused with a security_exception, got %v", err)
	}
}

// TestElasticsearchEveryQueryRuns checks that the model answers 8 kinds, and
// that a query a walk answers has no one statement to build (D175).
func TestElasticsearchEveryQueryRuns(t *testing.T) {
	db := openElasticsearch(t)
	m := setupElasticsearch(t, db)
	var supported, statements int
	for _, q := range dbmeta.Queries() {
		if q.Support(m) != dbmeta.Supported {
			continue
		}
		supported++
		if _, _, err := q.Build(m, nil); err == nil {
			statements++
			if q.Name() != dbmeta.CurrentUser.Name() {
				t.Errorf("%s: expected a walk, which has no one statement to build", q.Name())
			}
		}
	}
	if supported != 8 || statements != 1 {
		t.Errorf("%d queries are supported and %d are one statement, and the model answers 8, 1 of them one statement:"+
			" change this test with the model", supported, statements)
	}
}

// TestElasticsearchScansEveryQuery reads every query through its own Scan,
// with the system objects included.
func TestElasticsearchScansEveryQuery(t *testing.T) {
	db := openElasticsearch(t)
	scanEveryQuery(t, setupElasticsearch(t, db), db)
}

// TestElasticsearchFixtureObjects reads the fixture back through the typed
// API.
func TestElasticsearchFixtureObjects(t *testing.T) {
	db := openElasticsearch(t)
	m := setupElasticsearch(t, db)
	ctx := t.Context()
	prefix := dbmeta.Args{Name: "dbmeta%"}.Map()

	tables := map[string]string{}
	var cluster string
	for v, err := range dbmeta.Tables.All(ctx, m, db, prefix) {
		if err != nil {
			t.Fatalf("reading tables: %v", err)
		}
		if v.Schema != "" || v.Comment.Valid {
			t.Errorf("table %s: expected no schema and no comment, got %+v", v.Name, v)
		}
		if cluster == "" {
			cluster = v.Catalog
		} else if v.Catalog != cluster {
			t.Errorf("table %s: catalog %q, want %q", v.Name, v.Catalog, cluster)
		}
		tables[v.Name] = v.Type
	}
	for name, want := range map[string]string{
		"dbmeta_author": "table", "dbmeta_book": "table", "dbmeta_region": "table",
		"dbmeta_shipment": "table", "dbmeta_types": "table", "dbmeta_recent": "view",
		"dbmeta_stream": "view",
	} {
		if tables[name] != want {
			t.Errorf("expected %s as a %s, got %q", name, want, tables[name])
		}
	}
	for name := range tables {
		if strings.HasPrefix(name, ".") || name == "secret_idx" {
			t.Errorf("expected no hidden index and no index outside dbmeta, got %s", name)
		}
		if !strings.HasPrefix(name, "dbmeta") {
			t.Errorf("expected the pattern dbmeta%% to keep %s out", name)
		}
	}

	views := map[string]bool{}
	for v, err := range dbmeta.Views.All(ctx, m, db, prefix) {
		if err != nil {
			t.Fatalf("reading views: %v", err)
		}
		if v.Definition.Valid || v.Updatable.Valid || v.Catalog != cluster {
			t.Errorf("view %s: expected a name alone, got %+v", v.Name, v)
		}
		views[v.Name] = true
	}
	if !views["dbmeta_recent"] || !views["dbmeta_stream"] || len(views) != 2 {
		t.Errorf("expected the alias and the data stream as the views, got %v", views)
	}
	if n := countRows(t, dbmeta.Tables.All(ctx, m, db, dbmeta.Args{Name: "dbmeta%", Types: []string{"view"}}.Map())); n != 2 {
		t.Errorf("expected 2 tables of the type view, got %d", n)
	}
	if n := countRows(t, dbmeta.Tables.All(ctx, m, db, dbmeta.Args{Schema: "dbmeta"}.Map())); n != 0 {
		t.Errorf("expected no table in a schema, because there is none, got %d", n)
	}

	cols := map[string]map[string]dbmeta.Column{}
	for v, err := range dbmeta.Columns.All(ctx, m, db, dbmeta.Args{Parent: "dbmeta%"}.Map()) {
		if err != nil {
			t.Fatalf("reading columns: %v", err)
		}
		if cols[v.Table] == nil {
			cols[v.Table] = map[string]dbmeta.Column{}
		}
		cols[v.Table][v.Name] = v
		if !v.Nullable || v.PrimaryKey || v.Default.Valid || v.Comment.Valid ||
			!v.Identity.Valid || v.Identity.V != "" || !v.Generated.Valid || v.Generated.V != "" {
			t.Errorf("column %s.%s: expected a nullable column with no key, default or comment, got %+v", v.Table, v.Name, v)
		}
	}
	type want struct {
		ordinal  int
		dataType string
	}
	for table, fields := range map[string]map[string]want{
		// SYS COLUMNS sorts by name, and counts a field it leaves out.
		"dbmeta_author": {
			"author_id": {1, "LONG"}, "name": {2, "TEXT"}, "name.raw": {3, "KEYWORD"},
			"rating": {4, "INTEGER"}, "shade": {5, "KEYWORD"},
		},
		"dbmeta_book": {
			"author_id": {1, "LONG"}, "book_id": {2, "LONG"}, "published": {3, "DATETIME"}, "title": {4, "KEYWORD"},
		},
		"dbmeta_recent": {"published": {3, "DATETIME"}},
		"dbmeta_shipment": {
			"amount": {1, "LONG"}, "dims.h": {5, "INTEGER"}, "dims.w": {6, "INTEGER"},
			"fragile": {7, "BOOLEAN"}, "weight": {10, "DOUBLE"}, "tags.k": {9, "KEYWORD"},
		},
		"dbmeta_types": {
			"ip": {0, "IP"}, "version": {0, "VERSION"}, "unsigned": {0, "UNSIGNED_LONG"},
			"scaled": {0, "SCALED_FLOAT"}, "nanos": {0, "DATETIME"}, "point": {0, "GEO_POINT"},
			"blob": {0, "BINARY"}, "constant": {0, "KEYWORD"},
		},
	} {
		for name, w := range fields {
			c, ok := cols[table][name]
			if !ok {
				if name == "tags.k" {
					// A field of a nested object is not a column.
					continue
				}
				t.Errorf("%s.%s: no such column in %v", table, name, cols[table])
				continue
			}
			if c.DataType != w.dataType || (w.ordinal != 0 && c.Ordinal != w.ordinal) {
				t.Errorf("%s.%s: want ordinal %d and type %s, got %+v", table, name, w.ordinal, w.dataType, c)
			}
		}
	}
	// SYS COLUMNS leaves out an object, a nested field and the types SQL cannot
	// read, and the model returns exactly what it lists. A test of the
	// absence keeps it a decision (D177).
	for _, name := range []string{"vector", "flat", "span", "stats"} {
		if _, ok := cols["dbmeta_types"][name]; ok {
			t.Errorf("dbmeta_types.%s: expected SQL not to list a field of a type it cannot read", name)
		}
	}
	for _, name := range []string{"dims", "tags"} {
		if _, ok := cols["dbmeta_shipment"][name]; ok {
			t.Errorf("dbmeta_shipment.%s: expected SQL not to list a parent field", name)
		}
	}
	// SYS COLUMNS lists a data stream under its backing index, which is
	// hidden, so the view has no column of its own until with_system is set.
	if len(cols["dbmeta_stream"]) != 0 {
		t.Errorf("expected no column under the data stream, got %v", cols["dbmeta_stream"])
	}
	hiddenCols := map[string]string{}
	for v, err := range dbmeta.Columns.All(ctx, m, db, dbmeta.Args{Parent: ".ds-dbmeta_stream%", WithSystem: true}.Map()) {
		if err != nil {
			t.Fatalf("reading columns: %v", err)
		}
		hiddenCols[v.Name] = v.DataType
	}
	if hiddenCols["@timestamp"] != "DATETIME" || hiddenCols["message"] != "KEYWORD" || len(hiddenCols) != 2 {
		t.Errorf("expected the columns of the backing index with with_system, got %v", hiddenCols)
	}
	var backing string
	for v, err := range dbmeta.Tables.All(ctx, m, db, dbmeta.Args{Name: ".ds-%", WithSystem: true}.Map()) {
		if err != nil {
			t.Fatalf("reading tables: %v", err)
		}
		backing = v.Name + " " + v.Type
	}
	if !strings.HasPrefix(backing, ".ds-dbmeta_stream-") || !strings.HasSuffix(backing, " table") {
		t.Errorf("expected the backing index of the data stream as a table with with_system, got %q", backing)
	}
	if n := countRows(t, dbmeta.Tables.All(ctx, m, db, dbmeta.Args{Name: ".ds-%"}.Map())); n != 0 {
		t.Errorf("expected no hidden index without with_system, got %d", n)
	}
	if got := countRows(t, dbmeta.Columns.All(ctx, m, db, dbmeta.Args{Parent: "dbmeta_author", Name: "name%"}.Map())); got != 2 {
		t.Errorf("expected 2 columns of dbmeta_author that start with name, got %d", got)
	}

	// The functions are listed with with_system alone, and an aggregate is
	// also a function.
	if n := countRows(t, dbmeta.Functions.All(ctx, m, db, dbmeta.Args{Name: "SUM"}.Map())); n != 0 {
		t.Errorf("expected no function without with_system, got %d", n)
	}
	var sum dbmeta.Function
	for v, err := range dbmeta.Aggregates.All(ctx, m, db, dbmeta.Args{Name: "SUM", WithSystem: true}.Map()) {
		if err != nil {
			t.Fatalf("reading aggregates: %v", err)
		}
		sum = v
	}
	if sum.Name != "SUM" || sum.Kind != "agg" || sum.Language != "internal" {
		t.Errorf("expected SUM as an aggregate, got %+v", sum)
	}
	kinds := map[string]int{}
	for v, err := range dbmeta.Functions.All(ctx, m, db, dbmeta.Args{WithSystem: true}.Map()) {
		if err != nil {
			t.Fatalf("reading functions: %v", err)
		}
		kinds[v.Kind]++
	}
	if kinds["agg"] < 15 || kinds["func"] < 100 {
		t.Errorf("expected the aggregates and the scalar functions, got %v", kinds)
	}
	if n := countRows(t, dbmeta.Aggregates.All(ctx, m, db, dbmeta.Args{WithSystem: true}.Map())); n != kinds["agg"] {
		t.Errorf("expected %d aggregates, got %d", kinds["agg"], n)
	}

	types := map[string]dbmeta.Type{}
	for v, err := range dbmeta.Types.All(ctx, m, db, nil) {
		if err != nil {
			t.Fatalf("reading types: %v", err)
		}
		types[v.Name] = v
	}
	for _, name := range []string{"LONG", "KEYWORD", "TEXT", "DATETIME", "BOOLEAN", "UNSIGNED_LONG"} {
		if v, ok := types[name]; !ok || v.Kind != "base" || v.Internal != name {
			t.Errorf("expected the base type %s, got %+v", name, v)
		}
	}
	// Every data type of a column is a type that SYS TYPES lists.
	for table, byName := range cols {
		for name, c := range byName {
			if _, ok := types[c.DataType]; !ok {
				t.Errorf("%s.%s has the type %s, which SYS TYPES does not list", table, name, c.DataType)
			}
		}
	}

	dbs := map[string]bool{}
	for v, err := range dbmeta.Databases.All(ctx, m, db, nil) {
		if err != nil {
			t.Fatalf("reading databases: %v", err)
		}
		dbs[v.Name] = true
	}
	if !dbs[cluster] {
		t.Errorf("expected the cluster %s among the databases, got %v", cluster, dbs)
	}

	u, ok, err := dbmeta.First(dbmeta.CurrentUser.All(ctx, m, db, nil))
	if err != nil || !ok || u.Name != "elastic" || u.Session.Valid {
		t.Errorf("expected the current user elastic, got %+v, %v, %v", u, ok, err)
	}
}

// TestElasticsearchLeavesOut checks the kinds the model does not answer.
// Elasticsearch has no schema, and its SQL has no index, constraint,
// trigger, sequence, role or comment to list.
func TestElasticsearchLeavesOut(t *testing.T) {
	m, err := dbmeta.New(dbmeta.Elasticsearch, dbmeta.VersionSet{})
	if err != nil {
		t.Fatal(err)
	}
	for _, q := range []dbmeta.AnyQuery{
		dbmeta.Schemas, dbmeta.CurrentSchema, dbmeta.Indexes, dbmeta.IndexColumns,
		dbmeta.Constraints, dbmeta.ConstraintColumns, dbmeta.Triggers, dbmeta.Sequences,
		dbmeta.Roles, dbmeta.Privileges, dbmeta.RoleGrants, dbmeta.Settings,
		dbmeta.RoutineParameters, dbmeta.PartitionedTables, dbmeta.Comments,
	} {
		if s := q.Support(m); s != dbmeta.NotSupported {
			t.Errorf("%s: %v, want NotSupported", q.Name(), s)
		}
	}
	if _, _, err := dbmeta.Indexes.Build(m, nil); !errors.Is(err, dbmeta.ErrNotSupported) {
		t.Errorf("indexes: expected ErrNotSupported, got %v", err)
	}
}

// TestElasticsearchPages checks that SYS COLUMNS reads more than one page,
// that the walk follows the cursor of the driver, and that stopping early
// closes the cursor on the server. SYS COLUMNS answers 1000 rows to a page.
func TestElasticsearchPages(t *testing.T) {
	db := openElasticsearch(t)
	m := setupElasticsearch(t, db)
	ctx := t.Context()
	dsn := os.Getenv("DBMETA_ELASTICSEARCH")

	// 30 indices of 50 fields are 1500 rows, which are two pages.
	const indices, fields = 30, 50
	var props []string
	for i := range fields {
		props = append(props, fmt.Sprintf(`"f%02d":{"type":"keyword"}`, i))
	}
	mapping := `{"mappings":{"properties":{` + strings.Join(props, ",") + `}}}`
	var up, down []esfixture.Step
	for i := range indices {
		name := fmt.Sprintf("dbmeta_page_%02d", i)
		up = append(up, esfixture.Step{Name: name, Request: esfixture.Request{Method: http.MethodPut, Path: "/" + name, Body: mapping}})
		down = append(down, esfixture.Step{Name: name, Request: esfixture.Request{Method: http.MethodDelete, Path: "/" + name}})
	}
	if err := esRun(ctx, dsn, down, true); err != nil {
		t.Fatalf("removing earlier indices: %v", err)
	}
	if err := esRun(ctx, dsn, up, false); err != nil {
		t.Fatalf("making the indices: %v", err)
	}
	t.Cleanup(func() {
		//nolint:errcheck // a teardown is best effort
		esRun(context.WithoutCancel(ctx), dsn, down, true)
	})

	var got, first int
	tables := map[string]int{}
	for v, err := range dbmeta.Columns.All(ctx, m, db, dbmeta.Args{Parent: "dbmeta_page_%"}.Map()) {
		if err != nil {
			t.Fatalf("reading columns: %v", err)
		}
		tables[v.Table]++
		got++
	}
	if got != indices*fields || len(tables) != indices {
		t.Errorf("expected %d rows of %d indices, got %d rows of %d", indices*fields, indices, got, len(tables))
	}
	// The whole cluster is more than a page too, so the first row is read
	// before the first page ends.
	for _, err := range dbmeta.Columns.All(ctx, m, db, nil) {
		if err != nil {
			t.Fatalf("reading columns: %v", err)
		}
		first++
		break
	}
	if first != 1 {
		t.Errorf("expected one column before the break, got %d", first)
	}

	// Every cursor that a walk opened is closed, whether the walk read every
	// page or stopped on the first.
	status, body, err := esCall(ctx, dsn, esfixture.Request{Method: http.MethodGet, Path: "/_nodes/stats/indices/search"})
	if err != nil || status != http.StatusOK {
		t.Fatalf("reading the search statistics: %d, %v: %s", status, err, body)
	}
	var stats struct {
		Nodes map[string]struct {
			Indices struct {
				Search struct {
					OpenContexts int `json:"open_contexts"`
				} `json:"search"`
			} `json:"indices"`
		} `json:"nodes"`
	}
	if err := json.Unmarshal(body, &stats); err != nil {
		t.Fatalf("reading the search statistics from %s: %v", body, err)
	}
	for node, n := range stats.Nodes {
		if open := n.Indices.Search.OpenContexts; open != 0 {
			t.Errorf("node %s holds %d open search contexts after the walks", node, open)
		}
	}
}

// TestElasticsearchWalkStops checks that a walk ends when its caller stops.
func TestElasticsearchWalkStops(t *testing.T) {
	db := openElasticsearch(t)
	m := setupElasticsearch(t, db)
	if _, _, err := dbmeta.Columns.Build(m, nil); err == nil {
		t.Error("columns: expected no one statement to build")
	}
	n := 0
	for _, err := range dbmeta.Tables.All(t.Context(), m, db, nil) {
		if err != nil {
			t.Fatalf("reading tables: %v", err)
		}
		n++
		break
	}
	if n != 1 {
		t.Errorf("expected one table before the break, got %d", n)
	}
	// A canceled context ends the walk with its error.
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	var got error
	for _, err := range dbmeta.Tables.All(ctx, m, db, nil) {
		got = err
		break
	}
	if !errors.Is(got, context.Canceled) {
		t.Errorf("expected the error of the context, got %v", got)
	}
}

// TestElasticsearchHidesWhatTheUserCannotRead checks that the ordinary user
// sees the fixture and nothing outside the name dbmeta, and that the
// administrator sees the index secret_idx.
func TestElasticsearchHidesWhatTheUserCannotRead(t *testing.T) {
	dsn := os.Getenv("DBMETA_ELASTICSEARCH")
	if dsn == "" {
		t.Skip("set DBMETA_ELASTICSEARCH to run against a real server")
	}
	m := setupElasticsearch(t, openElasticsearch(t))
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("parsing %s: %v", dsn, err)
	}
	u.User = url.UserPassword(container.ElasticsearchUser, container.Password)
	names := func(db *sql.DB) []string {
		var out []string
		for v, err := range dbmeta.Tables.All(t.Context(), m, db, nil) {
			if err != nil {
				t.Fatalf("reading tables: %v", err)
			}
			out = append(out, v.Name)
		}
		return out
	}
	admin, user := names(openElasticsearch(t)), names(openElasticsearchAs(t, u.String()))
	if !slices.Contains(admin, "secret_idx") || slices.Contains(user, "secret_idx") {
		t.Errorf("expected secret_idx for the administrator alone, got %v and %v", admin, user)
	}
	if !slices.Contains(user, "dbmeta_author") {
		t.Errorf("expected the user to read dbmeta_author, got %v", user)
	}
}
