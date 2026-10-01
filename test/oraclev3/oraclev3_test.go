// Package oraclev3 runs the Oracle model through go-ora/v3, which is the
// driver dburl names for the oracle scheme (D154).
//
// The rest of the test module reaches Oracle with go-ora/v2, because v3.0.1
// panics on 11g and 18c (D59). The two register the same driver name, so they
// cannot share a binary, and this package is a binary of its own. It uses the
// v3 commit that fixes the panic, so it runs on every release.
//
// D59 said the statements are the same whichever major version carries them.
// They are, and the parameters are not: v3 refuses a Go bool, which v2 binds
// as a number, and usql found every Oracle query failing on it. This is what
// finds that class of fault. See D136.
package oraclev3

import (
	"context"
	"database/sql"
	"os"
	"testing"

	_ "github.com/sijms/go-ora/v3"

	"github.com/xo/dbmeta"
	_ "github.com/xo/dbmeta/models/oracle"
)

// open returns a connection to the server named by DBMETA_ORACLE.
func open(t *testing.T) *sql.DB {
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

// meta reads the version and returns the metadata for the server.
func meta(t *testing.T, db *sql.DB) *dbmeta.Meta {
	t.Helper()
	versions, err := dbmeta.Oracle.Version(t.Context(), db)
	if err != nil {
		t.Fatalf("reading the version: %v", err)
	}
	m, err := dbmeta.New(dbmeta.Oracle, versions)
	if err != nil {
		t.Fatalf("building the metadata: %v", err)
	}
	t.Logf("server reports %s", versions)
	return m
}

// TestEveryQueryRuns runs every query the model supports, with and without
// the system objects, and reads every row. It binds each parameter the way a
// caller does, from [dbmeta.Args].
func TestEveryQueryRuns(t *testing.T) {
	db := open(t)
	m := meta(t, db)
	passes := []bool{false, true}
	// Oracle 11g reads its whole catalog too slowly for the test timeout,
	// as the scan in the test module measured, so it runs without the
	// system objects, the way that scan does. docs/COVERAGE.md has the
	// timings.
	if m.Version().Main().Compare(dbmeta.V(12)) < 0 {
		t.Log("run without the system objects, which Oracle 11g reads too slowly")
		passes = passes[:1]
	}
	for _, system := range passes {
		args := dbmeta.Args{WithSystem: system}.Map()
		for _, q := range dbmeta.Queries() {
			if q.Support(m) != dbmeta.Supported {
				continue
			}
			given := map[string]any{}
			params, err := q.Params(m)
			if err != nil {
				t.Fatalf("%s: reading the parameters: %v", q.Name(), err)
			}
			for _, p := range params {
				if v, ok := args[p.Name]; ok {
					given[p.Name] = v
				}
			}
			n, err := run(t, db, q, m, given)
			if err != nil {
				t.Errorf("%s, system objects %v: %v", q.Name(), system, err)
				continue
			}
			t.Logf("%-26s system %-5v %d rows", q.Name(), system, n)
		}
	}
}

// run builds q and reads every row it returns as raw bytes.
func run(t *testing.T, db *sql.DB, q dbmeta.AnyQuery, m *dbmeta.Meta, args map[string]any) (int, error) {
	t.Helper()
	query, vals, err := q.Build(m, args)
	if err != nil {
		return 0, err
	}
	rows, err := db.QueryContext(t.Context(), query, vals...)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		return 0, err
	}
	dest := make([]any, len(cols))
	for i := range dest {
		dest[i] = new(sql.RawBytes)
	}
	var n int
	for rows.Next() {
		if err := rows.Scan(dest...); err != nil {
			return n, err
		}
		n++
	}
	return n, rows.Err()
}

// TestColumnsOfOneTable asks for the columns of one table by parent, which
// is how every model is asked, and reads them through the typed Scan.
func TestColumnsOfOneTable(t *testing.T) {
	db := open(t)
	m := meta(t, db)
	args := dbmeta.Args{Schema: "SYS", Parent: "DUAL", WithSystem: true}.Map()
	var names []string
	for v, err := range dbmeta.Columns.All(t.Context(), m, db, args) {
		if err != nil {
			t.Fatalf("reading columns: %v", err)
		}
		if v.Table != "DUAL" {
			t.Errorf("expected only DUAL, got %s.%s", v.Table, v.Name)
		}
		names = append(names, v.Name)
	}
	if len(names) != 1 || names[0] != "DUMMY" {
		t.Errorf("expected the one column DUMMY, got %v", names)
	}
}

// TestTheChildrenOfOneTable reads every child kind of one table by parent,
// with the system objects, through the typed Scan. usql asked this way for a
// table in SYSTEM with a primary key and a check, on 2026-09-30, and saw the
// indexes fail.
func TestTheChildrenOfOneTable(t *testing.T) {
	db := open(t)
	m := meta(t, db)
	ctx := t.Context()
	const table = "DBMETA_V3_CHILD"
	//nolint:errcheck // a table left by a failed run is dropped first
	db.ExecContext(ctx, "DROP TABLE "+table)
	if _, err := db.ExecContext(ctx, "CREATE TABLE "+table+
		" (id INT PRIMARY KEY, qty INT CHECK (qty > 0), note VARCHAR2(30))"); err != nil {
		t.Fatalf("creating the table: %v", err)
	}
	t.Cleanup(func() {
		//nolint:errcheck // the test has already reported what matters
		db.ExecContext(context.WithoutCancel(ctx), "DROP TABLE "+table)
	})
	var user string
	if err := db.QueryRowContext(ctx, "SELECT USER FROM dual").Scan(&user); err != nil {
		t.Fatalf("reading the user: %v", err)
	}
	args := dbmeta.Args{Schema: user, Parent: table, WithSystem: true}.Map()
	count := func(name string, n int, err error) {
		t.Helper()
		if err != nil {
			t.Errorf("%s: %v", name, err)
			return
		}
		if n == 0 {
			t.Errorf("%s: expected a row for %s", name, table)
		}
		t.Logf("%-20s %d rows", name, n)
	}
	count(drainAll(ctx, dbmeta.Columns, m, db, args))
	count(drainAll(ctx, dbmeta.Indexes, m, db, args))
	count(drainAll(ctx, dbmeta.IndexColumns, m, db, args))
	name, n, err := drainAll(ctx, dbmeta.Constraints, m, db, args)
	count(name, n, err)
	// The primary key and the check. Oracle names both SYS_C, and before
	// 12c the check cannot be told apart from a NOT NULL.
	if want := 2; m.Version().Main().Compare(dbmeta.V(12)) >= 0 && n != want {
		t.Errorf("constraints: expected %d, the key and the unnamed check, got %d", want, n)
	}
	count(drainAll(ctx, dbmeta.ConstraintColumns, m, db, args))
}

// drainAll reads every row of q and returns its name and the count.
func drainAll[T any](ctx context.Context, q *dbmeta.Query[T], m *dbmeta.Meta, db *sql.DB, args map[string]any) (string, int, error) {
	var n int
	for _, err := range q.All(ctx, m, db, args) {
		if err != nil {
			return q.Name(), n, err
		}
		n++
	}
	return q.Name(), n, nil
}
