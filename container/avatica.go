package container

import (
	"fmt"
	"net/url"
	"strings"
)

// The standalone Avatica releases dbrun starts.
//
// Avatica is the wire protocol of Apache Calcite: JDBC calls over HTTP, in
// protobuf or JSON. dbmeta has no Avatica model. The releases are here so that
// dbrun can start a server for the tests of the Avatica driver in
// github.com/xo/dbimp, which tests one driver against each product that speaks
// the protocol. This is the standalone server, and Druid and Phoenix are the
// others. No dialect is named yet, because dbimp settles the name with the
// driver. See D113.
//
// # The range
//
// docker.io/apache/calcite-avatica-hypersql, which the Calcite project builds,
// builds each release tag once and never again, so the rule in D112 applies:
// the newest release of each of the last two lines. Checked on 2026-09-28,
// that is 1.29.0, built on 2026-09-20, and 1.28.0, on 2026-05-12. Avatica is
// under the Apache 2.0 licence.
//
// # The server
//
// The server is Avatica over HSQLDB, held in memory, so every start begins
// empty. It speaks protobuf, which is the default and what the Go driver
// apache/calcite-avatica-go uses. A request it cannot read answers 500.
//
// # No ordinary user
//
// The Avatica server checks no user of its own. It passes the user and the
// password of each connection to HSQLDB, which has users. A user that SA made
// through the server was then not found by a second connection, measured on
// 2026-09-28, so the entry makes none. docs/BACKLOG.md holds it.

// avaticaMessage encodes a protobuf WireMessage that wraps one request with
// one connection id. The request types here have the connection id as field
// 1, and no other field is set.
func avaticaMessage(request, connection string) []byte {
	field := func(num int, b []byte) []byte {
		// Every length here is under 128 bytes, so it fits in one byte.
		return append([]byte{byte(num<<3 | 2), byte(len(b))}, b...)
	}
	inner := field(1, []byte(connection))
	return append(field(1, []byte("org.apache.calcite.avatica.proto.Requests$"+request)), field(2, inner)...)
}

// printfOctal writes bytes as a printf format of octal escapes, which the
// busybox shell of the image can write to a file.
func printfOctal(b []byte) string {
	var s strings.Builder
	for _, c := range b {
		fmt.Fprintf(&s, `\%03o`, c)
	}
	return s.String()
}

// avaticaReady opens a connection and closes it again. The image is busybox
// and has wget and no curl, and wget posts a file.
var avaticaReady = "printf '" + printfOctal(avaticaMessage("OpenConnectionRequest", "dbrun")) + "' > /tmp/open &&" +
	" printf '" + printfOctal(avaticaMessage("CloseConnectionRequest", "dbrun")) + "' > /tmp/close &&" +
	" wget -q -O /dev/null --post-file=/tmp/open http://127.0.0.1:8765/ &&" +
	" wget -q -O /dev/null --post-file=/tmp/close http://127.0.0.1:8765/"

// avatica is the standalone Avatica server over an in-memory HSQLDB.
var avatica = product{
	name:  "avatica",
	image: "docker.io/apache/calcite-avatica-hypersql",
	port:  8765,
	// The entry names Java and the whole command. The entrypoint of 1.28.0
	// and 1.27.0 runs /usr/bin/java, which those images do not have: Java is
	// in /opt/java/openjdk, and 1.29.0 fixed the path. The image's own
	// command names a read only sample database, and this one names an
	// empty database in memory.
	runFlags: []string{"--entrypoint", "/opt/java/openjdk/bin/java"},
	args: []string{
		"-cp", "/home/avatica/classpath/*",
		"org.apache.calcite.avatica.standalone.StandaloneServer",
		"-p", "8765", "-u", "jdbc:hsqldb:mem:dbmeta",
	},
	ready: []string{"sh", "-c", avaticaReady},
	// SA is the administrator of HSQLDB, and it has no password there.
	dsn: avaticaHTTP("SA"),
}

// avaticaHTTP is the address of the server as one user. The server checks no
// user, and the name says who a test means to be.
func avaticaHTTP(user string) func(port int) string {
	return func(port int) string {
		return (&url.URL{Scheme: "http", User: url.User(user), Host: fmt.Sprintf("127.0.0.1:%d", port)}).String()
	}
}

// Avatica is every standalone Avatica release dbrun starts.
//
// Staged, because dbmeta has no model that reads it, so CI runs none of
// them. See D119.
var Avatica = list{}.add(avatica, Staged, "1.28.0", "1.29.0")
