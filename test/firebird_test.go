package test

import (
	"context"
	"database/sql"
	"os"
	"slices"
	"strings"
	"testing"

	_ "github.com/nakagami/firebirdsql"

	"github.com/xo/dbmeta"
	_ "github.com/xo/dbmeta/models/firebird"
	fbfixture "github.com/xo/dbmeta/models/firebird/fixture"
)

// openFirebird returns a connection to the server named by
// DBMETA_FIREBIRDSQL.
func openFirebird(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("DBMETA_FIREBIRDSQL")
	if dsn == "" {
		t.Skip("set DBMETA_FIREBIRDSQL to run against a real server")
	}
	db, err := sql.Open("firebirdsql", dsn)
	if err != nil {
		t.Fatalf("opening: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.PingContext(t.Context()); err != nil {
		t.Fatalf("connecting: %v", err)
	}
	return db
}

// setupFirebird builds the fixture and returns the metadata for the server.
//
// The teardown runs first, because a run that failed part way leaves objects
// behind and CREATE then fails rather than the test reporting what actually
// went wrong. Firebird has no DROP ... IF EXISTS before 5.0, so an error from
// a teardown step is expected and ignored.
func setupFirebird(t *testing.T, db *sql.DB) *dbmeta.Meta {
	t.Helper()
	ctx := t.Context()
	versions, err := dbmeta.Firebird.Version(ctx, db)
	if err != nil {
		t.Fatalf("reading the version: %v", err)
	}

	down, err := fbfixture.Everything.ResolveTeardown(versions)
	if err != nil {
		t.Fatalf("resolving the teardown: %v", err)
	}
	drop := func(c context.Context) {
		for _, step := range down {
			if !step.Skipped {
				//nolint:errcheck // Firebird has no DROP IF EXISTS before 5.0
				db.ExecContext(c, step.Query)
			}
		}
	}
	drop(ctx)

	up, err := fbfixture.Everything.ResolveSetup(versions)
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

	m, err := dbmeta.New(dbmeta.Firebird, versions)
	if err != nil {
		t.Fatalf("building the metadata: %v", err)
	}
	t.Logf("server reports %s, fixture ran %d steps and skipped %d", versions, ran, skipped)
	return m
}

// TestFirebirdVersion reads the version and checks what the model makes of it.
func TestFirebirdVersion(t *testing.T) {
	db := openFirebird(t)
	versions, err := dbmeta.Firebird.Version(t.Context(), db)
	if err != nil {
		t.Fatalf("reading the version: %v", err)
	}
	main := versions.Main()
	if main.Unknown || len(main.Parts) < 2 {
		t.Fatalf("expected a parsed version, got %v", versions)
	}
	if main.Parts[0] < 3 {
		t.Errorf("expected 3.0 or newer, got %v", main)
	}
	if !strings.HasPrefix(versions.Display, "Firebird ") {
		t.Errorf("expected the display to name the product, got %q", versions.Display)
	}
	t.Logf("server reports %s", versions)
}

// TestFirebirdEveryQueryRuns executes every query Firebird answers and checks
// the columns match the declared fields.
func TestFirebirdEveryQueryRuns(t *testing.T) {
	db := openFirebird(t)
	m := setupFirebird(t, db)

	var ran, unsupported, tooOld int
	for _, q := range dbmeta.Queries() {
		switch q.Support(m) {
		case dbmeta.NotSupported, dbmeta.NotBuilt:
			unsupported++
			continue
		case dbmeta.TooOld:
			tooOld++
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
			if !strings.EqualFold(cols[i], fields[i].Name) {
				t.Errorf("%s: column %d is %q and the field is %q", q.Name(), i, cols[i], fields[i].Name)
			}
		}
		ran++
	}
	t.Logf("%d queries ran, %d not supported, %d too old", ran, unsupported, tooOld)
	if ran == 0 {
		t.Fatal("expected at least one query to run")
	}
}

// TestFirebirdTooOldIsTheAnswerBefore4 checks the three queries that read a
// catalog table Firebird 4.0 added. An older server must say so rather than
// returning nothing, which is D63.
func TestFirebirdTooOldIsTheAnswerBefore4(t *testing.T) {
	db := openFirebird(t)
	versions, err := dbmeta.Firebird.Version(t.Context(), db)
	if err != nil {
		t.Fatalf("reading the version: %v", err)
	}
	m, err := dbmeta.New(dbmeta.Firebird, versions)
	if err != nil {
		t.Fatalf("building the metadata: %v", err)
	}
	want := dbmeta.Supported
	if versions.Main().Compare(dbmeta.V(4, 0)) < 0 {
		want = dbmeta.TooOld
	}
	for _, q := range []struct {
		name    string
		support func(*dbmeta.Meta) dbmeta.Support
	}{
		{"settings", dbmeta.Settings.Support},
		{"publications", dbmeta.Publications.Support},
		{"publication_tables", dbmeta.PublicationTables.Support},
	} {
		if got := q.support(m); got != want {
			t.Errorf("%s reports %v on %v, expected %v", q.name, got, versions.Main(), want)
		}
	}
}

// TestFirebirdFindsTheFixture reads rows through the typed API and checks the
// objects the fixture built are there.
func TestFirebirdFindsTheFixture(t *testing.T) {
	db := openFirebird(t)
	m := setupFirebird(t, db)
	ctx := t.Context()

	want := map[string]string{
		"author": "table", "book": "table", "region": "table",
		"shipment": "table", "ticket": "table", "recent": "view",
	}
	got := map[string]string{}
	for v, err := range dbmeta.Tables.All(ctx, m, db, nil) {
		if err != nil {
			t.Fatalf("reading tables: %v", err)
		}
		got[strings.ToLower(v.Name)] = v.Type
	}
	for name, kind := range want {
		if got[name] != kind {
			t.Errorf("expected %s to be a %s, got %q", name, kind, got[name])
		}
	}

	// The comment the fixture wrote, and the absence of one where it wrote
	// none. Collapsing the two is the fault docs/NULLS.md exists for.
	var withComment, withoutComment int
	for v, err := range dbmeta.Tables.All(ctx, m, db, nil) {
		if err != nil {
			t.Fatalf("reading tables: %v", err)
		}
		switch {
		case strings.EqualFold(v.Name, "author"):
			if !v.Comment.Valid || v.Comment.V != "people who write" {
				t.Errorf("expected the author comment, got %#v", v.Comment)
			}
			withComment++
		case strings.EqualFold(v.Name, "book"):
			if v.Comment.Valid {
				t.Errorf("expected book to have no comment, got %q", v.Comment.V)
			}
			withoutComment++
		}
	}
	if withComment == 0 || withoutComment == 0 {
		t.Errorf("expected both a commented and an uncommented table, got %d and %d",
			withComment, withoutComment)
	}
}

// TestFirebirdColumns checks the parts of a column that Firebird stores in
// pieces and this model assembles.
func TestFirebirdColumns(t *testing.T) {
	db := openFirebird(t)
	m := setupFirebird(t, db)

	type want struct {
		dataType   string
		nullable   bool
		primaryKey bool
		identity   string
	}
	expect := map[string]want{
		"author_id": {"INTEGER", false, true, ""},
		"name":      {"VARCHAR(128)", false, false, ""},
		"rating":    {"INTEGER", true, false, ""},
		"shade":     {"VARCHAR(16)", true, false, ""},
	}
	seen := map[string]bool{}
	args := dbmeta.Args{Parent: "AUTHOR"}.Map()
	for v, err := range dbmeta.Columns.All(t.Context(), m, db, args) {
		if err != nil {
			t.Fatalf("reading columns: %v", err)
		}
		name := strings.ToLower(v.Name)
		w, ok := expect[name]
		if !ok {
			t.Errorf("unexpected column %q", v.Name)
			continue
		}
		seen[name] = true
		if v.DataType != w.dataType {
			t.Errorf("%s: expected type %q, got %q", name, w.dataType, v.DataType)
		}
		if v.Nullable != w.nullable {
			t.Errorf("%s: expected nullable %v, got %v", name, w.nullable, v.Nullable)
		}
		if v.PrimaryKey != w.primaryKey {
			t.Errorf("%s: expected primary key %v, got %v", name, w.primaryKey, v.PrimaryKey)
		}
		if v.Identity.V != w.identity {
			t.Errorf("%s: expected identity %q, got %q", name, w.identity, v.Identity.V)
		}
	}
	if len(seen) != len(expect) {
		t.Errorf("expected %d columns, saw %d", len(expect), len(seen))
	}

	// The identity column is on its own table, and it is the one thing here
	// that Firebird 3.0 gained and 2.5 did not have.
	var identity string
	for v, err := range dbmeta.Columns.All(t.Context(), m, db, dbmeta.Args{Parent: "TICKET"}.Map()) {
		if err != nil {
			t.Fatalf("reading columns: %v", err)
		}
		if strings.EqualFold(v.Name, "ticket_id") {
			identity = v.Identity.V
		}
	}
	if identity != "by default" {
		t.Errorf("expected ticket_id to be an identity by default, got %q", identity)
	}
}

// TestFirebirdSchemasAreNotSupported is the guard on the one decision this
// model makes that a reader would otherwise have to test for.
//
// Firebird has no schemas before 6.0, so the answer is ErrNotSupported and
// never an empty result. D34 is the rule and an empty result must never stand
// in for it. Without this test a query that broke and returned nothing would
// look the same as the deliberate answer.
func TestFirebirdSchemasAreNotSupported(t *testing.T) {
	db := openFirebird(t)
	m := setupFirebird(t, db)
	for _, q := range []struct {
		name    string
		support func(*dbmeta.Meta) dbmeta.Support
	}{
		{"schemas", dbmeta.Schemas.Support},
		{"current_schema", dbmeta.CurrentSchema.Support},
	} {
		if got := q.support(m); got != dbmeta.NotSupported {
			t.Errorf("%s reports %v, expected NotSupported", q.name, got)
		}
	}
	// And every object reports an empty schema rather than an invented one.
	for v, err := range dbmeta.Tables.All(t.Context(), m, db, nil) {
		if err != nil {
			t.Fatalf("reading tables: %v", err)
		}
		if v.Schema != "" {
			t.Errorf("expected %s to report no schema, got %q", v.Name, v.Schema)
		}
	}
}

// TestFirebirdConstraints checks the shape Firebird gives a constraint, which
// differs from PostgreSQL in one way worth asserting: NOT NULL is a
// constraint here and is not one there.
func TestFirebirdConstraints(t *testing.T) {
	db := openFirebird(t)
	m := setupFirebird(t, db)

	kinds := map[string]string{}
	var checkText string
	for v, err := range dbmeta.Constraints.All(t.Context(), m, db, dbmeta.Args{Parent: "BOOK"}.Map()) {
		if err != nil {
			t.Fatalf("reading constraints: %v", err)
		}
		kinds[strings.ToLower(v.Name)] = v.Type
		if strings.EqualFold(v.Name, "book_title_ck") {
			checkText = v.Definition.V
		}
	}
	for name, kind := range map[string]string{
		"book_pk":        "primary key",
		"book_author_fk": "foreign key",
		"book_title_uq":  "unique",
		"book_title_ck":  "check",
	} {
		if kinds[name] != kind {
			t.Errorf("expected %s to be a %s, got %q", name, kind, kinds[name])
		}
	}
	if !strings.Contains(strings.ToUpper(checkText), "CHAR_LENGTH") {
		t.Errorf("expected the check text, got %q", checkText)
	}

	// NOT NULL is a constraint in Firebird. It has a generated name, so this
	// counts the kind rather than naming one.
	var notNull int
	for v, err := range dbmeta.Constraints.All(t.Context(), m, db, nil) {
		if err != nil {
			t.Fatalf("reading constraints: %v", err)
		}
		if v.Type == "not null" {
			notNull++
		}
	}
	if notNull == 0 {
		t.Error("expected Firebird to report NOT NULL as a constraint, which is how it records one")
	}
}

// TestFirebirdCheckConstraintColumns is the guard on the second arm of the
// constraint columns query.
//
// A Firebird check has no index, so it has no index segments and the join
// every other constraint uses cannot reach it. The columns are recorded as
// the dependencies of the system triggers that implement the check, and
// nowhere else. Without this test that arm could stop working and the result
// would merely be a shorter list.
func TestFirebirdCheckConstraintColumns(t *testing.T) {
	db := openFirebird(t)
	m := setupFirebird(t, db)

	var cols []string
	for v, err := range dbmeta.ConstraintColumns.All(t.Context(), m, db, dbmeta.Args{Parent: "BOOK"}.Map()) {
		if err != nil {
			t.Fatalf("reading constraint columns: %v", err)
		}
		if strings.EqualFold(v.Constraint, "book_title_ck") {
			cols = append(cols, strings.ToLower(v.Name))
		}
	}
	if len(cols) != 1 || cols[0] != "title" {
		t.Errorf("expected the check to name title, got %v", cols)
	}
}

// TestFirebirdForeignKeyTarget checks the three joins it takes to reach the
// column a foreign key points at, because Firebird names the unique
// constraint rather than the table.
func TestFirebirdForeignKeyTarget(t *testing.T) {
	db := openFirebird(t)
	m := setupFirebird(t, db)

	var pairs []string
	for v, err := range dbmeta.ConstraintColumns.All(t.Context(), m, db, dbmeta.Args{Parent: "SHIPMENT"}.Map()) {
		if err != nil {
			t.Fatalf("reading constraint columns: %v", err)
		}
		if !strings.EqualFold(v.Constraint, "shipment_region_fk") {
			continue
		}
		if !v.ForeignTable.Valid {
			t.Errorf("expected %s to name a foreign table", v.Name)
			continue
		}
		pairs = append(pairs, strings.ToLower(v.Name+"->"+v.ForeignTable.V+"."+v.ForeignName.V))
	}
	want := []string{"country->region.country", "area->region.area"}
	if len(pairs) != len(want) {
		t.Fatalf("expected %d columns in the composite key, got %v", len(want), pairs)
	}
	for _, w := range want {
		if !slices.Contains(pairs, w) {
			t.Errorf("expected %s among %v", w, pairs)
		}
	}
}

// TestFirebirdRoutines checks that a package member is left out and a plain
// routine is not, which is the one judgement the Functions query makes.
func TestFirebirdRoutines(t *testing.T) {
	db := openFirebird(t)
	m := setupFirebird(t, db)

	kinds := map[string]string{}
	langs := map[string]string{}
	for v, err := range dbmeta.Functions.All(t.Context(), m, db, nil) {
		if err != nil {
			t.Fatalf("reading functions: %v", err)
		}
		kinds[strings.ToLower(v.Name)] = v.Kind
		langs[strings.ToLower(v.Name)] = v.Language
	}
	if kinds["doubled"] != "function" {
		t.Errorf("expected doubled to be a function, got %q", kinds["doubled"])
	}
	if kinds["book_count"] != "procedure" {
		t.Errorf("expected book_count to be a procedure, got %q", kinds["book_count"])
	}
	if _, ok := kinds["tripled"]; ok {
		t.Error("expected the package member tripled to be left out, the way models/oracle leaves one out")
	}
	if langs["doubled"] != "PSQL" {
		t.Errorf("expected doubled to be written in PSQL, got %q", langs["doubled"])
	}
}

// TestFirebirdEventTriggers checks that a trigger naming no table is read as
// an event trigger and not as an ordinary one.
func TestFirebirdEventTriggers(t *testing.T) {
	db := openFirebird(t)
	m := setupFirebird(t, db)
	ctx := t.Context()

	var events int
	for v, err := range dbmeta.EventTriggers.All(ctx, m, db, nil) {
		if err != nil {
			t.Fatalf("reading event triggers: %v", err)
		}
		if strings.EqualFold(v.Name, "dbmeta_on_connect") {
			if v.Event != "connect" {
				t.Errorf("expected a connect event, got %q", v.Event)
			}
			events++
		}
	}
	if events != 1 {
		t.Errorf("expected the database trigger among the event triggers, found %d", events)
	}
	for v, err := range dbmeta.Triggers.All(ctx, m, db, nil) {
		if err != nil {
			t.Fatalf("reading triggers: %v", err)
		}
		if strings.EqualFold(v.Name, "dbmeta_on_connect") {
			t.Error("expected the database trigger to be left out of Triggers, which reads the ones on a table")
		}
	}
}

// TestFirebirdPrivileges checks the grants the fixture made, including the
// column grant, which Firebird records in the same table as the rest.
func TestFirebirdPrivileges(t *testing.T) {
	db := openFirebird(t)
	m := setupFirebird(t, db)

	var access, columnAccess string
	for v, err := range dbmeta.Privileges.All(t.Context(), m, db, nil) {
		if err != nil {
			t.Fatalf("reading privileges: %v", err)
		}
		if strings.EqualFold(v.Name, "author") {
			access, columnAccess = v.Access.V, v.ColumnAccess
		}
	}
	if !strings.Contains(strings.ToUpper(access), "DBMETA_READER=S") {
		t.Errorf("expected the SELECT grant to the role, got %q", access)
	}
	if !strings.Contains(strings.ToUpper(columnAccess), "RATING:DBMETA_READER=U") {
		t.Errorf("expected the column grant on rating, got %q", columnAccess)
	}
}

// TestFirebirdRolesAndUsers checks that the two halves of the Roles result
// come from the two places Firebird keeps them, and that can_login is what
// separates them.
func TestFirebirdRolesAndUsers(t *testing.T) {
	db := openFirebird(t)
	m := setupFirebird(t, db)

	var user, role bool
	for v, err := range dbmeta.Roles.All(t.Context(), m, db, nil) {
		if err != nil {
			t.Fatalf("reading roles: %v", err)
		}
		switch {
		case strings.EqualFold(v.Name, "SYSDBA"):
			user = true
			if !v.CanLogin {
				t.Error("expected SYSDBA to be able to log in")
			}
			if !v.Superuser {
				t.Error("expected SYSDBA to be a superuser")
			}
		case strings.EqualFold(v.Name, "dbmeta_reader"):
			role = true
			if v.CanLogin {
				t.Error("expected a role not to be able to log in, which is what separates it from a user")
			}
		}
	}
	if !user {
		t.Error("expected SYSDBA from SEC$USERS")
	}
	if !role {
		t.Error("expected dbmeta_reader from RDB$ROLES")
	}
}
