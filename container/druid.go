package container

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"time"
)

// The Apache Druid releases dbmeta is tested against.
//
// Druid speaks Avatica, the wire protocol of Apache Calcite, on its Router at
// /druid/v2/sql/avatica-protobuf/. dbmeta has no Druid model. The releases are
// here so that dbrun can start a server for the tests of the Avatica driver in
// github.com/xo/dbimp. No dialect is named yet, because dbimp settles the name
// with the driver. See D113.
//
// # The range
//
// docker.io/apache/druid builds each release tag once and never again, so the
// rule in D112 applies: the newest release of each of the last two lines.
// Checked on 2026-09-28, that is 37.0.0, released on 2026-05-06, and 36.0.0, on
// 2026-02-06. 38.0.0-rc1 is a candidate and not a release. Druid is under the
// Apache 2.0 licence.
//
// # One container
//
// The image runs one Druid service in each container, and its scripts that
// run them all together need Python or Perl, which the image does not have. So
// the start command runs ZooKeeper and the five services itself, each in the
// background, with the nano quickstart configuration, which is the smallest.
//
// # The users
//
// The start command turns on the basic security extension. The administrator
// is admin with [Password], and Druid keeps its users in its metadata store.
// Init makes [DruidUser] through the security API of the Coordinator, with a
// role that may read every datasource and nothing else.

// DruidUser is the ordinary user that Init makes on every Druid release. Its
// password is [Password].
const DruidUser = "dbmeta_user"

// druidAuth is the authorization header of the administrator.
var druidAuth = "Authorization: Basic " + base64.StdEncoding.EncodeToString([]byte("admin:"+Password))

// druidConf is the nano quickstart configuration.
const druidConf = "conf/druid/single-server/nano-quickstart"

// druidServe turns on the security extension and starts every service.
//
// The properties are appended, and the last value of a property wins, except
// the list of extensions, which is edited so that it keeps the ones the
// quickstart loads.
var druidServe = `set -e
cd /opt/druid
c=` + druidConf + `/_common/common.runtime.properties
if ! grep -q druid-basic-security $c; then
sed -i 's/^druid.extensions.loadList=\[/druid.extensions.loadList=["druid-basic-security", /' $c
cat >> $c <<'PROPS'
druid.auth.authenticatorChain=["basic"]
druid.auth.authenticator.basic.type=basic
druid.auth.authenticator.basic.initialAdminPassword=` + Password + `
druid.auth.authenticator.basic.initialInternalClientPassword=` + Password + `
druid.auth.authenticator.basic.credentialsValidator.type=metadata
druid.auth.authenticator.basic.skipOnFailure=false
druid.auth.authenticator.basic.authorizerName=basic
druid.escalator.type=basic
druid.escalator.internalClientUsername=druid_system
druid.escalator.internalClientPassword=` + Password + `
druid.escalator.authorizerName=basic
druid.auth.authorizers=["basic"]
druid.auth.authorizer.basic.type=basic
PROPS
fi
bin/run-zk conf &
sleep 5
for s in coordinator-overlord broker router historical middleManager; do
	bin/run-druid $s ` + druidConf + ` &
done
wait`

// druidPost posts to the Coordinator as the administrator. An empty body is
// sent as {}.
func druidPost(path, body string) string {
	if body == "" {
		body = "{}"
	}
	return `wget -q -O /dev/null --header '` + druidAuth + `' --header 'Content-Type: application/json' --post-data '` +
		body + `' http://127.0.0.1:8081/druid-ext/basic-security/` + path
}

// druid is the Apache Druid image.
var druid = product{
	name:     "druid",
	image:    "docker.io/apache/druid",
	port:     8888,
	runFlags: []string{"--entrypoint", "/bin/bash"},
	args:     []string{"-c", druidServe},
	// The check runs a query through the Router as the administrator, so it
	// passes only when the Broker answers too.
	ready: []string{"sh", "-c", `wget -q -O /dev/null --header '` + druidAuth +
		`' --header 'Content-Type: application/json' --post-data '{"query":"SELECT 1"}' http://127.0.0.1:8888/druid/v2/sql`},
	// Making the user again answers an error, so those two steps may fail.
	// The password and the role are set every time.
	init: []string{"sh", "-c", "set -e\n" +
		druidPost("authentication/db/basic/users/"+DruidUser, "") + " || true\n" +
		druidPost("authentication/db/basic/users/"+DruidUser+"/credentials", `{"password":"`+Password+`"}`) + "\n" +
		druidPost("authorization/db/basic/users/"+DruidUser, "") + " || true\n" +
		druidPost("authorization/db/basic/roles/dbmeta_role", "") + " || true\n" +
		druidPost("authorization/db/basic/roles/dbmeta_role/permissions",
			`[{"resource":{"name":".*","type":"DATASOURCE"},"action":"READ"}]`) + "\n" +
		druidPost("authorization/db/basic/users/"+DruidUser+"/roles/dbmeta_role", "") + " || true\n"},
	startup: 5 * time.Minute,
	dsn:     druidHTTP("admin"),
	users:   []Principal{{Role: User, User: DruidUser, dsn: druidHTTP(DruidUser)}},
}

// druidHTTP is the address of the Router, with one user's credentials.
func druidHTTP(user string) func(port int) string {
	return func(port int) string {
		u := url.URL{
			Scheme: "http",
			User:   url.UserPassword(user, Password),
			Host:   fmt.Sprintf("127.0.0.1:%d", port),
		}
		return u.String()
	}
}

// Druid is every Apache Druid release dbmeta is tested against.
//
// Both on every push, because they are the newest of the last two lines.
var Druid = list{}.add(druid, Tested, "36.0.0", "37.0.0")
