package test

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/xo/dbmeta"
	_ "github.com/xo/dbmeta/models/cratedb"
	crfixture "github.com/xo/dbmeta/models/cratedb/fixture"
)

// openCrateDB returns a connection to the server named by DBMETA_CRATEDB.
//
// CrateDB is reached on its PostgreSQL port with pgx, which is what dburl's
// cratedb:// opens. lib/pq is not tested, because it opens a transaction that
// CrateDB refuses. See D123.
func openCrateDB(t *testing.T) *sql.DB {
	t.Helper()
	return openCrateDBAt(t, os.Getenv("DBMETA_CRATEDB"))
}

// openCrateDBAt returns a connection to dsn, or skips when dsn is empty.
func openCrateDBAt(t *testing.T, dsn string) *sql.DB {
	t.Helper()
	if dsn == "" {
		t.Skip("set DBMETA_CRATEDB to run against a real server")
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

// setupCrateDB builds the fixture and returns the metadata for the server.
//
// It tears down first, because a run that failed part way leaves the tables
// behind and CREATE TABLE then fails rather than the test reporting what
// actually went wrong.
func setupCrateDB(t *testing.T, db *sql.DB) *dbmeta.Meta {
	t.Helper()
	ctx := t.Context()
	versions, err := dbmeta.CrateDB.Version(ctx, db)
	if err != nil {
		t.Fatalf("reading the version: %v", err)
	}

	down, err := crfixture.Everything.ResolveTeardown(versions)
	if err != nil {
		t.Fatalf("resolving the teardown: %v", err)
	}
	for _, step := range down {
		if !step.Skipped {
			//nolint:errcheck // a teardown before setup is best effort
			db.ExecContext(ctx, step.Query)
		}
	}

	up, err := crfixture.Everything.ResolveSetup(versions)
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

	m, err := dbmeta.New(dbmeta.CrateDB, versions)
	if err != nil {
		t.Fatalf("building the metadata: %v", err)
	}
	t.Logf("server reports %s, fixture ran %d steps and skipped %d", versions, ran, skipped)
	return m
}

// crArgs is the filter the fixture's objects sit behind.
func crArgs() map[string]any {
	return dbmeta.Args{Schema: crfixture.Everything.Schema}.Map()
}

// TestCrateDBVersion reads the version and checks what the model makes of it.
// The main version is the PostgreSQL release CrateDB claims, and the release
// key holds CrateDB's own.
func TestCrateDBVersion(t *testing.T) {
	db := openCrateDB(t)
	versions, err := dbmeta.CrateDB.Version(t.Context(), db)
	if err != nil {
		t.Fatalf("reading the version: %v", err)
	}
	if versions.Main().Unknown {
		t.Fatal("expected the PostgreSQL release CrateDB claims")
	}
	own := versions.Get("cratedb")
	if own.Unknown || len(own.Parts) == 0 {
		t.Fatalf("expected CrateDB's own release in %s", versions)
	}
	// 6.3 is the floor, which is the oldest release whose image is rebuilt.
	if own.Parts[0] < 6 {
		t.Errorf("expected release 6 or newer, got %s", own)
	}
	if display := versions.String(); !strings.Contains(display, "CrateDB") {
		t.Errorf("expected the product in %q", display)
	}
	t.Logf("server reports %s", versions)
}

// TestCrateDBSmoke runs each registered query against the fixture and checks
// that it returns the columns it declares. This is the hard rule 9 check: a
// query that has never run is not finished.
func TestCrateDBSmoke(t *testing.T) {
	db := openCrateDB(t)
	m := setupCrateDB(t, db)
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
		fields, err := q.Fields(m)
		if err != nil {
			t.Errorf("%s: reading fields: %v", q.Name(), err)
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

// TestCrateDBScansEveryQuery reads every query through its own Scan, which is
// where a NULL arriving in a field that cannot hold one fails. The smoke test
// reads column names only and cannot find that.
func TestCrateDBScansEveryQuery(t *testing.T) {
	db := openCrateDB(t)
	m := setupCrateDB(t, db)
	var read, skipped int
	scan := func(name string, support dbmeta.Support, all func() (int, error)) {
		t.Helper()
		if support != dbmeta.Supported {
			skipped++
			return
		}
		n, err := all()
		if err != nil {
			t.Errorf("%s: %v", name, err)
			return
		}
		read++
		t.Logf("%-18s %d rows", name, n)
	}
	scan("schemas", dbmeta.Schemas.Support(m), func() (int, error) { return drain(t, dbmeta.Schemas, m, db) })
	scan("current schema", dbmeta.CurrentSchema.Support(m), func() (int, error) { return drain(t, dbmeta.CurrentSchema, m, db) })
	scan("current user", dbmeta.CurrentUser.Support(m), func() (int, error) { return drain(t, dbmeta.CurrentUser, m, db) })
	scan("databases", dbmeta.Databases.Support(m), func() (int, error) { return drain(t, dbmeta.Databases, m, db) })
	scan("tables", dbmeta.Tables.Support(m), func() (int, error) { return drain(t, dbmeta.Tables, m, db) })
	scan("columns", dbmeta.Columns.Support(m), func() (int, error) { return drain(t, dbmeta.Columns, m, db) })
	scan("column stats", dbmeta.ColumnStats.Support(m), func() (int, error) { return drain(t, dbmeta.ColumnStats, m, db) })
	scan("views", dbmeta.Views.Support(m), func() (int, error) { return drain(t, dbmeta.Views, m, db) })
	scan("partitioned tables", dbmeta.PartitionedTables.Support(m), func() (int, error) { return drain(t, dbmeta.PartitionedTables, m, db) })
	// Every type and collation is CrateDB's own, so these two are read with
	// the system objects or they read nothing.
	scan("types", dbmeta.Types.Support(m), func() (int, error) { return drainSystem(t, dbmeta.Types, m, db) })
	scan("collations", dbmeta.Collations.Support(m), func() (int, error) { return drainSystem(t, dbmeta.Collations, m, db) })
	scan("indexes", dbmeta.Indexes.Support(m), func() (int, error) { return drain(t, dbmeta.Indexes, m, db) })
	scan("index columns", dbmeta.IndexColumns.Support(m), func() (int, error) { return drain(t, dbmeta.IndexColumns, m, db) })
	scan("constraints", dbmeta.Constraints.Support(m), func() (int, error) { return drain(t, dbmeta.Constraints, m, db) })
	scan("constraint columns", dbmeta.ConstraintColumns.Support(m), func() (int, error) { return drain(t, dbmeta.ConstraintColumns, m, db) })
	scan("functions", dbmeta.Functions.Support(m), func() (int, error) { return drain(t, dbmeta.Functions, m, db) })
	scan("roles", dbmeta.Roles.Support(m), func() (int, error) { return drain(t, dbmeta.Roles, m, db) })
	scan("role grants", dbmeta.RoleGrants.Support(m), func() (int, error) { return drain(t, dbmeta.RoleGrants, m, db) })
	scan("privileges", dbmeta.Privileges.Support(m), func() (int, error) { return drain(t, dbmeta.Privileges, m, db) })
	scan("settings", dbmeta.Settings.Support(m), func() (int, error) { return drain(t, dbmeta.Settings, m, db) })
	scan("foreign servers", dbmeta.ForeignServers.Support(m), func() (int, error) { return drain(t, dbmeta.ForeignServers, m, db) })
	scan("foreign tables", dbmeta.ForeignTables.Support(m), func() (int, error) { return drain(t, dbmeta.ForeignTables, m, db) })
	scan("user mappings", dbmeta.UserMappings.Support(m), func() (int, error) { return drain(t, dbmeta.UserMappings, m, db) })
	scan("publications", dbmeta.Publications.Support(m), func() (int, error) { return drain(t, dbmeta.Publications, m, db) })
	scan("publication tables", dbmeta.PublicationTables.Support(m), func() (int, error) { return drain(t, dbmeta.PublicationTables, m, db) })
	scan("subscriptions", dbmeta.Subscriptions.Support(m), func() (int, error) { return drain(t, dbmeta.Subscriptions, m, db) })
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

// drainSystem reads every row of q, the system objects included, and counts
// them.
func drainSystem[T any](t *testing.T, q *dbmeta.Query[T], m *dbmeta.Meta, db *sql.DB) (int, error) {
	t.Helper()
	n := 0
	for _, err := range q.All(t.Context(), m, db, dbmeta.Args{WithSystem: true}.Map()) {
		if err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}

// TestCrateDBFixtureObjects checks that the fixture built one of every object
// the queries read, which is what hard rule 9 asks for, and reads back the
// values that are CrateDB's own.
func TestCrateDBFixtureObjects(t *testing.T) {
	db := openCrateDB(t)
	ctx := t.Context()
	m := setupCrateDB(t, db)
	schema := crfixture.Everything.Schema

	count := func(name string, n int, err error) {
		t.Helper()
		switch {
		case err != nil:
			t.Fatalf("reading %s: %v", name, err)
		case n == 0:
			t.Errorf("the fixture built no %s", name)
		default:
			t.Logf("%-18s %d", name, n)
		}
	}

	var generated, pk int
	for v, err := range dbmeta.Columns.All(ctx, m, db, crArgs()) {
		if err != nil {
			t.Fatalf("reading columns: %v", err)
		}
		if v.Generated.V != "" {
			generated++
		}
		if v.PrimaryKey {
			pk++
		}
	}
	if generated == 0 {
		t.Error("expected the generated column on sales")
	}
	if pk == 0 {
		t.Error("expected a primary key column")
	}

	var kinds []string
	for v, err := range dbmeta.Tables.All(ctx, m, db, crArgs()) {
		if err != nil {
			t.Fatalf("reading tables: %v", err)
		}
		kinds = append(kinds, v.Name+"="+v.Type)
	}
	for _, want := range []string{"author=table", "recent=view", "remote=foreign table"} {
		if !strings.Contains(strings.Join(kinds, " "), want) {
			t.Errorf("expected %s among %v", want, kinds)
		}
	}

	n := 0
	var err error
	for v, e := range dbmeta.Views.All(ctx, m, db, crArgs()) {
		if err = e; err != nil {
			break
		}
		if v.Schema == schema {
			n++
		}
	}
	count("views", n, err)

	n = 0
	for v, e := range dbmeta.Indexes.All(ctx, m, db, crArgs()) {
		if err = e; err != nil {
			break
		}
		if v.Schema == schema && v.Primary {
			n++
		}
	}
	count("indexes", n, err)

	n = 0
	for v, e := range dbmeta.PartitionedTables.All(ctx, m, db, crArgs()) {
		if err = e; err != nil {
			break
		}
		if v.Schema == schema {
			n++
		}
	}
	count("partitioned tables", n, err)

	n = 0
	for v, e := range dbmeta.Functions.All(ctx, m, db, crArgs()) {
		if err = e; err != nil {
			break
		}
		if v.Schema == schema && v.Language == "javascript" {
			n++
		}
	}
	count("functions", n, err)

	n = 0
	for v, e := range dbmeta.Constraints.All(ctx, m, db, crArgs()) {
		if err = e; err != nil {
			break
		}
		if v.Schema == schema {
			n++
		}
	}
	count("constraints", n, err)

	n = 0
	for v, e := range dbmeta.ForeignTables.All(ctx, m, db, crArgs()) {
		if err = e; err != nil {
			break
		}
		if v.Schema == schema {
			n++
		}
	}
	count("foreign tables", n, err)

	n = 0
	for v, e := range dbmeta.Roles.All(ctx, m, db, nil) {
		if err = e; err != nil {
			break
		}
		if strings.HasPrefix(v.Name, "dbmeta_") {
			n++
		}
	}
	count("roles", n, err)

	n = 0
	for v, e := range dbmeta.RoleGrants.All(ctx, m, db, nil) {
		if err = e; err != nil {
			break
		}
		if v.Role == "dbmeta_grantee" {
			n++
		}
	}
	count("role grants", n, err)

	// The grant to the role and the denial to the user are both on book.
	var access string
	for v, e := range dbmeta.Privileges.All(ctx, m, db, crArgs()) {
		if e != nil {
			t.Fatalf("reading privileges: %v", e)
		}
		if v.Name == "book" {
			access = v.Access.V
		}
	}
	if !strings.Contains(access, "dbmeta_reader=DQL/crate") || !strings.Contains(access, "denied") {
		t.Errorf("expected a grant and a denial on book, got %q", access)
	}

	for name, q := range map[string]func() (int, error){
		"foreign servers":    func() (int, error) { return drain(t, dbmeta.ForeignServers, m, db) },
		"user mappings":      func() (int, error) { return drain(t, dbmeta.UserMappings, m, db) },
		"publications":       func() (int, error) { return drain(t, dbmeta.Publications, m, db) },
		"publication tables": func() (int, error) { return drain(t, dbmeta.PublicationTables, m, db) },
	} {
		n, err := q()
		count(name, n, err)
	}
}
