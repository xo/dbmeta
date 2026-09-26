package container

import (
	"fmt"
	"net/url"
	"time"

	"github.com/xo/dbmeta"
)

// The Apache Hive releases dbmeta is tested against.
//
// # The floor, by the docs/EVALUATION.md procedure
//
// Step 2 decides it. apache/hive carries 15 tags. 4.2.1 was rebuilt five
// weeks before this was written and 4.2.0 ten months before, and there is a
// nightly that is not a release. Nothing older than 4.0 is published at all.
//
// So the range is the 4.x tags and the floor is 4.0.
//
// Hive matters here for a reason D66 records: it is the shape Impala already
// taught. D67 struck Impala because it has no queryable catalog and answers
// only through SHOW and DESCRIBE, which are statements rather than relations.
// Hive 3.0 added an information_schema and a sys database, both backed by the
// metastore, so the question is whether those answer rather than whether a
// catalog exists at all.
var hive = product{
	dialect: dbmeta.Hive,
	name:    "hive",
	image:   "docker.io/apache/hive",
	// 10000 is HiveServer2's Thrift port. 9083 is the metastore and 10002
	// is the web interface, and nothing here uses either.
	port: 10000,
	env: map[string]string{
		// The image is one program per container and this picks which.
		// Without it the entrypoint does not know what to start.
		"SERVICE_NAME": "hiveserver2",
	},
	// TERM is set because beeline builds a jline terminal before it
	// connects, and there is no TTY inside a podman exec:
	//
	//	java.lang.IllegalStateException: Unable to create a terminal
	//
	// That is beeline failing rather than the server, so a readiness
	// check without it reports a server that is up as one that never
	// answered.
	ready: []string{
		"sh", "-c",
		"TERM=dumb beeline -u jdbc:hive2://localhost:10000/ -e 'SELECT 1'",
	},
	// The metastore is relational and it is not readable through SQL
	// until the sys database is created over it. Hive ships the script
	// that does that and it needs a running HiveServer2 to run against,
	// so it is a step after the server answers.
	//
	// The script name carries the release, so this takes the newest one
	// that is not an alpha or a beta rather than naming a file that a
	// later image will not have. Every statement in it is CREATE ... IF
	// NOT EXISTS or CREATE OR REPLACE, so running it on every start is
	// safe and the second run is quick.
	init: []string{
		"sh", "-c",
		"TERM=dumb beeline -u jdbc:hive2://localhost:10000/ --silent=true -f " +
			"$(ls /opt/hive/scripts/metastore/upgrade/hive/hive-schema-*.hive.sql" +
			" | grep -v alpha | grep -v beta | sort -V | tail -1)",
	},
	// HiveServer2 starts a metastore, a compactor and a session pool
	// before it listens, and it took longer than the usual budget here.
	startup: 5 * time.Minute,
	// The DSN keeps its scheme, which is unusual here and is what the
	// driver requires: beltran/gohive/v2 rejects anything that does not
	// begin with hive://. dburl truncates the scheme for this driver and
	// that is recorded in D78 as a requirement on dburl rather than
	// worked around here.
	//
	// auth=NONE is the mode the image runs in and it has to be stated.
	// The driver panics rather than returning an error when the mode is
	// missing or unknown:
	//
	//	panic: Unrecognized auth
	//
	// A password is still sent because the SASL exchange happens even
	// when nothing is validated.
	dsn: func(port int) string {
		return fmt.Sprintf("hive://hive:%s@127.0.0.1:%d/default?auth=NONE",
			url.QueryEscape(Password), port)
	},
}

// Hive is every Apache Hive release dbmeta is tested against.
var Hive = list{}.add(hive, Tested, "4.2.1").
	add(hive, Nightly, "4.0.1")
