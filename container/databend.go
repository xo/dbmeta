package container

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/xo/dbmeta"
)

// The Databend releases dbrun starts.
//
// dbmeta has no Databend model. The releases are here so that dbrun can start
// a server for the tests of the Databend driver in github.com/xo/dbimp. The
// dialect is databend, which dburl and the usql driver
// github.com/datafuselabs/databend-go already name. See D112.
//
// # The range
//
// docker.io/datafuselabs/databend builds each tag once and never again, so the
// rule in D112 applies instead of step 2 of docs/EVALUATION.md. Ken chose the
// newest stable release and the newest weekly release. Checked on 2026-09-28,
// the stable release is v1.2.881, of 2026-04-17, and the weekly release is
// 1.2.948, of 2026-09-21. The vendor publishes the weekly releases as its
// releases, and a new one arrives almost every day, so the ceiling moves
// often. It tags a weekly release with the suffix -nightly, as in
// v1.2.948-nightly. The release is named without the suffix, so that the
// server is databend-1.2.948, and the tag keeps it. The core of Databend is under the Apache 2.0 licence.
//
// # The telemetry is blocked
//
// From v1.2.881, Databend sends a report to telemetry.databend.com at every
// start and every stop, with the operating system, the processors and the
// memory of the machine. Only a paid Enterprise key turns it off. Ken decided
// on 2026-09-27 that the container resolves that host to its own address, so
// the report goes nowhere. See D112.
//
// # The setup
//
// QUERY_DEFAULT_USER and QUERY_DEFAULT_PASSWORD make root, with the role
// account_admin, on every start. Init sends statements to the HTTP API as
// root. It makes the database dbmeta and a role that holds every privilege on
// it, and makes [DatabendUser] again with that role. Databend grants a CREATE
// privilege to a role and not to a user. A statement that fails still answers
// HTTP 200, so each one is checked for an error of null.
//
// # The first start can fail
//
// The image's bootstrap script starts the query server one second after the
// metadata server. On the first start after the pull of v1.2.948-nightly, the
// metadata server was still waiting to become the leader, the query server
// gave up with "cannot connect to http://0.0.0.0:9191", and the server never
// answered. It did not happen again in four fresh starts, each of which
// answered in two or three seconds. docs/BACKLOG.md holds it.

// DatabendUser is the ordinary user that Init makes on every Databend
// release. Its password is [Password].
const DatabendUser = "dbmeta_user"

// databendDatabase is the database that Init makes.
const databendDatabase = "dbmeta"

// databendRole is the role that holds the privileges of the ordinary user.
const databendRole = "dbmeta_role"

// databendSQL sends one statement to the HTTP API as root, and fails unless
// the answer holds no error. The body is in single quotes for the shell, so
// each single quote in the statement ends the quoting, adds a quote and
// begins it again.
func databendSQL(stmt string) string {
	stmt = strings.ReplaceAll(stmt, "'", `'\''`)
	return `curl -sf -u 'root:` + Password + `' -H 'Content-Type: application/json' -d '{"sql":"` + stmt +
		`"}' http://127.0.0.1:8000/v1/query | grep -q '"error":null'`
}

// databend is the Databend image.
var databend = product{
	dialect:   dbmeta.Databend,
	name:      "databend",
	image:     "docker.io/datafuselabs/databend",
	tagPrefix: "v",
	port:      8000,
	env: map[string]string{
		"QUERY_DEFAULT_USER":     "root",
		"QUERY_DEFAULT_PASSWORD": Password,
	},
	runFlags: []string{"--add-host", "telemetry.databend.com:127.0.0.1"},
	ready:    []string{"sh", "-c", databendSQL("SELECT 1")},
	init: []string{"sh", "-c", "set -e\n" +
		databendSQL("CREATE DATABASE IF NOT EXISTS "+databendDatabase) + "\n" +
		databendSQL("CREATE ROLE IF NOT EXISTS "+databendRole) + "\n" +
		databendSQL("GRANT ALL ON "+databendDatabase+".* TO ROLE "+databendRole) + "\n" +
		databendSQL("CREATE OR REPLACE USER "+DatabendUser+" IDENTIFIED BY '"+Password+
			"' WITH DEFAULT_ROLE = '"+databendRole+"'") + "\n" +
		databendSQL("GRANT ROLE "+databendRole+" TO "+DatabendUser) + "\n"},
	dsn:   databendDSN("root", "default"),
	users: []Principal{{Role: User, User: DatabendUser, dsn: databendDSN(DatabendUser, databendDatabase)}},
}

// databendDSN is the URL that github.com/datafuselabs/databend-go takes, and
// the dburl form, for one user on one database.
func databendDSN(user, database string) func(port int) string {
	return func(port int) string {
		u := url.URL{
			Scheme:   "databend",
			User:     url.UserPassword(user, Password),
			Host:     fmt.Sprintf("127.0.0.1:%d", port),
			Path:     "/" + database,
			RawQuery: "sslmode=disable",
		}
		return u.String()
	}
}

// Databend is every Databend release dbrun starts.
//
// Staged, because dbmeta has no model that reads it, so CI runs none of
// them. See D119.
var Databend = list{}.add(databend, Staged, "1.2.881", "1.2.948").
	on("1.2.948", func(s *Server) { s.Tag = "v1.2.948-nightly" })
