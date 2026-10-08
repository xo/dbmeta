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

	_ "github.com/xo/dbimp/solr"

	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/container"
	_ "github.com/xo/dbmeta/models/solr"
	slfixture "github.com/xo/dbmeta/models/solr/fixture"
)

// openSolr returns a connection to the server named by DBMETA_SOLR, which is
// the solr:// URL that github.com/xo/dbimp/solr takes, and which dburl's solr
// scheme opens (D154, D179). Its path names the collection dbmeta, whose SQL
// handler takes each statement.
func openSolr(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("DBMETA_SOLR")
	if dsn == "" {
		t.Skip("set DBMETA_SOLR to run against a real server")
	}
	db, err := sql.Open("solr", dsn)
	if err != nil {
		t.Fatalf("opening: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.PingContext(t.Context()); err != nil {
		t.Fatalf("connecting: %v", err)
	}
	return db
}

// solrCall sends one request to the HTTP API of the server that the DSN
// names, with the credentials of the DSN, and returns the answer.
func solrCall(ctx context.Context, dsn, method, path, body string) ([]byte, error) {
	u, err := url.Parse(dsn)
	if err != nil {
		return nil, fmt.Errorf("parsing the dsn: %w", err)
	}
	password, _ := u.User.Password()
	req, err := http.NewRequestWithContext(ctx, method, "http://"+u.Host+path, strings.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("building %s %s: %w", method, path, err)
	}
	req.SetBasicAuth(u.User.Username(), password)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sending %s %s: %w", method, path, err)
	}
	defer res.Body.Close()
	out, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("reading the answer to %s %s: %w", method, path, err)
	}
	if res.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("%s %s answered %d: %s", method, path, res.StatusCode, out)
	}
	return out, nil
}

// solrCollections lists the collections that the server has.
func solrCollections(ctx context.Context, dsn string) ([]string, error) {
	out, err := solrCall(ctx, dsn, http.MethodGet, "/solr/admin/collections?action=LIST&wt=json", "")
	if err != nil {
		return nil, err
	}
	var r struct {
		Collections []string `json:"collections"`
	}
	if err := json.Unmarshal(out, &r); err != nil {
		return nil, fmt.Errorf("reading the collections from %s: %w", out, err)
	}
	return r.Collections, nil
}

// setupSolr builds the fixture as the administrator and returns the metadata
// for the server, with the release that SELECT version() reports. A step
// of a collection that exists is skipped, so a second run changes nothing.
func setupSolr(t *testing.T, db *sql.DB) *dbmeta.Meta {
	t.Helper()
	ctx := t.Context()
	dsn := os.Getenv("DBMETA_SOLR")
	versions, err := dbmeta.Solr.Version(ctx, db)
	if err != nil {
		t.Fatalf("reading the version: %v", err)
	}
	have, err := solrCollections(ctx, dsn)
	if err != nil {
		t.Fatalf("listing the collections: %v", err)
	}
	var ran, skipped int
	for _, step := range slfixture.Everything.Setup {
		if step.Collection != "" && slices.Contains(have, step.Collection) {
			skipped++
			continue
		}
		_, err := solrCall(ctx, dsn, step.Method, step.Path, step.Body)
		if err != nil && (step.Exists == "" || !strings.Contains(err.Error(), step.Exists)) {
			t.Fatalf("setup %s: %v", step.Name, err)
		}
		ran++
	}
	m, err := dbmeta.New(dbmeta.Solr, versions)
	if err != nil {
		t.Fatalf("building the metadata: %v", err)
	}
	t.Logf("server reports %s, fixture ran %d steps and skipped %d", versions, ran, skipped)
	return m
}

// TestSolrVersion checks that SELECT version() gives the release to the
// administrator and that the model parses it.
func TestSolrVersion(t *testing.T) {
	db := openSolr(t)
	versions, err := dbmeta.Solr.Version(t.Context(), db)
	if err != nil {
		t.Fatalf("reading the version: %v", err)
	}
	if main := versions.Main(); main.Unknown || main.Parts[0] < 9 {
		t.Errorf("expected release 9 or newer, got %s", main)
	}
	if !strings.HasPrefix(versions.String(), "Apache Solr ") {
		t.Errorf("expected the display line to name Apache Solr, got %q", versions)
	}
	t.Logf("server reports %s", versions)
}

// TestSolrVersionForAnOrdinaryUser checks that a user who holds the role
// search reads the same release as the administrator, because security.json
// lets that role read /admin/info/system (D192). It checks that the same user
// is still refused the rest of the admin API, such as the list of collections.
func TestSolrVersionForAnOrdinaryUser(t *testing.T) {
	dsn := os.Getenv("DBMETA_SOLR")
	if dsn == "" {
		t.Skip("set DBMETA_SOLR to run against a real server")
	}
	admin, err := dbmeta.Solr.Version(t.Context(), openSolr(t))
	if err != nil {
		t.Fatalf("reading the version as the administrator: %v", err)
	}
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("parsing %s: %v", dsn, err)
	}
	u.User = url.UserPassword(container.SolrUser, container.Password)
	db, err := sql.Open("solr", u.String())
	if err != nil {
		t.Fatalf("opening: %v", err)
	}
	defer db.Close()
	got, err := dbmeta.Solr.Version(t.Context(), db)
	if err != nil {
		t.Fatalf("reading the version as the ordinary user on %s: %v", admin, err)
	}
	if got.String() != admin.String() {
		t.Errorf("expected the ordinary user to read %s, got %s", admin, got)
	}
	if _, err := solrCollections(t.Context(), u.String()); err == nil || !strings.Contains(err.Error(), "answered 403") {
		t.Errorf("expected the ordinary user to be refused the collections API with 403, got %v", err)
	}
}

// TestSolrEveryQueryRuns runs every query the model answers and checks that
// the columns of each row are the declared fields, by name and in order.
func TestSolrEveryQueryRuns(t *testing.T) {
	db := openSolr(t)
	m := setupSolr(t, db)
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
			t.Errorf("%s: no columns", q.Name())
		} else if strings.Join(cols, ",") != strings.Join(names, ",") {
			t.Errorf("%s: returns %v and declares %v", q.Name(), cols, names)
		}
		ran++
	}
	if ran != 4 {
		t.Errorf("%d queries ran, and the model answers 4: change this test with the model", ran)
	}
}

// TestSolrScansEveryQuery reads every query through its own Scan, with the
// system objects included.
func TestSolrScansEveryQuery(t *testing.T) {
	db := openSolr(t)
	scanEveryQuery(t, setupSolr(t, db), db)
}

// TestSolrFixtureObjects reads the fixture back through the typed API.
func TestSolrFixtureObjects(t *testing.T) {
	db := openSolr(t)
	m := setupSolr(t, db)
	ctx := t.Context()
	fx := slfixture.Everything

	tables := map[string]string{}
	for v, err := range dbmeta.Tables.All(ctx, m, db, dbmeta.Args{Schema: fx.Schema}.Map()) {
		if err != nil {
			t.Fatalf("reading tables: %v", err)
		}
		if v.Catalog != fx.Catalog || v.Schema != fx.Schema {
			t.Errorf("table %s: catalog %q and schema %q, want %q and %q", v.Name, v.Catalog, v.Schema, fx.Catalog, fx.Schema)
		}
		tables[v.Name] = v.Type
	}
	// An alias is a table, because SQL does not tell it from a collection.
	for _, name := range []string{"author", "book", "region", "shipment", "recent"} {
		if tables[name] != "table" {
			t.Errorf("expected %s as a table, got %q", name, tables[name])
		}
	}

	// The two tables of metadata are listed with with_system alone.
	sys := map[string]string{}
	for v, err := range dbmeta.Tables.All(ctx, m, db, dbmeta.Args{Schema: "metadata", WithSystem: true}.Map()) {
		if err != nil {
			t.Fatalf("reading the system tables: %v", err)
		}
		sys[v.Name] = v.Type
	}
	if sys["COLUMNS"] != "system table" || sys["TABLES"] != "system table" || len(sys) != 2 {
		t.Errorf("expected the two system tables, got %v", sys)
	}
	if n := countRows(t, dbmeta.Tables.All(ctx, m, db, dbmeta.Args{Schema: "metadata"}.Map())); n != 0 {
		t.Errorf("expected no metadata table without with_system, got %d", n)
	}
	if n := countRows(t, dbmeta.Tables.All(ctx, m, db, dbmeta.Args{Types: []string{"system table"}}.Map())); n != 0 {
		t.Errorf("expected no system table by the type filter without with_system, got %d", n)
	}

	cols := map[string]map[string]dbmeta.Column{}
	for v, err := range dbmeta.Columns.All(ctx, m, db, dbmeta.Args{Schema: fx.Schema}.Map()) {
		if err != nil {
			t.Fatalf("reading columns: %v", err)
		}
		if cols[v.Table] == nil {
			cols[v.Table] = map[string]dbmeta.Column{}
		}
		cols[v.Table][v.Name] = v
	}
	for table, want := range map[string]map[string]string{
		"author":   {"author_id": "BIGINT", "name": "VARCHAR", "rating": "BIGINT", "shade": "VARCHAR", "id": "VARCHAR", "_version_": "BIGINT", "score": "DOUBLE", "_text_": "ANY"},
		"book":     {"published": "TIMESTAMP"},
		"shipment": {"weight": "DOUBLE", "fragile": "VARCHAR"},
	} {
		for name, typ := range want {
			if c, ok := cols[table][name]; !ok || c.DataType != typ {
				t.Errorf("%s.%s: want type %s, got %+v", table, name, typ, c)
			}
		}
	}
	// The alias has the columns of its collection.
	if _, ok := cols["recent"]["published"]; !ok {
		t.Errorf("expected the alias recent to have the column published, got %v", cols["recent"])
	}
	// Every column reads nullable, with no default and no key, even the
	// unique key id.
	for table, byName := range cols {
		for name, c := range byName {
			if !c.Nullable || c.Default.Valid || c.PrimaryKey || c.Identity.V != "" || c.Generated.V != "" {
				t.Errorf("%s.%s: expected a nullable column with nothing else, got %+v", table, name, c)
			}
		}
	}

	s, ok, err := dbmeta.First(dbmeta.CurrentSchema.All(ctx, m, db, nil))
	if err != nil || !ok || s.Name != "solr" || s.Catalog != "solr" {
		t.Errorf("expected the current schema solr, got %+v, %v, %v", s, ok, err)
	}
	var names []string
	for v, err := range dbmeta.Schemas.All(ctx, m, db, dbmeta.Args{WithSystem: true}.Map()) {
		if err != nil {
			t.Fatalf("reading schemas: %v", err)
		}
		names = append(names, v.Name)
	}
	if strings.Join(names, ",") != "metadata,solr" {
		t.Errorf("expected the schemas metadata and solr, got %v", names)
	}
	if n := countRows(t, dbmeta.Schemas.All(ctx, m, db, nil)); n != 1 {
		t.Errorf("expected one schema without with_system, got %d", n)
	}
}

// TestSolrLeavesOut checks the kinds the model does not answer. Solr has no
// DDL, so it has no index, constraint, trigger, sequence or view to list, and
// its aliases, users and settings are in an HTTP API that only an
// administrator can call.
func TestSolrLeavesOut(t *testing.T) {
	m, err := dbmeta.New(dbmeta.Solr, dbmeta.VersionSet{})
	if err != nil {
		t.Fatal(err)
	}
	for _, q := range []dbmeta.AnyQuery{
		dbmeta.Indexes, dbmeta.IndexColumns, dbmeta.Constraints, dbmeta.Triggers, dbmeta.Sequences,
		dbmeta.Views, dbmeta.Databases, dbmeta.Roles, dbmeta.RoleGrants, dbmeta.Privileges, dbmeta.CurrentUser,
		dbmeta.Functions, dbmeta.Settings, dbmeta.PartitionedTables, dbmeta.Comments,
	} {
		if s := q.Support(m); s != dbmeta.NotSupported {
			t.Errorf("%s: %v, want NotSupported", q.Name(), s)
		}
	}
	if _, _, err := dbmeta.Indexes.Build(m, nil); !errors.Is(err, dbmeta.ErrNotSupported) {
		t.Errorf("indexes: expected ErrNotSupported, got %v", err)
	}
}
