package test

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"

	dbimpts "github.com/xo/dbimp/tablestore"
)

// tablestoreRetryDriver is the name of the driver that the Tablestore tests
// open. It is the driver of dbimp with a retry on a transient error. See D243.
const tablestoreRetryDriver = "tablestore-retry"

func init() {
	sql.Register(tablestoreRetryDriver, tsRetryDriver{})
}

// tsRetryDriver wraps the connector of the driver of dbimp.
//
// The service resets a connection to the endpoint in Jakarta now and then. The
// driver returns the error and does not retry a statement that can have reached
// the server, which is right for a library. The statements of the tests are
// reads and mappings that run twice without harm, so the wrapper retries them.
// The retry belongs to the tests and never to the library, because a caller of
// dbmeta decides what to do with a failed statement.
type tsRetryDriver struct{}

// Open is not supported, because the driver of dbimp opens a connection through
// a connector. database/sql calls OpenConnector first.
func (tsRetryDriver) Open(string) (driver.Conn, error) {
	return nil, errors.New("use sql.Open, which opens a connector")
}

// OpenConnector returns a connector of the wrapped driver for the DSN.
func (tsRetryDriver) OpenConnector(name string) (driver.Connector, error) {
	cfg, err := dbimpts.ParseDSN(name)
	if err != nil {
		return nil, fmt.Errorf("reading the DSN of Tablestore: %w", err)
	}
	return tsRetryConnector{dbimpts.NewConnector(*cfg)}, nil
}

// tsRetryConnector connects as the wrapped connector does.
type tsRetryConnector struct{ *dbimpts.Connector }

// Connect opens a connection that retries.
func (c tsRetryConnector) Connect(ctx context.Context) (driver.Conn, error) {
	conn, err := c.Connector.Connect(ctx)
	if err != nil {
		return nil, err
	}
	return &tsRetryConn{Conn: conn}, nil
}

// Driver returns the wrapper.
func (tsRetryConnector) Driver() driver.Driver { return tsRetryDriver{} }

// tsRetryConn is a connection of the driver whose statements retry.
type tsRetryConn struct{ driver.Conn }

// CheckNamedValue passes an argument to the wrapped connection, so that the
// driver converts it as it does without the wrapper.
func (c *tsRetryConn) CheckNamedValue(nv *driver.NamedValue) error {
	if ck, ok := c.Conn.(driver.NamedValueChecker); ok {
		return ck.CheckNamedValue(nv)
	}
	return driver.ErrSkip
}

// Ping checks the wrapped connection, and retries it.
func (c *tsRetryConn) Ping(ctx context.Context) error {
	p, ok := c.Conn.(driver.Pinger)
	if !ok {
		return nil
	}
	return retry(ctx, func() error { return p.Ping(ctx) })
}

// QueryContext runs a query, and runs it again after a transient error.
func (c *tsRetryConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	q, ok := c.Conn.(driver.QueryerContext)
	if !ok {
		return nil, driver.ErrSkip
	}
	var rows driver.Rows
	err := retry(ctx, func() error {
		var err error
		rows, err = q.QueryContext(ctx, query, args)
		return err
	})
	return rows, err
}

// ExecContext runs a statement, and runs it again after a transient error.
func (c *tsRetryConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	e, ok := c.Conn.(driver.ExecerContext)
	if !ok {
		return nil, driver.ErrSkip
	}
	var res driver.Result
	err := retry(ctx, func() error {
		var err error
		res, err = e.ExecContext(ctx, query, args)
		return err
	})
	return res, err
}
