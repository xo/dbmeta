package test

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/dump"
)

// dataRows are the rows of the two tables. They hold what a literal can get
// wrong: a quote, a backslash, a non-ASCII letter, a NUL in bytes, an empty
// string, NULL, a decimal with a fraction, and the ends of the integers.
var (
	dataOwner = [][]any{
		{int64(1), "ann", true},
		{int64(2), "it's a \\ test é€", false},
		{int64(3), "", nil},
	}
	dataItem = [][]any{
		{int64(1), int64(1), "12.50", int64(7), "hello", time.Date(2024, 2, 29, 13, 14, 15, 0, time.UTC), time.Date(2024, 2, 29, 0, 0, 0, 0, time.UTC), []byte{0, 1, 0xfe, 0xff}},
		{int64(2), int64(2), "-0.25", int64(-2147483648), "it's", time.Date(1999, 12, 31, 23, 59, 59, 0, time.UTC), time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC), []byte{}},
		{int64(3), int64(2), nil, nil, nil, nil, nil, []byte(nil)},
	}
)

// insertValues inserts rows into a table of any product with placeholders and
// the values of Go, which every driver takes.
func insertValues(t *testing.T, db *sql.DB, d dbmeta.Dialect, table string, cols []string, rows [][]any) {
	t.Helper()
	for _, row := range rows {
		marks := make([]string, len(cols))
		for i := range marks {
			marks[i], _ = d.Placeholder(i + 1)
		}
		stmt := "INSERT INTO " + table + " (" + strings.Join(cols, ", ") + ") VALUES (" + strings.Join(marks, ", ") + ")"
		if _, err := db.ExecContext(t.Context(), stmt, row...); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
}

// readCells runs a query and returns every cell as the driver gave it.
func readCells(t *testing.T, db *sql.DB, query string) [][]any {
	t.Helper()
	rows, err := db.QueryContext(t.Context(), query)
	if err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		t.Fatal(err)
	}
	var out [][]any
	for rows.Next() {
		cells := make([]any, len(cols))
		dest := make([]any, len(cols))
		for i := range dest {
			dest[i] = &cells[i]
		}
		if err := rows.Scan(dest...); err != nil {
			t.Fatalf("scanning %s: %v", query, err)
		}
		out = append(out, cells)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

// canonical writes a cell as one string that does not depend on the product, by
// the kind of its column.
func canonical(t *testing.T, v any, kind dump.Kind) string {
	t.Helper()
	typ := dump.Type{Kind: kind}
	conv, err := dump.Convert(v, typ, typ)
	if err != nil {
		t.Fatalf("reading %v as %s: %v", v, kind, err)
	}
	switch x := conv.(type) {
	case nil:
		return "NULL"
	case []byte:
		return fmt.Sprintf("bytes %x", x)
	case time.Time:
		return x.UTC().Format(time.RFC3339Nano)
	case string:
		if kind == dump.Decimal {
			f, err := strconv.ParseFloat(x, 64)
			if err != nil {
				t.Fatalf("%q is not a decimal: %v", x, err)
			}
			return strconv.FormatFloat(f, 'f', -1, 64)
		}
	}
	return fmt.Sprint(conv)
}

// dataKinds are the kinds of the columns of the two tables, in order.
var (
	ownerKinds = []dump.Kind{dump.Integer, dump.String, dump.Boolean}
	itemKinds  = []dump.Kind{dump.Integer, dump.Integer, dump.Decimal, dump.Integer, dump.String, dump.LocalTimestamp, dump.Date, dump.Binary}
)

// snapshotOf reads the snapshot that a side builds and puts it in one schema.
func snapshotOf(t *testing.T, side crossSide, db *sql.DB, filter dump.Filter) *dump.Snapshot {
	t.Helper()
	m := metaFor(t, side.dialect, db)
	snap, err := dump.Read(t.Context(), m, db, filter)
	if err != nil {
		t.Fatalf("reading: %v", err)
	}
	renameSchemas(snap, "dump_cross")
	return snap
}

// TestRowsMoveBetweenDialects creates the tables of every product with rows in
// them, writes the script and the INSERT of the rows for every product, in the
// two forms, with literals and with placeholders, runs them, and requires the
// same rows. The same dialect is tried too.
func TestRowsMoveBetweenDialects(t *testing.T) {
	policy := dump.Policy{Default: dump.Allow{Lossy: true}}
	sides := crossSides()
	for _, src := range sides {
		for _, dst := range sides {
			// The index of PostgreSQL and of DuckDB keeps the schema in its definition,
			// which the test renames, so the round trip test of the script covers the
			// same product.
			if src.name == dst.name && (src.name == "postgres" || src.name == "duckdb" || src.name == "cockroachdb" || src.shared) {
				continue
			}
			for _, form := range []string{"literal", "args"} {
				t.Run(src.name+" to "+dst.name+" "+form, func(t *testing.T) {
					srcDB, srcFilter := src.open(t, "src")
					for _, s := range src.create {
						mustExec(t, srcDB, s)
					}
					insertValues(t, srcDB, src.dialect, src.table("src", "owner"), []string{"id", "name", "active"}, dataOwner)
					insertValues(t, srcDB, src.dialect, src.table("src", "item"), []string{"id", "owner_id", "price", "qty", "note", "made", "born", "data"}, dataItem)
					snap := snapshotOf(t, src, srcDB, srcFilter)

					dstDB, _ := dst.open(t, "dst")
					for st, err := range dump.Script(snap, dst.dialect, policy) {
						if err != nil {
							t.Fatalf("writing the script: %v", err)
						}
						if st.Stage == dump.Pre && st.SQL != "" {
							mustExec(t, dstDB, st.SQL)
						}
					}
					for _, table := range []string{"owner", "item"} {
						moveRows(t, srcDB, dstDB, snap, src, dst, table, form, policy)
					}
					for st, err := range dump.Script(snap, dst.dialect, policy) {
						if err != nil {
							t.Fatalf("writing the script: %v", err)
						}
						if st.Stage == dump.Post && st.SQL != "" && !st.Untranslated {
							mustExec(t, dstDB, st.SQL)
						}
					}
					for _, c := range []struct {
						table string
						kinds []dump.Kind
					}{{"owner", ownerKinds}, {"item", itemKinds}} {
						want := rowsOf(t, srcDB, src, "src", c.table, c.kinds)
						got := rowsOf(t, dstDB, dst, "dst", c.table, c.kinds)
						if want != got {
							t.Errorf("the rows of %s differ.\nsource:\n%s\ncopy:\n%s", c.table, want, got)
						}
					}
				})
			}
		}
	}
}

// rowsOf reads the rows of a table in the order of the first column, as strings.
func rowsOf(t *testing.T, db *sql.DB, side crossSide, role, table string, kinds []dump.Kind) string {
	t.Helper()
	cols := "id, name, active"
	if table == "item" {
		cols = "id, owner_id, price, qty, note, made, born, data"
	}
	var lines []string
	for _, row := range readCells(t, db, "SELECT "+cols+" FROM "+side.table(role, table)+" ORDER BY id") {
		parts := make([]string, len(row))
		for i, v := range row {
			parts[i] = canonical(t, v, kinds[i])
		}
		lines = append(lines, strings.Join(parts, " | "))
	}
	return strings.Join(lines, "\n")
}

// moveRows reads a table from the source and writes it into the target in one of
// the two forms.
func moveRows(t *testing.T, srcDB, dstDB *sql.DB, snap *dump.Snapshot, src, dst crossSide, table, form string, policy dump.Policy) {
	t.Helper()
	l, err := dump.NewLoader(snap, "dump_cross", table, dst.dialect, policy)
	if err != nil {
		t.Fatalf("preparing the loader of %s: %v", table, err)
	}
	// The loader names the table in the snapshot, which is renamed, so the read
	// is written for the product directly.
	query := "SELECT " + strings.Join(l.Columns(), ", ") + " FROM " + src.table("src", table) + " ORDER BY " + l.Columns()[0]
	cells := readCells(t, srcDB, query)
	// A statement that lets an INSERT number an identity column holds for one
	// connection, so the whole table goes through one.
	conn, err := dstDB.Conn(t.Context())
	if err != nil {
		t.Fatalf("taking a connection: %v", err)
	}
	defer conn.Close()
	if p := l.Prologue(); p != "" {
		if _, err := conn.ExecContext(t.Context(), p); err != nil {
			t.Fatalf("%s: %v", p, err)
		}
	}
	for start := 0; start < len(cells); start += l.BatchSize() {
		batch := cells[start:min(start+l.BatchSize(), len(cells))]
		switch form {
		case "literal":
			stmt, err := l.Insert(batch)
			if err != nil {
				t.Fatalf("writing the INSERT of %s: %v", table, err)
			}
			if _, err := conn.ExecContext(t.Context(), stmt); err != nil {
				t.Fatalf("%s: %v", stmt, err)
			}
		default:
			args, err := l.Args(batch)
			if err != nil {
				t.Fatalf("converting the rows of %s: %v", table, err)
			}
			if _, err := conn.ExecContext(t.Context(), l.InsertArgs(len(batch)), args...); err != nil {
				t.Fatalf("%s: %v", l.InsertArgs(len(batch)), err)
			}
		}
	}
	if e := l.Epilogue(); e != "" {
		if _, err := conn.ExecContext(t.Context(), e); err != nil {
			t.Fatalf("%s: %v", e, err)
		}
	}
}
