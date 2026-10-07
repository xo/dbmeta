package test

import (
	"context"
	"database/sql"
	"net/url"
	"testing"

	"github.com/xo/dbmeta/container"
)

// makeAvaticaUser connects as the ordinary user the dbrun setup makes, who
// can read the table DBMETA.READABLE and nothing else, so becoming it is a
// change to the credentials of the URL. HSQLDB lists in INFORMATION_SCHEMA
// what the user has a privilege on.
func makeAvaticaUser(t *testing.T, _ *sql.DB, dsn, _ string) string {
	t.Helper()
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("parsing %s: %v", dsn, err)
	}
	u.User = url.UserPassword(container.AvaticaUser, container.Password)
	return u.String()
}

// avaticaGrantee is the user makeAvaticaGrantee makes. The name is quoted, so
// HSQLDB keeps its case, because a login must give the name as it is stored.
const avaticaGrantee = "dbmeta_grantee"

// makeAvaticaGrantee makes a user that holds the role dbmeta_reader, which
// the fixture grants SELECT on the table author, EXECUTE on a procedure and
// USAGE on a sequence, and that the role dbmeta_member makes a member of.
// The administrator makes it, and the test drops it at the end.
func makeAvaticaGrantee(t *testing.T, admin *sql.DB, dsn, _ string) string {
	t.Helper()
	ctx := t.Context()
	//nolint:errcheck // the user is not there on a first run
	admin.ExecContext(ctx, `DROP USER "`+avaticaGrantee+`"`)
	for _, q := range []string{
		`CREATE USER "` + avaticaGrantee + `" PASSWORD '` + container.Password + `'`,
		`GRANT dbmeta_reader TO "` + avaticaGrantee + `"`,
	} {
		if _, err := admin.ExecContext(ctx, q); err != nil {
			t.Fatalf("making %s: %v\n%s", avaticaGrantee, err, q)
		}
	}
	t.Cleanup(func() {
		//nolint:errcheck // removing the user is best effort
		admin.ExecContext(context.WithoutCancel(ctx), `DROP USER "`+avaticaGrantee+`"`)
	})
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("parsing %s: %v", dsn, err)
	}
	u.User = url.UserPassword(avaticaGrantee, container.Password)
	return u.String()
}
