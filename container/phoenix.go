package container

import (
	"net/url"
	"time"
)

// The Apache Phoenix releases dbrun starts.
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
// # No ordinary user
//
// Phoenix checks users only through Kerberos and the ACLs of HBase, which the
// image does not configure. So the Query Server takes every request, and there
// is no ordinary user to make, as on InfluxDB 3 Core.

// phoenixServe sets the Query Server to speak JSON and runs the image's own
// start script. The Query Server reads hbase-site.xml, which the script edits
// with xmlstarlet, so the entry adds its one property the same way, once,
// because a restart runs this again. dbimp's Avatica driver speaks JSON alone
// (dbimp D153), and the default is protobuf.
const phoenixServe = `grep -q phoenix.queryserver.serialization /opt/hbase/conf/hbase-site.xml ||
xmlstarlet ed -L -s /configuration -t elem -n property -v '' \
	-s '/configuration/property[last()]' -t elem -n name -v phoenix.queryserver.serialization \
	-s '/configuration/property[last()]' -t elem -n value -v JSON \
	/opt/hbase/conf/hbase-site.xml
exec /start-hbase-phoenix.sh`

// phoenix is the all in one image of HBase, Phoenix and the Query Server.
var phoenix = product{
	name:    "phoenix",
	image:   "docker.io/boostport/hbase-phoenix-all-in-one",
	port:    8765,
	args:    []string{"bash", "-c", phoenixServe},
	ready:   []string{"sh", "-c", avaticaReady},
	startup: 5 * time.Minute,
	// Nothing checks the name. It says who a test means to be, as the name
	// of the Hive entry does.
	dsn: avaticaHTTP("phoenix"),
	url: avaticaURL(url.User("phoenix")),
}

// Phoenix is every Apache Phoenix release dbrun starts.
//
// Staged, because dbmeta has no model that reads Phoenix, so CI runs none
// of them. The image is somebody other than the vendor's, so it is pinned by
// digest as well as by tag, as docs/CONTAINERS.md asks (D88). Without the
// digest, a push to the same tag changes what dbrun starts, and nothing here
// says so. Its cadence is Tested. See D119 and D120.
var Phoenix = list{}.staged(phoenix, Tested, "2.0-5.0").
	on("2.0-5.0", func(s *Server) {
		s.Tag = "2.0-5.0@sha256:0360b932974ef41a278b0dab14c80a865bdb702eb806b8c1b8a1d28ce1207625"
	})
