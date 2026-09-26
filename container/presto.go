package container

import (
	"fmt"
	"time"

	"github.com/xo/dbmeta"
)

// The Presto releases dbmeta is tested against.
//
// # Presto is its own dialect, not a flavor of Trino
//
// The two are the same program forked in 2019, when the original authors left
// Facebook and renamed PrestoSQL to Trino. Six years apart is enough that they
// no longer answer the same questions, and D73 has the measurement. The short
// version is that a shared dialect would branch on which product it was
// talking to rather than on a version, and a dialect that does that is two
// dialects sharing a struct.
//
// MariaDB and MySQL share one dialect and that is not the counter example it
// looks like. They share decades rather than six years, and their catalogs
// still agree on nearly everything.
//
// # The floor, by the docs/EVALUATION.md procedure
//
// Step 2 cannot decide it. Presto publishes a tag per release and rebuilds
// none of them, so "the oldest release whose image is still rebuilt" means
// nothing, which is the same answer Trino gave.
//
// Step 3 gives a floor of one. Presto supports the latest release and
// back-ports nothing.
//
// So the floor is 0.299, which is also the ceiling. It is the only release
// measured and there is no second one to compare against.

// presto is the official image from the Presto Foundation. It configures a
// writable memory catalog, which is what the fixture needs, so nothing is
// built here.
var presto = product{
	dialect: dbmeta.Presto,
	name:    "presto",
	image:   "docker.io/prestodb/presto",
	port:    8080,
	// presto-cli is on the path and defaults to localhost:8080.
	//
	// The check does what the fixture does, because nothing cheaper predicts
	// it. `SELECT 1` is answered by the coordinator alone, so it is ready
	// while the fixture's first statement still fails:
	//
	//	NO_NODES_AVAILABLE: No nodes available to run query
	//
	// Reading system.runtime.nodes was tried and is not enough. It plans to a
	// SOURCE fragment, which said it had to be scheduled on a node, and it
	// still answered three seconds before a write to the memory connector
	// would run. Reasoning from the plan was the mistake: the coordinator
	// serves its own node list before the scheduler will place connector work
	// on it.
	//
	// So the check creates a schema and drops it, which is the operation that
	// was failing. Both statements run in one presto-cli call and it exits
	// non-zero if either fails, so a ready server is one that has just done
	// the thing and cleaned up after itself. Nothing is left behind: dbrun
	// stops polling on the exit code, and a run that leaked the schema would
	// not be the run that returned zero. See D83.
	ready: []string{
		"presto-cli", "--execute",
		"CREATE SCHEMA IF NOT EXISTS memory.dbmeta_ready;" +
			" DROP SCHEMA IF EXISTS memory.dbmeta_ready",
	},
	// Doing the work once is still not enough, because the answer expires.
	// The coordinator refreshes which nodes it will schedule on by polling
	// discovery every five seconds, which the image logs as
	// internal-communication.node-discovery-polling-interval-millis=5000,
	// so a statement that runs now can be refused after the next refresh.
	// Measured in CI: the check created and dropped a schema, and the
	// fixture's first CREATE TABLE failed 1.1 seconds later.
	//
	// Twelve seconds is two of those refreshes and a margin. See D83.
	settle: 12 * time.Second,
	// presto-go-client/v2 takes the catalog and schema in the path and
	// refuses them as query parameters, where it reads any unknown name as a
	// session property and the server rejects it:
	//
	//	INVALID_SESSION_PROPERTY: Unknown session property schema
	//
	// It also refuses http://, which is the scheme the Trino driver wants. So
	// the DSN and the URL are the same string here, which is unusual and is
	// the driver's doing. v1 of this driver took the http:// form, so the
	// disagreement arrived with v2.
	dsn: func(port int) string {
		return fmt.Sprintf("presto://presto@127.0.0.1:%d/memory/default", port)
	},
}

// Presto is every Presto release dbmeta is tested against.
var Presto = list{}.add(presto, Tested, "0.299")
