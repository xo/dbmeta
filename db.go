package dbmeta

import (
	"context"
	"database/sql"
)

// DB is a database handle. Both [database/sql.DB] and [database/sql.Tx]
// satisfy it.
//
// Pass a [database/sql.Tx] to read more than one catalog in one snapshot. Two
// calls against a [database/sql.DB] are two statements, and the database can
// change between them.
//
// Every method takes a context. dbmeta never creates one.
type DB interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	PrepareContext(ctx context.Context, query string) (*sql.Stmt, error)
}
