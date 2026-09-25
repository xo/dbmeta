package container

import (
	"fmt"

	"github.com/xo/dbmeta"
)

// The Cassandra releases dbmeta is tested against.
//
// # The floor, by the docs/EVALUATION.md procedure
//
// Step 2 of docs/EVALUATION.md decides it: the oldest release whose image is still rebuilt. On
// docker.io/library/cassandra, 2.2 was last rebuilt in August 2021 and is
// dead. 3.0 and 3.11 were rebuilt in November 2025, and 4.0, 4.1 and 5.0 a
// week before this was written. Every one of them has a linux/amd64 build.
//
// So the floor could be 3.0 and it is 3.11. 3.0 and 3.11 carry the same
// system_schema catalog, so 3.0 adds a release without adding an answer, and
// 3.11 is the release people actually ran. The versions above it are all
// covered.
//
// This matters more here than elsewhere, because 3.0 is where the catalog
// changed shape. Before it, the schema lived in system.schema_columnfamilies
// and its siblings. From it, the schema lives in system_schema. A floor at
// 3.11 means one catalog on every release, with no fragment for the older
// form and no way to reach a server that has it.
//
// # The image is built here
//
// The Apache image refuses three things dbmeta has queries for, and its
// entrypoint maps only eight yaml keys to environment variables, none of them
// these. So every release is rebuilt from test/cassandra/Containerfile with
// user defined functions, materialized views, PasswordAuthenticator and
// CassandraAuthorizer turned on. Run test/cassandra/build.sh before the
// tests. usql does the same thing and publishes the result as
// docker.io/usql/cassandra.
//
// The superuser is cassandra and so is its password, which is what Cassandra
// creates and the only pair that works before somebody changes it. It is the
// one product here that does not use [Password], because the password is not
// ours to choose at startup.
//
// # Memory
//
// Cassandra sizes its heap from the memory it can see, and on a machine with
// a lot of it the JVM asks for more than the container is given and is
// killed before it logs anything. The first attempt here exited 137 with an
// empty log. MAX_HEAP_SIZE and HEAP_NEWSIZE are set so the release, rather
// than the host, decides.

// cassandra is the Apache image.
var cassandra = product{
	dialect: dbmeta.Cassandra,
	name:    "cassandra",
	image:   "localhost/dbmeta/cassandra",
	port:    9042,
	env: map[string]string{
		"MAX_HEAP_SIZE": "1G",
		"HEAP_NEWSIZE":  "256M",
	},
	// cqlsh takes no statement worth writing here, and connecting and
	// leaving is the whole test. This is the Oracle pattern and for the same
	// reason: the command holds no quotes, so it stays readable when
	// [Server.HealthCmd] renders it into a CI workflow.
	//
	// The credentials matter for readiness and not only for access. With
	// PasswordAuthenticator the server accepts connections before system_auth
	// can answer, and a cqlsh that does not authenticate reports ready too
	// early.
	ready: []string{"bash", "-c", "echo exit | cqlsh -u cassandra -p cassandra"},
	// The go-cql-driver DSN is a host list and query options rather than a
	// URL, which is what dburl's GenCassandra produces too.
	dsn: func(port int) string {
		return fmt.Sprintf(
			"127.0.0.1:%d?username=cassandra&password=cassandra"+
				"&timeout=30s&connectTimeout=30s", port)
	},
	// The go-cql-driver DSN above is a host list rather than a URL, so a
	// person needs the other form to paste into usql.
	url: func(port int) string {
		return fmt.Sprintf("cassandra://cassandra:cassandra@127.0.0.1:%d/", port)
	},
}

// Cassandra is every Cassandra release dbmeta is tested against.
//
// 3.11 and 5.0 on every push, because they are the two ends. system_views
// arrived in 4.0 and Settings is the one query that gates on it, so a pair
// that spans 4.0 exercises both sides of the only fragment there is. 4.0 and
// 4.1 run nightly.
var Cassandra = list{}.add(cassandra, Tested, "3.11", "5.0").
	add(cassandra, Nightly, "4.0", "4.1")
