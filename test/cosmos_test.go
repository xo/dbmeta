package test

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	_ "github.com/xo/dbimp/cosmos"

	"github.com/xo/dbmeta"
	_ "github.com/xo/dbmeta/models/cosmos"
	cmfixture "github.com/xo/dbmeta/models/cosmos/fixture"
)

// The driver is github.com/xo/dbimp/cosmos, which dburl v0.50.0 names for the
// cosmos scheme (D154). It reads only, so the fixture is built through the REST
// API by cosmos_rest_test.go. The DSN holds the account key as its password and
// no test prints it. See D228.

// cosmosContainer is the container that a connection names, so that the
// statements for the scripts of a container have one to read.
const cosmosContainer = cmfixture.Book

// withCosmosContainer returns dsn with the container in its path, after the
// database that the path names.
func withCosmosContainer(t *testing.T, dsn, container string) string {
	t.Helper()
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatal("the DSN of Cosmos DB is not a URL")
	}
	database, _, _ := strings.Cut(strings.Trim(u.Path, "/"), "/")
	u.Path = "/" + database
	if container != "" {
		u.Path += "/" + container
	}
	return u.String()
}

// openCosmos returns a connection to the account named by DBMETA_COSMOS, which
// dbrun resolves from the places D117 names, and which holds the account key.
// The path of the connection names the container book.
func openCosmos(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("DBMETA_COSMOS")
	if dsn == "" {
		t.Skip("set DBMETA_COSMOS to run against the account")
	}
	return openAt(t, "cosmos", withCosmosContainer(t, dsn, cosmosContainer))
}

// cosmosState is the one fixture that every Cosmos DB test shares.
//
// Each step of the fixture is a request that costs request units from the 400 that
// the containers of the database share, so a fixture for each test is more than the
// tests are worth. The first test builds it, every test reads it, and TestMain drops
// it when the last test ends. No test changes it.
var cosmosState struct {
	mu    sync.Mutex
	meta  *dbmeta.Meta
	err   error
	built bool
}

// cosmosSelf holds the link of each resource that a step made, by the name that a
// body uses for it.
type cosmosSelf map[string]string

// expand replaces the links that a body names with the links that earlier steps
// made.
func (s cosmosSelf) expand(body string) string {
	for name, link := range s {
		body = strings.ReplaceAll(body, "$("+name+")", link)
	}
	return body
}

// runCosmosSteps sends the requests one at a time. A step of a teardown that fails
// is not an error when quiet is true, because the object is already gone.
func runCosmosSteps(ctx context.Context, a *cosmosAccount, steps []cmfixture.Step, quiet bool) error {
	self := cosmosSelf{}
	for _, s := range steps {
		reply, err := a.do(ctx, s.Method, "/dbs/"+a.database+s.Path, nil, self.expand(s.Body))
		switch {
		case err != nil && !quiet:
			return fmt.Errorf("%s: %w", s.Name, err)
		case err != nil:
			continue
		}
		ok := reply.Status == http.StatusCreated || reply.Status == http.StatusNoContent
		if !ok && !quiet {
			return fmt.Errorf("%s: HTTP %d %s", s.Name, reply.Status, reply.Body)
		}
		if reply.Status == http.StatusCreated && s.Path == "/colls" {
			var res map[string]any
			if json.Unmarshal(reply.Body, &res) == nil {
				link, _ := res["_self"].(string)
				self["container "+s.Name] = link
			}
		}
	}
	return nil
}

// shutdownCosmos drops the fixture, if a test built it. TestMain calls it.
func shutdownCosmos() {
	cosmosState.mu.Lock()
	defer cosmosState.mu.Unlock()
	if !cosmosState.built {
		return
	}
	// DBMETA_COSMOS_KEEP leaves the fixture up, so that a person can write a
	// statement against it, and the next run with it set builds it again.
	if os.Getenv("DBMETA_COSMOS_KEEP") != "" {
		return
	}
	a, err := parseCosmosDSN(os.Getenv("DBMETA_COSMOS"))
	if err != nil {
		fmt.Fprintln(os.Stderr, "dropping the Cosmos DB fixture:", err)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	//nolint:errcheck // a teardown is best effort, and quiet says so
	runCosmosSteps(ctx, a, cmfixture.Everything.Teardown, true)
}

// setupCosmos builds the fixture, once, and returns the metadata for the account.
//
// It tears down first, because a run that failed part way can leave containers
// behind, and a container that exists makes its step fail with a conflict.
func setupCosmos(t *testing.T, db *sql.DB) *dbmeta.Meta {
	t.Helper()
	cosmosState.mu.Lock()
	defer cosmosState.mu.Unlock()
	if cosmosState.err != nil {
		t.Fatalf("the fixture failed to build in an earlier test: %v", cosmosState.err)
	}
	if cosmosState.meta != nil {
		return cosmosState.meta
	}
	ctx := context.WithoutCancel(t.Context())
	fail := func(format string, args ...any) {
		cosmosState.err = fmt.Errorf(format, args...)
		t.Fatal(cosmosState.err)
	}
	versions, err := dbmeta.Cosmos.Version(ctx, db)
	if err != nil {
		fail("reading the version: %v", err)
	}
	m, err := dbmeta.New(dbmeta.Cosmos, versions)
	if err != nil {
		fail("building the metadata: %v", err)
	}
	a, err := parseCosmosDSN(os.Getenv("DBMETA_COSMOS"))
	if err != nil {
		fail("reading the DSN: %v", err)
	}
	if a.database != cmfixture.Database {
		fail("the DSN names the database %q, and the fixture builds in %q", a.database, cmfixture.Database)
	}
	cosmosState.built = true
	//nolint:errcheck // a teardown before setup is best effort, and quiet says so
	runCosmosSteps(ctx, a, cmfixture.Everything.Teardown, true)
	if err := runCosmosSteps(ctx, a, cmfixture.Everything.Setup, false); err != nil {
		fail("setup: %v", err)
	}
	t.Logf("server reports %s, fixture ran %d steps", versions, len(cmfixture.Everything.Setup))
	cosmosState.meta = m
	return m
}

// cosmosAll reads every row a query answers, and stops the test on an error.
func cosmosAll[T any](t *testing.T, q *dbmeta.Query[T], m *dbmeta.Meta, db *sql.DB, args map[string]any) []T {
	t.Helper()
	var out []T
	for v, err := range q.All(t.Context(), m, db, args) {
		if err != nil {
			t.Fatalf("reading %s: %v", q.Name(), err)
		}
		out = append(out, v)
	}
	return out
}

// cosmosColumns runs the statement and returns the names of its columns.
func cosmosColumns(t *testing.T, db *sql.DB, query string) ([]string, error) {
	t.Helper()
	rows, err := db.QueryContext(t.Context(), query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	for rows.Next() {
	}
	return cols, rows.Err()
}

// cosmosOptions returns the pairs of an Options text, by name.
func cosmosOptions(opts sql.Null[string]) map[string]string {
	out := map[string]string{}
	if !opts.Valid {
		return out
	}
	for pair := range strings.SplitSeq(opts.V, ", ") {
		name, value, _ := strings.Cut(pair, "=")
		out[name] = value
	}
	return out
}

// TestCosmosVersion checks that the account reports no version, which is why the
// model declares no version query. See D228.
func TestCosmosVersion(t *testing.T) {
	db := openCosmos(t)
	versions, err := dbmeta.Cosmos.Version(t.Context(), db)
	if err != nil {
		t.Fatalf("reading the version: %v", err)
	}
	if !versions.Main().IsZero() && !versions.Main().Unknown {
		t.Errorf("expected no version, got %s", versions)
	}
	// usql declares no version for its driver, so it runs SELECT version(), which
	// is a query of the documents of a container and a function that has no name.
	if err := cosmosDrain(t, db, "SELECT version()"); err == nil {
		t.Error("SELECT version(): expected Cosmos DB to refuse it")
	}
	t.Logf("server reports %s", versions)
}

// cosmosDrain runs a statement and reads every row.
func cosmosDrain(t *testing.T, db *sql.DB, stmt string) error {
	t.Helper()
	rows, err := db.QueryContext(t.Context(), stmt)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
	}
	return rows.Err()
}

// TestCosmosSmoke runs each registered query against the fixture and checks that it
// returns the columns it declares.
func TestCosmosSmoke(t *testing.T) {
	db := openCosmos(t)
	m := setupCosmos(t, db)

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
		if len(vals) != 0 {
			t.Errorf("%s: expected a statement with no parameter, got %d", q.Name(), len(vals))
		}
		cs, err := cosmosColumns(t, db, query)
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
		for i, c := range cs {
			if i < len(fields) && c != fields[i].Name {
				t.Errorf("%s: column %d is %q and the field is %q", q.Name(), i, c, fields[i].Name)
			}
		}
		ran++
	}
	if ran != 10 {
		t.Errorf("expected the 10 queries the package comment names to run, %d ran", ran)
	}
	t.Logf("%d queries ran, %d not supported", ran, unsupported)
}

// TestCosmosDatabases reads the databases of the account. A database is the
// database and also the schema.
func TestCosmosDatabases(t *testing.T) {
	db := openCosmos(t)
	m := setupCosmos(t, db)

	args := dbmeta.Args{Name: cmfixture.Database}.Map()
	dbs := cosmosAll(t, dbmeta.Databases, m, db, args)
	if len(dbs) != 1 || dbs[0].Name != cmfixture.Database {
		t.Fatalf("expected the database %s, got %+v", cmfixture.Database, dbs)
	}
	if v := dbs[0]; v.Owner != "" || v.Encoding != "" || v.Access.Valid || v.Size.Valid || v.Comment.Valid || v.Tablespace.Valid {
		t.Errorf("expected a database with only a name, got %+v", v)
	}
	schemas := cosmosAll(t, dbmeta.Schemas, m, db, args)
	if len(schemas) != 1 || schemas[0].Name != cmfixture.Database {
		t.Fatalf("expected the schema %s, got %+v", cmfixture.Database, schemas)
	}
	if opts := cosmosOptions(schemas[0].Options); !strings.HasPrefix(opts["link"], "dbs/") || opts["rid"] == "" {
		t.Errorf("expected the rid and the link of the database in the options, got %+v", schemas[0].Options)
	}
	// A pattern that no database has leaves nothing, and is not an error.
	if got := cosmosAll(t, dbmeta.Databases, m, db, dbmeta.Args{Name: "no\\_such\\_database"}.Map()); len(got) != 0 {
		t.Errorf("expected no database, got %+v", got)
	}
	// The statement of both kinds is one request for the databases.
	if dbmeta.Databases.Support(m) != dbmeta.Supported || dbmeta.Schemas.Support(m) != dbmeta.Supported {
		t.Error("expected both kinds to be supported")
	}
}

// TestCosmosTables reads the containers of the fixture back through the typed API.
func TestCosmosTables(t *testing.T) {
	db := openCosmos(t)
	m := setupCosmos(t, db)

	tables := map[string]dbmeta.Table{}
	for _, v := range cosmosAll(t, dbmeta.Tables, m, db, dbmeta.Args{Schema: cmfixture.Database}.Map()) {
		tables[v.Name] = v
		if v.Catalog != "" || v.Schema != cmfixture.Database || v.Type != "container" {
			t.Errorf("table %s: expected a container of the database, got %+v", v.Name, v)
		}
		if v.Owner.Valid || v.Size.Valid || v.Rows.Valid || v.Comment.Valid || v.RowSecurity.Valid {
			t.Errorf("table %s: expected no owner, size, rows, comment or row security, got %+v", v.Name, v)
		}
	}
	if len(tables) != 4 {
		t.Fatalf("expected 4 containers, got %d: %v", len(tables), tables)
	}
	for name, want := range map[string]map[string]string{
		cmfixture.Author:   {"partition_key": "/id", "partition_key_kind": "Hash", "full_text": "true", "geospatial_type": "Geography"},
		cmfixture.Book:     {"partition_key": "/author_id", "unique_keys": "/isbn|/title+/edition"},
		cmfixture.Region:   {"partition_key": "/id"},
		cmfixture.Shipment: {"partition_key": "/region_id", "default_ttl": "2592000", "geospatial_type": "Geometry", "computed_properties": "carrier_lower"},
	} {
		got := cosmosOptions(tables[name].Options)
		for k, v := range want {
			if got[k] != v {
				t.Errorf("table %s: expected %s=%q in the options, got %q (%v)", name, k, v, got[k], tables[name].Options)
			}
		}
		if got["conflict_resolution"] != "LastWriterWins" || got["conflict_resolution_path"] != "/_ts" {
			t.Errorf("table %s: expected the default conflict resolution, got %v", name, tables[name].Options)
		}
		if !strings.HasPrefix(got["link"], "dbs/") || got["rid"] == "" {
			t.Errorf("table %s: expected the rid and the link, got %v", name, tables[name].Options)
		}
	}
	// A container with no time to live has no default_ttl, which is not the same as
	// a time to live of zero.
	if _, ok := cosmosOptions(tables[cmfixture.Book].Options)["default_ttl"]; ok {
		t.Errorf("table book: expected no default_ttl, got %v", tables[cmfixture.Book].Options)
	}

	// The filters narrow in Go, and the types filter reads the one type.
	if got := cosmosAll(t, dbmeta.Tables, m, db, dbmeta.Args{Name: "s%"}.Map()); len(got) != 1 || got[0].Name != "shipment" {
		t.Errorf("expected shipment for the pattern s%%, got %+v", got)
	}
	if got := cosmosAll(t, dbmeta.Tables, m, db, dbmeta.Args{Schema: cmfixture.Database, Types: []string{"view"}}.Map()); len(got) != 0 {
		t.Errorf("expected no view, got %+v", got)
	}
	if got := cosmosAll(t, dbmeta.Tables, m, db, dbmeta.Args{Schema: "no\\_such"}.Map()); len(got) != 0 {
		t.Errorf("expected no table in a database that is not there, got %+v", got)
	}
}

// TestCosmosIndexes reads the indexing policy of each container. A container has one
// index, which is its policy.
func TestCosmosIndexes(t *testing.T) {
	db := openCosmos(t)
	m := setupCosmos(t, db)

	indexes := map[string]dbmeta.Index{}
	for _, v := range cosmosAll(t, dbmeta.Indexes, m, db, nil) {
		indexes[v.Table] = v
		if v.Name != "indexing_policy" || v.Schema != cmfixture.Database || v.Unique || v.Primary || v.Predicate.Valid {
			t.Errorf("index of %s: expected the indexing policy, got %+v", v.Table, v)
		}
	}
	if len(indexes) != 4 {
		t.Fatalf("expected 4 indexes, got %d", len(indexes))
	}
	for name, want := range map[string]string{
		cmfixture.Author: "consistent", cmfixture.Book: "consistent",
		cmfixture.Region: "none", cmfixture.Shipment: "consistent",
	} {
		if indexes[name].Type != want {
			t.Errorf("index of %s: expected the mode %q, got %q", name, want, indexes[name].Type)
		}
	}
	book := cosmosOptions(indexes[cmfixture.Book].Options)
	if book["automatic"] != "true" || book["included_paths"] != "/title/?|/published/?" ||
		!strings.HasPrefix(book["excluded_paths"], "/*") {
		t.Errorf("index of book: expected the paths of the policy, got %v", indexes[cmfixture.Book].Options)
	}
	if got := cosmosOptions(indexes[cmfixture.Region].Options)["automatic"]; got != "false" {
		t.Errorf("index of region: expected automatic=false, got %q", got)
	}
	var policy struct {
		Composite [][]struct {
			Path  string `json:"path"`
			Order string `json:"order"`
		} `json:"compositeIndexes"`
		Spatial []struct {
			Path string `json:"path"`
		} `json:"spatialIndexes"`
	}
	if err := json.Unmarshal([]byte(indexes[cmfixture.Book].Definition.V), &policy); err != nil ||
		len(policy.Composite) != 1 || len(policy.Composite[0]) != 2 || policy.Composite[0][1].Order != "descending" {
		t.Errorf("index of book: expected a composite index in the definition, got %q (%v)", indexes[cmfixture.Book].Definition.V, err)
	}
	if err := json.Unmarshal([]byte(indexes[cmfixture.Shipment].Definition.V), &policy); err != nil ||
		len(policy.Spatial) != 1 || policy.Spatial[0].Path != "/location/*" {
		t.Errorf("index of shipment: expected a spatial index in the definition, got %q (%v)", indexes[cmfixture.Shipment].Definition.V, err)
	}

	parts := cosmosAll(t, dbmeta.PartitionedTables, m, db, nil)
	if len(parts) != 4 {
		t.Fatalf("expected 4 partitioned tables, got %+v", parts)
	}
	for _, v := range parts {
		want := map[string]string{"author": "/id", "book": "/author_id", "region": "/id", "shipment": "/region_id"}[v.Name]
		if v.Strategy != "hash" || v.Expression != want || v.Type != "container" || v.Table.Valid {
			t.Errorf("partitioned table %s: expected hash on %q, got %+v", v.Name, want, v)
		}
	}
}

// TestCosmosRoutines reads the function and the trigger of the container that the
// connection names, and checks that a connection that names none is refused.
func TestCosmosRoutines(t *testing.T) {
	db := openCosmos(t)
	m := setupCosmos(t, db)

	funcs := cosmosAll(t, dbmeta.Functions, m, db, nil)
	if len(funcs) != 1 {
		t.Fatalf("expected 1 function, got %+v", funcs)
	}
	f := funcs[0]
	if f.Name != "full_title" || f.Kind != "function" || f.Language != "javascript" || f.Schema != cmfixture.Database ||
		!strings.Contains(f.Source.V, "return title") || !strings.HasPrefix(f.ID.V, "dbs/") ||
		f.Definition.Valid || f.Owner.Valid || f.ResultType.Valid {
		t.Errorf("expected the function full_title, got %+v", f)
	}
	trigs := cosmosAll(t, dbmeta.Triggers, m, db, dbmeta.Args{Parent: "book"}.Map())
	if len(trigs) != 1 {
		t.Fatalf("expected 1 trigger, got %+v", trigs)
	}
	g := trigs[0]
	var def struct {
		ID               string `json:"id"`
		Body             string `json:"body"`
		TriggerType      string `json:"triggerType"`
		TriggerOperation string `json:"triggerOperation"`
	}
	if err := json.Unmarshal([]byte(g.Definition), &def); err != nil {
		t.Fatalf("reading the definition of the trigger: %v", err)
	}
	if g.Name != "stamp_book" || g.Table != "book" || g.Schema != cmfixture.Database || g.Enabled != "" ||
		def.TriggerType != "Pre" || def.TriggerOperation != "Create" || !strings.Contains(def.Body, "stamped") {
		t.Errorf("expected the trigger stamp_book, got %+v %+v", g, def)
	}
	if got := cosmosAll(t, dbmeta.Triggers, m, db, dbmeta.Args{Parent: "author"}.Map()); len(got) != 0 {
		t.Errorf("expected no trigger for the container author, got %+v", got)
	}

	// A connection with no container in its path has nothing to read the scripts
	// of, and the driver refuses the statement before it sends a request.
	bare := openAt(t, "cosmos", withCosmosContainer(t, os.Getenv("DBMETA_COSMOS"), ""))
	for _, q := range []dbmeta.AnyQuery{dbmeta.Functions, dbmeta.Triggers} {
		stmt, _, err := q.Build(m, nil)
		if err != nil {
			t.Fatalf("%s: building: %v", q.Name(), err)
		}
		err = cosmosDrain(t, bare, stmt)
		if err == nil || !strings.Contains(err.Error(), "container") {
			t.Errorf("%s: expected a refusal that names the container, got %v", q.Name(), err)
		}
	}
	// The statements for databases and containers need none.
	if _, err := cosmosColumns(t, bare, `SELECT * FROM "$containers"`); err != nil {
		t.Errorf("reading the containers with no container in the URL: %v", err)
	}
}

// TestCosmosRoles reads the users and the permissions of the database.
func TestCosmosRoles(t *testing.T) {
	db := openCosmos(t)
	m := setupCosmos(t, db)

	roles := map[string]dbmeta.Role{}
	for _, v := range cosmosAll(t, dbmeta.Roles, m, db, nil) {
		roles[v.Name] = v
		if v.Superuser || v.CanLogin || v.CreateRole || v.CreateDB || v.ValidUntil.Valid || v.Comment.Valid || v.ConnLimit != -1 {
			t.Errorf("role %s: expected a user with no attribute, got %+v", v.Name, v)
		}
	}
	if len(roles) != 2 || roles[cmfixture.Reader].Name == "" || roles[cmfixture.Auditor].Name == "" {
		t.Errorf("expected the users reader and auditor, got %v", roles)
	}

	grants := map[string]dbmeta.Privilege{}
	for _, v := range cosmosAll(t, dbmeta.Privileges, m, db, nil) {
		grants[v.Access.V] = v
		if v.Schema.V != cmfixture.Database || !strings.HasPrefix(v.Name, "dbs/") {
			t.Errorf("privilege %+v: expected the database and a link", v)
		}
	}
	if got := grants["reader=Read"]; got.Type != "container" || !strings.Contains(got.Name, "/colls/") {
		t.Errorf("expected the reader to read a container, got %+v", got)
	}
	if got := grants["auditor=All"]; got.Type != "container" || !strings.Contains(got.Name, "/colls/") {
		t.Errorf("expected the auditor to hold all on a container, got %+v", got)
	}
	if len(grants) != 2 {
		t.Errorf("expected 2 permissions, got %v", grants)
	}
}

// TestCosmosSettings reads the throughput that the database shares among its
// containers.
func TestCosmosSettings(t *testing.T) {
	db := openCosmos(t)
	m := setupCosmos(t, db)

	settings := cosmosAll(t, dbmeta.Settings, m, db, nil)
	if len(settings) == 0 {
		t.Fatal("expected the throughput of the database")
	}
	var found bool
	for _, v := range settings {
		if v.Name != "throughput dbmeta" {
			continue
		}
		found = true
		if v.Value.V != "400" || v.Type.V != "manual" || v.Context.V != "database" || v.Display.V != "400 RU/s" || v.Access.Valid {
			t.Errorf("expected 400 manual request units for the database, got %+v", v)
		}
	}
	if !found {
		t.Errorf("expected the setting throughput dbmeta, got %+v", settings)
	}
}

// TestCosmosUnanswered checks that every kind the model does not answer says so,
// and that the account holds no source for the ones that it does not read, so
// that each absence stays a decision. See D228.
func TestCosmosUnanswered(t *testing.T) {
	db := openCosmos(t)
	m := setupCosmos(t, db)
	answered := map[string]bool{
		"databases": true, "schemas": true, "tables": true, "indexes": true,
		"partitioned_tables": true, "functions": true, "triggers": true,
		"roles": true, "privileges": true, "settings": true,
	}
	var unanswered int
	for _, q := range dbmeta.Queries() {
		if answered[q.Name()] {
			if q.Support(m) != dbmeta.Supported {
				t.Errorf("%s: expected it to be supported on Cosmos DB", q.Name())
			}
			continue
		}
		unanswered++
		if q.Support(m) != dbmeta.NotSupported {
			t.Errorf("%s: expected it to be unsupported on Cosmos DB", q.Name())
		}
	}
	if unanswered != 55 {
		t.Errorf("expected 55 kinds that are not answered, got %d", unanswered)
	}

	// $account answers one row of ten columns, and no kind has a field for a row
	// that holds a list of settings.
	cols, err := cosmosColumns(t, db, `SELECT * FROM "$account"`)
	if err != nil || len(cols) != 10 {
		t.Errorf("expected $account to answer 10 columns, got %v, %v", cols, err)
	}
	// The stored procedures are a statement of their own, which no kind reads.
	cols, err = cosmosColumns(t, db, `SELECT * FROM "$stored_procedures"`)
	if err != nil || len(cols) != 6 {
		t.Errorf("expected $stored_procedures to answer 6 columns, got %v, %v", cols, err)
	}
}
