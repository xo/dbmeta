package test

import (
	"context"
	"database/sql"
	"os"
	"slices"
	"strings"
	"testing"

	_ "github.com/vertica/vertica-sql-go"

	"github.com/xo/dbmeta"
	_ "github.com/xo/dbmeta/models/vertica"
	vefixture "github.com/xo/dbmeta/models/vertica/fixture"
)

// setupVertica builds the fixture and returns the metadata for the server.
//
// The teardown runs first, because a run that failed part way leaves the
// schema behind and CREATE SCHEMA then fails rather than the test reporting
// what actually went wrong. Every teardown step says IF EXISTS, so it is
// safe on a clean server.
func setupVertica(t *testing.T, db *sql.DB) *dbmeta.Meta {
	t.Helper()
	ctx := t.Context()
	versions, err := dbmeta.Vertica.Version(ctx, db)
	if err != nil {
		t.Fatalf("reading the version: %v", err)
	}
	down, err := vefixture.Everything.ResolveTeardown(versions)
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

	up, err := vefixture.Everything.ResolveSetup(versions)
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

	m, err := dbmeta.New(dbmeta.Vertica, versions)
	if err != nil {
		t.Fatalf("building the metadata: %v", err)
	}
	t.Logf("server reports %s, fixture ran %d steps and skipped %d", versions, ran, skipped)
	return m
}

// openVertica returns a connection to the server named by DBMETA_VERTICA.
func openVertica(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("DBMETA_VERTICA")
	if dsn == "" {
		t.Skip("set DBMETA_VERTICA to run against a real server")
	}
	db, err := sql.Open("vertica", dsn)
	if err != nil {
		t.Fatalf("opening: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.PingContext(t.Context()); err != nil {
		t.Fatalf("connecting: %v", err)
	}
	return db
}

// veArgs is the filter the fixture's objects sit behind.
func veArgs() map[string]any {
	return dbmeta.Args{Schema: vefixture.Everything.Schema}.Map()
}

// TestVerticaEveryQueryRuns executes every query the model answers and checks
// the columns match the declared fields.
func TestVerticaEveryQueryRuns(t *testing.T) {
	db := openVertica(t)
	m := setupVertica(t, db)
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
	t.Logf("%d queries ran, %d not supported by Vertica", ran, unsupported)
	if ran == 0 {
		t.Fatal("expected at least one query to run")
	}
}

// TestVerticaFixtureBuilds proves every statement the fixture makes is
// accepted, and that the tables arrive.
func TestVerticaFixtureBuilds(t *testing.T) {
	db := openVertica(t)
	m := setupVertica(t, db)
	var tables int
	for v, err := range dbmeta.Tables.All(t.Context(), m, db, veArgs()) {
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

// TestVerticaVersion reads the version and checks what the model makes of it.
func TestVerticaVersion(t *testing.T) {
	db := openVertica(t)
	versions, err := dbmeta.Vertica.Version(t.Context(), db)
	if err != nil {
		t.Fatalf("reading the version: %v", err)
	}
	main := versions.Main()
	if main.Unknown || len(main.Parts) != 3 {
		t.Fatalf("expected a three part version, got %v", versions)
	}
	// 7.2.1, 9.1.0, 10.1.1 and 25.1.0 are what the tests run.
	if main.Parts[0] < 7 {
		t.Errorf("expected Vertica 7 or newer, got %v", main)
	}
	if !strings.HasPrefix(versions.Display, "Vertica Analytic Database v") {
		t.Errorf("expected the display to name the product, got %q", versions.Display)
	}
}

// TestVerticaColumns checks the parts of a column, and the two whose empty
// value means "not one of these".
func TestVerticaColumns(t *testing.T) {
	db := openVertica(t)
	m := setupVertica(t, db)
	type want struct {
		dataType   string
		nullable   bool
		primaryKey bool
		hasDefault bool
		identity   string
	}
	expect := map[string]want{
		"author.author_id": {"int", false, true, false, ""},
		"author.name":      {"varchar(128)", false, false, false, ""},
		"author.shade":     {"varchar(16)", true, false, true, ""},
		"ticket.ticket_id": {"int", false, true, false, "always"},
		"recent.title":     {"varchar(255)", true, false, false, ""},
	}
	seen := map[string]bool{}
	for v, err := range dbmeta.Columns.All(t.Context(), m, db, veArgs()) {
		if err != nil {
			t.Fatalf("reading columns: %v", err)
		}
		key := v.Table + "." + v.Name
		if !v.Identity.Valid || !v.Generated.Valid || v.Generated.V != "" {
			t.Errorf("%s: identity and generated must be present, and nothing here is generated: %v %v",
				key, v.Identity, v.Generated)
		}
		w, ok := expect[key]
		if !ok {
			continue
		}
		seen[key] = true
		if v.DataType != w.dataType || v.Nullable != w.nullable || v.PrimaryKey != w.primaryKey ||
			v.Default.Valid != w.hasDefault || v.Identity.V != w.identity {
			t.Errorf("%s: got type=%q nullable=%v pk=%v default=%v identity=%q, want %+v",
				key, v.DataType, v.Nullable, v.PrimaryKey, v.Default, v.Identity.V, w)
		}
		// A comment on a table column arrived in 10.1, and the fixture sets
		// one on author.name there.
		if key == "author.name" && v.Comment.Valid != (m.Version().Main().Compare(dbmeta.V(10, 1)) >= 0) {
			t.Errorf("author.name: expected a comment from 10.1 and none before, got %v", v.Comment)
		}
	}
	for key := range expect {
		if !seen[key] {
			t.Errorf("expected a column %s", key)
		}
	}
}

// TestVerticaConstraints checks the four kinds Vertica records, and that a
// CHECK constraint appears exactly where the release has one.
func TestVerticaConstraints(t *testing.T) {
	db := openVertica(t)
	m := setupVertica(t, db)
	kinds := map[string]int{}
	for v, err := range dbmeta.Constraints.All(t.Context(), m, db, veArgs()) {
		if err != nil {
			t.Fatalf("reading constraints: %v", err)
		}
		kinds[v.Type]++
		if v.Type == "check" && !v.Definition.Valid {
			t.Errorf("%s: a check constraint carries its predicate", v.Name)
		}
		if v.Type != "check" && v.Definition.Valid {
			t.Errorf("%s: only a check constraint has a definition, got %v", v.Name, v.Definition)
		}
	}
	for _, k := range []string{"primary key", "foreign key", "unique"} {
		if kinds[k] == 0 {
			t.Errorf("expected a %s constraint, got %v", k, kinds)
		}
	}
	// A NOT NULL is not a constraint of its own, the way it is not in
	// PostgreSQL, and a CHECK arrived in 9.1.
	if kinds["not null"] != 0 {
		t.Errorf("a NOT NULL must not be reported as a constraint: %v", kinds)
	}
	if wantCheck := m.Version().Main().Compare(dbmeta.V(9, 1)) >= 0; (kinds["check"] > 0) != wantCheck {
		t.Errorf("expected a check constraint from 9.1 and none before, got %v", kinds)
	}
	var foreign int
	for v, err := range dbmeta.ConstraintColumns.All(t.Context(), m, db, veArgs()) {
		if err != nil {
			t.Fatalf("reading constraint columns: %v", err)
		}
		if v.ForeignTable.Valid != v.ForeignCatalog.Valid {
			t.Errorf("%s: a foreign key's catalog is present and empty, and absent otherwise: %v", v.Constraint, v)
		}
		if v.ForeignTable.Valid {
			foreign++
		}
	}
	if foreign != 3 {
		t.Errorf("expected the three foreign key columns, got %d", foreign)
	}
}

// TestVerticaProjections checks that a projection is what Indexes reports,
// with its columns in the order it stores them.
func TestVerticaProjections(t *testing.T) {
	db := openVertica(t)
	m := setupVertica(t, db)
	var names []string
	for v, err := range dbmeta.Indexes.All(t.Context(), m, db, veArgs()) {
		if err != nil {
			t.Fatalf("reading projections: %v", err)
		}
		names = append(names, v.Table+"."+v.Name+"/"+v.Type)
		if v.Unique {
			t.Errorf("%s: a projection enforces nothing", v.Name)
		}
	}
	if !slices.Contains(names, "book.book_published/projection") {
		t.Errorf("expected the fixture's projection, got %v", names)
	}
	var cols []string
	for v, err := range dbmeta.IndexColumns.All(t.Context(), m, db,
		dbmeta.Args{Schema: vefixture.Everything.Schema, Name: "book_published"}.Map()) {
		if err != nil {
			t.Fatalf("reading projection columns: %v", err)
		}
		cols = append(cols, v.Name)
	}
	if strings.Join(cols, ",") != "book_id,published" {
		t.Errorf("expected book_id,published, got %v", cols)
	}
}

// TestVerticaPrivileges checks the two shapes Privileges has: a row per
// object from 10.1, where LISTAGG arrived, and a row per grantee before it.
func TestVerticaPrivileges(t *testing.T) {
	db := openVertica(t)
	m := setupVertica(t, db)
	var author, policies []string
	for v, err := range dbmeta.Privileges.All(t.Context(), m, db,
		dbmeta.Args{Schema: vefixture.Everything.Schema, Name: "author"}.Map()) {
		if err != nil {
			t.Fatalf("reading privileges: %v", err)
		}
		author = append(author, v.Access.V)
		policies = append(policies, v.Policies)
	}
	folded := m.Version().Main().Compare(dbmeta.V(10, 1)) >= 0
	switch {
	case folded && len(author) != 1:
		t.Errorf("expected one row for author from 10.1, got %v", author)
	case !folded && len(author) < 2:
		t.Errorf("expected a row per grantee before 10.1, got %v", author)
	}
	if !strings.Contains(strings.Join(author, ", "), "dbmeta_reader=SELECT") {
		t.Errorf("expected the reader's grant, got %v", author)
	}
	if folded != strings.Contains(strings.Join(policies, "; "), "name: ") {
		t.Errorf("expected the column policy from 10.1 and none before, got %q", policies)
	}
}

// TestVerticaReleaseGates checks the two kinds measured on 25.1 alone.
func TestVerticaReleaseGates(t *testing.T) {
	db := openVertica(t)
	m := setupVertica(t, db)
	want := dbmeta.Supported
	if m.Version().Main().Compare(dbmeta.V(25, 1)) < 0 {
		want = dbmeta.TooOld
	}
	for _, q := range []dbmeta.AnyQuery{dbmeta.Triggers, dbmeta.RoleSettings} {
		if got := q.Support(m); got != want {
			t.Errorf("%s: expected %v, got %v", q.Name(), want, got)
		}
	}
	if want != dbmeta.Supported {
		return
	}
	var triggers []string
	for v, err := range dbmeta.Triggers.All(t.Context(), m, db, veArgs()) {
		if err != nil {
			t.Fatalf("reading triggers: %v", err)
		}
		triggers = append(triggers, v.Name+"/"+v.Definition)
	}
	if strings.Join(triggers, ",") != "count_nightly/EXECUTE PROCEDURE book_count(1)" {
		t.Errorf("expected the fixture's trigger, got %v", triggers)
	}
	var settings []string
	for v, err := range dbmeta.RoleSettings.All(t.Context(), m, db, dbmeta.Args{Name: "dbmeta_tuned"}.Map()) {
		if err != nil {
			t.Fatalf("reading role settings: %v", err)
		}
		settings = append(settings, v.Settings.V)
	}
	if strings.Join(settings, ",") != "WithClauseRecursionLimit=4" {
		t.Errorf("expected the fixture user's setting, got %v", settings)
	}
}

// TestVerticaCurrentSchema checks the schema an unqualified name resolves in.
func TestVerticaCurrentSchema(t *testing.T) {
	db := openVertica(t)
	m := setupVertica(t, db)
	s, ok, err := dbmeta.First(dbmeta.CurrentSchema.All(t.Context(), m, db, nil))
	if err != nil || !ok {
		t.Fatalf("reading the current schema: %v, found %v", err, ok)
	}
	if s.Name != "public" {
		t.Errorf("expected public, the first schema on the default search path, got %q", s.Name)
	}
}
