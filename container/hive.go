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
	client:  beelineClient(),
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
	// later image will not have.
	//
	// The script is run from a copy with IF NOT EXISTS added to each CREATE
	// TABLE that lacks it, because it runs on every start and dbrun runs it
	// again after a failure. Every other statement in it is CREATE ... IF
	// NOT EXISTS or CREATE OR REPLACE. In 4.2.0 the one that is not is
	// CREATE EXTERNAL TABLE `HIVE_LOCKS`, and beeline stops at the first
	// error, so a second run failed there and never reached the rest. That
	// broke every start after the first, and a retry in CI. See D107.
	init: []string{
		"sh", "-c",
		"f=$(ls /opt/hive/scripts/metastore/upgrade/hive/hive-schema-*.hive.sql" +
			" | grep -v alpha | grep -v beta | sort -V | tail -1)" +
			" && sed -E 's/^CREATE (EXTERNAL )?TABLE `/CREATE \\1TABLE IF NOT EXISTS `/' \"$f\" > /tmp/dbmeta-schema.sql" +
			" && TERM=dumb beeline -u jdbc:hive2://localhost:10000/ --silent=true -f /tmp/dbmeta-schema.sql",
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
var Hive = list{}.add(hive, Verified, "4.2.1").
	add(hive, Verified, "4.0.1")

// hiveHTTP is Apache Hive with HiveServer2 in the HTTP transport, which is the
// only way dbimp's driver reaches it (dbimp D232, and D282 here). It is the same image
// as hive, and the container starts HiveServer2 with
// hive.server2.transport.mode set to http, so it does not answer on 10000.
//
// The port and the path are the defaults of HiveServer2: 10001 and cliservice.
// The properties are written out anyway, so that an image with other defaults
// still answers where the DSN says.
//
// # No users
//
// The image runs with hive.server2.authentication set to NONE, so the server
// accepts any user name and any password, and the PLAIN exchange that the HTTP
// mode of a client sends is not checked. A user cannot be made and no
// Init is needed. The sys database that hive creates is not made, because
// dbimp reads tables and not the metastore.
var hiveHTTP = product{
	dialect: dbmeta.Hive,
	name:    "hivehttp",
	client:  beelineClient(),
	image:   "docker.io/apache/hive",
	port:    10001,
	env: map[string]string{
		"SERVICE_NAME": "hiveserver2",
		"SERVICE_OPTS": "-Dhive.server2.transport.mode=http" +
			" -Dhive.server2.thrift.http.port=10001" +
			" -Dhive.server2.thrift.http.path=cliservice",
	},
	ready: []string{
		"sh", "-c",
		"TERM=dumb beeline -u 'jdbc:hive2://localhost:10001/;transportMode=http;httpPath=cliservice' -e 'SELECT 1'",
	},
	startup: 5 * time.Minute,
	dsn: func(port int) string {
		return fmt.Sprintf("http://hive:%s@127.0.0.1:%d/cliservice", url.QueryEscape(Password), port)
	},
	url: func(port int) string {
		return fmt.Sprintf("hive://hive:%s@127.0.0.1:%d/default?auth=PLAIN&transport=http&httpPath=cliservice",
			url.QueryEscape(Password), port)
	},
}

// HiveHTTP is the Hive releases that dbimp reaches over HTTP. They are Verified,
// so CI never starts them. models/hive reads the binary entries above.
var HiveHTTP = list{}.add(hiveHTTP, Verified, "4.2.1", "4.0.1")
