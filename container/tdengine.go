package container

import (
	"fmt"
	"net/url"
)

// The TDengine releases dbmeta is tested against.
//
// dbmeta has no TDengine model. The releases are here so that dbrun can start
// a server for the tests of the TDengine driver in github.com/xo/dbimp, which
// reads the REST interface of taosAdapter. No dialect is named yet, because
// dbimp settles the name with the driver. See D112.
//
// # The range
//
// docker.io/tdengine/tsdb, the vendor's Community image, builds each tag once
// and never again, so the rule in D112 applies instead of step 2 of
// docs/EVALUATION.md: the newest release of each of the last two lines.
// Checked on 2026-09-27, that is 3.4.2.8, built on 2026-08-31, and 3.3.8.8, the
// last of 3.3, on 2025-12-02. The older image tdengine/tdengine stopped at
// 3.3.6.13. The Community Edition is under the AGPL 3.0 licence.
//
// # The setup
//
// TAOS_ROOT_PASSWORD sets the password of root on the first start. Init runs
// taos, the shell in the image, as root. It makes [TDengineUser] or resets its
// password, with SYSINFO 0 so that it cannot read the state of the server.
// The ordinary user makes the database dbmeta itself, because the Community
// Edition has no GRANT: from 3.4 a GRANT is refused, and before 3.4 it does
// nothing.
//
// The ordinary user is lesser than root in one way only. It cannot read the
// state of the server, such as ins_dnodes. On 3.3.8.8 and 3.4.2.8 it can make
// and drop users, and a user it makes can see dbmeta, measured on 2026-09-28.
// That is the Community Edition and not the setup.
//
// The setting of each variable of the form TAOS_UPPER_SNAKE goes to the
// matching setting in taos.cfg.

// TDengineUser is the ordinary user that Init makes on every TDengine release.
// Its password is [Password].
const TDengineUser = "dbmeta_user"

// tdDatabase is the database that the ordinary user makes.
const tdDatabase = "dbmeta"

// tdREST runs one statement through the REST interface as one user, and fails
// unless the answer holds code 0. An error still answers HTTP 200.
func tdREST(user, stmt string) string {
	return `curl -sf -u '` + user + `:` + Password + `' -d "` + stmt +
		`" http://127.0.0.1:6041/rest/sql | grep -q '"code":0'`
}

// tdengine is the TDengine Community image.
var tdengine = product{
	name:  "tdengine",
	image: "docker.io/tdengine/tsdb",
	port:  6041,
	env: map[string]string{
		"TAOS_ROOT_PASSWORD": Password,
		// The reports that go to the vendor.
		"TAOS_TELEMETRY_REPORTING": "0",
		"TAOS_CRASH_REPORTING":     "0",
		// taoskeeper makes a database named log, which a query of the
		// databases would report, and the explorer is a web interface.
		// Neither is needed, and both use memory.
		"TAOS_DISABLE_KEEPER":   "1",
		"TAOS_DISABLE_EXPLORER": "1",
		"TAOS_MONITOR":          "0",
	},
	ready: []string{"sh", "-c", tdREST("root", "SELECT SERVER_VERSION()")},
	init: []string{"sh", "-c", "set -e\n" +
		"if " + tdREST("root", "SHOW USERS") + " && curl -sf -u 'root:" + Password +
		"' -d 'SHOW USERS' http://127.0.0.1:6041/rest/sql | grep -q '\"" + TDengineUser + "\"'; then\n" +
		"\t" + tdREST("root", "ALTER USER "+TDengineUser+" PASS '"+Password+"'") + "\n" +
		"else\n" +
		"\t" + tdREST("root", "CREATE USER "+TDengineUser+" PASS '"+Password+"' SYSINFO 0") + "\n" +
		"fi\n" +
		tdREST(TDengineUser, "CREATE DATABASE IF NOT EXISTS "+tdDatabase) + "\n"},
	dsn:   tdHTTP("root"),
	users: []Principal{{Role: User, User: TDengineUser, dsn: tdHTTP(TDengineUser)}},
}

// tdHTTP is the address of the REST interface, with one user's credentials.
func tdHTTP(user string) func(port int) string {
	return func(port int) string {
		u := url.URL{
			Scheme: "http",
			User:   url.UserPassword(user, Password),
			Host:   fmt.Sprintf("127.0.0.1:%d", port),
		}
		return u.String()
	}
}

// TDengine is every TDengine release dbmeta is tested against.
//
// Both on every push, because they are the newest of the last two lines.
var TDengine = list{}.add(tdengine, Tested, "3.3.8.8", "3.4.2.8")
