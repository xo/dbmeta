package test

import (
	"database/sql"
	"net/url"
	"testing"

	"github.com/xo/dbmeta/container"
)

// makeSolrUser connects as the ordinary user the dbrun setup makes, who holds
// the role search and can read a collection and run SQL on it, so becoming it
// is a change to the credentials of the URL.
func makeSolrUser(t *testing.T, _ *sql.DB, dsn, _ string) string {
	t.Helper()
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("parsing %s: %v", dsn, err)
	}
	u.User = url.UserPassword(container.SolrUser, container.Password)
	return u.String()
}
