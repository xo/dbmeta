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

	_ "github.com/googleapis/go-sql-spanner"

	"github.com/xo/dbmeta"
	_ "github.com/xo/dbmeta/models/spanner"
	spfixture "github.com/xo/dbmeta/models/spanner/fixture"
)

// openSpanner returns a connection to the server named by DBMETA_SPANNER.
//
// The driver is github.com/googleapis/go-sql-spanner, which dburl v0.49.0 names
// for the spanner scheme (D154). The value is the DSN that the driver takes, such
// as host:port/projects/default/instances/default/databases/dbmeta, with
// usePlainText for a server that has no certificate. dbrun sets it. The test
// reads Spanner Omni and Cloud Spanner. See D216 and D219.
func openSpanner(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("DBMETA_SPANNER")
	if dsn == "" {
		t.Skip("set DBMETA_SPANNER to run against a real server")
	}
	return openAt(t, "spanner", dsn)
}

// runSpannerDDL runs the statements as one batch on one connection, and returns
// the error of the batch. Spanner takes a schema change as a long running
// operation that lasts seconds, so a batch is one operation where separate
// statements are one each. A statement that Spanner refuses fails the whole
// batch and the error does not say which, so a caller that needs to know runs
// the statements one at a time, which is spannerDDLEach.
func runSpannerDDL(ctx context.Context, db *sql.DB, queries []string) error {
	conn, err := db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("taking a connection: %w", err)
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, "START BATCH DDL"); err != nil {
		return fmt.Errorf("starting the batch: %w", err)
	}
	for _, q := range queries {
		if _, err := conn.ExecContext(ctx, q); err != nil {
			//nolint:errcheck // the batch is abandoned and the first error is the one that matters
			conn.ExecContext(context.WithoutCancel(ctx), "ABORT BATCH")
			return fmt.Errorf("adding to the batch: %w", err)
		}
	}
	if _, err := conn.ExecContext(ctx, "RUN BATCH"); err != nil {
		return fmt.Errorf("running the batch: %w", err)
	}
	return nil
}

// spannerDDLEach runs each statement alone, and reports the one that failed. A
// statement of a teardown that fails is not an error when quiet is true,
// because the object is already gone.
func spannerDDLEach(ctx context.Context, db *sql.DB, steps []spfixture.Result, quiet bool) error {
	for _, s := range steps {
		if _, err := db.ExecContext(ctx, s.Query); err != nil && !quiet {
			return fmt.Errorf("%s: %w\n%s", s.Name, err, s.Query)
		}
	}
	return nil
}

func spannerQueries(steps []spfixture.Result) []string {
	out := make([]string, 0, len(steps))
	for _, s := range steps {
		out = append(out, s.Query)
	}
	return out
}

// spannerHasFixture reports whether the fixture schema exists.
func spannerHasFixture(ctx context.Context, db *sql.DB) (bool, error) {
	var n int64
	err := db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM information_schema.schemata WHERE schema_name = ?",
		spfixture.Everything.Schema).Scan(&n)
	return n > 0, err
}

// spannerState is the one fixture that every Spanner test shares.
//
// Spanner Omni takes seconds for each schema change, so building the fixture
// takes minutes, and a fixture for each test is more than the tests are worth.
// The first test builds it, every test reads it, and TestMain drops it when
// the last test ends. No test changes it.
var spannerState struct {
	mu    sync.Mutex
	meta  *dbmeta.Meta
	err   error
	built bool
}

// spannerTeardown returns the statements that drop the fixture.
func spannerTeardown(versions dbmeta.VersionSet) ([]spfixture.Result, error) {
	return spfixture.Everything.ResolveTeardown(versions)
}

// dropSpannerFixture drops the fixture. The teardown is one batch, and when the
// batch fails because something is already gone, each step runs alone and a
// refusal is ignored.
func dropSpannerFixture(ctx context.Context, db *sql.DB, down []spfixture.Result) {
	if err := runSpannerDDL(ctx, db, spannerQueries(down)); err != nil {
		//nolint:errcheck // a teardown is best effort, and quiet says so
		spannerDDLEach(ctx, db, down, true)
	}
}

// shutdownSpanner drops the fixture, if a test built it. TestMain calls it.
func shutdownSpanner() {
	spannerState.mu.Lock()
	defer spannerState.mu.Unlock()
	if !spannerState.built {
		return
	}
	// DBMETA_SPANNER_KEEP leaves the fixture up, so that a person can write a
	// statement against it, and the next run with it set reuses that fixture. A
	// run without it drops the fixture and builds its own.
	if os.Getenv("DBMETA_SPANNER_KEEP") != "" {
		return
	}
	dsn := os.Getenv("DBMETA_SPANNER")
	db, err := sql.Open("spanner", dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "dropping the Spanner fixture: %v\n", err)
		return
	}
	defer db.Close()
	ctx := context.Background()
	versions, err := dbmeta.Spanner.Version(ctx, db)
	if err != nil {
		fmt.Fprintf(os.Stderr, "dropping the Spanner fixture: %v\n", err)
		return
	}
	down, err := spannerTeardown(versions)
	if err != nil {
		fmt.Fprintf(os.Stderr, "dropping the Spanner fixture: %v\n", err)
		return
	}
	dropSpannerFixture(ctx, db, down)
}

// setupSpanner builds the fixture, once, and returns the metadata for the
// server.
//
// It tears down first when a run that failed part way left the schema behind,
// because CREATE SCHEMA then fails and the test reports that and not what went
// wrong.
func setupSpanner(t *testing.T, db *sql.DB) *dbmeta.Meta {
	t.Helper()
	spannerState.mu.Lock()
	defer spannerState.mu.Unlock()
	if spannerState.err != nil {
		t.Fatalf("the fixture failed to build in an earlier test: %v", spannerState.err)
	}
	if spannerState.meta != nil {
		return spannerState.meta
	}
	ctx := context.WithoutCancel(t.Context())
	fail := func(format string, args ...any) {
		spannerState.err = fmt.Errorf(format, args...)
		t.Fatal(spannerState.err)
	}
	versions, err := dbmeta.Spanner.Version(ctx, db)
	if err != nil {
		fail("reading the version: %v", err)
	}
	down, err := spannerTeardown(versions)
	if err != nil {
		fail("resolving the teardown: %v", err)
	}
	if has, err := spannerHasFixture(ctx, db); err != nil {
		fail("looking for the fixture: %v", err)
	} else if has && os.Getenv("DBMETA_SPANNER_KEEP") != "" {
		// A person kept the fixture of an earlier run to write against.
		m, err := dbmeta.New(dbmeta.Spanner, versions)
		if err != nil {
			fail("building the metadata: %v", err)
		}
		spannerState.built, spannerState.meta = true, m
		return m
	} else if has {
		dropSpannerFixture(ctx, db, down)
	}

	up, err := spfixture.Everything.ResolveSetup(versions)
	if err != nil {
		fail("resolving the setup: %v", err)
	}
	spannerState.built = true
	if err := runSpannerDDL(ctx, db, spannerQueries(up)); err != nil {
		t.Logf("the batch failed, running each step to find the one: %v", err)
		dropSpannerFixture(ctx, db, down)
		if err := spannerDDLEach(ctx, db, up, false); err != nil {
			fail("setup: %v", err)
		}
		fail("the batch failed and every step passed alone: %v", err)
	}
	m, err := dbmeta.New(dbmeta.Spanner, versions)
	if err != nil {
		fail("building the metadata: %v", err)
	}
	t.Logf("server reports %s, fixture ran %d steps", versions, len(up))
	spannerState.meta = m
	return m
}

// spannerArgs is the filter the fixture's objects sit behind.
func spannerArgs() map[string]any {
	return dbmeta.Args{Schema: spfixture.Everything.Schema}.Map()
}

// TestSpannerVersion reads the version and checks what the model makes of it.
func TestSpannerVersion(t *testing.T) {
	db := openSpanner(t)
	versions, err := dbmeta.Spanner.Version(t.Context(), db)
	if err != nil {
		t.Fatalf("reading the version: %v", err)
	}
	main := versions.Main()
	if main.Unknown || len(main.Parts) != 1 {
		t.Fatalf("expected a version of one number, got %s", versions)
	}
	// Optimizer version 9 is the highest that Spanner Omni 2026.r4-lts supports.
	if main.Compare(dbmeta.V(9)) < 0 {
		t.Errorf("expected optimizer version 9 or newer, got %s", main)
	}
	if display := versions.String(); !strings.HasPrefix(display, "Spanner optimizer ") {
		t.Errorf("expected the product in %q", display)
	}
	t.Logf("server reports %s", versions)
}

// spannerColumns runs the statement and returns the names of its columns. It
// reads every row, because Spanner streams a result and reports an error that
// the plan finds, such as an unsupported function, with the first row and not
// with the first call.
func spannerColumns(t *testing.T, db *sql.DB, query string, vals []any) ([]string, error) {
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
	for first := true; rows.Next(); first = false {
		if first {
			if cols, err = rows.Columns(); err != nil {
				return nil, err
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return cols, rows.Close()
}

// spannerIsOmni reports whether DBMETA_SPANNER names a Spanner Omni server and
// not Cloud Spanner. Omni has no certificate, so its DSN says usePlainText.
func spannerIsOmni() bool {
	return strings.Contains(os.Getenv("DBMETA_SPANNER"), "usePlainText=true")
}

// spannerDrain runs a statement and reads every row, and returns the error that
// either step gave. Spanner reports a plan error with the first row.
func spannerDrain(ctx context.Context, db *sql.DB, stmt string) error {
	rows, err := db.QueryContext(ctx, stmt)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
	}
	return rows.Err()
}

// TestSpannerSmoke runs each registered query against the fixture and checks
// that it returns the columns it declares.
func TestSpannerSmoke(t *testing.T) {
	db := openSpanner(t)
	m := setupSpanner(t, db)

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
		cs, err := spannerColumns(t, db, query, vals)
		if err != nil && q.Name() == dbmeta.CurrentUser.Name() && spannerIsOmni() &&
			strings.Contains(err.Error(), "user name is unknown") {
			// Spanner Omni with no authentication refuses SESSION_USER, and Cloud
			// Spanner answers it. See D219.
			ran++
			continue
		}
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
	if ran != 22 {
		t.Errorf("expected the 22 queries the package comment names to run, %d ran", ran)
	}
	t.Logf("%d queries ran, %d not supported", ran, unsupported)
}

// spannerAll reads every row a query answers, and stops the test on an error.
func spannerAll[T any](t *testing.T, q *dbmeta.Query[T], m *dbmeta.Meta, db *sql.DB, args map[string]any) []T {
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

// TestSpannerFixtureObjects reads the fixture back through the typed API.
func TestSpannerFixtureObjects(t *testing.T) {
	db := openSpanner(t)
	m := setupSpanner(t, db)
	schema := spfixture.Everything.Schema

	// Schemas holds the default schema, which is named empty, and the schema of
	// the fixture. The two Spanner keeps need with_system.
	schemas := map[string]dbmeta.Schema{}
	for _, v := range spannerAll(t, dbmeta.Schemas, m, db, nil) {
		schemas[v.Name] = v
	}
	for _, name := range []string{"", schema} {
		if _, ok := schemas[name]; !ok {
			t.Errorf("expected the schema %q in %v", name, schemas)
		}
	}
	if len(schemas) != 2 {
		t.Errorf("expected only the two schemas of the fixture, got %v", schemas)
	}
	if got := schemas[""].Owner; got != "spanner_admin" {
		t.Errorf("expected the owner spanner_admin for the default schema, got %q", got)
	}
	with := spannerAll(t, dbmeta.Schemas, m, db, dbmeta.Args{WithSystem: true}.Map())
	if len(with) != 4 {
		t.Errorf("expected four schemas with the system ones, got %d", len(with))
	}
	current, ok, err := dbmeta.First(dbmeta.CurrentSchema.All(t.Context(), m, db, nil))
	if err != nil || !ok || current.Name != "" || current.Owner != "spanner_admin" {
		t.Errorf("expected the default schema, got %+v, %v, %v", current, ok, err)
	}

	// Tables: a base table, a view and a synonym, and the table in the default
	// schema, which an empty schema pattern reaches.
	tables := map[string]dbmeta.Table{}
	for _, v := range spannerAll(t, dbmeta.Tables, m, db, nil) {
		tables[v.Schema+"."+v.Name] = v
		if v.Catalog != "" {
			t.Errorf("table %s: expected the empty catalog, got %q", v.Name, v.Catalog)
		}
		if v.Owner.Valid || v.Size.Valid || v.Rows.Valid || v.RowSecurity.Valid {
			t.Errorf("table %s: expected no owner, size, rows or row security, got %+v", v.Name, v)
		}
	}
	for name, typ := range map[string]string{
		"dbmeta_fixture.author": "table", "dbmeta_fixture.book": "table",
		"dbmeta_fixture.region": "table", "dbmeta_fixture.shipment": "table",
		"dbmeta_fixture.chapter": "table", "dbmeta_fixture.sales": "table",
		"dbmeta_fixture.event": "table", "dbmeta_fixture.ledger": "table",
		"dbmeta_fixture.recent": "view", "dbmeta_fixture.old_ledger": "synonym",
		"." + spfixture.Plain: "table",
	} {
		if tables[name].Type != typ {
			t.Errorf("table %s: expected type %q, got %q", name, typ, tables[name].Type)
		}
	}
	if len(tables) != 11 {
		t.Errorf("expected 11 relations, got %d: %v", len(tables), tables)
	}
	for name, want := range map[string]string{
		"dbmeta_fixture.chapter":    "parent_table=book, interleave_type=IN PARENT, on_delete=CASCADE",
		"dbmeta_fixture.event":      "row_deletion_policy=OLDER_THAN(happened, INTERVAL 30 DAY)",
		"dbmeta_fixture.ledger":     "locality_group=" + spfixture.Group,
		"dbmeta_fixture.old_ledger": "locality_group=" + spfixture.Group + ", synonym_for=ledger",
	} {
		if got := tables[name].Options; !got.Valid || got.V != want {
			t.Errorf("table %s: expected the options %q, got %+v", name, want, got)
		}
	}
	if got := tables["dbmeta_fixture.author"].Options; got.Valid {
		t.Errorf("table author: expected no options, got %q", got.V)
	}
	if got := tables["dbmeta_fixture.author"].Persistence; got.V != "permanent" {
		t.Errorf("table author: expected permanent, got %+v", got)
	}
	if got := tables["dbmeta_fixture.recent"].Persistence; got.Valid {
		t.Errorf("view recent: expected no persistence, got %+v", got)
	}
}

// TestSpannerColumnsAndIndexes reads the columns, the indexes and the key
// columns of the fixture.
func TestSpannerColumnsAndIndexes(t *testing.T) {
	db := openSpanner(t)
	m := setupSpanner(t, db)
	args := spannerArgs()

	columns := map[string]dbmeta.Column{}
	for _, v := range spannerAll(t, dbmeta.Columns, m, db, args) {
		columns[v.Table+"."+v.Name] = v
	}
	type want struct {
		ordinal  int
		dataType string
		nullable bool
		key      bool
	}
	for name, w := range map[string]want{
		"author.author_id":   {1, "INT64", false, true},
		"author.name":        {2, "STRING(100)", false, false},
		"author.rating":      {3, "INT64", true, false},
		"author.shade":       {4, "STRING(10)", true, false},
		"book.book_id":       {1, "INT64", false, true},
		"book.published":     {4, "DATE", true, false},
		"region.country":     {1, "STRING(2)", false, true},
		"region.area":        {2, "STRING(20)", false, true},
		"sales.doubled":      {4, "INT64", true, false},
		"sales.tokens":       {8, "TOKENLIST", true, false},
		"chapter.chapter_no": {2, "INT64", false, true},
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
	if got := columns["author.shade"].Default; !got.Valid || got.V != "'red'" {
		t.Errorf("column shade: expected the default 'red', got %+v", got)
	}
	if got := columns["sales.line_id"].Identity; got.V != "by default" {
		t.Errorf("column line_id: expected the identity by default, got %+v", got)
	}
	if got := columns["sales.ticket_id"].Default; !strings.Contains(got.V, "GET_NEXT_SEQUENCE_VALUE") {
		t.Errorf("column ticket_id: expected a default that reads the sequence, got %+v", got)
	}
	for name, kind := range map[string]string{"sales.doubled": "stored", "sales.tokens": "virtual"} {
		if got := columns[name].Generated; got.V != kind {
			t.Errorf("column %s: expected generated %q, got %+v", name, kind, got)
		}
	}
	if got := columns["author.name"].Generated; got.Valid {
		t.Errorf("column name: expected it not generated, got %+v", got)
	}
	for name, c := range columns {
		if c.Comment.Valid || c.Collation.Valid || c.Storage.Valid || c.Compression.Valid || c.StatsTarget.Valid {
			t.Errorf("column %s: expected no comment, collation, storage, compression or statistics target, got %+v", name, c)
		}
	}
	plain := spannerAll(t, dbmeta.Columns, m, db, dbmeta.Args{Parent: spfixture.Plain}.Map())
	if len(plain) != 2 || plain[0].Schema != "" || plain[1].DataType != "ARRAY<FLOAT32>(vector_length=>3)" {
		t.Errorf("expected the two columns of the table in the default schema, got %+v", plain)
	}

	// Indexes.
	indexes := map[string]dbmeta.Index{}
	for _, v := range spannerAll(t, dbmeta.Indexes, m, db, args) {
		indexes[v.Table+"."+v.Name] = v
	}
	for name, w := range map[string]struct {
		typ             string
		unique, primary bool
	}{
		"book.PRIMARY_KEY":             {"PRIMARY_KEY", true, true},
		"book.book_title":              {"INDEX", true, false},
		"book.book_published":          {"INDEX", false, false},
		"book.book_published_filtered": {"INDEX", false, false},
		"sales.sales_search":           {"SEARCH", true, false},
		"chapter.chapter_heading":      {"INDEX", false, false},
	} {
		got, ok := indexes[name]
		if !ok {
			t.Errorf("index %s: not found", name)
			continue
		}
		if got.Type != w.typ || got.Unique != w.unique || got.Primary != w.primary {
			t.Errorf("index %s: expected %+v, got type %q, unique %v, primary %v",
				name, w, got.Type, got.Unique, got.Primary)
		}
		if got.Persistence.V != "permanent" {
			t.Errorf("index %s: expected permanent, got %+v", name, got.Persistence)
		}
	}
	if got := indexes["book.book_published_filtered"].Predicate; got.V != "published IS NOT NULL" {
		t.Errorf("index book_published_filtered: expected the predicate, got %+v", got)
	}
	if got := indexes["book.book_title"].Predicate; got.Valid {
		t.Errorf("index book_title: expected no predicate, got %+v", got)
	}
	if got := indexes["book.PRIMARY_KEY"].Valid; got.Valid {
		t.Errorf("the primary key: expected no state, got %+v", got)
	}
	if got := indexes["book.book_title"].Valid; !got.Valid || !got.V {
		t.Errorf("index book_title: expected a valid index, got %+v", got)
	}
	if got := indexes["chapter.chapter_heading"].Options; got.V != "interleave_in=book" {
		t.Errorf("index chapter_heading: expected interleave_in=book, got %+v", got)
	}
	var managed int
	for name, v := range indexes {
		if strings.HasPrefix(v.Name, "IDX_") {
			managed++
			if v.Options.V != "managed=true" {
				t.Errorf("index %s: expected managed=true, got %+v", name, v.Options)
			}
		}
	}
	if managed != 2 {
		t.Errorf("expected the two indexes that back the foreign keys, got %d", managed)
	}
	vector, ok, err := dbmeta.First(dbmeta.Indexes.All(t.Context(), m, db, dbmeta.Args{Name: spfixture.Vector}.Map()))
	if err != nil || !ok || vector.Type != "VECTOR" || vector.Schema != "" ||
		!strings.Contains(vector.Options.V, "distance_type=COSINE") ||
		vector.Predicate.V != "embedding IS NOT NULL" {
		t.Errorf("expected the vector index in the default schema, got %+v, %v, %v", vector, ok, err)
	}

	// The columns of an index. A stored column has no ordinal in Spanner and is
	// numbered after the keys.
	cols := map[string]dbmeta.IndexColumn{}
	for _, v := range spannerAll(t, dbmeta.IndexColumns, m, db, args) {
		cols[v.Table+"."+v.Index+"."+v.Name.V] = v
	}
	for name, w := range map[string]struct {
		ordinal int64
		desc    sql.Null[bool]
		include bool
	}{
		"book.book_published.published":          {1, sql.Null[bool]{V: true, Valid: true}, false},
		"book.book_published_filtered.published": {1, sql.Null[bool]{V: false, Valid: true}, false},
		"book.book_published_filtered.title":     {2, sql.Null[bool]{}, true},
		"chapter.chapter_heading.book_id":        {1, sql.Null[bool]{V: false, Valid: true}, false},
		"chapter.chapter_heading.heading":        {2, sql.Null[bool]{V: false, Valid: true}, false},
		"sales.PRIMARY_KEY.region":               {2, sql.Null[bool]{V: false, Valid: true}, false},
	} {
		got, ok := cols[name]
		if !ok {
			t.Errorf("index column %s: not found", name)
			continue
		}
		if got.Ordinal != w.ordinal || got.Descending != w.desc || got.Include != w.include || got.Expression.Valid {
			t.Errorf("index column %s: expected %+v, got %+v", name, w, got)
		}
	}
}

// TestSpannerKeysAndConstraints reads the constraints, the columns of a key and
// the NOT NULL constraints.
func TestSpannerKeysAndConstraints(t *testing.T) {
	db := openSpanner(t)
	m := setupSpanner(t, db)
	args := spannerArgs()

	constraints := map[string]dbmeta.Constraint{}
	for _, v := range spannerAll(t, dbmeta.Constraints, m, db, args) {
		constraints[v.Table+"."+v.Name] = v
		if strings.HasPrefix(v.Name, "CK_IS_NOT_NULL_") {
			t.Errorf("constraint %s: expected the NOT NULL checks to be left to NotNulls", v.Name)
		}
		if !v.Enforced.Valid || !v.Enforced.V || v.Deferrable || v.Deferred {
			t.Errorf("constraint %s: expected an enforced constraint that cannot defer, got %+v", v.Name, v)
		}
	}
	for name, typ := range map[string]string{
		"author.PK_author": "primary key", "author.shade_check": "check",
		"book.book_author_fk": "foreign key", "book.title_not_empty": "check",
		"shipment.shipment_region_fk": "foreign key", "chapter.PK_chapter": "primary key",
	} {
		if got := constraints[name].Type; got != typ {
			t.Errorf("constraint %s: expected %q, got %q", name, typ, got)
		}
	}
	if got := constraints["author.shade_check"].Definition; got.V != "shade IN ('red', 'green', 'blue')" {
		t.Errorf("constraint shade_check: expected the clause, got %+v", got)
	}
	if got := constraints["book.book_author_fk"].Definition; got.Valid {
		t.Errorf("constraint book_author_fk: expected no definition, got %+v", got)
	}
	for _, v := range constraints {
		if v.Type == "unique" {
			t.Errorf("constraint %s: Spanner has no unique constraint", v.Name)
		}
	}

	// Constraint columns: a composite foreign key, the columns it points at, and
	// the order of both.
	cols := map[string]dbmeta.ConstraintColumn{}
	for _, v := range spannerAll(t, dbmeta.ConstraintColumns, m, db, args) {
		cols[fmt.Sprintf("%s.%s.%d", v.Table, v.Constraint, v.Ordinal)] = v
	}
	for name, w := range map[string][3]string{
		"shipment.shipment_region_fk.1": {"country", "region", "country"},
		"shipment.shipment_region_fk.2": {"area", "region", "area"},
		"book.book_author_fk.1":         {"author_id", "author", "author_id"},
	} {
		got, ok := cols[name]
		if !ok {
			t.Errorf("constraint column %s: not found", name)
			continue
		}
		if got.Name != w[0] || got.ForeignTable.V != w[1] || got.ForeignName.V != w[2] ||
			got.ForeignSchema.V != spfixture.Everything.Schema {
			t.Errorf("constraint column %s: expected %v, got %+v", name, w, got)
		}
	}
	if got := cols["region.PK_region.2"]; got.Name != "area" || got.ForeignTable.Valid {
		t.Errorf("constraint column PK_region.2: expected area and no target, got %+v", got)
	}
	if _, ok := cols["author.shade_check.1"]; ok {
		t.Error("expected no row for a check constraint, which KEY_COLUMN_USAGE does not hold")
	}

	// NOT NULL constraints.
	notNulls := map[string]dbmeta.NotNull{}
	for _, v := range spannerAll(t, dbmeta.NotNulls, m, db, args) {
		notNulls[v.Table+"."+v.Column] = v
	}
	for _, name := range []string{"author.author_id", "author.name", "book.title", "shipment.amount"} {
		got, ok := notNulls[name]
		if !ok || !got.Validated || got.NoInherit || !got.Local || got.Inherited ||
			!strings.HasPrefix(got.Name, "CK_IS_NOT_NULL_") {
			t.Errorf("not null %s: expected a validated local constraint, got %+v, %v", name, got, ok)
		}
	}
	if _, ok := notNulls["author.rating"]; ok {
		t.Error("expected no NOT NULL constraint on a nullable column")
	}
}

// TestSpannerRoutinesAndSequences reads the view, the sequence, the function and
// its parameters.
func TestSpannerRoutinesAndSequences(t *testing.T) {
	db := openSpanner(t)
	m := setupSpanner(t, db)
	args := spannerArgs()
	schema := spfixture.Everything.Schema

	view, ok, err := dbmeta.First(dbmeta.Views.All(t.Context(), m, db, dbmeta.Args{Name: "recent"}.Map()))
	if err != nil || !ok || view.Schema != schema ||
		!strings.Contains(view.Definition.V, "FROM dbmeta_fixture.book AS b") {
		t.Errorf("expected the view recent with its query, got %+v, %v, %v", view, ok, err)
	}
	if view.CheckOption.Valid || view.Updatable.Valid || view.Insertable.Valid || view.Comment.Valid {
		t.Errorf("expected no check option, updatable, insertable or comment, got %+v", view)
	}

	seq, ok, err := dbmeta.First(dbmeta.Sequences.All(t.Context(), m, db, args))
	if err != nil || !ok || seq.Name != "ticket" || seq.DataType.V != "INT64" || seq.Start.V != "1000" {
		t.Errorf("expected the sequence ticket starting at 1000, got %+v, %v, %v", seq, ok, err)
	}
	if seq.Minimum.Valid || seq.Maximum.Valid || seq.Increment.Valid || seq.Cycles.Valid || seq.CacheSize.Valid {
		t.Errorf("expected no bounds, step, cycle or cache, got %+v", seq)
	}

	// A function written with CREATE FUNCTION, and the table function that
	// Spanner makes for a change stream in the default schema.
	fns := map[string]dbmeta.Function{}
	for _, v := range spannerAll(t, dbmeta.Functions, m, db, nil) {
		fns[v.Schema+"."+v.Name] = v
	}
	double := fns[schema+".double_it"]
	if double.Kind != "function" || double.ResultType.V != "INT64" || double.ArgTypes.V != "INT64" ||
		double.Language != "sql" || double.Security != "invoker" || double.Source.V != "x * 2" {
		t.Errorf("expected the function double_it, got %+v", double)
	}
	if double.Owner.Valid || double.Definition.Valid || double.Prosrc.Valid || double.Leakproof ||
		double.Volatility != "" || double.Parallel != "" {
		t.Errorf("expected no owner, definition, prosrc, volatility or parallel, got %+v", double)
	}
	stream := fns[".READ_dbmeta_book_changes"]
	if stream.Kind != "table function" || !strings.HasPrefix(stream.ResultType.V, "TABLE<ChangeRecord") ||
		stream.ArgTypes.V != strings.Join([]string{"TIMESTAMP", "TIMESTAMP", "STRING(MAX)", "INT64", "ARRAY<STRING(MAX)>"}, ", ") {
		t.Errorf("expected the table function of the change stream, got %+v", stream)
	}
	if _, _, err := dbmeta.First(dbmeta.Aggregates.All(t.Context(), m, db, nil)); !errors.Is(err, dbmeta.ErrNotSupported) {
		t.Errorf("expected Aggregates to be unsupported, got %v", err)
	}

	params := spannerAll(t, dbmeta.RoutineParameters, m, db, dbmeta.Args{Schema: schema}.Map())
	if len(params) != 1 || params[0].Routine != "double_it" || params[0].Name.V != "x" ||
		params[0].Ordinal != 1 || params[0].Mode != "in" || params[0].DataType != "INT64" ||
		params[0].Default.Valid {
		t.Errorf("expected the one parameter x of double_it, got %+v", params)
	}
	all := spannerAll(t, dbmeta.RoutineParameters, m, db, dbmeta.Args{Parent: "READ_dbmeta_book_changes"}.Map())
	if len(all) != 5 || all[0].Name.V != "start_timestamp" || all[4].Name.V != "read_options" {
		t.Errorf("expected the five parameters of the change stream function, got %+v", all)
	}
}

// TestSpannerRolesAndPrivileges reads the roles, the grants and the settings.
func TestSpannerRolesAndPrivileges(t *testing.T) {
	db := openSpanner(t)
	m := setupSpanner(t, db)
	schema := spfixture.Everything.Schema

	roles := map[string]dbmeta.Role{}
	for _, v := range spannerAll(t, dbmeta.Roles, m, db, nil) {
		roles[v.Name] = v
	}
	if len(roles) != 3 || roles[spfixture.Staff].MemberOf != spfixture.Reader || roles[spfixture.Reader].MemberOf != "" {
		t.Errorf("expected the three roles of the fixture, and the staff in the reader, got %+v", roles)
	}
	for name, r := range roles {
		if r.Superuser || r.CreateRole || r.CreateDB || r.CanLogin || r.Replication || r.BypassRLS ||
			!r.Inherit || r.ConnLimit != -1 || r.ValidUntil.Valid || r.Comment.Valid {
			t.Errorf("role %s: expected the fixed answers, got %+v", name, r)
		}
	}
	system := spannerAll(t, dbmeta.Roles, m, db, dbmeta.Args{WithSystem: true}.Map())
	if len(system) != 6 {
		t.Errorf("expected public and the two readers with the system roles, got %d", len(system))
	}
	grants := spannerAll(t, dbmeta.RoleGrants, m, db, nil)
	if len(grants) != 1 || grants[0].Role != spfixture.Staff || grants[0].MemberOf != spfixture.Reader ||
		grants[0].Grantor.Valid || grants[0].Admin || !grants[0].Inherit || grants[0].Set {
		t.Errorf("expected the staff to belong to the reader, got %+v", grants)
	}

	privileges := map[string]dbmeta.Privilege{}
	for _, v := range spannerAll(t, dbmeta.Privileges, m, db, nil) {
		privileges[v.Schema.V+"."+v.Name] = v
		if v.Policies.Valid {
			t.Errorf("privilege %s: expected no policies, got %+v", v.Name, v.Policies)
		}
	}
	reader := spfixture.Reader
	for name, w := range map[string][2]string{
		schema + ".author":          {"table", reader + "=SELECT"},
		schema + ".recent":          {"view", reader + "=SELECT"},
		".dbmeta_book_changes":      {"change stream", reader + "=SELECT"},
		".READ_dbmeta_book_changes": {"table function", reader + "=EXECUTE"},
		schema + ".book":            {"table", ""},
		schema + ".double_it":       {"function", ""},
		schema + ".old_ledger":      {"synonym", ""},
		".dbmeta_all_changes":       {"change stream", ""},
		"." + spfixture.Plain:       {"table", ""},
	} {
		got, ok := privileges[name]
		if !ok {
			t.Errorf("privilege %s: not found", name)
			continue
		}
		if got.Type != w[0] || got.Access.V != w[1] || got.Access.Valid != (w[1] != "") {
			t.Errorf("privilege %s: expected type %q and access %q, got %+v", name, w[0], w[1], got)
		}
	}
	if got := privileges[schema+".book"].ColumnAccess; got.V != "title:"+reader+"=SELECT" {
		t.Errorf("privilege book: expected the column grant, got %+v", got)
	}
	if got := privileges[schema+".author"].ColumnAccess; got.Valid {
		t.Errorf("privilege author: expected the grant on the whole table to stay out of the column access, got %+v", got)
	}

	cp := spannerAll(t, dbmeta.ColumnPrivileges, m, db, nil)
	if len(cp) != 1 || cp[0].Table != "book" || cp[0].Column != "title" || cp[0].Grantee.V != reader ||
		cp[0].Privileges != "SELECT" || cp[0].Access != reader+"=SELECT" || cp[0].Ordinal != 1 || cp[0].Grantor.Valid {
		t.Errorf("expected the one grant on book.title, got %+v", cp)
	}

	settings := map[string]dbmeta.Setting{}
	for _, v := range spannerAll(t, dbmeta.Settings, m, db, nil) {
		settings[v.Name] = v
	}
	if got := settings["database_dialect"]; got.Value.V != "GOOGLE_STANDARD_SQL" || got.Type.V != "STRING" {
		t.Errorf("expected the GoogleSQL dialect, got %+v", got)
	}
}

// TestSpannerChangeStreamsAndLocalityGroups reads the publications and the
// tablespaces, which are analogues. See D216.
func TestSpannerChangeStreamsAndLocalityGroups(t *testing.T) {
	db := openSpanner(t)
	m := setupSpanner(t, db)

	pubs := map[string]dbmeta.Publication{}
	for _, v := range spannerAll(t, dbmeta.Publications, m, db, nil) {
		pubs[v.Name] = v
		if v.Owner != "" || v.Truncate || v.ViaRoot || v.Comment.Valid || v.GeneratedColumns.Valid {
			t.Errorf("publication %s: expected the fixed answers, got %+v", v.Name, v)
		}
	}
	if len(pubs) != 3 {
		t.Errorf("expected the three change streams, got %v", pubs)
	}
	book := pubs["dbmeta_book_changes"]
	if book.AllTables || !book.Insert || !book.Update || book.Delete {
		t.Errorf("expected a stream that leaves deletes out, got %+v", book)
	}
	all := pubs["dbmeta_all_changes"]
	if !all.AllTables || !all.Insert || !all.Update || !all.Delete {
		t.Errorf("expected a stream for everything, got %+v", all)
	}

	tables := map[string]dbmeta.PublicationTable{}
	for _, v := range spannerAll(t, dbmeta.PublicationTables, m, db, nil) {
		tables[v.Publication+"."+v.Name] = v
		if v.Where.Valid || v.Via.V != "table" || v.Schema != spfixture.Everything.Schema {
			t.Errorf("publication table %s: expected a table of the fixture, got %+v", v.Name, v)
		}
	}
	for name, columns := range map[string]string{
		"dbmeta_book_changes.book":   "(title)",
		"dbmeta_book_changes.author": "",
		"dbmeta_key_changes.region":  "()",
	} {
		got, ok := tables[name]
		if !ok || got.Columns != columns {
			t.Errorf("publication table %s: expected the columns %q, got %+v, %v", name, columns, got, ok)
		}
	}
	if len(tables) != 3 {
		t.Errorf("expected three tables, got %v", tables)
	}

	spaces := map[string]dbmeta.Tablespace{}
	for _, v := range spannerAll(t, dbmeta.Tablespaces, m, db, nil) {
		spaces[v.Name] = v
		if v.Owner.Valid || v.Location.Valid || v.Size.Valid || v.Access.Valid || v.Comment.Valid {
			t.Errorf("tablespace %s: expected only a name and options, got %+v", v.Name, v)
		}
	}
	if got := spaces[spfixture.Group].Options; got.V != "storage=hdd" {
		t.Errorf("expected the options storage=hdd, got %+v", got)
	}
	// Cloud Spanner reports storage=ssd for the default group, and Spanner Omni
	// reports no option for it.
	got, ok := spaces["default"]
	if !ok || (got.Options.Valid && got.Options.V != "storage=ssd") {
		t.Errorf("expected the default group with no options or storage=ssd, got %+v, %v", got, ok)
	}
}

// TestSpannerUnanswered checks the kinds that Spanner cannot answer, and the
// functions that it has no source for, so that each absence stays a decision.
// See D216.
func TestSpannerUnanswered(t *testing.T) {
	db := openSpanner(t)
	m := setupSpanner(t, db)
	ctx := t.Context()
	for _, q := range []dbmeta.AnyQuery{
		dbmeta.Databases, dbmeta.Inherits, dbmeta.Policies, dbmeta.Triggers,
		dbmeta.Partitions, dbmeta.PartitionedTables, dbmeta.Comments, dbmeta.ColumnStats,
		dbmeta.Types, dbmeta.EnumValues, dbmeta.Rules, dbmeta.Extensions, dbmeta.Collations,
	} {
		if q.Support(m) != dbmeta.NotSupported {
			t.Errorf("%s: expected it to be unsupported on Spanner", q.Name())
		}
	}

	// Spanner Omni has no function that reads the release or the user name, and
	// the model's answer to both waits for one.
	for _, stmt := range []string{"SELECT VERSION()", "SELECT CURRENT_USER()"} {
		if err := spannerDrain(ctx, db, stmt); err == nil {
			t.Errorf("%s: expected Spanner to refuse it", stmt)
		}
	}
	// SESSION_USER is the one that differs. Cloud Spanner answers it and Spanner
	// Omni with no authentication refuses it. See D219.
	err := spannerDrain(ctx, db, "SELECT SESSION_USER()")
	switch {
	case spannerIsOmni() && err == nil:
		t.Error("SELECT SESSION_USER(): expected Spanner Omni to refuse it")
	case !spannerIsOmni() && err != nil:
		t.Errorf("SELECT SESSION_USER(): expected Cloud Spanner to answer it, got %v", err)
	}
	// A column that a descriptor holds as one binary value, and no SQL reads.
	var n int64
	if err := db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM information_schema.schemata WHERE proto_bundle IS NOT NULL AND ARRAY_LENGTH(proto_bundle) > 0").
		Scan(&n); err == nil && n != 0 {
		t.Errorf("expected no proto bundle in the fixture, got %d", n)
	}
}

// TestSpannerSystemSchemas checks with_system, and that a schema pattern
// selects one schema.
func TestSpannerSystemSchemas(t *testing.T) {
	db := openSpanner(t)
	m := setupSpanner(t, db)

	system := spannerAll(t, dbmeta.Tables, m, db, dbmeta.Args{WithSystem: true, Schema: "SPANNER\\_SYS"}.Map())
	if len(system) < 40 {
		t.Errorf("expected the views of SPANNER_SYS, got %d", len(system))
	}
	for _, v := range system {
		if v.Schema != "SPANNER_SYS" || v.Type != "view" {
			t.Errorf("expected a view of SPANNER_SYS, got %+v", v)
		}
	}
	info := spannerAll(t, dbmeta.Tables, m, db, dbmeta.Args{WithSystem: true, Schema: "INFORMATION\\_SCHEMA", Name: "TABLES"}.Map())
	if len(info) != 1 {
		t.Errorf("expected the one view TABLES, got %+v", info)
	}
	for _, v := range spannerAll(t, dbmeta.Tables, m, db, dbmeta.Args{Types: []string{"view", "synonym"}}.Map()) {
		if v.Type != "view" && v.Type != "synonym" {
			t.Errorf("expected a view or a synonym, got %+v", v)
		}
	}
	if got := spannerAll(t, dbmeta.Tables, m, db, dbmeta.Args{Name: "auth%"}.Map()); len(got) != 1 {
		t.Errorf("expected the one table that begins with auth, got %+v", got)
	}
}

// TestSpannerEnforcesRoles checks that a role is enforced, because the parity
// test depends on it. A session that names a role that does not exist is
// refused, on Cloud Spanner and on Spanner Omni, and a session that names one
// that exists sees what the role was granted. The property is database_role. The
// name role is not a property of go-sql-spanner, and the driver ignores it, so
// the first version of this test read everything and said that Omni enforced no
// role. See D219.
func TestSpannerEnforcesRoles(t *testing.T) {
	admin := openSpanner(t)
	m := setupSpanner(t, admin)
	dsn := os.Getenv("DBMETA_SPANNER")

	// openAt pings and fails the test on a refusal, so this opens the pool itself.
	db, err := sql.Open("spanner", dsn+";database_role=dbmeta_no_such_role")
	if err != nil {
		t.Fatalf("opening: %v", err)
	}
	defer db.Close()
	if err := spannerDrain(t.Context(), db, "SELECT 1"); err == nil || !strings.Contains(err.Error(), "Role not found") {
		t.Errorf("expected a role that does not exist to be refused, got %v", err)
	}

	stranger := openAt(t, "spanner", dsn+";database_role="+spfixture.Stranger)
	if err := spannerDrain(t.Context(), stranger, "SELECT COUNT(*) FROM dbmeta_fixture.region"); err == nil {
		t.Error("expected a role with no grant to be refused a table")
	}
	reader := openAt(t, "spanner", dsn+";database_role="+spfixture.Reader)
	if err := spannerDrain(t.Context(), reader, "SELECT COUNT(*) FROM dbmeta_fixture.author"); err != nil {
		t.Errorf("expected the reader to read the table it holds SELECT on, got %v", err)
	}
	if err := spannerDrain(t.Context(), reader, "SELECT COUNT(*) FROM dbmeta_fixture.region"); err == nil {
		t.Error("expected the reader to be refused a table it holds no grant on")
	}
	want := spannerAll(t, dbmeta.Tables, m, admin, spannerArgs())
	got := spannerAll(t, dbmeta.Tables, m, reader, spannerArgs())
	t.Logf("the administrator reads %d relations and the reader reads %d", len(want), len(got))
}

// TestSpannerCurrentUser reads the IAM principal on Cloud Spanner, and the
// refusal on Spanner Omni. See D219.
func TestSpannerCurrentUser(t *testing.T) {
	db := openSpanner(t)
	m := setupSpanner(t, db)
	user, ok, err := dbmeta.First(dbmeta.CurrentUser.All(t.Context(), m, db, nil))
	if spannerIsOmni() {
		if err == nil || !strings.Contains(err.Error(), "user name is unknown") {
			t.Errorf("expected Spanner Omni to refuse SESSION_USER, got %+v, %v, %v", user, ok, err)
		}
		return
	}
	if err != nil || !ok || !strings.Contains(user.Name, "@") || user.Session.Valid {
		t.Errorf("expected the IAM principal and no session, got %+v, %v, %v", user, ok, err)
	}
}
