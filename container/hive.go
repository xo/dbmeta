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
	// 10001 is HiveServer2's HTTP port. 9083 is the metastore and 10002 is
	// the web interface, and nothing here uses either.
	port: 10001,
	env: map[string]string{
		// The image is one program per container and this picks which.
		// Without it the entrypoint does not know what to start.
		"SERVICE_NAME": "hiveserver2",
		"SERVICE_OPTS": "-Dhive.server2.transport.mode=http" +
			" -Dhive.server2.thrift.http.port=10001" +
			" -Dhive.server2.thrift.http.path=cliservice",
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
		"TERM=dumb beeline -u '" + beelineHTTP + "' -e 'SELECT 1'",
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
			" && TERM=dumb beeline -u '" + beelineHTTP + "' --silent=true -f /tmp/dbmeta-schema.sql",
	},
	// HiveServer2 starts a metastore, a compactor and a session pool
	// before it listens, and it took longer than the usual budget here.
	startup: 5 * time.Minute,
	// The driver of dbimp takes only the hive:// URL, so the DSN is that URL,
	// and the api is the http:// address that the tools of dbimp read. The
	// server checks neither the user nor the password, and the driver always
	// sends one, because Hive answers 401 to a request with none.
	dsn: func(port int) string {
		return fmt.Sprintf("hive://hive:%s@127.0.0.1:%d/default?httpPath=cliservice",
			url.QueryEscape(Password), port)
	},
	api: func(port int) string {
		return fmt.Sprintf("http://hive:%s@127.0.0.1:%d/cliservice", url.QueryEscape(Password), port)
	},
}

// beelineHTTP is the JDBC URL that beeline takes inside the container, for
// HiveServer2 in the HTTP transport.
const beelineHTTP = "jdbc:hive2://localhost:10001/;transportMode=http;httpPath=cliservice"

// Hive is every Apache Hive release dbmeta is tested against.
var Hive = list{}.add(hive, Verified, "4.2.1", "4.0.1")
