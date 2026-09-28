package container

// The Weaviate releases dbrun starts.
//
// dbmeta has no Weaviate model. The releases are here so that dbrun can start
// a server for the tests of the Weaviate driver in github.com/xo/dbimp, which
// reads the REST and GraphQL interfaces. No dialect is named yet, because
// dbimp settles the name with the driver. See D118.
//
// # The range
//
// docker.io/semitechnologies/weaviate builds each release tag once, and an
// older line still gets new releases, so the rule in D112 applies: the newest
// release of each of the last two lines. Checked on 2026-09-28, that is
// 1.39.7, of 2026-09-25, and 1.38.17, of 2026-09-22. A tag with a commit hash
// in it is a preview build. Weaviate is under the BSD 3-Clause licence.
//
// # Keys, not passwords
//
// Weaviate checks an API key, and the key says who the user is, so each user
// has a key of its own. The administrator admin has [Password], and
// [WeaviateUser] has [WeaviateUserKey]. The admin list makes the second user
// one that may only read. A request sends the key as a bearer token, and the
// DSN carries each key as its password.
//
// # The setup
//
// The image is busybox, which has wget and no curl. Init makes the class
// Dbmeta, which is where Weaviate keeps objects, if it is missing. Weaviate
// starts the name of a class with a capital letter.

// WeaviateUser is the user that may only read.
const WeaviateUser = "dbmeta_user"

// WeaviateUserKey is the key of [WeaviateUser]. It differs from [Password],
// because the key is what says who the user is.
const WeaviateUserKey = Password + "-user"

// weaviateGet is a wget of a path as the administrator.
func weaviateGet(path string) string {
	return "wget -q -O /dev/null --header 'Authorization: Bearer " + Password + "' http://127.0.0.1:8080" + path
}

// weaviate is the Weaviate image.
var weaviate = product{
	name:  "weaviate",
	image: "docker.io/semitechnologies/weaviate",
	port:  8080,
	env: map[string]string{
		"AUTHENTICATION_ANONYMOUS_ACCESS_ENABLED": "false",
		"AUTHENTICATION_APIKEY_ENABLED":           "true",
		"AUTHENTICATION_APIKEY_ALLOWED_KEYS":      Password + "," + WeaviateUserKey,
		"AUTHENTICATION_APIKEY_USERS":             "admin," + WeaviateUser,
		"AUTHORIZATION_ADMINLIST_ENABLED":         "true",
		"AUTHORIZATION_ADMINLIST_USERS":           "admin",
		"AUTHORIZATION_ADMINLIST_READONLY_USERS":  WeaviateUser,
		"PERSISTENCE_DATA_PATH":                   "/var/lib/weaviate",
		"DEFAULT_VECTORIZER_MODULE":               "none",
		"CLUSTER_HOSTNAME":                        "node1",
		// The usage report that goes to the vendor.
		"DISABLE_TELEMETRY": "true",
	},
	ready: []string{"sh", "-c", weaviateGet("/v1/schema")},
	init: []string{"sh", "-c", weaviateGet("/v1/schema/Dbmeta") + " || " +
		weaviateGet("/v1/schema") + ` --header 'Content-Type: application/json' --post-data '{"class":"Dbmeta","vectorizer":"none"}'`},
	dsn:   keyHTTP("admin", Password),
	users: []Principal{{Role: User, User: WeaviateUser, dsn: keyHTTP(WeaviateUser, WeaviateUserKey)}},
}

// Weaviate is every Weaviate release dbrun starts.
//
// Staged, because dbmeta has no model that reads it, so CI runs none of
// them. Each keeps the cadence it would have if a model read it, which is
// what dbimp runs on each push and at night. See D119 and D120.
var Weaviate = list{}.staged(weaviate, Tested, "1.38.17", "1.39.7")
