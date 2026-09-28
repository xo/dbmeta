package container

// The TerminusDB releases dbrun starts.
//
// dbmeta has no TerminusDB model. The releases are here so that dbrun can
// start a server for the tests of the TerminusDB driver in
// github.com/xo/dbimp, which sends WOQL to the HTTP interface. No dialect is
// named yet, because dbimp settles the name with the driver. See D118.
//
// # The range
//
// docker.io/terminusdb/terminusdb-server builds each release tag once, so the
// rule in D112 applies: the newest release of each of the last two lines.
// Checked on 2026-09-28, that is v12.0.7, of 2026-08-10, and v11.1.17, of
// 2025-11-07. 12.1-rc is a release candidate. TerminusDB is under the Apache
// 2.0 licence.
//
// # The users
//
// The server sets the password of the administrator admin to [Password] when
// it makes its store, on the first start. Init makes [TerminusDBUser] and a
// role that may read and not write, and grants the role on the database
// admin/dbmeta. It works through the terminusdb command, which writes the
// store directly, and it asks before each step whether the object is there.
//
// # The check
//
// The image has bash and no curl, so the check goes through bash's /dev/tcp,
// as the administrator.

// TerminusDBUser is the ordinary user that Init makes. Its password is
// [Password].
const TerminusDBUser = "dbmeta_user"

// terminusdbInit makes the database, the user and the role, and grants the
// role.
var terminusdbInit = `set -e
cd /app/terminusdb
T=./terminusdb
$T db list admin/dbmeta >/dev/null 2>&1 || $T db create admin/dbmeta
$T user get ` + TerminusDBUser + ` >/dev/null 2>&1 || $T user create ` + TerminusDBUser + ` --password '` + Password + `'
$T role get dbmeta_reader >/dev/null 2>&1 || $T role create dbmeta_reader instance_read_access schema_read_access meta_read_access commit_read_access
$T capability grant ` + TerminusDBUser + ` admin/dbmeta dbmeta_reader`

// terminusdb is the TerminusDB image.
var terminusdb = product{
	name:      "terminusdb",
	image:     "docker.io/terminusdb/terminusdb-server",
	tagPrefix: "v",
	port:      6363,
	env:       map[string]string{"TERMINUSDB_ADMIN_PASS": Password},
	ready: bashRequest(6363, "GET", "/api/db", "",
		map[string]string{"Authorization": adminBasic}, 200),
	init:  []string{"bash", "-c", terminusdbInit},
	dsn:   keyHTTP("admin", Password),
	users: []Principal{{Role: User, User: TerminusDBUser, dsn: keyHTTP(TerminusDBUser, Password)}},
}

// TerminusDB is every TerminusDB release dbrun starts.
//
// Staged, because dbmeta has no model that reads it, so CI runs none of
// them. See D119.
var TerminusDB = list{}.add(terminusdb, Staged, "11.1.17", "12.0.7")
