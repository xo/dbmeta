package test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"

	_ "github.com/xo/dbimp/drill"

	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/container"
	_ "github.com/xo/dbmeta/models/drill"
	dlfixture "github.com/xo/dbmeta/models/drill/fixture"
)

// drillAnswers is the number of kinds the Drill model answers.
const drillAnswers = 10

// openDrill returns a connection to the server named by DBMETA_DRILL, which is
// the drill:// URL that github.com/xo/dbimp/drill takes, and which dburl's
// drill scheme opens (D154, D178).
func openDrill(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("DBMETA_DRILL")
	if dsn == "" {
		t.Skip("set DBMETA_DRILL to run against a real server")
	}
	db, err := sql.Open("drill", dsn)
	if err != nil {
		t.Fatalf("opening: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.PingContext(t.Context()); err != nil {
		t.Fatalf("connecting: %v", err)
	}
	return db
}

// setupDrill builds the fixture as the administrator and returns the metadata
// for the server. The fixture turns the Metastore on and analyzes each table,
// because Drill lists no file table otherwise (D178).
func setupDrill(t *testing.T, db *sql.DB) *dbmeta.Meta {
	t.Helper()
	ctx := t.Context()
	versions, err := dbmeta.Drill.Version(ctx, db)
	if err != nil {
		t.Fatalf("reading the version: %v", err)
	}
	for _, step := range dlfixture.Everything.Setup {
		if err := drillRun(ctx, db, step.Query); err != nil {
			t.Fatalf("setup %s: %v\n%s", step.Name, err, step.Query)
		}
	}
	m, err := dbmeta.New(dbmeta.Drill, versions)
	if err != nil {
		t.Fatalf("building the metadata: %v", err)
	}
	t.Logf("server reports %s", versions)
	return m
}

// drillRun runs one statement and reads its result to the end, because Drill
// answers every statement with rows.
func drillRun(ctx context.Context, db *sql.DB, query string) error {
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return fmt.Errorf("running the statement: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("reading the answer: %w", err)
	}
	return nil
}

// TestDrillVersion reads the version and checks what the model makes of it.
func TestDrillVersion(t *testing.T) {
	db := openDrill(t)
	versions, err := dbmeta.Drill.Version(t.Context(), db)
	if err != nil {
		t.Fatalf("reading the version: %v", err)
	}
	if main := versions.Main(); main.Unknown || main.Parts[0] != 1 || main.Parts[1] < 21 {
		t.Errorf("expected release 1.21 or newer, got %s", main)
	}
	if !strings.HasPrefix(versions.String(), "Apache Drill 1.") {
		t.Errorf("expected the display line to name Apache Drill, got %q", versions)
	}
	t.Logf("server reports %s", versions)
}

// TestDrillVersionForAnOrdinaryUser checks that a user who cannot change an
// option reads the version too, because sys.version is open to every user.
func TestDrillVersionForAnOrdinaryUser(t *testing.T) {
	dsn := os.Getenv("DBMETA_DRILL")
	if dsn == "" {
		t.Skip("set DBMETA_DRILL to run against a real server")
	}
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("parsing %s: %v", dsn, err)
	}
	u.User = url.UserPassword(container.DrillUser, container.Password)
	db, err := sql.Open("drill", u.String())
	if err != nil {
		t.Fatalf("opening: %v", err)
	}
	defer db.Close()
	versions, err := dbmeta.Drill.Version(t.Context(), db)
	if err != nil {
		t.Fatalf("reading the version as an ordinary user: %v", err)
	}
	if versions.Main().Unknown {
		t.Errorf("expected a known release, got %s", versions)
	}
}

// TestDrillEveryQueryRuns runs every query the model answers and checks that
// the columns of each row are the declared fields, by name and in order.
func TestDrillEveryQueryRuns(t *testing.T) {
	db := openDrill(t)
	m := setupDrill(t, db)
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
		cols, err := columnsOf(t, db, query, vals)
		if err != nil {
			t.Errorf("%s: %v\n%s", q.Name(), err, query)
			continue
		}
		fields, err := q.Fields(m)
		if err != nil {
			t.Fatalf("%s: reading fields: %v", q.Name(), err)
		}
		names := make([]string, len(fields))
		for i, f := range fields {
			names[i] = f.Name
		}
		if len(cols) == 0 {
			t.Errorf("%s: no columns", q.Name())
		} else if strings.Join(cols, ",") != strings.Join(names, ",") {
			t.Errorf("%s: returns %v and declares %v", q.Name(), cols, names)
		}
		ran++
	}
	if ran != drillAnswers {
		t.Errorf("%d queries ran, and the model answers %d: change this test with the model", ran, drillAnswers)
	}
}

// TestDrillScansEveryQuery reads every query through its own Scan, with the
// system objects included.
func TestDrillScansEveryQuery(t *testing.T) {
	db := openDrill(t)
	scanEveryQuery(t, setupDrill(t, db), db)
}

// TestDrillFixtureObjects reads the fixture back through the typed API.
func TestDrillFixtureObjects(t *testing.T) {
	db := openDrill(t)
	m := setupDrill(t, db)
	ctx := t.Context()
	fx := dlfixture.Everything

	tables := map[string]string{}
	for v, err := range dbmeta.Tables.All(ctx, m, db, dbmeta.Args{Schema: fx.Schema}.Map()) {
		if err != nil {
			t.Fatalf("reading tables: %v", err)
		}
		if v.Catalog != "DRILL" || v.Schema != fx.Schema {
			t.Errorf("table %s: catalog %q and schema %q, want DRILL and %q", v.Name, v.Catalog, v.Schema, fx.Schema)
		}
		tables[v.Name] = v.Type
		// NUM_ROWS is the count ANALYZE stored, and a view has none. Drill
		// keeps no owner, persistence, size or options. See D210.
		switch v.Name {
		case "author":
			if !v.Rows.Valid || v.Rows.V != 2 {
				t.Errorf("expected two rows for author, got %+v", v.Rows)
			}
		case "recent":
			if v.Rows.Valid {
				t.Errorf("expected no row count for a view, got %+v", v.Rows)
			}
		}
		if v.Owner.Valid || v.Persistence.Valid || v.Size.Valid || v.Options.Valid {
			t.Errorf("table %s: expected no owner, persistence, size or options, got %+v", v.Name, v)
		}
	}
	for _, name := range []string{"author", "book", "region", "shipment"} {
		if tables[name] != "table" {
			t.Errorf("expected %s as a table, got %q", name, tables[name])
		}
	}
	if tables["recent"] != "view" {
		t.Errorf("expected recent as a view, got %q", tables["recent"])
	}

	// Types narrows the list, and the system tables need with_system.
	if n := countRows(t, dbmeta.Tables.All(ctx, m, db, dbmeta.Args{Schema: fx.Schema, Types: []string{"view"}}.Map())); n != 1 {
		t.Errorf("expected one view, got %d", n)
	}
	sys := map[string]string{}
	for v, err := range dbmeta.Tables.All(ctx, m, db, dbmeta.Args{Schema: "sys", WithSystem: true}.Map()) {
		if err != nil {
			t.Fatalf("reading the system tables: %v", err)
		}
		sys[v.Name] = v.Type
	}
	if sys["options"] != "system table" {
		t.Errorf("expected sys.options as a system table, got %v", sys)
	}
	if n := countRows(t, dbmeta.Tables.All(ctx, m, db, dbmeta.Args{Schema: "sys"}.Map())); n != 0 {
		t.Errorf("expected no sys table without with_system, got %d", n)
	}

	cols := map[string]map[string]dbmeta.Column{}
	for v, err := range dbmeta.Columns.All(ctx, m, db, dbmeta.Args{Schema: fx.Schema}.Map()) {
		if err != nil {
			t.Fatalf("reading columns: %v", err)
		}
		if cols[v.Table] == nil {
			cols[v.Table] = map[string]dbmeta.Column{}
		}
		cols[v.Table][v.Name] = v
	}
	for table, want := range map[string]map[string][3]any{
		"author":   {"author_id": {1, "BIGINT", false}, "name": {2, "CHARACTER VARYING", false}, "rating": {3, "BIGINT", true}},
		"shipment": {"amount": {4, "DECIMAL", false}, "weight": {5, "DOUBLE", false}},
		"book":     {"published": {4, "DATE", false}},
		"recent":   {"amount": {2, "ANY", true}},
	} {
		for name, w := range want {
			c, ok := cols[table][name]
			if !ok || c.Ordinal != w[0] || c.DataType != w[1] || c.Nullable != w[2] {
				t.Errorf("%s.%s: want ordinal %v, type %v and nullable %v, got %+v", table, name, w[0], w[1], w[2], c)
			}
		}
	}
	if c := cols["author"]["rating"]; c.Default.Valid || c.PrimaryKey || c.Comment.Valid {
		t.Errorf("expected author.rating with no default, key or comment, got %+v", c)
	}

	view, ok, err := dbmeta.First(dbmeta.Views.All(ctx, m, db, dbmeta.Args{Schema: fx.Schema}.Map()))
	if err != nil || !ok || view.Name != "recent" || !strings.Contains(view.Definition.V, "shipment") {
		t.Errorf("expected the view recent over shipment, got %+v, %v, %v", view, ok, err)
	}

	// Statistics come from ANALYZE TABLE. author.rating is NULL for one of
	// two rows.
	stats := map[string]dbmeta.ColumnStat{}
	for v, err := range dbmeta.ColumnStats.All(ctx, m, db, dbmeta.Args{Schema: fx.Schema, Parent: "author"}.Map()) {
		if err != nil {
			t.Fatalf("reading column statistics: %v", err)
		}
		stats[v.Name] = v
	}
	if s := stats["rating"]; !s.NullFrac.Valid || s.NullFrac.V != 0.5 || s.Min.V != "5" || s.Max.V != "5" {
		t.Errorf("expected author.rating half NULL between 5 and 5, got %+v", s)
	}
	if len(stats) != 4 {
		t.Errorf("expected the four columns of author, got %d", len(stats))
	}
	if n := countRows(t, dbmeta.ColumnStats.All(ctx, m, db, dbmeta.Args{Schema: fx.Schema, Parent: "recent"}.Map())); n != 0 {
		t.Errorf("expected no statistics for the view recent, got %d rows", n)
	}
	if s := stats["name"]; s.NullFrac.V != 0 || s.Min.V != "Terry" || s.Max.V != "Ursula" {
		t.Errorf("expected author.name between Terry and Ursula, got %+v", s)
	}

	dbs, ok, err := dbmeta.First(dbmeta.Databases.All(ctx, m, db, nil))
	if err != nil || !ok || dbs.Name != "DRILL" {
		t.Errorf("expected the database DRILL, got %+v, %v, %v", dbs, ok, err)
	}

	schemas := map[string]dbmeta.Schema{}
	for v, err := range dbmeta.Schemas.All(ctx, m, db, nil) {
		if err != nil {
			t.Fatalf("reading schemas: %v", err)
		}
		schemas[v.Name] = v
	}
	if s, ok := schemas[fx.Schema]; !ok || s.Owner != "" || s.Catalog != "DRILL" {
		t.Errorf("expected the schema %s with no owner, got %+v", fx.Schema, s)
	}
	if _, ok := schemas["sys"]; ok {
		t.Error("expected no sys schema without with_system")
	}

	if n := countRows(t, dbmeta.Functions.All(ctx, m, db, dbmeta.Args{Name: "upper"}.Map())); n != 0 {
		t.Errorf("expected no function without with_system, got %d", n)
	}
	fn, ok, err := dbmeta.First(dbmeta.Functions.All(ctx, m, db, dbmeta.Args{Name: "upper", WithSystem: true}.Map()))
	if err != nil || !ok || fn.Source.V != "built-in" || fn.ResultType.V != "VARCHAR" || fn.ArgTypes.V != "VARCHAR-REQUIRED" {
		t.Errorf("expected upper from the built in functions, got %+v, %v, %v", fn, ok, err)
	}

	set, ok, err := dbmeta.First(dbmeta.Settings.All(ctx, m, db, dbmeta.Args{Name: "metastore.enabled"}.Map()))
	if err != nil || !ok || set.Value.V != "true" || set.Type.V != "BIT" || !set.Context.Valid {
		t.Errorf("expected the Metastore on, got %+v, %v, %v", set, ok, err)
	}

	user, ok, err := dbmeta.First(dbmeta.CurrentUser.All(ctx, m, db, nil))
	if err != nil || !ok || user.Name != "admin" || user.Session.V != "admin" {
		t.Errorf("expected the user admin, got %+v, %v, %v", user, ok, err)
	}
}

// TestDrillNoCurrentSchema checks that a session with no default schema has no
// current schema, and not a row with an empty name.
func TestDrillNoCurrentSchema(t *testing.T) {
	db := openDrill(t)
	m := setupDrill(t, db)
	if n := countRows(t, dbmeta.CurrentSchema.All(t.Context(), m, db, nil)); n != 0 {
		t.Errorf("expected no current schema without a default, got %d rows", n)
	}
}

// TestDrillCurrentSchema checks that USE gives the session a current schema.
func TestDrillCurrentSchema(t *testing.T) {
	db := openDrill(t)
	m := setupDrill(t, db)
	// One connection, so that USE and the query share a session.
	conn, err := db.Conn(t.Context())
	if err != nil {
		t.Fatalf("taking a connection: %v", err)
	}
	t.Cleanup(func() {
		if err := conn.Close(); err != nil {
			t.Errorf("closing the connection: %v", err)
		}
	})
	if _, err := conn.ExecContext(t.Context(), "USE "+dlfixture.Everything.Schema); err != nil {
		t.Fatalf("running USE: %v", err)
	}
	s, ok, err := dbmeta.First(dbmeta.CurrentSchema.All(t.Context(), m, conn, nil))
	if err != nil || !ok || s.Name != dlfixture.Everything.Schema || s.Catalog != "DRILL" {
		t.Errorf("expected the current schema %s, got %+v, %v, %v", dlfixture.Everything.Schema, s, ok, err)
	}
}

// TestDrillMetastoreOff checks the price of D178. With the Metastore off,
// Tables lists no file table and gives no error.
func TestDrillMetastoreOff(t *testing.T) {
	db := openDrill(t)
	m := setupDrill(t, db)
	ctx := t.Context()
	set := func(on string) {
		if _, err := db.ExecContext(ctx, "ALTER SYSTEM SET `metastore.enabled` = "+on); err != nil {
			t.Fatalf("setting metastore.enabled to %s: %v", on, err)
		}
	}
	set("false")
	defer set("true")
	var files int
	for v, err := range dbmeta.Tables.All(ctx, m, db, dbmeta.Args{Schema: dlfixture.Everything.Schema}.Map()) {
		if err != nil {
			t.Fatalf("reading tables with the Metastore off: %v", err)
		}
		if v.Type == "table" {
			files++
		}
	}
	if files != 0 {
		t.Errorf("expected no file table with the Metastore off, got %d", files)
	}
}

// TestDrillLeavesOut checks the kinds the model does not answer. Drill has no
// index, constraint, trigger, sequence or role that SQL lists.
func TestDrillLeavesOut(t *testing.T) {
	m, err := dbmeta.New(dbmeta.Drill, dbmeta.VersionSet{})
	if err != nil {
		t.Fatal(err)
	}
	for _, q := range []dbmeta.AnyQuery{
		dbmeta.Indexes, dbmeta.IndexColumns, dbmeta.Constraints, dbmeta.Triggers, dbmeta.Sequences,
		dbmeta.Roles, dbmeta.Privileges, dbmeta.Aggregates, dbmeta.RoutineParameters,
		dbmeta.PartitionedTables, dbmeta.Comments,
	} {
		if s := q.Support(m); s != dbmeta.NotSupported {
			t.Errorf("%s: %v, want NotSupported", q.Name(), s)
		}
	}
	if _, _, err := dbmeta.Indexes.Build(m, nil); !errors.Is(err, dbmeta.ErrNotSupported) {
		t.Errorf("indexes: expected ErrNotSupported, got %v", err)
	}
}
