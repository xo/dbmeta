package container

import (
	"fmt"
	"net/url"
)

// The Qdrant releases dbrun starts.
//
// dbmeta has no Qdrant model. The releases are here so that dbrun can start a
// server for the tests of the Qdrant driver in github.com/xo/dbimp, which
// reads the REST interface. No dialect is named yet, because dbimp settles the
// name with the driver. See D118.
//
// # The range
//
// docker.io/qdrant/qdrant builds each release tag once, so the rule in D112
// applies: the newest release of each of the last two lines. Checked on
// 2026-09-28, that is v1.19.1, of 2026-09-03, and v1.18.3, of 2026-07-17.
// Qdrant is under the Apache 2.0 licence.
//
// # Keys, not users
//
// Qdrant has no users. It checks an API key, and a second key that may only
// read. The administrator's key is [Password], and [QdrantUser] holds the read
// only key, [QdrantReadKey]. A request sends the key in the api-key header.
// The DSN carries each key as its password.
//
// # The setup
//
// The image has bash and no curl, so every request goes through bash's
// /dev/tcp. Init makes the collection dbmeta, which is where Qdrant keeps
// points, if it is missing.

// QdrantUser is the name the read only key goes by. Qdrant itself has no
// user names.
const QdrantUser = "dbmeta_user"

// QdrantReadKey is the key that may only read. Qdrant refuses a read only key
// that is the same as the administrator's.
const QdrantReadKey = Password + "-read"

// qdrantAdmin is the name the administrator's key goes by.
const qdrantAdmin = "admin"

// qdrant is the Qdrant image.
var qdrant = product{
	name:      "qdrant",
	image:     "docker.io/qdrant/qdrant",
	tagPrefix: "v",
	port:      6333,
	env: map[string]string{
		"QDRANT__SERVICE__API_KEY":           Password,
		"QDRANT__SERVICE__READ_ONLY_API_KEY": QdrantReadKey,
		// The usage report that goes to the vendor.
		"QDRANT__TELEMETRY_DISABLED": "true",
	},
	ready: bashRequest(6333, "GET", "/collections", "", map[string]string{"api-key": Password}, 200),
	init: bashEither(
		bashRequest(6333, "GET", "/collections/dbmeta", "", map[string]string{"api-key": Password}, 200),
		bashRequest(6333, "PUT", "/collections/dbmeta", `{"vectors":{"size":4,"distance":"Cosine"}}`,
			map[string]string{"api-key": Password}, 200)),
	dsn:   keyHTTP(qdrantAdmin, Password),
	users: []Principal{{Role: User, User: QdrantUser, dsn: keyHTTP(QdrantUser, QdrantReadKey)}},
}

// keyHTTP is the address of an HTTP interface, with a name and a key as the
// user and the password.
func keyHTTP(user, key string) func(port int) string {
	return func(port int) string {
		u := url.URL{
			Scheme: "http",
			User:   url.UserPassword(user, key),
			Host:   fmt.Sprintf("127.0.0.1:%d", port),
		}
		return u.String()
	}
}

// Qdrant is every Qdrant release dbrun starts.
//
// Staged, because dbmeta has no model that reads it, so CI runs none of
// them. Each keeps the cadence it would have if a model read it, which is
// what dbimp runs on each push and at night. See D119 and D120.
var Qdrant = list{}.staged(qdrant, Tested, "1.18.3", "1.19.1")
