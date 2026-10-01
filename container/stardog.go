package container

// The Stardog releases dbrun starts.
//
// dbmeta has no Stardog model. The releases are here so that dbrun can start
// a server for the tests of the SPARQL driver in github.com/xo/dbimp, which
// sends queries to /{db}/query. No dialect is named yet, because dbimp settles
// the name with the driver. See D118.
//
// # The range
//
// docker.io/stardog/stardog builds each release tag once, so the rule in D112
// applies: the newest release of each of the last two lines. Checked on
// 2026-09-28, that is 12.1.4, of 2026-09-03, and 12.0.4, of 2026-06-15.
//
// # The license
//
// Stardog does not start without a license file. Stardog Free is a license of
// one year that a person gets by signing up with an email address, and Ken
// chose on 2026-09-28 to provision it. dbrun mounts the file at [Server.License]
// and lists these releases only while it finds the file. CI has no license,
// and no model reads them, so they are Staged.
//
// # Not yet measured
//
// No release has started here, because no license file is provisioned yet.
// The commands below follow the vendor's documentation and are the first
// thing to measure when the file arrives.
//
// # The users
//
// Stardog makes the superuser admin with the password admin. Init gives admin
// [Password], makes the database dbmeta and [StardogUser], and lets that user
// read dbmeta.

// StardogUser can read the database dbmeta. Its password is [Password].
const StardogUser = "dbmeta_user"

// stardogInit gives admin its password, and makes the database and the user
// when they are missing.
var stardogInit = `set -e
if ! curl -sf -o /dev/null -u 'admin:` + Password + `' http://127.0.0.1:5820/admin/databases; then
	stardog-admin user passwd -u admin -p admin -N '` + Password + `' admin
fi
a="-u admin -p ` + Password + `"
stardog-admin db list $a | grep -qw dbmeta || stardog-admin db create $a -n dbmeta
stardog-admin user list $a | grep -qw ` + StardogUser + ` || stardog-admin user add $a -N '` + Password + `' ` + StardogUser + `
stardog-admin user grant $a -a read -o db:dbmeta ` + StardogUser

// stardog is the Stardog image.
var stardog = product{
	name:    "stardog",
	image:   "docker.io/stardog/stardog",
	port:    5820,
	license: "/var/opt/stardog/stardog-license-key.bin",
	env: map[string]string{
		"STARDOG_SERVER_JAVA_ARGS": "-Xms1g -Xmx1g -XX:MaxDirectMemorySize=1g",
	},
	// The check runs before Init gives admin its password, so it takes
	// either one.
	ready: []string{"sh", "-c", "curl -sf -o /dev/null -u 'admin:" + Password + "' http://127.0.0.1:5820/admin/databases ||" +
		" curl -sf -o /dev/null -u admin:admin http://127.0.0.1:5820/admin/databases"},
	init:  []string{"bash", "-c", stardogInit},
	dsn:   keyHTTP("admin", Password),
	users: []Principal{{Role: User, User: StardogUser, dsn: keyHTTP(StardogUser, Password)}},
}

// Stardog is every Stardog release dbrun starts.
//
// Staged, because dbmeta has no model that reads Stardog, so CI runs none
// of them. Each also needs a license file that CI does not have,
// so its cadence is Verified. See D119 and D120.
var Stardog = list{}.staged(stardog, Verified, "12.0.4", "12.1.4")
