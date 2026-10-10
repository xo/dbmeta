package container

import "fmt"

// The BigQuery emulator releases dbrun starts.
//
// BigQuery is a hosted service, and the package hosted names it. This entry
// runs a community emulator of it, so that a person and CI can test without an
// account. usql reaches BigQuery with gorm.io/driver/bigquery, whose endpoint
// and disable_auth options point it at the emulator. The BigQuery model reads
// the hosted service, and this emulator answers too little of INFORMATION_SCHEMA
// for it, so no model reads this entry. See D117, D118 and D220.
//
// # The range
//
// ghcr.io/goccy/bigquery-emulator builds each release tag once, so the rule in
// D112 applies: the newest release of each of the last two lines. Checked on
// 2026-09-28, that is 0.8.1, of 2026-06-13, and 0.7.2. The driver of dbimp
// cannot read the result of a query job on 0.7.2, so that release is dropped
// and 0.8.1 is the only one (D225). The emulator is a
// community project under the MIT license, and Google does not maintain it.
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
	// The driver of dbimp reads the project, the dataset and the endpoint, and
	// no credential for an emulator. See D225.
	url: func(port int) string {
		return fmt.Sprintf("bigquery://admin@dbmeta/dbmeta?endpoint=http%%3A%%2F%%2F127.0.0.1%%3A%d", port)
	},
	api: bareHTTP,
}

// BigQuery is every BigQuery emulator release dbrun starts.
//
// Staged, because dbmeta has no model that reads it, so CI runs none of
// them. Each keeps the cadence it will have if a model reads it, which is
// what dbimp runs on each push and at night. See D119 and D120.
var BigQuery = list{}.staged(bigquery, Tested, "0.8.1")
