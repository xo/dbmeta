package test

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"

	_ "github.com/exasol/exasol-driver-go"

	"github.com/xo/dbmeta"
	_ "github.com/xo/dbmeta/models/exasol"
	exfixture "github.com/xo/dbmeta/models/exasol/fixture"
)

// setupExasol builds the fixture and returns the metadata for the server.
//
// The teardown runs first, because a run that failed part way leaves the
// schema behind and CREATE SCHEMA then fails rather than the test reporting
// what actually went wrong. Every teardown step says IF EXISTS, so it is
// safe on a clean server.
func setupExasol(t *testing.T, db *sql.DB) *dbmeta.Meta {
	t.Helper()
	ctx := t.Context()
	versions, err := dbmeta.Exasol.Version(ctx, db)
	if err != nil {
		t.Fatalf("reading the version: %v", err)
	}
	down, err := exfixture.Everything.ResolveTeardown(versions)
	if err != nil {
		t.Fatalf("resolving the teardown: %v", err)
	}
	drop := func(c context.Context) error {
		for _, step := range down {
			if step.Skipped {
				continue
			}
			if _, err := db.ExecContext(c, step.Query); err != nil {
				return err
			}
		}
		return nil
	}
	if err := drop(ctx); err != nil {
		t.Fatalf("clearing an earlier fixture: %v", err)
	}

	up, err := exfixture.Everything.ResolveSetup(versions)
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
	t.Cleanup(func() {
		if err := drop(context.WithoutCancel(ctx)); err != nil {
			t.Errorf("tearing the fixture down: %v", err)
		}
	})

	m, err := dbmeta.New(dbmeta.Exasol, versions)
	if err != nil {
		t.Fatalf("building the metadata: %v", err)
	}
	t.Logf("server reports %s, fixture ran %d steps and skipped %d", versions, ran, skipped)
	return m
}

// openExasol returns a connection to the server named by DBMETA_EXASOL.
//
// One connection, because Exasol holds a schema open per session and
// CREATE SCHEMA opens the one it creates. A pool would hand a later query a
// session with a different current schema than the one the fixture left.
func openExasol(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("DBMETA_EXASOL")
	if dsn == "" {
		t.Skip("set DBMETA_EXASOL to run against a real server")
	}
	db, err := sql.Open("exasol", dsn)
	if err != nil {
		t.Fatalf("opening: %v", err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	if err := db.PingContext(t.Context()); err != nil {
		t.Fatalf("connecting: %v", err)
	}
	return db
}

// exArgs is the filter the fixture's objects sit behind.
func exArgs() map[string]any {
	return dbmeta.Args{Schema: exfixture.Everything.Schema}.Map()
}

// TestExasolEveryQueryRuns executes every query the model answers and checks
// the columns match the declared fields.
func TestExasolEveryQueryRuns(t *testing.T) {
	db := openExasol(t)
	m := setupExasol(t, db)
	var ran, unsupported int
	for _, q := range dbmeta.Queries() {
		switch q.Support(m) {
		case dbmeta.NotSupported, dbmeta.NotBuilt, dbmeta.TooOld:
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
			t.Errorf("%s: declares %d fields and returns %d columns: %v",
				q.Name(), len(fields), len(cols), cols)
			continue
		}
		for i := range cols {
			if !strings.EqualFold(cols[i], fields[i].Name) {
				t.Errorf("%s: column %d is %q and the field is %q",
					q.Name(), i, cols[i], fields[i].Name)
			}
		}
		ran++
	}
	t.Logf("%d queries ran, %d not supported by Exasol", ran, unsupported)
	if ran == 0 {
		t.Fatal("expected at least one query to run")
	}
}

// TestExasolFixtureBuilds proves every statement the fixture makes is
// accepted, and that the tables arrive.
func TestExasolFixtureBuilds(t *testing.T) {
	db := openExasol(t)
	m := setupExasol(t, db)
	var tables int
	for v, err := range dbmeta.Tables.All(t.Context(), m, db, exArgs()) {
		if err != nil {
			t.Fatalf("reading tables: %v", err)
		}
		t.Logf("  %-12s %s", v.Type, v.Name)
		tables++
	}
	if tables == 0 {
		t.Fatal("expected the fixture's tables")
	}
}

// TestExasolVersion reads the version and checks what the model makes of it.
func TestExasolVersion(t *testing.T) {
	db := openExasol(t)
	versions, err := dbmeta.Exasol.Version(t.Context(), db)
	if err != nil {
		t.Fatalf("reading the version: %v", err)
	}
	main := versions.Main()
	if main.Unknown || len(main.Parts) != 3 {
		t.Fatalf("expected a three part version, got %v", versions)
	}
	// The releases are named for the year, 2025.2.1 and 2026.2.0.
	if main.Parts[0] < 2025 {
		t.Errorf("expected a release named for a year, got %v", main)
	}
	if !strings.HasPrefix(versions.Display, "Exasol ") {
		t.Errorf("expected the display to name the product, got %q", versions.Display)
	}
}

// TestExasolColumns checks what Exasol records about a column, and the two
// fields whose empty value Exasol cannot write.
func TestExasolColumns(t *testing.T) {
	db := openExasol(t)
	m := setupExasol(t, db)
	type want struct {
		dataType   string
		nullable   bool
		primaryKey bool
		hasDefault bool
		identity   string
	}
	expect := map[string]want{
		"AUTHOR.AUTHOR_ID": {"DECIMAL(18,0)", false, true, false, ""},
		"AUTHOR.NAME":      {"VARCHAR(128) UTF8", false, false, false, ""},
		"AUTHOR.RATING":    {"DECIMAL(18,0)", true, false, false, ""},
		"AUTHOR.SHADE":     {"VARCHAR(16) UTF8", true, false, true, ""},
		"TICKET.TICKET_ID": {"DECIMAL(18,0)", false, true, false, "by default"},
		// A view column records no nullability, and reads nullable the way
		// PostgreSQL reports one.
		"RECENT.TITLE": {"VARCHAR(255) UTF8", true, false, false, ""},
	}
	seen := map[string]bool{}
	for v, err := range dbmeta.Columns.All(t.Context(), m, db, exArgs()) {
		if err != nil {
			t.Fatalf("reading columns: %v", err)
		}
		key := v.Table + "." + v.Name
		// Every column has an identity and a generated kind, and they are
		// the empty string for most columns. Exasol reads '' as NULL, and
		// the model restores the empty value rather than reporting it
		// absent, which would say the release has no such thing.
		if !v.Identity.Valid || !v.Generated.Valid {
			t.Errorf("%s: identity and generated must be present, got %v and %v",
				key, v.Identity, v.Generated)
		}
		if v.Generated.V != "" {
			t.Errorf("%s: Exasol has no generated column, got %q", key, v.Generated.V)
		}
		w, ok := expect[key]
		if !ok {
			continue
		}
		seen[key] = true
		if v.DataType != w.dataType || v.Nullable != w.nullable ||
			v.PrimaryKey != w.primaryKey || v.Default.Valid != w.hasDefault ||
			v.Identity.V != w.identity {
			t.Errorf("%s: got type=%q nullable=%v pk=%v default=%v identity=%q, want %+v",
				key, v.DataType, v.Nullable, v.PrimaryKey, v.Default, v.Identity.V, w)
		}
	}
	for key := range expect {
		if !seen[key] {
			t.Errorf("expected a column %s", key)
		}
	}
}

// TestExasolConstraints checks the three kinds Exasol has and the two it
// refuses, so that an absence stays a decision rather than becoming a bug.
func TestExasolConstraints(t *testing.T) {
	db := openExasol(t)
	m := setupExasol(t, db)
	kinds := map[string]int{}
	for v, err := range dbmeta.Constraints.All(t.Context(), m, db, exArgs()) {
		if err != nil {
			t.Fatalf("reading constraints: %v", err)
		}
		kinds[v.Type]++
		if v.Definition.Valid || v.Deferrable || v.Deferred {
			t.Errorf("%s: Exasol keeps no constraint text and defers nothing, got %+v", v.Name, v)
		}
	}
	for _, k := range []string{"primary key", "foreign key", "not null"} {
		if kinds[k] == 0 {
			t.Errorf("expected a %s constraint, got %v", k, kinds)
		}
	}
	for _, k := range []string{"unique", "check"} {
		if kinds[k] != 0 {
			t.Errorf("Exasol refuses %s constraints and one was reported: %v", k, kinds)
		}
	}

	// A foreign key's catalog is the empty string, and every other kind has
	// none. Exasol reads the '' the statement selects as NULL, so the model
	// restores it from the row.
	var foreign, other int
	for v, err := range dbmeta.ConstraintColumns.All(t.Context(), m, db, exArgs()) {
		if err != nil {
			t.Fatalf("reading constraint columns: %v", err)
		}
		switch {
		case v.ForeignTable.Valid:
			foreign++
			if !v.ForeignCatalog.Valid || v.ForeignCatalog.V != "" {
				t.Errorf("%s: a foreign key's catalog must be present and empty, got %v",
					v.Constraint, v.ForeignCatalog)
			}
		default:
			other++
			if v.ForeignCatalog.Valid {
				t.Errorf("%s: only a foreign key has a catalog, got %v", v.Constraint, v.ForeignCatalog)
			}
		}
		if v.Ordinal < 1 {
			t.Errorf("%s.%s: expected an ordinal from one, got %d", v.Constraint, v.Name, v.Ordinal)
		}
	}
	if foreign == 0 || other == 0 {
		t.Errorf("expected foreign key columns and others, got %d and %d", foreign, other)
	}
}

// TestExasolIndexColumns checks the columns read out of REMARKS, in order,
// on the composite key Exasol indexes for region.
func TestExasolIndexColumns(t *testing.T) {
	db := openExasol(t)
	m := setupExasol(t, db)
	args := dbmeta.Args{Schema: exfixture.Everything.Schema, Parent: "REGION"}.Map()
	byIndex := map[string][]string{}
	for v, err := range dbmeta.IndexColumns.All(t.Context(), m, db, args) {
		if err != nil {
			t.Fatalf("reading index columns: %v", err)
		}
		if int(v.Ordinal) != len(byIndex[v.Index])+1 {
			t.Errorf("index %s: %s is at %d, expected %d", v.Index, v.Name.V, v.Ordinal,
				len(byIndex[v.Index])+1)
		}
		byIndex[v.Index] = append(byIndex[v.Index], v.Name.V)
	}
	if len(byIndex) == 0 {
		t.Fatal("expected Exasol to have indexed the region key")
	}
	for index, cols := range byIndex {
		if strings.Join(cols, ",") != "COUNTRY,AREA" {
			t.Errorf("index %s: expected COUNTRY,AREA, got %v", index, cols)
		}
	}
	for v, err := range dbmeta.Indexes.All(t.Context(), m, db, args) {
		if err != nil {
			t.Fatalf("reading indexes: %v", err)
		}
		if v.Unique || v.Primary {
			t.Errorf("index %s: an Exasol index enforces nothing, got %+v", v.Name, v)
		}
		if v.Type != "global" && v.Type != "local" {
			t.Errorf("index %s: expected global or local, got %q", v.Name, v.Type)
		}
	}
}

// TestExasolRoutines checks the kind each of the six routines reads as.
func TestExasolRoutines(t *testing.T) {
	db := openExasol(t)
	m := setupExasol(t, db)
	type want struct{ kind, result, language string }
	expect := map[string]want{
		"DOUBLED":       {"function", "", "SQL"},
		"TRIPLED":       {"function", "", "LUA"},
		"TOTAL":         {"aggregate", "", "LUA"},
		"SPREAD":        {"function", "setof record", "LUA"},
		"HELLO":         {"procedure", "rowcount", "LUA"},
		"FIXED_ADAPTER": {"adapter", "", "LUA"},
	}
	seen := map[string]bool{}
	for v, err := range dbmeta.Functions.All(t.Context(), m, db, exArgs()) {
		if err != nil {
			t.Fatalf("reading functions: %v", err)
		}
		w, ok := expect[v.Name]
		if !ok {
			t.Errorf("unexpected routine %s", v.Name)
			continue
		}
		seen[v.Name] = true
		if v.Kind != w.kind || v.ResultType.V != w.result || v.ResultType.Valid != (w.result != "") ||
			v.Language != w.language {
			t.Errorf("%s: got kind=%q result=%q language=%q, want %+v",
				v.Name, v.Kind, v.ResultType.V, v.Language, w)
		}
		// Exasol keeps the parameters and the return type only in the text,
		// so arg_types is absent, and volatility and security are empty.
		// The text is the whole statement, so it is the definition and there
		// is no source apart from it.
		if v.ArgTypes.Valid || v.Volatility != "" || v.Security != "" || v.Source.Valid || !v.Definition.Valid {
			t.Errorf("%s: expected no arg_types or source, empty volatility and security, and a definition, got %+v", v.Name, v)
		}
		if !v.ID.Valid || v.ID.V == "" {
			t.Errorf("%s: expected an object id", v.Name)
		}
	}
	for name := range expect {
		if !seen[name] {
			t.Errorf("expected a routine %s", name)
		}
	}
	var aggregates []string
	for v, err := range dbmeta.Aggregates.All(t.Context(), m, db, exArgs()) {
		if err != nil {
			t.Fatalf("reading aggregates: %v", err)
		}
		aggregates = append(aggregates, v.Name)
	}
	if strings.Join(aggregates, ",") != "TOTAL" {
		t.Errorf("expected the one set script that returns a value, got %v", aggregates)
	}
}

// TestExasolForeignData checks the adapter, the virtual schema and the
// virtual table the fixture builds, and the connection granted to a role.
func TestExasolForeignData(t *testing.T) {
	db := openExasol(t)
	m := setupExasol(t, db)
	ctx := t.Context()
	var wrappers []string
	for v, err := range dbmeta.ForeignDataWrappers.All(ctx, m, db,
		dbmeta.Args{Name: "DBMETA_FIXTURE.%"}.Map()) {
		if err != nil {
			t.Fatalf("reading wrappers: %v", err)
		}
		wrappers = append(wrappers, v.Name+"/"+v.Handler.V)
	}
	if strings.Join(wrappers, ",") != "DBMETA_FIXTURE.FIXED_ADAPTER/LUA" {
		t.Errorf("expected the fixture's adapter, got %v", wrappers)
	}
	var servers int
	for v, err := range dbmeta.ForeignServers.All(ctx, m, db, dbmeta.Args{Name: "DBMETA_REMOTE"}.Map()) {
		if err != nil {
			t.Fatalf("reading servers: %v", err)
		}
		servers++
		if v.Wrapper != "DBMETA_FIXTURE.FIXED_ADAPTER" ||
			v.Options.V != "CONNECTION_NAME=DBMETA_REMOTE_CONN, FLAVOR=fixed" {
			t.Errorf("unexpected virtual schema %+v", v)
		}
	}
	if servers != 1 {
		t.Errorf("expected the fixture's virtual schema, got %d", servers)
	}
	var tables []string
	for v, err := range dbmeta.ForeignTables.All(ctx, m, db, dbmeta.Args{Schema: "DBMETA_REMOTE"}.Map()) {
		if err != nil {
			t.Fatalf("reading foreign tables: %v", err)
		}
		tables = append(tables, v.Server+"."+v.Name)
	}
	if strings.Join(tables, ",") != "DBMETA_REMOTE.REMOTE_ITEM" {
		t.Errorf("expected the adapter's one table, got %v", tables)
	}
	var mappings []string
	for v, err := range dbmeta.UserMappings.All(ctx, m, db, dbmeta.Args{Name: "DBMETA_READER"}.Map()) {
		if err != nil {
			t.Fatalf("reading user mappings: %v", err)
		}
		mappings = append(mappings, v.Server+"|"+v.Options.V)
	}
	if strings.Join(mappings, ",") != "DBMETA_REMOTE_CONN|user=remote, address=https://example.invalid" {
		t.Errorf("expected the connection granted to the role, got %v", mappings)
	}
}

// TestExasolLanguages checks Lua, which is built in, and the aliases the
// SCRIPT_LANGUAGES parameter maps onto containers.
func TestExasolLanguages(t *testing.T) {
	db := openExasol(t)
	m := setupExasol(t, db)
	names := map[string]bool{}
	for v, err := range dbmeta.Languages.All(t.Context(), m, db, nil) {
		if err != nil {
			t.Fatalf("reading languages: %v", err)
		}
		if names[v.Name] {
			t.Errorf("%s is listed twice", v.Name)
		}
		names[v.Name] = true
		switch {
		case v.Name == "LUA" && (!v.Internal || v.Handler != ""):
			t.Errorf("LUA is built in and has no container, got %+v", v)
		case v.Name != "LUA" && (v.Internal || v.Handler == ""):
			t.Errorf("%s runs in a container and names it, got %+v", v.Name, v)
		}
	}
	if !names["LUA"] || len(names) < 2 {
		t.Errorf("expected LUA and at least one container language, got %v", names)
	}
}

// TestExasolCurrentSchema checks that a session with no open schema has no
// current schema rather than a row with an empty name.
func TestExasolCurrentSchema(t *testing.T) {
	db := openExasol(t)
	m := setupExasol(t, db)
	ctx := t.Context()
	current := func() []string {
		var out []string
		for v, err := range dbmeta.CurrentSchema.All(ctx, m, db, nil) {
			if err != nil {
				t.Fatalf("reading the current schema: %v", err)
			}
			out = append(out, v.Name)
		}
		return out
	}
	exec(t, db, `OPEN SCHEMA dbmeta_fixture`)
	if got := current(); strings.Join(got, ",") != "DBMETA_FIXTURE" {
		t.Errorf("expected DBMETA_FIXTURE after OPEN SCHEMA, got %v", got)
	}
	exec(t, db, `CLOSE SCHEMA`)
	if got := current(); len(got) != 0 {
		t.Errorf("expected no current schema after CLOSE SCHEMA, got %v", got)
	}
}

// TestExasolComments checks every kind of object Exasol takes a comment on.
func TestExasolComments(t *testing.T) {
	db := openExasol(t)
	m := setupExasol(t, db)
	want := map[string]string{
		"schema/DBMETA_FIXTURE":         "the fixture",
		"table/AUTHOR":                  "people who write",
		"view/RECENT":                   "the newest books",
		"function/DOUBLED":              "twice n",
		"script/TRIPLED":                "three times n",
		"role/DBMETA_READER":            "reads authors",
		"connection/DBMETA_REMOTE_CONN": "nowhere at all",
	}
	for v, err := range dbmeta.Comments.All(t.Context(), m, db, nil) {
		if err != nil {
			t.Fatalf("reading comments: %v", err)
		}
		key := v.Type + "/" + v.Name
		if w, ok := want[key]; ok {
			if v.Comment != w {
				t.Errorf("%s: got %q, want %q", key, v.Comment, w)
			}
			delete(want, key)
		}
	}
	for key := range want {
		t.Errorf("expected a comment on %s", key)
	}
}
