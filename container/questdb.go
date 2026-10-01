package container

import (
	"fmt"
	"net/url"

	"github.com/xo/dbmeta"
)

// The QuestDB releases dbmeta is tested against.
//
// QuestDB speaks PostgreSQL's protocol on 8812, and pgx reaches it, which is
// what dburl's questdb scheme opens from v0.36.0 and what usql uses. models/
// questdb reads it that way. dbimp plans a driver of its own for the HTTP
// interface on 9000, which sends SQL to /exec, and the entry publishes that
// interface too. See D118 and D124.
//
// # The range
//
// docker.io/questdb/questdb builds each release tag once, so the rule in D112
// applies: the newest release of each of the last two lines. Checked on
// 2026-09-28, that is 10.0.1, of 2026-08-24, and 9.4.3, of 2026-06-15. The
// tags that end in -rhel and the patch and nightly tags are left out. The open
// source edition is under the Apache 2.0 license.
//
// # Two users
//
// The open source edition has one user with every right, admin with
// [Password], on the HTTP interface and on the PostgreSQL one. The PostgreSQL
// interface can have one more user, which can only read, and the entry turns
// it on as [QuestDBUser] with [Password]. The edition has no roles and no
// grants. QuestDB has no databases, so nothing is named dbmeta, and the
// PostgreSQL interface takes the name qdb.
//
// # The version
//
// SHOW server_version and version() both answer PostgreSQL 12.3, on the
// PostgreSQL interface. Only SELECT build() names the release, as "Build
// Information: QuestDB 10.0.1, JDK 25.0.2, Commit Hash ...". The usql session
// measured it on 10.0.1 on 2026-09-29, and the model's version query is that.
//
// # Two ports
//
// The first port is the HTTP interface, which the check uses and dbimp's
// planned driver will read. The DSN and the URL are the PostgreSQL interface,
// which is published on the second host port (D124). The DSN is the form pgx
// takes, and the URL is dburl's questdb one.

// QuestDBUser is the user of the PostgreSQL interface that can only read.
// Its password is [Password].
const QuestDBUser = "dbmeta_user"

// questdb is the QuestDB image.
var questdb = product{
	dialect: dbmeta.QuestDB,
	name:    "questdb",
	image:   "docker.io/questdb/questdb",
	port:    9000,
	second:  8812,
	env: map[string]string{
		"QDB_HTTP_USER":                "admin",
		"QDB_HTTP_PASSWORD":            Password,
		"QDB_PG_USER":                  "admin",
		"QDB_PG_PASSWORD":              Password,
		"QDB_PG_READONLY_USER_ENABLED": "true",
		"QDB_PG_READONLY_USER":         QuestDBUser,
		"QDB_PG_READONLY_PASSWORD":     Password,
		// The usage report that goes to the vendor.
		"QDB_TELEMETRY_ENABLED": "false",
	},
	ready: []string{"sh", "-c", "curl -sf -o /dev/null -u 'admin:" + Password +
		"' -G --data-urlencode 'query=SELECT 1' http://127.0.0.1:9000/exec"},
	dsn:   questPG("postgres", "admin"),
	url:   questPG("questdb", "admin"),
	users: []Principal{{Role: User, User: QuestDBUser, dsn: questPG("postgres", QuestDBUser), url: questPG("questdb", QuestDBUser)}},
}

// questPG is the address of the PostgreSQL interface as one user, under the
// scheme given: postgres for the DSN pgx takes, and questdb for dburl's URL.
// It is on the second host port, computed from the first one given.
func questPG(scheme, user string) func(port int) string {
	return func(port int) string {
		u := url.URL{
			Scheme:   scheme,
			User:     url.UserPassword(user, Password),
			Host:     fmt.Sprintf("127.0.0.1:%d", SecondHostPort(port)),
			Path:     "/qdb",
			RawQuery: "sslmode=disable",
		}
		return u.String()
	}
}

// QuestDB is every QuestDB release dbmeta is tested against. Both are Tested,
// which is the cadence they kept while they were Staged (D120).
var QuestDB = list{}.add(questdb, Tested, "9.4.3", "10.0.1")
