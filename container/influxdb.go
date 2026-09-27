package container

import (
	"fmt"
	"net/url"

	"github.com/xo/dbmeta"
)

// The InfluxDB releases dbmeta is tested against.
//
// dbmeta has no InfluxDB model. The releases are here so that dbrun can start
// a server for the tests of the InfluxDB driver in github.com/xo/dbimp. The
// driver has two dialects, which Ken accepted in dbimp's D78: influxdb, which
// is SQL on InfluxDB 3, and influxql, which is InfluxQL through /query on
// InfluxDB 1, 2 and 3. InfluxDB 3 answers both, so its entry names influxdb
// and also influxql, and one container serves both. See D114.
//
// # The range
//
// Every line is still rebuilt on docker.io/library/influxdb, so step 2 of
// docs/EVALUATION.md applies. Checked on 2026-09-28, 1.13.1, 1.11.8, 2.9.1 and
// 2.8.0 were rebuilt on 2026-09-19, and the InfluxDB 3 Core lines 3.11.5,
// 3.10.6 and 3.9.13 between 2026-09-16 and 2026-09-18. Ken chose the tiers in
// dbimp's D79: InfluxDB 1 and 2 are in maintenance, so the newest of each is
// Tested and the oldest Nightly, and InfluxDB 3 changes fast, so its floor and
// its ceiling are both Tested.
//
// The InfluxDB 3 tag is the release with -core, such as 3.11.5-core. InfluxDB 3
// Enterprise is not here: its free licence and its trial both need a person to
// follow a link in an email before the server starts.
//
// # InfluxDB 1
//
// The image makes the database dbmeta, the admin user and [InfluxDBUser], who
// may read that database, from its environment on the first start. Init sets
// the user's password and grants it READ again on every start, so a user that
// somebody changed is put back.
//
// # InfluxDB 2
//
// The image makes the first user, the organization dbmeta, the bucket dbmeta
// and the admin token [InfluxDBToken] from its environment on the first start.
// /query reads InfluxQL through a mapping of a database and a retention policy
// to a bucket. InfluxDB 2 maps a database named for each bucket to it by
// itself, as a virtual mapping, so /query reads dbmeta with no mapping made
// here, measured on 2.9.1 and 2.8.0. Init makes [InfluxDBUser] as a v1 user
// who may read the bucket, which is how /query takes a user and a password. A
// v2 token that may only read the bucket is possible too, and the server gives
// it a random value, so no DSN here could name it.
//
// # InfluxDB 3
//
// InfluxDB 3 Core has admin tokens and no users. A token with fewer rights is
// an Enterprise feature, so there is no ordinary user to make. The start
// command writes [InfluxDBToken] to a file, and the server takes it as the
// admin token. A restart keeps the token it already has. Init makes the
// database dbmeta with the HTTP API. The server answers 409 when the database
// exists, so Init is safe to run twice.
//
// The DSN of every release carries a user and a password. On InfluxDB 1 it is
// the admin user, and on InfluxDB 2 and 3 it is the name of the admin token
// and the token, which /query takes as the password.

// InfluxDBToken is the admin token of every InfluxDB 2 and 3 release.
// InfluxDB 3 requires a token to begin with apiv3_.
const InfluxDBToken = "apiv3_" + Password

// InfluxDBUser is the ordinary user of every InfluxDB 1 and 2 release. Its
// password is [Password].
const InfluxDBUser = "dbmeta_user"

// influxTokenName is the name of the admin token on InfluxDB 3, and the user
// name the DSN of InfluxDB 2 and 3 carries.
const influxTokenName = "_admin"

// influxAdmin is the admin user of InfluxDB 1 and 2.
const influxAdmin = "admin"

// influxDatabase is the database that the setup makes, and on InfluxDB 2 the
// organization and the bucket too.
const influxDatabase = "dbmeta"

// influxdb1 is the InfluxDB 1 image.
var influxdb1 = product{
	dialect: dbmeta.InfluxQL,
	name:    "influxdb",
	image:   "docker.io/library/influxdb",
	port:    8086,
	env: map[string]string{
		"INFLUXDB_DB":                 influxDatabase,
		"INFLUXDB_HTTP_AUTH_ENABLED":  "true",
		"INFLUXDB_ADMIN_USER":         influxAdmin,
		"INFLUXDB_ADMIN_PASSWORD":     Password,
		"INFLUXDB_READ_USER":          InfluxDBUser,
		"INFLUXDB_READ_USER_PASSWORD": Password,
		// The usage report that goes to the vendor.
		"INFLUXDB_REPORTING_DISABLED": "true",
	},
	ready: []string{"sh", "-c", influx1("SHOW DATABASES")},
	init: []string{"sh", "-c", "set -e\n" +
		influx1("SET PASSWORD FOR "+InfluxDBUser+" = '"+Password+"'") + "\n" +
		influx1("GRANT READ ON "+influxDatabase+" TO "+InfluxDBUser) + "\n"},
	dsn:   influxHTTP(influxAdmin, Password),
	users: []Principal{{Role: User, User: InfluxDBUser, dsn: influxHTTP(InfluxDBUser, Password)}},
}

// influx1 runs one statement in the influx shell of InfluxDB 1 as the admin.
func influx1(stmt string) string {
	return `influx -username ` + influxAdmin + ` -password '` + Password + `' -execute "` + stmt + `"`
}

// influx2Setup makes the v1 user if it is missing, and sets its password.
const influx2Setup = `set -e
t="--token $DOCKER_INFLUXDB_INIT_ADMIN_TOKEN"
b=$(influx bucket list --name ` + influxDatabase + ` --hide-headers $t | cut -f1)
[ -n "$b" ] || { echo "the bucket ` + influxDatabase + ` is missing"; exit 1; }
if influx v1 auth list --username ` + InfluxDBUser + ` --hide-headers $t | grep -q ` + InfluxDBUser + `; then
	influx v1 auth set-password --username ` + InfluxDBUser + ` --password "$DOCKER_INFLUXDB_INIT_PASSWORD" $t > /dev/null
else
	influx v1 auth create --username ` + InfluxDBUser + ` --password "$DOCKER_INFLUXDB_INIT_PASSWORD" \
		--read-bucket "$b" --org ` + influxDatabase + ` $t > /dev/null
fi`

// influxdb2 is the InfluxDB 2 image.
var influxdb2 = product{
	dialect: dbmeta.InfluxQL,
	name:    "influxdb",
	image:   "docker.io/library/influxdb",
	port:    8086,
	env: map[string]string{
		"DOCKER_INFLUXDB_INIT_MODE":        "setup",
		"DOCKER_INFLUXDB_INIT_USERNAME":    influxAdmin,
		"DOCKER_INFLUXDB_INIT_PASSWORD":    Password,
		"DOCKER_INFLUXDB_INIT_ORG":         influxDatabase,
		"DOCKER_INFLUXDB_INIT_BUCKET":      influxDatabase,
		"DOCKER_INFLUXDB_INIT_ADMIN_TOKEN": InfluxDBToken,
		// The usage report that goes to the vendor.
		"INFLUXD_REPORTING_DISABLED": "true",
	},
	// The first start runs the setup on another port and then starts the
	// server again, so the check asks for the bucket with the token.
	ready: []string{"sh", "-c", "influx bucket list --name " + influxDatabase +
		" --token \"$DOCKER_INFLUXDB_INIT_ADMIN_TOKEN\" > /dev/null"},
	init:  []string{"sh", "-c", influx2Setup},
	dsn:   influxHTTP(influxTokenName, InfluxDBToken),
	users: []Principal{{Role: User, User: InfluxDBUser, dsn: influxHTTP(InfluxDBUser, Password)}},
}

// influxServe writes the token file and starts the server.
//
// The flags have the same names on every release here, and the environment
// variables do not, so flags set everything. The two memory pools default to
// a fifth of the memory each, and they are fixed so that they fit the 4 GB
// limit whatever the server reads. 3.11 logs that the two memory flags are
// deprecated and still applies them, and their new names are not on 3.9. The
// telemetry upload goes to the vendor, and it is off.
const influxServe = `printf '{"token":"%s","name":"` + influxTokenName + `"}' "$INFLUXDB_TOKEN" > /home/influxdb3/admin.json &&
chmod 600 /home/influxdb3/admin.json &&
exec influxdb3 serve --node-id dbmeta --object-store file --data-dir /home/influxdb3/.influxdb3 \
	--admin-token-file /home/influxdb3/admin.json --disable-telemetry-upload \
	--exec-mem-pool-bytes 536870912 --parquet-mem-cache-size 268435456`

// influxdb is the InfluxDB 3 Core image.
var influxdb = product{
	dialect:   dbmeta.InfluxDB,
	also:      []dbmeta.Dialect{dbmeta.InfluxQL},
	name:      "influxdb",
	image:     "docker.io/library/influxdb",
	tagSuffix: "-core",
	port:      8181,
	env:       map[string]string{"INFLUXDB_TOKEN": InfluxDBToken},
	args:      []string{"sh", "-c", influxServe},
	ready: []string{"curl", "-sf", "-H", "Authorization: Bearer " + InfluxDBToken,
		"http://127.0.0.1:8181/health"},
	init: []string{"sh", "-c", `code=$(curl -s -o /dev/null -w '%{http_code}' -X POST \
	-H "Authorization: Bearer $INFLUXDB_TOKEN" -H 'Content-Type: application/json' \
	-d '{"db":"` + influxDatabase + `"}' http://127.0.0.1:8181/api/v3/configure/database)
[ "$code" = 200 ] || [ "$code" = 409 ] || { echo "creating the database answered $code"; exit 1; }`},
	dsn: influxHTTP(influxTokenName, InfluxDBToken),
}

// influxHTTP is the address of the HTTP API, with one user and password.
func influxHTTP(user, password string) func(port int) string {
	return func(port int) string {
		u := url.URL{
			Scheme: "http",
			User:   url.UserPassword(user, password),
			Host:   fmt.Sprintf("127.0.0.1:%d", port),
		}
		return u.String()
	}
}

// InfluxDB is every InfluxDB release dbmeta is tested against.
var InfluxDB = list{}.add(influxdb1, Tested, "1.13.1").
	add(influxdb1, Nightly, "1.11.8").
	add(influxdb2, Tested, "2.9.1").
	add(influxdb2, Nightly, "2.8.0").
	add(influxdb, Tested, "3.9.13", "3.11.5").
	add(influxdb, Nightly, "3.10.6")
