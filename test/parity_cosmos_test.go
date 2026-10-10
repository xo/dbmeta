package test

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The principal of Cosmos DB that is not the administrator. dbsetup made the
// credential file cosmos-reader, which holds the read-only key of the account. The
// key reads every resource and writes none. The key of the administrator is the
// primary key. Both are account wide, so neither is limited to one database. The file
// holds a DSN with the key as its password, so the test reads it and never prints
// it. See D228.

// cosmosReaderDSN returns the DSN of the reader, and skips when the file is not
// there. The path names the container that the tests read.
func cosmosReaderDSN(t *testing.T) string {
	t.Helper()
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			t.Skipf("no home directory: %v", err)
		}
		dir = filepath.Join(home, ".config")
	}
	body, err := os.ReadFile(filepath.Join(dir, "dbmeta", "credentials", "cosmos-reader"))
	if err != nil {
		t.Skip("no credential file for the principal cosmos-reader")
	}
	return withCosmosContainer(t, strings.TrimSpace(string(body)), cosmosContainer)
}

// prepareCosmos returns the DSN of the administrator with the container that the
// tests read in its path.
func prepareCosmos(t *testing.T, _ *sql.DB, adminDSN string) string {
	t.Helper()
	return withCosmosContainer(t, adminDSN, cosmosContainer)
}

// makeCosmosReader connects with the read-only key of the account.
func makeCosmosReader(t *testing.T, _ *sql.DB, _, _ string) string {
	t.Helper()
	return cosmosReaderDSN(t)
}
