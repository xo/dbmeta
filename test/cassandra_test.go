package test

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"strings"
	"testing"

	_ "github.com/xo/cassandra"

	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/models/cassandra"
	cafixture "github.com/xo/dbmeta/models/cassandra/fixture"
)

// openCassandra returns a connection to the server named by DBMETA_CASSANDRA.
//
// The DSN is a cassandra:// URL with the options as query keys, which is the
// one form that both the old and the new driver read. dbrun writes it. See
// D195.
func openCassandra(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("DBMETA_CASSANDRA")
	if dsn == "" {
		t.Skip("set DBMETA_CASSANDRA to run against a real server")
	}
	db, err := sql.Open("cassandra", dsn)
	if err != nil {
		t.Fatalf("opening: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.PingContext(t.Context()); err != nil {
		t.Fatalf("connecting: %v", err)
	}
	return db
}

// setupCassandra builds the fixture and returns the metadata for the server.
//
// It tears down first, because a run that failed part way leaves the keyspace
// behind and CREATE KEYSPACE then fails rather than the test reporting what
// actually went wrong.
func setupCassandra(t *testing.T, db *sql.DB) *dbmeta.Meta {
	t.Helper()
	ctx := t.Context()
	versions, err := dbmeta.Cassandra.Version(ctx, db)
	if err != nil {
		t.Fatalf("reading the version: %v", err)
	}

	down, err := cafixture.Everything.ResolveTeardown(versions)
	if err != nil {
		t.Fatalf("resolving the teardown: %v", err)
	}
	for _, step := range down {
		if !step.Skipped {
			//nolint:errcheck // a teardown before setup is best effort
			db.ExecContext(ctx, step.Query)
		}
	}

	up, err := cafixture.Everything.ResolveSetup(versions)
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
			// A refused function, view or role means the server is the
			// published image rather than the one this repository builds.
			// Say so, because the statement alone does not.
			t.Fatalf("setup %s: %v\n%s\n\n%s", step.Name, err, step.Query, imageHint)
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

	m, err := dbmeta.New(dbmeta.Cassandra, versions)
	if err != nil {
		t.Fatalf("building the metadata: %v", err)
	}
	t.Logf("server reports %s, fixture ran %d steps and skipped %d", versions, ran, skipped)
	return m
}

// imageHint says what to do about a setup that was refused.
const imageHint = "The Apache image refuses a user defined function, a" +
	" materialized view and a role. Build the image this repository makes:" +
	" cd test && go run ./cmd/dbrun build cassandra"

// caArgs is the filter the fixture's objects sit behind.
//
// The model narrows every result to it, in the Keep function of each binding,
// which D200 explains. TestCassandraFilters checks each filter on its own.
func caArgs() map[string]any {
	return dbmeta.Args{Schema: cafixture.Everything.Schema}.Map()
}

// TestCassandraVersion reads the three versions and checks what the model
// makes of them.
//
// Cassandra is the only database here that reports more than one. The release
// is the main version, because that is what a fragment gates on, and CQL and
// the native protocol are recorded under their own keys.
func TestCassandraVersion(t *testing.T) {
	db := openCassandra(t)
	versions, err := dbmeta.Cassandra.Version(t.Context(), db)
	if err != nil {
		t.Fatalf("reading the version: %v", err)
	}
	if versions.Main().Unknown {
		t.Fatal("expected a release version")
	}
	// 3 is the floor, and the release where system_schema arrived.
	if got := versions.Main().Parts[0]; got < 3 {
		t.Errorf("expected release 3 or newer, got %d", got)
	}
	for _, key := range []string{"cql", "protocol"} {
		if !versions.Has(key) {
			t.Errorf("expected the %s version to be recorded", key)
		}
	}
	display := versions.String()
	for _, want := range []string{"Cassandra", "CQL", "Protocol v"} {
		if !strings.Contains(display, want) {
			t.Errorf("expected %q in %q", want, display)
		}
	}
	t.Logf("server reports %s", display)
}

// TestCassandraSmoke runs each registered query against the fixture and
// reports what came back.
func TestCassandraSmoke(t *testing.T) {
	db := openCassandra(t)
	ctx := t.Context()
	m := setupCassandra(t, db)

	// The fixture's own objects. Cassandra applies no filter, so this reads
	// every keyspace and counts the ones that are the fixture's.
	var tables, mine int
	for v, err := range dbmeta.Tables.All(ctx, m, db, caArgs()) {
		if err != nil {
			t.Fatalf("reading tables: %v", err)
		}
		tables++
		if v.Schema == cafixture.Everything.Schema {
			mine++
			t.Logf("  %s.%s %s comment=%q", v.Schema, v.Name, v.Type, v.Comment.V)
		}
	}
	if mine == 0 {
		t.Error("expected the fixture tables")
	}
	t.Logf("tables: %d in all, %d in the fixture", tables, mine)

	// Columns, which is where the two fields read from one kind show up.
	var cols int
	for v, err := range dbmeta.Columns.All(ctx, m, db, caArgs()) {
		if err != nil {
			t.Fatalf("reading columns: %v", err)
		}
		if v.Schema != cafixture.Everything.Schema {
			continue
		}
		cols++
		if v.Table == "book" {
			t.Logf("  book.%-10s %-8s ordinal=%d null=%v pk=%v",
				v.Name, v.DataType, v.Ordinal, v.Nullable, v.PrimaryKey)
		}
	}
	if cols == 0 {
		t.Error("expected the fixture columns")
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
		query, vals, err := q.Build(m, nil)
		if errors.Is(err, dbmeta.ErrVersionTooOld) {
			tooOld++
			continue
		}
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

// TestCassandraFixtureObjects checks that the fixture built one of every
// object the queries read, which is what hard rule 9 asks for.
//
// It is separate from the smoke test because a query that runs and returns
// nothing passes there and fails here, and those are different faults. The
// first says the statement is wrong and the second says the fixture is.
func TestCassandraFixtureObjects(t *testing.T) {
	db := openCassandra(t)
	ctx := t.Context()
	m := setupCassandra(t, db)
	schema := cafixture.Everything.Schema

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
		for v, err := range dbmeta.Views.All(ctx, m, db, caArgs()) {
			if err != nil {
				return 0, err
			}
			if v.Schema == schema {
				n++
			}
		}
		return n, nil
	})
	count("types", func() (int, error) {
		n := 0
		for v, err := range dbmeta.Types.All(ctx, m, db, caArgs()) {
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
		for v, err := range dbmeta.Indexes.All(ctx, m, db, caArgs()) {
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
		for v, err := range dbmeta.IndexColumns.All(ctx, m, db, caArgs()) {
			if err != nil {
				return 0, err
			}
			if v.Schema == schema {
				n++
			}
		}
		return n, nil
	})
	count("functions", func() (int, error) {
		n := 0
		for v, err := range dbmeta.Functions.All(ctx, m, db, caArgs()) {
			if err != nil {
				return 0, err
			}
			if v.Schema == schema {
				n++
			}
		}
		return n, nil
	})
	count("aggregates", func() (int, error) {
		n := 0
		for v, err := range dbmeta.Aggregates.All(ctx, m, db, caArgs()) {
			if err != nil {
				return 0, err
			}
			if v.Schema == schema {
				n++
			}
		}
		return n, nil
	})
	count("constraint columns", func() (int, error) {
		n := 0
		for v, err := range dbmeta.ConstraintColumns.All(ctx, m, db, caArgs()) {
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
		for v, err := range dbmeta.Comments.All(ctx, m, db, caArgs()) {
			if err != nil {
				return 0, err
			}
			if v.Schema == schema && v.Comment != "" {
				n++
			}
		}
		return n, nil
	})
	// The roles the fixture makes live outside the keyspace, so these are
	// counted by name rather than by schema.
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
		for v, err := range dbmeta.Privileges.All(ctx, m, db, nil) {
			if err != nil {
				return 0, err
			}
			if v.Schema.V == schema {
				n++
			}
		}
		return n, nil
	})
	// A service level attached to a role exists on ScyllaDB alone, and
	// TestScyllaIsItsOwnProduct checks what Cassandra says instead.
	if m.Version().Has(cassandra.Scylla) {
		count("role settings", func() (int, error) {
			n := 0
			for v, err := range dbmeta.RoleSettings.All(ctx, m, db, nil) {
				if err != nil {
					return 0, err
				}
				if v.Role.V == "dbmeta_reader" && v.Settings.V == "service_level=dbmeta_level" {
					n++
				}
			}
			return n, nil
		})
	}
}

// TestCassandraScansEveryQuery reads every query the model answers through
// its Scan, row by row, on whichever product is running.
//
// The smoke test runs each statement and counts its columns, and that does not
// reach Scan. A Scan that reads a NULL into a plain string passes there and
// fails for every caller. Settings did exactly that on Cassandra 5.0, once the
// driver began to report a NULL as one: the old driver sent an empty string
// instead, so the fault stayed hidden until the driver changed.
func TestCassandraScansEveryQuery(t *testing.T) {
	db := openCassandra(t)
	m := setupCassandra(t, db)
	var read, skipped int
	check := func(name string, support dbmeta.Support, n int, err error) {
		t.Helper()
		switch {
		case support != dbmeta.Supported:
			skipped++
		case err != nil:
			t.Errorf("%s: %v", name, err)
		default:
			read++
			t.Logf("%-18s %d rows", name, n)
		}
	}
	scan := func(name string, support dbmeta.Support, all func() (int, error)) {
		t.Helper()
		if support != dbmeta.Supported {
			check(name, support, 0, nil)
			return
		}
		n, err := all()
		check(name, support, n, err)
	}
	scan("schemas", dbmeta.Schemas.Support(m), func() (int, error) { return drain(t, dbmeta.Schemas, m, db) })
	scan("tables", dbmeta.Tables.Support(m), func() (int, error) { return drain(t, dbmeta.Tables, m, db) })
	scan("columns", dbmeta.Columns.Support(m), func() (int, error) { return drain(t, dbmeta.Columns, m, db) })
	scan("views", dbmeta.Views.Support(m), func() (int, error) { return drain(t, dbmeta.Views, m, db) })
	scan("types", dbmeta.Types.Support(m), func() (int, error) { return drain(t, dbmeta.Types, m, db) })
	scan("indexes", dbmeta.Indexes.Support(m), func() (int, error) { return drain(t, dbmeta.Indexes, m, db) })
	scan("index columns", dbmeta.IndexColumns.Support(m), func() (int, error) { return drain(t, dbmeta.IndexColumns, m, db) })
	scan("constraints", dbmeta.Constraints.Support(m), func() (int, error) { return drain(t, dbmeta.Constraints, m, db) })
	scan("constraint columns", dbmeta.ConstraintColumns.Support(m), func() (int, error) { return drain(t, dbmeta.ConstraintColumns, m, db) })
	scan("triggers", dbmeta.Triggers.Support(m), func() (int, error) { return drain(t, dbmeta.Triggers, m, db) })
	scan("comments", dbmeta.Comments.Support(m), func() (int, error) { return drain(t, dbmeta.Comments, m, db) })
	scan("functions", dbmeta.Functions.Support(m), func() (int, error) { return drain(t, dbmeta.Functions, m, db) })
	scan("aggregates", dbmeta.Aggregates.Support(m), func() (int, error) { return drain(t, dbmeta.Aggregates, m, db) })
	scan("roles", dbmeta.Roles.Support(m), func() (int, error) { return drain(t, dbmeta.Roles, m, db) })
	scan("role grants", dbmeta.RoleGrants.Support(m), func() (int, error) { return drain(t, dbmeta.RoleGrants, m, db) })
	scan("role settings", dbmeta.RoleSettings.Support(m), func() (int, error) { return drain(t, dbmeta.RoleSettings, m, db) })
	scan("privileges", dbmeta.Privileges.Support(m), func() (int, error) { return drain(t, dbmeta.Privileges, m, db) })
	scan("settings", dbmeta.Settings.Support(m), func() (int, error) { return drain(t, dbmeta.Settings, m, db) })
	// Every query the model registers is in the list above, so a new one
	// that is left out shows as a count that does not add up.
	var registered int
	for _, q := range dbmeta.Queries() {
		if s := q.Support(m); s == dbmeta.Supported || s == dbmeta.TooOld {
			registered++
		}
	}
	if read+countTooOld(m) != registered {
		t.Errorf("read %d queries and the model answers %d: add the missing one here",
			read, registered)
	}
	t.Logf("%d read, %d not asked", read, skipped)
}

// countTooOld counts the queries this release is too old for, which the list
// in TestCassandraScansEveryQuery skips.
func countTooOld(m *dbmeta.Meta) int {
	n := 0
	for _, q := range dbmeta.Queries() {
		if q.Support(m) == dbmeta.TooOld {
			n++
		}
	}
	return n
}

// drain reads every row of one query through its Scan and counts them.
func drain[T any](t *testing.T, q *dbmeta.Query[T], m *dbmeta.Meta, db *sql.DB) (int, error) {
	t.Helper()
	n := 0
	for _, err := range q.All(t.Context(), m, db, nil) {
		if err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}

// TestScyllaIsItsOwnProduct checks what the model makes of the product it is
// connected to. ScyllaDB and Cassandra share the cassandra dialect, and the version
// row is the only place the two are told apart, so a wrong answer there sends
// every ScyllaDB fragment to the wrong server. See D91.
func TestScyllaIsItsOwnProduct(t *testing.T) {
	db := openCassandra(t)
	versions, err := dbmeta.Cassandra.Version(t.Context(), db)
	if err != nil {
		t.Fatalf("reading the version: %v", err)
	}
	m, err := dbmeta.New(dbmeta.Cassandra, versions)
	if err != nil {
		t.Fatalf("building the metadata: %v", err)
	}
	scylla := versions.Has(cassandra.Scylla)
	display := versions.String()
	if got := strings.HasPrefix(display, "ScyllaDB"); got != scylla {
		t.Errorf("the scylla key is %v and the display line is %q", scylla, display)
	}
	// RoleSettings has a source on ScyllaDB and none on Cassandra.
	want := dbmeta.NotSupported
	if scylla {
		want = dbmeta.Supported
	}
	if got := dbmeta.RoleSettings.Support(m); got != want {
		t.Errorf("RoleSettings on %s: got %v, want %v", display, got, want)
	}
	if !scylla {
		return
	}
	// The administrator reads the ScyllaDB release from system.versions.
	if rel := versions.Get(cassandra.Scylla); rel.Unknown || rel.Parts[0] < 2025 {
		t.Errorf("expected a ScyllaDB release of 2025 or newer, got %s", rel)
	}
	// A role granted nothing is refused system.versions and served
	// system.local, so it learns the product and not the release.
	exec(t, db, `CREATE ROLE IF NOT EXISTS dbmeta_nobody WITH PASSWORD = '`+
		parityPassword+`' AND LOGIN = true`)
	t.Cleanup(func() { cleanup(t, db, `DROP ROLE IF EXISTS dbmeta_nobody`) })
	nobody := openAt(t, "cassandra", replaceUser(t, os.Getenv("DBMETA_CASSANDRA"), "dbmeta_nobody", parityPassword))
	theirs, err := dbmeta.Cassandra.Version(t.Context(), nobody)
	if err != nil {
		t.Fatalf("reading the version as a role granted nothing: %v", err)
	}
	if !theirs.Has(cassandra.Scylla) || !theirs.Get(cassandra.Scylla).Unknown {
		t.Errorf("a role granted nothing: expected ScyllaDB with no release, got %s", theirs)
	}
	// ScyllaDB records a type for every setting, so the field that is
	// padded on Cassandra has a value here.
	for v, err := range dbmeta.Settings.All(t.Context(), m, db, nil) {
		if err != nil {
			t.Fatalf("reading settings: %v", err)
		}
		if !v.Type.Valid {
			t.Errorf("setting %s has no type", v.Name)
		}
	}
	t.Logf("server reports %s", display)
}
