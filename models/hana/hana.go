// Package hana holds the metadata queries for SAP HANA.
//
// Import it for its effect:
//
//	import _ "github.com/xo/dbmeta/models/hana"
//
// # SYS, and how much of it there is
//
// SAP HANA has no information_schema. Its catalog is the SYS schema, which
// is a large set of views a statement reads like any other. It is the
// richest catalog in this project after PostgreSQL's, and the reason this
// model answers as much as it does: a concept PostgreSQL has usually has a
// view here rather than an analogue that has to be argued for.
//
// Three prefixes appear. A plain name such as SYS.TABLES is the catalog. A
// name beginning M_, such as SYS.M_DATABASES, is a monitoring view and
// reports what the server is doing now rather than what is defined. A name
// beginning _SYS_ is a schema the server owns, not a view.
//
// # What it answers
//
// 32 of the 55. Every one was run against SAP HANA 2.0 SPS 08 and SPS 07,
// which answer identically, so this model has no version fragment.
//
// The ones worth naming are the ones no other model here answers from a
// catalog rather than by analogy. Smart data access gives real foreign data:
// SYS.ADAPTERS is a wrapper, SYS.REMOTE_SOURCES is a server, SYS.REMOTE_USERS
// is a user mapping and SYS.VIRTUAL_TABLES is a foreign table, so all four
// are answered rather than reported absent. SYS.REMOTE_SUBSCRIPTIONS answers
// Subscriptions. SYS.DATA_STATISTICS answers extended statistics and
// SYS.PARTITIONED_TABLES answers partitioned tables.
//
// # What is missing
//
// 23 kinds, and almost all of them because HANA has no such object. There is
// no CREATE DOMAIN, no enumerated type, no user defined cast, operator or
// aggregate, no tablespace, no DDL trigger, no publication, and no default
// privilege. docs/COVERAGE.md holds the rest, including the leads a second
// opinion invented.
//
// # Reading a boolean
//
// HANA records a flag as the string TRUE or FALSE, and it refuses a bare
// comparison in a select list: `SELECT IS_PRIMARY_KEY = 'TRUE' FROM ...` is a
// syntax error rather than a boolean. Every flag here goes through [yes],
// which wraps it in a CASE.
package hana

import "github.com/xo/dbmeta"

// versionQuery reads the server version, which HANA reports as five parts
// and a build number, such as 2.00.088.00.1760424921.
//
// This is the statement usql runs for HANA as well. See docs/USQL.md.
const versionQuery = `SELECT VERSION FROM SYS.M_DATABASE`

// parseVersion reads the one column versionQuery returns.
func parseVersion(cols []string) (dbmeta.VersionSet, error) {
	var s dbmeta.VersionSet
	if len(cols) != 1 {
		return s, dbmeta.ErrInvalidVersion
	}
	s.Set("", dbmeta.ParseVersion(cols[0]))
	s.Display = "SAP HANA " + cols[0]
	return s, nil
}

func init() {
	dbmeta.RegisterDialect(dbmeta.HANA, &dbmeta.Info{
		// go-hdb binds by position and writes a question mark. A parameter
		// repeated in the statement is sent twice, which is what every
		// filter here does, and HANA infers the type from the comparison
		// without a cast. Both were measured rather than assumed.
		Placeholder:    func(int) string { return "?" },
		VersionQuery:   versionQuery,
		VersionColumns: 1,
		ParseVersion:   parseVersion,
	})
	registerRelations()
	registerRoutines()
	registerRoles()
	registerServer()
	registerForeign()
}

// always is a fragment every release takes.
func always(query string) dbmeta.Choice { return dbmeta.Choice{{Query: query}} }

// yes turns one of HANA's TRUE and FALSE strings into a boolean.
//
// The CASE is not decoration. HANA refuses a bare comparison in a select
// list, so `SELECT IS_PRIMARY_KEY = 'TRUE' FROM SYS.CONSTRAINTS` is
// answered with a syntax error and not with a column.
func yes(col string) string {
	return `CASE WHEN ` + col + ` = 'TRUE' THEN TRUE ELSE FALSE END`
}

// no is yes inverted, and it has to be written out rather than compared.
//
// HANA refuses a comparison in a select list whatever is on the left of it,
// so `CASE WHEN x = 'TRUE' THEN TRUE ELSE FALSE END = FALSE` is a syntax
// error for the same reason `x = 'TRUE'` is.
func no(col string) string {
	return `CASE WHEN ` + col + ` = 'TRUE' THEN FALSE ELSE TRUE END`
}

// notSystem leaves out the schemas the server owns.
//
// It tests a prefix with LEFT rather than with LIKE, because an underscore
// is a single character wildcard in LIKE and every one of these names
// begins with one. `LIKE '_SYS%'` also matches ASYS, BSYS and anything else
// of that shape.
func notSystem(col string) string {
	return `(@with_system = TRUE OR (` + col + ` NOT IN ('SYS', 'SYSTEM', 'PUBLIC')` +
		` AND LEFT(` + col + `, 4) <> '_SYS'` +
		` AND LEFT(` + col + `, 4) <> 'SYS_'))`
}

// like builds a filter that matches everything when the parameter is empty.
func like(col, param string) string {
	return `(` + param + ` = '' OR ` + col + ` LIKE ` + param + `)`
}

// schemaAndName is the filter set most queries here take.
func schemaAndName(kind string) []dbmeta.Param {
	return []dbmeta.Param{
		{Name: "schema", Desc: "schema name pattern, empty for every schema", Default: ""},
		{Name: "name", Desc: kind + " name pattern, empty for every one", Default: ""},
		{Name: "with_system", Desc: "include the schemas SAP HANA keeps for itself", Default: false},
	}
}

// parentAndName is the filter set for a child of a table.
func parentAndName(kind string) []dbmeta.Param {
	return []dbmeta.Param{
		{Name: "schema", Desc: "schema name pattern, empty for every schema", Default: ""},
		{Name: "parent", Desc: "table name pattern, empty for every table", Default: ""},
		{Name: "name", Desc: kind + " name pattern, empty for every one", Default: ""},
		{Name: "with_system", Desc: "include the schemas SAP HANA keeps for itself", Default: false},
	}
}
