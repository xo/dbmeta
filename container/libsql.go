package container

import (
	"encoding/base64"
	"fmt"
	"net/url"
)

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
// # One user
//
// sqld takes HTTP basic authentication from SQLD_HTTP_AUTH, for one user with
// full access. A principal with fewer rights needs a signed JWT with a
// read only claim, and a key pair that a test would have to hold, so there is
// no ordinary user. The administrator is libsqlAdmin with [Password].
//
// # No setup
//
// Every request goes to the database named default, so there is nothing to
// make.

// libsqlAdmin is the one user.
const libsqlAdmin = "admin"

// libsqlBasic is the basic authentication value for libsqlAdmin.
var libsqlBasic = base64.StdEncoding.EncodeToString([]byte(libsqlAdmin + ":" + Password))

// libsql is the libSQL server image.
var libsql = product{
	name:      "libsql",
	image:     "ghcr.io/tursodatabase/libsql-server",
	tagPrefix: "v",
	port:      8080,
	env:       map[string]string{"SQLD_HTTP_AUTH": "basic:" + libsqlBasic},
	// The image has bash and neither curl nor wget, so the check writes the
	// request to a socket that bash opens. It passes only when a query as the
	// administrator succeeds.
	ready: []string{"bash", "-c", `b='{"requests":[{"type":"execute","stmt":{"sql":"SELECT 1"}},{"type":"close"}]}'
exec 3<>/dev/tcp/127.0.0.1/8080 &&
printf 'POST /v2/pipeline HTTP/1.0\r\nHost: 127.0.0.1\r\nAuthorization: Basic ` + libsqlBasic + `\r\nContent-Type: application/json\r\nContent-Length: %d\r\n\r\n%s' ${#b} "$b" >&3 &&
grep -q '"type":"ok"' <&3`},
	dsn: libsqlHTTP,
}

// libsqlHTTP is the address of the HTTP API, with the one user's credentials.
func libsqlHTTP(port int) string {
	u := url.URL{
		Scheme: "http",
		User:   url.UserPassword(libsqlAdmin, Password),
		Host:   fmt.Sprintf("127.0.0.1:%d", port),
	}
	return u.String()
}

// LibSQL is every libSQL release dbrun starts.
//
// Staged, because dbmeta has no model that reads it, so CI runs none of
// them. Each keeps the cadence it would have if a model read it, which is
// what dbimp runs on each push and at night. See D119 and D120.
var LibSQL = list{}.staged(libsql, Tested, "0.24.33")
