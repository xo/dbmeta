package container

import (
	"fmt"
	"net/url"
)

// The standalone Avatica releases dbrun starts.
//
// Avatica is the wire protocol of Apache Calcite: JDBC calls over HTTP, in
// protobuf or JSON. dbmeta has no Avatica model. The releases are here so that
// dbrun can start a server for the tests of the Avatica driver in
// github.com/xo/dbimp, which tests one driver against each product that speaks
// the protocol. This is the standalone server, and Phoenix is the other.
// Druid was a third until dbimp gave it a driver of its own (D155). No
// dialect is named yet, because dbimp settles the name with the driver. See
// D113.
//
// # The range
//
// docker.io/apache/calcite-avatica-hypersql, which the Calcite project builds,
// builds each release tag once and never again, so the rule in D112 applies:
// the newest release of each of the last two lines. Checked on 2026-09-28,
// that is 1.29.0, built on 2026-09-20, and 1.28.0, on 2026-05-12. Avatica is
// under the Apache 2.0 license.
//
// # The server
//
// The server is Avatica over HSQLDB, held in memory, so every start begins
// empty. It speaks JSON, which dbimp's Avatica driver takes (dbimp D153).
// Protobuf is the server's default, and a JSON request to a server that
// speaks protobuf answers with a protobuf error, so the entry names JSON.
//
// # The ordinary user
//
// The Avatica server checks no user of its own. It passes the user and the
// password of each connection to HSQLDB, which has users. The setup runs as
// SA through the JSON API and makes [AvaticaUser] with [Password], quoted,
// because HSQLDB folds a name that is not quoted to upper case (D155). It
// makes the schema DBMETA and the table DBMETA.READABLE, and grants the user
// SELECT on that table and nothing else. The database is in memory, so the
// setup runs on every start, and a rerun after a failure finds each object
// there and goes on. It ends by reading the table as the user.

// avaticaReady opens a connection and closes it again, in JSON, and passes
// only when the server answers the open in JSON. The images are busybox and
// have wget and no curl.
const avaticaReady = `wget -q -O - --header 'Content-Type: application/json'` +
	` --post-data '{"request":"openConnection","connectionId":"dbrun"}' http://127.0.0.1:8765/ |` +
	` grep -q '"response":"openConnection"' &&` +
	` wget -q -O /dev/null --header 'Content-Type: application/json'` +
	` --post-data '{"request":"closeConnection","connectionId":"dbrun"}' http://127.0.0.1:8765/`

// AvaticaUser is the ordinary user of the standalone Avatica server. It can
// read DBMETA.READABLE and nothing else.
const AvaticaUser = "dbmeta_user"

// avaticaSetup makes the ordinary user and the table it can read, as SA, and
// then reads the table as the user. Avatica answers an error with HTTP 500,
// and wget then exits with an error and prints no body. Each statement that
// makes an object fails that way when a rerun finds the object, so those
// results are ignored, and the read at the end is what decides. Each run
// names its connections with its process id, because a run that dies leaves
// its connections open.
const avaticaSetup = `cd /tmp &&
post() { wget -q -O - --header 'Content-Type: application/json' --post-file "$1" http://127.0.0.1:8765/; }
open() { printf '{"request":"openConnection","connectionId":"%s","info":{"user":"%s","password":"%s"}}' "$1" "$2" "$3" > open.json; post open.json > /dev/null; }
statement() { printf '{"request":"createStatement","connectionId":"%s"}' "$1" > stmt.json; post stmt.json | sed -n 's/.*"statementId":\([0-9]*\).*/\1/p'; }
run() { printf '{"request":"prepareAndExecute","connectionId":"%s","statementId":%s,"sql":"%s","maxRowCount":-1}' "$1" "$2" "$3" > run.json; post run.json; }
close() { printf '{"request":"closeConnection","connectionId":"%s"}' "$1" > close.json; post close.json > /dev/null; }
open "setup$$" SA '' && id=$(statement "setup$$") || exit 1
run "setup$$" "$id" 'CREATE USER \"` + AvaticaUser + `\" PASSWORD '"'` + Password + `'"'' > /dev/null 2>&1
run "setup$$" "$id" 'CREATE SCHEMA DBMETA' > /dev/null 2>&1
run "setup$$" "$id" 'CREATE TABLE DBMETA.READABLE (ID INTEGER)' > /dev/null 2>&1
run "setup$$" "$id" 'GRANT SELECT ON DBMETA.READABLE TO \"` + AvaticaUser + `\"' > /dev/null 2>&1
close "setup$$"
open "check$$" ` + AvaticaUser + ` '` + Password + `' && id=$(statement "check$$") || exit 1
out=$(run "check$$" "$id" 'SELECT ID FROM DBMETA.READABLE')
close "check$$"
case "$out" in *'"response":"executeResults"'*) ;; *) echo "reading DBMETA.READABLE as ` + AvaticaUser + `: $out"; exit 1 ;; esac`

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
		"-p", "8765", "-u", "jdbc:hsqldb:mem:dbmeta", "--serialization", "JSON",
	},
	ready: []string{"sh", "-c", avaticaReady},
	init:  []string{"sh", "-c", avaticaSetup},
	// SA is the administrator of HSQLDB, and it has no password there.
	dsn: avaticaHTTP("SA"),
	url: avaticaURL(url.User("SA")),
	users: []Principal{{
		Role: User, User: AvaticaUser,
		dsn: func(port int) string {
			return (&url.URL{Scheme: "http", User: url.UserPassword(AvaticaUser, Password),
				Host: fmt.Sprintf("127.0.0.1:%d", port)}).String()
		},
		url: avaticaURL(url.UserPassword(AvaticaUser, Password)),
	}},
}

// avaticaURL is the address in the form dbimp's Avatica driver takes, with no
// path (dbimp D156). The driver sends the user and the password in the info of
// openConnection. The dsn stays http:// for dbimp's recorder.
func avaticaURL(user *url.Userinfo) func(port int) string {
	return func(port int) string {
		return (&url.URL{Scheme: "avatica", User: user, Host: fmt.Sprintf("127.0.0.1:%d", port)}).String()
	}
}

// avaticaHTTP is the address of the server as one user. The Avatica server
// checks no user itself. HSQLDB behind the standalone server checks the user,
// and on Phoenix the name only says who a test means to be.
func avaticaHTTP(user string) func(port int) string {
	return func(port int) string {
		return (&url.URL{Scheme: "http", User: url.User(user), Host: fmt.Sprintf("127.0.0.1:%d", port)}).String()
	}
}

// Avatica is every standalone Avatica release dbrun starts.
//
// Staged, because dbmeta has no model that reads it, so CI runs none of
// them. Each keeps the cadence it will have if a model reads it, which is
// what dbimp runs on each push and at night. See D119 and D120.
var Avatica = list{}.staged(avatica, Tested, "1.28.0", "1.29.0")
