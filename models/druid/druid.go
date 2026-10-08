// Package druid holds the metadata queries for Apache Druid, in Druid SQL.
//
// Import it for its effect:
//
//	import _ "github.com/xo/dbmeta/models/druid"
//
// # The catalog
//
// Druid SQL has an INFORMATION_SCHEMA with four tables: SCHEMATA, TABLES,
// COLUMNS and ROUTINES. It has a sys schema too, with the segments, the
// servers, the server properties, the supervisors and the tasks. Every query
// here is one statement over one of them. The model reads the same sources
// through dbimp's druid driver, which dburl's druid scheme opens. D171 holds
// the mapping and the reasons for it.
//
// # The mapping
//
// A datasource is a table, and each one has the column `__time`. The schema of
// every datasource is druid. The other schemas are INFORMATION_SCHEMA and
// sys, which hold the server's own tables, and lookup and view. The catalog
// is always druid. A column is a column of a datasource. A function is a
// function of Druid SQL, and Druid has no user defined function, so each one
// is built in.
//
// # What a user sees
//
// Druid filters INFORMATION_SCHEMA by the permission of the user. The
// ordinary user of the tests can read every datasource, and it sees the same
// rows as the administrator. A user with no permission on a datasource does
// not see it. sys.servers and sys.server_properties need the permission
// STATE, so the version and the settings are for an administrator, and an
// ordinary user is refused with an HTTP 403 error.
//
// # What it answers
//
// 7 of the 65. Schemas, the current schema, tables, columns, functions,
// aggregates and settings.
//
// # What is missing
//
// 49 kinds. Druid has no DDL, so it has no index, constraint, trigger,
// sequence, view that a user makes, type, comment or role that SQL lists. Its
// users and roles are in the security API of the Coordinator, which is an HTTP
// API and not a catalog. A segment is a part of a datasource and not a table,
// and the statement that reads its size and its rows scans no data but is not
// a kind of the model. docs/COVERAGE.md holds the rest.
//
// # The version
//
// Druid has no function that returns the release. sys.servers holds the
// release of each service, so the version query reads it from the broker.
// Only an administrator can read it. See D171.
package druid

import (
	"strings"

	"github.com/xo/dbmeta"
)

// versionQuery reads the release, such as 37.0.0. Every service of a Druid
// cluster runs one release, and the broker is the one that answers SQL.
const versionQuery = `SELECT version FROM sys.servers WHERE server_type = 'broker' LIMIT 1`

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
	s.Display = "Apache Druid " + release
	return s, nil
}

func init() {
	dbmeta.RegisterDialect(dbmeta.Druid, &dbmeta.Info{
		// Druid SQL has line comments and block comments and names in double quotes.
		// It keeps the case of a name that is not quoted, which is what
		// scanEveryQuery measures (D143).
		Syntax:         dbmeta.Syntax{BlockComments: true},
		Fold:           dbmeta.FoldNone,
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
// The cast gives the first mark a type, because Druid cannot infer one for a
// mark that is only compared with a string.
func like(expr, param string) string {
	return `(CAST(` + param + ` AS VARCHAR) = '' OR ` + expr + ` LIKE ` + param + `)`
}

// notSystem hides the two schemas Druid keeps for itself.
func notSystem(col string) string {
	return `(@with_system OR ` + col + ` NOT IN ('INFORMATION_SCHEMA', 'sys'))`
}

// system is the parameter that adds those two schemas.
var system = dbmeta.Param{
	Name:    "with_system",
	Desc:    "include INFORMATION_SCHEMA and sys, which Druid keeps for itself",
	Default: false,
}
