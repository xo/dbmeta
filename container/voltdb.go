package container

import (
	"fmt"
	"net/url"
)

// The Volt Active Data releases dbrun starts.
//
// usql reaches VoltDB with the scheme voltdb and
// github.com/VoltDB/voltdb-client-go. dbmeta has no VoltDB model. The
// releases are here so that dbrun can start a server for usql and for a model
// that comes later. See D118.
//
// # The range
//
// The community edition is discontinued, and the only image still published
// is the vendor's docker.io/voltactivedata/volt-developer-edition. It builds
// each release tag once, so the rule in D112 applies: the newest release of
// each of the last two lines. Checked on 2026-09-28, that is 15.2.0, of
// 2026-04-16, and 14.1.0, of 2025-01-31. The tag ends in _voltdb.
//
// # The license
//
// The developer edition does not start without a license file, which a person
// gets by signing up, and which lasts 100 days. Ken chose on 2026-09-28 to
// provision it. dbrun mounts the file at [Server.License] and lists these
// releases only while it finds the file. CI has no license, and no model
// reads them, so they are Staged.
//
// # Not yet measured
//
// No release has started here, because no license file is provisioned yet.
// The steps below follow the vendor's compose file and documentation and are
// the first thing to measure when the file arrives.
//
// # The users
//
// The command writes a deployment file with security on, which names admin,
// an administrator, and [VoltDBUser], with the role dbmeta_reader, both with
// [Password]. It makes the database directory once and starts the server on
// it. Init makes the role, which can read. VoltDB has one database, so
// nothing is named dbmeta.

// VoltDBUser can read. Its password is [Password].
const VoltDBUser = "dbmeta_user"

// voltdbServe writes the deployment file, makes the database directory once,
// and starts the server.
var voltdbServe = `set -e
d=/tmp/voltdb
cat > /tmp/deployment.xml <<'XML'
<?xml version="1.0"?>
<deployment>
  <cluster kfactor="0"/>
  <security enabled="true" provider="hash"/>
  <users>
    <user name="admin" password="` + Password + `" roles="administrator"/>
    <user name="` + VoltDBUser + `" password="` + Password + `" roles="dbmeta_reader"/>
  </users>
</deployment>
XML
[ -d $d/voltdbroot ] || voltdb init --dir=$d --config=/tmp/deployment.xml --license=/etc/voltdb/license.xml
exec voltdb start --dir=$d --ignore=thp --count=1 --host=localhost`

// voltdb is the Volt Active Data developer image.
var voltdb = product{
	name:      "voltdb",
	image:     "docker.io/voltactivedata/volt-developer-edition",
	tagSuffix: "_voltdb",
	port:      21212,
	license:   "/etc/voltdb/license.xml",
	runFlags:  []string{"--entrypoint", "/bin/bash"},
	args:      []string{"-c", voltdbServe},
	ready:     []string{"sh", "-c", "sqlcmd --user=admin --password='" + Password + "' --query='exec @Ping;'"},
	init: []string{"sh", "-c", "sqlcmd --user=admin --password='" + Password +
		"' --query='CREATE ROLE dbmeta_reader WITH SQLREAD;' 2>&1 | grep -qi 'already exists\\|command succeeded'"},
	dsn:   voltURL("admin"),
	users: []Principal{{Role: User, User: VoltDBUser, dsn: voltURL(VoltDBUser)}},
}

// voltURL is the address of the server as one user.
func voltURL(user string) func(port int) string {
	return func(port int) string {
		return fmt.Sprintf("voltdb://%s:%s@127.0.0.1:%d", user, url.QueryEscape(Password), port)
	}
}

// VoltDB is every Volt Active Data release dbrun starts.
//
// Staged, because dbmeta has no model that reads VoltDB, so CI runs none
// of them. Each also needs a license file that CI does not have,
// so its cadence is Verified. See D119 and D120.
var VoltDB = list{}.staged(voltdb, Verified, "14.1.0", "15.2.0")
