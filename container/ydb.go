package container

import (
	"fmt"
	"net/url"

	"github.com/xo/dbmeta"
)

// The YDB releases dbmeta is tested against.
//
// models/ydb reads them, and usql reaches YDB with the scheme ydb through
// github.com/ydb-platform/ydb-go-sdk. dbrun started them for usql before the
// model existed. See D118 and D161.
//
// # The range
//
// docker.io/ydbplatform/local-ydb builds each release tag once, and a tag
// for a line does not follow its releases, so the rule in D112 applies: the
// newest release of each of the last two lines. Checked on 2026-10-07, that
// is 26.3.1.19 and 26.2.1.14, of 2026-09-11. A tag that ends
// in .ent, -rc or .hotfix is left out. YDB is under the Apache 2.0 license.
//
// # The users
//
// The image runs a whole cluster in one container. Its deploy script makes
// the storage pools without a user, so any setting that turns on a login
// before the cluster is up leaves the server with no pools, and it cannot
// make a table. POSTGRES_USER and YDB_ENFORCE_USER_TOKEN_REQUIREMENT both do
// that. So the image starts with neither, and Init works after it.
//
// The cluster has the user root with no password. Init connects with no user
// and gives root [Password], and then as root makes [YDBUser] and the
// directory /local/dbmeta, and lets that user read it and describe it. YDB
// allows no underscore in a user name, so the name is dbmetauser and not
// dbmeta_user. The server still accepts a connection that has no user, and
// that connection can do anything.
//
// # The address
//
// The driver asks the server for the addresses of its nodes, and the server
// answers with the name and the port inside the container. go_balancer=disable
// makes the driver keep the address it was given.

// YDBUser can only read and describe the directory /local/dbmeta. Its
// password is [Password].
const YDBUser = "dbmetauser"

// ydbAnonymous runs the YDB command with no user.
const ydbAnonymous = "/ydb -e grpc://localhost:2136 -d /local"

// ydbCLI runs the YDB command as root.
const ydbCLI = "/ydb -e grpc://localhost:2136 -d /local --user root --password-file <(printf %s '" + Password + "')"

// ydb is the YDB image.
var ydb = product{
	dialect: dbmeta.YDB,
	name:    "ydb",
	image:   "docker.io/ydbplatform/local-ydb",
	port:    2136,
	env:     map[string]string{"GRPC_PORT": "2136"},
	// The server answers SELECT 1 before the deploy script has made the
	// storage pools, and a table cannot be made until it has. A slow CI runner
	// made the fixture fail in that gap, with "database doesn't have storage
	// pools at all". So the server is ready when it lists a storage pool.
	ready: []string{"bash", "-c", ydbAnonymous +
		" sql -s 'SELECT COUNT(*) FROM `.sys/ds_storage_pools`' --format csv | grep -q '[1-9]'"},
	init: []string{"bash", "-c", `set -e
` + ydbCLI + ` sql -s 'SELECT 1' >/dev/null 2>&1 || ` + ydbAnonymous + ` sql -s "ALTER USER root PASSWORD '` + Password + `'"
` + ydbCLI + ` sql -s "CREATE USER ` + YDBUser + ` PASSWORD '` + Password + `'" 2>/dev/null ||
	` + ydbCLI + ` sql -s "ALTER USER ` + YDBUser + ` PASSWORD '` + Password + `'"
` + ydbCLI + ` scheme describe /local/dbmeta >/dev/null 2>&1 || ` + ydbCLI + ` scheme mkdir /local/dbmeta
` + ydbCLI + " sql -s 'GRANT SELECT ROW, DESCRIBE SCHEMA ON `/local/dbmeta` TO " + YDBUser + "'"},
	dsn:   ydbURL("grpc", "root"),
	url:   ydbURL("ydb", "root"),
	users: []Principal{{Role: User, User: YDBUser, dsn: ydbURL("grpc", YDBUser), url: ydbURL("ydb", YDBUser)}},
}

// ydbURL is the address of the database /local as one user, with the scheme
// the SDK takes or the one usql takes.
func ydbURL(scheme, user string) func(port int) string {
	return func(port int) string {
		return fmt.Sprintf("%s://%s:%s@127.0.0.1:%d/local?go_balancer=disable",
			scheme, user, url.QueryEscape(Password), port)
	}
}

// YDB is every YDB release dbmeta is tested against.
//
// Both are Tested, which is the cadence each recorded while it was Staged.
// See D119 and D120.
var YDB = list{}.add(ydb, Tested, "26.2.1.14", "26.3.1.19")
