package container

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/xo/dbmeta"
)

// The SAP HANA releases dbmeta is tested against.
//
// # The floor, by the docs/EVALUATION.md procedure
//
// Step 2 does not decide this one, for the same reason it did not decide
// Trino. saplabs/hanaexpress carries four tags and SAP rebuilds none of them:
// each is one support package stack, published once. So "the oldest release
// whose image is still rebuilt" selects either all of them or only the newest
// and says nothing either way.
//
// Step 3 decides it. SAP HANA 2.0 is the only major SAP publishes an express
// edition of, and the three tags are SPS 07, SPS 08 in two revisions. All
// three are the same major and there is no older one to reach: 1.0 has no
// image at all.
//
// So the range is the three tags, and the floor is 2.00.076. That was
// verified rather than assumed: the whole suite passes on 2.00.076 and on
// 2.00.088 with the same statements, and the conformance and parity records
// recorded against the newer one hold on the older. The model carries no
// version fragment, which is what two support package stacks of one major
// look like.
//
// # What the image needs that no other here does
//
// Two things, and both are why [Server.RunFlags] and [Server.Args] exist.
//
// The entrypoint takes the initial SYSTEM password as a command line
// argument and reads no environment variable for it. There is a file form,
// --passwords-url, and it needs a file mounted into the container, so the
// argument is the smaller of the two.
//
// HANA will not start under the default open file limit. It wants a million
// descriptors and the container gets 1024 soft without being told otherwise.
//
// --dont-check-system skips the check of /proc/sys values, which are the
// host's and cannot be set from inside a container that is not privileged.
var hana = product{
	dialect: dbmeta.HANA,
	name:    "hana",
	image:   "docker.io/saplabs/hanaexpress",
	major:   hanaMajor,
	// The tenant database HXE, on instance 90. The system database is on
	// 39013 and holds the server's own configuration rather than a schema,
	// so a metadata model reads the tenant.
	port: 39017,
	// The one product that will not run inside container.MemoryLimit, and
	// the reason is its accounting rather than its appetite.
	//
	// HANA sizes its own global allocation limit from the cgroup limit it
	// is given and then refuses work when that computed limit is reached,
	// whatever it is actually using. At 4g it starts, passes the readiness
	// check, and then answers the first real query with:
	//
	//	SQL Error 591 - internal error: Allocation failed; Reason: global
	//	allocation limit configured in [memorymanager]
	//	global_allocation_limit reached
	//
	// At 8g it served the same query using 2.54g, so the ceiling was never
	// the memory it needs.
	//
	// 5g and 6g look like they work and do not. A container whose tenant
	// database already exists starts and answers within either, which is
	// what the first measurement caught. A fresh one has to create the
	// tenant during the post start phase, and that is where it fails:
	//
	//	Creating tenant database ...
	//	* 2: general error: Database could not be started;
	//	start databaseServer for database=3 failed SQLSTATE: HY000
	//
	// The container then exits after the readiness check has already
	// passed, so the failure arrives as a refused connection later rather
	// than as a start that did not finish. Measure this one by removing
	// the container first, never by restarting it.
	//
	// So it is 8g, which is what SAP documents as the minimum.
	//
	// The image offers no way to set the limit directly. Its entrypoint
	// takes one hook parameter, the licence flag, and there is no ini hook
	// to write [memorymanager] with.
	memory: "8g",
	// It answered in 108 seconds on a warm image here, which is past the
	// budget every other product keeps to.
	startup: 5 * time.Minute,
	args: []string{
		"--master-password", Password,
		// SAP HANA, express edition refuses to start without this. It
		// accepts the SAP Developer Center Software Developer License
		// Agreement, which Ken agreed to on 2026-09-26 for this project's
		// test containers.
		"--agree-to-sap-license",
		"--dont-check-system",
	},
	ready: []string{
		hanaBin + "hdbsql", "-n", "localhost:39017", "-u", "SYSTEM",
		"-p", Password, "-x", "SELECT 1 FROM DUMMY",
	},
	dsn: func(port int) string {
		return fmt.Sprintf("hdb://SYSTEM:%s@127.0.0.1:%d",
			url.QueryEscape(Password), port)
	},
}

// hanaBin is where the image puts the client tools. The SID and the instance
// number are baked into the path and the image sets neither from outside.
const hanaBin = "/usr/sap/HXE/HDB90/exe/"

// hanaMajor turns a tag into the name a person uses for it. SAP writes a
// release as five parts and a build date, 2.00.088.00.20251110.1, and calls
// it SPS 08 revision 88. The first three are what identifies it.
func hanaMajor(release string) string {
	parts := strings.SplitN(release, ".", 4)
	if len(parts) < 3 {
		return release
	}
	return strings.Join(parts[:3], ".")
}

// HANA is every SAP HANA release dbmeta is tested against.
var HANA = list{}.add(hana, Tested, "2.00.088.00.20251110.1").
	add(hana, Nightly, "2.00.076.00.20240701.1", "2.00.082.00.20250528.1")
