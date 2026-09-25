package test

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"strings"
	"testing"

	_ "github.com/MichaelS11/go-cql-driver"

	"github.com/xo/dbmeta"
	_ "github.com/xo/dbmeta/models/cassandra"
	cafixture "github.com/xo/dbmeta/models/cassandra/fixture"
)

// openCassandra returns a connection to the server named by DBMETA_CASSANDRA.
//
// The DSN is a host list and query options rather than a URL, which is what
// the go-cql-driver takes and what dburl produces for a cassandra scheme.
func openCassandra(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("DBMETA_CASSANDRA")
	if dsn == "" {
		t.Skip("set DBMETA_CASSANDRA to run against a real server")
	}
	db, err := sql.Open("cql", dsn)
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
			db.ExecContext(ctx, step.SQL)
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
		if _, err := db.ExecContext(ctx, step.SQL); err != nil {
			// A refused function, view or role means the server is the
			// published image rather than the one this repository builds.
			// Say so, because the statement alone does not.
			t.Fatalf("setup %s: %v\n%s\n\n%s", step.Name, err, step.SQL, imageHint)
		}
		ran++
	}
	t.Cleanup(func() {
		for _, step := range down {
			if !step.Skipped {
				//nolint:errcheck // the test has already reported what matters
				db.ExecContext(context.WithoutCancel(ctx), step.SQL)
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
	" cd test && ./cassandra/build.sh"

// caArgs is the filter the fixture's objects sit behind.
//
// Cassandra ignores every one of them, which D62 explains and every parameter
// description says. It is passed anyway, because a caller passing a filter is
// the case worth testing and the result has to hold the fixture's rows among
// the rest.
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
		sqlstr, vals, err := q.SQL(m, nil)
		if errors.Is(err, dbmeta.ErrVersionTooOld) {
			tooOld++
			continue
		}
		if err != nil {
			t.Errorf("%s: building: %v", q.Name(), err)
			continue
		}
		cs, err := columnsOf(t, db, sqlstr, vals)
		if err != nil {
			t.Errorf("%s: %v\n%s", q.Name(), err, sqlstr)
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
			if v.Schema == schema {
				n++
			}
		}
		return n, nil
	})
}
