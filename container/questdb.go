package container

// The QuestDB releases dbrun starts.
//
// dbmeta has no QuestDB model. The releases are here so that dbrun can start
// a server for the tests of the QuestDB driver in github.com/xo/dbimp, which
// sends SQL to /exec on the HTTP interface. No dialect is named yet, because
// dbimp settles the name with the driver. See D118.
//
// # The range
//
// docker.io/questdb/questdb builds each release tag once, so the rule in D112
// applies: the newest release of each of the last two lines. Checked on
// 2026-09-28, that is 10.0.1, of 2026-08-24, and 9.4.3, of 2026-06-15. The
// tags that end in -rhel and the patch and nightly tags are left out. The open
// source edition is under the Apache 2.0 licence.
//
// # One user
//
// The open source edition checks one user on the HTTP interface, admin with
// [Password], and has no ordinary user there. Its PostgreSQL port has a user
// that may only read, and dbrun publishes one port, which is the HTTP one.
// QuestDB has no databases, so nothing is named dbmeta.

// questdb is the QuestDB image.
var questdb = product{
	name:  "questdb",
	image: "docker.io/questdb/questdb",
	port:  9000,
	env: map[string]string{
		"QDB_HTTP_USER":     "admin",
		"QDB_HTTP_PASSWORD": Password,
		"QDB_PG_USER":       "admin",
		"QDB_PG_PASSWORD":   Password,
		// The usage report that goes to the vendor.
		"QDB_TELEMETRY_ENABLED": "false",
	},
	ready: []string{"sh", "-c", "curl -sf -o /dev/null -u 'admin:" + Password +
		"' -G --data-urlencode 'query=SELECT 1' http://127.0.0.1:9000/exec"},
	dsn: keyHTTP("admin", Password),
}

// QuestDB is every QuestDB release dbrun starts.
//
// Staged, because dbmeta has no model that reads it, so CI runs none of
// them. Each keeps the cadence it would have if a model read it, which is
// what dbimp runs on each push and at night. See D119 and D120.
var QuestDB = list{}.staged(questdb, Tested, "9.4.3", "10.0.1")
