// Package vertica holds the metadata queries for Vertica.
//
// Import it for its effect:
//
//	import _ "github.com/xo/dbmeta/models/vertica"
//
// # v_catalog, and a lineage from PostgreSQL
//
// Vertica's catalog is the v_catalog schema, with the engine's running state
// in v_monitor, and every query here reads those two. Vertica began as a
// fork of PostgreSQL, and it shows: vsql is psql, names fold to lower case,
// the empty string and NULL are different values, and standard_conforming
// strings decides how a literal is escaped. It is also columnar, and that
// shows too: there is no index, and a projection is what a query is planned
// against.
//
// # What it answers
//
// 26 of the 61. Tables, schemas, columns, views, indexes, index columns,
// constraints, constraint columns, sequences, partitioned tables, comments,
// functions, aggregates, types, triggers, roles, role settings, role grants,
// privileges, databases, tablespaces, settings, foreign servers, foreign
// tables, the current schema and the current user.
//
// Five answer with an analogue. A projection is an index: a stored, sorted
// copy of some of a table's columns that the optimizer chooses between. A
// storage location is a tablespace, an HCatalog schema is a foreign server,
// and an external table, which reads files through a COPY statement, is a
// foreign table. A trigger is Vertica's own word, and it runs a procedure on
// a schedule rather than on a change to a table.
//
// # Versions
//
// Four releases, 7.2.1, 9.1.0, 10.1.1 and 25.1.0, and the catalog grew at
// every one: v_catalog has 56 tables on 7.2 and 89 on 10.1. The gates are
// below, each at the oldest release measured to have the thing. Triggers and
// role settings are refused as too old before 25.1.
//
// # What is missing
//
// 30 kinds. Vertica has no domain, enumerated type, collation object, cast,
// operator, text search object, extension, publication or large object. The
// parameters of a routine are a comma separated list of types on its row,
// and the named parameters a library function declares are options rather
// than arguments, so RoutineParameters has no answer. docs/COVERAGE.md holds
// the rest, including the leads a second opinion invented.
package vertica

import (
	"strings"

	"github.com/xo/dbmeta"
)

// versionQuery reads the server version.
//
// It is the statement usql runs for Vertica, so the two agree. See
// docs/USQL.md.
const versionQuery = `SELECT version()`

// parseVersion reads the one column versionQuery returns, which names the
// product and then the release after a v, such as Vertica Analytic Database
// v25.1.0-0. The part after the dash is the hotfix, which ParseVersion keeps
// as the suffix.
func parseVersion(cols []string) (dbmeta.VersionSet, error) {
	var s dbmeta.VersionSet
	if len(cols) != 1 {
		return s, dbmeta.ErrInvalidVersion
	}
	i := strings.LastIndex(cols[0], " v")
	if i < 0 {
		return s, dbmeta.ErrInvalidVersion
	}
	s.Set("", dbmeta.ParseVersion(cols[0][i+2:]))
	s.Display = cols[0]
	return s, nil
}

// The releases a fragment gates on. Each is the oldest release measured to
// have the thing, and the release before it in the test list lacks it.
var (
	// v91 brought CHECK constraints, SET USING columns, a function's owner
	// and a user's connection limit.
	v91 = dbmeta.V(9, 1)
	// v101 brought LISTAGG and a comment on a table column rather than on a
	// projection column.
	v101 = dbmeta.V(10, 1)
	// v251 is the oldest release measured to have PL/vSQL, a procedure's
	// language, owner and security, and triggers. 10.1 has none of them and
	// no release between is measured, so the gate is where they were seen.
	v251 = dbmeta.V(25, 1)
)

func init() {
	dbmeta.RegisterDialect(dbmeta.Vertica, &dbmeta.Info{
		// The syntax is usql's lexer flags for this product, and the fold
		// is measured by scanEveryQuery (D143).
		Syntax: dbmeta.Syntax{DollarQuotes: true, BlockComments: true},
		// vertica-sql-go binds by position and writes a question mark.
		Placeholder:    func(int) string { return "?" },
		VersionQuery:   versionQuery,
		VersionColumns: 1,
		ParseVersion:   parseVersion,
		QuotingQuery:   quotingQuery,
		QuotingColumns: 2,
		ParseQuoting:   parseQuoting,
		ChangePassword: changePassword,
	})
	registerRelations()
	registerRoutines()
	registerRoles()
	registerServer()
}

// text casts an expression to VARCHAR.
//
// A CASE whose branches are literals of different lengths is typed CHAR of
// the longest, so without this 'unique' arrived padded to the width of
// 'primary key', with the spaces on the end.
func text(expr string) string { return `CAST(` + expr + ` AS VARCHAR)` }

// always is a fragment every release takes.
func always(query string) dbmeta.Choice { return dbmeta.Choice{{Query: query}} }

// since is a fragment for a release at from or newer, with the one before it
// for anything older. Both select the same column, which is hard rule 3.
func since(from dbmeta.Version, newer, older string) dbmeta.Choice {
	return dbmeta.Choice{{Query: older}, {Min: from, Query: newer}}
}

// like builds a filter that matches everything when the parameter is empty.
func like(col, param string) string {
	return `(` + param + ` = '' OR ` + col + ` LIKE ` + param + `)`
}

// notSystem leaves out the schemas Vertica keeps for itself, which it marks.
func notSystem(schema string) string {
	return `(@with_system OR ` + schema + ` NOT IN (SELECT s.schema_name` +
		` FROM v_catalog.schemata s WHERE s.is_system_schema))`
}

// schemaAndName is the filter set most queries here take.
func schemaAndName(kind string) []dbmeta.Param {
	return []dbmeta.Param{
		{Name: "schema", Desc: "schema name pattern, empty for every schema", Default: ""},
		{Name: "name", Desc: kind + " name pattern, empty for every one", Default: ""},
		{Name: "with_system", Desc: "include the schemas Vertica keeps for itself, such as v_catalog and v_monitor", Default: false},
	}
}

// parentAndName is the filter set for a child of a table.
func parentAndName(kind string) []dbmeta.Param {
	return []dbmeta.Param{
		{Name: "schema", Desc: "schema name pattern, empty for every schema", Default: ""},
		{Name: "parent", Desc: "table name pattern, empty for every table", Default: ""},
		{Name: "name", Desc: kind + " name pattern, empty for every one", Default: ""},
		{Name: "with_system", Desc: "include the schemas Vertica keeps for itself", Default: false},
	}
}

// nameOnly is the filter set for an object that belongs to the database.
func nameOnly(kind string) []dbmeta.Param {
	return []dbmeta.Param{
		{Name: "name", Desc: kind + " name pattern, empty for every one", Default: ""},
	}
}

// comment is the comment on one object, read from v_catalog.comments, which
// holds every comment in the database with its object's kind.
func comment(kind, schema, name string) string {
	return `(SELECT m.comment FROM v_catalog.comments m WHERE m.object_type = '` + kind + `'` +
		` AND m.object_schema = ` + schema + ` AND m.object_name = ` + name + `)`
}
