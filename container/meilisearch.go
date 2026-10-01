package container

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
)

// The Meilisearch releases dbrun starts.
//
// dbmeta has no Meilisearch model. The releases are here so that dbrun can
// start a server for the tests of the Meilisearch driver in
// github.com/xo/dbimp, which reads the HTTP interface. No dialect is named
// yet, because dbimp settles the name with the driver. See D118.
//
// # The range
//
// docker.io/getmeili/meilisearch builds each release tag once, so the rule in
// D112 applies: the newest release of each of the last two lines. Checked on
// 2026-09-28, that is v1.54.0, of 2026-09-21, and v1.53.2, of 2026-09-07. A
// minor release arrives about every two weeks, so the range goes stale fast.
// The community edition is under the MIT license.
//
// # Keys, not users
//
// Meilisearch has no users. It checks a master key, which is [Password], and
// keys made from it. Init makes a key that can only search and read the index
// dbmeta. Meilisearch computes the value of a key from its identifier and the
// master key, so the identifier is fixed and [MeilisearchReadKey] is known
// before the server starts. A request sends a key as a bearer token, and the
// DSN carries each key as its password.

// MeilisearchUser is the name the read only key goes by. Meilisearch itself
// has no user names.
const MeilisearchUser = "dbmeta_user"

// meilisearchKeyUID is the identifier of the read only key. It is fixed, so
// that the value of the key is fixed too.
const meilisearchKeyUID = "6a0d0f8e-3b1c-4d2a-9e5f-0d8b7c6a5e41"

// MeilisearchReadKey is the key that can only read the index dbmeta. It is
// the HMAC-SHA256 of the key's identifier under the master key, in hex,
// which is how Meilisearch makes the value of a key.
var MeilisearchReadKey = func() string {
	m := hmac.New(sha256.New, []byte(Password))
	m.Write([]byte(meilisearchKeyUID))
	return hex.EncodeToString(m.Sum(nil))
}()

// meilisearchInit makes the index dbmeta and the read only key. Making the
// index is a task that fails when the index is there, which does not fail the
// request, and making the key again answers 409.
var meilisearchInit = `set -e
post() {
	code=$(curl -s -o /dev/null -w '%{http_code}' -H 'Authorization: Bearer ` + Password + `' -H 'Content-Type: application/json' -d "$2" "http://127.0.0.1:7700$1")
	case $code in 200|201|202|409) ;; *) echo "POST $1 answered $code" >&2; exit 1 ;; esac
}
post /indexes '{"uid":"dbmeta"}'
post /keys '{"uid":"` + meilisearchKeyUID + `","name":"` + MeilisearchUser + `","actions":["search","documents.get","indexes.get"],"indexes":["dbmeta"],"expiresAt":null}'`

// meilisearch is the Meilisearch image.
var meilisearch = product{
	name:      "meilisearch",
	image:     "docker.io/getmeili/meilisearch",
	tagPrefix: "v",
	port:      7700,
	env: map[string]string{
		"MEILI_MASTER_KEY": Password,
		// A master key shorter than 16 bytes is refused in production and
		// accepted with a warning in development.
		"MEILI_ENV": "development",
		// The usage report that goes to the vendor.
		"MEILI_NO_ANALYTICS": "true",
	},
	// /health answers without a key, so the check lists the indexes.
	ready: []string{"sh", "-c", "curl -sf -o /dev/null -H 'Authorization: Bearer " + Password + "' http://127.0.0.1:7700/indexes"},
	init:  []string{"sh", "-c", meilisearchInit},
	dsn:   keyHTTP("admin", Password),
	users: []Principal{{Role: User, User: MeilisearchUser, dsn: keyHTTP(MeilisearchUser, MeilisearchReadKey)}},
}

// Meilisearch is every Meilisearch release dbrun starts.
//
// Staged, because dbmeta has no model that reads it, so CI runs none of
// them. Each keeps the cadence it will have if a model reads it, which is
// what dbimp runs on each push and at night. See D119 and D120.
var Meilisearch = list{}.staged(meilisearch, Tested, "1.53.2", "1.54.0")
