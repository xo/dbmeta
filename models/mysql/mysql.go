// Package mysql holds the metadata queries for MariaDB and MySQL.
//
// Import it for its effect:
//
//	import _ "github.com/xo/dbmeta/models/mysql"
//
// # One driver, two products
//
// D14 makes a driver a family rather than a product. MariaDB is the reference
// these queries are written and tested against, and MySQL must also work. The
// two share information_schema for most of what they report and diverge at the
// edges, so a fragment gated on a version is gated on the MariaDB version
// unless it says otherwise.
//
// # What MariaDB answers with something of its own
//
// This is a native model rather than a shared one, because MariaDB records
// several things information_schema has no view for, and answers several of
// psql's object kinds with an analogue:
//
//   - a storage engine is the closest thing to an access method, in ENGINES
//   - a plugin is the closest thing to an extension, in ALL_PLUGINS
//   - a comment lives on the object, in TABLES.TABLE_COMMENT and
//     COLUMNS.COLUMN_COMMENT, rather than in a catalog of comments
//   - indexes live in STATISTICS, which the standard does not define at all
//   - roles and their grants live in mysql.user and mysql.roles_mapping
//   - an aggregate is a row in mysql.proc or mysql.func, not a kind of routine
//   - a foreign server is a row in mysql.servers, and it carries the one
//     credential every local user reaches it with
//   - a foreign table is a table on an engine that reads remote data
//
// This model answers 23 of the 48 questions. The four that read the mysql
// schema need SELECT on it, because MariaDB publishes none of them through
// information_schema.
//
// A schema and a database are the same thing here, which is the one place the
// object model does not fit. See COVERAGE.md for what it cannot answer.
package mysql

import "strings"

import "github.com/xo/dbmeta"

// Reference is the MariaDB release these queries were written against.
const Reference = "11.8.9-MariaDB"

// Releases a fragment gates on. These are MariaDB releases.
var (
	v10_2 = dbmeta.V(10, 2)
	v11_5 = dbmeta.V(11, 5)
)

func init() {
	dbmeta.RegisterDialect(dbmeta.MySQL, &dbmeta.Info{
		Placeholder:    func(int) string { return "?" },
		VersionSQL:     `SELECT VERSION()`,
		VersionColumns: 1,
		ParseVersion:   parseVersion,
	})
	registerRelations()
	registerRoutines()
	registerServer()
	registerForeign()
}

// parseVersion reads what SELECT VERSION() returns, such as "11.8.9-MariaDB"
// or "8.0.36".
//
// The suffix is where the flavor shows. MariaDB appends its own name and MySQL
// does not. D14 records a flavor rather than ranking it, so the suffix is kept
// and never compared.
func parseVersion(cols []string) (dbmeta.VersionSet, error) {
	if len(cols) == 0 {
		return dbmeta.VersionSet{}, dbmeta.ErrInvalidVersion
	}
	raw := strings.TrimSpace(cols[0])
	ver := dbmeta.ParseVersion(raw)
	ver.Raw = raw
	product := "MySQL"
	if strings.Contains(strings.ToLower(ver.Suffix), "mariadb") {
		product = "MariaDB"
	}
	var set dbmeta.VersionSet
	set.Set("", ver)
	set.Display = product + " " + raw
	return set, nil
}

// IsMariaDB reports whether a version set came from MariaDB rather than MySQL.
// A caller narrowing behaviour by flavor reads this.
func IsMariaDB(versions dbmeta.VersionSet) bool {
	return strings.Contains(strings.ToLower(versions.Main().Suffix), "mariadb")
}

// systemSchemas are the schemas MariaDB keeps for itself.
const systemSchemas = `'mysql', 'information_schema', 'performance_schema', 'sys'`

func fields(names ...string) []dbmeta.Field { return dbmeta.Fields(names...) }

func schemaNameSystem(kind string) []dbmeta.Param {
	return []dbmeta.Param{
		{Name: "schema", Desc: "schema name pattern, empty for every schema", Default: ""},
		{Name: "name", Desc: kind + " name pattern, empty for every " + kind, Default: ""},
		{Name: "with_system", Desc: "include the schemas MariaDB keeps for itself", Default: false},
	}
}

func schemaParentName(kind string) []dbmeta.Param {
	return append([]dbmeta.Param{
		{Name: "parent", Desc: "table name pattern, empty for every table", Default: ""},
	}, schemaNameSystem(kind)...)
}

func nameSystem(kind string) []dbmeta.Param {
	return []dbmeta.Param{
		{Name: "name", Desc: kind + " name pattern, empty for every " + kind, Default: ""},
		{Name: "with_system", Desc: "include what MariaDB keeps for itself", Default: false},
	}
}
