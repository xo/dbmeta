package test

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"slices"
	"strings"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/xo/dbmeta"
	_ "github.com/xo/dbmeta/models/postgres"
	"github.com/xo/dbmeta/models/postgres/fixture"
)

// open returns a connection to the server named by DBMETA_POSTGRES, or skips.
func open(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("DBMETA_POSTGRES")
	if dsn == "" {
		t.Skip("set DBMETA_POSTGRES to run against a real server")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("opening: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.PingContext(t.Context()); err != nil {
		t.Fatalf("connecting: %v", err)
	}
	return db
}

// setup builds the fixture and returns the meta for the server.
//
// The fixture lives in the model package rather than here, because it changes
// with the queries and because other projects use it. A step the server is too
// old for is skipped, and the query that reads what it would have built is
// refused on the same release.
func setup(t *testing.T, db *sql.DB) *dbmeta.Meta {
	t.Helper()
	versions, err := dbmeta.PostgreSQL.Version(t.Context(), db)
	if err != nil {
		t.Fatalf("reading the version: %v", err)
	}
	m, err := dbmeta.New(dbmeta.PostgreSQL, versions)
	if err != nil {
		t.Fatalf("building the meta: %v", err)
	}

	// the context must be passed in rather than taken from t, because
	// [testing.T.Context] is cancelled just before a cleanup runs, and the
	// teardown is a cleanup
	run := func(ctx context.Context, steps []fixture.Result) {
		for _, s := range steps {
			if s.Skipped {
				continue
			}
			if _, err := db.ExecContext(ctx, s.SQL); err != nil {
				t.Fatalf("%s: %v\n%s", s.Name, err, s.SQL)
			}
		}
	}
	down, err := fixture.Everything.ResolveTeardown(versions)
	if err != nil {
		t.Fatalf("resolving the teardown: %v", err)
	}
	up, err := fixture.Everything.ResolveSetup(versions)
	if err != nil {
		t.Fatalf("resolving the setup: %v", err)
	}
	run(t.Context(), down)
	run(t.Context(), up)
	t.Cleanup(func() { run(context.Background(), down) })

	var skipped int
	for _, s := range up {
		if s.Skipped {
			skipped++
		}
	}
	t.Logf("server reports %s, fixture ran %d steps and skipped %d", m, len(up)-skipped, skipped)
	return m
}

func args() map[string]any {
	return dbmeta.Args{Schema: fixture.Everything.Schema}.Map()
}

// TestEveryQueryRuns executes every query the server is new enough for, and
// checks that the columns it returns are exactly the fields it declares.
func TestEveryQueryRuns(t *testing.T) {
	db := open(t)
	m := setup(t, db)

	var ran, tooOld int
	for _, q := range dbmeta.Queries() {
		if q.Support(m) != dbmeta.Supported {
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
	t.Logf("%d queries ran, %d were refused as too old", ran, tooOld)
	if ran == 0 {
		t.Fatal("expected at least one query to run")
	}
}

// TestPaddedFieldsAreNull is the invariant that replaces a golden file per
// release. Ten releases times 48 queries is 480 combinations, which nobody
// would maintain. [dbmeta.Field.Min] already declares the release each column
// arrived in, so the assertion can be generic: a field the server is too old
// for must be NULL in every row, because the statement padded it.
//
// This is the padding rule of D8 checked against a real server rather than
// against the text of the statement.
func TestPaddedFieldsAreNull(t *testing.T) {
	db := open(t)
	m := setup(t, db)
	server := m.Version().Main()

	var checked int
	for _, q := range dbmeta.Queries() {
		if q.Support(m) != dbmeta.Supported {
			continue
		}
		fields, err := q.Fields(m)
		if err != nil {
			continue
		}
		var padded []int
		for i, f := range fields {
			if !f.Min.IsZero() && !server.AtLeast(f.Min) {
				padded = append(padded, i)
			}
		}
		if len(padded) == 0 {
			continue
		}
		sqlstr, vals, err := q.SQL(m, nil)
		if err != nil {
			continue
		}
		err = eachRawRow(t, db, sqlstr, vals, len(fields), func(raw []sql.RawBytes) {
			for _, i := range padded {
				if raw[i] != nil {
					t.Errorf("%s: %q arrived in %s and the server is %s, so it must be NULL, got %q",
						q.Name(), fields[i].Name, fields[i].Min, server, raw[i])
				}
			}
			checked++
		})
		if err != nil {
			t.Errorf("%s: %v", q.Name(), err)
			continue
		}
	}
	t.Logf("checked %d padded values", checked)
}

// TestScanningWorks reads rows through the typed API, which rendering tests
// cannot check. A Go type that does not match what the driver returns fails
// here and nowhere else.
func TestScanningWorks(t *testing.T) {
	db := open(t)
	m := setup(t, db)
	ctx := t.Context()

	var tables []dbmeta.Table
	for v, err := range dbmeta.Tables.All(ctx, m, db, args()) {
		if err != nil {
			t.Fatalf("reading tables: %v", err)
		}
		tables = append(tables, v)
	}
	if len(tables) == 0 {
		t.Fatal("expected the fixture relations")
	}

	var withComment, withoutComment int
	for _, v := range tables {
		if v.Comment.Valid {
			withComment++
		} else {
			withoutComment++
		}
	}
	if withComment == 0 || withoutComment == 0 {
		t.Errorf("expected both a commented and an uncommented relation, got %d and %d",
			withComment, withoutComment)
	}

	var cols int
	a := dbmeta.Args{Schema: fixture.Everything.Schema, Parent: "author"}.Map()
	for v, err := range dbmeta.Columns.All(ctx, m, db, a) {
		if err != nil {
			t.Fatalf("reading columns: %v", err)
		}
		cols++
		if v.Table != "author" {
			t.Errorf("expected a column of author, got one of %s", v.Table)
		}
	}
	if cols < 4 {
		t.Errorf("expected at least the 4 declared columns on author, got %d", cols)
	}
}

// TestNullAccessDiffersFromEmpty is the regression test for the bug COALESCE
// hid. A relation with default privileges reports a NULL access list. A
// relation with every privilege revoked reports an empty one. Those are
// different answers and the API must keep them apart.
func TestNullAccessDiffersFromEmpty(t *testing.T) {
	db := open(t)
	m := setup(t, db)

	got := map[string]dbmeta.Text{}
	for v, err := range dbmeta.Privileges.All(t.Context(), m, db, args()) {
		if err != nil {
			t.Fatalf("reading privileges: %v", err)
		}
		got[v.Name] = v.Access
	}
	def, ok := got["default_privs"]
	if !ok {
		t.Fatal("expected the default_privs relation")
	}
	rev, ok := got["revoked_privs"]
	if !ok {
		t.Fatal("expected the revoked_privs relation")
	}
	if def.Valid {
		t.Errorf("expected default privileges to report an absent access list, got %q", def.V)
	}
	if !rev.Valid {
		t.Error("expected revoked privileges to report a present, empty access list")
	}
}

// TestConstraintColumnsKeepOrder covers the kind that exists because a rendered
// constraint definition cannot be parsed. A composite foreign key is the case
// that proves it: two rows, in order, each naming the column it points at.
func TestConstraintColumnsKeepOrder(t *testing.T) {
	db := open(t)
	m := setup(t, db)
	ctx := t.Context()

	// Keyed by table and constraint, not by constraint alone. A constraint
	// name is unique within a table and not within a schema, and a partition
	// carries its parent's constraint names.
	type ref struct{ col, ftable, fcol string }
	got := make(map[string][]ref)
	for v, err := range dbmeta.ConstraintColumns.All(ctx, m, db, args()) {
		if err != nil {
			t.Fatalf("reading constraint columns: %v", err)
		}
		key := v.Table + "." + v.Constraint
		if int64(len(got[key]))+1 != v.Ordinal {
			t.Errorf("%s: expected ordinal %d, got %d", key, len(got[key])+1, v.Ordinal)
		}
		got[key] = append(got[key], ref{v.Name, v.ForeignTable.V, v.ForeignName.V})
	}

	// the composite foreign key, in declaration order, each column paired
	// with the one it points at
	want := []ref{{"country", "region", "country"}, {"area", "region", "area"}}
	if fk := got["shipment.shipment_region_fk"]; !slices.Equal(fk, want) {
		t.Errorf("expected %v, got %v", want, fk)
	}
	// the composite primary key, with no foreign side
	pk := got["region.region_pkey"]
	if len(pk) != 2 || pk[0].col != "country" || pk[1].col != "area" {
		t.Errorf("expected the two key columns in order, got %v", pk)
	}
	for _, r := range pk {
		if r.ftable != "" {
			t.Errorf("expected no foreign side on a primary key, got %v", r)
		}
	}
}

// TestRoutineParametersCoverEveryMode covers the other kind that replaces
// rendered text. The fixture declares an input, an input with a default and an
// output parameter, so every mode the model reports has a row.
func TestRoutineParametersCoverEveryMode(t *testing.T) {
	db := open(t)
	m := setup(t, db)
	ctx := t.Context()

	var params []dbmeta.RoutineParameter
	a := dbmeta.Args{Schema: fixture.Everything.Schema, Parent: "addup"}.Map()
	for v, err := range dbmeta.RoutineParameters.All(ctx, m, db, a) {
		if err != nil {
			t.Fatalf("reading routine parameters: %v", err)
		}
		params = append(params, v)
	}
	if len(params) != 3 {
		t.Fatalf("expected three parameters, got %d: %+v", len(params), params)
	}
	for i, want := range []struct {
		name, mode string
		hasDefault bool
	}{
		{"a", "in", false},
		{"b", "in", true},
		{"total", "out", false},
	} {
		got := params[i]
		if got.Name.V != want.name || got.Mode != want.mode {
			t.Errorf("parameter %d: expected %s %s, got %s %s",
				i+1, want.name, want.mode, got.Name.V, got.Mode)
		}
		if got.Default.Valid != want.hasDefault {
			t.Errorf("parameter %d: expected default=%v, got %v", i+1, want.hasDefault, got.Default)
		}
		if got.Ordinal != int64(i+1) {
			t.Errorf("parameter %d: expected ordinal %d, got %d", i+1, i+1, got.Ordinal)
		}
		if got.DataType != "integer" {
			t.Errorf("parameter %d: expected integer, got %q", i+1, got.DataType)
		}
	}
	// The routine id is what a caller groups by, because PostgreSQL overloads
	// a name. It must match what Functions reports for the same routine.
	var funcID string
	for v, err := range dbmeta.Functions.All(ctx, m, db, dbmeta.Args{
		Schema: fixture.Everything.Schema, Name: "addup",
	}.Map()) {
		if err != nil {
			t.Fatalf("reading functions: %v", err)
		}
		funcID = v.ID.V
	}
	if funcID == "" {
		t.Fatal("expected Functions to report an id")
	}
	if params[0].RoutineID.V != funcID {
		t.Errorf("expected the ids to match, got %q and %q", params[0].RoutineID.V, funcID)
	}
}

// TestEnumValuesAreRows covers the kind that exists because a label can
// contain a comma, which makes splitting Type.Elements wrong.
func TestEnumValuesAreRows(t *testing.T) {
	db := open(t)
	m := setup(t, db)
	ctx := t.Context()

	var labels []string
	for v, err := range dbmeta.EnumValues.All(ctx, m, db, args()) {
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
}

// TestViewsCarryTheirDefinition covers the kind dbtpl needs. A materialized
// view is listed with a plain one, which is what psql does.
func TestViewsCarryTheirDefinition(t *testing.T) {
	db := open(t)
	m := setup(t, db)
	ctx := t.Context()

	found := make(map[string]dbmeta.View)
	for v, err := range dbmeta.Views.All(ctx, m, db, args()) {
		if err != nil {
			t.Fatalf("reading views: %v", err)
		}
		found[v.Name] = v
	}
	for _, name := range []string{"recent", "author_count"} {
		v, ok := found[name]
		if !ok {
			t.Errorf("expected the view %q", name)
			continue
		}
		if !strings.Contains(v.Definition, "SELECT") {
			t.Errorf("%s: expected the defining statement, got %q", name, v.Definition)
		}
	}
	if got := found["recent"]; got.CheckOption.V != "none" {
		t.Errorf("expected no check option, got %q", got.CheckOption.V)
	}
}

// TestColumnStatsNeedAnalyze covers the runtime kind. The fixture inserts rows
// and analyzes one table, so that table has statistics and the others do not,
// which is the distinction a caller has to be able to see.
func TestColumnStatsNeedAnalyze(t *testing.T) {
	db := open(t)
	m := setup(t, db)
	ctx := t.Context()

	byTable := make(map[string]int)
	var rating dbmeta.ColumnStat
	for v, err := range dbmeta.ColumnStats.All(ctx, m, db, args()) {
		if err != nil {
			t.Fatalf("reading column stats: %v", err)
		}
		byTable[v.Table]++
		if v.Table == "author" && v.Name == "rating" {
			rating = v
		}
	}
	if byTable["author"] == 0 {
		t.Fatal("expected statistics for the analyzed table")
	}
	if byTable["book"] != 0 {
		t.Errorf("expected no statistics for a table never analyzed, got %d", byTable["book"])
	}
	if !rating.AvgWidth.Valid || rating.AvgWidth.V <= 0 {
		t.Errorf("expected a width, got %v", rating.AvgWidth)
	}
	if !rating.Distinct.Valid || rating.Distinct.V != 5 {
		t.Errorf("expected five distinct ratings, got %v", rating.Distinct)
	}
	// rating has five values over 200 rows, so every one is a common value
	if !rating.TopN.Valid || len(strings.Split(rating.TopN.V, "\n")) != 5 {
		t.Errorf("expected five common values, got %q", rating.TopN.V)
	}
	// PostgreSQL computes no mean, and absent is not zero
	if rating.Mean.Valid {
		t.Errorf("expected no mean, got %v", rating.Mean)
	}
}

// TestCurrentSchemaIsSessionState covers the one kind that describes the
// connection rather than the database.
func TestCurrentSchemaIsSessionState(t *testing.T) {
	db := open(t)
	m := setup(t, db)
	ctx := t.Context()

	v, ok, err := dbmeta.First(dbmeta.CurrentSchema.All(ctx, m, db, nil))
	if err != nil {
		t.Fatalf("reading the current schema: %v", err)
	}
	if !ok {
		t.Fatal("expected one row")
	}
	if v.Name != "public" {
		t.Errorf("expected public, got %q", v.Name)
	}
	// it follows the session, which is what makes it session state
	if _, err := db.ExecContext(ctx, `SET search_path TO `+fixture.Everything.Schema); err != nil {
		t.Fatalf("setting the search path: %v", err)
	}
	t.Cleanup(func() {
		if _, err := db.ExecContext(context.Background(), `SET search_path TO public`); err != nil {
			t.Logf("resetting the search path: %v", err)
		}
	})
	v, _, err = dbmeta.First(dbmeta.CurrentSchema.All(ctx, m, db, nil))
	if err != nil {
		t.Fatalf("reading the current schema: %v", err)
	}
	if v.Name != fixture.Everything.Schema {
		t.Errorf("expected the schema to follow the session, got %q", v.Name)
	}
}

// TestPrimaryKeyOnColumn covers the field added under D47: psql does not print
// it and two consumers read it per column, and it costs one join.
func TestPrimaryKeyOnColumn(t *testing.T) {
	db := open(t)
	m := setup(t, db)
	ctx := t.Context()

	keyed := make(map[string]bool)
	for v, err := range dbmeta.Columns.All(ctx, m, db, args()) {
		if err != nil {
			t.Fatalf("reading columns: %v", err)
		}
		keyed[v.Table+"."+v.Name] = v.PrimaryKey
	}
	for key, want := range map[string]bool{
		"author.author_id": true,
		"author.name":      false,
		"region.country":   true,
		"region.area":      true,
		"region.__none":    false,
		"shipment.country": false,
	} {
		if key == "region.__none" {
			continue
		}
		if got, ok := keyed[key]; !ok {
			t.Errorf("expected a column %s", key)
		} else if got != want {
			t.Errorf("%s: expected primary_key=%v, got %v", key, want, got)
		}
	}
}

// TestNotNullIsNotAConstraintRow holds the decision in D49. PostgreSQL 18
// records a NOT NULL constraint in pg_constraint and every earlier release
// records it only on the column, so reporting it would make the same schema
// answer differently on two servers for a reason that has nothing to do with
// what either can do.
//
// The padding rule does not catch this. It governs the column set and says
// nothing about rows, and this is the case that found the gap.
func TestNotNullIsNotAConstraintRow(t *testing.T) {
	db := open(t)
	m := setup(t, db)
	ctx := t.Context()

	// the fixture declares NOT NULL on author.name and on several others, so
	// release 18 has rows here to leave out
	for v, err := range dbmeta.Constraints.All(ctx, m, db, args()) {
		if err != nil {
			t.Fatalf("reading constraints: %v", err)
		}
		if v.Type == "not null" || v.Type == "n" {
			t.Errorf("expected no NOT NULL constraint row, got %s.%s", v.Table, v.Name)
		}
		if strings.HasSuffix(v.Name, "_not_null") {
			t.Errorf("expected no NOT NULL constraint row, got %s.%s", v.Table, v.Name)
		}
	}
	for v, err := range dbmeta.ConstraintColumns.All(ctx, m, db, args()) {
		if err != nil {
			t.Fatalf("reading constraint columns: %v", err)
		}
		if strings.HasSuffix(v.Constraint, "_not_null") {
			t.Errorf("expected no NOT NULL constraint column, got %s.%s", v.Table, v.Constraint)
		}
	}

	// the fact is still reported, on the column, where it is filled on every
	// release
	nullable := make(map[string]bool)
	for v, err := range dbmeta.Columns.All(ctx, m, db, args()) {
		if err != nil {
			t.Fatalf("reading columns: %v", err)
		}
		nullable[v.Table+"."+v.Name] = v.Nullable
	}
	if got, ok := nullable["author.name"]; !ok {
		t.Error("expected the author.name column")
	} else if got {
		t.Error("expected author.name to be NOT NULL")
	}
	if got, ok := nullable["author.rating"]; !ok {
		t.Error("expected the author.rating column")
	} else if !got {
		t.Error("expected author.rating to be nullable")
	}
}

// TestQuerierIsOneMethod holds the other half of D49. The interface is the
// whole of what dbmeta asks a database to do, and a type with only
// QueryContext has to be enough.
func TestQuerierIsOneMethod(t *testing.T) {
	db := open(t)
	m := setup(t, db)

	// onlyQuery has one method and nothing else, so this fails to compile if
	// anything in dbmeta reaches for Exec, Prepare or QueryRow.
	var q dbmeta.Querier = onlyQuery{db}

	versions, err := dbmeta.PostgreSQL.Version(t.Context(), q)
	if err != nil {
		t.Fatalf("reading the version through one method: %v", err)
	}
	if versions.Main().IsZero() {
		t.Error("expected a version")
	}
	var n int
	for _, err := range dbmeta.Tables.All(t.Context(), m, q, args()) {
		if err != nil {
			t.Fatalf("reading tables through one method: %v", err)
		}
		n++
	}
	if n == 0 {
		t.Error("expected the fixture relations")
	}
}

// onlyQuery exposes QueryContext and nothing else.
type onlyQuery struct{ db *sql.DB }

func (o onlyQuery) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return o.db.QueryContext(ctx, query, args...)
}
