package dbmeta

import (
	"database/sql"
	"database/sql/driver"
	"io"
	"sync"
)

// A fake driver, replaying recorded rows. D28 makes this the shape of every
// root module test: the package needs no database driver and no container, and
// it can cover a server version whose container image no longer starts.
//
// database/sql/driver is in the standard library, so this costs no dependency.

// replay is one recorded answer.
type replay struct {
	cols []string
	rows [][]driver.Value
	err  error
}

var (
	replayMu sync.Mutex
	replays  = map[string]replay{}
)

// record registers the answer the fake driver gives for a statement.
func record(query string, cols []string, rows [][]driver.Value, err error) {
	replayMu.Lock()
	defer replayMu.Unlock()
	replays[query] = replay{cols: cols, rows: rows, err: err}
}

func init() {
	sql.Register("dbmetafake", fakeDriver{})
}

type fakeDriver struct{}

func (fakeDriver) Open(string) (driver.Conn, error) { return fakeConn{}, nil }

type fakeConn struct{}

func (c fakeConn) Prepare(q string) (driver.Stmt, error) { return fakeStmt{q: q}, nil }
func (c fakeConn) Close() error                          { return nil }
func (c fakeConn) Begin() (driver.Tx, error)             { return nil, io.EOF }

type fakeStmt struct{ q string }

func (s fakeStmt) Close() error                               { return nil }
func (s fakeStmt) NumInput() int                              { return -1 }
func (s fakeStmt) Exec([]driver.Value) (driver.Result, error) { return nil, io.EOF }

func (s fakeStmt) Query(_ []driver.Value) (driver.Rows, error) {
	replayMu.Lock()
	r, ok := replays[s.q]
	replayMu.Unlock()
	if !ok {
		return nil, io.ErrUnexpectedEOF
	}
	if r.err != nil {
		return nil, r.err
	}
	return &fakeRows{cols: r.cols, rows: r.rows}, nil
}

type fakeRows struct {
	cols []string
	rows [][]driver.Value
	i    int
	done bool
}

func (r *fakeRows) Columns() []string { return r.cols }

// Close records that the rows were released, which is how a test checks that
// stopping an iterator early gives the connection back.
func (r *fakeRows) Close() error {
	r.done = true
	return nil
}

func (r *fakeRows) Next(dest []driver.Value) error {
	if r.i >= len(r.rows) {
		return io.EOF
	}
	copy(dest, r.rows[r.i])
	r.i++
	return nil
}

// openFake returns a handle backed by the fake driver.
func openFake() (*sql.DB, error) { return sql.Open("dbmetafake", "") }
