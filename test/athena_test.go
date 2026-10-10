package test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"

	_ "github.com/xo/dbimp/athena"

	"github.com/xo/dbmeta"
	_ "github.com/xo/dbmeta/models/athena"
	athfixture "github.com/xo/dbmeta/models/athena/fixture"
)

// athenaLocationOf is the S3 prefix that the fixture puts its tables under. The
// account allows a table only under tables/dbmeta/ of its bucket, and the bucket
// is the one that the key output of the DSN names. The host of the DSN is the
// endpoint of the service. The error holds no part of the DSN, which has a secret.
func athenaLocationOf(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", errors.New("reading the bucket from the DSN of Athena: the value is not a URL")
	}
	bucket, _, _ := strings.Cut(strings.TrimPrefix(u.Query().Get("output"), "s3://"), "/")
	if bucket == "" {
		return "", errors.New("reading the bucket from the DSN of Athena: the value has no output")
	}
	return "s3://" + bucket + "/tables/dbmeta/", nil
}

// athenaLocation is athenaLocationOf for a test, which stops on an error.
func athenaLocation(t *testing.T, raw string) string {
	t.Helper()
	loc, err := athenaLocationOf(raw)
	if err != nil {
		t.Fatal(err)
	}
	return loc
}

// openAthena returns a connection to the service named by DBMETA_ATHENA, which
// dbrun resolves from the places D117 names. The value holds a secret and no
// test prints it.
//
// The driver is github.com/xo/dbimp/athena, which dburl v0.50.0 names for the
// athena scheme (D154, D229). It signs each request with the key pair of the DSN
// and reads no credential from the environment. See D222.
func openAthena(t *testing.T) *sql.DB {
	t.Helper()
	raw := os.Getenv("DBMETA_ATHENA")
	if raw == "" {
		t.Skip("set DBMETA_ATHENA to run against the service")
	}
	return openAt(t, "athena", raw)
}

// athenaState is the one fixture that every Athena test shares.
//
// Athena runs each statement as a job that takes a second or more, so a fixture
// for each test is more than the tests are worth. The first test builds it,
// every test reads it, and TestMain drops it when the last test ends. No test
// changes it.
var athenaState struct {
	mu    sync.Mutex
	meta  *dbmeta.Meta
	err   error
	built bool
}

// runAthenaSteps runs the statements one at a time. A step of a teardown that
// fails is not an error when quiet is true, because the object is already gone.
func runAthenaSteps(ctx context.Context, db *sql.DB, steps []athfixture.Result, quiet bool) error {
	for _, s := range steps {
		_, err := db.ExecContext(ctx, s.Query)
		if err != nil && !quiet {
			return fmt.Errorf("%s: %w\n%s", s.Name, err, s.Query)
		}
	}
	return nil
}

// shutdownAthena drops the fixture, if a test built it. TestMain calls it.
func shutdownAthena() {
	athenaState.mu.Lock()
	defer athenaState.mu.Unlock()
	if !athenaState.built {
		return
	}
	// DBMETA_ATHENA_KEEP leaves the fixture up, so that a person can write a
	// statement against it, and the next run with it set reuses that fixture.
	if os.Getenv("DBMETA_ATHENA_KEEP") != "" {
		return
	}
	raw := os.Getenv("DBMETA_ATHENA")
	db, err := sql.Open("athena", raw)
	if err != nil {
		fmt.Fprintln(os.Stderr, "dropping the Athena fixture: opening the driver failed")
		return
	}
	defer db.Close()
	loc, err := athenaLocationOf(raw)
	if err != nil {
		fmt.Fprintf(os.Stderr, "dropping the Athena fixture: %v\n", err)
		return
	}
	down, err := athfixture.Everything.ResolveTeardown(dbmeta.VersionSet{}, loc)
	if err != nil {
		fmt.Fprintf(os.Stderr, "dropping the Athena fixture: %v\n", err)
		return
	}
	//nolint:errcheck // a teardown is best effort, and quiet says so
	runAthenaSteps(context.Background(), db, down, true)
}

// setupAthena builds the fixture, once, and returns the metadata for the
// service.
//
// It tears down first, because a run that failed part way can leave the tables
// behind, and CREATE TABLE then fails and the test reports that and not what
// went wrong.
func setupAthena(t *testing.T, db *sql.DB) *dbmeta.Meta {
	t.Helper()
	athenaState.mu.Lock()
	defer athenaState.mu.Unlock()
	if athenaState.err != nil {
		t.Fatalf("the fixture failed to build in an earlier test: %v", athenaState.err)
	}
	if athenaState.meta != nil {
		return athenaState.meta
	}
	ctx := context.WithoutCancel(t.Context())
	fail := func(format string, args ...any) {
		athenaState.err = fmt.Errorf(format, args...)
		t.Fatal(athenaState.err)
	}
	location := athenaLocation(t, os.Getenv("DBMETA_ATHENA"))
	versions, err := dbmeta.Athena.Version(ctx, db)
	if err != nil {
		fail("reading the version: %v", err)
	}
	m, err := dbmeta.New(dbmeta.Athena, versions)
	if err != nil {
		fail("building the metadata: %v", err)
	}
	if os.Getenv("DBMETA_ATHENA_KEEP") != "" {
		// A person kept the fixture of an earlier run to write against, and the
		// last table that the setup makes says it is whole.
		if tables, err := collectAthena(t, dbmeta.Tables, m, db, dbmeta.Args{Name: "ticket"}.Map()); err == nil && len(tables) > 0 {
			athenaState.built, athenaState.meta = true, m
			return m
		}
	}
	down, err := athfixture.Everything.ResolveTeardown(versions, location)
	if err != nil {
		fail("resolving the teardown: %v", err)
	}
	//nolint:errcheck // a teardown before setup is best effort, and quiet says so
	runAthenaSteps(ctx, db, down, true)
	up, err := athfixture.Everything.ResolveSetup(versions, location)
	if err != nil {
		fail("resolving the setup: %v", err)
	}
	athenaState.built = true
	if err := runAthenaSteps(ctx, db, up, false); err != nil {
		fail("setup: %v", err)
	}
	t.Logf("server reports %s, fixture ran %d steps", versions, len(up))
	athenaState.meta = m
	return m
}

// collectAthena reads every row a query answers.
func collectAthena[T any](t *testing.T, q *dbmeta.Query[T], m *dbmeta.Meta, db *sql.DB, args map[string]any) ([]T, error) {
	t.Helper()
	var out []T
	for v, err := range q.All(t.Context(), m, db, args) {
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", q.Name(), err)
		}
		out = append(out, v)
	}
	return out, nil
}

// athenaAll reads every row a query answers, and stops the test on an error.
func athenaAll[T any](t *testing.T, q *dbmeta.Query[T], m *dbmeta.Meta, db *sql.DB, args map[string]any) []T {
	t.Helper()
	out, err := collectAthena(t, q, m, db, args)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// athenaDrain runs a statement and reads every row, and returns the error that
// either step gave.
func athenaDrain(ctx context.Context, db *sql.DB, stmt string) error {
	rows, err := db.QueryContext(ctx, stmt)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
	}
	return rows.Err()
}

// athenaColumns runs the statement and returns the names of its columns.
func athenaColumns(t *testing.T, db *sql.DB, query string, vals []any) ([]string, error) {
	t.Helper()
	rows, err := db.QueryContext(t.Context(), query, vals...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	for rows.Next() {
	}
	return cols, rows.Err()
}

// TestAthenaVersion checks that the service reports no version, which is why
// the model declares no version query. See D222.
func TestAthenaVersion(t *testing.T) {
	db := openAthena(t)
	versions, err := dbmeta.Athena.Version(t.Context(), db)
	if err != nil {
		t.Fatalf("reading the version: %v", err)
	}
	if !versions.Main().IsZero() && !versions.Main().Unknown {
		t.Errorf("expected no version, got %s", versions)
	}
	// usql reads node_version from system.runtime.nodes, and Athena refuses that
	// catalog. version() is not registered either.
	if err := athenaDrain(t.Context(), db, "SELECT node_version FROM system.runtime.nodes LIMIT 1"); err == nil {
		t.Error("system.runtime.nodes: expected Athena to refuse it")
	}
	if err := athenaDrain(t.Context(), db, "SELECT version()"); err == nil {
		t.Error("SELECT version(): expected Athena to refuse it")
	}
	t.Logf("server reports %s", versions)
}

// TestAthenaSmoke runs each registered query against the fixture and checks
// that it returns the columns it declares.
func TestAthenaSmoke(t *testing.T) {
	db := openAthena(t)
	m := setupAthena(t, db)

	var ran, unsupported int
	for _, q := range dbmeta.Queries() {
		switch q.Support(m) {
		case dbmeta.NotBuilt, dbmeta.NotSupported, dbmeta.TooOld:
			unsupported++
			continue
		case dbmeta.Supported:
		}
		query, vals, err := q.Build(m, nil)
		if err != nil {
			t.Errorf("%s: building: %v", q.Name(), err)
			continue
		}
		cs, err := athenaColumns(t, db, query, vals)
		if err != nil {
			t.Errorf("%s: %v\n%s", q.Name(), err, query)
			continue
		}
		fields, err := q.Fields(m)
		if err != nil {
			t.Errorf("%s: reading fields: %v", q.Name(), err)
			continue
		}
		if len(cs) < len(fields) {
			t.Errorf("%s: declares %d fields and returns %d columns", q.Name(), len(fields), len(cs))
		}
		for i, f := range fields {
			if i < len(cs) && cs[i] != f.Name {
				t.Errorf("%s: column %d is %q and the field is %q", q.Name(), i, cs[i], f.Name)
			}
		}
		ran++
	}
	if ran != 8 {
		t.Errorf("expected the 8 queries the package comment names to run, %d ran", ran)
	}
	t.Logf("%d queries ran, %d not supported", ran, unsupported)
}

// TestAthenaTables reads the relations of the fixture back through the typed
// API.
func TestAthenaTables(t *testing.T) {
	db := openAthena(t)
	m := setupAthena(t, db)

	// The Glue database also holds spectrum_t, which the Redshift Spectrum test
	// needs, so the test looks for the tables of the fixture and counts no more.
	tables := map[string]dbmeta.Table{}
	for _, v := range athenaAll(t, dbmeta.Tables, m, db, nil) {
		tables[v.Name] = v
		if v.Catalog != "awsdatacatalog" || v.Schema != athfixture.Everything.Schema {
			t.Errorf("table %s: expected the catalog awsdatacatalog and the database dbmeta, got %q and %q", v.Name, v.Catalog, v.Schema)
		}
		if v.Comment.Valid || v.Owner.Valid || v.Size.Valid || v.Rows.Valid || v.Options.Valid {
			t.Errorf("table %s: expected no comment, owner, size, rows or options, got %+v", v.Name, v)
		}
	}
	for name, typ := range map[string]string{
		"author": "table", "book": "table", "region": "table", "shipment": "table",
		"sales": "table", "tally": "table", "ticket": "table", "recent": "view",
	} {
		if tables[name].Type != typ {
			t.Errorf("table %s: expected type %q, got %q", name, typ, tables[name].Type)
		}
	}
	if _, ok := tables["information_schema"]; ok {
		t.Error("expected no table of information_schema without with_system")
	}

	// The filters.
	if got := athenaAll(t, dbmeta.Tables, m, db, dbmeta.Args{Name: "auth%"}.Map()); len(got) != 1 || got[0].Name != "author" {
		t.Errorf("expected author alone, got %+v", got)
	}
	for _, v := range athenaAll(t, dbmeta.Tables, m, db, dbmeta.Args{Types: []string{"view"}}.Map()) {
		if v.Type != "view" {
			t.Errorf("expected a view, got %+v", v)
		}
	}
	if got := athenaAll(t, dbmeta.Tables, m, db, dbmeta.Args{Schema: "no_such_database"}.Map()); len(got) != 0 {
		t.Errorf("expected no table in a database that does not exist, got %+v", got)
	}
	var system int
	for _, v := range athenaAll(t, dbmeta.Tables, m, db, dbmeta.Args{WithSystem: true, Schema: "information_schema"}.Map()) {
		if v.Schema == "information_schema" && v.Name == "columns" {
			system++
		}
	}
	if system != 1 {
		t.Errorf("expected the table columns of information_schema with with_system, got %d", system)
	}

	// Views.
	views := athenaAll(t, dbmeta.Views, m, db, dbmeta.Args{Name: "recent"}.Map())
	if len(views) != 1 || !strings.Contains(views[0].Definition.V, "published IS NOT NULL") {
		t.Fatalf("expected the one view recent, got %+v", views)
	}
	if v := views[0]; v.Comment.Valid || v.Updatable.V || v.Insertable.V || v.CheckOption.Valid {
		t.Errorf("view recent: expected no comment and no update, got %+v", v)
	}
}

// TestAthenaColumns reads the columns of the fixture.
func TestAthenaColumns(t *testing.T) {
	db := openAthena(t)
	m := setupAthena(t, db)

	columns := map[string]dbmeta.Column{}
	for _, v := range athenaAll(t, dbmeta.Columns, m, db, nil) {
		columns[v.Table+"."+v.Name] = v
	}
	type want struct {
		ordinal  int
		dataType string
	}
	for name, w := range map[string]want{
		"author.author_id":     {1, "integer"},
		"author.name":          {2, "varchar"},
		"author.shade":         {4, "varchar"},
		"book.published":       {4, "date"},
		"region.country":       {1, "varchar"},
		"region.area":          {2, "varchar"},
		"sales.amount":         {1, "bigint"},
		"sales.sold_on":        {3, "varchar"},
		"sales.region":         {4, "varchar"},
		"recent.title":         {2, "varchar"},
		"shipment.shipment_id": {1, "integer"},
		"tally.tally_id":       {1, "integer"},
		"ticket.created":       {3, "timestamp(6)"},
	} {
		got, ok := columns[name]
		if !ok {
			t.Errorf("column %s: not found", name)
			continue
		}
		if got.Ordinal != w.ordinal || got.DataType != w.dataType {
			t.Errorf("column %s: expected ordinal %d and type %q, got %d and %q", name, w.ordinal, w.dataType, got.Ordinal, got.DataType)
		}
	}
	for name, c := range columns {
		if strings.HasPrefix(name, "spectrum_t.") {
			continue
		}
		if !c.Nullable || c.PrimaryKey || c.Default.Valid || c.Identity.Valid || c.Generated.Valid || c.Collation.Valid {
			t.Errorf("column %s: expected a nullable column with no key, default, identity, generation or collation, got %+v", name, c)
		}
	}
	if got := columns["author.name"].Comment; got.V != "the full name" {
		t.Errorf("column name: expected its comment, got %+v", got)
	}
	if got := columns["author.rating"].Comment; got.Valid {
		t.Errorf("column rating: expected no comment, got %+v", got)
	}
	if got := athenaAll(t, dbmeta.Columns, m, db, dbmeta.Args{Parent: "shipment"}.Map()); len(got) != 4 {
		t.Errorf("expected the four columns of shipment, got %+v", got)
	}
}

// TestAthenaPartitionedTables reads the partition columns. A Hive table has
// them and an Iceberg table does not, because COLUMNS marks only the first.
func TestAthenaPartitionedTables(t *testing.T) {
	db := openAthena(t)
	m := setupAthena(t, db)

	var got []string
	for _, v := range athenaAll(t, dbmeta.PartitionedTables, m, db, nil) {
		if v.Schema != athfixture.Everything.Schema || v.Type != "table" || v.Strategy != "list" || v.Parent != "" {
			t.Errorf("partitioned table %s: expected a list partition of a table in dbmeta, got %+v", v.Name, v)
		}
		got = append(got, v.Name+"."+v.Expression)
		if v.Expression == "sold_on" && v.Comment.V != "the day" {
			t.Errorf("sold_on: expected its comment, got %+v", v.Comment)
		}
	}
	if strings.Join(got, ",") != "sales.sold_on,sales.region" {
		t.Errorf("expected the two partition columns of sales and nothing for ticket, got %v", got)
	}
}

// TestAthenaCurrent reads the schema and the user of the session.
func TestAthenaCurrent(t *testing.T) {
	db := openAthena(t)
	m := setupAthena(t, db)

	schema, ok, err := dbmeta.First(dbmeta.CurrentSchema.All(t.Context(), m, db, nil))
	if err != nil || !ok {
		t.Fatalf("reading the current schema: %v", err)
	}
	if schema.Catalog != "awsdatacatalog" || schema.Name != athfixture.Everything.Schema {
		t.Errorf("expected awsdatacatalog and dbmeta, got %+v", schema)
	}
	user, ok, err := dbmeta.First(dbmeta.CurrentUser.All(t.Context(), m, db, nil))
	if err != nil || !ok {
		t.Fatalf("reading the current user: %v", err)
	}
	if user.Name == "" || user.Session.Valid {
		t.Errorf("expected a user and no session user, got %+v", user)
	}

	dbs := athenaAll(t, dbmeta.Databases, m, db, nil)
	if len(dbs) != 1 || dbs[0].Name != "awsdatacatalog" {
		t.Errorf("expected the one catalog awsdatacatalog, got %+v", dbs)
	}
	schemas := athenaAll(t, dbmeta.Schemas, m, db, nil)
	var found bool
	for _, s := range schemas {
		if s.Name == "information_schema" {
			t.Errorf("expected no information_schema without with_system, got %+v", s)
		}
		found = found || s.Name == athfixture.Everything.Schema
	}
	if !found {
		t.Errorf("expected the Glue database dbmeta, got %+v", schemas)
	}
}

// TestAthenaFilterLiterals checks that a quote and a backslash in a filter reach
// Athena as the value that the caller wrote. The driver binds with the
// ExecutionParameters of the service, and an earlier driver wrote a backslash
// before each of them, which Athena reads as part of the value. See D222 and D229.
func TestAthenaFilterLiterals(t *testing.T) {
	db := openAthena(t)
	m := setupAthena(t, db)

	for _, name := range []string{"it's", `back\slash`, `x'' OR 1=1 --`, "100%", "a?b"} {
		if got := athenaAll(t, dbmeta.Tables, m, db, dbmeta.Args{Name: name}.Map()); len(got) != 0 {
			t.Errorf("filter %q: expected no table, got %+v", name, got)
		}
	}
	// A comment that was never set is a NULL, and the model must read it as one.
	columns := athenaAll(t, dbmeta.Columns, m, db, dbmeta.Args{Parent: "author", Name: "rating"}.Map())
	if len(columns) != 1 || columns[0].Comment.Valid {
		t.Errorf("expected one column with no comment, got %+v", columns)
	}
}

// TestAthenaUnanswered checks the sources that the model does not read, so that
// a change on the service shows. docs/COVERAGE.md says why each is out.
func TestAthenaUnanswered(t *testing.T) {
	db := openAthena(t)
	m := setupAthena(t, db)

	for name, stmt := range map[string]string{
		"table_privileges":  "SELECT * FROM information_schema.table_privileges",
		"roles":             "SELECT * FROM information_schema.roles",
		"system.metadata":   "SELECT * FROM system.metadata.table_comments",
		"system.jdbc":       "SELECT * FROM system.jdbc.tables",
		"show catalogs":     "SHOW CATALOGS",
		"show session":      "SHOW SESSION",
		"routines":          "SELECT * FROM information_schema.routines",
		"table_constraints": "SELECT * FROM information_schema.table_constraints",
	} {
		if err := athenaDrain(t.Context(), db, stmt); err == nil {
			t.Errorf("%s: expected Athena to refuse it, and the model reads nothing there", name)
		}
	}
	// A table has no comment in TABLES, and SHOW TBLPROPERTIES holds it.
	rows, err := db.QueryContext(t.Context(), "SHOW TBLPROPERTIES dbmeta.author")
	if err != nil {
		t.Fatalf("SHOW TBLPROPERTIES: %v", err)
	}
	defer rows.Close()
	// The service answers one text for each property, with a tab between the key
	// and the value, and the driver of Uber split the text. The driver of dbimp
	// returns it as the service sent it. See D229.
	var comment bool
	for rows.Next() {
		var line, rest sql.NullString
		if err := rows.Scan(&line, &rest); err != nil {
			t.Fatalf("reading the properties: %v", err)
		}
		comment = comment || line.String == "comment\tpeople who write books"
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("reading the properties: %v", err)
	}
	if !comment {
		t.Error("expected SHOW TBLPROPERTIES to hold the comment of the table author, which no SELECT reads")
	}
	for _, q := range []dbmeta.AnyQuery{dbmeta.Comments, dbmeta.Roles, dbmeta.Privileges, dbmeta.Functions, dbmeta.Types, dbmeta.Partitions, dbmeta.Constraints, dbmeta.Indexes} {
		if got := q.Support(m); got != dbmeta.NotSupported {
			t.Errorf("%s: expected NotSupported, got %v", q.Name(), got)
		}
	}
}
