package test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	_ "github.com/databricks/databricks-sql-go"
	dbsqllog "github.com/databricks/databricks-sql-go/logger"

	"github.com/xo/dbmeta"
	_ "github.com/xo/dbmeta/models/databricks"
	dbxfixture "github.com/xo/dbmeta/models/databricks/fixture"
)

// init turns the log of the driver off. The driver writes the text of an error to
// its log, and a connection error can hold the host and the token that the
// driver read, so the log must never reach a test run.
func init() {
	if err := dbsqllog.SetLogLevel("disabled"); err != nil {
		panic("turning the log of the Databricks driver off: " + err.Error())
	}
}

// openDatabricks returns a connection to the workspace named by
// DBMETA_DATABRICKS, which dbrun resolves from the places D117 names. The value
// holds a token and no test prints it.
//
// The driver is github.com/databricks/databricks-sql-go, which dburl v0.49.0
// names for the databricks scheme (D154). It moves to dbimp's driver when that
// is tagged. See D224.
func openDatabricks(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("DBMETA_DATABRICKS")
	if dsn == "" {
		t.Skip("set DBMETA_DATABRICKS to run against the workspace")
	}
	return openDatabricksAt(t, dsn)
}

// openDatabricksAt connects, and waits for the warehouse to start. A warehouse
// that has stopped needs up to a minute for its first statement, and the daily
// quota of the free edition makes a second run a waste, so the wait is long and
// the attempts are few.
func openDatabricksAt(t *testing.T, dsn string) *sql.DB {
	t.Helper()
	db, err := sql.Open("databricks", dsn)
	if err != nil {
		t.Fatal("opening databricks: the connection string is not valid")
	}
	t.Cleanup(func() { db.Close() })
	db.SetMaxOpenConns(2)
	var last error
	for attempt := range 4 {
		ctx, cancel := context.WithTimeout(t.Context(), 90*time.Second)
		last = db.PingContext(ctx)
		cancel()
		if last == nil {
			return db
		}
		t.Logf("waiting for the warehouse, attempt %d", attempt+1)
		time.Sleep(10 * time.Second)
	}
	t.Fatalf("connecting to the warehouse: %v", last)
	return nil
}

// databricksState is the one fixture that every Databricks test shares.
//
// Each statement of the fixture is a call to a warehouse that bills by the
// second and has a daily quota, so a fixture for each test is more than the
// tests are worth. The first test builds it, every test reads it, and TestMain
// drops it when the last test ends. No test changes it.
var databricksState struct {
	mu    sync.Mutex
	meta  *dbmeta.Meta
	err   error
	built bool
}

// runDatabricksSteps runs the statements one at a time. A step of a teardown
// that fails is not an error when quiet is true, because the object is already
// gone.
func runDatabricksSteps(ctx context.Context, db *sql.DB, steps []dbxfixture.Result, quiet bool) error {
	for _, s := range steps {
		err := databricksDrain(ctx, db, s.Query)
		if err != nil && !quiet {
			return fmt.Errorf("%s: %w\n%s", s.Name, err, s.Query)
		}
	}
	return nil
}

// shutdownDatabricks drops the fixture, if a test built it. TestMain calls it.
func shutdownDatabricks() {
	databricksState.mu.Lock()
	defer databricksState.mu.Unlock()
	if !databricksState.built {
		return
	}
	// DBMETA_DATABRICKS_KEEP leaves the fixture up, so that a person can write a
	// statement against it, and the next run with it set reuses that fixture.
	if os.Getenv("DBMETA_DATABRICKS_KEEP") != "" {
		return
	}
	db, err := sql.Open("databricks", os.Getenv("DBMETA_DATABRICKS"))
	if err != nil {
		fmt.Fprintln(os.Stderr, "dropping the Databricks fixture: the connection string is not valid")
		return
	}
	defer db.Close()
	down, err := dbxfixture.Everything.ResolveTeardown(dbmeta.VersionSet{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "dropping the Databricks fixture: %v\n", err)
		return
	}
	//nolint:errcheck // a teardown is best effort, and quiet says so
	runDatabricksSteps(context.Background(), db, down, true)
}

// setupDatabricks builds the fixture, once, and returns the metadata for the
// workspace.
//
// It tears down first, because a run that failed part way can leave the tables
// behind, and CREATE TABLE then fails and the test reports that and not what
// went wrong.
func setupDatabricks(t *testing.T, db *sql.DB) *dbmeta.Meta {
	t.Helper()
	databricksState.mu.Lock()
	defer databricksState.mu.Unlock()
	if databricksState.err != nil {
		t.Fatalf("the fixture failed to build in an earlier test: %v", databricksState.err)
	}
	if databricksState.meta != nil {
		return databricksState.meta
	}
	ctx := context.WithoutCancel(t.Context())
	fail := func(format string, args ...any) {
		databricksState.err = fmt.Errorf(format, args...)
		t.Fatal(databricksState.err)
	}
	versions, err := dbmeta.Databricks.Version(ctx, db)
	if err != nil {
		fail("reading the version: %v", err)
	}
	m, err := dbmeta.New(dbmeta.Databricks, versions)
	if err != nil {
		fail("building the metadata: %v", err)
	}
	if os.Getenv("DBMETA_DATABRICKS_KEEP") != "" {
		// A person kept the fixture of an earlier run to write against, and the
		// last table that the setup makes says it is whole.
		if tables, err := collectDatabricks(t, dbmeta.Tables, m, db, dbmeta.Args{Name: "author\\_clone"}.Map()); err == nil && len(tables) > 0 {
			databricksState.built, databricksState.meta = true, m
			return m
		}
	}
	down, err := dbxfixture.Everything.ResolveTeardown(versions)
	if err != nil {
		fail("resolving the teardown: %v", err)
	}
	//nolint:errcheck // a teardown before setup is best effort, and quiet says so
	runDatabricksSteps(ctx, db, down, true)
	up, err := dbxfixture.Everything.ResolveSetup(versions)
	if err != nil {
		fail("resolving the setup: %v", err)
	}
	databricksState.built = true
	if err := runDatabricksSteps(ctx, db, up, false); err != nil {
		fail("setup: %v", err)
	}
	t.Logf("server reports %s, fixture ran %d steps", versions, len(up))
	databricksState.meta = m
	return m
}

// collectDatabricks reads every row a query answers.
func collectDatabricks[T any](t *testing.T, q *dbmeta.Query[T], m *dbmeta.Meta, db *sql.DB, args map[string]any) ([]T, error) {
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

// databricksAll reads every row a query answers, and stops the test on an error.
func databricksAll[T any](t *testing.T, q *dbmeta.Query[T], m *dbmeta.Meta, db *sql.DB, args map[string]any) []T {
	t.Helper()
	out, err := collectDatabricks(t, q, m, db, args)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// databricksColumns runs the statement and returns the names of its columns.
func databricksColumns(t *testing.T, db *sql.DB, query string, vals []any) ([]string, error) {
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

// databricksDrain runs a statement and reads every row, and returns the error
// that either step gave.
func databricksDrain(ctx context.Context, db *sql.DB, stmt string) error {
	rows, err := db.QueryContext(ctx, stmt)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
	}
	return rows.Err()
}

// TestDatabricksVersion checks the release that the model reads, which is the
// release of the SQL channel. See D224.
func TestDatabricksVersion(t *testing.T) {
	db := openDatabricks(t)
	versions, err := dbmeta.Databricks.Version(t.Context(), db)
	if err != nil {
		t.Fatalf("reading the version: %v", err)
	}
	v := versions.Main()
	if v.Unknown || len(v.Parts) != 2 || v.Parts[0] < 2026 {
		t.Errorf("expected a release such as 2026.38, got %s", versions)
	}
	if !strings.HasPrefix(versions.Display, "Databricks 20") {
		t.Errorf("expected the display line to name the release, got %q", versions.Display)
	}
	// usql declares no version query for Databricks, so it runs SELECT version(),
	// which answers the release of Spark and a hash. The two are different
	// numbers, and the model reads the first.
	var spark string
	if err := db.QueryRowContext(t.Context(), "SELECT version()").Scan(&spark); err != nil {
		t.Fatalf("SELECT version(): %v", err)
	}
	if major, _, _ := strings.Cut(spark, "."); len(major) != 1 {
		t.Errorf("SELECT version(): expected the release of Spark, got %q", spark)
	}
	t.Logf("server reports %s, and Spark %s", versions, spark)
}

// TestDatabricksSmoke runs each registered query against the fixture and
// checks that it returns the columns it declares.
func TestDatabricksSmoke(t *testing.T) {
	db := openDatabricks(t)
	m := setupDatabricks(t, db)

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
		cs, err := databricksColumns(t, db, query, vals)
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
	if ran != 17 {
		t.Errorf("expected the 17 queries the package comment names to run, %d ran", ran)
	}
	t.Logf("%d queries ran, %d not supported", ran, unsupported)
}

// TestDatabricksTables reads the relations of the fixture back through the
// typed API.
func TestDatabricksTables(t *testing.T) {
	db := openDatabricks(t)
	m := setupDatabricks(t, db)

	tables := map[string]dbmeta.Table{}
	for _, v := range databricksAll(t, dbmeta.Tables, m, db, nil) {
		tables[v.Name] = v
		if v.Catalog == "" || v.Schema != dbxfixture.Everything.Schema {
			t.Errorf("table %s: expected the catalog and the schema, got %q and %q", v.Name, v.Catalog, v.Schema)
		}
		if !v.Owner.Valid || v.Size.Valid || v.Rows.Valid || v.RowSecurityForced.Valid {
			t.Errorf("table %s: expected an owner and no size, rows or forced row security, got %+v", v.Name, v)
		}
	}
	for name, typ := range map[string]string{
		"author": "table", "book": "table", "region": "table", "shipment": "table",
		"sales": "table", "ticket": "table", "clicks": "table", "author_clone": "table",
		"recent": "view",
	} {
		if tables[name].Type != typ {
			t.Errorf("table %s: expected type %q, got %q", name, typ, tables[name].Type)
		}
	}
	if len(tables) != 9 {
		t.Errorf("expected 9 relations, got %d: %v", len(tables), tables)
	}
	if got := tables["author"].Comment; got.V != "people who write books" {
		t.Errorf("table author: expected its comment, got %+v", got)
	}
	if got := tables["recent"].Comment; got.V != "recent books" {
		t.Errorf("view recent: expected its comment, got %+v", got)
	}
	if got := tables["book"].Comment; got.Valid {
		t.Errorf("table book: expected no comment, got %+v", got)
	}
	for name, v := range tables {
		switch {
		case name == "recent":
			if v.Persistence.Valid || v.AccessMethod.Valid {
				t.Errorf("view recent: expected no persistence and no format, got %+v", v)
			}
		case v.Persistence.V != "permanent" || v.AccessMethod.V != "delta":
			t.Errorf("table %s: expected permanent and delta, got %+v", name, v)
		}
	}
	// A tag is in the options. A table with no tag has no options, because the
	// storage path of a managed table is empty.
	if got := tables["author"].Options; got.V != "tag.team=books" {
		t.Errorf("table author: expected its tag in the options, got %+v", got)
	}
	if got := tables["book"].Options; got.Valid {
		t.Errorf("table book: expected no options, got %+v", got)
	}
	// A row filter is row security.
	for name, v := range tables {
		if want := name == "sales"; v.RowSecurity.V != want || !v.RowSecurity.Valid {
			t.Errorf("table %s: expected row security %v, got %+v", name, want, v.RowSecurity)
		}
	}

	// The filters. A clone is a table, so the pattern finds both.
	if got := databricksAll(t, dbmeta.Tables, m, db, dbmeta.Args{Name: "auth%"}.Map()); len(got) != 2 {
		t.Errorf("expected author and its clone, got %+v", got)
	}
	if got := databricksAll(t, dbmeta.Tables, m, db, dbmeta.Args{Types: []string{"view"}}.Map()); len(got) != 1 || got[0].Name != "recent" {
		t.Errorf("expected the one view, got %+v", got)
	}
	if got := databricksAll(t, dbmeta.Tables, m, db, dbmeta.Args{Schema: "no\\_such\\_schema"}.Map()); len(got) != 0 {
		t.Errorf("expected no relation in a schema that does not exist, got %+v", got)
	}
	// information_schema is a schema of every catalog, and WithSystem lists it.
	var system int
	for _, v := range databricksAll(t, dbmeta.Tables, m, db, dbmeta.Args{WithSystem: true}.Map()) {
		if v.Schema == "information_schema" {
			system++
		}
	}
	if system < 20 {
		t.Errorf("expected the views of information_schema with WithSystem, got %d", system)
	}

	views := databricksAll(t, dbmeta.Views, m, db, nil)
	if len(views) != 1 || views[0].Name != "recent" || !strings.Contains(views[0].Definition.V, "published IS NOT NULL") {
		t.Fatalf("expected the one view recent, got %+v", views)
	}
	if v := views[0]; v.Comment.V != "recent books" || v.CheckOption.Valid || !v.Updatable.Valid || v.Updatable.V || !v.Insertable.Valid || v.Insertable.V {
		t.Errorf("view recent: expected its comment, no check option and not updatable, got %+v", v)
	}
}

// TestDatabricksColumns reads the columns of the fixture, and the facts that
// INFORMATION_SCHEMA does not report. See D224.
func TestDatabricksColumns(t *testing.T) {
	db := openDatabricks(t)
	m := setupDatabricks(t, db)

	columns := map[string]dbmeta.Column{}
	for _, v := range databricksAll(t, dbmeta.Columns, m, db, nil) {
		columns[v.Table+"."+v.Name] = v
	}
	type want struct {
		ordinal  int
		dataType string
		nullable bool
		key      bool
	}
	for name, w := range map[string]want{
		"author.author_id":     {1, "bigint", false, true},
		"author.name":          {2, "string", false, false},
		"author.rating":        {3, "int", true, false},
		"author.shade":         {4, "string", true, false},
		"book.book_id":         {1, "bigint", false, true},
		"book.published":       {4, "date", true, false},
		"region.country":       {1, "string", false, true},
		"region.area":          {2, "string", false, true},
		"clicks.at":            {3, "timestamp", true, false},
		"sales.sold_on":        {3, "date", false, false},
		"recent.title":         {2, "string", false, false},
		"shipment.shipment_id": {1, "bigint", false, true},
		"author_clone.name":    {2, "string", false, false},
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
	if got := columns["author.name"].Comment; got.V != "the full name" {
		t.Errorf("column name: expected its comment, got %+v", got)
	}
	if got := columns["author.rating"].Comment; got.Valid {
		t.Errorf("column rating: expected no comment, got %+v", got)
	}
	if got := databricksAll(t, dbmeta.Columns, m, db, dbmeta.Args{Parent: "shipment"}.Map()); len(got) != 4 {
		t.Errorf("expected the four columns of shipment, got %+v", got)
	}

	// INFORMATION_SCHEMA reports no default, no identity and no generated
	// column, although the fixture makes one of each, and only SHOW CREATE TABLE
	// has them. The model passes on what the catalog says, so each is absent.
	// If one of these starts to answer, the fields say so and the model must
	// change.
	for name, c := range columns {
		if c.Default.Valid || c.Identity.Valid || c.Generated.Valid || c.Collation.Valid ||
			c.Storage.Valid || c.Compression.Valid || c.StatsTarget.Valid {
			t.Errorf("column %s: expected none of the fields that the catalog does not report, got %+v", name, c)
		}
	}
	for _, name := range []string{"author.shade", "ticket.ticket_id", "ticket.slug"} {
		if _, ok := columns[name]; !ok {
			t.Errorf("column %s: not found, so the fixture does not hold what the test needs", name)
		}
	}
	ctx := t.Context()
	var text string
	if err := db.QueryRowContext(ctx, "SHOW CREATE TABLE ticket").Scan(&text); err != nil ||
		!strings.Contains(text, "GENERATED ALWAYS AS IDENTITY") || !strings.Contains(text, "GENERATED ALWAYS AS ( lower(note) )") {
		t.Errorf("expected SHOW CREATE TABLE to hold the identity column and the generated column, got %q, %v", text, err)
	}
	if err := db.QueryRowContext(ctx, "SHOW CREATE TABLE author").Scan(&text); err != nil || !strings.Contains(text, "DEFAULT 'red'") {
		t.Errorf("expected SHOW CREATE TABLE to hold the default, got %q, %v", text, err)
	}
	var identity string
	if err := db.QueryRowContext(ctx, "SELECT is_identity FROM information_schema.columns WHERE table_name = 'ticket' AND column_name = 'ticket_id'").Scan(&identity); err != nil || identity != "NO" {
		t.Errorf("expected COLUMNS to say NO for the identity column, got %q, %v", identity, err)
	}
}

// TestDatabricksConstraints reads the keys of the fixture, and the check
// constraint that no relation lists.
func TestDatabricksConstraints(t *testing.T) {
	db := openDatabricks(t)
	m := setupDatabricks(t, db)

	type con struct{ table, typ, definition string }
	got := map[string]con{}
	for _, v := range databricksAll(t, dbmeta.Constraints, m, db, nil) {
		got[v.Name] = con{v.Table, v.Type, v.Definition.V}
		if v.Enforced.V || !v.Enforced.Valid || v.Comment.Valid || !v.Deferrable || !v.Deferred {
			t.Errorf("constraint %s: expected one that is not enforced, with no comment, and deferrable as the catalog says, got %+v", v.Name, v)
		}
	}
	want := map[string]con{
		"author_pk":          {"author", "primary key", "PRIMARY KEY (author_id)"},
		"author_clone_pk":    {"author_clone", "primary key", "PRIMARY KEY (author_id)"},
		"book_pk":            {"book", "primary key", "PRIMARY KEY (book_id)"},
		"book_author_fk":     {"book", "foreign key", "FOREIGN KEY (author_id) REFERENCES dbmeta.author (author_id)"},
		"region_pk":          {"region", "primary key", "PRIMARY KEY (country, area)"},
		"shipment_pk":        {"shipment", "primary key", "PRIMARY KEY (shipment_id)"},
		"shipment_region_fk": {"shipment", "foreign key", "FOREIGN KEY (country, area) REFERENCES dbmeta.region (country, area)"},
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("expected %v, got %v", want, got)
	}

	// The check constraint of book is a table property, and no relation lists
	// it.
	rows, err := db.QueryContext(t.Context(), "SHOW TBLPROPERTIES book")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var found bool
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			t.Fatal(err)
		}
		if key == "delta.constraints.title_not_empty" && value == "title <> ''" {
			found = true
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Error("expected SHOW TBLPROPERTIES to hold the check constraint title_not_empty")
	}
	var checks int
	if err := db.QueryRowContext(t.Context(), "SELECT COUNT(*) FROM information_schema.check_constraints").Scan(&checks); err != nil || checks != 0 {
		t.Errorf("expected CHECK_CONSTRAINTS to be empty, got %d, %v", checks, err)
	}

	type column struct {
		constraint, name, foreign string
		ordinal                   int64
	}
	var cols []column
	for _, v := range databricksAll(t, dbmeta.ConstraintColumns, m, db, dbmeta.Args{Parent: "shipment"}.Map()) {
		cols = append(cols, column{v.Constraint, v.Name, v.ForeignTable.V + "." + v.ForeignName.V, v.Ordinal})
	}
	wantCols := []column{
		{"shipment_pk", "shipment_id", ".", 1},
		{"shipment_region_fk", "country", "region.country", 1},
		{"shipment_region_fk", "area", "region.area", 2},
	}
	if fmt.Sprint(cols) != fmt.Sprint(wantCols) {
		t.Errorf("expected %v, got %v", wantCols, cols)
	}
}

// TestDatabricksRoutines reads the routines of the fixture.
func TestDatabricksRoutines(t *testing.T) {
	db := openDatabricks(t)
	m := setupDatabricks(t, db)

	functions := map[string]dbmeta.Function{}
	for _, v := range databricksAll(t, dbmeta.Functions, m, db, nil) {
		functions[v.Name] = v
		if !v.Owner.Valid || v.Definition.Valid || v.Prosrc.Valid || v.Leakproof || v.Parallel != "" || v.ID.V != v.Name {
			t.Errorf("function %s: expected an owner and no definition or prosrc, got %+v", v.Name, v)
		}
	}
	for name, w := range map[string]struct{ kind, result, args, language, volatility, security string }{
		"double_it":     {"function", "BIGINT", "bigint", "sql", "immutable", "definer"},
		"addup":         {"function", "INT", "int, int", "sql", "immutable", "definer"},
		"recent_books":  {"function", "(book_id BIGINT, title STRING)", "date", "sql", "immutable", "definer"},
		"shout":         {"function", "STRING", "string", "python", "", "definer"},
		"bump":          {"procedure", "", "int, int", "sql", "", "invoker"},
		"region_filter": {"function", "BOOLEAN", "string", "sql", "immutable", "definer"},
		"mask_name":     {"function", "STRING", "string", "sql", "immutable", "definer"},
	} {
		got, ok := functions[name]
		if !ok {
			t.Errorf("function %s: not found", name)
			continue
		}
		if got.Kind != w.kind || got.ResultType.V != w.result || got.ArgTypes.V != w.args ||
			got.Language != w.language || got.Volatility != w.volatility || got.Security != w.security {
			t.Errorf("function %s: expected %+v, got %+v", name, w, got)
		}
	}
	if len(functions) != 7 {
		t.Errorf("expected seven routines, got %v", functions)
	}
	if got := functions["double_it"]; got.Source.V != "x * 2" || got.Comment.V != "twice the argument" {
		t.Errorf("function double_it: expected its body and its comment, got %+v", got)
	}
	if got := functions["bump"].ResultType; got.Valid {
		t.Errorf("procedure bump: expected no result type, got %+v", got)
	}
	if got := functions["shout"].Source.V; got != "return s.upper()" {
		t.Errorf("function shout: expected its body, got %q", got)
	}

	type param struct {
		name, mode, dataType, def string
		ordinal                   int64
	}
	params := map[string][]param{}
	for _, v := range databricksAll(t, dbmeta.RoutineParameters, m, db, nil) {
		params[v.Routine] = append(params[v.Routine], param{v.Name.V, v.Mode, v.DataType, v.Default.V, v.Ordinal})
	}
	for name, want := range map[string][]param{
		"addup":        {{"a", "in", "int", "", 1}, {"b", "in", "int", "5", 2}},
		"bump":         {{"a", "in", "int", "", 1}, {"b", "out", "int", "", 2}},
		"double_it":    {{"x", "in", "bigint", "", 1}},
		"recent_books": {{"since", "in", "date", "", 1}, {"book_id", "table", "bigint", "", 2}, {"title", "table", "string", "", 3}},
	} {
		if fmt.Sprint(params[name]) != fmt.Sprint(want) {
			t.Errorf("routine %s: expected %v, got %v", name, want, params[name])
		}
	}
	if got := databricksAll(t, dbmeta.RoutineParameters, m, db, dbmeta.Args{Parent: "bump", Name: "b"}.Map()); len(got) != 1 {
		t.Errorf("expected the one parameter b of bump, got %+v", got)
	}
}

// TestDatabricksCommentsAndPartitions reads the comments, the partitioned
// tables, the grants and the policies.
func TestDatabricksCommentsAndPartitions(t *testing.T) {
	db := openDatabricks(t)
	m := setupDatabricks(t, db)

	comments := map[string]string{}
	for _, v := range databricksAll(t, dbmeta.Comments, m, db, nil) {
		comments[v.Type+" "+v.Name] = v.Comment
	}
	for k, v := range map[string]string{
		"table author":       "people who write books",
		"view recent":        "recent books",
		"table sales":        "sales by day and region",
		"function double_it": "twice the argument",
		"volume stash":       "files of the fixture",
		"schema dbmeta":      "dbmeta fixture and parity tests",
	} {
		if comments[k] != v {
			t.Errorf("comment on %s: expected %q, got %q", k, v, comments[k])
		}
	}

	// A table partitioned by two columns has a row for each, in the order of the
	// partition. A table with liquid clustering has none.
	type part struct{ name, column, strategy string }
	var pts []part
	for _, v := range databricksAll(t, dbmeta.PartitionedTables, m, db, nil) {
		pts = append(pts, part{v.Name, v.Expression, v.Strategy})
		if v.Type != "table" || v.Parent != "" || v.Table.Valid || v.DirectSize.Valid || v.TotalSize.Valid || v.Owner == "" {
			t.Errorf("partitioned table %s: expected a table with an owner and no size, got %+v", v.Name, v)
		}
	}
	if want := []part{{"sales", "sold_on", "list"}, {"sales", "region", "list"}}; fmt.Sprint(pts) != fmt.Sprint(want) {
		t.Errorf("expected %v, got %v", want, pts)
	}
	cols, detail := databricksDetail(t, db, "clicks")
	if detail["clusteringColumns"] != `["region","at"]` || cols == 0 {
		t.Errorf("expected DESCRIBE DETAIL to hold the clustering columns of clicks, got %v", detail)
	}

	// The grants. book has a grant of its own, and every relation has the
	// grant that the schema gives.
	access := map[string]string{}
	policies := map[string]string{}
	for _, v := range databricksAll(t, dbmeta.Privileges, m, db, nil) {
		access[v.Name] = v.Access.V
		policies[v.Name] = v.Policies.V
		if !v.Access.Valid || v.ColumnAccess.Valid {
			t.Errorf("privileges of %s: expected the grant of the schema and no column access, got %+v", v.Name, v)
		}
	}
	if len(access) != 9 {
		t.Errorf("expected nine relations, got %v", access)
	}
	if got := access["book"]; !strings.Contains(got, "account users=SELECT/") || !strings.Contains(got, "(inherited from schema)") || !strings.Contains(got, "\n") {
		t.Errorf("book: expected its own grant and the inherited one, got %q", got)
	}
	if got := access["region"]; strings.Contains(got, "account users") || strings.Contains(got, "\n") {
		t.Errorf("region: expected only the inherited grant, got %q", got)
	}
	if got := policies["sales"]; got != "row filter workspace.dbmeta.region_filter on (region)" {
		t.Errorf("sales: expected its row filter, got %q", got)
	}
	if got := policies["author"]; got != "column mask workspace.dbmeta.mask_name on name" {
		t.Errorf("author: expected its column mask, got %q", got)
	}
	if got := policies["book"]; got != "" {
		t.Errorf("book: expected no policy, got %q", got)
	}

	var pol []string
	for _, v := range databricksAll(t, dbmeta.Policies, m, db, nil) {
		pol = append(pol, v.Table+" "+v.Name+" "+v.Command+" "+v.Using.V)
		if !v.Permissive || v.Roles.Valid || v.WithCheck.Valid || v.Enabled.Valid {
			t.Errorf("policy %s: expected a permissive policy with no roles, check or switch, got %+v", v.Name, v)
		}
	}
	want := []string{
		"author workspace.dbmeta.mask_name select workspace.dbmeta.mask_name (name)",
		"sales workspace.dbmeta.region_filter all workspace.dbmeta.region_filter (region)",
	}
	if fmt.Sprint(pol) != fmt.Sprint(want) {
		t.Errorf("expected %v, got %v", want, pol)
	}

	schemas := databricksAll(t, dbmeta.Schemas, m, db, dbmeta.Args{Name: "dbmeta"}.Map())
	if len(schemas) != 1 || schemas[0].Comment.V != "dbmeta fixture and parity tests" || !schemas[0].Access.Valid ||
		!strings.Contains(schemas[0].Access.V, "USE_SCHEMA") || schemas[0].Owner == "" {
		t.Fatalf("expected the schema dbmeta with its comment and its grants, got %+v", schemas)
	}
	if got := databricksAll(t, dbmeta.Schemas, m, db, nil); len(got) != 2 {
		t.Errorf("expected dbmeta and default without WithSystem, got %+v", got)
	}
	if got := databricksAll(t, dbmeta.Schemas, m, db, dbmeta.Args{WithSystem: true}.Map()); len(got) != 3 {
		t.Errorf("expected information_schema too with WithSystem, got %+v", got)
	}
}

// TestDatabricksDatabasesAndUser reads the catalogs, the collations, the
// connections and the principal.
func TestDatabricksDatabasesAndUser(t *testing.T) {
	db := openDatabricks(t)
	m := setupDatabricks(t, db)

	names := map[string]bool{}
	for _, v := range databricksAll(t, dbmeta.Databases, m, db, nil) {
		names[v.Name] = true
		if v.Encoding != "" || v.Access.Valid || v.Tablespace.Valid || v.Size.Valid {
			t.Errorf("catalog %s: expected no encoding, access, tablespace or size, got %+v", v.Name, v)
		}
	}
	for _, name := range []string{"workspace", "system"} {
		if !names[name] {
			t.Errorf("expected the catalog %s, got %v", name, names)
		}
	}
	if got := databricksAll(t, dbmeta.Databases, m, db, dbmeta.Args{Name: "work%"}.Map()); len(got) != 1 || got[0].Name != "workspace" {
		t.Errorf("expected the catalog workspace for the name filter, got %+v", got)
	}

	collations := databricksAll(t, dbmeta.Collations, m, db, dbmeta.Args{Name: "UTF8%"}.Map())
	byName := map[string]dbmeta.Collation{}
	for _, v := range collations {
		byName[v.Name] = v
	}
	if af := databricksAll(t, dbmeta.Collations, m, db, dbmeta.Args{Name: "af"}.Map()); len(af) != 1 || af[0].Locale.V != "Afrikaans" {
		t.Errorf("expected the collation af, with its language as the locale, got %+v", af)
	}
	if len(byName) != 4 || !byName["UTF8_BINARY"].Deterministic.V || byName["UTF8_LCASE"].Deterministic.V ||
		byName["UTF8_BINARY"].Provider.V != "builtin" || byName["UTF8_LCASE"].Comment.V != "accent_sensitive, case_insensitive, no_pad" {
		t.Errorf("expected the four built in collations, got %+v", byName)
	}
	if all := databricksAll(t, dbmeta.Collations, m, db, nil); len(all) < 1000 {
		t.Errorf("expected more than a thousand collations, got %d", len(all))
	}
	var icu int
	for _, v := range databricksAll(t, dbmeta.Collations, m, db, dbmeta.Args{Name: "af%"}.Map()) {
		if v.Provider.V != "icu" || v.Schema != "builtin" {
			t.Errorf("collation %s: expected an ICU collation in builtin, got %+v", v.Name, v)
		}
		icu++
	}
	if icu == 0 {
		t.Error("expected ICU collations for the language Afrikaans")
	}

	// A connection needs a privilege on the metastore that the account lacks, so
	// the query is shown to run and to return no row.
	if got := databricksAll(t, dbmeta.ForeignServers, m, db, nil); len(got) != 0 {
		t.Errorf("expected no connection, got %+v", got)
	}

	schema, ok, err := dbmeta.First(dbmeta.CurrentSchema.All(t.Context(), m, db, nil))
	if err != nil || !ok || schema.Name != dbxfixture.Everything.Schema || schema.Catalog != "workspace" {
		t.Errorf("expected the schema of the connection, got %+v, %v, %v", schema, ok, err)
	}
	user, ok, err := dbmeta.First(dbmeta.CurrentUser.All(t.Context(), m, db, nil))
	if err != nil || !ok || user.Name == "" || user.Session.Valid {
		t.Errorf("expected the principal and no session, got %+v, %v, %v", user, ok, err)
	}
}

// TestDatabricksUnanswered checks the kinds that the model does not answer, and
// the sources that it does not read, so that each absence stays a decision. See
// D224.
func TestDatabricksUnanswered(t *testing.T) {
	db := openDatabricks(t)
	m := setupDatabricks(t, db)
	ctx := t.Context()
	for _, q := range []dbmeta.AnyQuery{
		dbmeta.Indexes, dbmeta.IndexColumns, dbmeta.Sequences, dbmeta.Triggers,
		dbmeta.NotNulls, dbmeta.Roles, dbmeta.RoleGrants, dbmeta.ColumnPrivileges,
		dbmeta.Types, dbmeta.Domains, dbmeta.EnumValues, dbmeta.Inherits,
		dbmeta.Partitions, dbmeta.ForeignTables, dbmeta.Settings, dbmeta.Tablespaces,
		dbmeta.Aggregates, dbmeta.Publications, dbmeta.ColumnStats, dbmeta.Extensions,
	} {
		if q.Support(m) != dbmeta.NotSupported {
			t.Errorf("%s: expected it to be unsupported on Databricks", q.Name())
		}
	}

	// Four relations that a model named, and that do not exist. A query that
	// reads one fails.
	for _, rel := range []string{
		"system.information_schema.table_sizes", "system.information_schema.roles",
		"system.information_schema.applicable_roles", "system.information_schema.sequences",
	} {
		err := databricksDrain(ctx, db, "SELECT 1 FROM "+rel+" LIMIT 1")
		if err == nil || !strings.Contains(err.Error(), "TABLE_OR_VIEW_NOT_FOUND") {
			t.Errorf("%s: expected Databricks to say it does not exist, got %v", rel, err)
		}
	}
	// The relations of the system catalog that hold what the model leaves out
	// answer, and the account holds no row in them. If one holds a row, a query
	// that reads it is possible.
	for _, rel := range []string{
		"external_locations", "storage_credentials", "shares", "recipients", "providers",
	} {
		var n int
		if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM system.information_schema."+rel).Scan(&n); err != nil || n != 0 {
			t.Errorf("%s: expected a relation with no row, got %d, %v", rel, n, err)
		}
	}
	// The schemas of the system catalog that hold the history of queries and the
	// storage statistics refuse the principals.
	for _, rel := range []string{"system.access.audit", "system.storage.predictive_optimization_operations_history"} {
		err := databricksDrain(ctx, db, "SELECT 1 FROM "+rel+" LIMIT 1")
		if err == nil || !strings.Contains(err.Error(), "INSUFFICIENT_PERMISSIONS") {
			t.Errorf("%s: expected Databricks to refuse it, got %v", rel, err)
		}
	}
	// The size of a table is only in DESCRIBE DETAIL, which is a statement.
	if _, detail := databricksDetail(t, db, "book"); detail["sizeInBytes"] == "" || detail["numFiles"] == "" {
		t.Errorf("expected DESCRIBE DETAIL to hold the size and the files of a table, got %v", detail)
	}
	if err := databricksDrain(ctx, db, "SELECT numFiles FROM (DESCRIBE DETAIL book)"); err == nil {
		t.Error("expected DESCRIBE DETAIL not to be a relation that a SELECT reads")
	}
}

// databricksDetail runs DESCRIBE DETAIL for a table and returns its only row as
// text for each column, and the number of columns.
func databricksDetail(t *testing.T, db *sql.DB, table string) (int, map[string]string) {
	t.Helper()
	rows, err := db.QueryContext(t.Context(), "DESCRIBE DETAIL "+table)
	if err != nil {
		t.Fatalf("DESCRIBE DETAIL %s: %v", table, err)
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	if rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			t.Fatal(err)
		}
		for i, c := range cols {
			out[c] = fmt.Sprint(vals[i])
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return len(cols), out
}
