package test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"iter"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	_ "github.com/xo/dbimp/druid"

	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/container"
	_ "github.com/xo/dbmeta/models/druid"
	drfixture "github.com/xo/dbmeta/models/druid/fixture"
)

// openDruid returns a connection to the server named by DBMETA_DRUID, which is
// the druid:// URL that github.com/xo/dbimp/druid takes, and which dburl's
// druid scheme opens (D154, D171).
func openDruid(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("DBMETA_DRUID")
	if dsn == "" {
		t.Skip("set DBMETA_DRUID to run against a real server")
	}
	db, err := sql.Open("druid", dsn)
	if err != nil {
		t.Fatalf("opening: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.PingContext(t.Context()); err != nil {
		t.Fatalf("connecting: %v", err)
	}
	return db
}

// druidCall sends one request to the Router and returns the answer.
func druidCall(ctx context.Context, dsn, method, path, body string) ([]byte, error) {
	u, err := url.Parse(dsn)
	if err != nil {
		return nil, fmt.Errorf("parsing the dsn: %w", err)
	}
	password, _ := u.User.Password()
	// The DSN is druid://user:password@host:port, and the Router answers HTTP
	// on the same port.
	req, err := http.NewRequestWithContext(ctx, method, "http://"+u.Host+path, strings.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("building %s %s: %w", method, path, err)
	}
	req.SetBasicAuth(u.User.Username(), password)
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sending %s %s: %w", method, path, err)
	}
	defer res.Body.Close()
	out, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("reading the answer to %s %s: %w", method, path, err)
	}
	if res.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("%s %s answered %d: %s", method, path, res.StatusCode, out)
	}
	return out, nil
}

// druidTask runs one ingestion task through the task API and waits until it
// ends. The nano quickstart has two slots, and a task needs both, so a task
// never starts while another runs.
func druidTask(ctx context.Context, dsn, query string) error {
	body, err := json.Marshal(map[string]any{"query": query})
	if err != nil {
		return fmt.Errorf("encoding the task: %w", err)
	}
	var id string
	// The Router answers 503 for a short time when the host is busy, and the
	// task API refuses a task that names it, so the submit tries again.
	for try := 0; ; try++ {
		out, err := druidCall(ctx, dsn, http.MethodPost, "/druid/v2/sql/task", string(body))
		if err == nil {
			var r struct {
				TaskID string `json:"taskId"`
			}
			if err := json.Unmarshal(out, &r); err != nil {
				return fmt.Errorf("reading the task id from %s: %w", out, err)
			}
			id = r.TaskID
			break
		}
		if try == 5 {
			return fmt.Errorf("submitting the task: %w", err)
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("submitting the task: %w", ctx.Err())
		case <-time.After(5 * time.Second):
		}
	}
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("waiting for task %s: %w", id, ctx.Err())
		case <-time.After(2 * time.Second):
		}
		out, err := druidCall(ctx, dsn, http.MethodGet, "/druid/indexer/v1/task/"+id+"/status", "")
		if err != nil {
			// A busy Router answers 503 for a moment. Ask again.
			continue
		}
		var r struct {
			Status struct {
				Code  string `json:"statusCode"`
				Error string `json:"errorMsg"`
			} `json:"status"`
		}
		if err := json.Unmarshal(out, &r); err != nil {
			return fmt.Errorf("reading the state of task %s from %s: %w", id, out, err)
		}
		switch r.Status.Code {
		case "SUCCESS":
			return nil
		case "FAILED":
			return fmt.Errorf("task %s failed: %s", id, r.Status.Error)
		}
	}
}

// druidHas reports whether a datasource answers a query.
func druidHas(ctx context.Context, db *sql.DB, name string) bool {
	var n int
	err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM INFORMATION_SCHEMA.TABLES WHERE TABLE_SCHEMA = 'druid' AND TABLE_NAME = ?`, name).Scan(&n)
	return err == nil && n > 0
}

// setupDruid builds the fixture as the administrator and returns the metadata
// for the server. A task takes from 5 to 12 seconds, so a step is skipped when
// its datasource is already there, and a second run waits for nothing. Druid
// has no DDL and nothing here drops a datasource.
func setupDruid(t *testing.T, db *sql.DB) *dbmeta.Meta {
	t.Helper()
	ctx := t.Context()
	dsn := os.Getenv("DBMETA_DRUID")
	versions, err := dbmeta.Druid.Version(ctx, db)
	if err != nil {
		t.Fatalf("reading the version: %v", err)
	}
	var ran, skipped int
	for _, step := range drfixture.Everything.Setup {
		if druidHas(ctx, db, step.Name) {
			skipped++
			continue
		}
		if err := druidTask(ctx, dsn, step.Query); err != nil {
			t.Fatalf("setup %s: %v", step.Name, err)
		}
		// The rows answer a query some seconds after the task ends.
		deadline := time.Now().Add(2 * time.Minute)
		for !druidHas(ctx, db, step.Name) {
			if time.Now().After(deadline) {
				t.Fatalf("setup %s: the datasource never answered", step.Name)
			}
			time.Sleep(2 * time.Second)
		}
		ran++
	}
	m, err := dbmeta.New(dbmeta.Druid, versions)
	if err != nil {
		t.Fatalf("building the metadata: %v", err)
	}
	t.Logf("server reports %s, fixture ran %d steps and skipped %d", versions, ran, skipped)
	return m
}

// TestDruidVersion reads the version and checks what the model makes of it.
func TestDruidVersion(t *testing.T) {
	db := openDruid(t)
	versions, err := dbmeta.Druid.Version(t.Context(), db)
	if err != nil {
		t.Fatalf("reading the version: %v", err)
	}
	if main := versions.Main(); main.Unknown || main.Parts[0] < 37 {
		t.Errorf("expected release 37 or newer, got %s", main)
	}
	if !strings.HasPrefix(versions.String(), "Apache Druid 3") {
		t.Errorf("expected the display line to name Apache Druid, got %q", versions)
	}
	t.Logf("server reports %s", versions)
}

// TestDruidVersionRefusedToAnOrdinaryUser checks that a user who cannot read
// sys.servers gets an error that says so, and not an unknown version. Druid
// answers HTTP 403 with Insufficient permission to view servers.
func TestDruidVersionRefusedToAnOrdinaryUser(t *testing.T) {
	dsn := os.Getenv("DBMETA_DRUID")
	if dsn == "" {
		t.Skip("set DBMETA_DRUID to run against a real server")
	}
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("parsing %s: %v", dsn, err)
	}
	u.User = url.UserPassword(container.DruidUser, container.Password)
	db, err := sql.Open("druid", u.String())
	if err != nil {
		t.Fatalf("opening: %v", err)
	}
	defer db.Close()
	_, err = dbmeta.Druid.Version(t.Context(), db)
	if err == nil {
		t.Fatal("expected the ordinary user to be refused the version")
	}
	if !strings.Contains(err.Error(), "Insufficient permission") && !strings.Contains(err.Error(), "403") {
		t.Errorf("expected a refusal that names the permission, got %v", err)
	}
}

// TestDruidEveryQueryRuns runs every query the model answers and checks that
// the columns of each row are the declared fields, by name and in order.
func TestDruidEveryQueryRuns(t *testing.T) {
	db := openDruid(t)
	m := setupDruid(t, db)
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
	if ran != 7 {
		t.Errorf("%d queries ran, and the model answers 7: change this test with the model", ran)
	}
}

// TestDruidScansEveryQuery reads every query through its own Scan, with the
// system objects included.
func TestDruidScansEveryQuery(t *testing.T) {
	db := openDruid(t)
	scanEveryQuery(t, setupDruid(t, db), db)
}

// TestDruidFixtureObjects reads the fixture back through the typed API.
func TestDruidFixtureObjects(t *testing.T) {
	db := openDruid(t)
	m := setupDruid(t, db)
	ctx := t.Context()
	fx := drfixture.Everything

	tables := map[string]string{}
	for v, err := range dbmeta.Tables.All(ctx, m, db, dbmeta.Args{Schema: fx.Schema}.Map()) {
		if err != nil {
			t.Fatalf("reading tables: %v", err)
		}
		if v.Catalog != "druid" || v.Schema != fx.Schema {
			t.Errorf("table %s: catalog %q and schema %q, want druid and %q", v.Name, v.Catalog, v.Schema, fx.Schema)
		}
		tables[v.Name] = v.Type
	}
	for _, name := range []string{"author", "book", "region", "shipment"} {
		if tables[name] != "table" {
			t.Errorf("expected %s as a table, got %q", name, tables[name])
		}
	}

	// A system table is listed with with_system alone, and as a system table.
	sys := map[string]string{}
	for v, err := range dbmeta.Tables.All(ctx, m, db, dbmeta.Args{Schema: "sys", WithSystem: true}.Map()) {
		if err != nil {
			t.Fatalf("reading the system tables: %v", err)
		}
		sys[v.Name] = v.Type
	}
	if sys["segments"] != "system table" {
		t.Errorf("expected sys.segments as a system table, got %v", sys)
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
	for table, want := range map[string]map[string][2]string{
		"author":   {"__time": {"1", "TIMESTAMP"}, "author_id": {"2", "BIGINT"}, "name": {"3", "VARCHAR"}, "rating": {"4", "BIGINT"}, "shade": {"5", "VARCHAR"}},
		"shipment": {"weight": {"6", "DOUBLE"}, "fragile": {"7", "BIGINT"}},
		"book":     {"published": {"5", "VARCHAR"}},
	} {
		for name, w := range want {
			c, ok := cols[table][name]
			if !ok || strconv.Itoa(c.Ordinal) != w[0] || c.DataType != w[1] {
				t.Errorf("%s.%s: want ordinal %s and type %s, got %+v", table, name, w[0], w[1], c)
			}
		}
	}
	if c := cols["author"]["__time"]; c.Nullable {
		t.Errorf("expected the time column NOT NULL, got %+v", c)
	}
	if c := cols["author"]["rating"]; !c.Nullable || c.Default.Valid || c.PrimaryKey {
		t.Errorf("expected author.rating nullable with no default and no key, got %+v", c)
	}

	var aggs int
	for v, err := range dbmeta.Aggregates.All(ctx, m, db, dbmeta.Args{Name: "SUM", WithSystem: true}.Map()) {
		if err != nil {
			t.Fatalf("reading aggregates: %v", err)
		}
		if v.Kind != "agg" || !v.ArgTypes.Valid {
			t.Errorf("expected SUM as an aggregate with its signatures, got %+v", v)
		}
		aggs++
	}
	if aggs != 1 {
		t.Errorf("expected one SUM among the aggregates, got %d", aggs)
	}
	if n := countRows(t, dbmeta.Functions.All(ctx, m, db, dbmeta.Args{Name: "SUM"}.Map())); n != 0 {
		t.Errorf("expected no function without with_system, got %d", n)
	}

	s, ok, err := dbmeta.First(dbmeta.CurrentSchema.All(ctx, m, db, nil))
	if err != nil || !ok || s.Name != "druid" || s.Catalog != "druid" {
		t.Errorf("expected the current schema druid, got %+v, %v, %v", s, ok, err)
	}

	set, ok, err := dbmeta.First(dbmeta.Settings.All(ctx, m, db, dbmeta.Args{Name: "druid.auth.authenticatorChain"}.Map()))
	if err != nil || !ok || !strings.Contains(set.Value.V, "basic") || !set.Context.Valid {
		t.Errorf("expected the authenticator chain from the entry, got %+v, %v, %v", set, ok, err)
	}
}

// TestDruidLeavesOut checks the kinds the model does not answer. Druid has no
// DDL, so it has no index, constraint, trigger, sequence or view to list, and
// its users are in an HTTP API.
func TestDruidLeavesOut(t *testing.T) {
	m, err := dbmeta.New(dbmeta.Druid, dbmeta.VersionSet{})
	if err != nil {
		t.Fatal(err)
	}
	for _, q := range []dbmeta.AnyQuery{
		dbmeta.Indexes, dbmeta.IndexColumns, dbmeta.Constraints, dbmeta.Triggers, dbmeta.Sequences,
		dbmeta.Views, dbmeta.Databases, dbmeta.Roles, dbmeta.Privileges, dbmeta.CurrentUser,
		dbmeta.RoutineParameters, dbmeta.PartitionedTables, dbmeta.Comments,
	} {
		if s := q.Support(m); s != dbmeta.NotSupported {
			t.Errorf("%s: %v, want NotSupported", q.Name(), s)
		}
	}
	if _, _, err := dbmeta.Indexes.Build(m, nil); !errors.Is(err, dbmeta.ErrNotSupported) {
		t.Errorf("indexes: expected ErrNotSupported, got %v", err)
	}
}

// countRows reads a result to its end and returns how many rows it held.
func countRows[T any](t *testing.T, rows iter.Seq2[T, error]) int {
	t.Helper()
	var n int
	for _, err := range rows {
		if err != nil {
			t.Fatalf("reading rows: %v", err)
		}
		n++
	}
	return n
}
