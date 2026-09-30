package container

// The libSQL releases dbrun starts.
//
// dbmeta has no libSQL model. The release is here so that dbrun can start a
// server for the tests of the libSQL driver in github.com/xo/dbimp, which
// reads the Hrana HTTP API of sqld, the libSQL server. Turso speaks the same
// protocol. No dialect is named yet, because dbimp settles the name with the
// driver. See D112.
//
// # The range
//
// ghcr.io/tursodatabase/libsql-server builds each release tag once and never
// again, so the rule in D112 applies instead of step 2 of
// docs/EVALUATION.md. libSQL has one line, and its newest release is v0.24.33,
// built on 2025-12-19. The tag latest is built from the main branch and is not
// a release. The project says it is maintained and that new work goes into
// Turso. libSQL is under the MIT licence.
//
// # Two tokens
//
// sqld checks a JWT signed with Ed25519 against the public key in
// SQLD_AUTH_JWT_KEY, the base64url of the raw 32 bytes. SQLD_HTTP_AUTH is not
// set, because sqld then takes basic authentication and ignores the key. Ken
// decided on 2026-10-01 that the entry has an ordinary user this way, for
// dbimp's libSQL driver (D153).
//
// The key and the two tokens are fixed test values, the way [Password] is,
// and the private key that signed them is kept nowhere. Each token has the
// header {"alg":"EdDSA","typ":"JWT"} and no exp. The administrator's claim is
// {"a":"rw"}, and the ordinary user's is {"a":"ro"}, which can read and not
// write. A URL carries a token as its password, as dbimp's D94 puts the
// secret in the password. sqld reads no user name, and the URL names the
// principal anyway, as every key and token entry here does (D102).
//
// # No setup
//
// Every request goes to the database named default, so there is nothing to
// make.

// libsqlKey is the public key that checks both tokens.
const libsqlKey = "QVY3Nb6qfUN-F-b2aebrqMonb1B8fex0ib0P7s9k594"

// LibSQLAdminToken is the administrator's token, with the claim {"a":"rw"}.
const LibSQLAdminToken = "eyJhbGciOiJFZERTQSIsInR5cCI6IkpXVCJ9.eyJhIjoicncifQ.x0cgp7k_MMquU5jYds_pV4zkKVtS5jrSySf3JxDokhQbN8Y1n_ob_qkdn-1BT-6L0oWgOgIZzPbjviiPHMYuAA"

// LibSQLUserToken is the ordinary user's token, with the claim {"a":"ro"}.
const LibSQLUserToken = "eyJhbGciOiJFZERTQSIsInR5cCI6IkpXVCJ9.eyJhIjoicm8ifQ.-9nOBn_k64Uou9fT9OqEovv04bKHUiBSbg13mjmyy9xJ30tkH5YCqVE-v70uRS_ja8TMugNHClihwAIbnfC0AA"

// libsqlAdmin names the administrator, and LibSQLUser the ordinary user. A
// token carries no name, so a name is only what the URL and dbrun print.
const (
	libsqlAdmin = "admin"
	LibSQLUser  = "dbmeta_user"
)

// libsql is the libSQL server image.
var libsql = product{
	name:      "libsql",
	image:     "ghcr.io/tursodatabase/libsql-server",
	tagPrefix: "v",
	port:      8080,
	env:       map[string]string{"SQLD_AUTH_JWT_KEY": libsqlKey},
	// The image has bash and neither curl nor wget, so the check writes the
	// request to a socket that bash opens. It passes only when a query as the
	// administrator succeeds.
	ready: []string{"bash", "-c", `b='{"requests":[{"type":"execute","stmt":{"sql":"SELECT 1"}},{"type":"close"}]}'
exec 3<>/dev/tcp/127.0.0.1/8080 &&
printf 'POST /v2/pipeline HTTP/1.0\r\nHost: 127.0.0.1\r\nAuthorization: Bearer ` + LibSQLAdminToken + `\r\nContent-Type: application/json\r\nContent-Length: %d\r\n\r\n%s' ${#b} "$b" >&3 &&
grep -q '"type":"ok"' <&3`},
	dsn:   keyHTTP(libsqlAdmin, LibSQLAdminToken),
	users: []Principal{{Role: User, User: LibSQLUser, dsn: keyHTTP(LibSQLUser, LibSQLUserToken)}},
}

// LibSQL is every libSQL release dbrun starts.
//
// Staged, because dbmeta has no model that reads it, so CI runs none of
// them. Each keeps the cadence it would have if a model read it, which is
// what dbimp runs on each push and at night. See D119 and D120.
var LibSQL = list{}.staged(libsql, Tested, "0.24.33")
