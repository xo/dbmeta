package container

import (
	"fmt"
	"net/url"
	"time"
)

// The Apache Pinot releases dbrun starts.
//
// dbmeta has no Pinot model. The releases are here so that dbrun can start a
// server for the tests of the Pinot driver in github.com/xo/dbimp, which reads
// the query API of the broker. No dialect is named yet, because dbimp settles
// the name with the driver. See D112.
//
// # The range
//
// docker.io/apachepinot/pinot builds each release tag once and never again,
// and Apache supports only the newest release, so the rule in D112 applies
// instead of step 2 of docs/EVALUATION.md: the newest release of each of the
// last two lines. Checked on 2026-09-27, that is 1.5.1, released on
// 2026-07-01, and 1.4.0, on 2025-09-30. The tag latest is a snapshot of the
// next release. Pinot is under the Apache 2.0 licence.
//
// # One container
//
// Pinot is a controller, a broker, a server and ZooKeeper. The Quickstart runs
// all of them in one process, and its broker answers on port 8000. It keeps
// its data in a new folder on every start and loads its tables again, so a
// stop and a start give the same tables. It loads one sample table here,
// baseballStats, rather than the ten of the batch Quickstart.
//
// # The users
//
// The broker checks a user and a password, and the controller does not,
// because the Quickstart loads its tables through the controller with none.
// The users are in the configuration of the broker, and there is no statement
// that makes one. pinotAdmin may query every table. [PinotUser] may query
// baseballStats and no other. A query through the broker cannot write, so
// that is the difference between the two. The Quickstart loads one table, so
// the limit on the ordinary user was not measured: both users read it, and a
// wrong password or none is refused with 401, on 2026-09-28.
//
// The configuration has to exist before Java starts, and the image's
// entrypoint is the Pinot command. So the entry starts sh instead, which
// writes the file and then starts the Quickstart.

// PinotUser is the ordinary user of every Pinot release. Its password is
// [Password].
const PinotUser = "dbmeta_user"

// pinotAdmin is the user who may query every table.
const pinotAdmin = "admin"

// pinotTable is the sample table the Quickstart loads.
const pinotTable = "baseballStats"

// pinotServe writes the configuration of the broker and starts the
// Quickstart.
const pinotServe = `printf '%s\n' \
	'pinot.broker.access.control.class=org.apache.pinot.broker.broker.BasicAuthAccessControlFactory' \
	'pinot.broker.access.control.principals=` + pinotAdmin + `,` + PinotUser + `' \
	'pinot.broker.access.control.principals.` + pinotAdmin + `.password=` + Password + `' \
	'pinot.broker.access.control.principals.` + PinotUser + `.password=` + Password + `' \
	'pinot.broker.access.control.principals.` + PinotUser + `.tables=` + pinotTable + `' \
	> /tmp/dbmeta.conf &&
exec bin/pinot-admin.sh QuickStart -type BATCH -configFile /tmp/dbmeta.conf \
	-bootstrapTableDir /opt/pinot/examples/batch/` + pinotTable

// pinot is the Apache Pinot image.
var pinot = product{
	name:  "pinot",
	image: "docker.io/apachepinot/pinot",
	port:  8000,
	// The image asks for a 4 GB heap, which is the whole limit of the
	// container.
	env:      map[string]string{"JAVA_OPTS": "-Xms1G -Xmx2G -Dpinot.admin.system.exit=false"},
	runFlags: []string{"--entrypoint", "sh"},
	args:     []string{"-c", pinotServe},
	ready: []string{"sh", "-c", `curl -sf -u '` + PinotUser + `:` + Password +
		`' -H 'Content-Type: application/json' -d '{"sql":"SELECT COUNT(*) FROM ` + pinotTable +
		`"}' http://127.0.0.1:8000/query/sql | grep -q '"exceptions":\[\]'`},
	// It answered in 35 seconds on 1.5.1 and in 89 on 1.4.0, the second with
	// the pull of the image, on a development machine. A GitHub runner is
	// slower, and one Java process starts four services.
	startup: 5 * time.Minute,
	dsn:     pinotHTTP(pinotAdmin),
	users:   []Principal{{Role: User, User: PinotUser, dsn: pinotHTTP(PinotUser)}},
}

// pinotHTTP is the address of the broker, with one user's credentials.
func pinotHTTP(user string) func(port int) string {
	return func(port int) string {
		u := url.URL{
			Scheme: "http",
			User:   url.UserPassword(user, Password),
			Host:   fmt.Sprintf("127.0.0.1:%d", port),
		}
		return u.String()
	}
}

// Pinot is every Apache Pinot release dbrun starts.
//
// Staged, because dbmeta has no model that reads it, so CI runs none of
// them. See D119.
var Pinot = list{}.add(pinot, Staged, "1.4.0", "1.5.1")
