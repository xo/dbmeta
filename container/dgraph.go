package container

// The Dgraph releases dbrun starts.
//
// dbmeta has no Dgraph model. The releases are here so that dbrun can start a
// server for the tests of the Dgraph driver in github.com/xo/dbimp, which
// sends DQL to /query on the HTTP interface after it logs in at /login. No
// dialect is named yet, because dbimp settles the name with the driver. See
// D118.
//
// # The range
//
// docker.io/dgraph/standalone builds each release tag once, so the rule in
// D112 applies: the newest release of each of the last two lines. Checked on
// 2026-09-28, that is v25.4.1, of 2026-08-24, and v25.3.8, of 2026-07-09.
// Dgraph is under the Apache 2.0 licence.
//
// # The users
//
// Dgraph has users only while its access control is on, and that needs a
// secret of at least 32 bytes in a file. So the command writes the file and
// starts the image's own script, which runs Zero and Alpha. Dgraph makes the
// administrator groot with the password password. Init logs in, gives groot
// [Password], and makes the group dbmeta, which may read the predicate
// dgraph.type, and [DgraphUser] in that group. Dgraph grants a group each
// predicate by name, so a test that reads more grants more as groot.
// Dgraph numbers a namespace and does not name one, so nothing is named
// dbmeta but the group.

// DgraphUser is the ordinary user that Init makes. Its password is
// [Password].
const DgraphUser = "dbmeta_user"

// dgraphServe writes the secret of the access control and starts the image.
const dgraphServe = `f=/dgraph/acl-secret
[ -s $f ] || head -c 32 /dev/urandom | base64 | head -c 32 > $f
export DGRAPH_ALPHA_ACL="secret-file=$f"
exec /run.sh`

// dgraphLogin prints the access token of groot, logged in with the password
// given, or fails.
const dgraphLogin = `login() {
	curl -sf -X POST http://127.0.0.1:8080/login -d "{\"userid\":\"groot\",\"password\":\"$1\"}" | jq -er .data.accessJWT
}`

// dgraphInit gives groot its password, and makes the group and the user when
// they are missing. A GraphQL error answers 200, so every answer is read for
// one.
var dgraphInit = `set -e
` + dgraphLogin + `
gql() {
	curl -sf -X POST http://127.0.0.1:8080/admin -H "X-Dgraph-AccessToken: $t" -H 'Content-Type: application/graphql' --data-binary "$1" | jq -e '.errors == null' >/dev/null
}
has() {
	curl -sf -X POST http://127.0.0.1:8080/admin -H "X-Dgraph-AccessToken: $t" -H 'Content-Type: application/graphql' --data-binary "$1" | jq -e '.data[] != null' >/dev/null
}
if ! t=$(login '` + Password + `'); then
	t=$(login password)
	gql 'mutation { updateUser(input: {filter: {name: {eq: "groot"}}, set: {password: "` + Password + `"}}) { user { name } } }'
fi
has '{ getGroup(name: "dbmeta") { name } }' ||
	gql 'mutation { addGroup(input: [{name: "dbmeta", rules: [{predicate: "dgraph.type", permission: 4}]}]) { group { name } } }'
has '{ getUser(name: "` + DgraphUser + `") { name } }' ||
	gql 'mutation { addUser(input: [{name: "` + DgraphUser + `", password: "` + Password + `", groups: [{name: "dbmeta"}]}]) { user { name } } }'`

// dgraph is the Dgraph image.
var dgraph = product{
	name:      "dgraph",
	image:     "docker.io/dgraph/standalone",
	tagPrefix: "v",
	port:      8080,
	env: map[string]string{
		// The usage reports that Zero and Alpha send to the vendor.
		"DGRAPH_ZERO_TELEMETRY":  "reports=false;",
		"DGRAPH_ALPHA_TELEMETRY": "reports=false;",
	},
	runFlags: []string{"--entrypoint", "/bin/bash"},
	args:     []string{"-c", dgraphServe},
	// The check runs before Init gives groot its password, so it takes
	// either one.
	ready: []string{"bash", "-c", dgraphLogin + "\nlogin '" + Password + "' >/dev/null || login password >/dev/null"},
	init:  []string{"bash", "-c", dgraphInit},
	dsn:   keyHTTP("groot", Password),
	users: []Principal{{Role: User, User: DgraphUser, dsn: keyHTTP(DgraphUser, Password)}},
}

// Dgraph is every Dgraph release dbrun starts.
//
// Staged, because dbmeta has no model that reads it, so CI runs none of
// them. Each keeps the cadence it would have if a model read it, which is
// what dbimp runs on each push and at night. See D119 and D120.
var Dgraph = list{}.staged(dgraph, Tested, "25.3.8", "25.4.1")
