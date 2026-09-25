package test

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"

	_ "github.com/microsoft/go-mssqldb"

	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/models/sqlserver"
	msfixture "github.com/xo/dbmeta/models/sqlserver/fixture"
)

// openSQLServer returns a connection to the server named by DBMETA_SQLSERVER,
// or skips. The driver is github.com/microsoft/go-mssqldb, which is the one
// usql uses. See D52.
func openSQLServer(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("DBMETA_SQLSERVER")
	if dsn == "" {
		t.Skip("set DBMETA_SQLSERVER to run against a real server")
	}
	db, err := sql.Open("sqlserver", dsn)
	if err != nil {
		t.Fatalf("opening: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.PingContext(t.Context()); err != nil {
		t.Fatalf("connecting: %v", err)
	}
	return db
}

func setupSQLServer(t *testing.T, db *sql.DB) *dbmeta.Meta {
	t.Helper()
	versions, err := dbmeta.SQLServer.Version(t.Context(), db)
	if err != nil {
		t.Fatalf("reading the version: %v", err)
	}
	m, err := dbmeta.New(dbmeta.SQLServer, versions)
	if err != nil {
		t.Fatalf("building the meta: %v", err)
	}
	run := func(ctx context.Context, steps []msfixture.Result, fatal bool) {
		for _, s := range steps {
			if s.Skipped {
				continue
			}
			if _, err := db.ExecContext(ctx, s.SQL); err != nil && fatal {
				t.Fatalf("%s: %v\n%s", s.Name, err, s.SQL)
			}
		}
	}
	down, err := msfixture.Everything.ResolveTeardown(versions)
	if err != nil {
		t.Fatalf("resolving the teardown: %v", err)
	}
	up, err := msfixture.Everything.ResolveSetup(versions)
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

func msArgs() map[string]any {
	return dbmeta.Args{Schema: msfixture.Everything.Schema}.Map()
}

// TestSQLServerEveryQueryRuns executes every query SQL Server answers and
// checks the columns match the declared fields.
func TestSQLServerEveryQueryRuns(t *testing.T) {
	db := openSQLServer(t)
	m := setupSQLServer(t, db)

	var ran, tooOld, unsupported int
	var names []string
	for _, q := range dbmeta.Queries() {
		if q.Support(m) != dbmeta.Supported {
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
		names = append(names, q.Name())
	}
	t.Logf("%d queries ran, %d too old, %d not supported: %s",
		ran, tooOld, unsupported, strings.Join(names, " "))
	if ran == 0 {
		t.Fatal("expected at least one query to run")
	}
}

// TestSQLServerScanning reads rows through the typed API.
func TestSQLServerScanning(t *testing.T) {
	db := openSQLServer(t)
	m := setupSQLServer(t, db)
	ctx := t.Context()

	want := map[string]string{
		"author": "table", "book": "table", "region": "table",
		"shipment": "table", "extras": "table", "recent": "view",
	}
	found := make(map[string]string)
	for v, err := range dbmeta.Tables.All(ctx, m, db, msArgs()) {
		if err != nil {
			t.Fatalf("reading tables: %v", err)
		}
		found[v.Name] = v.Type
	}
	for name, kind := range want {
		if found[name] != kind {
			t.Errorf("expected %q to be a %s, got %q", name, kind, found[name])
		}
	}

	cols := make(map[string]dbmeta.Column)
	for v, err := range dbmeta.Columns.All(ctx, m, db, msArgs()) {
		if err != nil {
			t.Fatalf("reading columns: %v", err)
		}
		cols[v.Table+"."+v.Name] = v
	}
	if got := cols["author.author_id"]; !got.PrimaryKey || got.Nullable {
		t.Errorf("expected author_id to be a non null primary key, got %+v", got)
	}
	// the length is halved for nvarchar, because max_length counts bytes
	if got := cols["author.name"]; got.DataType != "nvarchar(255)" {
		t.Errorf("expected nvarchar(255), got %q", got.DataType)
	}
	// an identity column and a persisted computed column, on the extras table
	if got := cols["extras.extras_id"]; got.Identity.V != "identity" {
		t.Errorf("expected an identity column, got %v", got.Identity)
	}
	if got := cols["extras.slug"]; got.Generated.V != "stored" {
		t.Errorf("expected a persisted computed column, got %v", got.Generated)
	}
	// the extended property, which is what SQL Server has instead of a comment
	if got := cols["author.author_id"]; !got.Comment.Valid || got.Comment.V != "surrogate key" {
		t.Errorf("expected the column comment, got %v", got.Comment)
	}
}

// TestSQLServerConstraints covers the constraint catalog, which SQL Server
// splits across four views and which is the only one here that models a
// default as a named constraint.
func TestSQLServerConstraints(t *testing.T) {
	db := openSQLServer(t)
	m := setupSQLServer(t, db)
	ctx := t.Context()

	byType := make(map[string]int)
	for v, err := range dbmeta.Constraints.All(ctx, m, db, msArgs()) {
		if err != nil {
			t.Fatalf("reading constraints: %v", err)
		}
		byType[v.Type]++
		if v.Type == "not null" {
			t.Errorf("expected no NOT NULL constraint row, got %s.%s", v.Table, v.Name)
		}
	}
	for _, kind := range []string{"primary key", "unique", "foreign key", "check", "default"} {
		if byType[kind] == 0 {
			t.Errorf("expected at least one %s constraint, got %v", kind, byType)
		}
	}

	type ref struct{ col, ftable, fcol string }
	got := make(map[string][]ref)
	for v, err := range dbmeta.ConstraintColumns.All(ctx, m, db, msArgs()) {
		if err != nil {
			t.Fatalf("reading constraint columns: %v", err)
		}
		key := v.Table + "." + v.Constraint
		if int64(len(got[key]))+1 != v.Ordinal {
			t.Errorf("%s: expected ordinal %d, got %d", key, len(got[key])+1, v.Ordinal)
		}
		got[key] = append(got[key], ref{v.Name, v.ForeignTable.V, v.ForeignName.V})
	}
	want := []ref{{"country", "region", "country"}, {"area", "region", "area"}}
	if fk := got["shipment.shipment_region_fk"]; !slices.Equal(fk, want) {
		t.Errorf("expected %v, got %v", want, fk)
	}
	pk := got["region.region_pk"]
	if len(pk) != 2 || pk[0].col != "country" || pk[1].col != "area" {
		t.Errorf("expected the composite key in order, got %v", pk)
	}
}

// TestSQLServerRoutines covers the routine catalog and its parameters. SQL
// Server records a function's return as the parameter with id zero, which is
// the same shape MariaDB uses.
func TestSQLServerRoutines(t *testing.T) {
	db := openSQLServer(t)
	m := setupSQLServer(t, db)
	ctx := t.Context()

	kinds := make(map[string]string)
	ids := make(map[string]string)
	for v, err := range dbmeta.Functions.All(ctx, m, db, msArgs()) {
		if err != nil {
			t.Fatalf("reading functions: %v", err)
		}
		kinds[v.Name] = v.Kind
		ids[v.Name] = v.ID.V
	}
	if kinds["addup"] != "proc" {
		t.Errorf("expected addup to be a procedure, got %q", kinds["addup"])
	}
	if kinds["shout"] != "func" {
		t.Errorf("expected shout to be a function, got %q", kinds["shout"])
	}

	modes := make(map[string]string)
	for v, err := range dbmeta.RoutineParameters.All(ctx, m, db, msArgs()) {
		if err != nil {
			t.Fatalf("reading routine parameters: %v", err)
		}
		modes[v.Routine+"."+v.Name.V] = v.Mode
		if v.Ordinal == 0 && v.Mode != "return" {
			t.Errorf("%s: expected ordinal zero to be the return value, got %q", v.Routine, v.Mode)
		}
		if v.RoutineID.V != ids[v.Routine] {
			t.Errorf("%s: the routine id does not match Functions: %q and %q",
				v.Routine, v.RoutineID.V, ids[v.Routine])
		}
	}
	for key, want := range map[string]string{
		"addup.@a": "in", "addup.@b": "in", "addup.@total": "out",
	} {
		if got := modes[key]; got != want {
			t.Errorf("%s: expected mode %q, got %q", key, want, got)
		}
	}
	// a function records its return as the row with no name
	if got := modes["shout."]; got != "return" {
		t.Errorf("expected a return row for the function, got %q", got)
	}
}

// TestSQLServerCatalogExtras covers the kinds SQL Server answers that most
// databases here do not: a catalog of comments, tablespaces, roles, privileges
// and DDL triggers.
func TestSQLServerCatalogExtras(t *testing.T) {
	db := openSQLServer(t)
	m := setupSQLServer(t, db)
	ctx := t.Context()

	comments := make(map[string]string)
	for v, err := range dbmeta.Comments.All(ctx, m, db, msArgs()) {
		if err != nil {
			t.Fatalf("reading comments: %v", err)
		}
		comments[v.Name] = v.Type
	}
	if comments["author"] != "table" {
		t.Errorf("expected the table comment, got %v", comments)
	}
	if comments["author.author_id"] != "column" {
		t.Errorf("expected the column comment, got %v", comments)
	}

	// PRIMARY is the filegroup every database has, which is the closest thing
	// to a tablespace here
	var primary bool
	for v, err := range dbmeta.Tablespaces.All(ctx, m, db, nil) {
		if err != nil {
			t.Fatalf("reading tablespaces: %v", err)
		}
		primary = primary || v.Name == "PRIMARY"
	}
	if !primary {
		t.Error("expected the PRIMARY filegroup")
	}

	// dbo owns the database and is the one principal every database has
	var dbo bool
	for v, err := range dbmeta.Roles.All(ctx, m, db, nil) {
		if err != nil {
			t.Fatalf("reading roles: %v", err)
		}
		if v.Name == "dbo" {
			dbo = true
			if !v.Superuser {
				t.Error("expected dbo to report as the owner")
			}
		}
	}
	if !dbo {
		t.Error("expected the dbo principal")
	}

	// A sequence arrived in 2012, so 2008 R2 refuses the query and the
	// fixture skips the step that would have made one. Both halves have to
	// agree, and this checks that they do rather than skipping quietly.
	seq, ok, err := dbmeta.First(dbmeta.Sequences.All(ctx, m, db, msArgs()))
	switch {
	case errors.Is(err, dbmeta.ErrVersionTooOld):
		if tooOld := m.Version().Main().Parts[0] < 11; !tooOld {
			t.Errorf("the server is %s and still refuses a sequence as too old", m.Version())
		}
	case err != nil:
		t.Fatalf("reading sequences: %v", err)
	case !ok || seq.Name != "counter" || seq.Start.V != 10 || seq.Increment.V != 2:
		t.Errorf("expected the fixture sequence, got %+v ok=%v", seq, ok)
	}

	// an alias type is what SQL Server has instead of a domain
	dom, ok, err := dbmeta.First(dbmeta.Domains.All(ctx, m, db, msArgs()))
	if err != nil {
		t.Fatalf("reading domains: %v", err)
	}
	if !ok || dom.Name != "shortname" || dom.DataType != "nvarchar" {
		t.Errorf("expected the alias type, got %+v ok=%v", dom, ok)
	}

	v, ok, err := dbmeta.First(dbmeta.CurrentSchema.All(ctx, m, db, nil))
	if err != nil {
		t.Fatalf("reading the current schema: %v", err)
	}
	if !ok || v.Name != "dbo" {
		t.Errorf("expected dbo, got %q ok=%v", v.Name, ok)
	}
}

// TestSQLServerStats covers the two statistics kinds. ColumnStats reports far
// less than PostgreSQL does, because everything else is in the histogram and
// that needs DBCC SHOW_STATISTICS, which is a second statement.
func TestSQLServerStats(t *testing.T) {
	db := openSQLServer(t)
	m := setupSQLServer(t, db)
	ctx := t.Context()

	// sys.dm_db_stats_properties arrived in 2012, so 2008 R2 refuses this.
	if err := dbmeta.ColumnStats.Support(m); err == dbmeta.Supported {
		if _, _, e := dbmeta.ColumnStats.SQL(m, msArgs()); errors.Is(e, dbmeta.ErrVersionTooOld) {
			t.Skipf("column statistics need release 11, and this server is %s", m.Version())
		}
	}

	var stats int
	for v, err := range dbmeta.ColumnStats.All(ctx, m, db, msArgs()) {
		if errors.Is(err, dbmeta.ErrVersionTooOld) {
			t.Skipf("column statistics need release 11, and this server is %s", m.Version())
		}
		if err != nil {
			t.Fatalf("reading column stats: %v", err)
		}
		stats++
		// the fields that are always absent must be absent, not zero
		if v.AvgWidth.Valid || v.NullFrac.Valid || v.Min.Valid || v.TopN.Valid {
			t.Errorf("expected the histogram fields to be absent, got %+v", v)
		}
	}
	if stats == 0 {
		t.Error("expected statistics after the fixture updated them")
	}

	var extended bool
	for v, err := range dbmeta.ExtendedStats.All(ctx, m, db, msArgs()) {
		if err != nil {
			t.Fatalf("reading extended stats: %v", err)
		}
		if v.Name == "author_name_rating" {
			extended = true
			if !strings.Contains(v.Kinds, "name") || !strings.Contains(v.Kinds, "rating") {
				t.Errorf("expected both columns in the kinds, got %q", v.Kinds)
			}
		}
	}
	if extended && m.Version().Main().AtLeast(dbmeta.V(13)) {
		return
	}
	if !extended {
		t.Error("expected the multi column statistics object the fixture created")
	}
}

// TestSQLServerUnsupported checks that what SQL Server does not have says so.
func TestSQLServerUnsupported(t *testing.T) {
	db := openSQLServer(t)
	m := setupSQLServer(t, db)
	for _, q := range []dbmeta.AnyQuery{
		// no enumerated type, so nothing to list
		dbmeta.EnumValues,
		// no operator catalog and nothing that can be created
		dbmeta.Operators, dbmeta.OperatorClasses, dbmeta.OperatorFamilies,
		dbmeta.Casts, dbmeta.Conversions,
		// no procedural language catalog: T-SQL is the language and CLR is an
		// assembly, which is not a language
		dbmeta.Languages,
		// no text search objects of the shape psql names, and no logical
		// replication
		dbmeta.TextSearchConfigs, dbmeta.TextSearchParsers,
		dbmeta.Publications, dbmeta.Subscriptions,
		// analogues that were named by a review and rejected. See docs/COVERAGE.md.
		dbmeta.Extensions, dbmeta.ExtensionObjects, dbmeta.AccessMethods,
		dbmeta.LargeObjects, dbmeta.DefaultACLs, dbmeta.RoleSettings,
		dbmeta.ForeignDataWrappers,
	} {
		if got := q.Support(m); got != dbmeta.NotSupported {
			t.Errorf("%s: expected it to be reported unsupported, got %v", q.Name(), got)
		}
		if _, _, err := q.SQL(m, nil); !errors.Is(err, dbmeta.ErrNotSupported) {
			t.Errorf("%s: expected ErrNotSupported, got: %v", q.Name(), err)
		}
	}
}

// TestSQLServerVersion checks the version comes from SERVERPROPERTY rather
// than from @@VERSION, which is a sentence and changes with the language.
// productName matches the name SQL Server is sold under, which @@VERSION
// gives as "Microsoft SQL Server 2022" or "Microsoft SQL Server 2008 R2".
var productName = regexp.MustCompile(`^Microsoft SQL Server (19|20)[0-9]{2}( R[0-9])?$`)

func TestSQLServerVersion(t *testing.T) {
	db := openSQLServer(t)
	versions, err := dbmeta.SQLServer.Version(t.Context(), db)
	if err != nil {
		t.Fatalf("reading the version: %v", err)
	}
	if versions.Main().Unknown {
		t.Fatal("expected a version")
	}
	// 10 is 2008 R2, which is the oldest release this project provisions at
	// all. It was 14 when a Linux container was the only way to run one, and
	// a virtual machine reaches further back now. See D57.
	if got := versions.Main().Parts[0]; got < 10 {
		t.Errorf("expected release 10 or newer, got %d", got)
	}
	// The line a person reads, which usql prints on connecting. It names the
	// product, the build, the patch level and the edition, and every part
	// comes from the server rather than from a table here.
	display := versions.String()
	if !strings.HasPrefix(display, "Microsoft SQL Server ") {
		t.Errorf("unexpected display line %q", display)
	}
	for _, want := range []string{versions.Main().Raw, "Edition"} {
		if !strings.Contains(display, want) {
			t.Errorf("expected %q in the display line, got %q", want, display)
		}
	}
	// "Microsoft SQL Server 2022 16.0.4295.3, RTM-CU27, Developer Edition"
	name, rest, ok := strings.Cut(display, " "+versions.Main().Raw+", ")
	if !ok {
		t.Fatalf("expected the product name then the build, got %q", display)
	}
	if !productName.MatchString(name) {
		t.Errorf("expected the display to name a release year, got %q", name)
	}
	if rest == "" {
		t.Errorf("expected a level and an edition after the build, got %q", display)
	}
	if !strings.HasPrefix(sqlserver.Reference, "1") {
		t.Errorf("unexpected reference %q", sqlserver.Reference)
	}
}
