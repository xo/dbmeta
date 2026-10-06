package container

// The Apache Drill releases dbrun starts.
//
// dbmeta has no Drill model. The releases are here so that dbrun can start a
// server for the tests of the Drill driver in github.com/xo/dbimp, which sends
// SQL to /query.json on the HTTP interface. No dialect is named yet, because
// dbimp settles the name with the driver. See D118.
//
// # The range
//
// docker.io/apache/drill builds each release tag once, so the rule in D112
// applies: the newest release of each of the last two lines. Checked on
// 2026-09-28, that is 1.22.0, of 2025-06-28, and 1.21.2, of 2024-08-02. Drill
// releases slowly. It is under the Apache 2.0 license.
//
// # Embedded mode
//
// The image runs Drill in embedded mode, which is sqlline on a terminal, and
// it ends when its input does. So the container gets a terminal and an open
// input.
//
// # The answers
//
// Drill answers a request with no user with 404, and a wrong password with a
// redirect to its login form, 307, and not with 401.
//
// # The users
//
// Authentication is off in the image. The command copies the configuration,
// which the image does not let its user write, turns on the htpasswd
// authenticator with a file that holds admin and [DrillUser], each with
// [Password], and asks for basic authentication on the HTTP interface. admin
// is the one administrator, and [DrillUser] can query and cannot change an
// option or a storage plugin.

// DrillUser can query, and cannot change an option or a storage plugin. Its
// password is [Password].
const DrillUser = "dbmeta_user"

// drillServe writes the configuration and starts Drill.
var drillServe = `set -e
c=/tmp/drill-conf
if [ ! -d $c ]; then
	cp -r $DRILL_HOME/conf $c
	printf 'admin:%s\n` + DrillUser + `:%s\n' '` + Password + `' '` + Password + `' > $c/htpasswd
	cat >> $c/drill-override.conf <<'CONF'
drill.exec.security.user.auth.enabled: true
drill.exec.security.user.auth.packages += "org.apache.drill.exec.rpc.user.security"
drill.exec.security.user.auth.impl: "htpasswd"
drill.exec.security.user.auth.htpasswd.path: "/tmp/drill-conf/htpasswd"
drill.exec.http.auth.mechanisms: ["BASIC"]
drill.exec.options.security.admin.users: "admin"
CONF
fi
export DRILL_CONF_DIR=$c DRILL_HEAP=1G DRILL_MAX_DIRECT_MEMORY=1G
exec $DRILL_HOME/bin/drill-embedded -n admin -p '` + Password + `'`

// drill is the Apache Drill image.
var drill = product{
	name:  "drill",
	image: "docker.io/apache/drill",
	port:  8047,
	env: map[string]string{
		// The Java of 1.22.0 is 17.0.2, which fails with a
		// NullPointerException in CgroupV2Subsystem when it reads the
		// cgroups of a current host. Its container support is what reads
		// them, and the heap is set here, so it is off.
		"JAVA_TOOL_OPTIONS": "-XX:-UseContainerSupport",
	},
	runFlags: []string{"--interactive", "--tty", "--entrypoint", "/bin/bash"},
	args:     []string{"-c", drillServe},
	ready: bashRequest(8047, "POST", "/query.json", `{"queryType":"SQL","query":"SELECT 1"}`,
		map[string]string{"Authorization": adminBasic}, 200),
	dsn:   keyURL("drill", "admin"),
	api:   keyHTTP("admin", Password),
	users: []Principal{{Role: User, User: DrillUser, dsn: keyURL("drill", DrillUser), api: keyHTTP(DrillUser, Password)}},
}

// Drill is every Apache Drill release dbrun starts.
//
// Staged, because dbmeta has no model that reads it, so CI runs none of
// them. Each keeps the cadence it will have if a model reads it, which is
// what dbimp runs on each push and at night. See D119 and D120.
var Drill = list{}.staged(drill, Tested, "1.21.2", "1.22.0")
