package container

import (
	"fmt"
	"net/url"

	"github.com/xo/dbmeta"
)

// The SurrealDB releases dbrun starts.
//
// models/surrealdb reads them, and dbrun also starts them for the tests of
// github.com/xo/dbimp/surrealdb, the driver dburl names. See D103 and D164.
//
// # The range, by the docs/EVALUATION.md procedure
//
// Step 2 decides it. On docker.io/surrealdb/surrealdb, checked on 2026-09-27,
// 2.7.0 was rebuilt on 2026-09-23 and 3.3.0 on 2026-09-24, and between them
// 3.1.6 on 2026-09-01 and 3.2.4 on 2026-08-03. 1.5.6 was last rebuilt in
// November 2024, and 2.6.5 and 3.0.5 in March 2026. Every one has a
// linux/amd64 build, and each tag is the release with a v, such as v3.3.0.
//
// # Storage
//
// The server keeps its data in RocksDB under /tmp/dbmeta, in the container's
// own layer, so that the users and the data survive a stop and a start. The
// image runs as a user that cannot write /data, and RocksDB refused to start
// there. A memory store loses the ordinary user at every stop.
//
// # The setup has no shell
//
// The image is the surreal binary and nothing else: no shell, no curl. Init
// runs surreal import on /dev/stdin, and dbrun sends it the statements as
// [Server.InitInput]. surreal import exits 1 when a statement fails, and
// surreal sql exits 0 whatever happens, which is why it is import. Import
// wants OPTION IMPORT as its first statement.
//
// The statements make the namespace and the database dbmeta if they are
// missing, and make [SurrealDBUser] or reset its password, so they are safe on
// every start. The user holds the role EDITOR on that database: it can define
// and change tables and records there, and it is refused defining a user.
//
// The administrator is root with [Password], which the start command sets.

// SurrealDBUser is the ordinary user that Init makes on every SurrealDB
// release, on the database dbmeta in the namespace dbmeta. Its password is
// [Password].
const SurrealDBUser = "dbmeta_user"

// surrealDBName is the namespace and the database that Init makes.
const surrealDBName = "dbmeta"

// surrealdb is the SurrealDB image.
var surrealdb = product{
	dialect:   dbmeta.SurrealDB,
	name:      "surrealdb",
	image:     "docker.io/surrealdb/surrealdb",
	tagPrefix: "v",
	port:      8000,
	args: []string{
		"start", "--bind", "0.0.0.0:8000",
		"--user", "root", "--pass", Password,
		"rocksdb:/tmp/dbmeta",
	},
	ready: []string{"/surreal", "isready", "--endpoint", "http://127.0.0.1:8000"},
	init: []string{
		"/surreal", "import", "--endpoint", "http://127.0.0.1:8000",
		"--username", "root", "--password", Password,
		"--namespace", surrealDBName, "--database", surrealDBName,
		"/dev/stdin",
	},
	initInput: "OPTION IMPORT;\n" +
		"DEFINE NAMESPACE IF NOT EXISTS " + surrealDBName + ";\n" +
		"DEFINE DATABASE IF NOT EXISTS " + surrealDBName + ";\n" +
		"DEFINE USER OVERWRITE " + SurrealDBUser + " ON DATABASE PASSWORD '" + Password +
		"' ROLES EDITOR;\n",
	// The driver takes only the surrealdb:// URL, so dbrun connects with
	// it, and the DSN stays the http:// address that other projects read.
	connectURL: true,
	dsn:        surrealDBHTTP("root"),
	url:        surrealDBURL("root", ""),
	users: []Principal{{
		Role: User, User: SurrealDBUser,
		dsn: surrealDBHTTP(SurrealDBUser), url: surrealDBURL(SurrealDBUser, "database"),
	}},
}

// surrealDBHTTP is the HTTP address of the server, with one user's
// credentials.
func surrealDBHTTP(user string) func(port int) string {
	return func(port int) string {
		u := url.URL{
			Scheme: "http",
			User:   url.UserPassword(user, Password),
			Host:   fmt.Sprintf("127.0.0.1:%d", port),
		}
		return u.String()
	}
}

// surrealDBURL is the URL that github.com/xo/dbimp/surrealdb takes and dburl
// parses: the namespace and the database are the two segments of the path.
// dbimp settled the form in its D47 and D48.
//
// auth names the level the user is defined at, and is empty for root, which
// is the default. A database user sends Surreal-Auth-NS and Surreal-Auth-DB
// to sign in, and root is refused with 401 when it sends them, so the driver
// is told the level rather than trying both. dbimp's D51 settled that.
func surrealDBURL(user, auth string) func(port int) string {
	return func(port int) string {
		u := url.URL{
			Scheme: "surrealdb",
			User:   url.UserPassword(user, Password),
			Host:   fmt.Sprintf("127.0.0.1:%d", port),
			Path:   "/" + surrealDBName + "/" + surrealDBName,
		}
		if auth != "" {
			u.RawQuery = url.Values{"auth": {auth}}.Encode()
		}
		return u.String()
	}
}

// SurrealDB is every SurrealDB release dbmeta is tested against.
//
// models/surrealdb reads all four, so each takes the cadence it recorded
// while it was Staged: 2.7.0 and 3.3.0 on every push, and 3.1.6 and 3.2.4 at
// night (D120, D164). 2.7.0 answers one kind, because a 2.x statement
// cannot read INFO as a value.
var SurrealDB = list{}.add(surrealdb, Tested, "2.7.0", "3.3.0").
	add(surrealdb, Nightly, "3.1.6", "3.2.4")
