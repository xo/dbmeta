package test

import (
	"context"
	"database/sql"
	"os"
	"slices"
	"strings"
	"testing"

	_ "github.com/beltran/gohive/v2"

	"github.com/xo/dbmeta"
	_ "github.com/xo/dbmeta/models/hive"
	hvfixture "github.com/xo/dbmeta/models/hive/fixture"
)

// setupHive builds the fixture and returns the metadata for the server.
func setupHive(t *testing.T, db *sql.DB) *dbmeta.Meta {
	t.Helper()
	ctx := t.Context()
	versions, err := dbmeta.Hive.Version(ctx, db)
	if err != nil {
		t.Fatalf("reading the version: %v", err)
	}
	down, err := hvfixture.Everything.ResolveTeardown(versions)
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

	up, err := hvfixture.Everything.ResolveSetup(versions)
	if err != nil {
		t.Fatalf("resolving the setup: %v", err)
	}
	var ran int
	for _, step := range up {
		if step.Skipped {
			continue
		}
		if _, err := db.ExecContext(ctx, step.Query); err != nil {
			t.Fatalf("setup %s: %v\n%s", step.Name, err, step.Query)
		}
		ran++
	}
	t.Cleanup(func() { drop(context.WithoutCancel(ctx)) })

	m, err := dbmeta.New(dbmeta.Hive, versions)
	if err != nil {
		t.Fatalf("building the metadata: %v", err)
	}
	t.Logf("server reports %s, fixture ran %d steps", versions, ran)
	return m
}

// openHive returns a connection to the server named by DBMETA_HIVE.
func openHive(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("DBMETA_HIVE")
	if dsn == "" {
		t.Skip("set DBMETA_HIVE to run against a real server")
	}
	db, err := sql.Open("hive", dsn)
	if err != nil {
		t.Fatalf("opening: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.PingContext(t.Context()); err != nil {
		t.Fatalf("connecting: %v", err)
	}
	return db
}

func hiveMeta(t *testing.T, db *sql.DB) *dbmeta.Meta {
	t.Helper()
	versions, err := dbmeta.Hive.Version(t.Context(), db)
	if err != nil {
		t.Fatalf("reading the version: %v", err)
	}
	m, err := dbmeta.New(dbmeta.Hive, versions)
	if err != nil {
		t.Fatalf("building the metadata: %v", err)
	}
	return m
}

// TestHiveEveryQueryRuns executes every query Hive answers and checks the
// columns match the declared fields.
func TestHiveEveryQueryRuns(t *testing.T) {
	db := openHive(t)
	m := hiveMeta(t, db)
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
		// Hive cannot bind, so a built statement carries its values and
		// hands back none. A value here would be sent to a driver that
		// discards it.
		if len(vals) != 0 {
			t.Errorf("%s: returned %d values and Hive cannot bind one", q.Name(), len(vals))
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
		ran++
	}
	t.Logf("%d queries ran, %d not supported by Hive", ran, unsupported)
	if ran == 0 {
		t.Fatal("expected at least one query to run")
	}
}

// TestHiveEscapingHoldsOnTheServer is the test that matters for D78.
//
// Hive cannot bind, so the model writes filter values into the statement and
// the escaping is the whole of the protection. A unit test can show the
// string looks right. Only the server can show it means what it should.
//
// The value here closes the literal and adds a disjunction that would match
// every row. Escaped, it is an ordinary string that matches no table. If the
// escaping ever breaks, this returns every table rather than none, so the
// assertion is on the count and not on an error.
func TestHiveEscapingHoldsOnTheServer(t *testing.T) {
	db := openHive(t)
	m := hiveMeta(t, db)
	ctx := t.Context()

	var all int
	for _, err := range dbmeta.Tables.All(ctx, m, db, dbmeta.Args{WithSystem: true}.Map()) {
		if err != nil {
			t.Fatalf("reading every table: %v", err)
		}
		all++
	}
	if all == 0 {
		t.Fatal("expected the sys tables, so that the comparison below means something")
	}

	for _, hostile := range []string{
		`x' OR '1'='1`,
		`%' OR tbl_name LIKE '%`,
		`x' --`,
		`x'; DROP TABLE sys.TBLS; --`,
	} {
		var n int
		for _, err := range dbmeta.Tables.All(ctx, m, db,
			dbmeta.Args{Name: hostile, WithSystem: true}.Map()) {
			if err != nil {
				t.Errorf("%q: %v", hostile, err)
				n = -1
				break
			}
			n++
		}
		switch {
		case n < 0:
		case n == 0:
			t.Logf("%-32q matched nothing, as an ordinary string should", hostile)
		default:
			t.Errorf("%q matched %d of %d tables. The value was not escaped and"+
				" changed what the statement means.", hostile, n, all)
		}
	}

	// And the catalog is still there, which the third value would have
	// removed if a statement separator got through.
	var after int
	for _, err := range dbmeta.Tables.All(ctx, m, db, dbmeta.Args{WithSystem: true}.Map()) {
		if err != nil {
			t.Fatalf("reading every table afterwards: %v", err)
		}
		after++
	}
	if after != all {
		t.Errorf("the table count went from %d to %d, so something executed", all, after)
	}
}

// TestHiveFixtureBuilds proves every statement the fixture makes is accepted.
func TestHiveFixtureBuilds(t *testing.T) {
	db := openHive(t)
	m := setupHive(t, db)
	args := dbmeta.Args{Schema: hvfixture.Everything.Schema}.Map()
	var n int
	for v, err := range dbmeta.Tables.All(t.Context(), m, db, args) {
		if err != nil {
			t.Fatalf("reading tables: %v", err)
		}
		t.Logf("  %-18s %s", v.Type, v.Name)
		n++
	}
	if n == 0 {
		t.Fatal("expected the fixture's tables")
	}
}

// hvArgs is the filter the fixture's objects sit behind.
func hvArgs() map[string]any {
	return dbmeta.Args{Schema: hvfixture.Everything.Schema}.Map()
}

// TestHiveVersion reads the version and checks what the model makes of it.
//
// Hive reports a release and the commit it was built from, separated by a
// space, so the model takes the part before the space.
func TestHiveVersion(t *testing.T) {
	db := openHive(t)
	versions, err := dbmeta.Hive.Version(t.Context(), db)
	if err != nil {
		t.Fatalf("reading the version: %v", err)
	}
	main := versions.Main()
	if main.Unknown || len(main.Parts) < 2 {
		t.Fatalf("expected a parsed version, got %v", versions)
	}
	if main.Parts[0] < 4 {
		t.Errorf("expected 4.0 or newer, got %v", main)
	}
	if !strings.HasPrefix(versions.Display, "Apache Hive ") {
		t.Errorf("expected the display to name the product, got %q", versions.Display)
	}
}

// TestHiveColumnsReadConstraints checks the thing Hive does differently from
// everything else here: nullability and a default are constraints rather
// than properties of the column, so Columns reads them out of
// KEY_CONSTRAINTS.
func TestHiveColumnsReadConstraints(t *testing.T) {
	db := openHive(t)
	m := setupHive(t, db)

	type got struct {
		nullable   bool
		hasDefault bool
		primaryKey bool
	}
	seen := map[string]got{}
	args := dbmeta.Args{Schema: hvfixture.Everything.Schema, Parent: "ticket"}.Map()
	for v, err := range dbmeta.Columns.All(t.Context(), m, db, args) {
		if err != nil {
			t.Fatalf("reading columns: %v", err)
		}
		seen[strings.ToLower(v.Name)] = got{v.Nullable, v.Default.Valid, v.PrimaryKey}
	}
	if g, ok := seen["ticket_id"]; !ok {
		t.Error("expected ticket_id")
	} else if g.nullable {
		t.Error("expected ticket_id to be NOT NULL, which Hive records as a constraint")
	}
	if g, ok := seen["note"]; !ok {
		t.Error("expected note")
	} else if !g.hasDefault {
		t.Error("expected note to have a default, which Hive records as a constraint")
	}

	// And the primary key, which is read the same way.
	var key bool
	args = dbmeta.Args{Schema: hvfixture.Everything.Schema, Parent: "author"}.Map()
	for v, err := range dbmeta.Columns.All(t.Context(), m, db, args) {
		if err != nil {
			t.Fatalf("reading columns: %v", err)
		}
		if strings.EqualFold(v.Name, "author_id") && v.PrimaryKey {
			key = true
		}
	}
	if !key {
		t.Error("expected author_id to be the primary key")
	}
}

// TestHiveForeignKeyTarget checks the composite foreign key, which Hive
// records by column position rather than by name on both sides.
func TestHiveForeignKeyTarget(t *testing.T) {
	db := openHive(t)
	m := setupHive(t, db)

	var pairs []string
	args := dbmeta.Args{Schema: hvfixture.Everything.Schema, Parent: "shipment"}.Map()
	for v, err := range dbmeta.ConstraintColumns.All(t.Context(), m, db, args) {
		if err != nil {
			t.Fatalf("reading constraint columns: %v", err)
		}
		if !v.ForeignTable.Valid {
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

// TestHivePartitionColumnIsNotAColumn asserts a difference that would
// otherwise look like a missing column.
//
// A Hive partition key is not in the table's column list. It is in
// PARTITION_KEYS, so PartitionedTables reports it and Columns does not, and
// a caller that reads only Columns sees a table without its partition
// column. Without this test that absence is indistinguishable from a bug.
func TestHivePartitionColumnIsNotAColumn(t *testing.T) {
	db := openHive(t)
	m := setupHive(t, db)
	ctx := t.Context()

	args := dbmeta.Args{Schema: hvfixture.Everything.Schema, Parent: "archive"}.Map()
	for v, err := range dbmeta.Columns.All(ctx, m, db, args) {
		if err != nil {
			t.Fatalf("reading columns: %v", err)
		}
		if strings.EqualFold(v.Name, "year") {
			t.Error("the partition key appeared in Columns. Hive keeps it in" +
				" PARTITION_KEYS and PartitionedTables is what reports it.")
		}
	}
	var found bool
	for v, err := range dbmeta.PartitionedTables.All(ctx, m, db, hvArgs()) {
		if err != nil {
			t.Fatalf("reading partitioned tables: %v", err)
		}
		if strings.EqualFold(v.Name, "archive") && strings.EqualFold(v.Expression, "year") {
			found = true
		}
	}
	if !found {
		t.Error("expected archive to be partitioned by year")
	}
}

// TestHiveAccessMethods checks the one analogy this model draws, that a
// SerDe is how Hive reads and writes a table.
func TestHiveAccessMethods(t *testing.T) {
	db := openHive(t)
	m := setupHive(t, db)
	var n int
	for v, err := range dbmeta.AccessMethods.All(t.Context(), m, db, nil) {
		if err != nil {
			t.Fatalf("reading access methods: %v", err)
		}
		if !strings.Contains(v.Name, ".") {
			t.Errorf("expected a SerDe class name, got %q", v.Name)
		}
		n++
	}
	if n < 2 {
		t.Errorf("expected more than one SerDe, since the fixture stores one"+
			" table as ORC and the rest by default, got %d", n)
	}
}

// TestHiveTellsNullFromEmpty is the guard on the driver property that
// docs/NULLS.md exists for.
//
// sqlflow.org/gohive, which usql shipped before this, could not represent a
// NULL at all: CAST(NULL AS string) and ” both arrived as a valid empty
// string, and a NULL bigint arrived as a valid zero. Every nullable field in
// this model would have been a lie, silently, and the model looked correct
// until this was measured.
//
// beltran/gohive/v2 distinguishes them. This asserts that it still does,
// because a driver change that regressed it would leave no other trace.
func TestHiveTellsNullFromEmpty(t *testing.T) {
	db := openHive(t)
	ctx := t.Context()

	var s sql.Null[string]
	if err := db.QueryRowContext(ctx, "SELECT CAST(NULL AS string) AS `v`").Scan(&s); err != nil {
		t.Fatalf("reading a null string: %v", err)
	}
	if s.Valid {
		t.Error("a NULL string arrived valid. The driver cannot tell NULL from" +
			" empty, so every nullable field in this model is wrong.")
	}
	if err := db.QueryRowContext(ctx, "SELECT '' AS `v`").Scan(&s); err != nil {
		t.Fatalf("reading an empty string: %v", err)
	}
	if !s.Valid {
		t.Error("an empty string arrived NULL, which is the same fault the other way")
	}
	var i sql.Null[int64]
	if err := db.QueryRowContext(ctx, "SELECT CAST(NULL AS bigint) AS `v`").Scan(&i); err != nil {
		t.Fatalf("reading a null bigint: %v", err)
	}
	if i.Valid {
		t.Error("a NULL bigint arrived valid")
	}

	// And through the model, where it matters: a table with no comment
	// must report absent rather than empty.
	m := setupHive(t, db)
	var checked bool
	for v, err := range dbmeta.Tables.All(ctx, m, db, hvArgs()) {
		if err != nil {
			t.Fatalf("reading tables: %v", err)
		}
		switch strings.ToLower(v.Name) {
		case "author":
			if !v.Comment.Valid || v.Comment.V != "people who write" {
				t.Errorf("expected the author comment, got %#v", v.Comment)
			}
			checked = true
		case "book":
			if v.Comment.Valid {
				t.Errorf("expected book to have no comment, got %q", v.Comment.V)
			}
		}
	}
	if !checked {
		t.Error("expected the author table")
	}
}
