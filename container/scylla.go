package container

import (
	"fmt"

	"github.com/xo/dbmeta"
)

// The ScyllaDB releases dbmeta is tested against.
//
// ScyllaDB is the flavor and Apache Cassandra is the reference product. The
// two share the cql dialect and one model, models/cassandra, and a query is
// written against Cassandra first. dburl reads scylla and scy as aliases of
// the cql scheme, so a consumer reaches this model with no change.
//
// # The floor, by the docs/EVALUATION.md procedure
//
// Step 2 decides it: the oldest release whose image is still rebuilt. On
// docker.io/scylladb/scylla, checked on 2026-09-27, 2025.1 was rebuilt on
// 2026-09-01, 2026.1 on 2026-09-09, and 2026.2 and 2026.3 on 2026-09-13.
// 2025.2, 2025.3 and 2025.4 were last rebuilt between December 2025 and June
// 2026, so the vendor no longer patches them. 6.2, the last open source
// release, was last rebuilt in February 2025. Every one has a linux/amd64
// build.
//
// So the floor is 2025.1 and the ceiling is 2026.3. Releases from 2025.1 are
// source available with a free tier rather than open source, and D90 says
// that qualifies.
//
// # The settings are arguments
//
// The image passes every argument its entrypoint does not know to scylla
// itself, so nothing is built. PasswordAuthenticator and CassandraAuthorizer
// are turned on for the reason the Cassandra image turns them on: Roles,
// RoleGrants and Privileges have nothing to read without them. A user defined
// function is an experimental feature and is turned on too.
//
// The resource arguments are the ones ScyllaDB documents for a container on
// a shared machine: one shard, one gigabyte, and a reactor that does not
// spin a core. Without --memory it sizes itself from the memory it can see,
// which is the host's.
//
// # The superuser is named at startup
//
// Cassandra creates the role cassandra with the password cassandra. 2026.3
// does not: a login as cassandra was refused with "Username and/or password
// are incorrect", and no other role exists. So the superuser is named with
// --auth-superuser-name and --auth-superuser-salted-password, which 2025.1
// takes too. It is cassandra with the password cassandra, the same pair the
// reference product has, so that one set of test helpers logs in to both.
// That makes it the second product here that does not use [Password].

// cassandraHash is the password cassandra, hashed the way
// --auth-superuser-salted-password takes it, with SHA-512 crypt and a fixed
// salt. It is what this prints, with each dollar sign escaped:
//
//	openssl passwd -6 -salt dbmetadbmeta cassandra
//
// The escape is for the entrypoint. It writes the arguments into
// /etc/scylla.d/docker.conf inside double quotes, and a shell reads that file
// before scylla starts. Unescaped, the shell read $6 and the words after each
// dollar sign as variables, and scylla received /KG/EZD65b/ as the hash, so
// no password matched.
const cassandraHash = `\$6\$dbmetadbmeta\$U6VbAa9ibQS2pWMc0OTLfatYXOeWP5wG0SVGQtQO1h7x6RWhP7P0pjQuRcZF13911uw4Xga4SqL/KG/EZD65b/`

// scylla is the ScyllaDB image.
var scylla = product{
	dialect: dbmeta.Cassandra,
	name:    "scylla",
	image:   "docker.io/scylladb/scylla",
	port:    9042,
	args: []string{
		"--smp", "1",
		"--memory", "1G",
		"--overprovisioned", "1",
		"--developer-mode", "1",
		"--disable-version-check",
		"--authenticator", "PasswordAuthenticator",
		"--authorizer", "CassandraAuthorizer",
		"--experimental-features", "udf",
		"--enable-user-defined-functions", "1",
		"--auth-superuser-name", "cassandra",
		"--auth-superuser-salted-password", cassandraHash,
	},
	// The same command as Cassandra's, for the same reason: with
	// PasswordAuthenticator a connection is accepted before the roles can
	// answer, and only a login proves that they can.
	ready: []string{"bash", "-c", "echo exit | cqlsh -u cassandra -p cassandra"},
	dsn: func(port int) string {
		return fmt.Sprintf(
			"127.0.0.1:%d?username=cassandra&password=cassandra"+
				"&timeout=30s&connectTimeout=30s", port)
	},
	url: func(port int) string {
		return fmt.Sprintf("scylla://cassandra:cassandra@127.0.0.1:%d/", port)
	},
}

// Scylla is every ScyllaDB release dbmeta is tested against.
//
// 2025.1 and 2026.3 on every push, because they are the two ends. 2026.1 and
// 2026.2 run nightly.
var Scylla = list{}.add(scylla, Tested, "2025.1", "2026.3").
	add(scylla, Nightly, "2026.1", "2026.2")
