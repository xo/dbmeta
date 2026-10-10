package test

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"slices"
	"strings"
	"testing"

	_ "github.com/xo/dbimp/neo4j"

	"github.com/xo/dbmeta"
	_ "github.com/xo/dbmeta/models/neo4j"
	njfixture "github.com/xo/dbmeta/models/neo4j/fixture"
)

// openNeo4j returns a connection to the server named by DBMETA_NEO4J, which
// is the neo4j:// URL that github.com/xo/dbimp/neo4j takes. Its path names
// the database dbmeta, which every statement of the model reads.
func openNeo4j(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("DBMETA_NEO4J")
	if dsn == "" {
		t.Skip("set DBMETA_NEO4J to run against a real server")
	}
	db, err := sql.Open("neo4j", dsn)
	if err != nil {
		t.Fatalf("opening: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.PingContext(t.Context()); err != nil {
		t.Fatalf("connecting: %v", err)
	}
	return db
}

// njComposed is the release from which a SHOW command can be joined with
// other clauses, which four kinds need.
var njComposed = dbmeta.V(2026, 5)

// njComposedKinds are the kinds that 5.26 is too old for.
var njComposedKinds = []string{
	dbmeta.IndexColumns.Name(), dbmeta.ConstraintColumns.Name(),
	dbmeta.Functions.Name(), dbmeta.RoutineParameters.Name(),
}

// setupNeo4j builds the fixture and returns the metadata for the server.
func setupNeo4j(t *testing.T, db *sql.DB) *dbmeta.Meta {
	t.Helper()
	ctx := t.Context()
	versions, err := dbmeta.Neo4j.Version(ctx, db)
	if err != nil {
		t.Fatalf("reading the version: %v", err)
	}
	down, err := njfixture.Everything.ResolveTeardown(versions)
	if err != nil {
		t.Fatalf("resolving the teardown: %v", err)
	}
	drop := func(c context.Context) {
		for _, step := range down {
			if !step.Skipped {
				//nolint:errcheck // a teardown before setup is best effort
				db.ExecContext(c, step.Query)
			}
		}
	}
	drop(ctx)
	up, err := njfixture.Everything.ResolveSetup(versions)
	if err != nil {
		t.Fatalf("resolving the setup: %v", err)
	}
	var ran, skipped int
	for _, step := range up {
		if step.Skipped {
			skipped++
			continue
		}
		if _, err := db.ExecContext(ctx, step.Query); err != nil {
			t.Fatalf("setup %s: %v\n%s", step.Name, err, step.Query)
		}
		ran++
	}
	t.Cleanup(func() { drop(context.WithoutCancel(ctx)) })
	m, err := dbmeta.New(dbmeta.Neo4j, versions)
	if err != nil {
		t.Fatalf("building the metadata: %v", err)
	}
	t.Logf("server reports %s, fixture ran %d steps and skipped %d", versions, ran, skipped)
	return m
}

// TestNeo4jVersion reads the version the way usql does and checks what the
// model makes of it.
func TestNeo4jVersion(t *testing.T) {
	db := openNeo4j(t)
	versions, err := dbmeta.Neo4j.Version(t.Context(), db)
	if err != nil {
		t.Fatalf("reading the version: %v", err)
	}
	if main := versions.Main(); main.Unknown || !main.AtLeast(dbmeta.V(5, 26)) {
		t.Errorf("expected release 5.26 or newer, got %s", main)
	}
	if !strings.HasPrefix(versions.String(), "Neo4j ") || !strings.HasSuffix(versions.String(), " enterprise") {
		t.Errorf("expected the display line to name Neo4j and the edition, got %q", versions)
	}
	t.Logf("server reports %s", versions)
}

// TestNeo4jEveryQueryRuns runs every query the model answers and checks that
// the columns are the declared fields, by name and in order, and that only
// the four kinds that need a SHOW command joined with other clauses are too
// old for 5.26.
func TestNeo4jEveryQueryRuns(t *testing.T) {
	db := openNeo4j(t)
	m := setupNeo4j(t, db)
	var ran int
	var tooOld []string
	for _, q := range dbmeta.Queries() {
		switch q.Support(m) {
		case dbmeta.NotSupported, dbmeta.NotBuilt:
			continue
		case dbmeta.TooOld:
			tooOld = append(tooOld, q.Name())
			continue
		case dbmeta.Supported:
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
		if strings.Join(cols, ",") != strings.Join(names, ",") {
			t.Errorf("%s: returns %v and declares %v", q.Name(), cols, names)
		}
		ran++
	}
	t.Logf("%d queries ran, too old: %v", ran, tooOld)
	want := []string(nil)
	if !m.Version().Main().AtLeast(njComposed) {
		want = slices.Clone(njComposedKinds)
	}
	slices.Sort(want)
	slices.Sort(tooOld)
	if !slices.Equal(tooOld, want) {
		t.Errorf("on %s the kinds too old are %v, want %v", m.Version(), tooOld, want)
	}
}

// TestNeo4jScansEveryQuery reads every query the model answers through its
// Scan, row by row, with the built in functions included.
func TestNeo4jScansEveryQuery(t *testing.T) {
	db := openNeo4j(t)
	m := setupNeo4j(t, db)
	scanEveryQuery(t, m, db)
}

// TestNeo4jColumnsIsNotSupported checks that Columns stays unanswered. The
// only source scans every node, which D47 forbids, and Ken chose on
// 2026-10-01 to leave it so (D162).
func TestNeo4jColumnsIsNotSupported(t *testing.T) {
	m, err := dbmeta.New(dbmeta.Neo4j, dbmeta.VersionSet{})
	if err != nil {
		t.Fatal(err)
	}
	if s := dbmeta.Columns.Support(m); s != dbmeta.NotSupported {
		t.Errorf("columns is %v, want not supported", s)
	}
}

// TestNeo4jFixtureObjects reads the fixture back through the typed API.
func TestNeo4jFixtureObjects(t *testing.T) {
	db := openNeo4j(t)
	m := setupNeo4j(t, db)
	ctx := t.Context()
	fx := njfixture.Everything
	composedOK := m.Version().Main().AtLeast(njComposed)

	tables := map[string]string{}
	for v, err := range dbmeta.Tables.All(ctx, m, db, dbmeta.Args{Schema: fx.Schema}.Map()) {
		if err != nil {
			t.Fatalf("reading tables: %v", err)
		}
		if v.Schema != fx.Schema || v.Catalog != "" {
			t.Errorf("table %s: catalog %q and schema %q, want empty and %q", v.Name, v.Catalog, v.Schema, fx.Schema)
		}
		tables[v.Name] = v.Type
	}
	for name, typ := range map[string]string{
		"author": "node label", "book": "node label", "region": "node label", "shipment": "node label",
		"written_by": "relationship type", "ships_to": "relationship type",
	} {
		if tables[name] != typ {
			t.Errorf("table %s: type %q, want %q", name, tables[name], typ)
		}
	}
	for range dbmeta.Tables.All(ctx, m, db, dbmeta.Args{Schema: "neo4j"}.Map()) {
		t.Error("a table in the schema neo4j, which is another database than the connection's")
		break
	}

	indexes := map[string]dbmeta.Index{}
	for v, err := range dbmeta.Indexes.All(ctx, m, db, nil) {
		if err != nil {
			t.Fatalf("reading indexes: %v", err)
		}
		indexes[v.Name] = v
	}
	for name, want := range map[string]dbmeta.Index{
		"book_published":  {Table: "book", Type: "range"},
		"book_pkey":       {Table: "book", Type: "range", Unique: true, Primary: true},
		"book_title_key":  {Table: "book", Type: "range", Unique: true},
		"book_title_text": {Table: "book", Type: "text"},
		"book_search":     {Table: "book|author", Type: "fulltext"},
		"ships_to_key":    {Table: "ships_to", Type: "range", Unique: true, Primary: true},
	} {
		got := indexes[name]
		if got.Table != want.Table || got.Type != want.Type || got.Unique != want.Unique ||
			got.Primary != want.Primary || got.Schema != fx.Schema {
			t.Errorf("index %s: got %+v, want %+v", name, got, want)
		}
	}
	// The fields of D210: the state, the statement, the provider and the
	// settings. Neo4j has no owner, size or predicate to read.
	if v := indexes["book_published"]; !v.Valid.Valid || !v.Valid.V ||
		!strings.HasPrefix(v.Definition.V, "CREATE RANGE INDEX `book_published`") ||
		!strings.HasPrefix(v.Using.V, "range-") || v.Options.Valid || v.Owner.Valid || v.Size.Valid || v.Predicate.Valid {
		t.Errorf("book_published: unexpected fields %+v", v)
	}
	if v := indexes["book_search"]; !strings.Contains(v.Options.V, "fulltext.analyzer=") || !strings.HasPrefix(v.Using.V, "fulltext-") {
		t.Errorf("book_search: expected the full text settings, got options %q and using %q", v.Options.V, v.Using.V)
	}
	// A pattern on parent matches either label of an index on two.
	var search bool
	for v, err := range dbmeta.Indexes.All(ctx, m, db, dbmeta.Args{Parent: "author"}.Map()) {
		if err != nil {
			t.Fatalf("reading the indexes of author: %v", err)
		}
		search = search || v.Name == "book_search"
	}
	if !search {
		t.Error("the indexes of author do not hold book_search, which is on book and author")
	}

	kinds := map[string]string{}
	for v, err := range dbmeta.Constraints.All(ctx, m, db, nil) {
		if err != nil {
			t.Fatalf("reading constraints: %v", err)
		}
		kinds[v.Name] = v.Type
		if !strings.HasPrefix(v.Definition.V, "CREATE CONSTRAINT") {
			t.Errorf("constraint %s: definition %q", v.Name, v.Definition.V)
		}
	}
	for name, want := range map[string]string{
		"author_pkey": "primary key", "ships_to_key": "primary key", "book_title_key": "unique",
		"author_name_not_null": "not null", "book_title_type": "node property type",
	} {
		if kinds[name] != want {
			t.Errorf("constraint %s: type %q, want %q", name, kinds[name], want)
		}
	}

	if composedOK {
		var keys []string
		for v, err := range dbmeta.IndexColumns.All(ctx, m, db, dbmeta.Args{Name: "book_author_title"}.Map()) {
			if err != nil {
				t.Fatalf("reading index columns: %v", err)
			}
			keys = append(keys, v.Name.V)
		}
		if strings.Join(keys, ",") != "author_id,title" {
			t.Errorf("book_author_title: properties %v", keys)
		}
		var cols []string
		for v, err := range dbmeta.ConstraintColumns.All(ctx, m, db, dbmeta.Args{Name: "region_pkey"}.Map()) {
			if err != nil {
				t.Fatalf("reading constraint columns: %v", err)
			}
			if v.ForeignTable.Valid {
				t.Errorf("region_pkey: a foreign table %q", v.ForeignTable.V)
			}
			cols = append(cols, v.Name)
		}
		if strings.Join(cols, ",") != "country,area" {
			t.Errorf("region_pkey: properties %v", cols)
		}
		kinds := map[string]string{}
		for v, err := range dbmeta.Functions.All(ctx, m, db, dbmeta.Args{Name: "db.%", WithSystem: true}.Map()) {
			if err != nil {
				t.Fatalf("reading functions: %v", err)
			}
			kinds[v.Name] = v.Kind
		}
		if kinds["db.labels"] != "proc" || kinds["db.nameFromElementId"] != "func" {
			t.Errorf("functions: db.labels is %q and db.nameFromElementId is %q", kinds["db.labels"], kinds["db.nameFromElementId"])
		}
		var params []string
		for v, err := range dbmeta.RoutineParameters.All(ctx, m, db, dbmeta.Args{Parent: "db.labels", WithSystem: true}.Map()) {
			if err != nil {
				t.Fatalf("reading routine parameters: %v", err)
			}
			params = append(params, v.Mode+" "+v.Name.V+" "+v.DataType)
		}
		if strings.Join(params, ",") != "table label STRING" {
			t.Errorf("db.labels: parameters %v", params)
		}
	}

	var count bool
	for v, err := range dbmeta.Aggregates.All(ctx, m, db, dbmeta.Args{Name: "count", WithSystem: true}.Map()) {
		if err != nil {
			t.Fatalf("reading aggregates: %v", err)
		}
		count = v.Kind == "agg" && v.Language == "internal" && v.ArgTypes.V == "ANY"
	}
	if !count {
		t.Error("the aggregate count is missing or wrong")
	}

	var grant, deny bool
	for v, err := range dbmeta.Privileges.All(ctx, m, db, dbmeta.Args{Schema: fx.Schema}.Map()) {
		if err != nil {
			t.Fatalf("reading privileges: %v", err)
		}
		grant = grant || v.Name == "NODE(book)" && strings.Contains(v.Access.V, "dbmeta_reader=match")
		deny = deny || v.Name == "NODE(author)" && v.Type == "property(rating)" &&
			strings.Contains(v.Access.V, "dbmeta_reader=DENIED read")
	}
	if !grant || !deny {
		t.Errorf("privileges: the grant is there %v, and the denial %v", grant, deny)
	}

	var member bool
	for v, err := range dbmeta.RoleGrants.All(ctx, m, db, dbmeta.Args{Name: "dbmeta_user"}.Map()) {
		if err != nil {
			t.Fatalf("reading role grants: %v", err)
		}
		member = member || v.MemberOf == "dbmeta_reader"
	}
	if !member {
		t.Error("dbmeta_user is not a member of dbmeta_reader")
	}
	home, ok, err := dbmeta.First(dbmeta.RoleSettings.All(ctx, m, db, dbmeta.Args{Name: "dbmeta_user"}.Map()))
	if err != nil || !ok || home.Settings.V != "home=dbmeta" {
		t.Errorf("role settings: got %+v, %v, %v", home, ok, err)
	}
	user, ok, err := dbmeta.First(dbmeta.CurrentUser.All(ctx, m, db, nil))
	if err != nil || !ok || user.Name != "neo4j" {
		t.Errorf("current user: got %+v, %v, %v", user, ok, err)
	}
	schema, ok, err := dbmeta.First(dbmeta.CurrentSchema.All(ctx, m, db, nil))
	if err != nil || !ok || schema.Name != fx.Schema {
		t.Errorf("current schema: got %+v, %v, %v", schema, ok, err)
	}
}

// TestNeo4jPatterns checks that a pattern means what dbmeta.Like says it
// means, because Cypher has no LIKE and the model turns each pattern into a
// regular expression.
func TestNeo4jPatterns(t *testing.T) {
	db := openNeo4j(t)
	m := setupNeo4j(t, db)
	names := func(pattern string) []string {
		var out []string
		for v, err := range dbmeta.Tables.All(t.Context(), m, db, dbmeta.Args{Name: pattern}.Map()) {
			if err != nil {
				t.Fatalf("reading tables named %q: %v", pattern, err)
			}
			out = append(out, v.Name)
		}
		slices.Sort(out)
		return out
	}
	every := names("")
	for _, c := range []struct{ pattern string }{
		{"book"}, {"b%"}, {"%_to"}, {"ships\\_to"}, {"shi_ment"}, {"s%t"}, {"%"},
		{"."}, {".%"}, {"book\\"}, {"\\%"}, {"(book)"}, {"[a-z]%"},
	} {
		var want []string
		for _, n := range every {
			if dbmeta.Like(c.pattern, n) {
				want = append(want, n)
			}
		}
		if got := names(c.pattern); !slices.Equal(got, want) {
			t.Errorf("pattern %q: got %v, want %v", c.pattern, got, want)
		}
	}
}

// TestNeo4jTooOld checks that 5.26 reports the four kinds that need a SHOW
// command joined with other clauses as too old, rather than answering them.
func TestNeo4jTooOld(t *testing.T) {
	db := openNeo4j(t)
	versions, err := dbmeta.Neo4j.Version(t.Context(), db)
	if err != nil {
		t.Fatalf("reading the version: %v", err)
	}
	if versions.Main().AtLeast(njComposed) {
		t.Skipf("the server is %s, which joins a SHOW command with other clauses", versions)
	}
	m, err := dbmeta.New(dbmeta.Neo4j, versions)
	if err != nil {
		t.Fatal(err)
	}
	for v, err := range dbmeta.Functions.All(t.Context(), m, db, nil) {
		if !errors.Is(err, dbmeta.ErrVersionTooOld) {
			t.Fatalf("expected ErrVersionTooOld, got a row %+v and %v", v, err)
		}
	}
}
