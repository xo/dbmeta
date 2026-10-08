// Package avatica holds the metadata queries for the standalone Apache
// Avatica server, which stands in front of HSQLDB.
//
// Import it for its effect:
//
//	import _ "github.com/xo/dbmeta/models/avatica"
//
// # What Avatica is
//
// Avatica is the wire protocol of Apache Calcite. A client sends the calls
// of JDBC to a server over HTTP, and the server runs each one on a database
// behind it. The protocol has no catalog of its own, so what a statement
// reads is the catalog of that database. The standalone server of the Avatica
// project, the docker.io/apache/calcite-avatica-hypersql image, runs HSQLDB
// 2.4.1 in memory. This model reads that catalog, through dbimp's avatica
// driver, which dburl's avatica scheme opens. D186 holds the mapping.
//
// Apache Phoenix speaks the same protocol and has no model here. Its catalog
// is SYSTEM.CATALOG, it has no INFORMATION_SCHEMA, it has no statement for its
// version and it refuses a trailing semicolon. D186 says why one model cannot
// serve both and what a Phoenix dialect needs.
//
// # The catalog
//
// HSQLDB has an INFORMATION_SCHEMA of the SQL standard, with 64 views, and 27
// more whose names begin with SYSTEM_. They hold the JDBC metadata, such as
// SYSTEM_INDEXINFO, SYSTEM_COMMENTS and SYSTEM_PROPERTIES. Every query here is
// one statement over those. The catalog is always PUBLIC.
//
// # What a user sees
//
// HSQLDB filters INFORMATION_SCHEMA by what the user can access. An ordinary
// user sees the schemas in SYSTEM_SCHEMAS, and sees a table, its columns and
// the grants on it only when it has a privilege there. SCHEMATA lists only the
// schemas the user owns, so the owner of a schema is absent for an ordinary
// user. The administrator SA sees everything.
//
// # What it answers
//
// 24 of the 61. Databases, schemas, the current schema, the current user,
// tables, views, columns, indexes, index columns, constraints, constraint
// columns, triggers, sequences, functions, aggregates, routine parameters,
// types, domains, collations, comments, settings, roles, role grants and
// privileges.
//
// # What is missing
//
// 32 kinds. HSQLDB has no tablespace, access method, language, conversion,
// cast, operator, event trigger, large object catalog, default privilege,
// foreign data wrapper, text search object, publication, subscription,
// extension, partitioned table, enumerated type or statistic of a column.
// docs/COVERAGE.md holds the rest and the leads that were rejected.
//
// # The version
//
// VALUES (DATABASE_VERSION()) returns the release of HSQLDB, such as 2.4.1,
// and not the release of Avatica. usql runs the same statement. The Avatica
// release is not in any catalog.
package avatica

import (
	"strings"

	"github.com/xo/dbmeta"
)

// versionQuery reads the release of HSQLDB. It is the statement that usql's
// avatica driver runs, and it answers for every user.
const versionQuery = `VALUES (DATABASE_VERSION())`

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
	s.Display = "Avatica, HSQLDB " + release
	return s, nil
}

func init() {
	dbmeta.RegisterDialect(dbmeta.Avatica, &dbmeta.Info{
		// HSQLDB has line comments and block comments, and a name between
		// double quotes. It folds a name that is not quoted to upper case.
		// It takes a trailing semicolon, so the terminator stays as it is.
		Syntax:         dbmeta.Syntax{BlockComments: true},
		Fold:           dbmeta.FoldUpper,
		Placeholder:    func(int) string { return "?" },
		VersionQuery:   versionQuery,
		VersionColumns: 1,
		ParseVersion:   parseVersion,
	})
	registerRelations()
	registerRoutines()
	registerRoles()
	registerServer()
}

// always is a fragment that every release takes.
func always(query string) dbmeta.Choice { return dbmeta.Choice{{Query: query}} }

// text is a NULL of the type the select list needs. HSQLDB refuses a bare
// NULL in a select list or a UNION branch.
const text = `CAST(NULL AS VARCHAR(1))`

// like builds a filter that matches everything when the parameter is empty.
// The cast gives the first mark a type, because HSQLDB cannot infer one for a
// mark that is only compared with a literal.
func like(col, param string) string {
	return `(CAST(` + param + ` AS VARCHAR(256)) = '' OR ` + col + ` LIKE ` + param + `)`
}

// notSystem leaves out the two schemas the server keeps for itself.
func notSystem(col string) string {
	return `(CAST(@with_system AS BOOLEAN) OR ` + col + ` NOT IN ('INFORMATION_SCHEMA', 'SYSTEM_LOBS'))`
}

// inList is the condition that item is one of the words of a list parameter.
func inList(param, item string) string {
	return `(',' || CAST(` + param + ` AS VARCHAR(256)) || ',') LIKE ('%,' || ` + item + ` || ',%')`
}

// sqlType assembles a type from the columns the standard splits it into: the
// name, the length, the precision and the scale. udt names the column that
// holds the name of a user defined type.
func sqlType(p, udt string) string {
	return `CASE WHEN ` + p + `.DATA_TYPE = 'USER-DEFINED' THEN ` + p + `.` + udt +
		` WHEN ` + p + `.DATA_TYPE IN ('DECIMAL', 'NUMERIC') AND ` + p + `.NUMERIC_PRECISION IS NOT NULL` +
		` THEN ` + p + `.DATA_TYPE || '(' || CAST(` + p + `.NUMERIC_PRECISION AS VARCHAR(10)) || ',' || CAST(` + p + `.NUMERIC_SCALE AS VARCHAR(10)) || ')'` +
		` WHEN ` + p + `.CHARACTER_MAXIMUM_LENGTH IS NOT NULL` +
		` THEN ` + p + `.DATA_TYPE || '(' || CAST(` + p + `.CHARACTER_MAXIMUM_LENGTH AS VARCHAR(20)) || ')'` +
		` ELSE ` + p + `.DATA_TYPE END`
}

// schemaAndName is the filter set most queries here take.
func schemaAndName(kind string) []dbmeta.Param {
	return []dbmeta.Param{
		{Name: "schema", Desc: "schema name pattern, empty for every schema", Default: ""},
		{Name: "name", Desc: kind + " name pattern, empty for every one", Default: ""},
		system,
	}
}

// parentAndName is the filter set for a child of a table.
func parentAndName(kind string) []dbmeta.Param {
	return []dbmeta.Param{
		{Name: "schema", Desc: "schema name pattern, empty for every schema", Default: ""},
		{Name: "parent", Desc: "table name pattern, empty for every table", Default: ""},
		{Name: "name", Desc: kind + " name pattern, empty for every one", Default: ""},
		system,
	}
}

// system is the parameter that adds the two schemas the server keeps.
var system = dbmeta.Param{
	Name:    "with_system",
	Desc:    "include INFORMATION_SCHEMA and SYSTEM_LOBS, which HSQLDB keeps for itself",
	Default: false,
}

// catalogDesc describes the catalog field of every row.
const catalogDesc = "always PUBLIC, the one catalog HSQLDB has"
