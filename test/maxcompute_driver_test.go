package test

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/aliyun/aliyun-odps-go-sdk/sqldriver"
)

// retryDriverName is the name of the driver that the MaxCompute tests open. It
// is the driver of Alibaba with a retry on a reset connection. See D239.
const retryDriverName = "odps-retry"

func init() {
	sql.Register(retryDriverName, retryDriver{})
}

// retryDriver wraps the driver of Alibaba, which registers itself as odps.
//
// The service resets a connection to the endpoint in Jakarta often, and the
// driver reports it as "connection reset by peer". The first attempt of a
// statement fails and the second works, so a test that does not retry fails
// about one run in three. The retry belongs to the tests and never to the
// library, because a caller of dbmeta decides what to do with a failed
// statement. Only that one error is retried, and a statement that this driver
// runs is a query or a data definition that the tests can run twice.
type retryDriver struct{}

// Open opens the connection of the wrapped driver.
func (retryDriver) Open(name string) (driver.Conn, error) {
	conn, err := sqldriver.OdpsDriver{}.Open(name)
	if err != nil {
		return nil, err
	}
	return retryConn{conn}, nil
}

// retryConn runs each statement again when the service resets the connection.
type retryConn struct {
	driver.Conn
}

// QueryContext runs a query, and runs it again after a reset.
func (c retryConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	q, ok := c.Conn.(driver.QueryerContext)
	if !ok {
		return nil, driver.ErrSkip
	}
	var (
		rows driver.Rows
		err  error
	)
	for attempt := range resetAttempts {
		start := time.Now()
		if rows, err = q.QueryContext(ctx, query, args); !isReset(err) {
			trace(attempt, start, query, err)
			return rows, err
		}
		trace(attempt, start, query, err)
		if err := pause(ctx); err != nil {
			return nil, err
		}
	}
	return rows, err
}

// ExecContext runs a statement, and runs it again after a reset.
func (c retryConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	e, ok := c.Conn.(driver.ExecerContext)
	if !ok {
		return nil, driver.ErrSkip
	}
	var (
		res driver.Result
		err error
	)
	for attempt := range resetAttempts {
		start := time.Now()
		if res, err = e.ExecContext(ctx, query, args); !isReset(err) {
			trace(attempt, start, query, err)
			return res, err
		}
		trace(attempt, start, query, err)
		if err := pause(ctx); err != nil {
			return nil, err
		}
	}
	return res, err
}

// trace writes one line for an attempt when DBMETA_MAXCOMPUTE_TRACE is set. The
// line holds no value of the statement beyond its first words.
func trace(attempt int, start time.Time, query string, err error) {
	if os.Getenv("DBMETA_MAXCOMPUTE_TRACE") == "" {
		return
	}
	head, _, _ := strings.Cut(query, "\n")
	fmt.Fprintf(os.Stderr, "maxcompute attempt %d took %s, reset %t: %.40s\n", attempt+1, time.Since(start).Round(time.Millisecond), isReset(err), head)
}

// resetAttempts is how many times a statement runs before the reset is a
// failure.
const resetAttempts = 100

// isReset reports whether the error is the reset of a connection.
func isReset(err error) bool {
	return err != nil && strings.Contains(err.Error(), "connection reset by peer")
}

// pause waits before the next attempt, and stops when the context ends.
func pause(ctx context.Context) error {
	t := time.NewTimer(time.Second)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// IsValid says that a connection is never worth keeping. Each connection of the
// driver builds an HTTP client of its own that keeps its sockets open, and the
// service closes an idle socket, so the next statement on the same connection is
// reset. database/sql drops a connection that says it is not valid when it
// returns the connection, so every statement runs on a new socket.
func (retryConn) IsValid() bool { return false }
