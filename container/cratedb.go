package container

import (
	"fmt"
	"net/url"
)

// The CrateDB releases dbrun starts.
//
// dbmeta has no CrateDB model. The releases are here so that dbrun can start a
// server for the tests of the CrateDB driver in github.com/xo/dbimp, which
// reads the HTTP interface. No dialect is named yet, because dbimp settles the
// name with the driver. See D112.
//
// # The range, by the docs/EVALUATION.md procedure
//
// Step 2 decides it. On docker.io/library/crate, checked on 2026-09-27, 6.4.5
// was rebuilt on 2026-09-21 and 6.3.7 on 2026-09-03. 6.2, 6.1 and 6.0 were
// last rebuilt on 2026-07-09, and 5.10 on 2026-01-29. So the floor is 6.3 and
// the ceiling is 6.4. CrateDB is under the Apache 2.0 licence.
//
// # The administrator has no password
//
// The superuser is crate, and CrateDB gives it no password and takes none. The
// start command turns on host based authentication, which trusts crate and
// asks every other user for a password. Without it, CrateDB trusts every
// connection and takes the user name from the request, so the ordinary user's
// password would check nothing.
//
// # The setup
//
// Init runs crash, the CrateDB shell in the image, as crate. It makes
// [CrateDBUser] or resets its password, and grants it DQL, DML and DDL on the
// schema dbmeta. CrateDB has schemas and no databases, and a schema exists
// once a table is made in it, so the grant names a schema that is empty.

// CrateDBUser is the ordinary user that Init makes on every CrateDB release.
// Its password is [Password].
const CrateDBUser = "dbmeta_user"

// crateSchema is the schema that the ordinary user may use.
const crateSchema = "dbmeta"

// crateShell runs one statement in crash as crate.
func crateShell(stmt string) string {
	return `crash --hosts localhost:4200 -U crate -c "` + stmt + `"`
}

// cratedb is the CrateDB image.
var cratedb = product{
	name:  "cratedb",
	image: "docker.io/library/crate",
	port:  4200,
	// Half of the memory of the container is the vendor's advice for the heap,
	// and the limit is 4 GB.
	env: map[string]string{"CRATE_HEAP_SIZE": "1g"},
	args: []string{
		"crate",
		// One node, which also skips the checks a production host needs, such
		// as vm.max_map_count.
		"-Cdiscovery.type=single-node",
		// The usage report goes to the vendor after ten minutes.
		"-Cudc.enabled=false",
		"-Cauth.host_based.enabled=true",
		"-Cauth.host_based.config.0.user=crate",
		"-Cauth.host_based.config.0.method=trust",
		"-Cauth.host_based.config.1.method=password",
	},
	ready: []string{"sh", "-c", crateShell("SELECT 1")},
	init: []string{"sh", "-c", "set -e\n" +
		"{ " + crateShell("CREATE USER "+CrateDBUser+" WITH (password = '"+Password+"')") + " || " +
		crateShell("ALTER USER "+CrateDBUser+" SET (password = '"+Password+"')") + "; } > /dev/null\n" +
		crateShell("GRANT DQL, DML, DDL ON SCHEMA "+crateSchema+" TO "+CrateDBUser) + "\n"},
	dsn:   crateHTTP("crate", false),
	users: []Principal{{Role: User, User: CrateDBUser, dsn: crateHTTP(CrateDBUser, true)}},
}

// crateHTTP is the address of the HTTP interface as one user. crate has no
// password.
func crateHTTP(user string, password bool) func(port int) string {
	return func(port int) string {
		u := url.URL{
			Scheme: "http",
			User:   url.User(user),
			Host:   fmt.Sprintf("127.0.0.1:%d", port),
		}
		if password {
			u.User = url.UserPassword(user, Password)
		}
		return u.String()
	}
}

// CrateDB is every CrateDB release dbrun starts.
//
// Staged, because dbmeta has no model that reads it, so CI runs none of
// them. See D119.
var CrateDB = list{}.add(cratedb, Staged, "6.3.7", "6.4.5")
