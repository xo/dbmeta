package test

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
)

// The principal of Databricks that is not the administrator. dbsetup made a
// service principal, dbmeta-reader, that the credential file databricks-reader
// names. It holds USE CATALOG on the catalog and USE SCHEMA and SELECT on the
// schema dbmeta, so it reads the tables and cannot create or change anything.
// The file holds the URL of the reader in the form of dburl, with a token as
// its password, so the test reads it and never prints it. See D224 and D229.

// databricksReaderDSN returns the connection string of the reader as the driver
// reads it, and skips when the file is not there.
func databricksReaderDSN(t *testing.T) string {
	t.Helper()
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			t.Skipf("no home directory: %v", err)
		}
		dir = filepath.Join(home, ".config")
	}
	body, err := os.ReadFile(filepath.Join(dir, "dbmeta", "credentials", "databricks-reader"))
	if err != nil {
		t.Skip("no credential file for the principal databricks-reader")
	}
	return hostedDriverDSN(t, string(body))
}

// makeDatabricksReader connects as the service principal that can read the
// schema.
func makeDatabricksReader(t *testing.T, _ *sql.DB, _, _ string) string {
	t.Helper()
	return databricksReaderDSN(t)
}
