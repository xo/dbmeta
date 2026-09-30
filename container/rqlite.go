package container

import (
	"fmt"
	"net/url"

	"github.com/xo/dbmeta"
)

// The rqlite releases dbrun starts.
//
// rqlite is SQLite behind an HTTP API, and models/rqlite shares the sqlite3
// model's statements (D148). The releases were here first for the tests of
// the rqlite driver in github.com/xo/dbimp (D112), which the tests here use
// from dbimp v0.8.0 (D151).
//
// # The range
//
// rqlite never rebuilds a tag: each tag on docker.io/rqlite/rqlite is built
// once, on the day of its release. So the rule in D112 applies instead of step
// 2 of docs/EVALUATION.md: the newest release of each of the last two lines.
// Checked on 2026-09-27, that is 10.3.6, released on 2026-09-22, and 9.4.5, the
// last release of 9, on 2026-03-10. rqlite is under the MIT licence.
//
// # The users
//
// rqlite takes its users from a file named by -auth. No environment variable
// names it, so the start command writes the file and then starts the server
// through the image's own entrypoint, which adds the node id and the data
// directory. The file holds rqliteAdmin, with every permission, and
// [RqliteUser], who can query and execute and nothing else: no backup, no
// load, no status and no change to the cluster. rqlite has no administrator of
// its own, so the name is chosen here.

// RqliteUser is the ordinary user of every rqlite release. Its password is
// [Password].
const RqliteUser = "dbmeta_user"

// rqliteAdmin is the user with every permission.
const rqliteAdmin = "admin"

// rqliteServe writes the users and starts the server.
const rqliteServe = `printf '%s' '[{"username":"` + rqliteAdmin + `","password":"` + Password + `","perms":["all"]},` +
	`{"username":"` + RqliteUser + `","password":"` + Password + `","perms":["query","execute"]}]' > /rqlite/file/auth.json &&
exec docker-entrypoint.sh -auth /rqlite/file/auth.json`

// rqlite is the rqlite image.
var rqlite = product{
	name:    "rqlite",
	dialect: dbmeta.Rqlite,
	image:   "docker.io/rqlite/rqlite",
	port:    4001,
	args:    []string{"sh", "-c", rqliteServe},
	// The image has busybox wget and no curl. The check asks for a query as
	// the administrator, so it passes only when a login works.
	ready: []string{"sh", "-c", "wget -q -O- 'http://" + rqliteAdmin + ":" + Password +
		"@127.0.0.1:4001/db/query?q=SELECT%201' | grep -q '\"values\"'"},
	// dbimp's driver takes rqlite:// with no path and no query, and refuses
	// http:// (dbimp D141).
	dsn: rqliteAt(rqliteAdmin),
	users: []Principal{{
		Role: User, User: RqliteUser,
		dsn: rqliteAt(RqliteUser),
	}},
}

// rqliteAt is the address of the HTTP API, with one user's credentials.
func rqliteAt(user string) func(port int) string {
	return func(port int) string {
		u := url.URL{
			Scheme: "rqlite",
			User:   url.UserPassword(user, Password),
			Host:   fmt.Sprintf("127.0.0.1:%d", port),
		}
		return u.String()
	}
}

// Rqlite is every rqlite release dbrun starts. Both are Tested, the cadence
// each recorded while it was Staged (D120).
var Rqlite = list{}.add(rqlite, Tested, "9.4.5", "10.3.6")
