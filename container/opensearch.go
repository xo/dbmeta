package container

// The OpenSearch releases dbrun starts.
//
// dbmeta has no OpenSearch model. The releases are here so that dbrun can
// start a server for the tests of the OpenSearch driver in
// github.com/xo/dbimp, which sends SQL to /_plugins/_sql on the HTTP
// interface. No dialect is named yet, because dbimp settles the name with the
// driver. See D118.
//
// # The range
//
// docker.io/opensearchproject/opensearch builds each release tag once, so the
// rule in D112 applies: the newest release of each of the last two lines.
// Checked on 2026-09-28, that is 3.8.0 and 2.19.6, both of 2026-09-15.
// OpenSearch is under the Apache 2.0 licence.
//
// # The password
//
// From 2.12 the image needs an initial password for the administrator admin,
// and its installer rates it for strength with a rule of its own. It rates
// [Password] weak and refuses it, and no setting changes that rule. Ken chose
// on 2026-09-28 to lower the rating rather than give OpenSearch a password of
// its own. So the image starts with [openSearchBootstrap], which the
// installer accepts, and Init gives admin [Password].
//
// admin is a reserved user, and the REST interface refuses to change it. So
// the command runs the installer itself, and writes the hash of [Password]
// into internal_users.yml before the server first starts and loads its users
// from that file. securityadmin.sh, which loads the file into a running
// server, speaks HTTPS, and the entry turns TLS off on the HTTP port, so a
// password works over plain HTTP. The REST interface rates the password of
// any other user by a setting, and the entry lowers that setting to fair, so
// [OpenSearchUser] can have [Password].
//
// # The users
//
// Init makes the role dbmeta_role, which may read the indices whose names
// start with dbmeta, [OpenSearchUser], and the mapping of the one to the
// other, and the index dbmeta. Each is a PUT, which is safe to run twice.

// OpenSearchUser may only read the indices whose names start with dbmeta.
// Its password is [Password].
const OpenSearchUser = "dbmeta_user"

// openSearchBootstrap is the password the first start gives admin, strong
// enough for the installer. Init replaces it with [Password].
const openSearchBootstrap = "Dbmeta-Bootstrap-7q#Vz"

// openSearchServe runs the demo installer once, with [openSearchBootstrap],
// and puts the hash of [Password] in place of the one it wrote for admin,
// before the server first starts and loads its users from the file. Then it
// starts the image's own entrypoint with the installer turned off, because
// the installer has run.
const openSearchServe = `set -e
cd /usr/share/opensearch
if [ ! -f config/.dbmeta ]; then
	OPENSEARCH_INITIAL_ADMIN_PASSWORD='` + openSearchBootstrap + `' bash plugins/opensearch-security/tools/install_demo_configuration.sh -y -i -s
	h=$(plugins/opensearch-security/tools/hash.sh -p '` + Password + `' | tail -1)
	sed -i "/^admin:/,/^[a-z]/ s|^  hash: .*|  hash: \"$h\"|" config/opensearch-security/internal_users.yml
	touch config/.dbmeta
fi
export DISABLE_INSTALL_DEMO_CONFIG=true
exec ./opensearch-docker-entrypoint.sh opensearch`

// osCurl is a curl of the HTTP interface as the administrator.
const osCurl = "curl -sf -o /dev/null -u 'admin:" + Password + "' -H 'Content-Type: application/json' "

// opensearch is the OpenSearch image.
var opensearch = product{
	name:  "opensearch",
	image: "docker.io/opensearchproject/opensearch",
	port:  9200,
	env: map[string]string{
		"discovery.type":                    "single-node",
		"plugins.security.ssl.http.enabled": "false",
		"plugins.security.restapi.password_score_based_validation_strength": "fair",
		"DISABLE_PERFORMANCE_ANALYZER_AGENT_CLI":                            "true",
		"OPENSEARCH_JAVA_OPTS":                                              "-Xms1g -Xmx1g",
	},
	ready:    []string{"sh", "-c", osCurl + `-d '{"query":"SELECT 1"}' http://127.0.0.1:9200/_plugins/_sql`},
	runFlags: []string{"--entrypoint", "/bin/bash"},
	args:     []string{"-c", openSearchServe},
	init: []string{"sh", "-c", `set -e
api=http://127.0.0.1:9200/_plugins/_security/api
` + osCurl + `-X PUT -d '{"index_permissions":[{"index_patterns":["dbmeta*"],"allowed_actions":["read","indices:admin/mappings/get","indices:monitor/settings/get"]}]}' $api/roles/dbmeta_role
` + osCurl + `-X PUT -d '{"password":"` + Password + `"}' $api/internalusers/` + OpenSearchUser + `
` + osCurl + `-X PUT -d '{"users":["` + OpenSearchUser + `"]}' $api/rolesmapping/dbmeta_role
` + osCurl + `-I http://127.0.0.1:9200/dbmeta || ` + osCurl + `-X PUT http://127.0.0.1:9200/dbmeta`},
	dsn:   keyHTTP("admin", Password),
	users: []Principal{{Role: User, User: OpenSearchUser, dsn: keyHTTP(OpenSearchUser, Password)}},
}

// OpenSearch is every OpenSearch release dbrun starts.
//
// Staged, because dbmeta has no model that reads it, so CI runs none of
// them. See D119.
var OpenSearch = list{}.add(opensearch, Staged, "2.19.6", "3.8.0")
