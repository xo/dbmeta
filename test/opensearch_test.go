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
	"slices"
	"strings"
	"testing"

	_ "github.com/xo/dbimp/opensearch"

	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/container"
	_ "github.com/xo/dbmeta/models/opensearch"
	osfixture "github.com/xo/dbmeta/models/opensearch/fixture"
)

// openOpenSearch returns a connection to the server named by
// DBMETA_OPENSEARCH, which is the opensearch:// URL that
// github.com/xo/dbimp/opensearch takes, and which dburl's opensearch scheme
// opens (D154, D181).
func openOpenSearch(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("DBMETA_OPENSEARCH")
	if dsn == "" {
		t.Skip("set DBMETA_OPENSEARCH to run against a real server")
	}
	return openOpenSearchAs(t, dsn)
}

// openOpenSearchAs opens a connection with the DSN dsn.
func openOpenSearchAs(t *testing.T, dsn string) *sql.DB {
	t.Helper()
	db, err := sql.Open("opensearch", dsn)
	if err != nil {
		t.Fatalf("opening: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.PingContext(t.Context()); err != nil {
		t.Fatalf("connecting: %v", err)
	}
	return db
}

// osCall sends one request to the HTTP interface, as the user the DSN names,
// and returns the status and the body. The DSN is
// opensearch://user:password@host:port, and the interface answers HTTP on the
// same port.
func osCall(ctx context.Context, dsn string, r osfixture.Request) (int, []byte, error) {
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

// osRun sends the steps in order, and returns the first refusal. A step that
// the server refuses with a status of 400 or above is an error, unless ignore
// is true.
func osRun(ctx context.Context, dsn string, steps []osfixture.Step, ignore bool) error {
	for _, step := range steps {
		status, body, err := osCall(ctx, dsn, step.Request)
		switch {
		case err != nil:
			return fmt.Errorf("%s: %w", step.Name, err)
		case status >= http.StatusBadRequest && !ignore:
			return fmt.Errorf("%s: %s %s answered %d: %s", step.Name, step.Request.Method, step.Request.Path, status, body)
		}
	}
	return nil
}

// setupOpenSearch builds the fixture as the administrator and returns the
// metadata for the server. It removes what an earlier run left, and removes
// what it made when the test ends.
func setupOpenSearch(t *testing.T, db *sql.DB) *dbmeta.Meta {
	t.Helper()
	ctx := t.Context()
	dsn := os.Getenv("DBMETA_OPENSEARCH")
	versions, err := dbmeta.OpenSearch.Version(ctx, db)
	if err != nil {
		t.Fatalf("reading the version: %v", err)
	}
	fx := osfixture.Everything
	if err := osRun(ctx, dsn, fx.Teardown, true); err != nil {
		t.Fatalf("removing an earlier fixture: %v", err)
	}
	if err := osRun(ctx, dsn, fx.Setup, false); err != nil {
		t.Fatalf("building the fixture: %v", err)
	}
	t.Cleanup(func() {
		//nolint:errcheck // a teardown is best effort
		osRun(context.WithoutCancel(ctx), dsn, fx.Teardown, true)
	})
	m, err := dbmeta.New(dbmeta.OpenSearch, versions)
	if err != nil {
		t.Fatalf("building the metadata: %v", err)
	}
	t.Logf("server reports %s", versions)
	return m
}

// TestOpenSearchVersion checks that SELECT version() gives the release to the
// administrator and that the model parses it.
func TestOpenSearchVersion(t *testing.T) {
	db := openOpenSearch(t)
	versions, err := dbmeta.OpenSearch.Version(t.Context(), db)
	if err != nil {
		t.Fatalf("reading the version: %v", err)
	}
	if main := versions.Main(); main.Unknown || main.Parts[0] < 2 || !strings.HasPrefix(versions.String(), "OpenSearch ") {
		t.Errorf("expected OpenSearch 2 or newer, got %s", versions)
	}
	t.Logf("server reports %s", versions)
}

// TestOpenSearchVersionForAnOrdinaryUser checks what the user of the entry
// gets. On 2.x, GET / needs the cluster privilege cluster:monitor/main, which
// the role of the user does not hold. The server answers HTTP 403 with a
// security_exception, which the caller gets as the error and not as an unknown
// version. On 3.x the driver reads the release from the header
// X-OpenSearch-Version, which every user gets. D191 records that no other way
// to read the release on 2.x is in the model.
func TestOpenSearchVersionForAnOrdinaryUser(t *testing.T) {
	dsn := os.Getenv("DBMETA_OPENSEARCH")
	if dsn == "" {
		t.Skip("set DBMETA_OPENSEARCH to run against a real server")
	}
	admin, err := dbmeta.OpenSearch.Version(t.Context(), openOpenSearch(t))
	if err != nil {
		t.Fatalf("reading the version as the administrator: %v", err)
	}
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("parsing %s: %v", dsn, err)
	}
	u.User = url.UserPassword(container.OpenSearchUser, container.Password)
	got, err := dbmeta.OpenSearch.Version(t.Context(), openOpenSearchAs(t, u.String()))
	if admin.Main().Parts[0] < 3 {
		if err == nil || !strings.Contains(err.Error(), "security_exception") {
			t.Errorf("expected the ordinary user to be refused with a security_exception on %s, got %v", admin, err)
		}
		return
	}
	if err != nil {
		t.Fatalf("reading the version as the ordinary user on %s: %v", admin, err)
	}
	if got.String() != admin.String() {
		t.Errorf("expected the ordinary user to read %s, got %s", admin, got)
	}
}

// TestOpenSearchEveryQueryRuns checks that the model answers 3 kinds, and that
// each is a walk with no one statement to build (D175).
func TestOpenSearchEveryQueryRuns(t *testing.T) {
	db := openOpenSearch(t)
	m := setupOpenSearch(t, db)
	var supported int
	for _, q := range dbmeta.Queries() {
		if q.Support(m) != dbmeta.Supported {
			continue
		}
		supported++
		if _, _, err := q.Build(m, nil); err == nil {
			t.Errorf("%s: expected a walk, which has no one statement to build", q.Name())
		}
	}
	if supported != 3 {
		t.Errorf("%d queries are supported, and the model answers 3: change this test with the model", supported)
	}
}

// TestOpenSearchScansEveryQuery reads every query through its own Scan, with
// the system objects included.
func TestOpenSearchScansEveryQuery(t *testing.T) {
	db := openOpenSearch(t)
	m := setupOpenSearch(t, db)
	scanEveryQuery(t, m, db)
}

// TestOpenSearchFixtureObjects reads the fixture back through the typed API.
func TestOpenSearchFixtureObjects(t *testing.T) {
	db := openOpenSearch(t)
	m := setupOpenSearch(t, db)
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
	for _, name := range []string{
		"dbmeta_author", "dbmeta_book", "dbmeta_region", "dbmeta_shipment", "dbmeta_types", "dbmeta_empty", "dbmeta",
	} {
		if tables[name] != "table" {
			t.Errorf("expected %s as a table, got %q", name, tables[name])
		}
	}
	// SQL lists an alias as BASE TABLE on 2.19.6 and does not list it on 3.9.0.
	_, listed := tables["dbmeta_recent"]
	if main := m.Version().Main().Parts[0]; (main < 3) != listed {
		t.Errorf("expected the alias dbmeta_recent to be listed on 2.x alone, got listed %v on %s", listed, m.Version().Main())
	}
	for name := range tables {
		if strings.HasPrefix(name, ".") || name == "secret_idx" {
			t.Errorf("expected no hidden index and no index outside dbmeta, got %s", name)
		}
		if !strings.HasPrefix(name, "dbmeta") {
			t.Errorf("expected the pattern dbmeta%% to keep %s out", name)
		}
	}
	if n := countRows(t, dbmeta.Tables.All(ctx, m, db, dbmeta.Args{Name: "dbmeta%", Types: []string{"view"}}.Map())); n != 0 {
		t.Errorf("expected no table of the type view, because SQL cannot tell an alias from an index, got %d", n)
	}
	if n := countRows(t, dbmeta.Tables.All(ctx, m, db, dbmeta.Args{Schema: "dbmeta"}.Map())); n != 0 {
		t.Errorf("expected no table in a schema, because there is none, got %d", n)
	}
	// An underscore is a wildcard in SHOW TABLES LIKE and has no escape, so the
	// walk matches the name in Go, where a backslash escapes it.
	if n := countRows(t, dbmeta.Tables.All(ctx, m, db, dbmeta.Args{Name: `dbmeta\_author`}.Map())); n != 1 {
		t.Errorf("expected one table for the escaped name, got %d", n)
	}
	if n := countRows(t, dbmeta.Tables.All(ctx, m, db, dbmeta.Args{Name: `dbmeta-author`}.Map())); n != 0 {
		t.Errorf("expected no table for a name that differs in one character, got %d", n)
	}
	// The hidden indices need with_system.
	if n := countRows(t, dbmeta.Tables.All(ctx, m, db, dbmeta.Args{Name: ".opendistro%"}.Map())); n != 0 {
		t.Errorf("expected no hidden index without with_system, got %d", n)
	}
	if n := countRows(t, dbmeta.Tables.All(ctx, m, db, dbmeta.Args{Name: ".opendistro%", WithSystem: true}.Map())); n == 0 {
		t.Error("expected the security index with with_system")
	}

	dbs := map[string]bool{}
	for v, err := range dbmeta.Databases.All(ctx, m, db, nil) {
		if err != nil {
			t.Fatalf("reading databases: %v", err)
		}
		dbs[v.Name] = true
	}
	if len(dbs) != 1 || !dbs[cluster] {
		t.Errorf("expected the cluster %s alone among the databases, got %v", cluster, dbs)
	}
	if n := countRows(t, dbmeta.Databases.All(ctx, m, db, dbmeta.Args{Name: "no-such-cluster"}.Map())); n != 0 {
		t.Errorf("expected no database for a pattern that does not match, got %d", n)
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
	for table, fields := range map[string][]string{
		"dbmeta_author":   {"author_id", "name", "rating", "shade"},
		"dbmeta_book":     {"author_id", "book_id", "published", "title"},
		"dbmeta_region":   {"area", "country", "population"},
		"dbmeta_shipment": {"amount", "area", "country", "dims", "dims.h", "dims.w", "fragile", "shipment_id", "tags", "tags.k", "weight"},
	} {
		got := make([]string, 0, len(cols[table]))
		for name := range cols[table] {
			got = append(got, name)
		}
		slices.Sort(got)
		if !slices.Equal(got, fields) {
			t.Errorf("%s: expected the columns %v, got %v", table, fields, got)
		}
	}
	for table, want := range map[string]map[string]string{
		"dbmeta_author":   {"author_id": "long", "name": "text", "rating": "integer", "shade": "keyword"},
		"dbmeta_shipment": {"weight": "double", "fragile": "boolean", "dims": "object", "dims.h": "integer", "tags": "nested"},
		"dbmeta_book":     {"published": "timestamp"},
		"dbmeta_types": {
			"ip": "ip", "scaled": "scaled_float", "half": "half_float", "point": "geo_point",
			"blob": "binary", "tiny": "byte", "small": "short", "real": "float", "flag": "boolean",
			"nanos": "timestamp",
		},
	} {
		for name, typ := range want {
			if c, ok := cols[table][name]; !ok || c.DataType != typ {
				t.Errorf("%s.%s: expected the type %s, got %+v", table, name, typ, c)
			}
		}
	}
	// A multi-field, such as name.raw, is not a row of DESCRIBE. An object and a
	// nested field are, with the types object and nested. A field of a type SQL
	// cannot read, such as integer_range, is not, and a test of each absence
	// keeps it a decision (D181).
	for table, name := range map[string]string{
		"dbmeta_author": "name.raw", "dbmeta_types": "span",
	} {
		if _, ok := cols[table][name]; ok {
			t.Errorf("%s.%s: expected DESCRIBE not to list it", table, name)
		}
	}
	if len(cols["dbmeta_empty"]) != 0 {
		t.Errorf("expected no column in an index with no field, got %v", cols["dbmeta_empty"])
	}
	if got := countRows(t, dbmeta.Columns.All(ctx, m, db, dbmeta.Args{Parent: "dbmeta_author", Name: "name%"}.Map())); got != 1 {
		t.Errorf("expected 1 column of dbmeta_author that starts with name, got %d", got)
	}
	// The ordinal is the position DESCRIBE reports, and every column of an index
	// has a different one.
	for table, byName := range cols {
		seen := map[int]string{}
		for name, c := range byName {
			if other, dup := seen[c.Ordinal]; dup {
				t.Errorf("%s: %s and %s share the ordinal %d", table, name, other, c.Ordinal)
			}
			seen[c.Ordinal] = name
		}
	}
	if n := countRows(t, dbmeta.Columns.All(ctx, m, db, dbmeta.Args{Parent: ".opendistro%"}.Map())); n != 0 {
		t.Errorf("expected no column of a hidden index without with_system, got %d", n)
	}
}

// TestOpenSearchSemicolon checks that the SQL takes a semicolon at the end of a
// statement, which is why the model keeps it and does not set
// TerminatorStripped.
func TestOpenSearchSemicolon(t *testing.T) {
	db := openOpenSearch(t)
	for _, query := range []string{"SELECT 1;", "SHOW TABLES LIKE dbmeta;", "DESCRIBE TABLES LIKE 'dbmeta';"} {
		if _, err := countStatementRows(t.Context(), db, query); err != nil {
			t.Errorf("%s: %v", query, err)
		}
	}
	if n, err := countStatementRows(t.Context(), db, "SHOW TABLES LIKE dbmeta;"); err != nil || n != 1 {
		t.Errorf("expected the index dbmeta that the entry makes, got %d rows and %v", n, err)
	}
	info, _ := dbmeta.OpenSearch.Info()
	if info.Terminator != dbmeta.TerminatorKept {
		t.Errorf("expected the terminator to be kept, got %v", info.Terminator)
	}
}

// countStatementRows runs one statement and counts the rows it answers.
func countStatementRows(ctx context.Context, db *sql.DB, query string) (int, error) {
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return 0, fmt.Errorf("running %s: %w", query, err)
	}
	defer rows.Close()
	n := 0
	for rows.Next() {
		n++
	}
	return n, rows.Err()
}

// TestOpenSearchLeavesOut checks the kinds the model does not answer.
// OpenSearch has no schema, and its SQL has no view it can tell from an index,
// and no index, constraint, trigger, sequence, function list, role or comment
// to list.
func TestOpenSearchLeavesOut(t *testing.T) {
	m, err := dbmeta.New(dbmeta.OpenSearch, dbmeta.VersionSet{})
	if err != nil {
		t.Fatal(err)
	}
	for _, q := range []dbmeta.AnyQuery{
		dbmeta.Views, dbmeta.Schemas, dbmeta.CurrentSchema, dbmeta.CurrentUser, dbmeta.Indexes,
		dbmeta.IndexColumns, dbmeta.Constraints, dbmeta.ConstraintColumns, dbmeta.Triggers,
		dbmeta.Sequences, dbmeta.Roles, dbmeta.Privileges, dbmeta.RoleGrants, dbmeta.Settings,
		dbmeta.Functions, dbmeta.Aggregates, dbmeta.Types, dbmeta.RoutineParameters,
		dbmeta.PartitionedTables, dbmeta.Comments,
	} {
		if s := q.Support(m); s != dbmeta.NotSupported {
			t.Errorf("%s: %v, want NotSupported", q.Name(), s)
		}
	}
	if _, _, err := dbmeta.Indexes.Build(m, nil); !errors.Is(err, dbmeta.ErrNotSupported) {
		t.Errorf("indexes: expected ErrNotSupported, got %v", err)
	}
}

// TestOpenSearchWalkStops checks that a walk ends when its caller stops.
func TestOpenSearchWalkStops(t *testing.T) {
	db := openOpenSearch(t)
	m := setupOpenSearch(t, db)
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
	got = nil
	for _, err := range dbmeta.Columns.All(ctx, m, db, nil) {
		got = err
		break
	}
	if !errors.Is(got, context.Canceled) {
		t.Errorf("expected the error of the context for the columns, got %v", got)
	}
}

// TestOpenSearchTheUserSeesEveryName checks what the role of the entry costs:
// the ordinary user can read the indices dbmeta* and sees the name of every
// index, secret_idx included, because SHOW TABLES and DESCRIBE need
// indices:admin/get on every index (D176). The columns of an index the user
// cannot read are another matter, which TestOpenSearchTheUserReadsNoSecret
// checks.
func TestOpenSearchTheUserSeesEveryName(t *testing.T) {
	dsn := os.Getenv("DBMETA_OPENSEARCH")
	if dsn == "" {
		t.Skip("set DBMETA_OPENSEARCH to run against a real server")
	}
	m := setupOpenSearch(t, openOpenSearch(t))
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("parsing %s: %v", dsn, err)
	}
	u.User = url.UserPassword(container.OpenSearchUser, container.Password)
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
	admin, user := names(openOpenSearch(t)), names(openOpenSearchAs(t, u.String()))
	if !slices.Contains(admin, "secret_idx") || !slices.Contains(user, "secret_idx") {
		t.Errorf("expected secret_idx for both, got %v and %v", admin, user)
	}
	if !slices.Contains(user, "dbmeta_author") {
		t.Errorf("expected the user to read dbmeta_author, got %v", user)
	}
}

// TestOpenSearchTheUserReadsNoSecret checks that a walk of Columns as the
// ordinary user goes on after an index it is refused, and that the refusal is
// not an error of the walk. It reads the permission of the user: on 2.19.6 the
// plugin refuses DESCRIBE of secret_idx, and on 3.9.0 it describes it.
func TestOpenSearchTheUserReadsNoSecret(t *testing.T) {
	dsn := os.Getenv("DBMETA_OPENSEARCH")
	if dsn == "" {
		t.Skip("set DBMETA_OPENSEARCH to run against a real server")
	}
	m := setupOpenSearch(t, openOpenSearch(t))
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("parsing %s: %v", dsn, err)
	}
	u.User = url.UserPassword(container.OpenSearchUser, container.Password)
	db := openOpenSearchAs(t, u.String())
	tables := map[string]int{}
	for v, err := range dbmeta.Columns.All(t.Context(), m, db, nil) {
		if err != nil {
			t.Fatalf("reading columns: %v", err)
		}
		tables[v.Table]++
	}
	if tables["dbmeta_author"] != 4 {
		t.Errorf("expected the 4 columns of dbmeta_author, got %v", tables)
	}
	// The role of the user holds indices:admin/mappings/get on dbmeta* alone, so
	// the plugin refuses DESCRIBE of secret_idx. The walk goes on to the next
	// index, and secret_idx has no column for the user.
	if tables["secret_idx"] != 0 {
		t.Errorf("expected no column of secret_idx for the ordinary user, got %d", tables["secret_idx"])
	}
	admin := 0
	for _, err := range dbmeta.Columns.All(t.Context(), m, openOpenSearch(t), dbmeta.Args{Parent: "secret_idx"}.Map()) {
		if err != nil {
			t.Fatalf("reading columns: %v", err)
		}
		admin++
	}
	if admin != 1 {
		t.Errorf("expected the column of secret_idx for the administrator, got %d", admin)
	}
}
