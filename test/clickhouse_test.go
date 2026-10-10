package test

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"

	_ "github.com/xo/dbimp/clickhouse"

	"github.com/xo/dbmeta"
	_ "github.com/xo/dbmeta/models/clickhouse"
	chfixture "github.com/xo/dbmeta/models/clickhouse/fixture"
)

// openClickHouse returns a connection to the server named by DBMETA_CLICKHOUSE.
func openClickHouse(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("DBMETA_CLICKHOUSE")
	if dsn == "" {
		t.Skip("set DBMETA_CLICKHOUSE to run against a real server")
	}
	db, err := sql.Open("clickhouse", dsn)
	if err != nil {
		t.Fatalf("opening: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.PingContext(t.Context()); err != nil {
		t.Fatalf("connecting: %v", err)
	}
	return db
}

// setupClickHouse builds the fixture and returns the metadata for the server.
//
// It tears down first, because a run that failed part way leaves the database
// behind and CREATE DATABASE then fails rather than the test reporting what
// actually went wrong.
func setupClickHouse(t *testing.T, db *sql.DB) *dbmeta.Meta {
	t.Helper()
	ctx := t.Context()
	versions, err := dbmeta.ClickHouse.Version(ctx, db)
	if err != nil {
		t.Fatalf("reading the version: %v", err)
	}

	down, err := chfixture.Everything.ResolveTeardown(versions)
	if err != nil {
		t.Fatalf("resolving the teardown: %v", err)
	}
	for _, step := range down {
		if !step.Skipped {
			//nolint:errcheck // a teardown before setup is best effort
			db.ExecContext(ctx, step.Query)
		}
	}

	up, err := chfixture.Everything.ResolveSetup(versions)
	if err != nil {
		t.Fatalf("resolving the setup: %v", err)
	}
	var ran, skipped int
	for _, step := range up {
		if step.Skipped {
			skipped++
			continue
		}
		if _, err := db.ExecContext(ctx, step.Query); err != nil {
			t.Fatalf("setup %s: %v\n%s", step.Name, err, step.Query)
		}
		ran++
	}
	t.Cleanup(func() {
		for _, step := range down {
			if !step.Skipped {
				//nolint:errcheck // the test has already reported what matters
				db.ExecContext(context.WithoutCancel(ctx), step.Query)
			}
		}
	})

	m, err := dbmeta.New(dbmeta.ClickHouse, versions)
	if err != nil {
		t.Fatalf("building the metadata: %v", err)
	}
	t.Logf("server reports %s, fixture ran %d steps and skipped %d", versions, ran, skipped)
	return m
}

// chArgs is the filter the fixture's objects sit behind.
func chArgs() map[string]any {
	return dbmeta.Args{Schema: chfixture.Everything.Schema}.Map()
}

// TestClickHouseVersion reads the version and checks what the model makes of it.
func TestClickHouseVersion(t *testing.T) {
	db := openClickHouse(t)
	versions, err := dbmeta.ClickHouse.Version(t.Context(), db)
	if err != nil {
		t.Fatalf("reading the version: %v", err)
	}
	if versions.Main().Unknown {
		t.Fatal("expected a version")
	}
	// 25 is the floor, which is the oldest release whose image is rebuilt.
	if got := versions.Main().Parts[0]; got < 25 {
		t.Errorf("expected release 25 or newer, got %d", got)
	}
	if display := versions.String(); !strings.Contains(display, "ClickHouse") {
		t.Errorf("expected the product in %q", display)
	}
	t.Logf("server reports %s", versions)
}

// TestClickHouseSmoke runs each registered query against the fixture.
func TestClickHouseSmoke(t *testing.T) {
	db := openClickHouse(t)
	ctx := t.Context()
	m := setupClickHouse(t, db)

	var mine int
	for v, err := range dbmeta.Tables.All(ctx, m, db, chArgs()) {
		if err != nil {
			t.Fatalf("reading tables: %v", err)
		}
		mine++
		t.Logf("  %s.%s %s comment=%q", v.Schema, v.Name, v.Type, v.Comment.V)
	}
	if mine == 0 {
		t.Error("expected the fixture tables")
	}

	var cols int
	for v, err := range dbmeta.Columns.All(ctx, m, db, chArgs()) {
		if err != nil {
			t.Fatalf("reading columns: %v", err)
		}
		cols++
		if v.Table == "author" {
			t.Logf("  author.%-11s %-18s null=%v pk=%v default=%q generated=%q",
				v.Name, v.DataType, v.Nullable, v.PrimaryKey, v.Default.V, v.Generated.V)
		}
	}
	if cols == 0 {
		t.Error("expected the fixture columns")
	}

	// Every query the model registers. This is the hard rule 9 check: a query
	// that has never run is not finished.
	var ran, tooOld, unsupported int
	for _, q := range dbmeta.Queries() {
		switch q.Support(m) {
		case dbmeta.NotBuilt, dbmeta.NotSupported:
			unsupported++
			continue
		case dbmeta.TooOld:
			tooOld++
			continue
		case dbmeta.Supported:
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
		fields, ferr := q.Fields(m)
		if ferr != nil {
			t.Errorf("%s: reading fields: %v", q.Name(), ferr)
			continue
		}
		if len(cs) != len(fields) {
			t.Errorf("%s: declares %d fields and returns %d columns",
				q.Name(), len(fields), len(cs))
		}
		ran++
	}
	t.Logf("%d queries ran, %d too old, %d not supported", ran, tooOld, unsupported)
}

// TestClickHouseFixtureObjects checks that the fixture built one of every
// object the queries read, which is what hard rule 9 asks for.
func TestClickHouseFixtureObjects(t *testing.T) {
	db := openClickHouse(t)
	ctx := t.Context()
	m := setupClickHouse(t, db)
	schema := chfixture.Everything.Schema

	count := func(name string, seq func() (int, error)) {
		t.Helper()
		n, err := seq()
		if err != nil {
			t.Fatalf("reading %s: %v", name, err)
		}
		if n == 0 {
			t.Errorf("the fixture built no %s", name)
			return
		}
		t.Logf("%-18s %d", name, n)
	}

	count("views", func() (int, error) {
		n := 0
		for v, err := range dbmeta.Views.All(ctx, m, db, chArgs()) {
			if err != nil {
				return 0, err
			}
			if v.Schema == schema {
				n++
			}
		}
		return n, nil
	})
	count("indexes", func() (int, error) {
		n := 0
		for v, err := range dbmeta.Indexes.All(ctx, m, db, chArgs()) {
			if err != nil {
				return 0, err
			}
			if v.Schema == schema {
				n++
			}
		}
		return n, nil
	})
	count("index columns", func() (int, error) {
		n := 0
		for v, err := range dbmeta.IndexColumns.All(ctx, m, db, chArgs()) {
			if err != nil {
				return 0, err
			}
			if v.Schema == schema {
				n++
			}
		}
		return n, nil
	})
	count("partitioned tables", func() (int, error) {
		n := 0
		for v, err := range dbmeta.PartitionedTables.All(ctx, m, db, chArgs()) {
			if err != nil {
				return 0, err
			}
			if v.Schema == schema {
				n++
			}
		}
		return n, nil
	})
	count("comments", func() (int, error) {
		n := 0
		for v, err := range dbmeta.Comments.All(ctx, m, db, chArgs()) {
			if err != nil {
				return 0, err
			}
			if v.Schema == schema {
				n++
			}
		}
		return n, nil
	})
	count("foreign tables", func() (int, error) {
		n := 0
		for v, err := range dbmeta.ForeignTables.All(ctx, m, db, chArgs()) {
			if err != nil {
				return 0, err
			}
			if v.Schema == schema {
				n++
			}
		}
		return n, nil
	})
	count("roles", func() (int, error) {
		n := 0
		for v, err := range dbmeta.Roles.All(ctx, m, db, nil) {
			if err != nil {
				return 0, err
			}
			if strings.HasPrefix(v.Name, "dbmeta_") {
				n++
			}
		}
		return n, nil
	})
	count("role grants", func() (int, error) {
		n := 0
		for v, err := range dbmeta.RoleGrants.All(ctx, m, db, nil) {
			if err != nil {
				return 0, err
			}
			if strings.HasPrefix(v.Role, "dbmeta_") {
				n++
			}
		}
		return n, nil
	})
	count("privileges", func() (int, error) {
		n := 0
		for v, err := range dbmeta.Privileges.All(ctx, m, db, chArgs()) {
			if err != nil {
				return 0, err
			}
			if v.Schema.V == schema {
				n++
			}
		}
		return n, nil
	})
	// Constraints needs a server new enough to have system.constraints.
	if dbmeta.Constraints.Support(m) == dbmeta.Supported {
		count("constraints", func() (int, error) {
			n := 0
			for v, err := range dbmeta.Constraints.All(ctx, m, db, chArgs()) {
				if err != nil {
					return 0, err
				}
				if v.Schema == schema {
					n++
				}
			}
			return n, nil
		})
	} else {
		t.Logf("constraints    skipped: %v", dbmeta.Constraints.Support(m))
	}

	// Foreign servers are not counted. A named collection needs
	// access_control_improvements.named_collection_control, which the image
	// exposes no variable for, so the fixture builds none. The query is still
	// run by the smoke test and returns no rows. See docs/COVERAGE.md.
	var servers int
	for _, err := range dbmeta.ForeignServers.All(ctx, m, db, nil) {
		if err != nil {
			t.Fatalf("reading foreign servers: %v", err)
		}
		servers++
	}
	t.Logf("%-18s %d, and the fixture builds none on purpose", "foreign servers", servers)
}

// TestClickHouseDescribeFields reads the fields that D198 and D199 added, for
// the ones ClickHouse has a source for (D208). It has no owner, no storage
// choice and no statistics target, and it keeps the row policy and the
// partitions in tables that an ordinary user is refused.
func TestClickHouseDescribeFields(t *testing.T) {
	db := openClickHouse(t)
	ctx := t.Context()
	m := setupClickHouse(t, db)

	tables := map[string]dbmeta.Table{}
	for v, err := range dbmeta.Tables.All(ctx, m, db, chArgs()) {
		if err != nil {
			t.Fatalf("reading tables: %v", err)
		}
		tables[v.Name] = v
		if v.Owner.Valid || v.RowSecurity.Valid {
			t.Errorf("%s: expected no owner and no row security, got %+v", v.Name, v)
		}
		if !v.AccessMethod.Valid || v.AccessMethod.V == "" {
			t.Errorf("%s: expected the engine, got %+v", v.Name, v.AccessMethod)
		}
	}
	sales := tables["sales"]
	if sales.AccessMethod.V != "MergeTree" || sales.Persistence.V != "permanent" {
		t.Errorf("sales: expected a permanent MergeTree table, got %+v", sales)
	}
	if !sales.Rows.Valid || sales.Rows.V != 3 || !sales.Size.Valid || sales.Size.V <= 0 {
		t.Errorf("sales: expected 3 rows and a size, got rows %+v and size %+v", sales.Rows, sales.Size)
	}
	if want := "partition_by=toYYYYMM(sold_on), order_by=sold_on, primary_key=sold_on, storage_policy=default"; sales.Options.V != want {
		t.Errorf("sales: expected the options %q, got %q", want, sales.Options.V)
	}
	if recent := tables["recent"]; recent.Persistence.Valid || recent.Options.Valid {
		t.Errorf("recent: a view has no persistence and no options, got %+v", recent)
	}
	if remote := tables["remote"]; remote.AccessMethod.V != "URL" || remote.Options.Valid {
		t.Errorf("remote: expected the URL engine and no options, got %+v", remote)
	}

	codecs := map[string]string{}
	for v, err := range dbmeta.Columns.All(ctx, m, db, dbmeta.Args{Schema: chfixture.Everything.Schema, Parent: "sales"}.Map()) {
		if err != nil {
			t.Fatalf("reading columns: %v", err)
		}
		if v.Storage.Valid || v.StatsTarget.Valid {
			t.Errorf("%s: expected no storage and no statistics target, got %+v", v.Name, v)
		}
		codecs[v.Name] = v.Compression.V
	}
	if codecs["region"] != "CODEC(ZSTD(3))" || codecs["amount"] != "CODEC(Delta(8), LZ4)" || codecs["sold_on"] != "" {
		t.Errorf("expected the codecs of sales, got %v", codecs)
	}

	for v, err := range dbmeta.Indexes.All(ctx, m, db, chArgs()) {
		if err != nil {
			t.Fatalf("reading indexes: %v", err)
		}
		if v.Name == "book_title_idx" {
			if v.Options.V != "granularity=1" || v.Using.V != "set(100)" || !v.Size.Valid {
				t.Errorf("book_title_idx: expected granularity, using and size, got %+v", v)
			}
		}
	}

	if dbmeta.Constraints.Support(m) == dbmeta.Supported {
		enforced := map[string]bool{}
		for v, err := range dbmeta.Constraints.All(ctx, m, db, chArgs()) {
			if err != nil {
				t.Fatalf("reading constraints: %v", err)
			}
			if !v.Enforced.Valid {
				t.Errorf("%s: expected enforced to be read", v.Name)
			}
			enforced[v.Name] = v.Enforced.V
		}
		if !enforced["sales_amount_ck"] || enforced["sales_amount_hint"] {
			t.Errorf("expected the check to be enforced and the assume not to be, got %v", enforced)
		}
	}

	bounds := map[string]string{}
	for v, err := range dbmeta.Partitions.All(ctx, m, db, dbmeta.Args{Schema: chfixture.Everything.Schema, Parent: "sales"}.Map()) {
		if err != nil {
			t.Fatalf("reading partitions: %v", err)
		}
		if v.Table != "sales" || v.Type != "partition" || v.Partitioned || v.DetachPending {
			t.Errorf("unexpected partition %+v", v)
		}
		bounds[v.Partition] = v.Bound.V
	}
	if len(bounds) != 2 || bounds["202501"] != "202501" || bounds["202502"] != "202502" {
		t.Errorf("expected the partitions 202501 and 202502, got %v", bounds)
	}

	var partitioned int
	for v, err := range dbmeta.PartitionedTables.All(ctx, m, db, chArgs()) {
		if err != nil {
			t.Fatalf("reading partitioned tables: %v", err)
		}
		partitioned++
		if v.Name == "sales" && (v.AccessMethod.V != "MergeTree" || v.Table.Valid ||
			v.DirectSize.V != tables["sales"].Size.V || v.TotalSize.V != v.DirectSize.V) {
			t.Errorf("sales: expected the engine and the size of the table, got %+v", v)
		}
	}
	if partitioned < 2 {
		t.Errorf("expected sales and book to be partitioned, got %d", partitioned)
	}

	var policies int
	for v, err := range dbmeta.Policies.All(ctx, m, db, chArgs()) {
		if err != nil {
			t.Fatalf("reading policies: %v", err)
		}
		policies++
		if v.Name != "book_recent" || v.Table != "book" || v.Command != "select" || !v.Permissive ||
			v.Roles.V != "dbmeta_reader" || v.Using.V != "book_id > 0" || v.WithCheck.Valid {
			t.Errorf("unexpected policy %+v", v)
		}
		// system.row_policies has no column for a switch on any release, so
		// the answer is unknown and not true (D211)
		if v.Enabled.Valid {
			t.Errorf("expected no enabled state, got %+v", v.Enabled)
		}
	}
	if policies != 1 {
		t.Errorf("expected the policy book_recent, got %d", policies)
	}
}
