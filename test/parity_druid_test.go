package test

import (
	"context"
	"database/sql"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/xo/dbmeta/container"
)

// makeDruidUser connects as the ordinary user the dbrun setup makes, who can
// READ every datasource and nothing else, so becoming it is a change to the
// credentials of the URL. Druid filters INFORMATION_SCHEMA by permission, and
// the user cannot read sys.servers or sys.server_properties.
func makeDruidUser(t *testing.T, _ *sql.DB, dsn, _ string) string {
	t.Helper()
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("parsing %s: %v", dsn, err)
	}
	u.User = url.UserPassword(container.DruidUser, container.Password)
	return u.String()
}

// druidReader is the user makeDruidReader makes.
const druidReader = "dbmeta_reader"

// makeDruidReader makes a user who can READ the datasource author and no
// other, which Druid answers by hiding every other datasource from
// INFORMATION_SCHEMA. A user belongs to the cluster, and a role grants a
// permission on a resource, so this is a grantee with less than the ordinary
// user. The security API of the Coordinator makes it, through the Router.
func makeDruidReader(t *testing.T, _ *sql.DB, dsn, _ string) string {
	t.Helper()
	ctx := t.Context()
	const api = "/proxy/coordinator/druid-ext/basic-security/"
	role := druidReader + "_role"
	for _, r := range []struct{ path, body string }{
		{"authentication/db/basic/users/" + druidReader, ""},
		{"authentication/db/basic/users/" + druidReader + "/credentials", `{"password":"` + container.Password + `"}`},
		{"authorization/db/basic/users/" + druidReader, ""},
		{"authorization/db/basic/roles/" + role, ""},
		{"authorization/db/basic/roles/" + role + "/permissions",
			`[{"resource":{"name":"author","type":"DATASOURCE"},"action":"READ"}]`},
		{"authorization/db/basic/users/" + druidReader + "/roles/" + role, ""},
	} {
		body := r.body
		if body == "" {
			body = "{}"
		}
		// Making the user again answers an error, so the steps that make a
		// name can fail on a second run. The password and the permission are
		// set every time.
		_, err := druidCall(ctx, dsn, http.MethodPost, api+r.path, body)
		if err != nil && !strings.HasSuffix(r.path, "/users/"+druidReader) && !strings.HasSuffix(r.path, "/roles/"+role) {
			t.Fatalf("making %s: %v", druidReader, err)
		}
	}
	t.Cleanup(func() {
		for _, p := range []string{
			"authentication/db/basic/users/" + druidReader,
			"authorization/db/basic/users/" + druidReader,
			"authorization/db/basic/roles/" + role,
		} {
			//nolint:errcheck // removing the user is best effort
			druidCall(context.WithoutCancel(ctx), dsn, http.MethodDelete, api+p, "")
		}
	})
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("parsing %s: %v", dsn, err)
	}
	u.User = url.UserPassword(druidReader, container.Password)
	// The services copy a new user and its permissions from the Coordinator
	// some seconds later, and the Router answers 401 until then.
	deadline := time.Now().Add(time.Minute)
	for {
		_, err := druidCall(ctx, u.String(), http.MethodPost, "/druid/v2/sql", `{"query":"SELECT 1"}`)
		if err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("waiting for %s to be known: %v", druidReader, err)
		}
		time.Sleep(2 * time.Second)
	}
	return u.String()
}
