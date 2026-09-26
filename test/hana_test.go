package test

import (
	"context"
	"database/sql"
	"os"
	"slices"
	"strings"
	"testing"

	_ "github.com/SAP/go-hdb/driver"

	"github.com/xo/dbmeta"
	_ "github.com/xo/dbmeta/models/hana"
	hafixture "github.com/xo/dbmeta/models/hana/fixture"
)

// setupHANA builds the fixture and returns the metadata for the server.
//
// The teardown runs first, because a run that failed part way leaves the
// schema behind and CREATE SCHEMA then fails rather than the test reporting
// what actually went wrong. HANA has no DROP ... IF EXISTS, so an error from
// a teardown step is expected and ignored.
func setupHANA(t *testing.T, db *sql.DB) *dbmeta.Meta {
	t.Helper()
	ctx := t.Context()
	versions, err := dbmeta.HANA.Version(ctx, db)
	if err != nil {
		t.Fatalf("reading the version: %v", err)
	}
	down, err := hafixture.Everything.ResolveTeardown(versions)
	if err != nil {
		t.Fatalf("resolving the teardown: %v", err)
	}
	drop := func(c context.Context) {
		for _, step := range down {
			if !step.Skipped {
				//nolint:errcheck // HANA has no DROP IF EXISTS
				db.ExecContext(c, step.Query)
			}
		}
	}
	drop(ctx)

	up, err := hafixture.Everything.ResolveSetup(versions)
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

	m, err := dbmeta.New(dbmeta.HANA, versions)
	if err != nil {
		t.Fatalf("building the metadata: %v", err)
	}
	t.Logf("server reports %s, fixture ran %d steps and skipped %d", versions, ran, skipped)
	return m
}

// openHANA returns a connection to the server named by DBMETA_HDB.
func openHANA(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("DBMETA_HDB")
	if dsn == "" {
		t.Skip("set DBMETA_HDB to run against a real server")
	}
	db, err := sql.Open("hdb", dsn)
	if err != nil {
		t.Fatalf("opening: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.PingContext(t.Context()); err != nil {
		t.Fatalf("connecting: %v", err)
	}
	return db
}

// TestHANAEveryQueryRuns executes every query the model answers and checks
// the columns match the declared fields. It needs no fixture: this is the
// first pass, and it proves the SQL rather than the contents.
func TestHANAEveryQueryRuns(t *testing.T) {
	db := openHANA(t)
	m := setupHANA(t, db)
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
	t.Logf("%d queries ran, %d not supported by SAP HANA", ran, unsupported)
	if ran == 0 {
		t.Fatal("expected at least one query to run")
	}
}

// TestHANAFixtureBuilds is the first thing to run against a new server: it
// proves every statement the fixture makes is accepted.
func TestHANAFixtureBuilds(t *testing.T) {
	db := openHANA(t)
	m := setupHANA(t, db)
	var tables int
	for v, err := range dbmeta.Tables.All(t.Context(), m, db,
		dbmeta.Args{Schema: hafixture.Everything.Schema}.Map()) {
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

// haArgs is the filter the fixture's objects sit behind.
func haArgs() map[string]any {
	return dbmeta.Args{Schema: hafixture.Everything.Schema}.Map()
}

// TestHANAVersion reads the version and checks what the model makes of it.
func TestHANAVersion(t *testing.T) {
	db := openHANA(t)
	versions, err := dbmeta.HANA.Version(t.Context(), db)
	if err != nil {
		t.Fatalf("reading the version: %v", err)
	}
	main := versions.Main()
	if main.Unknown || len(main.Parts) < 3 {
		t.Fatalf("expected a parsed version, got %v", versions)
	}
	if main.Parts[0] != 2 {
		t.Errorf("expected SAP HANA 2, got %v", main)
	}
	if !strings.HasPrefix(versions.Display, "SAP HANA ") {
		t.Errorf("expected the display to name the product, got %q", versions.Display)
	}
}

// TestHANAColumns checks the parts of a column HANA stores separately and
// this model assembles.
func TestHANAColumns(t *testing.T) {
	db := openHANA(t)
	m := setupHANA(t, db)

	type want struct {
		dataType   string
		nullable   bool
		primaryKey bool
		hasDefault bool
	}
	expect := map[string]want{
		"author_id": {"INTEGER", false, true, false},
		"name":      {"NVARCHAR(128)", false, false, false},
		"rating":    {"INTEGER", true, false, false},
		"shade":     {"NVARCHAR(16)", true, false, true},
	}
	seen := map[string]bool{}
	args := dbmeta.Args{Schema: hafixture.Everything.Schema, Parent: "AUTHOR"}.Map()
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
		if v.Default.Valid != w.hasDefault {
			t.Errorf("%s: expected a default %v, got %#v", name, w.hasDefault, v.Default)
		}
	}
	if len(seen) != len(expect) {
		t.Errorf("expected %d columns, saw %d", len(expect), len(seen))
	}

	var identity string
	args = dbmeta.Args{Schema: hafixture.Everything.Schema, Parent: "TICKET"}.Map()
	for v, err := range dbmeta.Columns.All(t.Context(), m, db, args) {
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

// TestHANAConstraints checks that the kind is derived correctly, since
// SYS.CONSTRAINTS has no type column and a foreign key is in another view
// altogether.
func TestHANAConstraints(t *testing.T) {
	db := openHANA(t)
	m := setupHANA(t, db)

	kinds := map[string]string{}
	var checkText string
	args := dbmeta.Args{Schema: hafixture.Everything.Schema, Parent: "BOOK"}.Map()
	for v, err := range dbmeta.Constraints.All(t.Context(), m, db, args) {
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
	if !strings.Contains(strings.ToUpper(checkText), "LENGTH") {
		t.Errorf("expected the check text, got %q", checkText)
	}
}

// TestHANAForeignKeyTarget checks the composite foreign key, which HANA
// records on the row itself rather than through the referenced constraint.
func TestHANAForeignKeyTarget(t *testing.T) {
	db := openHANA(t)
	m := setupHANA(t, db)

	var pairs []string
	args := dbmeta.Args{Schema: hafixture.Everything.Schema, Parent: "SHIPMENT"}.Map()
	for v, err := range dbmeta.ConstraintColumns.All(t.Context(), m, db, args) {
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
	for _, w := range []string{"country->region.country", "area->region.area"} {
		if !slices.Contains(pairs, w) {
			t.Errorf("expected %s among %v", w, pairs)
		}
	}
}

// TestHANARowAndColumnStore checks the one analogy this model draws.
//
// HANA has no catalog of storage kinds, so AccessMethods counts the tables
// that name each one. The fixture builds both a row table and a column
// table so that the answer is not trivially one row.
func TestHANARowAndColumnStore(t *testing.T) {
	db := openHANA(t)
	m := setupHANA(t, db)

	kinds := map[string]bool{}
	for v, err := range dbmeta.AccessMethods.All(t.Context(), m, db, nil) {
		if err != nil {
			t.Fatalf("reading access methods: %v", err)
		}
		kinds[v.Name] = true
	}
	for _, want := range []string{"row store", "column store"} {
		if !kinds[want] {
			t.Errorf("expected %q among %v", want, kinds)
		}
	}

	// And the table kind reaches Tables, which is where a caller looks.
	types := map[string]string{}
	for v, err := range dbmeta.Tables.All(t.Context(), m, db, haArgs()) {
		if err != nil {
			t.Fatalf("reading tables: %v", err)
		}
		types[strings.ToLower(v.Name)] = v.Type
	}
	if types["ledger"] != "row table" {
		t.Errorf("expected ledger to be a row table, got %q", types["ledger"])
	}
	if types["author"] != "column table" {
		t.Errorf("expected author to be a column table, got %q", types["author"])
	}
}

// TestHANAHasNoColumnGrant asserts an absence, so that it stays a decision
// rather than becoming a bug.
//
// SYS.GRANTED_PRIVILEGES carries a COLUMN_NAME column and HANA 2.0 SPS 08
// has no GRANT syntax that fills it: every spelling of a column list is a
// syntax error. The Privileges query keeps column_access because the
// catalog has the column, and it is always empty. Without this test a
// query that stopped reading it would look the same.
func TestHANAHasNoColumnGrant(t *testing.T) {
	db := openHANA(t)
	m := setupHANA(t, db)

	if _, err := db.ExecContext(t.Context(),
		`GRANT UPDATE (rating) ON dbmeta_fixture.author TO dbmeta_reader`); err == nil {
		t.Error("SAP HANA accepted a column level grant. It did not on 2.0 SPS 08," +
			" and the Privileges query and docs/COVERAGE.md both say so.")
	}

	var found bool
	for v, err := range dbmeta.Privileges.All(t.Context(), m, db, haArgs()) {
		if err != nil {
			t.Fatalf("reading privileges: %v", err)
		}
		if strings.EqualFold(v.Name, "author") {
			found = true
			if v.ColumnAccess != "" {
				t.Errorf("expected no column grant, got %q", v.ColumnAccess)
			}
			if !strings.Contains(strings.ToUpper(v.Access.V), "DBMETA_READER=SELECT") {
				t.Errorf("expected the SELECT grant to the role, got %q", v.Access.V)
			}
		}
	}
	if !found {
		t.Error("expected the grants on author")
	}
}

// TestHANAForeignDataAnswers checks the four smart data access queries.
//
// No other model here answers all four from a catalog. The fixture builds
// no remote source, because federating needs a second database, so these
// are verified to run and to return nothing rather than to return rows.
func TestHANAForeignDataAnswers(t *testing.T) {
	db := openHANA(t)
	m := setupHANA(t, db)
	ctx := t.Context()

	// The wrapper list is the one that has rows without a remote source:
	// HANA ships its adapters whether anything uses them or not.
	var wrappers int
	for v, err := range dbmeta.ForeignDataWrappers.All(ctx, m, db, nil) {
		if err != nil {
			t.Fatalf("reading foreign data wrappers: %v", err)
		}
		_ = v
		wrappers++
	}
	if wrappers == 0 {
		t.Error("expected SYS.ADAPTERS to list the adapters the server ships")
	}

	for name, run := range map[string]func() error{
		"foreign servers": func() error {
			for _, err := range dbmeta.ForeignServers.All(ctx, m, db, nil) {
				if err != nil {
					return err
				}
			}
			return nil
		},
		"user mappings": func() error {
			for _, err := range dbmeta.UserMappings.All(ctx, m, db, nil) {
				if err != nil {
					return err
				}
			}
			return nil
		},
		"foreign tables": func() error {
			for _, err := range dbmeta.ForeignTables.All(ctx, m, db, nil) {
				if err != nil {
					return err
				}
			}
			return nil
		},
		"subscriptions": func() error {
			for _, err := range dbmeta.Subscriptions.All(ctx, m, db, nil) {
				if err != nil {
					return err
				}
			}
			return nil
		},
	} {
		if err := run(); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}
