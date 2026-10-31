package test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	dbimpts "github.com/xo/dbimp/tablestore"

	"github.com/xo/dbmeta"
	_ "github.com/xo/dbmeta/models/tablestore"
	tsfixture "github.com/xo/dbmeta/models/tablestore/fixture"
)

// The driver is github.com/xo/dbimp/tablestore, which dburl v0.54.0 names for
// the tablestore scheme (D154, D243). The service resets a connection now and
// then and answers "user is disabled" for about a minute after the account is
// made, so the tests open the driver through a wrapper that retries a statement
// that fails that way, and each call of the connector of dbimp that makes the
// tables, because SQL cannot, goes through retry. The retry is in the tests alone,
// and the model has none.

// transient reports whether the error is one that a second try can clear.
func transient(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	for _, w := range []string{"reset by peer", "user is disabled", "EOF", "i/o timeout", "OTSServerBusy", "OTSPartitionUnavailable", "OTSTimeout", "OTSInternalServerError"} {
		if strings.Contains(s, w) {
			return true
		}
	}
	return false
}

// retry runs f up to six times while it fails with a transient error, with a
// longer wait before each try.
func retry(ctx context.Context, f func() error) error {
	var err error
	for try := range 6 {
		if err = f(); !transient(err) {
			return err
		}
		select {
		case <-ctx.Done():
			return err
		case <-time.After(time.Duration(try+1) * 2 * time.Second):
		}
	}
	return err
}

// openTablestore returns a connection to the service named by DBMETA_TABLESTORE,
// which dbrun resolves from the places D117 names. The value is the DSN that the
// driver reads, which dburl writes with the scheme tablestore, and it holds a secret
// that no test prints.
func openTablestore(t *testing.T) *sql.DB {
	t.Helper()
	raw := os.Getenv("DBMETA_TABLESTORE")
	if raw == "" {
		t.Skip("set DBMETA_TABLESTORE to run against the service")
	}
	return openAt(t, tablestoreRetryDriver, raw)
}

// tablestoreClient returns the connector of dbimp for the DSN. The fixture needs
// it, because SQL maps a table and cannot make one. A fault names no part of the
// DSN, which has a secret. The caller closes the connector.
func tablestoreClient(raw string) (*dbimpts.Connector, error) {
	cfg, err := dbimpts.ParseDSN(raw)
	if err != nil {
		return nil, errors.New("reading the DSN of Tablestore: the value is not a DSN that the driver reads")
	}
	return dbimpts.NewConnector(*cfg), nil
}

func columnType(t tsfixture.Type) dbimpts.ColumnType {
	if t == tsfixture.Integer {
		return dbimpts.Integer
	}
	return dbimpts.String
}

// makeTablestoreBase makes the tables, the secondary indexes and the search
// indexes of the fixture with the connector. A table that exists is left as it is,
// so that a second run reuses it. Each table is pay as you go, with no reserved
// throughput.
func makeTablestoreBase(ctx context.Context, c *dbimpts.Connector, f tsfixture.Fixture) error {
	// A retry of a request that reached the server finds the object made.
	made := func(err error) error {
		if errors.Is(err, dbimpts.ErrExists) {
			return nil
		}
		return err
	}
	for _, bt := range f.Base {
		t := dbimpts.Table{Name: bt.Name}
		for _, k := range bt.Keys {
			t.Keys = append(t.Keys, dbimpts.Column{Name: k.Name, Type: columnType(k.Type)})
		}
		for _, col := range bt.Columns {
			t.Columns = append(t.Columns, dbimpts.Column{Name: col.Name, Type: columnType(col.Type)})
		}
		if err := retry(ctx, func() error { return made(c.CreateTable(ctx, t)) }); err != nil {
			return fmt.Errorf("creating the table %s: %w", bt.Name, err)
		}
		for _, ix := range bt.Indexes {
			im := dbimpts.Index{Name: ix.Name, Keys: ix.Columns}
			if err := retry(ctx, func() error { return made(c.CreateIndex(ctx, bt.Name, im)) }); err != nil {
				return fmt.Errorf("creating the index %s: %w", ix.Name, err)
			}
		}
		for _, si := range bt.Searches {
			s := dbimpts.SearchIndex{Name: si.Name}
			for _, fld := range si.Fields {
				s.Fields = append(s.Fields, dbimpts.SearchField{Name: fld.Name, Type: dbimpts.SearchKeyword})
			}
			if err := retry(ctx, func() error { return made(c.CreateSearchIndex(ctx, bt.Name, s)) }); err != nil {
				return fmt.Errorf("creating the search index %s: %w", si.Name, err)
			}
		}
	}
	return nil
}

// dropTablestoreBase deletes every table of the instance whose name starts with
// the prefix of the fixture, with its search indexes and its secondary indexes.
// The service refuses a table that has a search index, so each search index goes
// first, and the table takes its secondary indexes with it. It touches no other
// table.
func dropTablestoreBase(ctx context.Context, c *dbimpts.Connector) error {
	var names []string
	if err := retry(ctx, func() error {
		var err error
		names, err = c.ListTables(ctx)
		return err
	}); err != nil {
		return fmt.Errorf("listing the tables: %w", err)
	}
	for _, name := range names {
		if !tsfixture.Is(name) {
			continue
		}
		var indexes []string
		if err := retry(ctx, func() error {
			var err error
			indexes, err = c.ListSearchIndexes(ctx, name)
			return err
		}); err != nil {
			return fmt.Errorf("listing the search indexes of %s: %w", name, err)
		}
		for _, ix := range indexes {
			if err := retry(ctx, func() error { return c.DeleteSearchIndex(ctx, name, ix, dbimpts.WithIfExists()) }); err != nil {
				return fmt.Errorf("deleting the search index %s: %w", ix, err)
			}
		}
		if err := retry(ctx, func() error { return c.DeleteTable(ctx, name, dbimpts.WithIfExists()) }); err != nil {
			return fmt.Errorf("deleting the table %s: %w", name, err)
		}
	}
	return nil
}

// tablestoreState is the one fixture that every Tablestore test shares. The first
// test builds it, every test reads it, and TestMain drops it when the last test
// ends. No test changes it.
var tablestoreState struct {
	mu    sync.Mutex
	meta  *dbmeta.Meta
	err   error
	built bool
}

// runTablestoreSteps runs the statements one at a time. A step of a teardown that
// fails is not an error when quiet is true.
func runTablestoreSteps(ctx context.Context, db *sql.DB, steps []tsfixture.Result, quiet bool) error {
	for _, s := range steps {
		_, err := db.ExecContext(ctx, s.Query)
		if err != nil && !quiet {
			return fmt.Errorf("%s: %w\n%s", s.Name, err, s.Query)
		}
	}
	return nil
}

// shutdownTablestore drops the fixture, if a test built it. TestMain calls it.
func shutdownTablestore() {
	tablestoreState.mu.Lock()
	defer tablestoreState.mu.Unlock()
	if !tablestoreState.built || os.Getenv("DBMETA_TABLESTORE_KEEP") != "" {
		return
	}
	raw := os.Getenv("DBMETA_TABLESTORE")
	db, err := sql.Open(tablestoreRetryDriver, raw)
	if err != nil {
		fmt.Fprintln(os.Stderr, "dropping the Tablestore fixture: opening the driver failed")
		return
	}
	defer db.Close()
	c, err := tablestoreClient(raw)
	if err != nil {
		fmt.Fprintf(os.Stderr, "dropping the Tablestore fixture: %v\n", err)
		return
	}
	defer c.Close()
	ctx := context.Background()
	down, err := tsfixture.Everything.ResolveTeardown(dbmeta.VersionSet{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "dropping the Tablestore fixture: %v\n", err)
		return
	}
	//nolint:errcheck // a teardown is best effort, and quiet says so
	runTablestoreSteps(ctx, db, down, true)
	if err := dropTablestoreBase(ctx, c); err != nil {
		fmt.Fprintf(os.Stderr, "dropping the Tablestore fixture: %v\n", err)
	}
}

// setupTablestore builds the fixture, once, and returns the metadata for the
// service. It tears down first, because a run that failed part way can leave a
// table behind.
func setupTablestore(t *testing.T, db *sql.DB) *dbmeta.Meta {
	t.Helper()
	tablestoreState.mu.Lock()
	defer tablestoreState.mu.Unlock()
	if tablestoreState.err != nil {
		t.Fatalf("the fixture failed to build in an earlier test: %v", tablestoreState.err)
	}
	if tablestoreState.meta != nil {
		return tablestoreState.meta
	}
	ctx := context.WithoutCancel(t.Context())
	fail := func(format string, args ...any) {
		tablestoreState.err = fmt.Errorf(format, args...)
		t.Fatal(tablestoreState.err)
	}
	c, err := tablestoreClient(os.Getenv("DBMETA_TABLESTORE"))
	if err != nil {
		fail("%v", err)
	}
	defer c.Close()
	versions, err := dbmeta.Tablestore.Version(ctx, db)
	if err != nil {
		fail("reading the version: %v", err)
	}
	m, err := dbmeta.New(dbmeta.Tablestore, versions)
	if err != nil {
		fail("building the metadata: %v", err)
	}
	down, err := tsfixture.Everything.ResolveTeardown(versions)
	if err != nil {
		fail("resolving the teardown: %v", err)
	}
	if os.Getenv("DBMETA_TABLESTORE_KEEP") == "" {
		//nolint:errcheck // a teardown before setup is best effort, and quiet says so
		runTablestoreSteps(ctx, db, down, true)
		if err := dropTablestoreBase(ctx, c); err != nil {
			fail("clearing an earlier run: %v", err)
		}
	}
	up, err := tsfixture.Everything.ResolveSetup(versions)
	if err != nil {
		fail("resolving the setup: %v", err)
	}
	tablestoreState.built = true
	if err := makeTablestoreBase(ctx, c, tsfixture.Everything); err != nil {
		fail("making the tables: %v", err)
	}
	//nolint:errcheck // a mapping that exists is a fault of CREATE TABLE, and the drop above cleared it
	runTablestoreSteps(ctx, db, down, true)
	if err := runTablestoreSteps(ctx, db, up, false); err != nil {
		fail("setup: %v", err)
	}
	t.Logf("server reports %s, fixture mapped %d tables", versions, len(up))
	tablestoreState.meta = m
	return m
}

// tablestoreAll reads every row a query answers, and stops the test on an error.
func tablestoreAll[T any](t *testing.T, q *dbmeta.Query[T], m *dbmeta.Meta, db *sql.DB, args map[string]any) []T {
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

// tablestoreDrain runs a statement and reads every row, and returns the columns and
// the error that either step gave.
func tablestoreDrain(ctx context.Context, db *sql.DB, stmt string) ([]string, error) {
	rows, err := db.QueryContext(ctx, stmt)
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

// TestTablestoreVersion checks what the model makes of @@version. The service
// has no release, and usql runs SELECT version(), which it refuses. See D241.
func TestTablestoreVersion(t *testing.T) {
	db := openTablestore(t)
	versions, err := dbmeta.Tablestore.Version(t.Context(), db)
	if err != nil {
		t.Fatalf("reading the version: %v", err)
	}
	main := versions.Main()
	if main.Unknown || len(main.Parts) != 3 || main.Parts[0] != 5 || main.Parts[1] != 7 {
		t.Errorf("expected the MySQL 5.7 level that the front end claims, got %+v", main)
	}
	if !strings.Contains(main.Suffix, "SQLFrontend") {
		t.Errorf("expected the build of the SQL front end in the suffix, got %q", main.Suffix)
	}
	if !strings.HasPrefix(versions.Display, "Tablestore SQL 5.7") {
		t.Errorf("expected the display to name Tablestore SQL, got %q", versions.Display)
	}
	// usql falls to SELECT version(), and the service refuses it, so usql prints
	// no version.
	if _, err := tablestoreDrain(t.Context(), db, "SELECT version()"); err == nil {
		t.Error("SELECT version(): expected the service to refuse it")
	}
	t.Logf("server reports %s", versions)
}

// TestTablestoreSmoke runs each registered query against the fixture and checks
// that it returns the columns it declares. SHOW FULL TABLES names its columns
// Tables_in_<instance> and Table_type, so the test checks the count. A walk has
// no one statement, so Build refuses it with ErrSeveralStatements, and the test
// reads it through All and checks that it yields a row.
func TestTablestoreSmoke(t *testing.T) {
	db := openTablestore(t)
	m := setupTablestore(t, db)

	var ran, walked int
	for _, q := range dbmeta.Queries() {
		if q.Support(m) != dbmeta.Supported {
			continue
		}
		query, vals, err := q.Build(m, nil)
		if errors.Is(err, dbmeta.ErrSeveralStatements) {
			walked++
			continue
		}
		if err != nil {
			t.Errorf("%s: building: %v", q.Name(), err)
			continue
		}
		if len(vals) != 0 {
			t.Errorf("%s: expected no bound value, got %d", q.Name(), len(vals))
		}
		cols, err := tablestoreDrain(t.Context(), db, query)
		if err != nil {
			t.Errorf("%s: %v\n%s", q.Name(), err, query)
			continue
		}
		fields, err := q.Fields(m)
		if err != nil {
			t.Errorf("%s: reading fields: %v", q.Name(), err)
			continue
		}
		if len(cols) != len(fields) {
			t.Errorf("%s: declares %d fields and returns %d columns", q.Name(), len(fields), len(cols))
		}
		if len(cols) == 2 && !strings.HasPrefix(cols[0], "Tables_in_") {
			t.Errorf("%s: expected the first column to be Tables_in_<instance>, got %q", q.Name(), cols[0])
		}
		ran++
	}
	if ran != 1 || walked != 3 {
		t.Errorf("expected the 1 statement and the 3 walks the package comment names, got %d and %d", ran, walked)
	}
	// A walk yields rows for the fixture, and each declared field is a field of
	// the model.
	for _, q := range []dbmeta.AnyQuery{dbmeta.Columns, dbmeta.Indexes, dbmeta.IndexColumns} {
		fields, err := q.Fields(m)
		if err != nil || len(fields) == 0 {
			t.Errorf("%s: expected declared fields, got %v, %v", q.Name(), fields, err)
		}
	}
}

// TestTablestoreTables reads the tables of the fixture back through the typed
// API, and checks the filters.
func TestTablestoreTables(t *testing.T) {
	db := openTablestore(t)
	m := setupTablestore(t, db)

	tables := map[string]dbmeta.Table{}
	for _, v := range tablestoreAll(t, dbmeta.Tables, m, db, nil) {
		tables[v.Name] = v
		if v.Catalog != "" || v.Schema != "" {
			t.Errorf("table %s: expected no catalog and no schema, got %q and %q", v.Name, v.Catalog, v.Schema)
		}
		if v.Comment.Valid || v.Owner.Valid || v.Size.Valid || v.Rows.Valid || v.Options.Valid ||
			v.Persistence.Valid || v.AccessMethod.Valid || v.RowSecurity.Valid {
			t.Errorf("table %s: expected nothing but the name and the type, got %+v", v.Name, v)
		}
	}
	for _, name := range tsfixture.Everything.Names() {
		got, ok := tables[name]
		if !ok {
			t.Errorf("table %s: not listed", name)
			continue
		}
		if got.Type != "table" {
			t.Errorf("table %s: expected type table, got %q", name, got.Type)
		}
	}
	// An index is not a table, and SHOW FULL TABLES does not list one.
	for _, name := range []string{"dbmeta_fx_author_name", "dbmeta_fx_book_title"} {
		if _, ok := tables[name]; ok {
			t.Errorf("%s: expected an index to be absent from the tables", name)
		}
	}

	// The filters, which Keep applies after the statement.
	got := tablestoreAll(t, dbmeta.Tables, m, db, dbmeta.Args{Name: "dbmeta_fx_auth%"}.Map())
	if len(got) != 1 || got[0].Name != "dbmeta_fx_author" {
		t.Errorf("expected dbmeta_fx_author alone, got %+v", got)
	}
	if got := tablestoreAll(t, dbmeta.Tables, m, db, dbmeta.Args{Name: "dbmeta_fx_%"}.Map()); len(got) != 4 {
		t.Errorf("expected the four tables of the fixture, got %+v", got)
	}
	if got := tablestoreAll(t, dbmeta.Tables, m, db, dbmeta.Args{Types: []string{"view"}}.Map()); len(got) != 0 {
		t.Errorf("expected no view, because Tablestore SQL has none, got %+v", got)
	}
	if got := tablestoreAll(t, dbmeta.Tables, m, db, dbmeta.Args{Types: []string{"table"}, Name: "dbmeta_fx_region"}.Map()); len(got) != 1 {
		t.Errorf("expected region, got %+v", got)
	}
	if got := tablestoreAll(t, dbmeta.Tables, m, db, dbmeta.Args{Schema: "dbmeta"}.Map()); len(got) != 0 {
		t.Errorf("expected no table in a schema, because a table has none, got %+v", got)
	}
	// A quote and a backslash in a filter match nothing and do not fail, because
	// the statement takes no parameter.
	for _, name := range []string{"it's", `back\slash`, `x'' OR 1=1 --`} {
		if got := tablestoreAll(t, dbmeta.Tables, m, db, dbmeta.Args{Name: name}.Map()); len(got) != 0 {
			t.Errorf("filter %q: expected no table, got %+v", name, got)
		}
	}
}

// tsCounter is a Queryer that records every statement that a walk runs, and
// calls hook before each one. A hook can change the instance or return an error
// for a statement.
type tsCounter struct {
	db      *sql.DB
	queries []string
	hook    func(query string) error
}

func (c *tsCounter) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	c.queries = append(c.queries, query)
	if c.hook != nil {
		if err := c.hook(query); err != nil {
			return nil, err
		}
	}
	return c.db.QueryContext(ctx, query, args...)
}

// tablestoreWalk reads every row of a walk through a counter, and stops the
// test on an error.
func tablestoreWalk[T any](t *testing.T, q *dbmeta.Query[T], m *dbmeta.Meta, db *sql.DB, args map[string]any) ([]T, []string) {
	t.Helper()
	c := &tsCounter{db: db}
	var out []T
	for v, err := range q.All(t.Context(), m, c, args) {
		if err != nil {
			t.Fatalf("reading %s: %v", q.Name(), err)
		}
		out = append(out, v)
	}
	return out, c.queries
}

// TestTablestoreColumns reads the columns of the fixture back through the walk
// of D242, and checks what each field holds against DESCRIBE, the filters and
// the number of statements.
func TestTablestoreColumns(t *testing.T) {
	db := openTablestore(t)
	m := setupTablestore(t, db)

	type col struct {
		name, typ string
		nullable  bool
		key       bool
	}
	want := map[string][]col{
		"dbmeta_fx_author":   {{"author_id", "bigint(20)", false, true}, {"name", "mediumtext", true, false}, {"rating", "bigint(20)", true, false}},
		"dbmeta_fx_book":     {{"book_id", "bigint(20)", false, true}, {"author_id", "bigint(20)", true, false}, {"title", "mediumtext", true, false}, {"published", "mediumtext", true, false}},
		"dbmeta_fx_region":   {{"country", "varchar(1024)", false, true}, {"area", "varchar(1024)", false, true}, {"population", "bigint(20)", true, false}},
		"dbmeta_fx_shipment": {{"shipment_id", "bigint(20)", false, true}, {"country", "mediumtext", true, false}, {"area", "mediumtext", true, false}, {"amount", "bigint(20)", true, false}},
	}
	got, queries := tablestoreWalk(t, dbmeta.Columns, m, db, nil)
	// One statement for the list and one DESCRIBE for each of the four tables.
	if len(queries) != 5 || queries[0] != "SHOW FULL TABLES" {
		t.Errorf("expected SHOW FULL TABLES and 4 DESCRIBE, got %q", queries)
	}
	byTable := map[string][]dbmeta.Column{}
	for _, v := range got {
		if v.Catalog != "" || v.Schema != "" {
			t.Errorf("%s.%s: expected no catalog and no schema, got %q and %q", v.Table, v.Name, v.Catalog, v.Schema)
		}
		if v.Default.Valid || v.Identity.Valid || v.Generated.Valid || v.Comment.Valid || v.Collation.Valid ||
			v.Storage.Valid || v.Compression.Valid || v.StatsTarget.Valid {
			t.Errorf("%s.%s: expected nothing but the name, the type, the null flag and the key, got %+v", v.Table, v.Name, v)
		}
		byTable[v.Table] = append(byTable[v.Table], v)
	}
	for table, cols := range want {
		g := byTable[table]
		if len(g) != len(cols) {
			t.Errorf("%s: expected %d columns, got %+v", table, len(cols), g)
			continue
		}
		for i, c := range cols {
			if g[i].Name != c.name || g[i].Ordinal != i+1 || g[i].DataType != c.typ || g[i].Nullable != c.nullable || g[i].PrimaryKey != c.key {
				t.Errorf("%s column %d: expected %s %s nullable=%v key=%v, got %+v", table, i+1, c.name, c.typ, c.nullable, c.key, g[i])
			}
		}
	}

	// The filters. A parent pattern runs no DESCRIBE for a table it drops.
	one, queries := tablestoreWalk(t, dbmeta.Columns, m, db, dbmeta.Args{Parent: "dbmeta_fx_reg%"}.Map())
	if len(one) != 3 || len(queries) != 2 {
		t.Errorf("expected the 3 columns of region with 2 statements, got %d with %q", len(one), queries)
	}
	if got, _ := tablestoreWalk(t, dbmeta.Columns, m, db, dbmeta.Args{Parent: "dbmeta_fx_%", Name: "country"}.Map()); len(got) != 2 {
		t.Errorf("expected the country of region and of shipment, got %+v", got)
	}
	if got, _ := tablestoreWalk(t, dbmeta.Columns, m, db, dbmeta.Args{Schema: "dbmeta"}.Map()); len(got) != 0 {
		t.Errorf("expected no column in a schema, got %+v", got)
	}
	if got, queries := tablestoreWalk(t, dbmeta.Columns, m, db, dbmeta.Args{Schema: "dbmeta"}.Map()); len(queries) != 0 {
		t.Errorf("expected no statement for a schema that no table has, got %q for %+v", queries, got)
	}
	// A quote in a pattern matches nothing and does not fail.
	for _, name := range []string{"it's", `x'' OR 1=1 --`} {
		if got, _ := tablestoreWalk(t, dbmeta.Columns, m, db, dbmeta.Args{Parent: name}.Map()); len(got) != 0 {
			t.Errorf("parent %q: expected no column, got %+v", name, got)
		}
	}
}

// TestTablestoreIndexes reads the indexes and the index columns of the fixture
// through the walk of D242.
func TestTablestoreIndexes(t *testing.T) {
	db := openTablestore(t)
	m := setupTablestore(t, db)

	idx, queries := tablestoreWalk(t, dbmeta.Indexes, m, db, nil)
	if len(queries) != 5 || queries[0] != "SHOW FULL TABLES" {
		t.Errorf("expected SHOW FULL TABLES and 4 SHOW INDEX, got %q", queries)
	}
	want := []dbmeta.Index{
		{Table: "dbmeta_fx_author", Name: "PRIMARY", Type: "", Unique: true, Primary: true},
		{Table: "dbmeta_fx_author", Name: "dbmeta_fx_author_name", Type: "GlobalIndex"},
		{Table: "dbmeta_fx_book", Name: "PRIMARY", Type: "", Unique: true, Primary: true},
		{Table: "dbmeta_fx_book", Name: "dbmeta_fx_book_title", Type: "SearchIndex"},
		{Table: "dbmeta_fx_region", Name: "PRIMARY", Type: "", Unique: true, Primary: true},
		{Table: "dbmeta_fx_shipment", Name: "PRIMARY", Type: "", Unique: true, Primary: true},
	}
	if len(idx) != len(want) {
		t.Fatalf("expected %d indexes, got %+v", len(want), idx)
	}
	for i, w := range want {
		if idx[i] != w {
			t.Errorf("index %d: expected %+v, got %+v", i, w, idx[i])
		}
	}

	type ic struct {
		table, index, name string
		ordinal            int64
	}
	wantCols := []ic{
		{"dbmeta_fx_author", "PRIMARY", "author_id", 1},
		{"dbmeta_fx_author", "dbmeta_fx_author_name", "name", 1},
		{"dbmeta_fx_author", "dbmeta_fx_author_name", "author_id", 2},
		{"dbmeta_fx_book", "PRIMARY", "book_id", 1},
		{"dbmeta_fx_book", "dbmeta_fx_book_title", "title", 1},
		{"dbmeta_fx_region", "PRIMARY", "country", 1},
		{"dbmeta_fx_region", "PRIMARY", "area", 2},
		{"dbmeta_fx_shipment", "PRIMARY", "shipment_id", 1},
	}
	cols, queries := tablestoreWalk(t, dbmeta.IndexColumns, m, db, nil)
	if len(queries) != 5 {
		t.Errorf("expected SHOW FULL TABLES and 4 SHOW INDEX, got %q", queries)
	}
	if len(cols) != len(wantCols) {
		t.Fatalf("expected %d index columns, got %+v", len(wantCols), cols)
	}
	for i, w := range wantCols {
		g := cols[i]
		if g.Schema != "" || g.Table != w.table || g.Index != w.index || !g.Name.Valid || g.Name.V != w.name || g.Ordinal != w.ordinal {
			t.Errorf("index column %d: expected %+v, got %+v", i, w, g)
		}
		if g.Expression.Valid || g.Include {
			t.Errorf("index column %d: expected no expression and no include, got %+v", i, g)
		}
		// SHOW INDEX says A for the order of every row.
		if !g.Descending.Valid || g.Descending.V {
			t.Errorf("index column %d: expected an ascending column, got %+v", i, g.Descending)
		}
	}

	// The filters. Name matches the index for both kinds.
	if got, _ := tablestoreWalk(t, dbmeta.Indexes, m, db, dbmeta.Args{Name: "dbmeta_fx_%"}.Map()); len(got) != 2 {
		t.Errorf("expected the secondary index and the search index, got %+v", got)
	}
	if got, _ := tablestoreWalk(t, dbmeta.IndexColumns, m, db, dbmeta.Args{Parent: "dbmeta_fx_region", Name: "PRIMARY"}.Map()); len(got) != 2 {
		t.Errorf("expected the 2 columns of the key of region, got %+v", got)
	}
	if got, _ := tablestoreWalk(t, dbmeta.Indexes, m, db, dbmeta.Args{Schema: "dbmeta"}.Map()); len(got) != 0 {
		t.Errorf("expected no index in a schema, got %+v", got)
	}
}

// TestTablestoreWalkErrors checks what a walk does when a statement about one
// table fails. A table that is dropped between the list and its DESCRIBE is
// skipped. Any other error reaches the caller. See D242.
func TestTablestoreWalkErrors(t *testing.T) {
	db := openTablestore(t)
	m := setupTablestore(t, db)
	ctx := context.WithoutCancel(t.Context())

	// A fifth table, which the hook drops after the list names it.
	c, err := tablestoreClient(os.Getenv("DBMETA_TABLESTORE"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close() })
	gone := tsfixture.Fixture{Base: []tsfixture.BaseTable{{
		Name: tsfixture.Prefix + "gone",
		Keys: []tsfixture.Column{{Name: "id", Type: tsfixture.Integer}},
	}}}
	if err := makeTablestoreBase(ctx, c, gone); err != nil {
		t.Fatalf("making the table: %v", err)
	}
	t.Cleanup(func() {
		//nolint:errcheck // a drop of a mapping that is gone already is not a fault
		db.ExecContext(ctx, "DROP MAPPING TABLE IF EXISTS dbmeta_fx_gone")
		if err := retry(ctx, func() error {
			return c.DeleteTable(ctx, tsfixture.Prefix+"gone", dbimpts.WithIfExists())
		}); err != nil {
			t.Errorf("deleting the table: %v", err)
		}
	})
	if _, err := db.ExecContext(ctx, "CREATE TABLE dbmeta_fx_gone (id BIGINT, PRIMARY KEY(id))"); err != nil {
		t.Fatalf("mapping the table: %v", err)
	}
	counter := &tsCounter{db: db}
	counter.hook = func(query string) error {
		if query == "DESCRIBE `dbmeta_fx_gone`" {
			_, err := db.ExecContext(ctx, "DROP MAPPING TABLE dbmeta_fx_gone")
			return err
		}
		return nil
	}
	var n int
	for v, err := range dbmeta.Columns.All(t.Context(), m, counter, dbmeta.Args{Parent: "dbmeta_fx_%"}.Map()) {
		if err != nil {
			t.Fatalf("expected the dropped table to be skipped, got %v", err)
		}
		if v.Table == "dbmeta_fx_gone" {
			t.Errorf("expected no column of the dropped table, got %+v", v)
		}
		n++
	}
	if n != 14 {
		t.Errorf("expected the 14 columns of the four tables, got %d", n)
	}
	// SHOW FULL TABLES, five DESCRIBE, and a second SHOW FULL TABLES that found
	// the table gone.
	if len(counter.queries) != 7 {
		t.Errorf("expected 7 statements, got %q", counter.queries)
	}

	// An error for a table that is still listed ends the walk, with that error.
	boom := errors.New("the service is unwell")
	counter = &tsCounter{db: db}
	counter.hook = func(query string) error {
		if query == "SHOW INDEX FROM `dbmeta_fx_book`" {
			return boom
		}
		return nil
	}
	var got []dbmeta.Index
	var failed error
	for v, err := range dbmeta.Indexes.All(t.Context(), m, counter, nil) {
		if err != nil {
			failed = err
			break
		}
		got = append(got, v)
	}
	if !errors.Is(failed, boom) {
		t.Errorf("expected the error of the service, got %v", failed)
	}
	// author has 2 indexes, and the walk stops at book.
	if len(got) != 2 {
		t.Errorf("expected the 2 indexes of author before the error, got %+v", got)
	}
	// Stopping early closes the walk and runs no more statements.
	counter = &tsCounter{db: db}
	for _, err := range dbmeta.Columns.All(t.Context(), m, counter, nil) {
		if err != nil {
			t.Fatal(err)
		}
		break
	}
	if len(counter.queries) != 2 {
		t.Errorf("expected the list and one DESCRIBE before the early stop, got %q", counter.queries)
	}
	// A table that does not exist errors, and the quote function survives a
	// name that holds a backtick.
	if _, err := tablestoreDrain(ctx, db, "DESCRIBE `a``b`"); err == nil || !strings.Contains(err.Error(), "a`b") {
		t.Errorf("expected the service to read a doubled backtick as one, got %v", err)
	}
}

// TestTablestoreUnanswered checks that the 4 kinds answer and that the sources
// that the model does not read are still refused, so that a change on the service
// shows. docs/COVERAGE.md says why each is out.
func TestTablestoreUnanswered(t *testing.T) {
	db := openTablestore(t)
	m := setupTablestore(t, db)

	for name, stmt := range map[string]string{
		"information_schema": "SELECT * FROM information_schema.tables",
		"show databases":     "SHOW DATABASES",
		"show variables":     "SHOW VARIABLES",
		"show status":        "SHOW STATUS",
		"show grants":        "SHOW GRANTS",
		"show table status":  "SHOW TABLE STATUS",
		"show functions":     "SHOW FUNCTIONS",
		"show triggers":      "SHOW TRIGGERS",
		"show collation":     "SHOW COLLATION",
		"show privileges":    "SHOW PRIVILEGES",
		"show create view":   "SHOW CREATE VIEW v",
		"create view":        "CREATE VIEW v AS SELECT 1",
		"select user":        "SELECT user()",
		"select database":    "SELECT database()",
		"select version":     "SELECT version()",
		"mysql schema":       "SELECT * FROM mysql.user",
	} {
		if _, err := tablestoreDrain(t.Context(), db, stmt); err == nil {
			t.Errorf("%s: expected the service to refuse it, and the model reads nothing there", name)
		}
	}
	var supported int
	for _, q := range dbmeta.Queries() {
		if q.Support(m) == dbmeta.Supported {
			supported++
			switch q.Name() {
			case "tables", "columns", "indexes", "index_columns":
			default:
				t.Errorf("%s: expected only tables, columns, indexes and index_columns to be supported", q.Name())
			}
		}
	}
	if supported != 4 {
		t.Errorf("expected 4 supported queries, got %d", supported)
	}
	for _, q := range []dbmeta.AnyQuery{dbmeta.Tables, dbmeta.Columns, dbmeta.Indexes, dbmeta.IndexColumns} {
		if got := q.Support(m); got != dbmeta.Supported {
			t.Errorf("%s: expected Supported, got %v", q.Name(), got)
		}
	}
	for _, q := range []dbmeta.AnyQuery{dbmeta.Views, dbmeta.Schemas, dbmeta.Databases, dbmeta.CurrentUser, dbmeta.CurrentSchema} {
		if got := q.Support(m); got != dbmeta.NotSupported {
			t.Errorf("%s: expected NotSupported, got %v", q.Name(), got)
		}
	}
}
