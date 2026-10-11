package test

import (
	"database/sql"
	"testing"

	spfixture "github.com/xo/dbmeta/models/spanner/fixture"
)

// The principals of Spanner. On Cloud Spanner dbsetup made one, the credential file
// spanner-reader, whose service account holds databaseReader on the database.
// spannerReaderDSN reads it, and each test binary has its own (D231). A database
// role is a principal on both Cloud Spanner and Spanner Omni. See D218, D219 and
// D231.

// makeSpannerReader connects as the service account that holds databaseReader
// on the database. It holds no database role, so it reads as an IAM principal,
// and it cannot name a role either, because it lacks
// spanner.databases.useRoleBasedAccess.
func makeSpannerReader(t *testing.T, _ *sql.DB, _, _ string) string {
	t.Helper()
	return spannerReaderDSN(t)
}

// makeSpannerRole connects as the administrator and names the database role
// dbmeta_reader, which holds SELECT on one table, one column, one view, one
// change stream and one function, so that fine grained access control filters
// what it reads. Spanner Omni enforces a role too. The property is
// database_role in both drivers. See D219.
func makeSpannerRole(t *testing.T, _ *sql.DB, dsn, _ string) string {
	t.Helper()
	return spannerWithRole(dsn, spfixture.Reader)
}

// makeSpannerStranger names the role dbmeta_stranger, which holds no grant at
// all.
func makeSpannerStranger(t *testing.T, _ *sql.DB, dsn, _ string) string {
	t.Helper()
	return spannerWithRole(dsn, spfixture.Stranger)
}
