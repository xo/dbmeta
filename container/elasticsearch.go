package container

// The Elasticsearch releases dbrun starts.
//
// dbmeta has no Elasticsearch model. The releases are here so that dbrun can
// start a server for the tests of the Elasticsearch driver in
// github.com/xo/dbimp, which sends SQL to /_sql on the HTTP interface. No
// dialect is named yet, because dbimp settles the name with the driver. See
// D118.
//
// # The range
//
// docker.io/library/elasticsearch builds each release tag once, and three
// lines still get releases. Checked on 2026-09-28, 9.5.3 and 9.4.6 were built
// on 2026-09-22 and 8.19.22 on 2026-09-23, and 9.3.8, of 2026-08-11, looks
// finished. The rule in D112 gives 9.4.6 and 9.5.3. 8.19 is still
// patched and is what many servers run, so it is the floor, 9.5.3 is the
// ceiling, and 9.4.6 is kept between them. The image is under the Elastic License 2.0,
// the SSPL or the AGPL, and the basic license it makes for itself includes
// security and SQL.
//
// # The users
//
// The image gives the administrator elastic [Password]. Security is on and
// TLS is off, so a password works over plain HTTP. Init makes the role
// dbmeta_role, which can read the indices whose names start with dbmeta,
// [ElasticsearchUser] with that role, and the index dbmeta.

// ElasticsearchUser can only read the indices whose names start with dbmeta.
// Its password is [Password].
const ElasticsearchUser = "dbmeta_user"

// esCurl is a curl of the HTTP interface as the administrator.
const esCurl = "curl -sf -o /dev/null -u 'elastic:" + Password + "' -H 'Content-Type: application/json' "

// elasticsearch is the Elasticsearch image.
var elasticsearch = product{
	name:  "elasticsearch",
	image: "docker.io/library/elasticsearch",
	port:  9200,
	env: map[string]string{
		"ELASTIC_PASSWORD":                     Password,
		"discovery.type":                       "single-node",
		"xpack.security.enabled":               "true",
		"xpack.security.http.ssl.enabled":      "false",
		"xpack.security.transport.ssl.enabled": "false",
		"xpack.ml.enabled":                     "false",
		"ES_JAVA_OPTS":                         "-Xms1g -Xmx1g",
	},
	ready: []string{"sh", "-c", esCurl + `-d '{"query":"SELECT 1"}' 'http://127.0.0.1:9200/_sql?format=json'`},
	init: []string{"sh", "-c", `set -e
` + esCurl + `-X PUT -d '{"indices":[{"names":["dbmeta*"],"privileges":["read","view_index_metadata"]}]}' http://127.0.0.1:9200/_security/role/dbmeta_role
` + esCurl + `-X PUT -d '{"password":"` + Password + `","roles":["dbmeta_role"]}' http://127.0.0.1:9200/_security/user/` + ElasticsearchUser + `
` + esCurl + `-I http://127.0.0.1:9200/dbmeta || ` + esCurl + `-X PUT http://127.0.0.1:9200/dbmeta`},
	dsn:   keyHTTP("elastic", Password),
	users: []Principal{{Role: User, User: ElasticsearchUser, dsn: keyHTTP(ElasticsearchUser, Password)}},
}

// Elasticsearch is every Elasticsearch release dbrun starts.
//
// Staged, because dbmeta has no model that reads it, so CI runs none of
// them. Each keeps the cadence it will have if a model reads it, which is
// what dbimp runs on each push and at night. See D119 and D120.
var Elasticsearch = list{}.staged(elasticsearch, Tested, "8.19.22", "9.5.3").
	staged(elasticsearch, Nightly, "9.4.6")
