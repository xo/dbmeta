package test

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/xo/dbmeta"
	_ "github.com/xo/dbmeta/models/redshift"
	rsfixture "github.com/xo/dbmeta/models/redshift/fixture"
)

// openRedshift returns a connection to the service named by DBMETA_REDSHIFT, which
// dbrun resolves from the places D117 names. The model was written before
// an account was provisioned, and these tests are what finish it (D144).
func openRedshift(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("DBMETA_REDSHIFT")
	if dsn == "" {
		t.Skip("set DBMETA_REDSHIFT to run against the service")
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

// setupRedshift builds the fixture and returns the metadata for the service.
func setupRedshift(t *testing.T, db *sql.DB) *dbmeta.Meta {
	t.Helper()
	ctx := t.Context()
	versions, err := dbmeta.Redshift.Version(ctx, db)
	if err != nil {
		t.Fatalf("reading the version: %v", err)
	}
	down, err := rsfixture.Everything.ResolveTeardown(versions)
	if err != nil {
		t.Fatalf("resolving the teardown: %v", err)
	}
	for _, step := range down {
		if !step.Skipped {
			//nolint:errcheck // a teardown before setup is best effort
			db.ExecContext(ctx, step.Query)
		}
	}
	up, err := rsfixture.Everything.ResolveSetup(versions)
	if err != nil {
		t.Fatalf("resolving the setup: %v", err)
	}
	for _, step := range up {
		if step.Skipped {
			continue
		}
		if _, err := db.ExecContext(ctx, step.Query); err != nil {
			if step.SkipWhen != "" && strings.Contains(err.Error(), step.SkipWhen) {
				t.Logf("skipping setup %s: %v", step.Name, err)
				continue
			}
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
	m, err := dbmeta.New(dbmeta.Redshift, versions)
	if err != nil {
		t.Fatalf("building the metadata: %v", err)
	}
	t.Logf("service reports %s", versions)
	return m
}

// TestRedshiftSmoke runs each query and checks the columns match the fields.
func TestRedshiftSmoke(t *testing.T) {
	db := openRedshift(t)
	m := setupRedshift(t, db)
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
	}
}

// TestRedshiftScansEveryQuery reads every query through its own Scan.
func TestRedshiftScansEveryQuery(t *testing.T) {
	db := openRedshift(t)
	scanEveryQuery(t, setupRedshift(t, db), db)
}

// TestRedshiftTableFields reads the fields that D198 and D199 added to Tables,
// Columns, Functions and Schemas (D207), and the default privilege that has no
// schema (D197). A temporary table lives as long as its session, so the test
// keeps one connection.
func TestRedshiftTableFields(t *testing.T) {
	db := openRedshift(t)
	db.SetMaxOpenConns(1)
	ctx := t.Context()
	m := setupRedshift(t, db)
	exec(t, db, `CREATE TEMPORARY TABLE dbmeta_scratch (id INTEGER)`)

	args := dbmeta.Args{Schema: rsfixture.Everything.Schema}.Map()
	tables := map[string]dbmeta.Table{}
	for v, err := range dbmeta.Tables.All(ctx, m, db, args) {
		if err != nil {
			t.Fatalf("reading tables: %v", err)
		}
		tables[v.Name] = v
	}
	author := tables["author"]
	if !author.Owner.Valid || author.Owner.V == "" || author.Persistence.V != "permanent" {
		t.Errorf("author: expected an owner and permanent, got %+v and %+v", author.Owner, author.Persistence)
	}
	if recent := tables["recent"]; !recent.Owner.Valid || recent.Size.Valid || recent.Rows.Valid || recent.Options.Valid {
		t.Errorf("recent: expected an owner and no size, rows or options, got %+v", recent)
	}

	// SVV_TABLE_INFO lists a table only when it holds a row, and the fixture
	// gives two of them one (D212). The size is the blocks of 1 MB, so it is a
	// multiple of 1048576.
	events := tables["events"]
	if !events.Size.Valid || events.Size.V <= 0 || events.Size.V%1048576 != 0 ||
		!events.Rows.Valid || events.Rows.V != 1 ||
		events.Options.V != "diststyle=KEY(event_id), sortkey=happened" {
		t.Errorf("events: expected a size in blocks, one row and the options, got %+v", events)
	}
	if author.Size.V <= 0 || author.Rows.V != 1 || !strings.HasPrefix(author.Options.V, "diststyle=") {
		t.Errorf("author: expected a size, one row and the options, got %+v", author)
	}
	if book := tables["book"]; book.Size.Valid || book.Rows.Valid || book.Options.Valid {
		t.Errorf("book: an empty table is not in SVV_TABLE_INFO, so expected no size, rows or options, got %+v", book)
	}

	encodings := map[string]string{}
	for v, err := range dbmeta.Columns.All(ctx, m, db, dbmeta.Args{Schema: rsfixture.Everything.Schema, Parent: "events"}.Map()) {
		if err != nil {
			t.Fatalf("reading columns: %v", err)
		}
		encodings[v.Name] = v.Compression.V
		if v.Storage.Valid || v.StatsTarget.Valid {
			t.Errorf("%s: Redshift has no storage or statistics target, got %+v", v.Name, v)
		}
	}
	for name, want := range map[string]string{"event_id": "az64", "happened": "raw", "kind": "lzo"} {
		if encodings[name] != want {
			t.Errorf("events.%s: expected the encoding %s, got %q", name, want, encodings[name])
		}
	}
	for v, err := range dbmeta.Columns.All(ctx, m, db, dbmeta.Args{Schema: rsfixture.Everything.Schema, Parent: "recent"}.Map()) {
		if err != nil {
			t.Fatalf("reading columns: %v", err)
		}
		if v.Compression.Valid {
			t.Errorf("recent.%s: a view column has no encoding, got %+v", v.Name, v.Compression)
		}
	}

	var temporary bool
	for v, err := range dbmeta.Tables.All(ctx, m, db, dbmeta.Args{Name: "dbmeta_scratch", WithSystem: true}.Map()) {
		if err != nil {
			t.Fatalf("reading tables: %v", err)
		}
		temporary = v.Persistence.V == "temporary"
	}
	if !temporary {
		t.Error("dbmeta_scratch: expected a temporary table")
	}

	functions := map[string]dbmeta.Function{}
	for v, err := range dbmeta.Functions.All(ctx, m, db, dbmeta.Args{Schema: rsfixture.Everything.Schema}.Map()) {
		if err != nil {
			t.Fatalf("reading functions: %v", err)
		}
		functions[v.Name] = v
	}
	if shout := functions["f_shout"]; !strings.Contains(shout.Prosrc.V, "UPPER") || shout.Prosrc != shout.Source {
		t.Errorf("f_shout: expected the body in prosrc and in source, got %+v and %+v", shout.Prosrc, shout.Source)
	}
	for v, err := range dbmeta.Schemas.All(ctx, m, db, dbmeta.Args{Name: rsfixture.Everything.Schema}.Map()) {
		if err != nil {
			t.Fatalf("reading schemas: %v", err)
		}
		if v.Name == rsfixture.Everything.Schema && v.Access.Valid {
			t.Errorf("%s: expected the default privileges and no access text, got %q", v.Name, v.Access.V)
		}
	}

	// A default privilege made with no schema has no schema, and the
	// field says so.
	var everywhere bool
	for v, err := range dbmeta.DefaultACLs.All(ctx, m, db, nil) {
		if err != nil {
			t.Fatalf("reading default privileges: %v", err)
		}
		everywhere = everywhere || !v.Schema.Valid
	}
	if !everywhere {
		t.Error("expected a default privilege with no schema")
	}
}
