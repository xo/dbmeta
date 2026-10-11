//go:build !omni

package test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/xo/dbimp/spanner"
)

// This file holds what differs between the two Spanner test binaries. The
// default binary, which this file builds, opens the driver of dbimp and reads
// Cloud Spanner. The binary built with -tags omni opens go-sql-spanner and
// reads Spanner Omni, and spanner_omni_test.go is its half. The build tag is not
// an operating system or an architecture, so hard rule 11 does not forbid it.
// Both drivers register the name spanner, so one binary cannot import both.
// See D231.

// spannerIsOmni reports whether the binary reads Spanner Omni. This one reads
// Cloud Spanner.
func spannerIsOmni() bool {
	return false
}

// runSpannerDDL runs the statements as one batch and returns the error of the
// batch. The driver of dbimp sends one Exec of several DDL statements, cut at
// each semicolon that sits outside a quote and a comment, as one request and one
// operation. Spanner takes a schema change as a long running operation, so a
// batch is one operation where separate statements are one each. A statement
// that Spanner refuses fails the whole batch and the error does not say which,
// so a caller that needs to know runs the statements one at a time, which is
// spannerDDLEach. See D231.
func runSpannerDDL(ctx context.Context, db *sql.DB, queries []string) error {
	if _, err := db.ExecContext(ctx, strings.Join(queries, ";\n")); err != nil {
		return fmt.Errorf("running the batch: %w", err)
	}
	return nil
}

// spannerWithRole returns the DSN that names the database role. The key is
// database_role, and the driver of dbimp refuses a key that it does not know.
func spannerWithRole(dsn, role string) string {
	sep := "?"
	if strings.Contains(dsn, "?") {
		sep = "&"
	}
	return dsn + sep + "database_role=" + role
}

// spannerReaderDSN returns the URL of the reader, and skips when the file is not
// there. The credential file spanner-reader holds a URL in the form of dburl
// with the path of the key file of the reader in credential_file, which the
// driver takes as it is.
func spannerReaderDSN(t *testing.T) string {
	t.Helper()
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			t.Skipf("no home directory: %v", err)
		}
		dir = filepath.Join(home, ".config")
	}
	body, err := os.ReadFile(filepath.Join(dir, "dbmeta", "credentials", "spanner-reader"))
	if err != nil {
		t.Skip("no credential file for the principal spanner-reader")
	}
	return strings.TrimSpace(string(body))
}
