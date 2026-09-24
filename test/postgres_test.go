package test

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/xo/dbmeta"
	_ "github.com/xo/dbmeta/models/postgres"
	"github.com/xo/dbmeta/models/postgres/fixture"
)

// open returns a connection to the server named by DBMETA_POSTGRES, or skips.
func open(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("DBMETA_POSTGRES")
	if dsn == "" {
		t.Skip("set DBMETA_POSTGRES to run against a real server")
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

// setup builds the fixture and returns the meta for the server.
//
// The fixture lives in the model package rather than here, because it changes
// with the queries and because other projects use it. A step the server is too
// old for is skipped, and the query that reads what it would have built is
// refused on the same release.
func setup(t *testing.T, db *sql.DB) *dbmeta.Meta {
	t.Helper()
	versions, err := dbmeta.PostgreSQL.Version(t.Context(), db)
	if err != nil {
		t.Fatalf("reading the version: %v", err)
	}
	m, err := dbmeta.New(dbmeta.PostgreSQL, versions)
	if err != nil {
		t.Fatalf("building the meta: %v", err)
	}

	// the context must be passed in rather than taken from t, because
	// [testing.T.Context] is cancelled just before a cleanup runs, and the
	// teardown is a cleanup
	run := func(ctx context.Context, steps []fixture.Result) {
		for _, s := range steps {
			if s.Skipped {
				continue
			}
			if _, err := db.ExecContext(ctx, s.SQL); err != nil {
				t.Fatalf("%s: %v\n%s", s.Name, err, s.SQL)
			}
		}
	}
	down, err := fixture.Everything.ResolveTeardown(versions)
	if err != nil {
		t.Fatalf("resolving the teardown: %v", err)
	}
	up, err := fixture.Everything.ResolveSetup(versions)
	if err != nil {
		t.Fatalf("resolving the setup: %v", err)
	}
	run(t.Context(), down)
	run(t.Context(), up)
	t.Cleanup(func() { run(context.Background(), down) })

	var skipped int
	for _, s := range up {
		if s.Skipped {
			skipped++
		}
	}
	t.Logf("server reports %s, fixture ran %d steps and skipped %d", m, len(up)-skipped, skipped)
	return m
}

func args() map[string]any {
	return dbmeta.Args{Schema: fixture.Everything.Schema}.Map()
}

// TestEveryQueryRuns executes every query the server is new enough for, and
// checks that the columns it returns are exactly the fields it declares.
func TestEveryQueryRuns(t *testing.T) {
	db := open(t)
	m := setup(t, db)

	var ran, tooOld int
	for _, q := range dbmeta.Queries() {
		if q.Support(m) != dbmeta.Supported {
			continue
		}
		sqlstr, vals, err := q.SQL(m, nil)
		switch {
		case errors.Is(err, dbmeta.ErrVersionTooOld):
			tooOld++
			continue
		case err != nil:
			t.Errorf("%s: rendering: %v", q.Name(), err)
			continue
		}
		rows, err := db.QueryContext(t.Context(), sqlstr, vals...)
		if err != nil {
			t.Errorf("%s: executing: %v\n%s", q.Name(), err, sqlstr)
			continue
		}
		cols, err := rows.Columns()
		rows.Close()
		if err != nil {
			t.Errorf("%s: reading columns: %v", q.Name(), err)
			continue
		}
		fields, err := q.Fields(m)
		if err != nil {
			t.Errorf("%s: reading fields: %v", q.Name(), err)
			continue
		}
		if len(cols) != len(fields) {
			t.Errorf("%s: declares %d fields and returns %d columns", q.Name(), len(fields), len(cols))
			continue
		}
		for i := range cols {
			if cols[i] != fields[i].Name {
				t.Errorf("%s: column %d is %q and the field is %q", q.Name(), i, cols[i], fields[i].Name)
			}
		}
		ran++
	}
	t.Logf("%d queries ran, %d were refused as too old", ran, tooOld)
	if ran == 0 {
		t.Fatal("expected at least one query to run")
	}
}

// TestPaddedFieldsAreNull is the invariant that replaces a golden file per
// release. Ten releases times 48 queries is 480 combinations, which nobody
// would maintain. [dbmeta.Field.Min] already declares the release each column
// arrived in, so the assertion can be generic: a field the server is too old
// for must be NULL in every row, because the statement padded it.
//
// This is the padding rule of D8 checked against a real server rather than
// against the text of the statement.
func TestPaddedFieldsAreNull(t *testing.T) {
	db := open(t)
	m := setup(t, db)
	server := m.Version().Main()

	var checked int
	for _, q := range dbmeta.Queries() {
		if q.Support(m) != dbmeta.Supported {
			continue
		}
		fields, err := q.Fields(m)
		if err != nil {
			continue
		}
		var padded []int
		for i, f := range fields {
			if !f.Min.IsZero() && !server.AtLeast(f.Min) {
				padded = append(padded, i)
			}
		}
		if len(padded) == 0 {
			continue
		}
		sqlstr, vals, err := q.SQL(m, nil)
		if err != nil {
			continue
		}
		rows, err := db.QueryContext(t.Context(), sqlstr, vals...)
		if err != nil {
			t.Errorf("%s: %v", q.Name(), err)
			continue
		}
		dest := make([]any, len(fields))
		raw := make([]sql.RawBytes, len(fields))
		for i := range dest {
			dest[i] = &raw[i]
		}
		for rows.Next() {
			if err := rows.Scan(dest...); err != nil {
				t.Errorf("%s: scanning: %v", q.Name(), err)
				break
			}
			for _, i := range padded {
				if raw[i] != nil {
					t.Errorf("%s: %q arrived in %s and the server is %s, so it must be NULL, got %q",
						q.Name(), fields[i].Name, fields[i].Min, server, raw[i])
				}
			}
			checked++
		}
		rows.Close()
	}
	t.Logf("checked %d padded values", checked)
}

// TestScanningWorks reads rows through the typed API, which rendering tests
// cannot check. A Go type that does not match what the driver returns fails
// here and nowhere else.
func TestScanningWorks(t *testing.T) {
	db := open(t)
	m := setup(t, db)
	ctx := t.Context()

	var tables []dbmeta.Table
	for v, err := range dbmeta.Tables.All(ctx, m, db, args()) {
		if err != nil {
			t.Fatalf("reading tables: %v", err)
		}
		tables = append(tables, v)
	}
	if len(tables) == 0 {
		t.Fatal("expected the fixture relations")
	}

	var withComment, withoutComment int
	for _, v := range tables {
		if v.Comment.Valid {
			withComment++
		} else {
			withoutComment++
		}
	}
	if withComment == 0 || withoutComment == 0 {
		t.Errorf("expected both a commented and an uncommented relation, got %d and %d",
			withComment, withoutComment)
	}

	var cols int
	a := dbmeta.Args{Schema: fixture.Everything.Schema, Parent: "author"}.Map()
	for v, err := range dbmeta.Columns.All(ctx, m, db, a) {
		if err != nil {
			t.Fatalf("reading columns: %v", err)
		}
		cols++
		if v.Table != "author" {
			t.Errorf("expected a column of author, got one of %s", v.Table)
		}
	}
	if cols < 4 {
		t.Errorf("expected at least the 4 declared columns on author, got %d", cols)
	}
}

// TestNullAccessDiffersFromEmpty is the regression test for the bug COALESCE
// hid. A relation with default privileges reports a NULL access list. A
// relation with every privilege revoked reports an empty one. Those are
// different answers and the API must keep them apart.
func TestNullAccessDiffersFromEmpty(t *testing.T) {
	db := open(t)
	m := setup(t, db)

	got := map[string]dbmeta.Text{}
	for v, err := range dbmeta.Privileges.All(t.Context(), m, db, args()) {
		if err != nil {
			t.Fatalf("reading privileges: %v", err)
		}
		got[v.Name] = v.Access
	}
	def, ok := got["default_privs"]
	if !ok {
		t.Fatal("expected the default_privs relation")
	}
	rev, ok := got["revoked_privs"]
	if !ok {
		t.Fatal("expected the revoked_privs relation")
	}
	if def.Valid {
		t.Errorf("expected default privileges to report an absent access list, got %q", def.V)
	}
	if !rev.Valid {
		t.Error("expected revoked privileges to report a present, empty access list")
	}
}
