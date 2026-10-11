//go:build omni

package test

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	_ "github.com/googleapis/go-sql-spanner"
)

// This file is the Spanner Omni half of the pair that spanner_hosted_test.go
// starts. The binary built with -tags omni opens go-sql-spanner, because Omni
// speaks gRPC only and the driver of dbimp speaks REST. See D231.

// spannerIsOmni reports whether the binary reads Spanner Omni, which this one
// does.
func spannerIsOmni() bool {
	return true
}

// runSpannerDDL runs the statements as one batch on one connection, and returns
// the error of the batch. go-sql-spanner takes START BATCH DDL and RUN BATCH.
// Spanner takes a schema change as a long running operation that lasts seconds,
// so a batch is one operation where separate statements are one each. A
// statement that Spanner refuses fails the whole batch and the error does not
// say which, so a caller that needs to know runs the statements one at a time,
// which is spannerDDLEach.
func runSpannerDDL(ctx context.Context, db *sql.DB, queries []string) error {
	conn, err := db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("taking a connection: %w", err)
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, "START BATCH DDL"); err != nil {
		return fmt.Errorf("starting the batch: %w", err)
	}
	for _, q := range queries {
		if _, err := conn.ExecContext(ctx, q); err != nil {
			//nolint:errcheck // the batch is abandoned and the first error is the one that matters
			conn.ExecContext(context.WithoutCancel(ctx), "ABORT BATCH")
			return fmt.Errorf("adding to the batch: %w", err)
		}
	}
	if _, err := conn.ExecContext(ctx, "RUN BATCH"); err != nil {
		return fmt.Errorf("running the batch: %w", err)
	}
	return nil
}

// spannerWithRole returns the DSN that names the database role. The property is
// database_role, and a name that go-sql-spanner does not know, such as role, is
// ignored without a word. See D219.
func spannerWithRole(dsn, role string) string {
	return dsn + ";database_role=" + role
}

// spannerReaderDSN skips. Spanner Omni has no authentication, so it has no
// second IAM principal.
func spannerReaderDSN(t *testing.T) string {
	t.Helper()
	t.Skip("Spanner Omni has no authentication, so it has no second IAM principal")
	return ""
}
