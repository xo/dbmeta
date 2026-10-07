package test

import (
	"database/sql"
	"net/url"
	"testing"

	"github.com/xo/dbmeta/container"
)

// makeDrillUser connects as the ordinary user the entry makes, who can query
// and cannot change an option or a storage plugin, so becoming it is a change
// to the credentials of the URL. Drill filters nothing in the catalog by user.
func makeDrillUser(t *testing.T, _ *sql.DB, dsn, _ string) string {
	t.Helper()
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("parsing %s: %v", dsn, err)
	}
	u.User = url.UserPassword(container.DrillUser, container.Password)
	return u.String()
}
