package container

import (
	"fmt"

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
	ready: []string{"presto-cli", "--execute", "SELECT 1"},
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
