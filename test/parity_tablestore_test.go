package test

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
)

// The principal of Tablestore that is not the administrator. dbsetup made a RAM
// user that the credential file tablestore-reader names. It can read the tables
// of the instance and cannot create or delete one. The file holds the URL of the
// reader in the form of dburl, with its key pair, so the test reads it and never
// prints it. See D241.

// tablestoreReaderDSN returns the connection string of the reader as the driver
// reads it, and skips when the file is not there.
func tablestoreReaderDSN(t *testing.T) string {
	t.Helper()
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			t.Skipf("no home directory: %v", err)
		}
		dir = filepath.Join(home, ".config")
	}
	body, err := os.ReadFile(filepath.Join(dir, "dbmeta", "credentials", "tablestore-reader"))
	if err != nil {
		t.Skip("no credential file for the principal tablestore-reader")
	}
	return hostedDriverDSN(t, string(body))
}

// makeTablestoreReader connects as the RAM user that can read the tables.
func makeTablestoreReader(t *testing.T, _ *sql.DB, _, _ string) string {
	t.Helper()
	return tablestoreReaderDSN(t)
}
