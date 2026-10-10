package test

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The principal of Athena that is not the administrator. dbsetup made an IAM
// user that the credential file athena-reader names. It can query the tables and
// the views of the Glue database dbmeta and cannot create or drop a table or
// write under tables/. The file holds the connection string of the reader in the
// form of dburl, with a secret, so the test reads it and never prints it. See
// D222.

// athenaReaderDSN returns the connection string of the reader as the driver
// reads it, and skips when the file is not there.
func athenaReaderDSN(t *testing.T) string {
	t.Helper()
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			t.Skipf("no home directory: %v", err)
		}
		dir = filepath.Join(home, ".config")
	}
	body, err := os.ReadFile(filepath.Join(dir, "dbmeta", "credentials", "athena-reader"))
	if err != nil {
		t.Skip("no credential file for the principal athena-reader")
	}
	return athenaDriverDSN(t, strings.TrimSpace(string(body)))
}

// makeAthenaReader connects as the IAM user that can read the Glue database.
func makeAthenaReader(t *testing.T, _ *sql.DB, _, _ string) string {
	t.Helper()
	return athenaReaderDSN(t)
}
