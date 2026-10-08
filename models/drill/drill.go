// Package drill holds the metadata queries for Apache Drill.
//
// Import it for its effect:
//
//	import _ "github.com/xo/dbmeta/models/drill"
//
// # The catalog
//
// Drill has an INFORMATION_SCHEMA with the tables CATALOGS, SCHEMATA, TABLES,
// COLUMNS, VIEWS and PARTITIONS. It has a sys schema too, with the options,
// the functions and the version. Every query here is one statement over one of
// them. The model reads them through dbimp's drill driver, which dburl's drill
// scheme opens. D178 holds the mapping and the reasons for it.
//
// Drill quotes a name with backticks. A double quoted alias fails with no
// message, so every alias in this model is between backticks.
//
// # The mapping
//
// The catalog is always DRILL. A schema is a workspace or a system schema,
// such as dfs.tmp, and its name keeps the dot. A table is a file table that
// the Drill Metastore knows, a view, or a system table. A column is a column
// of one of them.
//
// # The Metastore
//
// INFORMATION_SCHEMA lists no file table by default. It lists a file table
// only when the Metastore is on (the option metastore.enabled) and the table
// went through ANALYZE TABLE ... REFRESH METADATA. A model cannot set the
// option. If the Metastore is off, Tables and Columns answer the views and the
// system tables, with no error. COLUMNS holds the statistics of ColumnStats
// only for a table that was analyzed.
//
// # What a user sees
//
// Drill filters nothing in the catalog by user. The ordinary user of the tests
// sees the same rows as the administrator in every query here, and can read
// the version.
//
// # What it answers
//
// 10 of the 61. Schemas, the current schema, tables, columns, views,
// databases, functions, column statistics, settings and the current user.
//
// # What is missing
//
// 46 kinds. Drill has no index, constraint, key, trigger, sequence, comment,
// type or role that SQL lists, and a user defined function needs a JAR file.
// Its storage plugins are in an HTTP API and not in a table. docs/COVERAGE.md
// holds the rest.
//
// # The version
//
// sys.version holds the release, and every user can read it. This is the same
// statement that usql runs.
package drill

import (
	"strings"

	"github.com/xo/dbmeta"
)

// versionQuery reads the release, such as 1.22.0.
const versionQuery = "SELECT version FROM sys.version"

// parseVersion reads the one column versionQuery returns.
func parseVersion(cols []string) (dbmeta.VersionSet, error) {
	var s dbmeta.VersionSet
	if len(cols) != 1 || cols[0] == "" {
		return s, dbmeta.ErrInvalidVersion
	}
	release := strings.TrimSpace(cols[0])
	ver := dbmeta.ParseVersion(release)
	if ver.Unknown {
		return s, dbmeta.ErrInvalidVersion
	}
	ver.Raw = release
	s.Set("", ver)
	s.Display = "Apache Drill " + release
	return s, nil
}

func init() {
	dbmeta.RegisterDialect(dbmeta.Drill, &dbmeta.Info{
		// Drill has block comments and names in backticks. It refuses a
		// semicolon at the end of a statement, so a client removes it.
		Syntax:     dbmeta.Syntax{BlockComments: true, Backticks: true},
		Terminator: dbmeta.TerminatorStripped,
		// Drill compares names without their case and keeps the case it
		// reads, which is what scanEveryQuery measures (D143).
		Fold: dbmeta.FoldNone,
		// dbimp's driver writes each argument into the statement as a
		// literal, because Drill binds no parameter.
		Placeholder:    func(int) string { return "?" },
		VersionQuery:   versionQuery,
		VersionColumns: 1,
		ParseVersion:   parseVersion,
	})
	registerRelations()
	registerRoutines()
	registerServer()
}

// always is a fragment that every release takes.
func always(query string) dbmeta.Choice { return dbmeta.Choice{{Query: query}} }

// like builds a filter that matches everything when the parameter is empty.
// Drill refuses a LIKE whose pattern is not a constant, and the driver writes
// the argument as a literal, so the pattern is one.
func like(expr, param string) string {
	return `(` + param + ` = '' OR ` + expr + ` LIKE ` + param + `)`
}

// notSystem hides the two schemas Drill keeps for itself.
func notSystem(col string) string {
	return `(@with_system OR ` + col + ` NOT IN ('sys', 'information_schema'))`
}

// system is the parameter that adds those two schemas.
var system = dbmeta.Param{
	Name:    "with_system",
	Desc:    "include information_schema and sys, which Drill keeps for itself",
	Default: false,
}
