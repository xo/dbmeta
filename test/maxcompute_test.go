package test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/xo/dbmeta"
	_ "github.com/xo/dbmeta/models/maxcompute"
	mcfixture "github.com/xo/dbmeta/models/maxcompute/fixture"
)

// openMaxCompute returns a connection to the project named by DBMETA_MAXCOMPUTE,
// which dbrun resolves from the places D117 names. The value holds a secret and
// no test prints it.
//
// The driver is github.com/aliyun/aliyun-odps-go-sdk/sqldriver, which dburl
// names for the maxcompute scheme (D154). The tests open it through
// retryDriverName, which retries a reset connection. See D237 and D239.
func openMaxCompute(t *testing.T) *sql.DB {
	t.Helper()
	raw := os.Getenv("DBMETA_MAXCOMPUTE")
	if raw == "" {
		t.Skip("set DBMETA_MAXCOMPUTE to run against the service")
	}
	return openAt(t, retryDriverName, raw)
}

// maxComputeState is the one fixture that every MaxCompute test shares.
//
// MaxCompute runs each statement as a job that takes a second or more, and
// INFORMATION_SCHEMA shows a change after a delay, so a fixture for each test is
// more than the tests are worth. The first test builds it, every test reads it,
// and TestMain drops it when the last test ends. No test changes it.
var maxComputeState struct {
	mu    sync.Mutex
	meta  *dbmeta.Meta
	err   error
	built bool
}

// runMaxComputeSteps runs the statements one at a time. A step of a teardown
// that fails is not an error when quiet is true, because the object is already
// gone. A step that says its object is already there is not an error either, because the
// fixture stays up between runs. See D240.
func runMaxComputeSteps(ctx context.Context, db *sql.DB, steps []mcfixture.Result, quiet bool) error {
	for _, s := range steps {
		_, err := db.ExecContext(ctx, s.Query)
		if err != nil && !quiet && !strings.Contains(err.Error(), "already") {
			return fmt.Errorf("%s: %w\n%s", s.Name, err, s.Query)
		}
	}
	return nil
}

// shutdownMaxCompute drops the fixture, if a test built it. TestMain calls it.
func shutdownMaxCompute() {
	maxComputeState.mu.Lock()
	defer maxComputeState.mu.Unlock()
	if !maxComputeState.built {
		return
	}
	// The fixture stays up unless DBMETA_MAXCOMPUTE_DROP is set, because
	// INFORMATION_SCHEMA takes about an hour to show a new object, and a run that
	// drops the fixture waits that long at the next run. See D240.
	if os.Getenv("DBMETA_MAXCOMPUTE_DROP") == "" {
		return
	}
	db, err := sql.Open(retryDriverName, os.Getenv("DBMETA_MAXCOMPUTE"))
	if err != nil {
		fmt.Fprintln(os.Stderr, "dropping the MaxCompute fixture: opening the driver failed")
		return
	}
	defer db.Close()
	down, err := mcfixture.Everything.ResolveTeardown(dbmeta.VersionSet{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "dropping the MaxCompute fixture: %v\n", err)
		return
	}
	//nolint:errcheck // a teardown is best effort, and quiet says so
	runMaxComputeSteps(context.Background(), db, down, true)
}

// waitMaxCompute waits until INFORMATION_SCHEMA shows the last table that the
// setup makes. The views show a change after a delay, so a query that ran
// right after the setup reads the project as it was.
func waitMaxCompute(ctx context.Context, m *dbmeta.Meta, db *sql.DB, limit time.Duration) error {
	deadline := time.Now().Add(limit)
	for {
		var found int
		for _, err := range dbmeta.Tables.All(ctx, m, db, dbmeta.Args{Name: "ticket"}.Map()) {
			if err != nil {
				return fmt.Errorf("waiting for the fixture: %w", err)
			}
			found++
		}
		if found > 0 {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("waiting for the fixture: INFORMATION_SCHEMA did not show the table ticket within %s", limit)
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("waiting for the fixture: %w", ctx.Err())
		case <-time.After(10 * time.Second):
		}
	}
}

// setupMaxCompute builds the fixture, once, and returns the metadata for the
// service.
//
// It builds nothing when INFORMATION_SCHEMA already shows the last table of the
// fixture. Otherwise it runs every step, and a step that finds its object there
// is not an error. See D240 for how long INFORMATION_SCHEMA takes to show the
// objects.
func setupMaxCompute(t *testing.T, db *sql.DB) *dbmeta.Meta {
	t.Helper()
	maxComputeState.mu.Lock()
	defer maxComputeState.mu.Unlock()
	if maxComputeState.err != nil {
		t.Fatalf("the fixture failed to build in an earlier test: %v", maxComputeState.err)
	}
	if maxComputeState.meta != nil {
		return maxComputeState.meta
	}
	ctx := context.WithoutCancel(t.Context())
	fail := func(format string, args ...any) {
		maxComputeState.err = fmt.Errorf(format, args...)
		t.Fatal(maxComputeState.err)
	}
	versions, err := dbmeta.MaxCompute.Version(ctx, db)
	if err != nil {
		fail("reading the version: %v", err)
	}
	m, err := dbmeta.New(dbmeta.MaxCompute, versions)
	if err != nil {
		fail("building the metadata: %v", err)
	}
	// DBMETA_MAXCOMPUTE_WAIT is how long a run waits for INFORMATION_SCHEMA to
	// show the fixture that the run just made.
	wait := 3 * time.Minute
	if d, err := time.ParseDuration(os.Getenv("DBMETA_MAXCOMPUTE_WAIT")); err == nil {
		wait = d
	}
	// The last table that the setup makes says that the fixture is whole, and a
	// fixture that is whole needs no step. Every step is safe to run again.
	if err := waitMaxCompute(ctx, m, db, 0); err == nil {
		maxComputeState.meta = m
		return m
	}
	up, err := mcfixture.Everything.ResolveSetup(versions)
	if err != nil {
		fail("resolving the setup: %v", err)
	}
	maxComputeState.built = true
	if err := runMaxComputeSteps(ctx, db, up, false); err != nil {
		fail("setup: %v", err)
	}
	if err := waitMaxCompute(ctx, m, db, wait); err != nil {
		fail("%v. The views show a new object after about an hour, so run the tests again later", err)
	}
	t.Logf("server reports %s, fixture ran %d steps", versions, len(up))
	maxComputeState.meta = m
	return m
}

// collectMaxCompute reads every row a query answers.
func collectMaxCompute[T any](t *testing.T, q *dbmeta.Query[T], m *dbmeta.Meta, db *sql.DB, args map[string]any) ([]T, error) {
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

// maxComputeAll reads every row a query answers, and stops the test on an error.
func maxComputeAll[T any](t *testing.T, q *dbmeta.Query[T], m *dbmeta.Meta, db *sql.DB, args map[string]any) []T {
	t.Helper()
	out, err := collectMaxCompute(t, q, m, db, args)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// maxComputeDrain runs a statement and reads every row, and returns the error
// that either step gave.
func maxComputeDrain(ctx context.Context, db *sql.DB, stmt string) error {
	rows, err := db.QueryContext(ctx, stmt)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
	}
	return rows.Err()
}

// maxComputeColumns runs the statement and returns the names of its columns.
func maxComputeColumns(t *testing.T, db *sql.DB, query string, vals []any) ([]string, error) {
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

// TestMaxComputeVersion checks the text that version() returns and what the
// model makes of it. See D237.
func TestMaxComputeVersion(t *testing.T) {
	t.Parallel()
	db := openMaxCompute(t)
	versions, err := dbmeta.MaxCompute.Version(t.Context(), db)
	if err != nil {
		t.Fatalf("reading the version: %v", err)
	}
	if !strings.HasPrefix(versions.Display, "MaxCompute V") {
		t.Errorf("expected a display that names the build, got %q", versions.Display)
	}
	if versions.Main().IsZero() || versions.Main().Unknown {
		t.Errorf("expected the sprint as the main version, got %s", versions)
	}
	// The service needs the semicolon, and the statement of the model has it.
	if err := maxComputeDrain(t.Context(), db, "SELECT version()"); err == nil {
		t.Error("SELECT version() without a semicolon: expected the service to refuse it")
	}
	t.Logf("server reports %s", versions)
}

// TestMaxComputeSmoke runs each registered query against the fixture and checks
// that it returns the columns it declares.
func TestMaxComputeSmoke(t *testing.T) {
	t.Parallel()
	db := openMaxCompute(t)
	m := setupMaxCompute(t, db)

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
		if len(vals) != 0 {
			t.Errorf("%s: expected a statement with no bound value, got %d", q.Name(), len(vals))
		}
		cs, err := maxComputeColumns(t, db, query, vals)
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
	if ran != 15 {
		t.Errorf("expected the 15 queries the package comment names to run, %d ran", ran)
	}
	t.Logf("%d queries ran, %d not supported", ran, unsupported)
}

// TestMaxComputeTables reads the relations of the fixture back through the typed
// API.
func TestMaxComputeTables(t *testing.T) {
	t.Parallel()
	db := openMaxCompute(t)
	m := setupMaxCompute(t, db)

	tables := map[string]dbmeta.Table{}
	for _, v := range maxComputeAll(t, dbmeta.Tables, m, db, dbmeta.Args{Schema: mcfixture.Everything.Schema}.Map()) {
		tables[v.Name] = v
		if v.Catalog != "odps" || v.Schema != mcfixture.Everything.Schema {
			t.Errorf("table %s: expected the catalog odps and the project dbmeta, got %q and %q", v.Name, v.Catalog, v.Schema)
		}
		if v.Rows.Valid || v.RowSecurity.Valid || v.RowSecurityForced.Valid || v.AccessMethod.Valid {
			t.Errorf("table %s: expected no rows, row security or access method, got %+v", v.Name, v)
		}
	}
	for name, typ := range map[string]string{
		"author": "table", "book": "table", "region": "table", "shipment": "table",
		"sales": "table", "ticket": "table", "recent": "view", "book_titles": "materialized view",
	} {
		got, ok := tables[name]
		if !ok || got.Type != typ {
			t.Errorf("table %s: expected type %q, got %+v", name, typ, got)
		}
	}
	if got := tables["author"]; got.Comment.V != "people who write books" || !got.Owner.Valid || got.Persistence.V != "permanent" {
		t.Errorf("table author: expected its comment, an owner and permanent, got %+v", got)
	}
	if got := tables["author"]; !got.Size.Valid || got.Size.V != 0 {
		t.Errorf("table author: expected the size 0 of an empty table, got %+v", got.Size)
	}
	if got := tables["book"]; got.Comment.Valid {
		t.Errorf("table book: expected no comment, got %+v", got.Comment)
	}
	if got := tables["recent"]; got.Size.Valid || got.Persistence.Valid {
		t.Errorf("view recent: expected no size and no persistence, got %+v", got)
	}
	if got := tables["sales"]; got.Size.Valid {
		t.Errorf("table sales: expected no size for a partitioned table, got %+v", got.Size)
	}
	// A transactional table is clustered by hash into 16 buckets.
	if got := tables["ticket"]; got.Options.V != "cluster_type=hash, number_buckets=16" {
		t.Errorf("table ticket: expected its clustering, got %+v", got.Options)
	}

	// The filters.
	if got := maxComputeAll(t, dbmeta.Tables, m, db, dbmeta.Args{Name: "auth%"}.Map()); len(got) != 1 || got[0].Name != "author" {
		t.Errorf("expected author alone, got %+v", got)
	}
	for _, v := range maxComputeAll(t, dbmeta.Tables, m, db, dbmeta.Args{Types: []string{"view"}}.Map()) {
		if v.Type != "view" {
			t.Errorf("expected a view, got %+v", v)
		}
	}
	if got := maxComputeAll(t, dbmeta.Tables, m, db, dbmeta.Args{Schema: "no_such_project"}.Map()); len(got) != 0 {
		t.Errorf("expected no table in a project that does not exist, got %+v", got)
	}

	// Views.
	views := map[string]dbmeta.View{}
	for _, v := range maxComputeAll(t, dbmeta.Views, m, db, nil) {
		views[v.Name] = v
	}
	if v := views["recent"]; !strings.Contains(v.Definition.V, "published IS NOT NULL") || v.Comment.V != "recent books" {
		t.Errorf("expected the view recent with its text and comment, got %+v", v)
	}
	if v := views["book_titles"]; !strings.Contains(v.Definition.V, "SELECT title FROM book") {
		t.Errorf("expected the materialized view book_titles, got %+v", v)
	}
	if _, ok := views["author"]; ok {
		t.Error("expected no table among the views")
	}
	for name, v := range views {
		if v.Updatable.V || v.Insertable.V || v.CheckOption.Valid {
			t.Errorf("view %s: expected no update and no check option, got %+v", name, v)
		}
	}

	comments := map[string]string{}
	for _, v := range maxComputeAll(t, dbmeta.Comments, m, db, nil) {
		comments[v.Name] = v.Type + ": " + v.Comment
	}
	if comments["author"] != "table: people who write books" || comments["recent"] != "view: recent books" {
		t.Errorf("expected the comments of author and recent, got %v", comments)
	}
	if _, ok := comments["book"]; ok {
		t.Error("expected no comment for book, which has none")
	}
}

// TestMaxComputeColumns reads the columns of the fixture.
func TestMaxComputeColumns(t *testing.T) {
	t.Parallel()
	db := openMaxCompute(t)
	m := setupMaxCompute(t, db)

	columns := map[string]dbmeta.Column{}
	for _, v := range maxComputeAll(t, dbmeta.Columns, m, db, nil) {
		columns[v.Table+"."+v.Name] = v
	}
	type want struct {
		ordinal  int
		dataType string
		nullable bool
	}
	for name, w := range map[string]want{
		"author.author_id":  {1, "bigint", false},
		"author.name":       {2, "string", false},
		"author.rating":     {3, "int", true},
		"author.shade":      {4, "string", true},
		"book.published":    {4, "date", true},
		"region.country":    {1, "string", false},
		"shipment.amount":   {4, "int", false},
		"recent.title":      {2, "string", true},
		"book_titles.title": {1, "string", true},
		"ticket.ticket_id":  {1, "bigint", false},
		"ticket.note":       {2, "string", true},
		"sales.amount":      {1, "int", false},
		"sales.body":        {2, "string", true},
		"sales.sold_on":     {3, "string", false},
		"sales.region":      {4, "string", false},
	} {
		got, ok := columns[name]
		if !ok {
			t.Errorf("column %s: not found", name)
			continue
		}
		if got.Ordinal != w.ordinal || got.DataType != w.dataType || got.Nullable != w.nullable {
			t.Errorf("column %s: expected ordinal %d, type %q and nullable %t, got %+v", name, w.ordinal, w.dataType, w.nullable, got)
		}
		if got.Catalog != "odps" || got.Schema != mcfixture.Everything.Schema || got.PrimaryKey || got.Identity.Valid || got.Generated.Valid || got.Collation.Valid {
			t.Errorf("column %s: expected no key, identity, generation or collation, got %+v", name, got)
		}
	}
	if got := columns["author.shade"].Default; got.V != "red" || !got.Valid {
		t.Errorf("column shade: expected the default red, got %+v", got)
	}
	if got := columns["author.name"].Default; got.Valid {
		t.Errorf("column name: expected no default, got %+v", got)
	}
	if got := columns["author.name"].Comment; got.V != "the full name" {
		t.Errorf("column name: expected its comment, got %+v", got)
	}
	// COLUMNS holds the empty text where there is no comment.
	if got := columns["author.rating"].Comment; !got.Valid || got.V != "" {
		t.Errorf("column rating: expected the empty comment that COLUMNS holds, got %+v", got)
	}
	if got := columns["sales.sold_on"].Comment; got.V != "the day" {
		t.Errorf("column sold_on: expected its comment, got %+v", got)
	}
	// A filter on the name does not change an ordinal.
	if got := maxComputeAll(t, dbmeta.Columns, m, db, dbmeta.Args{Parent: "sales", Name: "region"}.Map()); len(got) != 1 || got[0].Ordinal != 4 {
		t.Errorf("expected the one column region at 4, got %+v", got)
	}
	if got := maxComputeAll(t, dbmeta.Columns, m, db, dbmeta.Args{Parent: "shipment"}.Map()); len(got) != 4 {
		t.Errorf("expected the four columns of shipment, got %+v", got)
	}
}

// TestMaxComputePartitions reads the partitioned tables and the partitions.
func TestMaxComputePartitions(t *testing.T) {
	t.Parallel()
	db := openMaxCompute(t)
	m := setupMaxCompute(t, db)

	var got []string
	for _, v := range maxComputeAll(t, dbmeta.PartitionedTables, m, db, dbmeta.Args{Name: "sales"}.Map()) {
		if v.Schema != mcfixture.Everything.Schema || v.Type != "table" || v.Strategy != "list" || v.Parent != "" || v.Owner == "" {
			t.Errorf("partitioned table %s: expected a list partition of a table in dbmeta, got %+v", v.Name, v)
		}
		got = append(got, v.Name+"."+v.Expression)
		if v.Expression == "sold_on" && v.Comment.V != "the day" {
			t.Errorf("sold_on: expected its comment, got %+v", v.Comment)
		}
	}
	if strings.Join(got, ",") != "sales.sold_on,sales.region" {
		t.Errorf("expected the two partition columns of sales, got %v", got)
	}
	for _, v := range maxComputeAll(t, dbmeta.PartitionedTables, m, db, nil) {
		if v.Name == "ticket" || v.Name == "author" {
			t.Errorf("expected no partition column for %s, got %+v", v.Name, v)
		}
	}
	parts := maxComputeAll(t, dbmeta.Partitions, m, db, dbmeta.Args{Parent: "sales"}.Map())
	if len(parts) != 1 || parts[0].Partition != "sold_on=2026-01-01/region=north" || parts[0].Table != "sales" || parts[0].Type != "partition" {
		t.Fatalf("expected the one partition of sales, got %+v", parts)
	}
	if p := parts[0]; p.Partitioned || p.DetachPending || p.Constraint.Valid {
		t.Errorf("partition: expected the fixed values, got %+v", p)
	}
}

// TestMaxComputeRoles reads the users, the roles, the memberships and the
// privileges.
func TestMaxComputeRoles(t *testing.T) {
	t.Parallel()
	db := openMaxCompute(t)
	m := setupMaxCompute(t, db)

	roles := map[string]dbmeta.Role{}
	var users int
	for _, v := range maxComputeAll(t, dbmeta.Roles, m, db, nil) {
		roles[v.Name] = v
		if v.CanLogin {
			users++
			if v.Comment.Valid {
				t.Errorf("user %s: expected no comment, got %+v", v.Name, v.Comment)
			}
		}
		if v.CreateRole || v.CreateDB || v.Replication || v.BypassRLS || !v.Inherit || v.ConnLimit != -1 || v.ValidUntil.Valid {
			t.Errorf("role %s: expected the fixed attributes, got %+v", v.Name, v)
		}
	}
	if users < 2 {
		t.Errorf("expected the administrator and the reader among the users, got %d", users)
	}
	for _, name := range []string{"admin", "reader", "super_administrator", "report_reader"} {
		if got, ok := roles[name]; !ok || got.CanLogin {
			t.Errorf("role %s: expected a role that cannot sign in, got %+v", name, got)
		}
	}
	if got := roles["admin"].Comment.V; got != "reserved by the system" {
		t.Errorf("role admin: expected its comment, got %q", got)
	}
	if got := roles["report_reader"].Comment; !got.Valid || got.V != "" {
		t.Errorf("role report_reader: expected the empty comment that ROLES holds, got %+v", got)
	}
	if !roles["super_administrator"].Superuser || roles["report_reader"].Superuser {
		t.Error("expected super_administrator to be a superuser and report_reader not to be")
	}
	var administrators int
	for _, v := range roles {
		if v.CanLogin && v.MemberOf == "admin" {
			administrators++
		}
	}
	if administrators != 1 {
		t.Errorf("expected one user that holds the role admin, got %d", administrators)
	}

	grants := maxComputeAll(t, dbmeta.RoleGrants, m, db, nil)
	var admin bool
	for _, g := range grants {
		if g.MemberOf == "admin" && strings.HasPrefix(g.Role, "RAM$") {
			admin = true
		}
		if g.Grantor.Valid || g.Admin || !g.Inherit || !g.Set {
			t.Errorf("role grant %+v: expected the fixed values", g)
		}
	}
	if !admin {
		t.Errorf("expected a RAM user that holds the role admin, got %+v", grants)
	}

	// TABLE_PRIVILEGES lists users and never roles, so the grant to report_reader
	// is in no row, and the owner of a table is in its rows.
	privileges := map[string]dbmeta.Privilege{}
	for _, v := range maxComputeAll(t, dbmeta.Privileges, m, db, nil) {
		privileges[v.Name] = v
		if v.ColumnAccess.Valid || v.Policies.Valid {
			t.Errorf("privilege %s: expected no column access and no policy, got %+v", v.Name, v)
		}
		if strings.Contains(v.Access.V, "report_reader") {
			t.Errorf("privilege %s: expected no role among the grantees, got %q", v.Name, v.Access.V)
		}
	}
	if got := privileges["author"]; got.Type != "table" || !strings.Contains(got.Access.V, "=select") {
		t.Errorf("privileges of author: expected the privileges of its users, got %+v", got)
	}
	if got := privileges["book_titles"]; got.Type != "materialized view" {
		t.Errorf("privileges of book_titles: expected the materialized view, got %+v", got)
	}
	if got := maxComputeAll(t, dbmeta.ColumnPrivileges, m, db, nil); len(got) != 0 {
		t.Errorf("expected no column privilege, because a grant to a role is in no row, got %+v", got)
	}
}

// TestMaxComputeFunctions reads the functions.
func TestMaxComputeFunctions(t *testing.T) {
	t.Parallel()
	db := openMaxCompute(t)
	m := setupMaxCompute(t, db)

	got := maxComputeAll(t, dbmeta.Functions, m, db, nil)
	if len(got) != 1 || got[0].Name != "double_it" || got[0].Kind != "function" || got[0].Catalog != "odps" || got[0].Schema != mcfixture.Everything.Schema {
		t.Fatalf("expected the function double_it, got %+v", got)
	}
	f := got[0]
	if !f.Owner.Valid || f.ResultType.Valid || f.ArgTypes.Valid || f.Source.Valid || f.Definition.Valid || f.Comment.Valid || f.Language != "" {
		t.Errorf("function double_it: expected an owner and nothing else, because UDFS holds no more, got %+v", f)
	}
	if !strings.Contains(f.Access.V, "=execute") {
		t.Errorf("function double_it: expected the privileges of its users, got %+v", f.Access)
	}
}

// TestMaxComputeCurrent reads the schema and the user of the session.
func TestMaxComputeCurrent(t *testing.T) {
	t.Parallel()
	db := openMaxCompute(t)
	m := setupMaxCompute(t, db)

	schema, ok, err := dbmeta.First(dbmeta.CurrentSchema.All(t.Context(), m, db, nil))
	if err != nil || !ok {
		t.Fatalf("reading the current schema: %v", err)
	}
	if schema.Catalog != "odps" || schema.Name != mcfixture.Everything.Schema {
		t.Errorf("expected odps and dbmeta, got %+v", schema)
	}
	user, ok, err := dbmeta.First(dbmeta.CurrentUser.All(t.Context(), m, db, nil))
	if err != nil || !ok {
		t.Fatalf("reading the current user: %v", err)
	}
	if !strings.HasPrefix(user.Name, "p") || user.Session.Valid {
		t.Errorf("expected the id of a user and no session user, got %+v", user)
	}

	dbs := maxComputeAll(t, dbmeta.Databases, m, db, nil)
	if len(dbs) != 1 || dbs[0].Name != "odps" {
		t.Errorf("expected the one database odps, got %+v", dbs)
	}
	if got := maxComputeAll(t, dbmeta.Databases, m, db, dbmeta.Args{Name: "x%"}.Map()); len(got) != 0 {
		t.Errorf("expected no database that starts with x, got %+v", got)
	}
	schemas := maxComputeAll(t, dbmeta.Schemas, m, db, nil)
	if len(schemas) != 1 || schemas[0].Name != mcfixture.Everything.Schema || schemas[0].Catalog != "odps" {
		t.Errorf("expected the one schema dbmeta, got %+v", schemas)
	}
}

// TestMaxComputeFilterLiterals checks that a quote and a backslash in a filter
// reach MaxCompute as the value that the caller wrote. The driver writes a
// parameter into the statement with no quote, so the dialect writes each literal
// itself. See D238.
func TestMaxComputeFilterLiterals(t *testing.T) {
	t.Parallel()
	db := openMaxCompute(t)
	m := setupMaxCompute(t, db)

	for _, name := range []string{"it's", `back\slash`, `x'' OR 1=1 --`, "100%", "a?b", "@name"} {
		if got := maxComputeAll(t, dbmeta.Tables, m, db, dbmeta.Args{Name: name}.Map()); len(got) != 0 {
			t.Errorf("filter %q: expected no table, got %+v", name, got)
		}
	}
	// The underscore is a wildcard and a backslash escapes it, as in a LIKE.
	if got := maxComputeAll(t, dbmeta.Tables, m, db, dbmeta.Args{Name: `book\_titles`}.Map()); len(got) != 1 {
		t.Errorf("expected the table book_titles for an escaped underscore, got %+v", got)
	}
}

// TestMaxComputeUnanswered checks the sources that the model does not read, so
// that a change on the service shows. docs/COVERAGE.md says why each is out.
func TestMaxComputeUnanswered(t *testing.T) {
	t.Parallel()
	db := openMaxCompute(t)
	m := setupMaxCompute(t, db)

	ctx := t.Context()
	// A statement that is not a SELECT returns no row through the driver.
	for name, stmt := range map[string]string{
		"show tables":         "SHOW TABLES;",
		"desc":                "DESC author;",
		"list functions":      "LIST FUNCTIONS;",
		"current_user":        "SELECT current_user();",
		"user":                "SELECT user();",
		"without a semicolon": "SELECT 1",
	} {
		if err := maxComputeDrain(ctx, db, stmt); err == nil {
			t.Errorf("%s: expected the service or the driver to refuse it", name)
		}
	}
	// The views that the model leaves out do not hold what a kind needs.
	for name, stmt := range map[string]string{
		"views":     "SELECT * FROM information_schema.views;",
		"schemata":  "SELECT * FROM information_schema.schemata;",
		"routines":  "SELECT * FROM information_schema.routines;",
		"functions": "SELECT * FROM information_schema.functions;",
	} {
		if err := maxComputeDrain(ctx, db, stmt); err == nil {
			t.Errorf("%s: expected MaxCompute to say the view does not exist", name)
		}
	}
	// A transactional table has a primary key, and no column of COLUMNS says so.
	rows, err := db.QueryContext(ctx, "SELECT * FROM information_schema.columns WHERE table_name = 'ticket';")
	if err != nil {
		t.Fatalf("reading the columns of ticket: %v", err)
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		t.Fatalf("reading the columns of COLUMNS: %v", err)
	}
	for _, c := range cols {
		if strings.Contains(c, "key") && c != "is_partition_key" {
			t.Errorf("expected COLUMNS to hold no column for a primary key, got %q", c)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("reading the rows of COLUMNS: %v", err)
	}
	for _, q := range []dbmeta.AnyQuery{dbmeta.Indexes, dbmeta.Constraints, dbmeta.ConstraintColumns, dbmeta.Sequences, dbmeta.Triggers, dbmeta.Types, dbmeta.RoutineParameters, dbmeta.Policies, dbmeta.ForeignTables, dbmeta.NotNulls} {
		if got := q.Support(m); got != dbmeta.NotSupported {
			t.Errorf("%s: expected NotSupported, got %v", q.Name(), got)
		}
	}
}
