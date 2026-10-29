package test

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The principal of MaxCompute that is not the administrator. The credential file
// maxcompute-reader names a RAM user that holds the role reader, which has List,
// Describe and Select on the tables of the project. The file holds the URL of the
// reader in the form of dburl, with its key pair, so the test reads it and never
// prints it. See D237 and D240.

// maxComputeReaderDSN returns the connection string of the reader as the driver
// reads it, and skips when the file is not there.
func maxComputeReaderDSN(t *testing.T) string {
	t.Helper()
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			t.Skipf("no home directory: %v", err)
		}
		dir = filepath.Join(home, ".config")
	}
	body, err := os.ReadFile(filepath.Join(dir, "dbmeta", "credentials", "maxcompute-reader"))
	if err != nil {
		t.Skip("no credential file for the principal maxcompute-reader")
	}
	return hostedDriverDSN(t, string(body))
}

// makeMaxComputeReader connects as the RAM user that holds the role reader. The
// service refuses every statement of a user that lacks CreateInstance on the
// project, and the role reader lacks it until the administrator grants it, so the
// scene skips with that reason and records nothing. The test grants nothing,
// because a grant to a role changes the account. See D240.
func makeMaxComputeReader(t *testing.T, _ *sql.DB, _, _ string) string {
	t.Helper()
	dsn := maxComputeReaderDSN(t)
	db, err := sql.Open(retryDriverName, dsn)
	if err != nil {
		t.Fatalf("opening the reader: %v", err)
	}
	defer db.Close()
	if err := maxComputeDrain(context.WithoutCancel(t.Context()), db, "SELECT 1 AS one;"); err != nil {
		if strings.Contains(err.Error(), "odps:CreateInstance") {
			t.Skip("the role reader has no CreateInstance on the project, so the service refuses every statement. See D240")
		}
		t.Fatalf("connecting as the reader: %v", err)
	}
	return dsn
}
