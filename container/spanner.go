package container

import (
	"fmt"

	"github.com/xo/dbmeta"
)

// The Spanner Omni releases dbrun starts.
//
// Cloud Spanner is a hosted service, and the package hosted names it. Google
// also ships Spanner Omni, the same engine as a container that runs on one
// machine. This entry runs Omni, which is the engine and not a mock of the
// API. The emulator of Google has its own entry below. usql reaches Spanner with
// github.com/googleapis/go-sql-spanner, and so does the test module, because the
// driver of dbimp that dburl v0.50.0 names speaks REST and cannot reach Omni, which
// speaks gRPC only. The Spanner model reads this release. See D117, D118, D215,
// D216 and D229.
//
// # The range
//
// us-docker.pkg.dev/spanner-omni/images/spanner-omni has one long term
// support line at a time. Checked on 2026-10-10, that is 2026.r4-lts. Google
// offers Omni at no charge for development and testing, and a commercial
// edition needs a license from Google.
//
// # Reaching it
//
// The server listens on localhost unless it is told another address, so the
// arguments name every address. The image has a shell and no wget, and its
// own spanner program makes the database. The project and the instance are
// both default, and Omni checks no password unless it is started with an
// authentication method, so the URL names the user admin and nothing checks
// it. The driver needs usePlainText, because the server has no certificate.
//
// # The license
//
// Omni prints a warning that its license expires in 90 days. The warning
// comes from the server and the program, and the server kept working in the
// measurement. A person extends the license with the form that the warning
// names.

// spannerOmni is the Spanner Omni image.
var spannerOmni = product{
	dialect: dbmeta.Spanner,
	name:    "spanner",
	image:   "us-docker.pkg.dev/spanner-omni/images/spanner-omni",
	port:    15000,
	args:    []string{"start-single-server", "--listen-addresses", "0.0.0.0"},
	ready:   []string{"/google/spanner/bin/spanner", "databases", "list"},
	// Init runs again after a failure and on a restart that kept the volume, so it
	// makes the database only when it is not there. See D105.
	init: []string{"sh", "-c", "/google/spanner/bin/spanner databases describe dbmeta >/dev/null 2>&1 || /google/spanner/bin/spanner databases create dbmeta"},
	dsn: func(port int) string {
		return fmt.Sprintf("127.0.0.1:%d/projects/default/instances/default/databases/dbmeta;usePlainText=true", port)
	},
	url: func(port int) string {
		return fmt.Sprintf("spanner://admin@127.0.0.1:%d/default/default/dbmeta?usePlainText=true", port)
	},
}

// Spanner is every Spanner Omni release dbmeta is tested against.
//
// Tested, which is the cadence the release recorded while it was Staged. The
// image is public, because a request for its manifest needs no credential, so
// CI pulls it as it pulls every other image. See D119, D120 and D216.
var Spanner = list{}.add(spannerOmni, Tested, "2026.r4-lts")

// The Cloud Spanner emulator releases dbrun starts.
//
// gcr.io/cloud-spanner-emulator/emulator builds each release tag once. It has
// one line, 1.5, so the rule in D112 gives one release. Checked on
// 2026-10-10, that is 1.5.58, of 2026-09-15. The emulator is under the Apache
// 2.0 license. It runs two programs, emulator_main for gRPC on 9010 and
// gateway_main for REST on 9020. Google's image has no shell, so the
// Containerfile in test/cmd/dbrun/image adds a static busybox to it.
//
// The emulator checks nothing and makes no instance and no database. Init
// makes the instance dbmeta in the project dbmeta, and the database dbmeta in
// it, through the REST interface on 9020. The URL names the REST port, which
// is the address that the driver of dbimp reads, and the DSN names the gRPC
// port for go-sql-spanner. See D225.

// spannerREST posts to the REST interface of the emulator. Making a thing
// that is there answers 409, and busybox wget has no way to accept one status
// and not another, so each step asks first whether the thing is there.
func spannerREST(path, name, body string) string {
	u := "http://127.0.0.1:9020/v1/projects/dbmeta/" + path
	return "wget -q -O /dev/null " + u + "/" + name + " || wget -q -O /dev/null --header 'Content-Type: application/json' --post-data '" +
		body + "' " + u
}

// spannerEmulator is the Cloud Spanner emulator, built here. It serves gRPC on
// 9010 and REST on 9020, and the REST address is what the api field gives.
var spannerEmulator = product{
	name:   "spanneremulator",
	image:  "localhost/dbmeta/spanneremulator",
	port:   9010,
	second: 9020,
	ready:  []string{"sh", "-c", "wget -q -O /dev/null http://127.0.0.1:9020/v1/projects/dbmeta/instanceConfigs"},
	init: []string{"sh", "-c", "set -e\n" +
		spannerREST("instances", "dbmeta",
			`{"instanceId":"dbmeta","instance":{"config":"emulator-config","displayName":"dbmeta","nodeCount":1}}`) + "\n" +
		spannerREST("instances/dbmeta/databases", "dbmeta", "{\"createStatement\":\"CREATE DATABASE `dbmeta`\"}")},
	dsn: func(port int) string {
		return fmt.Sprintf("127.0.0.1:%d/projects/dbmeta/instances/dbmeta/databases/dbmeta;usePlainText=true", port)
	},
	url: func(port int) string {
		return fmt.Sprintf("spanner://admin@127.0.0.1:%d/dbmeta/dbmeta/dbmeta", SecondHostPort(port))
	},
	api: func(port int) string {
		return fmt.Sprintf("http://127.0.0.1:%d", SecondHostPort(port))
	},
}

// SpannerEmulator is every Cloud Spanner emulator release dbrun starts.
//
// Staged, because dbmeta has no model that reads it. The model reads Spanner
// Omni, which answers the real catalog, and the emulator is here for a driver
// that speaks REST, which Omni does not serve. Each release keeps the cadence
// it will have if a model reads it. See D119, D120 and D217.
var SpannerEmulator = list{}.staged(spannerEmulator, Tested, "1.5.58")
