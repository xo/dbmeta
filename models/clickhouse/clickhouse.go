// Package clickhouse holds the metadata queries for ClickHouse.
//
// Import it for its effect:
//
//	import _ "github.com/xo/dbmeta/models/clickhouse"
//
// # system, not information_schema
//
// ClickHouse ships both and the native one answers more. information_schema is
// an emulation layer that reports what the standard names and drops what makes
// a ClickHouse table what it is: the engine, the partition key, the sorting
// key, the compression codec per column and the data skipping indices. Every
// query here reads system.
//
// # A database is a schema
//
// ClickHouse has one level of namespace and calls it a database. Schemas and
// Databases both read system.databases, which is what models/mysql does with
// information_schema.SCHEMATA for the same reason.
//
// # What it answers
//
// 23 of the 55. Schemas and databases, tables, columns, views, indexes, index
// columns, constraints, comments, partitioned tables, types, collations,
// settings, roles, role grants, privileges, functions, aggregates,
// tablespaces, foreign servers, foreign tables, the current schema and the
// current user.
//
// Constraints needs a server with system.constraints. It is absent on 25.3,
// 25.8 and 26.1 and present on 26.8, so the query gates at the release it was
// first seen in rather than at a release nobody here runs.
package clickhouse

import (
	"database/sql"

	"github.com/xo/dbmeta"
)

// versionQuery reads the server version, which ClickHouse reports as four
// numbers such as 26.9.2.8.
const versionQuery = `SELECT version()`

// parseVersion reads the one column versionQuery returns.
func parseVersion(cols []string) (dbmeta.VersionSet, error) {
	var s dbmeta.VersionSet
	if len(cols) != 1 {
		return s, dbmeta.ErrInvalidVersion
	}
	s.Set("", dbmeta.ParseVersion(cols[0]))
	s.Display = "ClickHouse " + cols[0]
	return s, nil
}

// changePassword builds the statement that sets a user's password.
//
// ClickHouse spells it ALTER USER ... IDENTIFIED BY, and the server hashes
// what it is given. A string literal there takes backslash escapes as well as
// a doubled quote, so both are applied and nothing needs detecting: the
// behaviour is not a server setting here the way it is in MySQL.
func changePassword(c dbmeta.PasswordChange, _ dbmeta.Quoting) (string, error) {
	backslashes := dbmeta.Quoting{BackslashEscapes: sql.Null[bool]{V: true, Valid: true}}
	return "ALTER USER " + dbmeta.QuoteIdentifier(c.User, "`", "`") +
		" IDENTIFIED BY " + dbmeta.QuoteLiteral(c.Password, backslashes), nil
}

func init() {
	dbmeta.RegisterDialect(dbmeta.ClickHouse, &dbmeta.Info{
		// clickhouse-go binds by position and writes a question mark.
		Placeholder:    func(int) string { return "?" },
		VersionQuery:   versionQuery,
		VersionColumns: 1,
		ParseVersion:   parseVersion,
		ChangePassword: changePassword,
	})
	registerRelations()
	registerExtra()
	registerRoles()
}

// v268 is where system.constraints appeared.
//
// It is absent on 25.3, 25.8 and 26.1 and present on 26.8 and 26.9, all
// checked against a running server. Somewhere in 26.2 to 26.8 it arrived, and
// the gate is the release it was seen in rather than a guess at the real one:
// a server between the two is told the query is too old, which under reports
// and never lies.
var v268 = dbmeta.V(26, 8)

// v256 is where system.named_collections gained create_query and source,
// which is ClickHouse pull request 78582 and the 25.6 changelog. This one is
// known rather than bracketed: measured absent on 25.3.14.14 and present on
// 25.8.33.6, and the release between them is what upstream records.
var v256 = dbmeta.V(25, 6)

// always is a fragment every release takes.
func always(query string) dbmeta.Choice { return dbmeta.Choice{{Query: query}} }

// systemDatabases are the databases ClickHouse keeps for itself.
const systemDatabases = `'system', 'information_schema', 'INFORMATION_SCHEMA'`

// notSystem filters those out unless the caller asks for them. The column is
// named by the caller, because system.databases calls it name and everything
// else calls it database.
func notSystem(col string) string {
	return `(@with_system = 1 OR ` + col + ` NOT IN (` + systemDatabases + `))`
}

// schemaNameSystem is the filter set most queries take.
func schemaNameSystem(kind string) []dbmeta.Param {
	return []dbmeta.Param{
		{Name: "schema", Desc: "database name pattern, empty for every database", Default: ""},
		{Name: "name", Desc: kind + " name pattern, empty for every one", Default: ""},
		{Name: "with_system", Desc: "include the databases ClickHouse keeps for itself", Default: false},
	}
}

// viewEngines are the table engines that make a table a view.
//
// ClickHouse has three and they are not the same thing. View is a stored
// query. MaterializedView writes rows as they arrive into a target table.
// LiveView and WindowView are experimental and are views all the same.
const viewEngines = `'View', 'MaterializedView', 'LiveView', 'WindowView'`

// foreignEngines are the table engines whose data lives somewhere else.
//
// A ClickHouse table with one of these is a foreign table in the PostgreSQL
// sense: the statement reads it like any other and the rows are held by
// another system. The list is the engines that reach a named remote rather
// than every integration engine, because a Kafka or a File engine is a pipe
// rather than a table somebody else owns.
const foreignEngines = `'MySQL', 'PostgreSQL', 'MaterializedPostgreSQL',` +
	` 'MongoDB', 'SQLite', 'ODBC', 'JDBC', 'Redis', 'S3', 'URL', 'HDFS',` +
	` 'Iceberg', 'DeltaLake', 'Hudi'`
