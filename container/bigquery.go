package container

import "fmt"

// The BigQuery emulator releases dbrun starts.
//
// BigQuery is a hosted service, and the package hosted names it. This entry
// runs a community emulator of it, so that a person and CI can test without an
// account. usql reaches BigQuery with gorm.io/driver/bigquery, whose endpoint
// and disable_auth options point it at the emulator. dbmeta has no BigQuery
// model yet. See D117 and D118.
//
// # The range
//
// ghcr.io/goccy/bigquery-emulator builds each release tag once, so the rule in
// D112 applies: the newest release of each of the last two lines. Checked on
// 2026-09-28, that is 0.8.1, of 2026-06-13, and 0.7.2. The emulator is a
// community project under the MIT licence, and Google does not maintain it.
// Its INFORMATION_SCHEMA holds SCHEMATA, TABLES, TABLE_OPTIONS and COLUMNS,
// and not VIEWS, ROUTINES or JOBS, so it answers less than the service does.
//
// # No users
//
// The emulator checks nothing. The command makes the project dbmeta and the
// dataset dbmeta in it. The URL names the user admin, which nothing checks.
// The image has bash and no curl, so the check goes through bash's /dev/tcp.

// bigquery is the BigQuery emulator image.
var bigquery = product{
	name:  "bigquery",
	image: "ghcr.io/goccy/bigquery-emulator",
	port:  9050,
	args:  []string{"--project=dbmeta", "--dataset=dbmeta", "--port=9050"},
	ready: bashRequest(9050, "GET", "/bigquery/v2/projects/dbmeta/datasets/dbmeta", "", nil, 200),
	dsn: func(port int) string {
		return fmt.Sprintf("bigquery://admin@dbmeta/dbmeta?endpoint=http%%3A%%2F%%2F127.0.0.1%%3A%d&disable_auth=true", port)
	},
}

// BigQuery is every BigQuery emulator release dbrun starts.
//
// Staged, because dbmeta has no model that reads it, so CI runs none of
// them. See D119.
var BigQuery = list{}.add(bigquery, Staged, "0.7.2", "0.8.1")
