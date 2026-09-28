package container

import "fmt"

// The Cloud Spanner emulator releases dbrun starts.
//
// Cloud Spanner is a hosted service, and the package hosted names it. Google
// publishes an emulator of it, and this entry runs the emulator, so that a
// person and CI can test without an account. usql reaches Spanner with
// github.com/googleapis/go-sql-spanner. dbmeta has no Spanner model yet. See
// D117 and D118.
//
// # The range
//
// gcr.io/cloud-spanner-emulator/emulator builds each release tag once. It has
// one line, 1.5, so the rule in D112 gives one release. Checked on
// 2026-09-28, that is 1.5.58, of 2026-09-15. The emulator is under the Apache
// 2.0 licence.
//
// # The image is built here
//
// Google's image has no shell, so the Containerfile in test/cmd/dbrun/image
// adds a static busybox to it.
//
// # No users
//
// The emulator checks nothing and makes no instance and no database. Init
// makes the instance dbmeta in the project dbmeta, and the database dbmeta in
// it, through the REST interface on 9020. The driver speaks gRPC on 9010. The
// URL names the user admin, which nothing checks.
//
// # Reaching it
//
// dburl turns spanner://project/instance/database into the driver's form and
// drops the host, so usql reaches the emulator only when
// SPANNER_EMULATOR_HOST names it, as 127.0.0.1 and the published port. The DSN
// the tests use names the host itself, with usePlainText.

// spannerREST posts to the REST interface of the emulator. Making a thing
// that is there answers 409, and busybox wget has no way to accept one status
// and not another, so each step asks first whether the thing is there.
func spannerREST(path, name, body string) string {
	u := "http://127.0.0.1:9020/v1/projects/dbmeta/" + path
	return "wget -q -O /dev/null " + u + "/" + name + " || wget -q -O /dev/null --header 'Content-Type: application/json' --post-data '" +
		body + "' " + u
}

// spanner is the Cloud Spanner emulator, built here.
var spanner = product{
	name:  "spanner",
	image: "localhost/dbmeta/spanner",
	port:  9010,
	ready: []string{"sh", "-c", "wget -q -O /dev/null http://127.0.0.1:9020/v1/projects/dbmeta/instanceConfigs"},
	init: []string{"sh", "-c", "set -e\n" +
		spannerREST("instances", "dbmeta",
			`{"instanceId":"dbmeta","instance":{"config":"emulator-config","displayName":"dbmeta","nodeCount":1}}`) + "\n" +
		spannerREST("instances/dbmeta/databases", "dbmeta", "{\"createStatement\":\"CREATE DATABASE `dbmeta`\"}")},
	dsn: func(port int) string {
		return fmt.Sprintf("127.0.0.1:%d/projects/dbmeta/instances/dbmeta/databases/dbmeta;usePlainText=true", port)
	},
	url: func(int) string { return "spanner://admin@dbmeta/dbmeta/dbmeta" },
}

// Spanner is every Cloud Spanner emulator release dbrun starts.
//
// Staged, because dbmeta has no model that reads it, so CI runs none of
// them. Each keeps the cadence it would have if a model read it, which is
// what dbimp runs on each push and at night. See D119 and D120.
var Spanner = list{}.staged(spanner, Tested, "1.5.58")
