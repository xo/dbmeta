package container

import (
	"time"
)

// The Apache Phoenix releases dbmeta is tested against.
//
// The Phoenix Query Server speaks Avatica, the wire protocol of Apache
// Calcite, in front of Phoenix on HBase. dbmeta has no Phoenix model. The
// release is here so that dbrun can start a server for the tests of the
// Avatica driver in github.com/xo/dbimp. No dialect is named yet, because
// dbimp settles the name with the driver. See D113.
//
// # The image is an exception
//
// The Apache Phoenix project publishes no image. The only one that runs
// ZooKeeper, HBase, Phoenix and the Query Server in one container is
// docker.io/boostport/hbase-phoenix-all-in-one, which the Go driver
// apache/calcite-avatica-go tests against. Its newest tag, 2.0-5.0, which is
// HBase 2.0 and Phoenix 5.0, was pushed on 2023-03-14 and has not been rebuilt
// since, so step 2 of docs/EVALUATION.md does not admit it. Ken made it an
// exception on 2026-09-28 (D113).
//
// # No user
//
// Phoenix checks users only through Kerberos and the ACLs of HBase, which the
// image does not configure. So the Query Server takes every request, and there
// is no ordinary user to make.

// phoenix is the all in one image of HBase, Phoenix and the Query Server.
var phoenix = product{
	name:    "phoenix",
	image:   "docker.io/boostport/hbase-phoenix-all-in-one",
	port:    8765,
	ready:   []string{"sh", "-c", avaticaReady},
	startup: 5 * time.Minute,
	// Nothing checks the name. It says who a test means to be, as the name
	// of the Hive entry does.
	dsn: avaticaHTTP("phoenix"),
}

// Phoenix is every Apache Phoenix release dbmeta is tested against.
//
// One release on every push, because the image has one line that runs. The
// image is somebody other than the vendor's, so it is pinned by digest as well
// as by tag, as docs/CONTAINERS.md asks (D88): a push to the same tag would
// otherwise change what is tested with nothing here saying so.
var Phoenix = list{}.add(phoenix, Tested, "2.0-5.0").
	on("2.0-5.0", func(s *Server) {
		s.Tag = "2.0-5.0@sha256:0360b932974ef41a278b0dab14c80a865bdb702eb806b8c1b8a1d28ce1207625"
	})
