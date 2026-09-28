package container

import (
	"fmt"
	"net/url"
)

// The CockroachDB releases dbrun starts.
//
// CockroachDB speaks the PostgreSQL wire protocol, and usql reaches it with
// the scheme cockroachdb, which dburl sends to pgx. The entry names no dialect
// yet. models/postgres reads the version with SHOW server_version, which
// CockroachDB answers with a PostgreSQL compatibility number, so the model
// would read it as PostgreSQL 13. The entry takes the postgres dialect when
// the model detects CockroachDB and sets its own version key, which is D44.
// See D118.
//
// # The range
//
// docker.io/cockroachdb/cockroach builds each point release tag once, so the
// rule in D112 applies: the newest release of each of the last two lines.
// Checked on 2026-09-28, that is v26.3.2, of 2026-09-23, and v26.2.7, of
// 2026-09-24. v24.3.36 is kept too, because 24.3 is the oldest line with
// long term support that is still patched, to 2027-05-05. CockroachDB is under
// the CockroachDB Software License, which is source available and needs no
// licence key for a single node.
//
// # The users
//
// The image makes certificates and runs the server in secure mode.
// --accept-sql-without-tls lets a password log in without TLS, which is what
// sslmode=disable asks for. The check and Init log in as root with its client
// certificate, so they work before root has a password. Init gives root
// [Password] and makes the database dbmeta, owned by [CockroachDBOwner], and
// [CockroachDBUser], who may connect to it. That is the superuser, the owner
// and the grantee that PostgreSQL has. Every user may create a table in the
// schema public, as on PostgreSQL 14 and older, so Init gives the schema to
// the owner and takes that right from everybody else.

// CockroachDBOwner owns the database dbmeta. Its password is [Password].
const CockroachDBOwner = "dbmeta_owner"

// CockroachDBUser may connect to the database dbmeta. Its password is
// [Password].
const CockroachDBUser = "dbmeta_user"

// cockroachSQL runs SQL as root with the client certificate.
const cockroachSQL = "cockroach sql --certs-dir=/cockroach/certs --host=127.0.0.1"

// cockroachdb is the CockroachDB image.
var cockroachdb = product{
	name:      "cockroachdb",
	image:     "docker.io/cockroachdb/cockroach",
	tagPrefix: "v",
	port:      26257,
	env: map[string]string{
		// The usage report that goes to the vendor. It is read before the
		// first start.
		"COCKROACH_SKIP_ENABLING_DIAGNOSTIC_REPORTING": "true",
	},
	args:  []string{"start-single-node", "--accept-sql-without-tls"},
	ready: []string{"sh", "-c", cockroachSQL + " -e 'SELECT 1'"},
	init: []string{"sh", "-c", cockroachSQL + ` -e "
ALTER USER root WITH PASSWORD '` + Password + `';
CREATE USER IF NOT EXISTS ` + CockroachDBOwner + ` WITH PASSWORD '` + Password + `';
CREATE USER IF NOT EXISTS ` + CockroachDBUser + ` WITH PASSWORD '` + Password + `';
CREATE DATABASE IF NOT EXISTS dbmeta;
ALTER DATABASE dbmeta OWNER TO ` + CockroachDBOwner + `;
GRANT CONNECT ON DATABASE dbmeta TO ` + CockroachDBUser + `;
ALTER SCHEMA dbmeta.public OWNER TO ` + CockroachDBOwner + `;
REVOKE CREATE ON SCHEMA dbmeta.public FROM public;"`},
	dsn: cockroachURL("postgres", "root"),
	url: cockroachURL("cockroachdb", "root"),
	users: []Principal{
		{Role: User, User: CockroachDBOwner, dsn: cockroachURL("postgres", CockroachDBOwner), url: cockroachURL("cockroachdb", CockroachDBOwner)},
		{Role: User, User: CockroachDBUser, dsn: cockroachURL("postgres", CockroachDBUser), url: cockroachURL("cockroachdb", CockroachDBUser)},
	},
}

// cockroachURL is the address of the database dbmeta as one user, with the
// scheme pgx takes or the one usql takes.
func cockroachURL(scheme, user string) func(port int) string {
	return func(port int) string {
		return fmt.Sprintf("%s://%s:%s@127.0.0.1:%d/dbmeta?sslmode=disable",
			scheme, user, url.QueryEscape(Password), port)
	}
}

// CockroachDB is every CockroachDB release dbrun starts.
//
// Staged until models/postgres detects CockroachDB and gives it a version
// key of its own, so CI runs none of them. Each keeps the cadence it would have, and
// takes it as its tier when the model arrives. See D118, D119 and D120.
var CockroachDB = list{}.staged(cockroachdb, Tested, "26.2.7", "26.3.2").
	staged(cockroachdb, Nightly, "24.3.36")
