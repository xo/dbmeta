package container

// The Apache CouchDB releases dbrun starts.
//
// dbmeta has no CouchDB model. The releases are here so that dbrun can start
// a server for the tests of the CouchDB driver in github.com/xo/dbimp, which
// sends Mango queries to the HTTP interface. No dialect is named yet, because
// dbimp settles the name with the driver. See D118.
//
// # The range
//
// docker.io/library/couchdb is an official image and is rebuilt, so the
// floor is the oldest release still rebuilt. Checked on 2026-09-28, 3.5.2 and
// 3.4.3 were rebuilt on 2026-09-19, and 3.3.3 was last rebuilt on 2025-04-29.
// The tag 3.5.2 holds 3.5.2.1. A tag that ends in -nouveau adds a search
// service and is left out. CouchDB is under the Apache 2.0 license.
//
// # The users
//
// The image makes the administrator admin with [Password] on the first start.
// Init makes [CouchDBUser] in the _users database and names that user as a
// member of the database dbmeta, so it can read and write documents there and
// cannot change the design or the security of the database.
// CouchDB refuses a member that writes a design document with 403. It refuses
// a member that writes the security object with 500 and the reason
// no_majority, and leaves the object as it was.
//
// # The setup
//
// A server that is not in single node mode does not make its system databases,
// so Init makes _users and _replicator. Each step answers 412 or 409 when its
// object is there, and Init accepts that, so it is safe to run twice.

// CouchDBUser is the ordinary user that Init makes. Its password is
// [Password].
const CouchDBUser = "dbmeta_user"

// couchdbInit makes the system databases, the database dbmeta and the user.
var couchdbInit = `set -e
put() {
	code=$(curl -s -o /dev/null -w '%{http_code}' -u 'admin:` + Password + `' -X PUT -H 'Content-Type: application/json' "http://127.0.0.1:5984$1" ${2:+-d "$2"})
	case $code in 200|201|202|409|412) ;; *) echo "PUT $1 answered $code" >&2; exit 1 ;; esac
}
put /_users
put /_replicator
put /dbmeta
put /_users/org.couchdb.user:` + CouchDBUser + ` '{"name":"` + CouchDBUser + `","password":"` + Password + `","roles":[],"type":"user"}'
put /dbmeta/_security '{"admins":{"names":["admin"],"roles":[]},"members":{"names":["` + CouchDBUser + `"],"roles":[]}}'`

// couchdb is the Apache CouchDB image.
var couchdb = product{
	name:  "couchdb",
	image: "docker.io/library/couchdb",
	port:  5984,
	env: map[string]string{
		"COUCHDB_USER":     "admin",
		"COUCHDB_PASSWORD": Password,
	},
	// _all_dbs answers only the administrator.
	ready: []string{"sh", "-c", "curl -sf -o /dev/null -u 'admin:" + Password + "' http://127.0.0.1:5984/_all_dbs"},
	init:  []string{"bash", "-c", couchdbInit},
	dsn:   keyHTTP("admin", Password),
	users: []Principal{{Role: User, User: CouchDBUser, dsn: keyHTTP(CouchDBUser, Password)}},
}

// CouchDB is every Apache CouchDB release dbrun starts.
//
// Staged, because dbmeta has no model that reads it, so CI runs none of
// them. Each keeps the cadence it will have if a model reads it, which is
// what dbimp runs on each push and at night. See D119 and D120.
var CouchDB = list{}.staged(couchdb, Tested, "3.4.3", "3.5.2")
