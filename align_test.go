package dbmeta_test

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"io"
	"testing"

	"github.com/xo/dbmeta"
)

// alignDriver is a database/sql driver that answers one query with one column
// whose type name is the DSN. Only a driver can make a [sql.ColumnType], and the
// root module has no driver, so the test brings this one.
type alignDriver struct{}

func (alignDriver) Open(name string) (driver.Conn, error) { return alignConn(name), nil }

type alignConn string

func (alignConn) Prepare(string) (driver.Stmt, error) { return nil, driver.ErrSkip }
func (alignConn) Close() error                        { return nil }
func (alignConn) Begin() (driver.Tx, error)           { return nil, driver.ErrSkip }

func (c alignConn) QueryContext(context.Context, string, []driver.NamedValue) (driver.Rows, error) {
	return alignRows(c), nil
}

type alignRows string

func (alignRows) Columns() []string                       { return []string{"c"} }
func (alignRows) Close() error                            { return nil }
func (alignRows) Next([]driver.Value) error               { return io.EOF }
func (r alignRows) ColumnTypeDatabaseTypeName(int) string { return string(r) }

func init() {
	sql.Register("dbmeta-align", alignDriver{})
}

// typed returns the column type of a column whose driver reports the name.
func typed(t *testing.T, name string) *sql.ColumnType {
	t.Helper()
	db, err := sql.Open("dbmeta-align", name)
	if err != nil {
		t.Fatalf("opening the driver: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	rows, err := db.QueryContext(t.Context(), "select")
	if err != nil {
		t.Fatalf("querying: %v", err)
	}
	defer rows.Close()
	cts, err := rows.ColumnTypes()
	if err := rows.Err(); err != nil {
		t.Fatalf("reading the rows: %v", err)
	}
	if err != nil || len(cts) != 1 {
		t.Fatalf("reading the column types: %v", err)
	}
	return cts[0]
}

// TestRightAlignedFollowsThePsqlRule checks the helper that models use. A name
// matches in any case and without its size, any other name aligns left, and a
// column with no name says nothing.
func TestRightAlignedFollowsThePsqlRule(t *testing.T) {
	f := dbmeta.RightAligned("INT4", "decimal")
	for name, want := range map[string]dbmeta.Align{
		"int4": dbmeta.AlignRight, "DECIMAL(10,2)": dbmeta.AlignRight, "TEXT": dbmeta.AlignLeft,
	} {
		if a, ok := f(typed(t, name)); !ok || a != want {
			t.Errorf("%s: expected %v, got %v and %v", name, want, a, ok)
		}
	}
	if _, ok := f(typed(t, "")); ok {
		t.Error("expected a column with no type name to say nothing")
	}
}

// TestColumnAlignOfAnUnbuiltDialectSaysNothing checks that a dialect with no
// model gives no alignment, so that a client falls back to the values.
func TestColumnAlignOfAnUnbuiltDialectSaysNothing(t *testing.T) {
	if _, ok := dbmeta.Dialect("nosuchdialect").ColumnAlign(typed(t, "INT")); ok {
		t.Error("expected no alignment for a dialect with no model")
	}
}
