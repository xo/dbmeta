// Package influxql holds the metadata queries for InfluxQL, the query
// language of InfluxDB 1 and of InfluxDB 2 through its v1 API.
//
// Import it for its effect:
//
//	import _ "github.com/xo/dbmeta/models/influxql"
//
// # SHOW, not SELECT
//
// InfluxQL reads metadata only through SHOW statements, and each one reads a
// single database, which ON names. So most kinds here are answered by a
// walk, [dbmeta.Binding.Walk], which lists the databases and then asks
// inside each one. Ken allowed that for InfluxQL on 2026-10-01, and the cost
// of each walk is written beside it. A kind that one SHOW statement answers
// is one statement. The caller's patterns are matched in Go, with
// [dbmeta.Like], because a SHOW statement takes no LIKE. See D146 and D159.
//
// A database is the schema and a measurement is the table. Its time, its
// tags and its fields are the columns. A user is the role, and the grants of
// each user on each database are the privileges. D165 holds the mapping and
// the reasons for it.
//
// The model reads InfluxQL through dbimp's influxdb driver, which is what
// dburl's influxql scheme opens. The driver speaks InfluxQL with the key
// sqlmode=disable, which dburl adds. InfluxDB 3 answers InfluxQL too, and
// its SQL is another dialect, influxdb, which models/influxdb reads (D152).
//
// It answers 7 of the 56 questions on 1.11.8 and 1.13.1. On 2.8.0 and
// 2.9.1, and on InfluxDB 3, the server answers four of them and refuses
// Roles, Privileges and Settings, because it has no SHOW USERS, SHOW GRANTS
// or SHOW DIAGNOSTICS. No InfluxQL statement names the release, so
// [dbmeta.Dialect.Version] reports an unknown version, and the model cannot
// say so in advance. The server's refusal is the answer. A caller that reads
// the release from GET /ping, as usql does, passes it to
// [dbmeta.Dialect.ParseVersion].
package influxql

import (
	"strconv"
	"strings"

	"github.com/xo/dbmeta"
)

func init() {
	dbmeta.RegisterDialect(dbmeta.InfluxQL, &dbmeta.Info{
		// InfluxQL takes `--` and `/* */` comments, and keeps the case of a
		// name, measured on 1.13.1 and 3.11.5. usql sets no lexer flags for
		// InfluxQL (D143).
		Syntax: dbmeta.Syntax{BlockComments: true},
		Fold:   dbmeta.FoldNone,
		// The server binds $name and $1, and the driver sends a positional
		// argument as $1 (dbimp D111).
		Placeholder: func(n int) string { return "$" + strconv.Itoa(n) },
		// No InfluxQL statement names the release. SHOW DIAGNOSTICS does on
		// InfluxDB 1, and only for an administrator, and InfluxDB 2 and 3
		// do not have it. Only GET /ping names it, which no statement
		// reaches, so there is no version statement and Dialect.Version
		// reports an unknown version. A caller that reads the release from
		// the driver, as usql does, passes it to ParseVersion (D165).
		ParseVersion: parseVersion,
	})
	register()
}

// parseVersion reads the release that GET /ping reports in the header
// X-Influxdb-Version, such as 1.13.1, v2.9.1 or 3.11.5. dbimp's influxdb
// driver returns it from its Version function, without the v.
func parseVersion(cols []string) (dbmeta.VersionSet, error) {
	if len(cols) < 1 {
		return dbmeta.VersionSet{}, dbmeta.ErrInvalidVersion
	}
	release := strings.TrimPrefix(strings.TrimSpace(cols[0]), "v")
	v := dbmeta.ParseVersion(release)
	if v.Unknown {
		return dbmeta.VersionSet{}, dbmeta.ErrInvalidVersion
	}
	v.Raw = release
	var set dbmeta.VersionSet
	set.Set("", v)
	set.Display = "InfluxDB " + release
	return set, nil
}

// quote returns name between double quotes, as InfluxQL writes an
// identifier. A backslash, a double quote and a newline are escaped with a
// backslash, as the InfluxQL parser reads them.
func quote(name string) string {
	return `"` + identEscaper.Replace(name) + `"`
}

// identEscaper escapes the characters that end or break a quoted identifier.
var identEscaper = strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`)
