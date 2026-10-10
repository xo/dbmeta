package test

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	spfixture "github.com/xo/dbmeta/models/spanner/fixture"
)

// The principals of Spanner. On Cloud Spanner dbsetup made one, the credential file
// spanner-reader, whose service account holds databaseReader on the database.
// Its key is a file that dbrun names with GOOGLE_APPLICATION_CREDENTIALS for the
// administrator, so a second principal names its own file in the DSN with the
// credentials property of go-sql-spanner. The DSN holds a path and no secret.
// A database role is a principal on both Cloud Spanner and Spanner Omni. See
// D218 and D219.

// spannerReaderKey returns the key file of the reader, and skips when this is
// not a hosted Cloud Spanner or the file is not there. Spanner Omni, which has
// no authentication, has no second IAM principal.
func spannerReaderKey(t *testing.T, dsn string) string {
	t.Helper()
	if strings.Contains(dsn, "usePlainText=true") {
		t.Skip("Spanner Omni has no authentication, so it has no second IAM principal")
	}
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			t.Skipf("no home directory: %v", err)
		}
		dir = filepath.Join(home, ".config")
	}
	file := filepath.Join(dir, "dbmeta", "gcp", "spanner-reader.json")
	if info, err := os.Stat(file); err != nil || !info.Mode().IsRegular() {
		t.Skip("no key file for the principal spanner-reader")
	}
	return file
}

// makeSpannerReader connects as the service account that holds databaseReader
// on the database. It holds no database role, so it reads as an IAM principal,
// and it cannot name a role either, because it lacks
// spanner.databases.useRoleBasedAccess.
func makeSpannerReader(t *testing.T, _ *sql.DB, dsn, _ string) string {
	t.Helper()
	return dsn + ";credentials=" + spannerReaderKey(t, dsn)
}

// makeSpannerRole connects as the administrator and names the database role
// dbmeta_reader, which holds SELECT on one table, one column, one view, one
// change stream and one function, so that fine grained access control filters
// what it reads. Spanner Omni enforces a role too. The property is
// database_role, and a name that go-sql-spanner does not know, such as role, is
// ignored without a word. See D219.
func makeSpannerRole(t *testing.T, _ *sql.DB, dsn, _ string) string {
	t.Helper()
	return dsn + ";database_role=" + spfixture.Reader
}

// makeSpannerStranger names the role dbmeta_stranger, which holds no grant at
// all.
func makeSpannerStranger(t *testing.T, _ *sql.DB, dsn, _ string) string {
	t.Helper()
	return dsn + ";database_role=" + spfixture.Stranger
}
