package test

import (
	"context"
	"database/sql"
	"errors"
	"slices"
	"strings"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	_ "modernc.org/sqlite"

	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/models/sqlite3"
	sqfixture "github.com/xo/dbmeta/models/sqlite3/fixture"
)

// SQLite needs no server and no environment variable. It is a library, so the
// test makes a file in the directory the test framework gives it and removes
// it with everything else. Every other integration test here skips without a
// server, and this one never skips. See D42.
//
// Every test runs twice, once per driver. The two are not interchangeable:
// mattn/go-sqlite3 compiles the upstream SQLite source and modernc.org/sqlite
// is a translation of it, and they ship different library versions. A query
// that works on one and not the other is worth finding. See D48.
//
// mattn/go-sqlite3 needs cgo, which the test module may use and the root
// module may not.
var sqliteDrivers = []string{"sqlite3", "sqlite"}

func openSQLiteWith(t *testing.T, driver string) *sql.DB {
	t.Helper()
	db, err := sql.Open(driver, embeddedFile(t, "DBMETA_SQLITE3", "dbmeta.db"))
	if err != nil {
		t.Fatalf("opening with %s: %v", driver, err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.PingContext(t.Context()); err != nil {
		t.Fatalf("connecting with %s: %v", driver, err)
	}
	return db
}

// eachSQLite runs fn once per driver, as a subtest named for it, with the
// fixture already built.
func eachSQLite(t *testing.T, fn func(t *testing.T, db *sql.DB, m *dbmeta.Meta)) {
	t.Helper()
	for _, driver := range sqliteDrivers {
		t.Run(driver, func(t *testing.T) {
			db := openSQLiteWith(t, driver)
			fn(t, db, setupSQLite(t, db))
		})
	}
}

func setupSQLite(t *testing.T, db *sql.DB) *dbmeta.Meta {
	t.Helper()
	versions, err := dbmeta.SQLite3.Version(t.Context(), db)
	if err != nil {
		t.Fatalf("reading the version: %v", err)
	}
	m, err := dbmeta.New(dbmeta.SQLite3, versions)
	if err != nil {
		t.Fatalf("building the meta: %v", err)
	}
	run := func(ctx context.Context, steps []sqfixture.Result, fatal bool) {
		for _, s := range steps {
			if _, err := db.ExecContext(ctx, s.Query); err != nil && fatal {
				t.Fatalf("%s: %v\n%s", s.Name, err, s.Query)
			}
		}
	}
	up, err := sqfixture.Everything.ResolveSetup(versions)
	if err != nil {
		t.Fatalf("resolving the setup: %v", err)
	}
	down, err := sqfixture.Everything.ResolveTeardown(versions)
	if err != nil {
		t.Fatalf("resolving the teardown: %v", err)
	}
	run(t.Context(), up, true)
	t.Cleanup(func() { run(context.Background(), down, false) })
	t.Logf("library reports %s, fixture ran %d steps", m, len(up))
	return m
}

func sqArgs() map[string]any {
	return dbmeta.Args{Schema: sqfixture.Everything.Schema}.Map()
}

// TestSQLiteEveryQueryRuns executes every query SQLite answers and checks the
// columns match the declared fields.
func TestSQLiteEveryQueryRuns(t *testing.T) {
	eachSQLite(t, func(t *testing.T, db *sql.DB, m *dbmeta.Meta) {
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
	})
}

// TestSQLiteScanning reads rows through the typed API, which the rendering
// test cannot check. A Go type the driver cannot fill fails here.
func TestSQLiteScanning(t *testing.T) {
	eachSQLite(t, func(t *testing.T, db *sql.DB, m *dbmeta.Meta) {
		ctx := t.Context()

		want := map[string]string{"author": "table", "book": "table", "sales": "table", "recent": "view"}
		found := make(map[string]string)
		for v, err := range dbmeta.Tables.All(ctx, m, db, sqArgs()) {
			if err != nil {
				t.Fatalf("reading tables: %v", err)
			}
			found[v.Name] = v.Type
			if v.Schema != "main" {
				t.Errorf("expected schema main, got %q", v.Schema)
			}
		}
		for name, kind := range want {
			if found[name] != kind {
				t.Errorf("expected %q to be a %s, got %q", name, kind, found[name])
			}
		}
		// sqlite_sequence exists because the fixture uses AUTOINCREMENT, and it
		// must not be listed unless the caller asks for the system objects
		if _, ok := found["sqlite_sequence"]; ok {
			t.Error("expected a system table to be left out")
		}
		all := dbmeta.Args{Schema: "main", WithSystem: true}.Map()
		var system bool
		for v, err := range dbmeta.Tables.All(ctx, m, db, all) {
			if err != nil {
				t.Fatalf("reading tables: %v", err)
			}
			system = system || strings.HasPrefix(v.Name, "sqlite_")
		}
		if !system {
			t.Error("expected with_system to include the tables SQLite keeps for itself")
		}
	})
}

// TestSQLiteColumns checks the parts of a column SQLite reports differently
// from every other database: an unenforced declared type, the rowid, and a
// generated column in both of its forms.
func TestSQLiteColumns(t *testing.T) {
	eachSQLite(t, func(t *testing.T, db *sql.DB, m *dbmeta.Meta) {
		ctx := t.Context()

		cols := make(map[string]dbmeta.Column)
		for v, err := range dbmeta.Columns.All(ctx, m, db, sqArgs()) {
			if err != nil {
				t.Fatalf("reading columns: %v", err)
			}
			cols[v.Table+"."+v.Name] = v
		}
		for _, c := range []struct {
			key       string
			ordinal   int
			dataType  string
			nullable  bool
			identity  string
			generated string
		}{
			{key: "author.author_id", ordinal: 1, dataType: "INTEGER", nullable: true, identity: "rowid"},
			{key: "author.name", ordinal: 2, dataType: "TEXT"},
			{key: "author.rating", ordinal: 3, dataType: "INTEGER", nullable: true},
			{key: "sales.title_length", ordinal: 5, dataType: "INTEGER", nullable: true, generated: "stored"},
			{key: "sales.doubled", ordinal: 4, dataType: "INTEGER", nullable: true, generated: "virtual"},
		} {
			got, ok := cols[c.key]
			if !ok {
				t.Errorf("expected a column %s", c.key)
				continue
			}
			if got.Ordinal != c.ordinal {
				t.Errorf("%s: expected ordinal %d, got %d", c.key, c.ordinal, got.Ordinal)
			}
			if got.DataType != c.dataType {
				t.Errorf("%s: expected %q, got %q", c.key, c.dataType, got.DataType)
			}
			if got.Nullable != c.nullable {
				t.Errorf("%s: expected nullable=%v, got %v", c.key, c.nullable, got.Nullable)
			}
			if got.Identity.V != c.identity {
				t.Errorf("%s: expected identity %q, got %q", c.key, c.identity, got.Identity.V)
			}
			if got.Generated.V != c.generated {
				t.Errorf("%s: expected generated %q, got %q", c.key, c.generated, got.Generated.V)
			}
		}
		// A default is reported as SQLite stores it, with the quotes, and a
		// column with no default is absent rather than an empty string. That is
		// the docs/NULLS.md rule.
		if got := cols["author.shade"]; !got.Default.Valid || got.Default.V != "'red'" {
			t.Errorf("expected the default to keep its quotes, got %v", got.Default)
		}
		if got := cols["author.name"]; got.Default.Valid {
			t.Errorf("expected no default on a column that has none, got %v", got.Default)
		}
	})
}

// TestSQLiteConstraints covers the query that answers incompletely on purpose.
// It reports a primary key, a unique constraint and a foreign key exactly, and
// it never reports a check constraint, because SQLite keeps one only as DDL
// text. The fixture has two check constraints so that the absence is tested
// rather than assumed.
func TestSQLiteConstraints(t *testing.T) {
	eachSQLite(t, func(t *testing.T, db *sql.DB, m *dbmeta.Meta) {
		ctx := t.Context()

		byType := make(map[string][]dbmeta.Constraint)
		for v, err := range dbmeta.Constraints.All(ctx, m, db, sqArgs()) {
			if err != nil {
				t.Fatalf("reading constraints: %v", err)
			}
			byType[v.Type] = append(byType[v.Type], v)
		}
		for _, kind := range []string{"primary key", "unique", "foreign key"} {
			if len(byType[kind]) == 0 {
				t.Errorf("expected at least one %s constraint", kind)
			}
		}
		if got := byType["check"]; len(got) != 0 {
			t.Errorf("expected no check constraint, because SQLite does not publish one, got %v", got)
		}
		// the composite primary key on sales is one row naming both columns
		var composite bool
		for _, v := range byType["primary key"] {
			if v.Table == "sales" {
				composite = true
				if !v.Definition.Valid || !strings.Contains(v.Definition.V, "sold_on") ||
					!strings.Contains(v.Definition.V, "region") {
					t.Errorf("expected both key columns, got %v", v.Definition)
				}
			}
		}
		if !composite {
			t.Error("expected the composite primary key on sales")
		}
		// The foreign key names what it points at. There are two, so each is
		// checked by the table it is on: book references author with
		// ON DELETE CASCADE, and shipment references region with no action.
		var toAuthor, toRegion bool
		for _, v := range byType["foreign key"] {
			switch v.Table {
			case "book":
				toAuthor = true
				if !v.Definition.Valid || !strings.Contains(v.Definition.V, "author") {
					t.Errorf("expected the book key to name author, got %v", v.Definition)
				}
				if !v.Deferred {
					t.Error("expected ON DELETE CASCADE to report as acting")
				}
			case "shipment":
				toRegion = true
				if !v.Definition.Valid || !strings.Contains(v.Definition.V, "region") {
					t.Errorf("expected the shipment key to name region, got %v", v.Definition)
				}
				if v.Deferred {
					t.Error("expected a key with no referential action to report as not acting")
				}
			}
		}
		if !toAuthor || !toRegion {
			t.Errorf("expected both foreign keys, got author=%v region=%v", toAuthor, toRegion)
		}
	})
}

// TestSQLiteIndexes checks that an index SQLite made for a constraint is told
// apart from one the fixture created, and that a descending index reports as
// one.
func TestSQLiteIndexes(t *testing.T) {
	eachSQLite(t, func(t *testing.T, db *sql.DB, m *dbmeta.Meta) {
		ctx := t.Context()

		kinds := make(map[string]dbmeta.Index)
		for v, err := range dbmeta.Indexes.All(ctx, m, db, sqArgs()) {
			if err != nil {
				t.Fatalf("reading indexes: %v", err)
			}
			kinds[v.Name] = v
		}
		if got := kinds["book_published"]; got.Type != "btree" || got.Unique {
			t.Errorf("expected a plain index, got %+v", got)
		}
		// SQLite makes an index for a UNIQUE clause and for a composite primary
		// key, and names both sqlite_autoindex. The origin tells them apart, and
		// the fixture has one of each: book has UNIQUE(title) and sales has a
		// two column primary key.
		var unique, primary bool
		for name, v := range kinds {
			if !strings.HasPrefix(name, "sqlite_autoindex") {
				continue
			}
			if !v.Unique {
				t.Errorf("%s: expected an index SQLite made for a constraint to be unique, got %+v", name, v)
			}
			switch v.Type {
			case "unique constraint":
				unique = true
				if v.Primary {
					t.Errorf("%s: expected it not to be the primary key", name)
				}
			case "primary key":
				primary = true
				if !v.Primary {
					t.Errorf("%s: expected it to be the primary key", name)
				}
			default:
				t.Errorf("%s: unexpected type %q", name, v.Type)
			}
		}
		if !unique {
			t.Error("expected the index SQLite made for the UNIQUE clause on book")
		}
		if !primary {
			t.Error("expected the index SQLite made for the composite primary key on sales")
		}

		var descending bool
		for v, err := range dbmeta.IndexColumns.All(ctx, m, db, sqArgs()) {
			if err != nil {
				t.Fatalf("reading index columns: %v", err)
			}
			if v.Index == "book_title_desc" {
				descending = true
				if !v.Descending {
					t.Errorf("expected a descending column, got %+v", v)
				}
				if v.Name != "title" {
					t.Errorf("expected title, got %q", v.Name)
				}
				if v.Ordinal != 1 {
					t.Errorf("expected ordinal 1, got %d", v.Ordinal)
				}
			}
		}
		if !descending {
			t.Error("expected the descending index to be listed")
		}
	})
}

// TestSQLiteSettings covers the query built from a list of pragmas. It is the
// one query here assembled in Go rather than written out, so a mistake in the
// assembly shows as a wrong column set or a missing row.
func TestSQLiteSettings(t *testing.T) {
	eachSQLite(t, func(t *testing.T, db *sql.DB, m *dbmeta.Meta) {
		ctx := t.Context()

		got := make(map[string]dbmeta.Setting)
		for v, err := range dbmeta.Settings.All(ctx, m, db, nil) {
			if err != nil {
				t.Fatalf("reading settings: %v", err)
			}
			got[v.Name] = v
		}
		if len(got) < 30 {
			t.Errorf("expected every pragma in the list, got %d", len(got))
		}
		if v := got["page_size"]; v.Value.V == "" || v.Type.V != "number" || v.Context.V != "database" {
			t.Errorf("unexpected page_size: %+v", v)
		}
		if v := got["encoding"]; v.Value.V != "UTF-8" || v.Type.V != "text" {
			t.Errorf("unexpected encoding: %+v", v)
		}
		// the name filter narrows the assembled statement rather than the pragmas
		var narrowed int
		for v, err := range dbmeta.Settings.All(ctx, m, db, map[string]any{"name": "page_%"}) {
			if err != nil {
				t.Fatalf("reading settings: %v", err)
			}
			narrowed++
			if !strings.HasPrefix(v.Name, "page_") {
				t.Errorf("expected only page_ settings, got %q", v.Name)
			}
		}
		if narrowed == 0 {
			t.Error("expected the filter to match something")
		}
	})
}

// TestSQLiteFunctions checks the query that folds one row per argument count
// into one row per function, and the kind it reports.
func TestSQLiteFunctions(t *testing.T) {
	eachSQLite(t, func(t *testing.T, db *sql.DB, m *dbmeta.Meta) {
		ctx := t.Context()

		// One row per name and kind. SQLite lists a function once per argument
		// count, which the query folds away, and max really is both a scalar
		// function of two arguments and an aggregate, so it keeps two rows.
		kinds := make(map[string]string)
		seen := make(map[string]bool)
		args := make(map[string]string)
		all := map[string]any{"with_system": true}
		for v, err := range dbmeta.Functions.All(ctx, m, db, all) {
			if err != nil {
				t.Fatalf("reading functions: %v", err)
			}
			key := v.Name + "/" + v.Kind
			if seen[key] {
				t.Errorf("%s is listed twice, so the argument counts were not folded", key)
			}
			seen[key] = true
			kinds[v.Name] = v.Kind
			args[key] = v.ArgTypes
		}
		if !seen["max/s"] && !seen["max/func"] {
			t.Error("expected max to be listed as a plain function as well")
		}
		if !seen["max/window"] {
			t.Error("expected max to be listed as a window function as well")
		}
		// length takes one argument and nothing else, so its folded list is one
		if got := args["length/func"]; got != "1" {
			t.Errorf("expected length to take one argument, got %q", got)
		}
		// printf takes any number
		if got := args["printf/func"]; !strings.Contains(got, "variadic") {
			t.Errorf("expected printf to be variadic, got %q", got)
		}
		// SQLite reports an aggregate usable over a window as a window function,
		// which is why Aggregates is unsupported. This records that rather than
		// wishing it away.
		if kinds["length"] != "func" {
			t.Errorf("expected length to be a plain function, got %q", kinds["length"])
		}
		if kinds["sum"] != "window" {
			t.Errorf("expected SQLite to call sum a window function, got %q", kinds["sum"])
		}
		if kinds["row_number"] != "window" {
			t.Errorf("expected row_number to be a window function, got %q", kinds["row_number"])
		}
	})
}

// TestSQLiteUnsupported checks that what SQLite does not have says so, rather
// than answering with an empty result. D34 requires that difference.
func TestSQLiteUnsupported(t *testing.T) {
	eachSQLite(t, func(t *testing.T, _ *sql.DB, m *dbmeta.Meta) {
		for _, q := range []dbmeta.AnyQuery{
			// no users at all, so none of these mean anything
			dbmeta.Roles, dbmeta.RoleGrants, dbmeta.Privileges, dbmeta.RoleSettings,
			dbmeta.DefaultACLs,
			// no type catalog: a declared type is an unenforced affinity hint
			dbmeta.Types, dbmeta.Domains, dbmeta.Casts, dbmeta.Conversions,
			// SQLite records no comment on anything
			dbmeta.Comments,
			// an internal counter is not a sequence, and a module is not an
			// extension. Both were suggested and rejected. See docs/COVERAGE.md.
			dbmeta.Sequences, dbmeta.Extensions, dbmeta.AccessMethods,
			dbmeta.ExtendedStats, dbmeta.ForeignTables, dbmeta.Tablespaces,
			// SQLite reports an aggregate as a window function and cannot tell
			// sum from row_number
			dbmeta.Aggregates,
			// The kinds added under D47 that SQLite cannot answer. A function is
			// compiled C with no named parameters, there is no enumerated type,
			// and sqlite_stat1 holds one string per index rather than anything
			// about a column's values.
			dbmeta.RoutineParameters, dbmeta.EnumValues, dbmeta.ColumnStats,
		} {
			if got := q.Support(m); got != dbmeta.NotSupported {
				t.Errorf("%s: expected it to be reported unsupported, got %v", q.Name(), got)
			}
			if _, _, err := q.Build(m, nil); !errors.Is(err, dbmeta.ErrNotSupported) {
				t.Errorf("%s: expected ErrNotSupported, got: %v", q.Name(), err)
			}
		}
	})
}

// TestSQLiteVersion checks that the version comes from the library rather than
// from a server, which is what makes SQLite different from everything else
// here.
func TestSQLiteVersion(t *testing.T) {
	eachSQLite(t, func(t *testing.T, db *sql.DB, _ *dbmeta.Meta) {
		versions, err := dbmeta.SQLite3.Version(t.Context(), db)
		if err != nil {
			t.Fatalf("reading the version: %v", err)
		}
		if versions.Main().Unknown {
			t.Fatal("expected a version")
		}
		if !strings.HasPrefix(versions.String(), "SQLite 3.") {
			t.Errorf("expected a SQLite 3 display line, got %q", versions.String())
		}
		if got := versions.Main().Parts[0]; got != 3 {
			t.Errorf("expected major 3, got %d", got)
		}
		// the reference the queries were written against is a 3.x release
		if !strings.HasPrefix(sqlite3.Reference, "3.") {
			t.Errorf("unexpected reference %q", sqlite3.Reference)
		}
	})
}

// TestSQLiteConstraintColumns covers the kind that replaces a rendered
// constraint definition. The fixture has a composite primary key and a foreign
// key, so the ordinals have something to order.
func TestSQLiteConstraintColumns(t *testing.T) {
	eachSQLite(t, func(t *testing.T, db *sql.DB, m *dbmeta.Meta) {
		ctx := t.Context()

		got := make(map[string][]string)
		foreign := make(map[string]string)
		for v, err := range dbmeta.ConstraintColumns.All(ctx, m, db, sqArgs()) {
			if err != nil {
				t.Fatalf("reading constraint columns: %v", err)
			}
			key := v.Table + "." + v.Constraint
			if int64(len(got[key]))+1 != v.Ordinal {
				t.Errorf("%s: expected ordinal %d, got %d", key, len(got[key])+1, v.Ordinal)
			}
			got[key] = append(got[key], v.Name)
			if v.ForeignTable.Valid {
				foreign[key] = v.ForeignTable.V + "." + v.ForeignName.V
			}
		}
		// the composite primary key on sales, in declaration order
		if want := []string{"sold_on", "region"}; !slices.Equal(got["sales.pk_sales"], want) {
			t.Errorf("expected %v, got %v", want, got["sales.pk_sales"])
		}
		// the foreign key from book to author
		var found bool
		for key, ref := range foreign {
			if strings.HasPrefix(key, "book.fk_book") {
				found = true
				if ref != "author.author_id" {
					t.Errorf("%s: expected author.author_id, got %s", key, ref)
				}
			}
		}
		if !found {
			t.Error("expected the foreign key from book")
		}
		// a check constraint has no columns here, for the same reason it has no
		// row in Constraints
		for key := range got {
			if strings.Contains(key, "title_not_empty") {
				t.Errorf("expected no check constraint, got %s", key)
			}
		}
	})
}

// TestSQLiteViews covers the kind dbtpl needs. SQLite keeps the whole CREATE
// statement and nothing else, so that is what the definition is.
func TestSQLiteViews(t *testing.T) {
	eachSQLite(t, func(t *testing.T, db *sql.DB, m *dbmeta.Meta) {
		ctx := t.Context()

		v, ok, err := dbmeta.First(dbmeta.Views.All(ctx, m, db, sqArgs()))
		if err != nil {
			t.Fatalf("reading views: %v", err)
		}
		if !ok {
			t.Fatal("expected the fixture view")
		}
		if v.Name != "recent" {
			t.Errorf("expected recent, got %q", v.Name)
		}
		if !strings.HasPrefix(v.Definition, "CREATE VIEW") {
			t.Errorf("expected the whole statement, got %q", v.Definition)
		}
		if v.Updatable.V || v.CheckOption.Valid {
			t.Errorf("expected a read only view with no check option, got %+v", v)
		}
	})
}

// TestSQLiteCurrentSchemaAndPrimaryKey covers the last two additions. The
// primary key flag is free here, because the pragma already reports it.
func TestSQLiteCurrentSchemaAndPrimaryKey(t *testing.T) {
	eachSQLite(t, func(t *testing.T, db *sql.DB, m *dbmeta.Meta) {
		ctx := t.Context()

		v, ok, err := dbmeta.First(dbmeta.CurrentSchema.All(ctx, m, db, nil))
		if err != nil {
			t.Fatalf("reading the current schema: %v", err)
		}
		if !ok || v.Name != "main" {
			t.Errorf("expected main, got %q ok=%v", v.Name, ok)
		}

		keyed := make(map[string]bool)
		for c, err := range dbmeta.Columns.All(ctx, m, db, sqArgs()) {
			if err != nil {
				t.Fatalf("reading columns: %v", err)
			}
			keyed[c.Table+"."+c.Name] = c.PrimaryKey
		}
		for key, want := range map[string]bool{
			"author.author_id": true,
			"author.name":      false,
			"sales.sold_on":    true,
			"sales.region":     true,
			"sales.amount":     false,
		} {
			if got, ok := keyed[key]; !ok {
				t.Errorf("expected a column %s", key)
			} else if got != want {
				t.Errorf("%s: expected primary_key=%v, got %v", key, want, got)
			}
		}
	})
}
