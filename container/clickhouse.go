package container

import (
	"fmt"
	"net/url"

	"github.com/xo/dbmeta"
)

// The ClickHouse releases dbmeta is tested against.
//
// # The floor, by the docs/EVALUATION.md procedure
//
// Step 2 decides it: the oldest release whose image is still rebuilt. On
// clickhouse/clickhouse-server, 23.8 was last rebuilt in August 2024 and
// 24.3 and 24.8 in February 2025, so all three are dead. 25.3 was rebuilt in
// February 2026, 25.8 in August, and 26.8 and 26.9 in September. The floor is
// 25.3.
//
// ClickHouse releases monthly and marks some releases long term, so a floor
// here goes stale faster than anywhere else in this package. That is what the
// tiers are for, and D21's removal trigger applies the same way.
//
// # Why this pair runs on every push
//
// 25.8 and 26.9 are the ends, and they span the one fragment the model has.
// system.constraints does not exist on 25.3, 25.8 or 26.1, and does exist on
// 26.8 and 26.9, so a pair either side of it exercises both.
// clickhouse is the official server image.
var clickhouse = product{
	dialect: dbmeta.ClickHouse,
	name:    "clickhouse",
	image:   "docker.io/clickhouse/clickhouse-server",
	// The native protocol, which nothing here uses since the tests moved to
	// dbimp's driver (D174). The HTTP interface, 8123, is published on the
	// second host port (D124), and dbimp's driver reads it.
	port:   9000,
	second: 8123,
	env: map[string]string{
		"CLICKHOUSE_PASSWORD": Password,
		// Without this the default user cannot CREATE ROLE, CREATE USER or
		// GRANT, so the fixture cannot build the objects the roles, role
		// grants and privileges queries read.
		"CLICKHOUSE_DEFAULT_ACCESS_MANAGEMENT": "1",
	},
	// The image warns about the open file limit and starts anyway, which was
	// checked rather than assumed, so there is no ulimit to set here.
	//
	// Named collections stay out of reach. They are gated by
	// access_control_improvements.named_collection_control in the server
	// configuration, which the image exposes no variable for, so the fixture
	// builds none and the foreign servers query returns no rows. Enabling it
	// means building an image, which is not worth it for one query that is
	// already verified to run.
	ready: []string{"clickhouse-client", "--password", Password, "-q", "SELECT 1"},
	// The ordinary user reads and writes the database dbmeta, and can see its
	// own queries. The user and the grants are set every time, so a start
	// that finds them is safe (D105, D107).
	init: []string{"clickhouse-client", "--password", Password, "--multiquery", "-q",
		"CREATE DATABASE IF NOT EXISTS " + clickHouseDatabase + ";" +
			" CREATE USER IF NOT EXISTS " + ClickHouseUser + " IDENTIFIED BY '" + Password + "';" +
			" ALTER USER " + ClickHouseUser + " IDENTIFIED BY '" + Password + "';" +
			" GRANT SELECT, INSERT, ALTER, CREATE, DROP, TRUNCATE, OPTIMIZE ON " + clickHouseDatabase + ".* TO " + ClickHouseUser + ";" +
			" GRANT SELECT ON system.processes TO " + ClickHouseUser},
	// dbimp's driver takes clickhouse://user:password@host:port/database over
	// the HTTP interface, which is on the second host port, so the DSN and
	// the URL are the same string (dbimp D177, D174).
	dsn: clickHouseDSN("default"),
	api: clickHouseHTTP("default"),
	users: []Principal{{
		Role: User, User: ClickHouseUser,
		dsn: clickHouseDSN(ClickHouseUser),
		api: clickHouseHTTP(ClickHouseUser),
	}},
}

// ClickHouseUser is the ordinary user of every ClickHouse release. Its
// password is [Password].
const ClickHouseUser = "dbmeta_user"

// clickHouseDatabase is the database that the ordinary user can write.
const clickHouseDatabase = "dbmeta"

// clickHouseDSN is the address of the HTTP interface as one user, in the form
// of dbimp's driver. The interface is on the second host port.
func clickHouseDSN(user string) func(port int) string {
	return func(port int) string {
		return fmt.Sprintf("clickhouse://%s:%s@127.0.0.1:%d/default",
			user, url.QueryEscape(Password), SecondHostPort(port))
	}
}

// clickHouseHTTP is the address of the HTTP interface as one user, on the
// second host port.
func clickHouseHTTP(user string) func(port int) string {
	return func(port int) string {
		u := url.URL{
			Scheme: "http",
			User:   url.UserPassword(user, Password),
			Host:   fmt.Sprintf("127.0.0.1:%d", SecondHostPort(port)),
		}
		return u.String()
	}
}

// ClickHouse is every ClickHouse release dbmeta is tested against.
var ClickHouse = list{}.add(clickhouse, Tested, "25.8", "26.9").
	add(clickhouse, Nightly, "25.3", "26.8")
