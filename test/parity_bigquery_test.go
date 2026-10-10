package test

import (
	"database/sql"
	"net/url"
	"os"
	"path/filepath"
	"testing"
)

// The principal of BigQuery that is not the administrator. dbsetup made the
// service account that the credential file bigquery-reader names. It holds
// dataViewer on the dataset and jobUser on the project, so it reads the
// metadata of the dataset and cannot change anything. Its key is a file that
// dbrun names with GOOGLE_APPLICATION_CREDENTIALS for the administrator, so the
// reader names its own file in the DSN with the option credential_file of the
// driver. The DSN holds a path and no secret. See D218 and D220.

// bigQueryReaderKey returns the key file of the reader, and skips when the file
// is not there.
func bigQueryReaderKey(t *testing.T) string {
	t.Helper()
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			t.Skipf("no home directory: %v", err)
		}
		dir = filepath.Join(home, ".config")
	}
	file := filepath.Join(dir, "dbmeta", "gcp", "bigquery-reader.json")
	if info, err := os.Stat(file); err != nil || !info.Mode().IsRegular() {
		t.Skip("no key file for the principal bigquery-reader")
	}
	return file
}

// bigQueryReaderDSN is the DSN of the administrator with the key of the reader.
func bigQueryReaderDSN(t *testing.T, dsn string) string {
	t.Helper()
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("parsing the DSN: %v", err)
	}
	q := u.Query()
	q.Set("credential_file", bigQueryReaderKey(t))
	u.RawQuery = q.Encode()
	return u.String()
}

// makeBigQueryReader connects as the service account that holds dataViewer on
// the dataset.
func makeBigQueryReader(t *testing.T, _ *sql.DB, dsn, _ string) string {
	t.Helper()
	return bigQueryReaderDSN(t, dsn)
}
