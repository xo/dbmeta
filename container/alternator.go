package container

import (
	"fmt"
	"net/url"
	"strings"
)

// The ScyllaDB Alternator releases dbrun starts.
//
// Alternator is the interface of ScyllaDB that speaks the DynamoDB API, on
// its own port. It is its own entry, and not a second dialect of the scylla
// entry, because an entry publishes one port and has one DSN. dbmeta has no
// DynamoDB model. The releases are here so that dbrun can start a server for
// usql's godynamo driver and for dbimp. See D118.
//
// # The range
//
// The image and the range are the scylla entry's, which D90 chose: 2025.1 is
// the oldest release the vendor still rebuilds and 2026.3 the newest. The
// scylla entry also has the two between them, and this one does not.
//
// # The users
//
// A DynamoDB access key is the name of a CQL role, and its secret key is the
// salted hash of the role's password, as system.roles holds it. The command
// sets the hash of the superuser cassandra, so its secret is [cassandraHash]
// with the dollar signs unescaped. Init makes [AlternatorUser] with a hash
// that is fixed too, and grants it SELECT on every keyspace, so it can read
// and not write. Alternator keeps a table named t in the keyspace
// alternator_t, and has no database, so nothing is named dbmeta.
//
// Alternator has no PartiQL. godynamo sends SELECT, INSERT, UPDATE and DELETE
// through ExecuteStatement, which Alternator refuses, and LIST TABLES and
// DESCRIBE TABLE through calls it answers.

// AlternatorUser can read every table and write none. Its password is
// [Password].
const AlternatorUser = "dbmeta_user"

// alternatorUserHash is [Password] hashed with SHA-512 crypt and the salt
// dbmetadbmeta, which is the secret key of [AlternatorUser]. It is what this
// prints:
//
//	openssl passwd -6 -salt dbmetadbmeta 'P4ssw0rd!x'
const alternatorUserHash = `$6$dbmetadbmeta$Tw/8k19INomXuI4myHx1GV8scyBNtOXz0lur7MXB/mbxu782OD4p4tQAk.eYqNxXl0tn.LSRNXHp9G4YVaTQs/`

// alternatorAdminHash is the secret key of cassandra.
var alternatorAdminHash = strings.ReplaceAll(cassandraHash, `\$`, `$`)

// alternator is the ScyllaDB image with Alternator on.
var alternator = product{
	name:  "alternator",
	image: scylla.image,
	port:  8000,
	args: append(append([]string{}, scylla.args...),
		"--alternator-port", "8000",
		// The entrypoint otherwise sets the address to the one the node
		// listens on outside, and the check inside cannot reach it.
		"--alternator-address", "0.0.0.0",
		"--alternator-write-isolation", "only_rmw_uses_lwt",
		"--alternator-enforce-authorization", "1",
	),
	// A login through cqlsh proves that the roles answer, and GET / is the
	// health check of Alternator. A signed call is not the check: the curl
	// in the image signs a request that 2025.1 accepts and 2026.3 refuses
	// with "wrong signature", and the Go SDK's signature passes on both.
	ready: []string{"bash", "-c", "echo exit | cqlsh -u cassandra -p cassandra && curl -sf -o /dev/null http://127.0.0.1:8000/"},
	// The statement is in double quotes, so each dollar sign in the hash is
	// escaped.
	init: []string{"bash", "-c", `cqlsh -u cassandra -p cassandra -e "
CREATE ROLE IF NOT EXISTS ` + AlternatorUser + ` WITH HASHED PASSWORD = '` + strings.ReplaceAll(alternatorUserHash, `$`, `\$`) + `' AND LOGIN = true;
GRANT SELECT ON ALL KEYSPACES TO ` + AlternatorUser + `;"`},
	dsn:   dynamoURL("cassandra", alternatorAdminHash),
	api:   bareHTTP,
	users: []Principal{{Role: User, User: AlternatorUser, dsn: dynamoURL(AlternatorUser, alternatorUserHash), api: bareHTTP}},
}

// dynamoURL is the DSN of one key, in the form of dbimp's DynamoDB driver
// and dburl: the key and the secret as the user and the password, at the
// endpoint on the port on the host. The region is the one every release here
// uses (D167).
func dynamoURL(key, secret string) func(port int) string {
	return func(port int) string {
		u := url.URL{
			Scheme:   "dynamodb",
			User:     url.UserPassword(key, secret),
			Host:     fmt.Sprintf("127.0.0.1:%d", port),
			RawQuery: url.Values{"region": {"us-east-1"}}.Encode(),
		}
		return u.String()
	}
}

// bareHTTP is the address of the HTTP endpoint on the port, with no
// credentials. DynamoDB signs a request with the key, and the BigQuery
// emulator takes no credential.
func bareHTTP(port int) string {
	return fmt.Sprintf("http://127.0.0.1:%d", port)
}

// Alternator is every ScyllaDB Alternator release dbrun starts.
//
// Staged, because dbmeta has no model that reads it, so CI runs none of
// them. Each keeps the cadence it will have if a model reads it, which is
// what dbimp runs on each push and at night. See D119 and D120.
var Alternator = list{}.staged(alternator, Tested, "2025.1", "2026.3")
