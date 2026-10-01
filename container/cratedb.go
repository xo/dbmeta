package container

import (
	"fmt"
	"net/url"

	"github.com/xo/dbmeta"
)

// The CrateDB releases dbrun starts.
//
// CrateDB speaks the PostgreSQL wire protocol on 5432, and it is reached with
// pgx, as dburl's cratedb:// scheme opens it from v0.35.0. There is no driver
// of its own: Ken decided on 2026-09-29 that dbimp writes none, because the
// HTTP interface builds each whole result in memory and has no paging. It has
// a dialect of its own, cratedb, and a model of its own (D123).
//
// CrateDB answers SHOW server_version with 14.0, as if it were PostgreSQL
// 14, and only version() says CrateDB. It has no transactions: ROLLBACK is a
// parse error, so database/sql's Rollback fails, and lib/pq's BEGIN fails
// too. COPY and LISTEN are refused. D123 holds what was measured.
//
// # The range, by the docs/EVALUATION.md procedure
//
// Step 2 decides it. On docker.io/library/crate, checked on 2026-09-27, 6.4.5
// was rebuilt on 2026-09-21 and 6.3.7 on 2026-09-03. 6.2, 6.1 and 6.0 were
// last rebuilt on 2026-07-09, and 5.10 on 2026-01-29. So the floor is 6.3 and
// the ceiling is 6.4. CrateDB is under the Apache 2.0 license.
//
// # The administrator has no password
//
// The superuser is crate, and CrateDB gives it no password and takes none. The
// start command turns on host based authentication, which trusts crate and
// asks every other user for a password. Without it, CrateDB trusts every
// connection and takes the user name from the request, so the ordinary user's
// password checks nothing.
//
// # The setup
//
// The entry publishes 5432, the PostgreSQL port. The HTTP interface on 4200
// is still there inside the container, and the check and Init use it.
//
// Init runs crash, the CrateDB shell in the image, as crate. It makes
// [CrateDBUser] or resets its password, and grants it DQL, DML and DDL on the
// schema dbmeta. CrateDB has schemas and no databases, and a schema exists
// once a table is made in it, so the grant names a schema that is empty.

// CrateDBUser is the ordinary user that Init makes on every CrateDB release.
// Its password is [Password].
const CrateDBUser = "dbmeta_user"

// crateSchema is the schema that the ordinary user can use.
const crateSchema = "dbmeta"

// crateShell runs one statement in crash as crate.
func crateShell(stmt string) string {
	return `crash --hosts localhost:4200 -U crate -c "` + stmt + `"`
}

// cratedb is the CrateDB image.
var cratedb = product{
	name:    "cratedb",
	dialect: dbmeta.CrateDB,
	image:   "docker.io/library/crate",
	port:    5432,
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
	dsn:   cratePG("crate", false),
	users: []Principal{{Role: User, User: CrateDBUser, dsn: cratePG(CrateDBUser, true)}},
}

// cratePG is the address of the PostgreSQL port as one user, in the form
// pgx, lib/pq and dburl all take. The database in the path is doc, the schema
// CrateDB uses when none is named.
//
// crate has no password, and the address says so with an empty one, as
// crate:@. usql read postgres://crate@ with no colon as the user postgres,
// measured on 2026-09-29, because dburl's passfile package put the user of a
// passfile entry in place of the URL's. dburl's D31 fixed that in v0.35.0,
// and the empty password reads the same everywhere, before the fix or after.
func cratePG(user string, password bool) func(port int) string {
	return func(port int) string {
		u := url.URL{
			Scheme:   "postgres",
			User:     url.UserPassword(user, ""),
			Host:     fmt.Sprintf("127.0.0.1:%d", port),
			Path:     "/doc",
			RawQuery: "sslmode=disable",
		}
		if password {
			u.User = url.UserPassword(user, Password)
		}
		return u.String()
	}
}

// CrateDB is every CrateDB release dbrun starts. Both are Tested, which is
// the cadence they kept while they were Staged (D120).
var CrateDB = list{}.add(cratedb, Tested, "6.3.7", "6.4.5")
