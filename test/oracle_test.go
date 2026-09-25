package test

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"strings"
	"testing"

	_ "github.com/sijms/go-ora/v2"

	"github.com/xo/dbmeta"
	_ "github.com/xo/dbmeta/models/oracle"
	orfixture "github.com/xo/dbmeta/models/oracle/fixture"
)

// setupOracle builds the fixture and returns the metadata for the server.
//
// It tears down first, because a previous run that failed part way leaves the
// user behind and CREATE USER then fails rather than the test reporting what
// actually went wrong.
func setupOracle(t *testing.T, db *sql.DB) *dbmeta.Meta {
	t.Helper()
	ctx := t.Context()
	versions := mustVersion(t, db)

	run := func(steps []orfixture.Result, what string) (int, int) {
		t.Helper()
		var ran, skipped int
		for _, step := range steps {
			if step.Skipped {
				skipped++
				continue
			}
			if _, err := db.ExecContext(ctx, step.Query); err != nil {
				t.Fatalf("%s %s: %v\n%s", what, step.Name, err, step.Query)
			}
			ran++
		}
		return ran, skipped
	}

	down, err := orfixture.Everything.ResolveTeardown(versions)
	if err != nil {
		t.Fatalf("resolving the teardown: %v", err)
	}
	for _, step := range down {
		if !step.Skipped {
			//nolint:errcheck // a teardown before setup is best effort
			db.ExecContext(ctx, step.Query)
		}
	}

	up, err := orfixture.Everything.ResolveSetup(versions)
	if err != nil {
		t.Fatalf("resolving the setup: %v", err)
	}
	ran, skipped := run(up, "setup")
	t.Cleanup(func() {
		for _, step := range down {
			if !step.Skipped {
				//nolint:errcheck // the test has already reported what matters
				db.ExecContext(context.WithoutCancel(ctx), step.Query)
			}
		}
	})

	m, err := dbmeta.New(dbmeta.Oracle, versions)
	if err != nil {
		t.Fatalf("building the metadata: %v", err)
	}
	t.Logf("server reports %s, fixture ran %d steps and skipped %d", versions, ran, skipped)
	return m
}

func openOracle(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("DBMETA_ORACLE")
	if dsn == "" {
		t.Skip("set DBMETA_ORACLE to run against a real server")
	}
	db, err := sql.Open("oracle", dsn)
	if err != nil {
		t.Fatalf("opening: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.PingContext(t.Context()); err != nil {
		t.Fatalf("connecting: %v", err)
	}
	return db
}

// TestOracleVersion reads the banner and checks what the model makes of it.
//
// The banner is the only version source every release from 11g up has.
// product_component_version.version_full is more precise and is a parse error
// before 18c, and v$version.banner_full likewise. The banner also carries the
// name Oracle sells the release under, which nothing else does.
func TestOracleVersion(t *testing.T) {
	db := openOracle(t)
	versions, err := dbmeta.Oracle.Version(t.Context(), db)
	if err != nil {
		t.Fatalf("reading the version: %v", err)
	}
	if versions.Main().Unknown {
		t.Fatal("expected a version")
	}
	// 11 is 11g Release 2, the oldest release with a free image.
	if got := versions.Main().Parts[0]; got < 11 {
		t.Errorf("expected release 11 or newer, got %d", got)
	}
	display := versions.String()
	if !strings.Contains(display, "Oracle") || !strings.Contains(display, versions.Main().Raw) {
		t.Errorf("expected the product and the build in %q", display)
	}
	t.Logf("server reports %s", display)
}

// oraArgs is the filter the fixture's objects sit behind.
func oraArgs() map[string]any {
	return dbmeta.Args{Schema: "DBMETA_FIXTURE"}.Map()
}

// TestOracleSmoke runs each registered query and reports what came back. It is
// a scaffold for building the model, not a claim about correctness.
func TestOracleSmoke(t *testing.T) {
	db := openOracle(t)
	ctx := t.Context()
	m := setupOracle(t, db)
	var schemas int
	for v, err := range dbmeta.Schemas.All(ctx, m, db, nil) {
		if err != nil {
			t.Fatalf("reading schemas: %v", err)
		}
		schemas++
		_ = v
	}
	t.Logf("schemas: %d", schemas)

	var tables int
	for v, err := range dbmeta.Tables.All(ctx, m, db, nil) {
		if err != nil {
			t.Fatalf("reading tables: %v", err)
		}
		tables++
		_ = v
	}
	t.Logf("tables and views: %d", tables)

	// The fixture's own objects, which is what the queries are for.
	var mine int
	for v, err := range dbmeta.Tables.All(ctx, m, db, oraArgs()) {
		if err != nil {
			t.Fatalf("reading the fixture tables: %v", err)
		}
		mine++
		t.Logf("  %s.%s %s comment=%v", v.Schema, v.Name, v.Type, v.Comment.V)
	}
	if mine == 0 {
		t.Error("expected the fixture objects")
	}

	// Columns, which is where Oracle's LONG default and its 12c identity
	// column both show up.
	var cols int
	for v, err := range dbmeta.Columns.All(ctx, m, db, oraArgs()) {
		if err != nil {
			t.Fatalf("reading columns: %v", err)
		}
		cols++
		if v.Table == "AUTHOR" || v.Table == "EXTRAS" {
			t.Logf("  %s.%-12s %-14s null=%v pk=%v default=%q identity=%v generated=%v",
				v.Table, v.Name, v.DataType, v.Nullable, v.PrimaryKey,
				v.Default.V, v.Identity.V, v.Generated.V)
		}
	}
	if cols == 0 {
		t.Error("expected columns")
	}

	// Every query the model registers, run against the fixture. This is the
	// hard rule 9 check: a query that has never run is not finished.
	var ran, tooOld, unsupported int
	for _, q := range dbmeta.Queries() {
		switch q.Support(m) {
		case dbmeta.NotBuilt, dbmeta.NotSupported:
			unsupported++
			continue
		case dbmeta.TooOld:
			// The product has the object and this release does not, which
			// Support says on its own since D54 was answered.
			tooOld++
			continue
		case dbmeta.Supported:
		}
		// nil rather than a filter, so every query takes its own defaults.
		// Passing a schema to a query that declares no schema parameter is
		// ErrUnknownParam, which the framework refuses on purpose.
		query, vals, err := q.Build(m, nil)
		if errors.Is(err, dbmeta.ErrVersionTooOld) {
			tooOld++
			continue
		}
		if err != nil {
			t.Errorf("%s: building: %v", q.Name(), err)
			continue
		}
		// columnsOf runs it and closes the rows, which is why this loop does
		// not hold one open across an iteration.
		cols, err := columnsOf(t, db, query, vals)
		if err != nil {
			t.Errorf("%s: %v\n%s", q.Name(), err, query)
			continue
		}
		fields, ferr := q.Fields(m)
		if ferr != nil {
			t.Errorf("%s: reading fields: %v", q.Name(), ferr)
			continue
		}
		// The column set is the contract. A query that returns a different
		// number than it declares will not scan.
		if len(cols) != len(fields) {
			t.Errorf("%s: declares %d fields and returns %d columns",
				q.Name(), len(fields), len(cols))
		}
		ran++
	}
	t.Logf("%d queries ran, %d too old, %d not supported", ran, tooOld, unsupported)
}

func mustVersion(t *testing.T, db *sql.DB) dbmeta.VersionSet {
	t.Helper()
	v, err := dbmeta.Oracle.Version(t.Context(), db)
	if err != nil {
		t.Fatalf("reading the version: %v", err)
	}
	return v
}
