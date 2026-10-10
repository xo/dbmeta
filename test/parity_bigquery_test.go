package test

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
)

// The principal of BigQuery that is not the administrator. dbsetup made the
// service account that the credential file bigquery-reader names. It holds
// dataViewer on the dataset and jobUser on the project, so it reads the
// metadata of the dataset and cannot change anything. The file holds the URL
// of the reader in the form of dburl, with the path of its own key file in
// credential_file. The URL holds a path and no secret. See D218, D220 and D229.

// bigQueryReaderDSN returns the connection string of the reader as the driver
// reads it, and skips when the file is not there.
func bigQueryReaderDSN(t *testing.T) string {
	t.Helper()
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			t.Skipf("no home directory: %v", err)
		}
		dir = filepath.Join(home, ".config")
	}
	body, err := os.ReadFile(filepath.Join(dir, "dbmeta", "credentials", "bigquery-reader"))
	if err != nil {
		t.Skip("no credential file for the principal bigquery-reader")
	}
	return hostedDriverDSN(t, string(body))
}

// makeBigQueryReader connects as the service account that holds dataViewer on
// the dataset.
func makeBigQueryReader(t *testing.T, _ *sql.DB, _, _ string) string {
	t.Helper()
	return bigQueryReaderDSN(t)
}
