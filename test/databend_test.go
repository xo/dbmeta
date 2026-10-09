package test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"testing"

	_ "github.com/xo/dbimp/databend"

	"github.com/xo/dbmeta"
	_ "github.com/xo/dbmeta/models/databend"
	dbfixture "github.com/xo/dbmeta/models/databend/fixture"
)

// openDatabend returns a connection to the server named by DBMETA_DATABEND.
//
// dburl names dbimp's databend driver, which replaced databend-go in dbimp
// v0.6.0, so that is the one used here (D154).
func openDatabend(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("DBMETA_DATABEND")
	if dsn == "" {
		t.Skip("set DBMETA_DATABEND to run against a real server")
	}
	db, err := sql.Open("databend", dsn)
	if err != nil {
		t.Fatalf("opening: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.PingContext(t.Context()); err != nil {
		t.Fatalf("connecting: %v", err)
	}
	return db
}

// setupDatabend builds the fixture and returns the metadata for the server.
// It tears down first, because a run that failed part way leaves the
// database, and the objects that belong to no database, behind.
func setupDatabend(t *testing.T, db *sql.DB) *dbmeta.Meta {
	t.Helper()
	ctx := t.Context()
	versions, err := dbmeta.Databend.Version(ctx, db)
	if err != nil {
		t.Fatalf("reading the version: %v", err)
	}
	down, err := dbfixture.Everything.ResolveTeardown(versions)
	if err != nil {
		t.Fatalf("resolving the teardown: %v", err)
	}
	for _, step := range down {
		if !step.Skipped {
			//nolint:errcheck // a teardown before setup is best effort
			db.ExecContext(ctx, step.Query)
		}
	}
	up, err := dbfixture.Everything.ResolveSetup(versions)
	if err != nil {
		t.Fatalf("resolving the setup: %v", err)
	}
	for _, step := range up {
		if step.Skipped {
			continue
		}
		if _, err := db.ExecContext(ctx, step.Query); err != nil {
			t.Fatalf("setup %s: %v\n%s", step.Name, err, step.Query)
		}
	}
	t.Cleanup(func() {
		for _, step := range down {
			if !step.Skipped {
				//nolint:errcheck // the test has already reported what matters
				db.ExecContext(context.WithoutCancel(ctx), step.Query)
			}
		}
	})
	m, err := dbmeta.New(dbmeta.Databend, versions)
	if err != nil {
		t.Fatalf("building the metadata: %v", err)
	}
	t.Logf("server reports %s", versions)
	return m
}

// dbArgs is the filter the fixture's objects sit behind.
func dbArgs() map[string]any {
	return dbmeta.Args{Schema: dbfixture.Everything.Schema}.Map()
}

// TestDatabendVersion reads the banner and checks what the model makes of it.
func TestDatabendVersion(t *testing.T) {
	db := openDatabend(t)
	versions, err := dbmeta.Databend.Version(t.Context(), db)
	if err != nil {
		t.Fatalf("reading the version: %v", err)
	}
	if main := versions.Main(); main.Unknown || main.Compare(dbmeta.V(1, 2)) < 0 {
		t.Errorf("expected release 1.2 or newer in %s", versions)
	}
	if !strings.HasPrefix(versions.String(), "Databend ") {
		t.Errorf("expected the product in %q", versions.String())
	}
	t.Logf("server reports %s", versions)
}

// TestDatabendSmoke runs each query and checks the columns match the fields.
func TestDatabendSmoke(t *testing.T) {
	db := openDatabend(t)
	m := setupDatabend(t, db)
	var ran int
	for _, q := range dbmeta.Queries() {
		if q.Support(m) != dbmeta.Supported {
			continue
		}
		query, vals, err := q.Build(m, nil)
		if err != nil {
			t.Errorf("%s: building: %v", q.Name(), err)
			continue
		}
		cs, err := columnsOf(t, db, query, vals)
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
			t.Errorf("%s: declares %d fields and returns %d columns", q.Name(), len(fields), len(cs))
		}
		ran++
	}
	t.Logf("%d queries ran", ran)
}

// TestDatabendScansEveryQuery reads every query through its own Scan.
func TestDatabendScansEveryQuery(t *testing.T) {
	db := openDatabend(t)
	scanEveryQuery(t, setupDatabend(t, db), db)
}

// TestDatabendFixtureObjects reads the fixture back through the typed API and
// checks the values that are Databend's own.
func TestDatabendFixtureObjects(t *testing.T) {
	db := openDatabend(t)
	ctx := t.Context()
	m := setupDatabend(t, db)

	types := map[string]string{}
	comments := map[string]string{}
	for v, err := range dbmeta.Tables.All(ctx, m, db, dbArgs()) {
		if err != nil {
			t.Fatalf("reading tables: %v", err)
		}
		types[v.Name] = v.Type
		comments[v.Name] = v.Comment.V
	}
	for name, want := range map[string]string{"author": "table", "book": "table", "recent": "view"} {
		if types[name] != want {
			t.Errorf("%s: expected %s, got %q", name, want, types[name])
		}
	}
	if comments["author"] != "people who write" {
		t.Errorf("author: expected its comment, got %q", comments["author"])
	}

	// The ordinal is counted in the order the columns were declared, because
	// information_schema reports 1 for every one.
	want := []string{"author_id", "name", "rating", "shade"}
	for v, err := range dbmeta.Columns.All(ctx, m, db, dbmeta.Args{Schema: dbfixture.Everything.Schema, Parent: "author"}.Map()) {
		if err != nil {
			t.Fatalf("reading columns: %v", err)
		}
		if v.Ordinal < 1 || v.Ordinal > len(want) || want[v.Ordinal-1] != v.Name {
			t.Errorf("%s: unexpected ordinal %d", v.Name, v.Ordinal)
		}
		switch v.Name {
		case "author_id":
			if v.Nullable || v.Comment.V != "surrogate key" {
				t.Errorf("author_id: expected not nullable with its comment, got %+v", v)
			}
		case "shade":
			if v.Default.V != "'red'" {
				t.Errorf("shade: expected the default 'red', got %q", v.Default.V)
			}
		}
	}

	var indexes []string
	for v, err := range dbmeta.Indexes.All(ctx, m, db, dbArgs()) {
		if err != nil {
			t.Fatalf("reading indexes: %v", err)
		}
		indexes = append(indexes, v.Name+" "+v.Type+" "+v.Table)
	}
	if strings.Join(indexes, ", ") != "book_body inverted book, book_title ngram book" {
		t.Errorf("expected both indexes on book, got %v", indexes)
	}

	var keyed []string
	for v, err := range dbmeta.IndexColumns.All(ctx, m, db, dbArgs()) {
		if err != nil {
			t.Fatalf("reading index columns: %v", err)
		}
		keyed = append(keyed, fmt.Sprintf("%s.%s:%d", v.Index, v.Name.V, v.Ordinal))
	}
	if strings.Join(keyed, ", ") != "book_body.body:1, book_title.title:1" {
		t.Errorf("expected the column of each index, got %v", keyed)
	}
	col, ok, err := dbmeta.First(dbmeta.ConstraintColumns.All(ctx, m, db, dbArgs()))
	if err != nil || !ok || col.Constraint != "title_not_empty" || col.Name != "title" || col.Ordinal != 1 {
		t.Errorf("expected title behind the check, got %+v, %v", col, err)
	}

	check, ok, err := dbmeta.First(dbmeta.Constraints.All(ctx, m, db, dbArgs()))
	if err != nil || !ok || check.Name != "title_not_empty" || check.Type != "check" || !strings.Contains(check.Definition.V, "title") {
		t.Errorf("expected the check on title, got %+v, %v", check, err)
	}

	var shout, addup bool
	for v, err := range dbmeta.Functions.All(ctx, m, db, dbmeta.Args{Name: "dbmeta_fixture%"}.Map()) {
		if err != nil {
			t.Fatalf("reading functions: %v", err)
		}
		shout = shout || v.Name == "dbmeta_fixture_shout" && v.Kind == "func" && v.Language == "sql"
		addup = addup || v.Name == "dbmeta_fixture_addup" && v.Kind == "proc" && v.ID.Valid
	}
	if !shout || !addup {
		t.Errorf("expected the function and the procedure, got shout=%v addup=%v", shout, addup)
	}

	seq, ok, err := dbmeta.First(dbmeta.Sequences.All(ctx, m, db, dbmeta.Args{Name: "dbmeta_fixture%"}.Map()))
	if err != nil || !ok || seq.Name != "dbmeta_fixture_counter" || seq.Increment.V != "1" {
		t.Errorf("expected the sequence counting by 1, got %+v, %v", seq, err)
	}

	grants := map[string]bool{}
	for v, err := range dbmeta.RoleGrants.All(ctx, m, db, dbmeta.Args{Name: "dbmeta_fixture_reader"}.Map()) {
		if err != nil {
			t.Fatalf("reading role grants: %v", err)
		}
		grants[v.Role] = true
	}
	if !grants["dbmeta_fixture_writer"] || !grants["dbmeta_fixture_member"] {
		t.Errorf("expected the reader granted to a role and a user, got %v", grants)
	}

	var rating bool
	for v, err := range dbmeta.ColumnStats.All(ctx, m, db, dbmeta.Args{Schema: dbfixture.Everything.Schema, Parent: "author", Name: "rating"}.Map()) {
		if err != nil {
			t.Fatalf("reading column statistics: %v", err)
		}
		rating = v.NullFrac.Valid && v.NullFrac.V > 0.3 && v.NullFrac.V < 0.4 && v.Distinct.V == 2
	}
	if !rating {
		t.Error("expected the statistics of rating: one in three NULL, and two values")
	}
}

// TestDatabendAnswersNoneOfThese checks that what Databend does not have is
// refused rather than answered with no rows (D34).
func TestDatabendAnswersNoneOfThese(t *testing.T) {
	db := openDatabend(t)
	versions, err := dbmeta.Databend.Version(t.Context(), db)
	if err != nil {
		t.Fatalf("reading the version: %v", err)
	}
	m, err := dbmeta.New(dbmeta.Databend, versions)
	if err != nil {
		t.Fatalf("building the metadata: %v", err)
	}
	for _, q := range []dbmeta.AnyQuery{
		dbmeta.Triggers, dbmeta.Types,
		dbmeta.PartitionedTables, dbmeta.Privileges, dbmeta.Collations, dbmeta.Tablespaces,
		dbmeta.ForeignServers, dbmeta.Extensions, dbmeta.RoutineParameters,
	} {
		if s := q.Support(m); s != dbmeta.NotSupported {
			t.Errorf("%s: expected %v, got %v", q.Name(), dbmeta.NotSupported, s)
		}
	}
}

// TestDatabendDescribeFields reads the fields that D198 and D199 added, for
// the ones Databend has a source for (D208). It keeps no row policy in a table
// that a statement can filter, and no storage choice for a column.
func TestDatabendDescribeFields(t *testing.T) {
	db := openDatabend(t)
	ctx := t.Context()
	m := setupDatabend(t, db)

	tables := map[string]dbmeta.Table{}
	for v, err := range dbmeta.Tables.All(ctx, m, db, dbArgs()) {
		if err != nil {
			t.Fatalf("reading tables: %v", err)
		}
		tables[v.Name] = v
		if v.RowSecurity.Valid {
			t.Errorf("%s: expected no row security, got %+v", v.Name, v)
		}
	}
	author := tables["author"]
	if !author.Owner.Valid || author.Owner.V == "" || author.Persistence.V != "permanent" || author.AccessMethod.V != "FUSE" {
		t.Errorf("author: expected an owner, permanent and FUSE, got %+v", author)
	}
	if !author.Rows.Valid || author.Rows.V != 3 || !author.Size.Valid || author.Size.V <= 0 || author.Options.Valid {
		t.Errorf("author: expected 3 rows, a size and no options, got %+v", author)
	}
	if got := tables["book"].Options.V; got != "cluster_by=(published)" {
		t.Errorf("book: expected the cluster key, got %q", got)
	}
	if got := tables["ledger"].Persistence.V; got != "transient" {
		t.Errorf("ledger: expected transient, got %q", got)
	}
	if recent := tables["recent"]; recent.Owner.Valid || recent.Persistence.Valid || recent.Size.Valid || recent.Rows.Valid || recent.Options.Valid {
		t.Errorf("recent: expected a view to fill only the engine, got %+v", recent)
	}

	var shout int
	for v, err := range dbmeta.Functions.All(ctx, m, db, dbmeta.Args{}.Map()) {
		if err != nil {
			t.Fatalf("reading functions: %v", err)
		}
		if v.Name == "dbmeta_fixture_shout" {
			shout++
			if !v.Prosrc.Valid || v.Prosrc.V != v.Source.V {
				t.Errorf("shout: expected prosrc to be the definition, got %+v", v)
			}
		}
	}
	if shout != 1 {
		t.Errorf("expected the function dbmeta_fixture_shout once, got %d", shout)
	}
}
