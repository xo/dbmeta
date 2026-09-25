// Package sqlite3 holds the metadata queries for SQLite.
//
// Import it for its effect:
//
//	import _ "github.com/xo/dbmeta/models/sqlite3"
//
// # An embedded database has no server
//
// Everything else dbmeta reads is a server with a version an administrator
// chose. SQLite is a library, and the version is whichever one the Go driver
// was built with. There is nothing to upgrade separately and nothing to run in
// a container, so SQLite is tested in the unit job against the version the
// pinned driver ships. See D42.
//
// That makes a version gate almost pointless here, and there are none. Every
// pragma these queries read arrived by SQLite 3.37, released in 2021, and no
// Go driver still shipping is older than that. A gate would claim a precision
// nothing can check, because the only way to reach an older SQLite is to pin
// an old driver, which a consumer does deliberately.
//
// # No catalog, a set of pragmas instead
//
// SQLite has one catalog table, sqlite_schema, and it holds the DDL text of
// every object rather than its parts. The parts come from table valued
// pragmas, which take the name of an object and return its detail. A query
// joins the two:
//
//	FROM sqlite_schema m JOIN pragma_table_xinfo(m.name) c
//
// That correlated join is what makes one statement answer for every table
// rather than one statement per table. It needs SQLite 3.16, which is well
// below the floor.
//
// # What it answers, and the one thing it half answers
//
// Fourteen of the 55 object kinds, which is two more than the shared
// information_schema model. SQLite has no users, no roles and no grants at
// all, and it has no type catalog, because a declared type is an unenforced
// affinity hint rather than an object.
//
// Constraints is the half answer. Primary key, unique and foreign key are
// read from pragmas and are exact. A check constraint exists only as text
// inside sqlite_schema.sql, and dbmeta does not parse DDL, so a check
// constraint is missing from the result rather than wrong in it. That is
// stated on the query and in docs/COVERAGE.md.
package sqlite3

import (
	"strings"

	"github.com/xo/dbmeta"
)

// Reference is the SQLite release these queries were written against.
const Reference = "3.50.4"

func init() {
	dbmeta.RegisterDialect(dbmeta.SQLite3, &dbmeta.Info{
		Placeholder:    func(int) string { return "?" },
		VersionSQL:     `SELECT sqlite_version()`,
		VersionColumns: 1,
		ParseVersion:   parseVersion,
	})
	registerRelations()
	registerServer()
	registerExtra()
}

// parseVersion reads what sqlite_version() returns, such as "3.50.4".
//
// It is the version of the library the caller linked, not of anything running
// elsewhere, so the display line says SQLite and nothing about a host.
func parseVersion(cols []string) (dbmeta.VersionSet, error) {
	if len(cols) == 0 {
		return dbmeta.VersionSet{}, dbmeta.ErrInvalidVersion
	}
	raw := strings.TrimSpace(cols[0])
	ver := dbmeta.ParseVersion(raw)
	ver.Raw = raw
	var set dbmeta.VersionSet
	set.Set("", ver)
	set.Display = "SQLite " + raw
	return set, nil
}

// systemTables are the tables SQLite keeps for itself. Every one of them is
// named with the sqlite_ prefix, which is reserved, so one pattern covers them
// all and a new one does not need this list updated.
const systemTables = `m.name LIKE 'sqlite\_%' ESCAPE '\'`

// notSystem excludes them unless the caller asked for them.
const notSystem = `(@with_system OR NOT ` + systemTables + `)`

// The schema of an object. SQLite calls an attached database a schema and
// reaches it by name, and sqlite_schema is the one of the main database. A
// query that reads sqlite_schema therefore reports main, because that is the
// schema it read.
const mainSchema = `'main' AS "schema"`

func schemaNameSystem(kind string) []dbmeta.Param {
	return []dbmeta.Param{
		{Name: "schema", Desc: "schema name pattern, empty for every schema", Default: ""},
		{Name: "name", Desc: kind + " name pattern, empty for every " + kind, Default: ""},
		{Name: "with_system", Desc: "include the objects SQLite keeps for itself", Default: false},
	}
}

func schemaParentName(kind string) []dbmeta.Param {
	return append([]dbmeta.Param{
		{Name: "parent", Desc: "table name pattern, empty for every table", Default: ""},
	}, schemaNameSystem(kind)...)
}

// always wraps SQL that is the same on every SQLite release, which is all of
// it. It reads better than a nested literal at 60 call sites.
func always(sqlstr string) dbmeta.Choice { return dbmeta.Choice{{SQL: sqlstr}} }
