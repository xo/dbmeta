package test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"

	_ "gorm.io/driver/bigquery/driver"

	"github.com/xo/dbmeta"
	_ "github.com/xo/dbmeta/models/bigquery"
	bqfixture "github.com/xo/dbmeta/models/bigquery/fixture"
)

// openBigQuery returns a connection to the service named by DBMETA_BIGQUERY,
// which dbrun resolves from the places D117 names, and which holds no secret.
// The service account comes from GOOGLE_APPLICATION_CREDENTIALS, which dbrun
// sets to the key file of the service (D218).
//
// The driver is gorm.io/driver/bigquery, which dburl v0.49.0 names for the
// bigquery scheme (D154). It moves to dbimp's driver when that is tagged. See
// D220.
func openBigQuery(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("DBMETA_BIGQUERY")
	if dsn == "" {
		t.Skip("set DBMETA_BIGQUERY to run against the service")
	}
	return openAt(t, "bigquery", dsn)
}

// bigQueryState is the one fixture that every BigQuery test shares.
//
// BigQuery takes a second or more for each statement of the fixture, and it
// bills each job, so a fixture for each test is more than the tests are worth.
// The first test builds it, every test reads it, and TestMain drops it when the
// last test ends. No test changes it.
var bigQueryState struct {
	mu    sync.Mutex
	meta  *dbmeta.Meta
	err   error
	built bool
	// quota names the index steps that BigQuery refused for its quota, so the
	// fixture has no such index. See the package doc of the fixture.
	quota map[string]bool
}

// The steps that make an index, which BigQuery limits by table in a day.
const (
	searchIndexStep = "sales search index"
	vectorIndexStep = "plain vector index"
)

// hasIndex reports whether the fixture has the index that the step makes.
func hasIndex(step string) bool { return !bigQueryState.quota[step] }

// runBigQuerySteps runs the statements one at a time. A step of a teardown that
// fails is not an error when quiet is true, because the object is already gone.
func runBigQuerySteps(ctx context.Context, db *sql.DB, steps []bqfixture.Result, quiet bool) error {
	for _, s := range steps {
		err := bigQueryDrain(ctx, db, s.Query)
		if err != nil && !quiet {
			return fmt.Errorf("%s: %w\n%s", s.Name, err, s.Query)
		}
	}
	return nil
}

// shutdownBigQuery drops the fixture, if a test built it. TestMain calls it.
func shutdownBigQuery() {
	bigQueryState.mu.Lock()
	defer bigQueryState.mu.Unlock()
	if !bigQueryState.built {
		return
	}
	// DBMETA_BIGQUERY_KEEP leaves the fixture up, so that a person can write a
	// statement against it, and the next run with it set reuses that fixture.
	if os.Getenv("DBMETA_BIGQUERY_KEEP") != "" {
		return
	}
	db, err := sql.Open("bigquery", os.Getenv("DBMETA_BIGQUERY"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "dropping the BigQuery fixture: %v\n", err)
		return
	}
	defer db.Close()
	down, err := bqfixture.Everything.ResolveTeardown(dbmeta.VersionSet{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "dropping the BigQuery fixture: %v\n", err)
		return
	}
	//nolint:errcheck // a teardown is best effort, and quiet says so
	runBigQuerySteps(context.Background(), db, down, true)
}

// setupBigQuery builds the fixture, once, and returns the metadata for the
// service.
//
// It tears down first, because a run that failed part way can leave the tables
// behind, and CREATE TABLE then fails and the test reports that and not what
// went wrong.
func setupBigQuery(t *testing.T, db *sql.DB) *dbmeta.Meta {
	t.Helper()
	bigQueryState.mu.Lock()
	defer bigQueryState.mu.Unlock()
	if bigQueryState.err != nil {
		t.Fatalf("the fixture failed to build in an earlier test: %v", bigQueryState.err)
	}
	if bigQueryState.meta != nil {
		return bigQueryState.meta
	}
	ctx := context.WithoutCancel(t.Context())
	fail := func(format string, args ...any) {
		bigQueryState.err = fmt.Errorf(format, args...)
		t.Fatal(bigQueryState.err)
	}
	versions, err := dbmeta.BigQuery.Version(ctx, db)
	if err != nil {
		fail("reading the version: %v", err)
	}
	m, err := dbmeta.New(dbmeta.BigQuery, versions)
	if err != nil {
		fail("building the metadata: %v", err)
	}
	down, err := bqfixture.Everything.ResolveTeardown(versions)
	if err != nil {
		fail("resolving the teardown: %v", err)
	}
	if os.Getenv("DBMETA_BIGQUERY_KEEP") != "" {
		// A person kept the fixture of an earlier run to write against, and the
		// last table that the setup makes says it is whole.
		if tables, err := collectBigQuery(t, dbmeta.Tables, m, db, dbmeta.Args{Name: "author\\_clone"}.Map()); err == nil && len(tables) > 0 {
			bigQueryState.built, bigQueryState.meta = true, m
			return m
		}
	}
	//nolint:errcheck // a teardown before setup is best effort, and quiet says so
	runBigQuerySteps(ctx, db, down, true)
	up, err := bqfixture.Everything.ResolveSetup(versions)
	if err != nil {
		fail("resolving the setup: %v", err)
	}
	bigQueryState.built = true
	for _, step := range up {
		err := bigQueryDrain(ctx, db, step.Query)
		switch {
		case err == nil:
		case (step.Name == searchIndexStep || step.Name == vectorIndexStep) &&
			strings.Contains(err.Error(), "quotaExceeded"):
			t.Logf("BigQuery refused %s for its quota, so the fixture has none: %v", step.Name, err)
			if bigQueryState.quota == nil {
				bigQueryState.quota = map[string]bool{}
			}
			bigQueryState.quota[step.Name] = true
		default:
			fail("setup: %s: %v\n%s", step.Name, err, step.Query)
		}
	}
	t.Logf("server reports %s, fixture ran %d steps", versions, len(up))
	bigQueryState.meta = m
	return m
}

// collectBigQuery reads every row a query answers.
func collectBigQuery[T any](t *testing.T, q *dbmeta.Query[T], m *dbmeta.Meta, db *sql.DB, args map[string]any) ([]T, error) {
	t.Helper()
	var out []T
	for v, err := range q.All(t.Context(), m, db, args) {
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", q.Name(), err)
		}
		out = append(out, v)
	}
	return out, nil
}

// bigQueryAll reads every row a query answers, and stops the test on an error.
func bigQueryAll[T any](t *testing.T, q *dbmeta.Query[T], m *dbmeta.Meta, db *sql.DB, args map[string]any) []T {
	t.Helper()
	out, err := collectBigQuery(t, q, m, db, args)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// bigQueryColumns runs the statement and returns the names of its columns.
func bigQueryColumns(t *testing.T, db *sql.DB, query string, vals []any) ([]string, error) {
	t.Helper()
	rows, err := db.QueryContext(t.Context(), query, vals...)
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

// bigQueryDrain runs a statement and reads every row, and returns the error
// that either step gave.
func bigQueryDrain(ctx context.Context, db *sql.DB, stmt string) error {
	rows, err := db.QueryContext(ctx, stmt)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
	}
	return rows.Err()
}

// TestBigQueryVersion checks that the service reports no version, which is why
// the model declares no version query. See D220.
func TestBigQueryVersion(t *testing.T) {
	db := openBigQuery(t)
	versions, err := dbmeta.BigQuery.Version(t.Context(), db)
	if err != nil {
		t.Fatalf("reading the version: %v", err)
	}
	if !versions.Main().IsZero() && !versions.Main().Unknown {
		t.Errorf("expected no version, got %s", versions)
	}
	// usql runs SELECT version() for a driver that declares no version, and
	// BigQuery has no such function.
	if err := bigQueryDrain(t.Context(), db, "SELECT version()"); err == nil {
		t.Error("SELECT version(): expected BigQuery to refuse it")
	}
	t.Logf("server reports %s", versions)
}

// TestBigQuerySmoke runs each registered query against the fixture and checks
// that it returns the columns it declares.
func TestBigQuerySmoke(t *testing.T) {
	db := openBigQuery(t)
	m := setupBigQuery(t, db)

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
		cs, err := bigQueryColumns(t, db, query, vals)
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
	if ran != 18 {
		t.Errorf("expected the 18 queries the package comment names to run, %d ran", ran)
	}
	t.Logf("%d queries ran, %d not supported", ran, unsupported)
}

// TestBigQueryTables reads the relations of the fixture back through the typed
// API.
func TestBigQueryTables(t *testing.T) {
	db := openBigQuery(t)
	m := setupBigQuery(t, db)

	tables := map[string]dbmeta.Table{}
	for _, v := range bigQueryAll(t, dbmeta.Tables, m, db, nil) {
		tables[v.Name] = v
		if v.Catalog == "" || v.Schema != bqfixture.Everything.Schema {
			t.Errorf("table %s: expected the project and the dataset, got %q and %q", v.Name, v.Catalog, v.Schema)
		}
		if v.Owner.Valid || v.AccessMethod.Valid || v.RowSecurity.Valid || v.RowSecurityForced.Valid {
			t.Errorf("table %s: expected no owner, access method or row security, got %+v", v.Name, v)
		}
	}
	for name, typ := range map[string]string{
		"author": "table", "book": "table", "region": "table", "shipment": "table",
		"sales": "table", "ticket": "table", "plain": "table",
		"recent": "view", "sales_total": "materialized view",
		"author_snap": "snapshot", "author_clone": "clone", "states": "external table",
	} {
		if tables[name].Type != typ {
			t.Errorf("table %s: expected type %q, got %q", name, typ, tables[name].Type)
		}
	}
	if len(tables) != 12 {
		t.Errorf("expected 12 relations, got %d: %v", len(tables), tables)
	}
	if got := tables["author"].Comment; got.V != "people who write books" {
		t.Errorf("table author: expected its description, got %+v", got)
	}
	if got := tables["recent"].Comment; got.V != "recent books" {
		t.Errorf("view recent: expected its description, got %+v", got)
	}
	if got := tables["book"].Comment; got.Valid {
		t.Errorf("table book: expected no description, got %+v", got)
	}

	// Size and rows come from `__TABLES__`, and a view and an external table have
	// none.
	if got := tables["plain"].Rows; !got.Valid || got.V != 6000 {
		t.Errorf("table plain: expected 6000 rows, got %+v", got)
	}
	if got := tables["plain"].Size; !got.Valid || got.V <= 0 {
		t.Errorf("table plain: expected a size, got %+v", got)
	}
	if got := tables["sales"].Rows; !got.Valid || got.V != 2 {
		t.Errorf("table sales: expected 2 rows, got %+v", got)
	}
	if got := tables["book"].Rows; !got.Valid || got.V != 0 {
		t.Errorf("table book: expected 0 rows, got %+v", got)
	}
	for _, name := range []string{"recent", "states"} {
		if v := tables[name]; v.Size.Valid || v.Rows.Valid || v.Persistence.Valid {
			t.Errorf("%s: expected no size, rows or persistence, got %+v", name, v)
		}
	}
	if got := tables["author"].Persistence; got.V != "permanent" {
		t.Errorf("table author: expected permanent, got %+v", got)
	}
	if got := tables["sales"].Options.V; !strings.Contains(got, "cluster_by=region") ||
		!strings.Contains(got, "partition_expiration_days=3650.0") ||
		!strings.Contains(got, "require_partition_filter=true") ||
		strings.Contains(got, "description") {
		t.Errorf("table sales: expected the clustering and the partition options, got %q", got)
	}
	if got := tables["author_snap"].Options.V; !strings.Contains(got, "base_table=dbmeta.author") {
		t.Errorf("snapshot author_snap: expected its base table, got %q", got)
	}
	if got := tables["states"].Options.V; !strings.Contains(got, `format="CSV"`) {
		t.Errorf("external table states: expected its format, got %q", got)
	}

	// The filters.
	if got := bigQueryAll(t, dbmeta.Tables, m, db, dbmeta.Args{Name: "auth%"}.Map()); len(got) != 3 {
		t.Errorf("expected author, its clone and its snapshot, got %+v", got)
	}
	for _, v := range bigQueryAll(t, dbmeta.Tables, m, db, dbmeta.Args{Types: []string{"view", "materialized view"}}.Map()) {
		if v.Type != "view" && v.Type != "materialized view" {
			t.Errorf("expected a view or a materialized view, got %+v", v)
		}
	}
	if got := bigQueryAll(t, dbmeta.Tables, m, db, dbmeta.Args{Schema: "no\\_such\\_dataset"}.Map()); len(got) != 0 {
		t.Errorf("expected no relation in a dataset that is not the default, got %+v", got)
	}

	// Views.
	views := bigQueryAll(t, dbmeta.Views, m, db, nil)
	if len(views) != 1 || views[0].Name != "recent" || !strings.Contains(views[0].Definition.V, "published IS NOT NULL") {
		t.Fatalf("expected the one view recent, got %+v", views)
	}
	if v := views[0]; v.Comment.V != "recent books" || v.Updatable.Valid || !v.Insertable.Valid || v.Insertable.V || v.CheckOption.Valid {
		t.Errorf("view recent: expected its description, no updatable and not insertable, got %+v", v)
	}
}

// TestBigQueryColumnsAndIndexes reads the columns, the indexes and the index
// columns of the fixture.
func TestBigQueryColumnsAndIndexes(t *testing.T) {
	db := openBigQuery(t)
	m := setupBigQuery(t, db)

	columns := map[string]dbmeta.Column{}
	for _, v := range bigQueryAll(t, dbmeta.Columns, m, db, nil) {
		columns[v.Table+"."+v.Name] = v
	}
	type want struct {
		ordinal  int
		dataType string
		nullable bool
		key      bool
	}
	for name, w := range map[string]want{
		"author.author_id":     {1, "INT64", false, true},
		"author.name":          {2, "STRING", false, false},
		"author.rating":        {3, "INT64", true, false},
		"author.shade":         {4, "STRING", true, false},
		"book.book_id":         {1, "INT64", false, true},
		"book.published":       {4, "DATE", true, false},
		"region.country":       {1, "STRING", false, true},
		"region.area":          {2, "STRING", false, true},
		"plain.embedding":      {2, "ARRAY<FLOAT64>", false, false},
		"sales.sold_on":        {1, "DATE", false, false},
		"recent.title":         {2, "STRING", true, false},
		"shipment.shipment_id": {1, "INT64", false, true},
	} {
		got, ok := columns[name]
		if !ok {
			t.Errorf("column %s: not found", name)
			continue
		}
		if got.Ordinal != w.ordinal || got.DataType != w.dataType ||
			got.Nullable != w.nullable || got.PrimaryKey != w.key {
			t.Errorf("column %s: expected %+v, got ordinal %d, type %q, nullable %v, key %v",
				name, w, got.Ordinal, got.DataType, got.Nullable, got.PrimaryKey)
		}
	}
	// BigQuery reports the text NULL for a column with no default, and the
	// model turns it into an absent value. A clone and a snapshot copy the
	// default.
	if got := columns["author.shade"].Default; !got.Valid || got.V != "'red'" {
		t.Errorf("column shade: expected the default 'red', got %+v", got)
	}
	for name, c := range columns {
		if !strings.HasSuffix(name, ".shade") && c.Default.Valid {
			t.Errorf("column %s: expected no default, got %+v", name, c.Default)
		}
		if c.Collation.Valid || c.Generated.Valid || c.Storage.Valid || c.Compression.Valid || c.StatsTarget.Valid {
			t.Errorf("column %s: expected no collation, generated, storage, compression or statistics target, got %+v", name, c)
		}
		if name != "ticket.ticket_id" && c.Identity.Valid {
			t.Errorf("column %s: expected no identity, got %+v", name, c.Identity)
		}
	}
	if got := columns["ticket.ticket_id"].Identity; got.V != "always" {
		t.Errorf("column ticket_id: expected the identity always, got %+v", got)
	}
	if got := columns["author.name"].Comment; got.V != "the full name" {
		t.Errorf("column name: expected its description, got %+v", got)
	}
	if got := columns["author.rating"].Comment; got.Valid {
		t.Errorf("column rating: expected no description, got %+v", got)
	}
	if got := bigQueryAll(t, dbmeta.Columns, m, db, dbmeta.Args{Parent: "shipment"}.Map()); len(got) != 4 {
		t.Errorf("expected the four columns of shipment, got %+v", got)
	}

	// Indexes. A search index on a table below the size threshold is disabled
	// for a while, so it is not valid, and BigQuery says so.
	indexes := map[string]dbmeta.Index{}
	for _, v := range bigQueryAll(t, dbmeta.Indexes, m, db, nil) {
		indexes[v.Table+"."+v.Name] = v
	}
	wantIndexes := map[string]string{"sales.sales_search": "search", "plain.plain_vector": "vector"}
	if !hasIndex(searchIndexStep) {
		delete(wantIndexes, "sales.sales_search")
	}
	if !hasIndex(vectorIndexStep) {
		delete(wantIndexes, "plain.plain_vector")
	}
	if len(indexes) != len(wantIndexes) {
		t.Fatalf("expected %d indexes, got %+v", len(wantIndexes), indexes)
	}
	for name, typ := range wantIndexes {
		got, ok := indexes[name]
		if !ok {
			t.Errorf("index %s: not found", name)
			continue
		}
		if got.Type != typ || got.Unique || got.Primary || got.Persistence.V != "permanent" ||
			!got.Definition.Valid || !strings.HasPrefix(got.Definition.V, "CREATE ") ||
			!got.Valid.Valid || !got.Size.Valid {
			t.Errorf("index %s: expected a %s index with its definition, got %+v", name, typ, got)
		}
		if got.Predicate.Valid || got.Owner.Valid || got.Clustered.Valid || got.ConstraintType.Valid {
			t.Errorf("index %s: expected no predicate, owner, cluster or constraint, got %+v", name, got)
		}
	}
	if got := indexes["plain.plain_vector"]; hasIndex(vectorIndexStep) &&
		(got.Using.V != "IVF" || !strings.Contains(got.Options.V, "distance_type=COSINE")) {
		t.Errorf("index plain_vector: expected IVF and its distance, got %+v", got)
	}
	if got := indexes["sales.sales_search"]; hasIndex(searchIndexStep) &&
		(got.Using.Valid || !strings.Contains(got.Options.V, "analyzer=LOG_ANALYZER")) {
		t.Errorf("index sales_search: expected its analyzer and no access method, got %+v", got)
	}

	cols := bigQueryAll(t, dbmeta.IndexColumns, m, db, nil)
	if len(cols) != len(wantIndexes) {
		t.Fatalf("expected one column for each index, got %+v", cols)
	}
	for _, c := range cols {
		if c.Ordinal != 1 || c.Include || c.Descending.Valid || c.Expression.Valid {
			t.Errorf("index column %+v: expected ordinal 1 and no more", c)
		}
	}
	names := map[string]string{}
	for _, c := range cols {
		names[c.Index] = c.Name.V
	}
	if hasIndex(vectorIndexStep) && names["plain_vector"] != "embedding" ||
		hasIndex(searchIndexStep) && names["sales_search"] != "body" {
		t.Errorf("expected embedding and body, got %+v", cols)
	}
}

// TestBigQueryKeysAndConstraints reads the keys. BigQuery records a primary key
// and a foreign key, and enforces neither.
func TestBigQueryKeysAndConstraints(t *testing.T) {
	db := openBigQuery(t)
	m := setupBigQuery(t, db)

	all := map[string]dbmeta.Constraint{}
	for _, v := range bigQueryAll(t, dbmeta.Constraints, m, db, nil) {
		all[v.Name] = v
		if !v.Enforced.Valid || v.Enforced.V || v.Deferrable || v.Deferred || v.Definition.Valid {
			t.Errorf("constraint %s: expected it not enforced and not deferred, got %+v", v.Name, v)
		}
	}
	for name, typ := range map[string]string{
		"author.pk$": "primary key", "book.pk$": "primary key", "region.pk$": "primary key",
		"shipment.pk$":        "primary key",
		"book.book_author_fk": "foreign key", "shipment.shipment_region_fk": "foreign key",
	} {
		if all[name].Type != typ {
			t.Errorf("constraint %s: expected %q, got %+v", name, typ, all[name])
		}
	}
	for _, v := range all {
		if v.Type != "primary key" && v.Type != "foreign key" {
			t.Errorf("constraint %s: expected a primary key or a foreign key, got %q", v.Name, v.Type)
		}
	}

	type col struct {
		name, foreignTable, foreignName string
		ordinal                         int64
	}
	byConstraint := map[string][]col{}
	for _, v := range bigQueryAll(t, dbmeta.ConstraintColumns, m, db, nil) {
		byConstraint[v.Constraint] = append(byConstraint[v.Constraint], col{v.Name, v.ForeignTable.V, v.ForeignName.V, v.Ordinal})
		if v.ForeignTable.Valid != strings.HasSuffix(v.Constraint, "_fk") {
			t.Errorf("constraint column %+v: only a foreign key points at a table", v)
		}
	}
	for name, want := range map[string][]col{
		"author.pk$":                  {{"author_id", "", "", 1}},
		"region.pk$":                  {{"country", "", "", 1}, {"area", "", "", 2}},
		"book.book_author_fk":         {{"author_id", "author", "author_id", 1}},
		"shipment.shipment_region_fk": {{"country", "region", "country", 1}, {"area", "region", "area", 2}},
	} {
		got := byConstraint[name]
		if fmt.Sprint(got) != fmt.Sprint(want) {
			t.Errorf("constraint %s: expected %v, got %v", name, want, got)
		}
	}
}

// TestBigQueryRoutines reads the routines of the fixture.
func TestBigQueryRoutines(t *testing.T) {
	db := openBigQuery(t)
	m := setupBigQuery(t, db)

	functions := map[string]dbmeta.Function{}
	for _, v := range bigQueryAll(t, dbmeta.Functions, m, db, nil) {
		functions[v.Name] = v
		if v.Owner.Valid || v.Access.Valid || v.Prosrc.Valid || v.Leakproof || !v.Definition.Valid ||
			!strings.HasPrefix(v.Definition.V, "CREATE ") || v.Parallel != "" {
			t.Errorf("function %s: expected the definition and no owner, access or prosrc, got %+v", v.Name, v)
		}
	}
	for name, w := range map[string]struct{ kind, result, args, language, volatility string }{
		"double_it":    {"function", "INT64", "INT64", "sql", ""},
		"shout":        {"function", "STRING", "STRING", "javascript", "immutable"},
		"recent_books": {"table function", "", "DATE", "sql", ""},
		"addup":        {"procedure", "", "INT64, " + "INT64, INT64", "sql", ""},
	} {
		got, ok := functions[name]
		if !ok {
			t.Errorf("function %s: not found", name)
			continue
		}
		if got.Kind != w.kind || got.ResultType.V != w.result || got.ArgTypes.V != w.args ||
			got.Language != w.language || got.Volatility != w.volatility {
			t.Errorf("function %s: expected %+v, got %+v", name, w, got)
		}
	}
	if len(functions) != 4 {
		t.Errorf("expected four routines that are not aggregates, got %v", functions)
	}
	if got := functions["double_it"]; got.Source.V != "x * 2" || got.Comment.V != "twice the argument" {
		t.Errorf("function double_it: expected its body and its description, got %+v", got)
	}
	if got := functions["recent_books"].ResultType; got.Valid {
		t.Errorf("table function recent_books: expected no result type, got %+v", got)
	}
	aggregates := bigQueryAll(t, dbmeta.Aggregates, m, db, nil)
	if len(aggregates) != 1 || aggregates[0].Name != "my_sum" || aggregates[0].Kind != "aggregate function" ||
		aggregates[0].ResultType.V != "INT64" || aggregates[0].Source.V != "SUM(x)" {
		t.Errorf("expected the one aggregate my_sum, got %+v", aggregates)
	}

	type param struct {
		name, mode, dataType string
		ordinal              int64
	}
	params := map[string][]param{}
	for _, v := range bigQueryAll(t, dbmeta.RoutineParameters, m, db, nil) {
		params[v.Routine] = append(params[v.Routine], param{v.Name.V, v.Mode, v.DataType, v.Ordinal})
		if v.Default.Valid {
			t.Errorf("parameter %s of %s: expected no default, got %+v", v.Name.V, v.Routine, v.Default)
		}
	}
	for name, want := range map[string][]param{
		"addup":     {{"a", "in", "INT64", 1}, {"b", "in", "INT64", 2}, {"c", "out", "INT64", 3}},
		"double_it": {{"", "return", "INT64", 0}, {"x", "in", "INT64", 1}},
		"my_sum":    {{"", "return", "INT64", 0}, {"x", "in", "INT64", 1}},
	} {
		if fmt.Sprint(params[name]) != fmt.Sprint(want) {
			t.Errorf("routine %s: expected %v, got %v", name, want, params[name])
		}
	}
}

// TestBigQueryCommentsAndPartitions reads the descriptions, the partitioned
// tables and the partitions.
func TestBigQueryCommentsAndPartitions(t *testing.T) {
	db := openBigQuery(t)
	m := setupBigQuery(t, db)

	comments := map[string]string{}
	for _, v := range bigQueryAll(t, dbmeta.Comments, m, db, nil) {
		comments[v.Type+" "+v.Name] = v.Comment
	}
	want := map[string]string{
		"table author":       "people who write books",
		"view recent":        "recent books",
		"function double_it": "twice the argument",
	}
	for k, v := range want {
		if comments[k] != v {
			t.Errorf("comment on %s: expected %q, got %q", k, v, comments[k])
		}
	}
	// A clone and a snapshot copy the description of the table.
	if comments["clone author_clone"] != "people who write books" || comments["snapshot author_snap"] != "people who write books" {
		t.Errorf("expected the description on the clone and on the snapshot, got %v", comments)
	}

	pts := bigQueryAll(t, dbmeta.PartitionedTables, m, db, nil)
	if len(pts) != 1 {
		t.Fatalf("expected the one partitioned table sales, got %+v", pts)
	}
	if v := pts[0]; v.Name != "sales" || v.Type != "table" || v.Strategy != "range" || v.Expression != "sold_on" ||
		v.Owner != "" || v.Parent != "" || v.Table.Valid || !v.TotalSize.Valid || v.TotalSize.V <= 0 {
		t.Errorf("expected sales, partitioned by sold_on, got %+v", v)
	}

	parts := bigQueryAll(t, dbmeta.Partitions, m, db, nil)
	if len(parts) != 2 {
		t.Fatalf("expected the two days of sales, got %+v", parts)
	}
	for i, id := range []string{"20260101", "20260102"} {
		v := parts[i]
		if v.Table != "sales" || v.Partition != "sales$"+id || v.Bound.V != id || v.Type != "partition" ||
			v.Partitioned || v.DetachPending || v.Constraint.Valid {
			t.Errorf("partition %s: expected sales$%s, got %+v", id, id, v)
		}
	}
	if got := bigQueryAll(t, dbmeta.Partitions, m, db, dbmeta.Args{Name: "%20260102"}.Map()); len(got) != 1 {
		t.Errorf("expected one partition for the name filter, got %+v", got)
	}
}

// TestBigQueryDatabasesAndUser reads the project and the principal.
func TestBigQueryDatabasesAndUser(t *testing.T) {
	db := openBigQuery(t)
	m := setupBigQuery(t, db)

	dbs := bigQueryAll(t, dbmeta.Databases, m, db, nil)
	if len(dbs) != 1 || dbs[0].Name == "" || dbs[0].Owner != "" || dbs[0].Size.Valid {
		t.Fatalf("expected the one project that the session runs in, got %+v", dbs)
	}
	tables := bigQueryAll(t, dbmeta.Tables, m, db, nil)
	if tables[0].Catalog != dbs[0].Name {
		t.Errorf("expected the catalog of a table to be the project %q, got %q", dbs[0].Name, tables[0].Catalog)
	}
	if got := bigQueryAll(t, dbmeta.Databases, m, db, dbmeta.Args{Name: "no\\_such\\_project"}.Map()); len(got) != 0 {
		t.Errorf("expected no project for a name that does not match, got %+v", got)
	}
	user, ok, err := dbmeta.First(dbmeta.CurrentUser.All(t.Context(), m, db, nil))
	if err != nil || !ok || !strings.Contains(user.Name, "@") || user.Session.Valid {
		t.Errorf("expected the IAM principal and no session, got %+v, %v, %v", user, ok, err)
	}
}

// TestBigQuerySchemas reads the dataset of the connection, its options and its
// grants. The principals need roles/bigquery.metadataViewer on the project for
// SCHEMATA, so a refusal skips the test. See D220.
func TestBigQuerySchemas(t *testing.T) {
	db := openBigQuery(t)
	m := setupBigQuery(t, db)
	ctx := t.Context()

	schemas, err := collectBigQuery(t, dbmeta.Schemas, m, db, nil)
	if err != nil {
		if strings.Contains(err.Error(), "accessDenied") {
			t.Skipf("the principal cannot read SCHEMATA: %v", err)
		}
		t.Fatalf("reading the schemas: %v", err)
	}
	tables := bigQueryAll(t, dbmeta.Tables, m, db, nil)
	if len(schemas) != 1 || schemas[0].Name != tables[0].Schema || schemas[0].Catalog != tables[0].Catalog {
		t.Fatalf("expected the one dataset of the connection, got %+v", schemas)
	}
	s := schemas[0]
	if s.Owner != "" || s.Access.Valid || !s.Comment.Valid || s.Comment.V == "" ||
		!s.Options.Valid || !strings.Contains(s.Options.V, "location=") ||
		strings.Contains(s.Options.V, "description=") {
		t.Errorf("expected the description as the comment and the other options, got %+v", s)
	}
	cur, ok, err := dbmeta.First(dbmeta.CurrentSchema.All(ctx, m, db, nil))
	if err != nil || !ok || cur.Name != s.Name {
		t.Errorf("expected the current schema to be %q, got %+v, %v, %v", s.Name, cur, ok, err)
	}
	if got := bigQueryAll(t, dbmeta.Schemas, m, db, dbmeta.Args{Name: "no\\_such\\_dataset"}.Map()); len(got) != 0 {
		t.Errorf("expected no dataset for a name that does not match, got %+v", got)
	}

	// The grants on the dataset are IAM bindings. The administrator owns the
	// dataset, so its own row is there, and no table holds a grant of its own.
	privs := bigQueryAll(t, dbmeta.Privileges, m, db, nil)
	user, _, _ := dbmeta.First(dbmeta.CurrentUser.All(ctx, m, db, nil))
	var own bool
	for _, p := range privs {
		if p.Name != s.Name || p.Type != "schema" || p.Schema.Valid || !p.Access.Valid {
			t.Errorf("expected a grant on the dataset, got %+v", p)
		}
		if strings.Contains(p.Access.V, user.Name+"=roles/") {
			own = true
		}
	}
	if !own {
		t.Errorf("expected a grant to %q on the dataset, got %+v", user.Name, privs)
	}
}

// TestBigQueryUnanswered checks the kinds that the model does not answer, and
// the views that it does not read, so that each absence stays a decision. See
// D220.
func TestBigQueryUnanswered(t *testing.T) {
	db := openBigQuery(t)
	m := setupBigQuery(t, db)
	ctx := t.Context()
	for _, q := range []dbmeta.AnyQuery{
		dbmeta.Sequences, dbmeta.Triggers,
		dbmeta.Policies, dbmeta.NotNulls, dbmeta.Roles,
		dbmeta.ColumnPrivileges, dbmeta.Types, dbmeta.EnumValues, dbmeta.Inherits,
		dbmeta.ForeignTables, dbmeta.Settings, dbmeta.Tablespaces,
	} {
		if q.Support(m) != dbmeta.NotSupported {
			t.Errorf("%s: expected it to be unsupported on BigQuery", q.Name())
		}
	}

	// TABLE_STORAGE is a view of the region that the project must enable, and
	// the model does not read it. The other views that are named in D220 are not
	// views of this service, so they answer not found. If one of them starts to
	// answer, a query that reads it is possible and the model must change.
	for stmt, want := range map[string]string{
		"SELECT table_name FROM `region-us`.INFORMATION_SCHEMA.TABLE_STORAGE": "enable",
		"SELECT * FROM `region-us`.INFORMATION_SCHEMA.ROW_ACCESS_POLICIES":    "notFound",
		"SELECT * FROM `region-us`.INFORMATION_SCHEMA.DATA_POLICIES":          "notFound",
		"SELECT * FROM `region-us`.INFORMATION_SCHEMA.CONNECTIONS":            "notFound",
		"SELECT * FROM `region-us`.INFORMATION_SCHEMA.LINKED_DATASETS":        "notFound",
	} {
		err := bigQueryDrain(ctx, db, stmt)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: expected an error with %q, got %v", stmt, want, err)
		}
	}
	// The grants of an object are in OBJECT_PRIVILEGES, which answers a query
	// that names one object and refuses every other.
	err := bigQueryDrain(ctx, db, "SELECT grantee FROM `region-us`.INFORMATION_SCHEMA.OBJECT_PRIVILEGES")
	if err == nil || !strings.Contains(err.Error(), "object_name") {
		t.Errorf("expected OBJECT_PRIVILEGES to ask for an object name, got %v", err)
	}
	// The session has no default dataset name that a statement can read.
	var isNull bool
	if err := db.QueryRowContext(ctx, "SELECT @@dataset_id IS NULL").Scan(&isNull); err != nil || !isNull {
		t.Errorf("expected @@dataset_id to be NULL, got %v, %v", isNull, err)
	}
}
