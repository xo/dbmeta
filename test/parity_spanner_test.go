package test

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	spfixture "github.com/xo/dbmeta/models/spanner/fixture"
	"github.com/xo/dbmeta/test/internal/spannerdsn"
)

// The principals of Spanner. On Cloud Spanner dbsetup made one, the credential file
// spanner-reader, whose service account holds databaseReader on the database.
// The file holds a URL in the form of dburl with the path of the key file of the
// reader in credential_file, and the test turns it into the DSN of go-sql-spanner,
// which is the driver these tests open. The DSN holds a path and no secret. A
// database role is a principal on both Cloud Spanner and Spanner Omni. See D218,
// D219 and D229.

// spannerReaderDSN returns the DSN of the reader, and skips when this is not a
// hosted Cloud Spanner or the file is not there. Spanner Omni, which has no
// authentication, has no second IAM principal.
func spannerReaderDSN(t *testing.T, dsn string) string {
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
	body, err := os.ReadFile(filepath.Join(dir, "dbmeta", "credentials", "spanner-reader"))
	if err != nil {
		t.Skip("no credential file for the principal spanner-reader")
	}
	out, err := spannerdsn.FromURL(string(body))
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// makeSpannerReader connects as the service account that holds databaseReader
// on the database. It holds no database role, so it reads as an IAM principal,
// and it cannot name a role either, because it lacks
// spanner.databases.useRoleBasedAccess.
func makeSpannerReader(t *testing.T, _ *sql.DB, dsn, _ string) string {
	t.Helper()
	return spannerReaderDSN(t, dsn)
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
