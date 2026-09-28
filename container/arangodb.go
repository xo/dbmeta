package container

import (
	"fmt"
	"net/url"
)

// The ArangoDB releases dbrun starts.
//
// dbmeta has no ArangoDB model. The release is here so that dbrun can start a
// server for the tests of the ArangoDB driver in github.com/xo/dbimp. No
// dialect is named yet, because dbimp settles the name with the driver. See
// D112.
//
// # The range, by the docs/EVALUATION.md procedure
//
// Step 2 decides it. On docker.io/library/arangodb, checked on 2026-09-27,
// 3.12.12 was rebuilt on 2026-09-24. 3.11.14 was last built on 2025-05-24 and
// 3.10.14 on 2024-04-05, and the official image names only the 3.12 line. So
// the floor and the ceiling are both 3.12, and the release moves with each
// patch.
//
// # The licence
//
// 3.12 is under the ArangoDB Community License. It is free for development and
// testing, which D90 counts, and it asks for no acceptance at start.
//
// # The setup
//
// Init runs arangosh, which is in the image, as root. It makes the database
// dbmeta if it is missing, and makes [ArangoDBUser] or resets its password,
// with read and write on that database and nothing on _system. The
// administrator is root with [Password], which ARANGO_ROOT_PASSWORD sets on
// the first start.
//
// # Telemetry
//
// The documents describe --server.telemetrics-api, which stops what arangosh
// reports to the vendor. 3.12.12 logs that the option is obsolete and has no
// effect, so nothing is set.

// ArangoDBUser is the ordinary user that Init makes on every ArangoDB release.
// Its password is [Password].
const ArangoDBUser = "dbmeta_user"

// arangoDatabase is the database that Init makes.
const arangoDatabase = "dbmeta"

// arangoShell runs arangosh as root with a script.
func arangoShell(script string) []string {
	return []string{
		"arangosh", "--server.endpoint", "tcp://127.0.0.1:8529",
		"--server.username", "root", "--server.password", Password,
		"--javascript.execute-string", script,
	}
}

// arangodb is the ArangoDB image.
var arangodb = product{
	name:  "arangodb",
	image: "docker.io/library/arangodb",
	port:  8529,
	env: map[string]string{
		"ARANGO_ROOT_PASSWORD": Password,
		// ArangoDB reads the memory of the host and not the limit of the
		// container. It saw 64 GB on a development machine and sized the
		// memory limit of each query from that. This tells it the 4 GB
		// that the container is allowed.
		"ARANGODB_OVERRIDE_DETECTED_TOTAL_MEMORY": "4G",
	},
	ready: arangoShell("db._version()"),
	init: arangoShell(`const u = require("@arangodb/users");
if (!db._databases().includes("` + arangoDatabase + `")) { db._createDatabase("` + arangoDatabase + `"); }
if (u.exists("` + ArangoDBUser + `")) { u.update("` + ArangoDBUser + `", "` + Password + `"); } else { u.save("` + ArangoDBUser + `", "` + Password + `"); }
u.grantDatabase("` + ArangoDBUser + `", "` + arangoDatabase + `", "rw");`),
	dsn:   arangoHTTP("root"),
	users: []Principal{{Role: User, User: ArangoDBUser, dsn: arangoHTTP(ArangoDBUser)}},
}

// arangoHTTP is the address of the HTTP API, with one user's credentials.
func arangoHTTP(user string) func(port int) string {
	return func(port int) string {
		u := url.URL{
			Scheme: "http",
			User:   url.UserPassword(user, Password),
			Host:   fmt.Sprintf("127.0.0.1:%d", port),
		}
		return u.String()
	}
}

// ArangoDB is every ArangoDB release dbrun starts.
//
// Staged, because dbmeta has no model that reads it, so CI runs none of
// them. Each keeps the cadence it would have if a model read it, which is
// what dbimp runs on each push and at night. See D119 and D120.
var ArangoDB = list{}.staged(arangodb, Tested, "3.12.12")
