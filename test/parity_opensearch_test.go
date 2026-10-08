package test

import (
	"context"
	"database/sql"
	"net/http"
	"net/url"
	"testing"

	"github.com/xo/dbmeta/container"
	osfixture "github.com/xo/dbmeta/models/opensearch/fixture"
)

// makeOpenSearchUser connects as the ordinary user the dbrun entry makes. Its
// role reads and views the metadata of the indices whose names start with
// dbmeta, can list every index, and holds the cluster permissions
// cluster:monitor/health and cluster:monitor/main, so it reads the release
// (D192). Becoming it is a change to the credentials of the URL.
func makeOpenSearchUser(t *testing.T, _ *sql.DB, dsn, _ string) string {
	t.Helper()
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("parsing %s: %v", dsn, err)
	}
	u.User = url.UserPassword(container.OpenSearchUser, container.Password)
	return u.String()
}

// The users that the parity test makes through the security API.
const (
	openSearchReader = "dbmeta_reader"
	openSearchLister = "dbmeta_lister"
)

// openSearchPatterns are the permissions of the two users. The reader can read,
// list and describe the index dbmeta_author alone, and SHOW TABLES needs
// indices:admin/get on every index, so the plugin refuses it. The lister can
// list every index and read none, so SHOW TABLES works and every DESCRIBE is
// refused.
var openSearchPatterns = map[string]string{
	openSearchReader: `{"index_patterns":["dbmeta_author"],` +
		`"allowed_actions":["read","indices:admin/get","indices:admin/mappings/get","indices:monitor/settings/get"]}`,
	openSearchLister: `{"index_patterns":["*"],"allowed_actions":["indices:admin/get"]}`,
}

// makeOpenSearchReader makes a user who can read the index dbmeta_author and no
// other. A user belongs to the cluster, and a role grants a permission on a
// name, so this is a grantee with less than the ordinary user. The security API
// makes it, as the administrator of dsn.
func makeOpenSearchReader(t *testing.T, _ *sql.DB, dsn, _ string) string {
	t.Helper()
	return makeOpenSearchGrantee(t, dsn, openSearchReader)
}

// makeOpenSearchLister makes a user who can list every index and describe none.
func makeOpenSearchLister(t *testing.T, _ *sql.DB, dsn, _ string) string {
	t.Helper()
	return makeOpenSearchGrantee(t, dsn, openSearchLister)
}

// makeOpenSearchGrantee makes the user name, with the role of openSearchPatterns,
// and returns its dsn. It removes the user when the test ends.
func makeOpenSearchGrantee(t *testing.T, dsn, name string) string {
	t.Helper()
	ctx := t.Context()
	const api = "/_plugins/_security/api"
	role := name + "_role"
	steps := []osfixture.Step{
		{Name: "role", Request: osfixture.Request{
			Method: http.MethodPut, Path: api + "/roles/" + role,
			Body: `{"cluster_permissions":["cluster:monitor/health"],"index_permissions":[` + openSearchPatterns[name] + `]}`,
		}},
		{Name: "user", Request: osfixture.Request{
			Method: http.MethodPut, Path: api + "/internalusers/" + name,
			Body: `{"password":"` + container.Password + `"}`,
		}},
		{Name: "mapping", Request: osfixture.Request{
			Method: http.MethodPut, Path: api + "/rolesmapping/" + role,
			Body: `{"users":["` + name + `"]}`,
		}},
	}
	if err := osRun(ctx, dsn, steps, false); err != nil {
		t.Fatalf("making %s: %v", name, err)
	}
	t.Cleanup(func() {
		//nolint:errcheck // removing the user is best effort
		osRun(context.WithoutCancel(ctx), dsn, []osfixture.Step{
			{Name: "mapping", Request: osfixture.Request{Method: http.MethodDelete, Path: api + "/rolesmapping/" + role}},
			{Name: "user", Request: osfixture.Request{Method: http.MethodDelete, Path: api + "/internalusers/" + name}},
			{Name: "role", Request: osfixture.Request{Method: http.MethodDelete, Path: api + "/roles/" + role}},
		}, true)
	})
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("parsing %s: %v", dsn, err)
	}
	u.User = url.UserPassword(name, container.Password)
	return u.String()
}
