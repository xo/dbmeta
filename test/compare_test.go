package test

import (
	"context"
	"database/sql"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/models/mysql"
	myfixture "github.com/xo/dbmeta/models/mysql/fixture"
)

// MariaDB and MySQL share one dialect and one model, so the same Go code reads
// both. This checks that it really does: the same fixture is built on each
// server, the same queries are asked of each, and the answers are compared row
// by row.
//
// They are not identical and they are not meant to be. Two kinds of difference
// are expected and are recorded here rather than hidden.
//
// One product holds an object the other does not. MariaDB has a sequence and a
// stored aggregate, and MySQL has neither, so the rows those produce are left
// out of the comparison by name.
//
// One product spells a value its own way. MariaDB keeps the display width of
// an integer and MySQL dropped it, so a column reads int(11) on one and int on
// the other. Those columns are named in productSpecific below. Everything else
// must match exactly, and a new difference fails this test rather than being
// discovered by a user.

// productSpecific names the columns whose value differs between MariaDB and
// MySQL for the same object. The comparison checks that the row exists on both
// and that every other column agrees.
var productSpecific = map[string][]string{
	// MariaDB keeps the display width of an integer, so int(11) against int,
	// and bigint(21) against bigint.
	"columns": {"data_type", "default"},
	// MariaDB records the check clause as written. MySQL rewrites it with the
	// character set introducer, so `title` <> '' becomes (`title` <> _utf8mb4'').
	"constraints": {"definition"},
}

// mariaOnly names the objects only MariaDB builds, by the value of the column
// that identifies a row. They are left out rather than compared against
// nothing.
var mariaOnly = []string{"counter", "total"}

// TestMySQLAgainstMariaDB runs the same fixture and the same queries against
// both products and compares the answers. Set DBMETA_MYSQL to one server and
// DBMETA_MYSQL_COMPARE to the other.
func TestMySQLAgainstMariaDB(t *testing.T) {
	first, second := os.Getenv("DBMETA_MYSQL"), os.Getenv("DBMETA_MYSQL_COMPARE")
	if first == "" || second == "" {
		t.Skip("set DBMETA_MYSQL and DBMETA_MYSQL_COMPARE to two servers of different products")
	}
	dbA, mA := openCompare(t, first)
	dbB, mB := openCompare(t, second)
	// Which server is which comes from what they reported, not from the order
	// of the two variables, so the two settings can be given either way round.
	if mysql.IsMariaDB(mA.Version()) == mysql.IsMariaDB(mB.Version()) {
		t.Fatalf("expected one MariaDB server and one MySQL server, got %s and %s", mA, mB)
	}
	if !mysql.IsMariaDB(mA.Version()) {
		dbA, mA, dbB, mB = dbB, mB, dbA, mA
	}
	t.Logf("comparing %s against %s", mA, mB)

	args := dbmeta.Args{Schema: myfixture.Everything.Schema}.Map()
	var compared, skipped int
	for _, q := range dbmeta.Queries() {
		if q.Support(mA) != dbmeta.Supported || q.Support(mB) != dbmeta.Supported {
			skipped++
			continue
		}
		// Only the queries that narrow to one schema. A server wide query,
		// such as the one for settings, answers about the product itself and
		// has no reason to agree.
		if !takesSchema(t, q, mA) {
			continue
		}
		rowsA := readRows(t, dbA, mA, q, args)
		rowsB := readRows(t, dbB, mB, q, args)
		compareRows(t, q.Name(), rowsA, rowsB)
		compared++
	}
	t.Logf("compared %d queries, %d unsupported on one side", compared, skipped)
	if compared == 0 {
		t.Fatal("expected at least one query to compare")
	}
}

// row is one result row, as a name to value map, with the values it is
// identified by.
type row struct {
	key    string
	cells  map[string]string
	nulls  map[string]bool
	labels []string
}

func openCompare(t *testing.T, dsn string) (*sql.DB, *dbmeta.Meta) {
	t.Helper()
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("opening: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.PingContext(t.Context()); err != nil {
		t.Fatalf("connecting: %v", err)
	}
	versions, err := dbmeta.MySQL.Version(t.Context(), db)
	if err != nil {
		t.Fatalf("reading the version: %v", err)
	}
	m, err := dbmeta.New(dbmeta.MySQL, versions)
	if err != nil {
		t.Fatalf("building the meta: %v", err)
	}
	down, err := myfixture.Everything.ResolveTeardown(versions)
	if err != nil {
		t.Fatalf("resolving the teardown: %v", err)
	}
	up, err := myfixture.Everything.ResolveSetup(versions)
	if err != nil {
		t.Fatalf("resolving the setup: %v", err)
	}
	run := func(ctx context.Context, steps []myfixture.Result, fatal bool) {
		for _, s := range steps {
			if s.Skipped {
				continue
			}
			if _, err := db.ExecContext(ctx, s.SQL); err != nil && fatal {
				t.Fatalf("%s on %s: %v", s.Name, m, err)
			}
		}
	}
	run(t.Context(), down, false)
	run(t.Context(), up, true)
	t.Cleanup(func() { run(context.Background(), down, false) })
	return db, m
}

// takesSchema reports whether the query narrows to one schema.
func takesSchema(t *testing.T, q dbmeta.AnyQuery, m *dbmeta.Meta) bool {
	t.Helper()
	params, err := q.Params(m)
	if err != nil {
		t.Fatalf("%s: reading params: %v", q.Name(), err)
	}
	return slices.ContainsFunc(params, func(p dbmeta.Param) bool { return p.Name == "schema" })
}

// readRows runs the query and returns its rows, keyed by the columns that name
// the object. Every value is read as a nullable string, because the point is
// to compare what the server said rather than to use it.
func readRows(t *testing.T, db *sql.DB, m *dbmeta.Meta, q dbmeta.AnyQuery, args map[string]any) []row {
	t.Helper()
	sqlstr, vals, err := q.SQL(m, args)
	if err != nil {
		t.Fatalf("%s: rendering: %v", q.Name(), err)
	}
	rs, err := db.QueryContext(t.Context(), sqlstr, vals...)
	if err != nil {
		t.Fatalf("%s: executing: %v\n%s", q.Name(), err, sqlstr)
	}
	defer rs.Close()
	cols, err := rs.Columns()
	if err != nil {
		t.Fatalf("%s: reading columns: %v", q.Name(), err)
	}
	var out []row
	for rs.Next() {
		cells := make([]sql.Null[string], len(cols))
		ptrs := make([]any, len(cols))
		for i := range cells {
			ptrs[i] = &cells[i]
		}
		if err := rs.Scan(ptrs...); err != nil {
			t.Fatalf("%s: scanning: %v", q.Name(), err)
		}
		r := row{cells: make(map[string]string, len(cols)), nulls: make(map[string]bool, len(cols))}
		for i, name := range cols {
			r.cells[name] = cells[i].V
			r.nulls[name] = !cells[i].Valid
			if isKeyColumn(name) {
				r.labels = append(r.labels, cells[i].V)
			}
		}
		r.key = strings.Join(r.labels, "/")
		out = append(out, r)
	}
	if err := rs.Err(); err != nil {
		t.Fatalf("%s: reading rows: %v", q.Name(), err)
	}
	return out
}

// isKeyColumn reports whether a column names the object rather than describing
// it. Two rows with the same values here are the same object.
func isKeyColumn(name string) bool {
	switch name {
	case "schema", "table", "parent", "name", "ordinal", "column":
		return true
	}
	return false
}

// compareRows checks that both products found the same objects and said the
// same things about them.
func compareRows(t *testing.T, name string, a, b []row) {
	t.Helper()
	keysA, keysB := keyed(a), keyed(b)
	for key := range keysA {
		if _, ok := keysB[key]; !ok && !expectedOnlyOnMaria(key) {
			t.Errorf("%s: MariaDB found %q and MySQL did not", name, key)
		}
	}
	for key := range keysB {
		if _, ok := keysA[key]; !ok {
			t.Errorf("%s: MySQL found %q and MariaDB did not", name, key)
		}
	}
	allowed := productSpecific[name]
	for key, ra := range keysA {
		rb, ok := keysB[key]
		if !ok {
			continue
		}
		for col, va := range ra.cells {
			vb := rb.cells[col]
			if va == vb && ra.nulls[col] == rb.nulls[col] {
				continue
			}
			if slices.Contains(allowed, col) {
				continue
			}
			t.Errorf("%s: %s: column %q differs\n MariaDB %s\n MySQL   %s",
				name, key, col, show(ra, col), show(rb, col))
		}
	}
}

func show(r row, col string) string {
	if r.nulls[col] {
		return "NULL"
	}
	return `"` + r.cells[col] + `"`
}

// expectedOnlyOnMaria reports whether a row is one of the objects only MariaDB
// builds, which is not a difference worth reporting.
func expectedOnlyOnMaria(key string) bool {
	for _, part := range strings.Split(key, "/") {
		if slices.Contains(mariaOnly, part) {
			return true
		}
	}
	return false
}

func keyed(rows []row) map[string]row {
	out := make(map[string]row, len(rows))
	for _, r := range rows {
		out[r.key] = r
	}
	return out
}
