package test

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	_ "github.com/duckdb/duckdb-go/v2"

	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/models/duckdb"
	dkfixture "github.com/xo/dbmeta/models/duckdb/fixture"
)

// DuckDB needs no server and no environment variable, like SQLite. It is a
// library, so the release under test is whichever one the driver was built
// with, and this test never skips. See D42.
//
// The driver is github.com/duckdb/duckdb-go/v2, which is the one usql uses.
// D52 requires that. It needs cgo, which the test module may use and the root
// module may not. See D48.
//
// The file is not named after the fixture schema. DuckDB names the catalog
// after the file, and a catalog and a schema of the same name make a qualified
// reference ambiguous.
func openDuckDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("duckdb", filepath.Join(t.TempDir(), "dbm.duckdb"))
	if err != nil {
		t.Fatalf("opening: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.PingContext(t.Context()); err != nil {
		t.Fatalf("connecting: %v", err)
	}
	return db
}

func setupDuckDB(t *testing.T, db *sql.DB) *dbmeta.Meta {
	t.Helper()
	versions, err := dbmeta.DuckDB.Version(t.Context(), db)
	if err != nil {
		t.Fatalf("reading the version: %v", err)
	}
	m, err := dbmeta.New(dbmeta.DuckDB, versions)
	if err != nil {
		t.Fatalf("building the meta: %v", err)
	}
	run := func(ctx context.Context, steps []dkfixture.Result, fatal bool) {
		for _, s := range steps {
			if _, err := db.ExecContext(ctx, s.Query); err != nil && fatal {
				t.Fatalf("%s: %v\n%s", s.Name, err, s.Query)
			}
		}
	}
	up, err := dkfixture.Everything.ResolveSetup(versions)
	if err != nil {
		t.Fatalf("resolving the setup: %v", err)
	}
	down, err := dkfixture.Everything.ResolveTeardown(versions)
	if err != nil {
		t.Fatalf("resolving the teardown: %v", err)
	}
	run(t.Context(), up, true)
	t.Cleanup(func() { run(context.Background(), down, false) })
	t.Logf("library reports %s, fixture ran %d steps", m, len(up))
	return m
}

func dkArgs() map[string]any {
	return dbmeta.Args{Schema: dkfixture.Everything.Schema}.Map()
}

// TestDuckDBEveryQueryRuns executes every query DuckDB answers and checks the
// columns match the declared fields.
func TestDuckDBEveryQueryRuns(t *testing.T) {
	db := openDuckDB(t)
	m := setupDuckDB(t, db)

	var ran, unsupported int
	var names []string
	for _, q := range dbmeta.Queries() {
		if q.Support(m) != dbmeta.Supported {
			unsupported++
			continue
		}
		query, vals, err := q.Build(m, nil)
		if err != nil {
			t.Errorf("%s: rendering: %v", q.Name(), err)
			continue
		}
		cols, err := columnsOf(t, db, query, vals)
		if err != nil {
			t.Errorf("%s: executing: %v\n%s", q.Name(), err, query)
			continue
		}
		fields, err := q.Fields(m)
		if err != nil {
			t.Errorf("%s: reading fields: %v", q.Name(), err)
			continue
		}
		if len(cols) != len(fields) {
			t.Errorf("%s: declares %d fields and returns %d columns", q.Name(), len(fields), len(cols))
			continue
		}
		for i := range cols {
			if cols[i] != fields[i].Name {
				t.Errorf("%s: column %d is %q and the field is %q", q.Name(), i, cols[i], fields[i].Name)
			}
		}
		ran++
		names = append(names, q.Name())
	}
	t.Logf("%d of %d queries ran: %s", ran, ran+unsupported, strings.Join(names, " "))
	if ran == 0 {
		t.Fatal("expected at least one query to run")
	}
}

// TestDuckDBScanning reads rows through the typed API, which the rendering
// test cannot check.
func TestDuckDBScanning(t *testing.T) {
	db := openDuckDB(t)
	m := setupDuckDB(t, db)
	ctx := t.Context()

	want := map[string]string{
		"author": "table", "book": "table", "region": "table",
		"shipment": "table", "recent": "view",
	}
	found := make(map[string]string)
	for v, err := range dbmeta.Tables.All(ctx, m, db, dkArgs()) {
		if err != nil {
			t.Fatalf("reading tables: %v", err)
		}
		found[v.Name] = v.Type
	}
	for name, kind := range want {
		if found[name] != kind {
			t.Errorf("expected %q to be a %s, got %q", name, kind, found[name])
		}
	}

	cols := make(map[string]dbmeta.Column)
	for v, err := range dbmeta.Columns.All(ctx, m, db, dkArgs()) {
		if err != nil {
			t.Fatalf("reading columns: %v", err)
		}
		cols[v.Table+"."+v.Name] = v
	}
	if got := cols["author.author_id"]; !got.PrimaryKey || got.Nullable {
		t.Errorf("expected author_id to be a non null primary key, got %+v", got)
	}
	if got := cols["author.rating"]; got.PrimaryKey || !got.Nullable {
		t.Errorf("expected rating to be nullable and not a key, got %+v", got)
	}
	// the fixture comments the table and one column, and nothing else
	if got := cols["author.author_id"]; !got.Comment.Valid || got.Comment.V != "surrogate key" {
		t.Errorf("expected the column comment, got %v", got.Comment)
	}
	if got := cols["author.name"]; got.Comment.Valid {
		t.Errorf("expected no comment on an uncommented column, got %v", got.Comment)
	}
}

// TestDuckDBConstraints covers the constraint catalog, which is unusually good
// for an embedded database: it names the columns of a key and the columns they
// reference, so a composite foreign key comes back whole.
func TestDuckDBConstraints(t *testing.T) {
	db := openDuckDB(t)
	m := setupDuckDB(t, db)
	ctx := t.Context()

	byType := make(map[string]int)
	for v, err := range dbmeta.Constraints.All(ctx, m, db, dkArgs()) {
		if err != nil {
			t.Fatalf("reading constraints: %v", err)
		}
		byType[v.Type]++
		if v.Type == "not null" {
			t.Errorf("expected no NOT NULL constraint row, got %s.%s. See D49", v.Table, v.Name)
		}
	}
	for _, kind := range []string{"primary key", "unique", "foreign key", "check"} {
		if byType[kind] == 0 {
			t.Errorf("expected at least one %s constraint, got %v", kind, byType)
		}
	}

	type ref struct{ col, ftable, fcol string }
	got := make(map[string][]ref)
	for v, err := range dbmeta.ConstraintColumns.All(ctx, m, db, dkArgs()) {
		if err != nil {
			t.Fatalf("reading constraint columns: %v", err)
		}
		key := v.Table + "." + v.Constraint
		if int64(len(got[key]))+1 != v.Ordinal {
			t.Errorf("%s: expected ordinal %d, got %d", key, len(got[key])+1, v.Ordinal)
		}
		got[key] = append(got[key], ref{v.Name, v.ForeignTable.V, v.ForeignName.V})
	}
	// The composite foreign key, in order, each column paired with the one it
	// points at. It is found by the table rather than by name, because DuckDB
	// generates its own constraint name and ignores the one the fixture gave:
	// shipment_region_fk becomes shipment_country_area_country_area_fkey.
	var fk []ref
	for key, cols := range got {
		if strings.HasPrefix(key, "shipment.") && cols[0].ftable == "region" {
			fk = cols
		}
	}
	want := []ref{{"country", "region", "country"}, {"area", "region", "area"}}
	if !slices.Equal(fk, want) {
		t.Errorf("expected %v, got %v", want, fk)
	}
	// and no NOT NULL rows here either
	for key := range got {
		if strings.HasSuffix(key, "_not_null") {
			t.Errorf("expected no NOT NULL constraint columns, got %s", key)
		}
	}
}

// TestDuckDBLists covers the four kinds read by unnesting a list with
// ordinality, which is the shape this catalog uses where PostgreSQL uses an
// array.
func TestDuckDBLists(t *testing.T) {
	db := openDuckDB(t)
	m := setupDuckDB(t, db)
	ctx := t.Context()

	var labels []string
	for v, err := range dbmeta.EnumValues.All(ctx, m, db, dkArgs()) {
		if err != nil {
			t.Fatalf("reading enum values: %v", err)
		}
		if v.Enum != "colour" {
			continue
		}
		labels = append(labels, v.Label)
		if v.Ordinal != int64(len(labels)) {
			t.Errorf("%s: expected ordinal %d, got %d", v.Label, len(labels), v.Ordinal)
		}
	}
	if want := []string{"red", "green", "blue"}; !slices.Equal(labels, want) {
		t.Errorf("expected %v in declaration order, got %v", want, labels)
	}

	// IndexColumns is not one of them, and it is the one place DuckDB looks
	// like it can answer and cannot: duckdb_indexes.expressions prints as a
	// list and its type is VARCHAR. The index itself is listed.
	var indexed bool
	for v, err := range dbmeta.Indexes.All(ctx, m, db, dkArgs()) {
		if err != nil {
			t.Fatalf("reading indexes: %v", err)
		}
		indexed = indexed || v.Name == "book_published"
	}
	if !indexed {
		t.Error("expected the fixture index to be listed")
	}
	if got := dbmeta.IndexColumns.Support(m); got != dbmeta.NotSupported {
		t.Errorf("expected index columns to be unsupported, got %v", got)
	}

	// the macro's parameters, which is the only routine a caller can create
	var params []string
	a := dbmeta.Args{Schema: dkfixture.Everything.Schema, Parent: "addup"}.Map()
	for v, err := range dbmeta.RoutineParameters.All(ctx, m, db, a) {
		if err != nil {
			t.Fatalf("reading routine parameters: %v", err)
		}
		params = append(params, v.Name.V)
		if v.Mode != "in" {
			t.Errorf("expected mode in, got %q", v.Mode)
		}
	}
	if want := []string{"a", "b"}; !slices.Equal(params, want) {
		t.Errorf("expected %v, got %v", want, params)
	}
}

// TestDuckDBCatalogExtras covers the kinds DuckDB answers that most embedded
// databases do not: a comment catalog gathered from five places, real
// extensions, and a sequence with its bounds.
func TestDuckDBCatalogExtras(t *testing.T) {
	db := openDuckDB(t)
	m := setupDuckDB(t, db)
	ctx := t.Context()

	kinds := make(map[string]string)
	for v, err := range dbmeta.Comments.All(ctx, m, db, dkArgs()) {
		if err != nil {
			t.Fatalf("reading comments: %v", err)
		}
		kinds[v.Name] = v.Type
	}
	if kinds["author"] != "table" {
		t.Errorf("expected the table comment, got %v", kinds)
	}
	if kinds["author.author_id"] != "column" {
		t.Errorf("expected the column comment, got %v", kinds)
	}

	seq, ok, err := dbmeta.First(dbmeta.Sequences.All(ctx, m, db, dkArgs()))
	if err != nil {
		t.Fatalf("reading sequences: %v", err)
	}
	if !ok || seq.Name != "counter" {
		t.Fatalf("expected the fixture sequence, got %q ok=%v", seq.Name, ok)
	}
	if seq.Start.V != 10 || seq.Increment.V != 2 || seq.Cycles.V {
		t.Errorf("expected start 10 increment 2 and no cycle, got %+v", seq)
	}

	var extensions int
	for _, err := range dbmeta.Extensions.All(ctx, m, db, nil) {
		if err != nil {
			t.Fatalf("reading extensions: %v", err)
		}
		extensions++
	}
	if extensions == 0 {
		t.Error("expected the extensions DuckDB loads by itself")
	}

	v, ok, err := dbmeta.First(dbmeta.CurrentSchema.All(ctx, m, db, nil))
	if err != nil {
		t.Fatalf("reading the current schema: %v", err)
	}
	if !ok || v.Name != "main" {
		t.Errorf("expected main, got %q ok=%v", v.Name, ok)
	}
}

// TestDuckDBUnsupported checks that what DuckDB does not have says so, rather
// than answering with an empty result. D34 requires that difference.
func TestDuckDBUnsupported(t *testing.T) {
	db := openDuckDB(t)
	m := setupDuckDB(t, db)
	for _, q := range []dbmeta.AnyQuery{
		// DuckDB is embedded and single process, so none of these exist
		dbmeta.Roles, dbmeta.RoleGrants, dbmeta.Privileges, dbmeta.RoleSettings,
		dbmeta.DefaultACLs, dbmeta.Triggers, dbmeta.Tablespaces,
		// no replication, no text search objects, no operator catalog
		dbmeta.Publications, dbmeta.Subscriptions, dbmeta.TextSearchConfigs,
		dbmeta.Operators, dbmeta.OperatorClasses, dbmeta.Casts, dbmeta.Domains,
		dbmeta.Languages, dbmeta.LargeObjects, dbmeta.EventTriggers,
		// analogues that were named by a review and rejected. See docs/COVERAGE.md.
		dbmeta.ColumnStats, dbmeta.ExtensionObjects, dbmeta.ForeignTables,
		dbmeta.ForeignServers, dbmeta.PartitionedTables,
	} {
		if got := q.Support(m); got != dbmeta.NotSupported {
			t.Errorf("%s: expected it to be reported unsupported, got %v", q.Name(), got)
		}
		if _, _, err := q.Build(m, nil); !errors.Is(err, dbmeta.ErrNotSupported) {
			t.Errorf("%s: expected ErrNotSupported, got: %v", q.Name(), err)
		}
	}
}

// TestDuckDBVersion checks that the version comes from the library.
func TestDuckDBVersion(t *testing.T) {
	db := openDuckDB(t)
	versions, err := dbmeta.DuckDB.Version(t.Context(), db)
	if err != nil {
		t.Fatalf("reading the version: %v", err)
	}
	if versions.Main().Unknown {
		t.Fatal("expected a version")
	}
	if !strings.HasPrefix(versions.String(), "DuckDB v") {
		t.Errorf("expected a DuckDB display line, got %q", versions.String())
	}
	if !strings.HasPrefix(duckdb.Reference, "1.") {
		t.Errorf("unexpected reference %q", duckdb.Reference)
	}
}
