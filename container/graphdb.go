package container

// The Ontotext GraphDB releases dbrun starts.
//
// dbmeta has no GraphDB model. The releases are here so that dbrun can start
// a server for the tests of the SPARQL driver in github.com/xo/dbimp, which
// sends queries to /repositories/{id}. No dialect is named yet, because dbimp
// settles the name with the driver. See D118.
//
// # The range
//
// docker.io/ontotext/graphdb builds each release tag once, so the rule in
// D112 applies: the newest release of each of the last two lines. Checked on
// 2026-09-28, that is 11.5.1, of 2026-09-23, and 11.4.3, of 2026-08-06.
// 12.0.0-TR5 is a pre-release.
//
// # The licence
//
// GraphDB 11 does not run without a licence file. GraphDB Free is a licence
// that a person requests with an email address, and Ken chose on 2026-09-28
// to provision it. dbrun mounts the file at [Server.License] and lists these
// releases only while it finds the file. CI has no licence, and no model
// reads them, so they are Staged. GraphDB 10.8 ran free with no file, and is a line older.
//
// # Not yet measured
//
// No release has started here, because no licence file has been provisioned.
// The steps below follow the vendor's documentation and are the first thing
// to measure when the file arrives.
//
// # The users
//
// GraphDB starts with security off and the administrator admin. While
// security is off, Init makes the repository dbmeta, gives admin [Password],
// makes [GraphDBUser], who may read dbmeta, and turns security on. Once it is
// on, Init sends the administrator's password with each request.

// GraphDBUser may read the repository dbmeta. Its password is [Password].
const GraphDBUser = "dbmeta_user"

// graphdbInit makes the repository and the users, and turns security on.
var graphdbInit = `set -e
g=http://127.0.0.1:7200
j="Content-Type: application/json"
a=
curl -sf $g/rest/security | grep -q true && a="-u admin:` + Password + `"
curl -sf -o /dev/null $a $g/rest/repositories/dbmeta ||
	curl -sf -o /dev/null $a -X POST -H "$j" -d '{"id":"dbmeta","type":"graphdb","title":"dbmeta","params":{}}' $g/rest/repositories
if [ -z "$a" ]; then
	curl -sf -o /dev/null -X PATCH -H "$j" -d '{"password":"` + Password + `"}' $g/rest/security/users/admin
	curl -sf -o /dev/null -X POST -H "$j" -d '{"password":"` + Password + `","grantedAuthorities":["ROLE_USER","READ_REPO_dbmeta"]}' $g/rest/security/users/` + GraphDBUser + `
	curl -sf -o /dev/null -X POST -H "$j" -d 'true' $g/rest/security
fi`

// graphdb is the GraphDB image.
var graphdb = product{
	name:    "graphdb",
	image:   "docker.io/ontotext/graphdb",
	port:    7200,
	license: "/opt/graphdb/home/conf/graphdb.license",
	env:     map[string]string{"GDB_HEAP_SIZE": "2g"},
	ready:   []string{"sh", "-c", "curl -sf -o /dev/null -u 'admin:" + Password + "' http://127.0.0.1:7200/rest/repositories"},
	init:    []string{"sh", "-c", graphdbInit},
	dsn:     keyHTTP("admin", Password),
	users:   []Principal{{Role: User, User: GraphDBUser, dsn: keyHTTP(GraphDBUser, Password)}},
}

// GraphDB is every GraphDB release dbrun starts.
//
// Staged, because dbmeta has no model that reads GraphDB, so CI runs none
// of them. Each also needs a licence file that CI does not have,
// so its cadence is Verified. See D119 and D120.
var GraphDB = list{}.staged(graphdb, Verified, "11.4.3", "11.5.1")
