package dbmeta_test

// A fake driver so the examples run without a database. A real client opens a
// real connection, usually with dburl.Open.

import (
	"database/sql"
	"database/sql/driver"
	"io"
	"strings"
)

type answer struct {
	cols []string
	rows [][]driver.Value
}

// answers is keyed by a distinctive substring of the statement, because the
// statements the examples run are assembled from version fragments.
var answers = []struct {
	match string
	answer
}{
	{"SHOW server_version", answer{
		[]string{"server_version"},
		[][]driver.Value{{"16.2"}},
	}},
	{"FROM pg_catalog.pg_namespace n\nWHERE", answer{
		[]string{"catalog", "name", "owner", "comment"},
		[][]driver.Value{
			{"example", "public", "postgres", nil},
			{"example", "sales", "reporting", "sales reporting"},
		},
	}},
	{"FROM pg_catalog.pg_class c", answer{
		[]string{"catalog", "schema", "name", "type", "comment"},
		[][]driver.Value{
			{"example", "public", "author", "table", "people who write"},
			// a relation with no comment reports NULL, not an empty string
			{"example", "public", "book", "table", nil},
			{"example", "public", "recent_book", "view", nil},
		},
	}},
	{"FROM pg_catalog.pg_attribute a", answer{
		[]string{"catalog", "schema", "table", "name", "ordinal", "data_type", "nullable", "default", "primary_key", "identity", "generated", "comment"},
		[][]driver.Value{
			{"example", "public", "book", "book_id", int64(1), "integer", false, nil, true, "a", nil, "surrogate key"},
			{"example", "public", "book", "title", int64(2), "text", false, nil, false, nil, nil, nil},
			{"example", "public", "book", "published", int64(3), "date", true, nil, false, nil, nil, nil},
			{"example", "public", "book", "slug", int64(4), "text", true, nil, false, nil, "s", "derived from the title"},
		},
	}},
}

func init() { sql.Register("examplefake", exDriver{}) }

type exDriver struct{}

func (exDriver) Open(string) (driver.Conn, error) { return exConn{}, nil }

type exConn struct{}

func (exConn) Prepare(q string) (driver.Stmt, error) { return exStmt{q: q}, nil }
func (exConn) Close() error                          { return nil }
func (exConn) Begin() (driver.Tx, error)             { return nil, io.EOF }

type exStmt struct{ q string }

func (exStmt) Close() error                               { return nil }
func (exStmt) NumInput() int                              { return -1 }
func (exStmt) Exec([]driver.Value) (driver.Result, error) { return nil, io.EOF }

func (s exStmt) Query([]driver.Value) (driver.Rows, error) {
	for _, a := range answers {
		if strings.Contains(s.q, a.match) {
			return &exRows{cols: a.cols, rows: a.rows}, nil
		}
	}
	return nil, io.ErrUnexpectedEOF
}

type exRows struct {
	cols []string
	rows [][]driver.Value
	i    int
}

func (r *exRows) Columns() []string { return r.cols }
func (r *exRows) Close() error      { return nil }

func (r *exRows) Next(dest []driver.Value) error {
	if r.i >= len(r.rows) {
		return io.EOF
	}
	copy(dest, r.rows[r.i])
	r.i++
	return nil
}
