package test

import (
	"context"
	"database/sql"
	"net/http"
	"net/url"
	"testing"

	"github.com/xo/dbmeta/container"
	esfixture "github.com/xo/dbmeta/models/elasticsearch/fixture"
)

// makeElasticsearchUser connects as the ordinary user the dbrun entry makes.
// Its role reads and views the metadata of the indices whose names start with
// dbmeta, and holds no cluster privilege, so GET / is refused to it. Becoming
// it is a change to the credentials of the URL.
func makeElasticsearchUser(t *testing.T, _ *sql.DB, dsn, _ string) string {
	t.Helper()
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("parsing %s: %v", dsn, err)
	}
	u.User = url.UserPassword(container.ElasticsearchUser, container.Password)
	return u.String()
}

// elasticsearchReader is the user makeElasticsearchReader makes.
const elasticsearchReader = "dbmeta_reader"

// makeElasticsearchReader makes a user who can read the index dbmeta_author and
// no other, which Elasticsearch answers by leaving every other index out of
// SYS TABLES and SYS COLUMNS. A user belongs to the cluster, and a role grants a
// privilege on a name, so this is a grantee with less than the ordinary user.
// The security API makes it, as the administrator of dsn.
func makeElasticsearchReader(t *testing.T, _ *sql.DB, dsn, _ string) string {
	t.Helper()
	ctx := t.Context()
	role := elasticsearchReader + "_role"
	steps := []esfixture.Step{
		{Name: "role", Request: esfixture.Request{
			Method: http.MethodPut, Path: "/_security/role/" + role,
			Body: `{"indices":[{"names":["dbmeta_author"],"privileges":["read","view_index_metadata"]}]}`,
		}},
		{Name: "user", Request: esfixture.Request{
			Method: http.MethodPut, Path: "/_security/user/" + elasticsearchReader,
			Body: `{"password":"` + container.Password + `","roles":["` + role + `"]}`,
		}},
	}
	if err := esRun(ctx, dsn, steps, false); err != nil {
		t.Fatalf("making %s: %v", elasticsearchReader, err)
	}
	t.Cleanup(func() {
		//nolint:errcheck // removing the user is best effort
		esRun(context.WithoutCancel(ctx), dsn, []esfixture.Step{
			{Name: "user", Request: esfixture.Request{Method: http.MethodDelete, Path: "/_security/user/" + elasticsearchReader}},
			{Name: "role", Request: esfixture.Request{Method: http.MethodDelete, Path: "/_security/role/" + role}},
		}, true)
	})
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("parsing %s: %v", dsn, err)
	}
	u.User = url.UserPassword(elasticsearchReader, container.Password)
	return u.String()
}
