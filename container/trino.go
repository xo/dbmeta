package container

import (
	"fmt"
	"time"

	"github.com/xo/dbmeta"
)

// The Trino releases dbmeta is tested against.
//
// # The floor, by the docs/EVALUATION.md procedure
//
// Step 2 does not decide this one, and it is the first product here where it
// cannot. Trino publishes a tag per release and never rebuilds an old one, so
// "the oldest release whose image is still rebuilt" selects either every tag
// back to 351 or only the newest, depending on how the question is read.
//
// Step 3 decides it instead. Trino supports one release, the latest, and
// back-ports nothing. There is no long term release and no security branch, so
// the vendor's answer is a floor of one.
//
// That is too narrow to be useful to a consumer, so step 5 sets the range:
// cost, measured. The floor is 476 and the fixture decides it rather than the
// catalog.
//
// 476 and 483 both answer every query and pass the whole suite. 451 starts and
// then refuses the fixture:
//
//	Catalog 'memory' does not support non-null column for column name 'author_id'
//
// The memory connector gained NOT NULL somewhere between the two. Whether the
// queries themselves answer on 451 is unmeasured, because there is
// nothing to read them against.
//
// Gating the fixture does not rescue it, and that is the part worth knowing.
// D53 keeps one conformance section per database rather than one per release,
// on the stated ground that nullability and ordinal position do not change
// between releases. A Trino fixture without NOT NULL reports every column
// nullable. 451 and 476 then need different sections, and the design that
// makes the cross family comparison readable must go. The cost is paid by
// every database to support one release of one of them.
//
// A floor is allowed to be recent. Not every product can be supported back to
// where it stops working, and an engine that changes its connectors release
// by release is the case where trying is most expensive.
//
// 400 and 351 were not measured. Neither can beat 451, so neither can change
// the floor.
//
// The range carries no version fragment. Every query is the same statement on
// both releases, which is what a two release span from one connector change
// looks like.
// trino is the official image, which carries the memory connector the
// fixture needs and needs no configuration to start.
var trino = product{
	dialect: dbmeta.Trino,
	name:    "trino",
	image:   "docker.io/trinodb/trino",
	port:    8080,
	// Trino ships no password by default and the HTTP protocol takes the user
	// from the DSN. The name is arbitrary and the server does not check it
	// until authentication is configured, which the image does not do.
	// Creating a schema and dropping it, for the reason in presto.go: a
	// constant is answered by the coordinator alone and is ready before the
	// server will schedule connector work. Trino has not failed that way in
	// CI and Presto has, and the two are the same server at this level, so
	// this is the same check rather than a different one. See D83.
	ready: []string{
		"trino", "--execute",
		"CREATE SCHEMA IF NOT EXISTS memory.dbmeta_ready;" +
			" DROP SCHEMA IF EXISTS memory.dbmeta_ready",
	},
	// The same settle as Presto, and for the same reason. See presto.go
	// and D83.
	settle: 12 * time.Second,
	// The JVM does not see the container's memory limit here, so the
	// heap that jvm.config gives as 80 percent of memory is 80 percent of
	// the host's. Measured on 2026-09-29 on a 64 GB host: the maximum heap
	// read 38 GB inside a container held to MemoryLimit, and 476 was killed
	// for memory twice while the test module read every query with the
	// system objects. An explicit heap size wins over the percentages, so
	// the heap is held to 2.5 GB and the rest of the JVM fits in the limit.
	env: map[string]string{"JAVA_TOOL_OPTIONS": "-Xms256m -Xmx2560m"},
	dsn: func(port int) string {
		return fmt.Sprintf("http://trino@127.0.0.1:%d?catalog=memory&schema=default", port)
	},
	url: func(port int) string {
		return fmt.Sprintf("trino://trino@127.0.0.1:%d/memory/default", port)
	},
}

// Trino is every Trino release dbmeta is tested against.
var Trino = list{}.add(trino, Tested, "476", "483")
