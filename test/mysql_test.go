package test

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"slices"
	"strings"
	"testing"

	_ "github.com/go-sql-driver/mysql"

	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/models/mysql"
	myfixture "github.com/xo/dbmeta/models/mysql/fixture"
)

// openMySQL returns a connection to the server named by DBMETA_MYSQL, or skips.
func openMySQL(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("DBMETA_MYSQL")
	if dsn == "" {
		t.Skip("set DBMETA_MYSQL to run against a real server")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("opening: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.PingContext(t.Context()); err != nil {
		t.Fatalf("connecting: %v", err)
	}
	return db
}

func setupMySQL(t *testing.T, db *sql.DB) *dbmeta.Meta {
	t.Helper()
	versions, err := dbmeta.MySQL.Version(t.Context(), db)
	if err != nil {
		t.Fatalf("reading the version: %v", err)
	}
	m, err := dbmeta.New(dbmeta.MySQL, versions)
	if err != nil {
		t.Fatalf("building the meta: %v", err)
	}
	run := func(ctx context.Context, steps []myfixture.Result, fatal bool) {
		for _, s := range steps {
			if s.Skipped {
				continue
			}
			if _, err := db.ExecContext(ctx, s.SQL); err != nil && fatal {
				t.Fatalf("%s: %v\n%s", s.Name, err, s.SQL)
			}
		}
	}
	down, err := myfixture.Everything.ResolveTeardown(versions)
	if err != nil {
		t.Fatalf("resolving the teardown: %v", err)
	}
	up, err := myfixture.Everything.ResolveSetup(versions)
	if err != nil {
		t.Fatalf("resolving the setup: %v", err)
	}
	run(t.Context(), down, false)
	run(t.Context(), up, true)
	t.Cleanup(func() { run(context.Background(), down, false) })

	var skipped int
	for _, s := range up {
		if s.Skipped {
			skipped++
		}
	}
	t.Logf("server reports %s, fixture ran %d steps and skipped %d", m, len(up)-skipped, skipped)
	return m
}

func myArgs() map[string]any {
	return dbmeta.Args{Schema: myfixture.Everything.Schema}.Map()
}

// TestMySQLEveryQueryRuns executes every query MariaDB answers and checks the
// columns match the declared fields.
func TestMySQLEveryQueryRuns(t *testing.T) {
	db := openMySQL(t)
	m := setupMySQL(t, db)

	var ran, tooOld, unsupported int
	for _, q := range dbmeta.Queries() {
		switch q.Support(m) {
		case dbmeta.NotSupported, dbmeta.NotBuilt:
			unsupported++
			continue
		}
		sqlstr, vals, err := q.SQL(m, nil)
		switch {
		case errors.Is(err, dbmeta.ErrVersionTooOld):
			tooOld++
			continue
		case err != nil:
			t.Errorf("%s: rendering: %v", q.Name(), err)
			continue
		}
		cols, err := columnsOf(t, db, sqlstr, vals)
		if err != nil {
			t.Errorf("%s: executing: %v\n%s", q.Name(), err, sqlstr)
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
	}
	t.Logf("%d queries ran, %d too old, %d not supported by MariaDB", ran, tooOld, unsupported)
	if ran == 0 {
		t.Fatal("expected at least one query to run")
	}
}

// TestMySQLScanning reads rows through the typed API.
func TestMySQLScanning(t *testing.T) {
	db := openMySQL(t)
	m := setupMySQL(t, db)
	ctx := t.Context()

	want := map[string]bool{"author": false, "book": false, "recent": false, "sales": false}
	var withComment, withoutComment int
	for v, err := range dbmeta.Tables.All(ctx, m, db, myArgs()) {
		if err != nil {
			t.Fatalf("reading tables: %v", err)
		}
		if _, ok := want[v.Name]; ok {
			want[v.Name] = true
		}
		if v.Comment.Valid {
			withComment++
		} else {
			withoutComment++
		}
	}
	for name, found := range want {
		if !found {
			t.Errorf("expected to find %q", name)
		}
	}
	// the fixture comments the author table and nothing else, and MariaDB
	// writes an empty string rather than NULL for no comment, so the query
	// turns it back into an absent value
	if withComment == 0 || withoutComment == 0 {
		t.Errorf("expected both a commented and an uncommented relation, got %d and %d",
			withComment, withoutComment)
	}

	var cols int
	a := dbmeta.Args{Schema: myfixture.Everything.Schema, Parent: "author"}.Map()
	for v, err := range dbmeta.Columns.All(ctx, m, db, a) {
		if err != nil {
			t.Fatalf("reading columns: %v", err)
		}
		cols++
		if v.Name == "author_id" && !v.Identity.Valid {
			t.Error("expected auto_increment to report as an identity column")
		}
	}
	if cols != 4 {
		t.Errorf("expected 4 columns on author, got %d", cols)
	}
}

// TestMySQLAnalogues checks the object kinds MariaDB answers with something of
// its own rather than with the thing psql names.
func TestMySQLAnalogues(t *testing.T) {
	db := openMySQL(t)
	m := setupMySQL(t, db)
	ctx := t.Context()

	var engines int
	for v, err := range dbmeta.AccessMethods.All(ctx, m, db, nil) {
		if err != nil {
			t.Fatalf("reading access methods: %v", err)
		}
		engines++
		if v.Type != "table" {
			t.Errorf("expected a storage engine to report as a table method, got %q", v.Type)
		}
	}
	if engines == 0 {
		t.Error("expected MariaDB storage engines to answer for access methods")
	}

	var plugins int
	for _, err := range dbmeta.Extensions.All(ctx, m, db, nil) {
		if err != nil {
			t.Fatalf("reading extensions: %v", err)
		}
		plugins++
	}
	if plugins == 0 {
		t.Error("expected MariaDB plugins to answer for extensions")
	}
}

// TestMySQLAggregates covers \da, which only MariaDB answers. MySQL dropped
// mysql.proc in 8.0 and has no aggregate of its own at any release, so it must
// report the query unsupported rather than returning nothing.
func TestMySQLAggregates(t *testing.T) {
	db := openMySQL(t)
	m := setupMySQL(t, db)
	ctx := t.Context()
	schema := myfixture.Everything.Schema

	if !mysql.IsMariaDB(m.Version()) {
		if got := dbmeta.Aggregates.Support(m); got != dbmeta.NotSupported {
			t.Errorf("expected MySQL to report aggregates unsupported, got %v", got)
		}
		if _, _, err := dbmeta.Aggregates.SQL(m, nil); !errors.Is(err, dbmeta.ErrNotSupported) {
			t.Errorf("expected ErrNotSupported, got: %v", err)
		}
		return
	}
	var aggs int
	for v, err := range dbmeta.Aggregates.All(ctx, m, db, myArgs()) {
		if err != nil {
			t.Fatalf("reading aggregates: %v", err)
		}
		aggs++
		if v.Kind != "agg" {
			t.Errorf("expected kind agg, got %q", v.Kind)
		}
		if v.Name == "total" && v.Schema != schema {
			t.Errorf("expected %q, got %q", schema, v.Schema)
		}
		// a plain function must not appear here, which is the whole reason
		// this query reads mysql.proc rather than information_schema.ROUTINES
		if v.Name == "shout" {
			t.Error("expected a plain function to be left out of the aggregates")
		}
	}
	if m.Version().Main().AtLeast(dbmeta.V(10, 3)) && aggs == 0 {
		t.Error("expected the fixture aggregate to be listed")
	}
}

// TestMySQLForeignData covers the kinds that only a second look found. They
// read mysql.servers, which information_schema does not publish, and both
// products keep it.
func TestMySQLForeignData(t *testing.T) {
	db := openMySQL(t)
	m := setupMySQL(t, db)
	ctx := t.Context()

	var servers int
	for v, err := range dbmeta.ForeignServers.All(ctx, m, db, nil) {
		if err != nil {
			t.Fatalf("reading foreign servers: %v", err)
		}
		servers++
		if v.Name == "dbmeta_fixture_remote" {
			if v.Wrapper != "mysql" {
				t.Errorf("expected the mysql wrapper, got %q", v.Wrapper)
			}
			if !v.Options.Valid || !strings.Contains(v.Options.V, "host 127.0.0.1") {
				t.Errorf("expected the host in the options, got %v", v.Options)
			}
		}
	}
	if servers == 0 {
		t.Error("expected the fixture server to be listed")
	}

	var mapped bool
	for v, err := range dbmeta.UserMappings.All(ctx, m, db, nil) {
		if err != nil {
			t.Fatalf("reading user mappings: %v", err)
		}
		if v.Server == "dbmeta_fixture_remote" {
			mapped = true
			if !v.Name.Valid || v.Name.V != "nobody" {
				t.Errorf("expected the remote user nobody, got %v", v.Name)
			}
		}
	}
	if !mapped {
		t.Error("expected a mapping for the fixture server")
	}

	// The fixture creates no foreign table, because every engine that reads
	// remote data needs a reachable remote and CI has none. The query still
	// has to run and return the right columns, and the local tables must not
	// leak into it.
	for v, err := range dbmeta.ForeignTables.All(ctx, m, db, myArgs()) {
		if err != nil {
			t.Fatalf("reading foreign tables: %v", err)
		}
		if v.Name == "author" || v.Name == "book" {
			t.Errorf("expected a local table to be left out, got %q", v.Name)
		}
	}
}

// TestMySQLUnsupported checks that what MariaDB does not have says so, rather
// than answering with an empty result. D34 requires that difference.
func TestMySQLUnsupported(t *testing.T) {
	db := openMySQL(t)
	m := setupMySQL(t, db)
	for _, q := range []dbmeta.AnyQuery{
		dbmeta.Types, dbmeta.Domains, dbmeta.Operators, dbmeta.Casts,
		dbmeta.Publications, dbmeta.Subscriptions, dbmeta.TextSearchConfigs,
		dbmeta.OperatorClasses, dbmeta.LargeObjects, dbmeta.DefaultACLs,
		// Both Gemini and DeepSeek named an analogue for each of these three.
		// Each one was run against a server and rejected. See docs/COVERAGE.md.
		dbmeta.Tablespaces, dbmeta.ForeignDataWrappers, dbmeta.ExtendedStats,
	} {
		checkUnsupported(t, m, q)
	}
	// Sequences and aggregates are the other kind of unsupported. The dialect
	// answers them and this product does not, at any release, so the answer
	// must be ErrNotSupported and never ErrVersionTooOld. Comparing the
	// numbers alone got this wrong: MySQL 9 is below MariaDB 11.5, which made
	// a product difference look like an old server. See D44.
	if !mysql.IsMariaDB(m.Version()) {
		for _, q := range []dbmeta.AnyQuery{dbmeta.Sequences, dbmeta.Aggregates} {
			checkUnsupported(t, m, q)
		}
	}
}

func checkUnsupported(t *testing.T, m *dbmeta.Meta, q dbmeta.AnyQuery) {
	t.Helper()
	{
		if got := q.Support(m); got != dbmeta.NotSupported {
			t.Errorf("%s: expected it to be reported unsupported, got %v", q.Name(), got)
		}
		if _, _, err := q.SQL(m, nil); !errors.Is(err, dbmeta.ErrNotSupported) {
			t.Errorf("%s: expected ErrNotSupported, got: %v", q.Name(), err)
		}
	}
}

// TestMySQLNewKinds covers the kinds added under D47. Most of them come from
// standard information_schema views that both products have, which is why the
// counts rise for both.
func TestMySQLNewKinds(t *testing.T) {
	db := openMySQL(t)
	m := setupMySQL(t, db)
	ctx := t.Context()
	maria := mysql.IsMariaDB(m.Version())

	// A composite foreign key is what proves the ordinals line up, because
	// key_column_usage carries the referenced side in the same row.
	cols := make(map[string][]string)
	refs := make(map[string]string)
	for v, err := range dbmeta.ConstraintColumns.All(ctx, m, db, myArgs()) {
		if err != nil {
			t.Fatalf("reading constraint columns: %v", err)
		}
		key := v.Table + "." + v.Constraint
		if int64(len(cols[key]))+1 != v.Ordinal {
			t.Errorf("%s: expected ordinal %d, got %d", key, len(cols[key])+1, v.Ordinal)
		}
		cols[key] = append(cols[key], v.Name)
		if v.ForeignTable.Valid {
			refs[key] += v.ForeignTable.V + "." + v.ForeignName.V + " "
		}
	}
	if want := []string{"country", "area"}; !slices.Equal(cols["shipment.shipment_region_fk"], want) {
		t.Errorf("expected %v, got %v", want, cols["shipment.shipment_region_fk"])
	}
	if got := refs["shipment.shipment_region_fk"]; got != "region.country region.area " {
		t.Errorf("expected both referenced columns in order, got %q", got)
	}
	if want := []string{"country", "area"}; !slices.Equal(cols["region.PRIMARY"], want) {
		t.Errorf("expected the composite primary key, got %v", cols["region.PRIMARY"])
	}

	// Every mode the products report. A procedure has IN and OUT parameters
	// and a function records its return value at ordinal zero.
	modes := make(map[string]string)
	for v, err := range dbmeta.RoutineParameters.All(ctx, m, db, myArgs()) {
		if err != nil {
			t.Fatalf("reading routine parameters: %v", err)
		}
		modes[v.Routine+"."+v.Name.V] = v.Mode
		if v.Ordinal == 0 && v.Mode != "return" {
			t.Errorf("%s: expected ordinal zero to be the return value, got mode %q", v.Routine, v.Mode)
		}
	}
	for key, want := range map[string]string{
		"addup.a": "in", "addup.b": "in", "addup.total": "out",
	} {
		if got := modes[key]; got != want {
			t.Errorf("%s: expected mode %q, got %q", key, want, got)
		}
	}
	// shout is a function, so it has a row with no name for its return value
	if got := modes["shout."]; got != "return" {
		t.Errorf("expected a return row for the function, got %q", got)
	}

	// The view carries the statement it selects.
	v, ok, err := dbmeta.First(dbmeta.Views.All(ctx, m, db, myArgs()))
	if err != nil {
		t.Fatalf("reading views: %v", err)
	}
	if !ok || v.Name != "recent" {
		t.Fatalf("expected the fixture view, got %q ok=%v", v.Name, ok)
	}
	if !strings.Contains(strings.ToLower(v.Definition), "select") {
		t.Errorf("expected a select, got %q", v.Definition)
	}

	// The current schema follows the connection, and the fixture connects
	// without naming a database, so there is no row until one is chosen.
	if _, ok, err := dbmeta.First(dbmeta.CurrentSchema.All(ctx, m, db, nil)); err != nil {
		t.Fatalf("reading the current schema: %v", err)
	} else if ok {
		t.Log("a database was already selected, which is fine")
	}

	// The primary key flag is free here: information_schema.COLUMNS carries it.
	keyed := make(map[string]bool)
	for c, err := range dbmeta.Columns.All(ctx, m, db, myArgs()) {
		if err != nil {
			t.Fatalf("reading columns: %v", err)
		}
		keyed[c.Table+"."+c.Name] = c.PrimaryKey
	}
	for key, want := range map[string]bool{
		"author.author_id": true, "author.name": false,
		"region.country": true, "region.area": true, "shipment.amount": false,
	} {
		if got, ok := keyed[key]; !ok {
			t.Errorf("expected a column %s", key)
		} else if got != want {
			t.Errorf("%s: expected primary_key=%v, got %v", key, want, got)
		}
	}

	// Statistics are MariaDB only, and MySQL must say so rather than answer
	// with nothing. MySQL keeps a JSON histogram and no width, no null
	// fraction and no distinct count. See docs/COVERAGE.md.
	if !maria {
		if got := dbmeta.ColumnStats.Support(m); got != dbmeta.NotSupported {
			t.Errorf("expected MySQL to report column stats unsupported, got %v", got)
		}
		return
	}
	var stats int
	for s, err := range dbmeta.ColumnStats.All(ctx, m, db, myArgs()) {
		if err != nil {
			t.Fatalf("reading column stats: %v", err)
		}
		stats++
		if s.Table == "author" && s.Name == "rating" {
			if !s.Distinct.Valid || s.Distinct.V < 4 || s.Distinct.V > 6 {
				t.Errorf("expected about five distinct ratings, got %v", s.Distinct)
			}
			if !s.Min.Valid || !s.Max.Valid {
				t.Errorf("expected bounds, got %v and %v", s.Min, s.Max)
			}
		}
	}
	if stats == 0 {
		t.Error("expected statistics after the fixture analyzed the table")
	}
}

// TestMySQLNullDefault pins what each product says is the default of a
// nullable column that was declared without one, because the two products
// disagree and the cross family conformance test maps the difference away.
//
// MariaDB records the four character string NULL. It is saying that the column
// defaults to NULL, which is true, and which nothing else here says. MySQL
// records SQL NULL, meaning there is no default, which is what PostgreSQL,
// SQLite and DuckDB say.
//
// dbmeta reports what the database said, and this is the record of that.
// canonical.go maps both to "no explicit default" for comparison only, and
// canonicalFields names the mapping.
func TestMySQLNullDefault(t *testing.T) {
	db := openMySQL(t)
	m := setupMySQL(t, db)

	defaults := make(map[string]sql.Null[string])
	for v, err := range dbmeta.Columns.All(t.Context(), m, db, myArgs()) {
		if err != nil {
			t.Fatalf("reading columns: %v", err)
		}
		defaults[v.Table+"."+v.Name] = v.Default
	}
	// nullable, no default: the two products differ
	got := defaults["author.rating"]
	if mysql.IsMariaDB(m.Version()) {
		if !got.Valid || got.V != "NULL" {
			t.Errorf("expected MariaDB to report the literal NULL, got %v", got)
		}
	} else if got.Valid {
		t.Errorf("expected MySQL to report no default, got %v", got)
	}
	// not null, no default: absent on both
	if got := defaults["author.name"]; got.Valid {
		t.Errorf("expected no default on a NOT NULL column with none, got %v", got)
	}
	// a real default is the value, not the literal NULL, on both
	if got := defaults["author.shade"]; !got.Valid || got.V == "NULL" {
		t.Errorf("expected the declared default, got %v", got)
	}
}
