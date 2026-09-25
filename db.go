package dbmeta

import (
	"context"
	"database/sql"
)

// Querier runs a query. It is the whole of what dbmeta asks a database to do.
//
// [database/sql.DB], [database/sql.Tx] and [database/sql.Conn] all satisfy it.
// [database/sql.Stmt] does not, because its QueryContext takes no statement.
//
// Pass a [database/sql.Tx] to read more than one catalog in one snapshot. Two
// calls against a [database/sql.DB] are two statements, and the database can
// change between them.
//
// # One method, on purpose
//
// This is the interface rather than [database/sql.DB] so that the type system
// states what the library does. dbmeta reads. It never writes, never prepares
// a statement, and never runs anything without a context, and a reader
// establishes that from this declaration rather than by searching the
// implementation.
//
// It had four methods until D49. Two were never called, and one of those was
// ExecContext, so the interface of a read only library advertised the one
// operation hard rule 8 forbids it from performing.
//
// # It is not a seam for a mock
//
// A fake cannot satisfy this. [database/sql.Rows] is a struct with unexported
// fields that only database/sql builds, from a registered driver, so nothing
// outside that package can return one.
//
// Test against a fake driver instead, registered with
// [database/sql.Register]. That is what dbmeta does: see examplefake_test.go,
// which replays recorded rows and needs no database. The driver interface is
// the seam Go provides, and it is a smaller thing to implement than it looks.
type Querier interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}
