package oracle_test

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"io"
	"testing"

	"github.com/xo/dbmeta"
	_ "github.com/xo/dbmeta/models/oracle"
)

// guidColumn is one column that a fake driver reports with a type name and,
// when length is not zero, a length.
type guidColumn struct {
	name   string
	length int64
	value  []byte
}

type guidConnector struct{ col guidColumn }

func (c guidConnector) Connect(context.Context) (driver.Conn, error) { return guidConn(c), nil }
func (guidConnector) Driver() driver.Driver                          { return nil }

type guidConn struct{ col guidColumn }

func (c guidConn) Prepare(string) (driver.Stmt, error) { return guidStmt(c), nil }
func (guidConn) Close() error                          { return nil }
func (guidConn) Begin() (driver.Tx, error)             { return nil, io.EOF }

type guidStmt struct{ col guidColumn }

func (guidStmt) Close() error                               { return nil }
func (guidStmt) NumInput() int                              { return 0 }
func (guidStmt) Exec([]driver.Value) (driver.Result, error) { return nil, io.EOF }
func (s guidStmt) Query([]driver.Value) (driver.Rows, error) {
	return &guidRows{col: s.col}, nil
}

type guidRows struct {
	col  guidColumn
	done bool
}

func (*guidRows) Columns() []string { return []string{"id"} }
func (*guidRows) Close() error      { return nil }
func (r *guidRows) Next(dest []driver.Value) error {
	if r.done {
		return io.EOF
	}
	r.done = true
	dest[0] = r.col.value
	return nil
}
func (r *guidRows) ColumnTypeDatabaseTypeName(int) string { return r.col.name }
func (r *guidRows) ColumnTypeLength(int) (int64, bool) {
	return r.col.length, r.col.length != 0
}

// TestFormatBinaryWritesARaw16AsAUUID checks D263 without a server. A fake
// driver reports the type name and the length that xo/oracle reports.
func TestFormatBinaryWritesARaw16AsAUUID(t *testing.T) {
	t.Parallel()
	guid := []byte{0x5f, 0x6c, 0xe3, 0x36, 0x2b, 0x56, 0x0f, 0xbd, 0xe0, 0x63, 0x05, 0x01, 0xa8, 0xc0, 0xaf, 0x0e}
	for _, tt := range []struct {
		name string
		col  guidColumn
		want string
	}{
		{"raw16", guidColumn{"RAW", 16, guid}, "5f6ce336-2b56-0fbd-e063-0501a8c0af0e"},
		{"lower case name", guidColumn{"raw", 16, guid}, "5f6ce336-2b56-0fbd-e063-0501a8c0af0e"},
		{"no length", guidColumn{"RAW", 0, guid}, `\x5f6ce3362b560fbde0630501a8c0af0e`},
		{"raw of another length", guidColumn{"RAW", 32, guid}, `\x5f6ce3362b560fbde0630501a8c0af0e`},
		{"short value in a raw16", guidColumn{"RAW", 16, guid[:4]}, `\x5f6ce336`},
		{"empty value in a raw16", guidColumn{"RAW", 16, []byte{}}, `\x`},
		{"blob", guidColumn{"BLOB", 16, guid}, `\x5f6ce3362b560fbde0630501a8c0af0e`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			db := sql.OpenDB(guidConnector{tt.col})
			defer db.Close()
			rows, err := db.QueryContext(context.Background(), "select")
			if err != nil {
				t.Fatal(err)
			}
			defer rows.Close()
			cts, err := rows.ColumnTypes()
			if err != nil {
				t.Fatal(err)
			}
			if err := rows.Err(); err != nil {
				t.Fatal(err)
			}
			got, ok := dbmeta.Oracle.FormatBinary(cts[0], tt.col.value)
			if !ok || got != tt.want {
				t.Errorf("got %q and %t, want %q", got, ok, tt.want)
			}
		})
	}
}
