// Package duckdb holds the metadata queries for DuckDB.
//
// Import it for its effect:
//
//	import _ "github.com/xo/dbmeta/models/duckdb"
//
// # A catalog of table functions
//
// DuckDB publishes its catalog as table functions named duckdb_something,
// rather than as views. They behave like tables in a FROM clause, so a query
// here reads much like a PostgreSQL one:
//
//	FROM duckdb_columns() c JOIN duckdb_tables() t ON t.table_oid = c.table_oid
//
// The functions carry lists where PostgreSQL carries arrays, and
// unnest(list) WITH ORDINALITY turns one into rows. That is how the columns of
// a constraint, the columns of an index, the labels of an enum and the
// parameters of a function are read, and it is the same shape the PostgreSQL
// model uses for the same four things.
//
// # An embedded database with no server
//
// Like SQLite, DuckDB is a library. The release under test is whichever one
// the Go driver was built with, there is nothing to run in a container, and
// the model carries no version gate. D42 puts both in a CI job that starts
// nothing.
//
// Unlike SQLite, the driver needs cgo, which D48 allows in the test module and
// nowhere else.
//
// # What it answers
//
// Twenty of the 55, which is more than any model here except PostgreSQL. The
// catalog is unusually complete for an embedded database: it has comments on
// most objects, real enumerated types, sequences, and a constraint catalog
// that names the columns of a key and the columns it references.
//
// What it has none of is everything that needs more than one user or more than
// one process. No roles, no privileges, no triggers, no tablespaces. See
// docs/COVERAGE.md for the rejected analogues, of which the interesting one is
// column statistics.
package duckdb

import (
	"strings"

	"github.com/xo/dbmeta"
)

// Reference is the DuckDB release these queries were written against.
const Reference = "1.5.5"

func init() {
	dbmeta.RegisterDialect(dbmeta.DuckDB, &dbmeta.Info{
		Placeholder:    func(int) string { return "?" },
		VersionQuery:   `SELECT version()`,
		VersionColumns: 1,
		ParseVersion:   parseVersion,
	})
	registerRelations()
	registerServer()
}

// parseVersion reads what version() returns, such as "v1.5.5". The leading v
// is what DuckDB prints and ParseVersion already skips it.
func parseVersion(cols []string) (dbmeta.VersionSet, error) {
	if len(cols) == 0 {
		return dbmeta.VersionSet{}, dbmeta.ErrInvalidVersion
	}
	raw := strings.TrimSpace(cols[0])
	ver := dbmeta.ParseVersion(raw)
	ver.Raw = raw
	var set dbmeta.VersionSet
	set.Set("", ver)
	set.Display = "DuckDB " + raw
	return set, nil
}

// internalOf excludes the objects DuckDB ships with itself, for one catalog
// alias. Every catalog function carries the flag, so this is the whole of the
// system object rule and there is no list of schema names to keep up to date.
// Every other model here carries such a list.
func internalOf(alias string) string {
	return `(@with_system OR NOT ` + alias + `.internal)`
}

func schemaNameSystem(kind string) []dbmeta.Param {
	return []dbmeta.Param{
		{Name: "schema", Desc: "schema name pattern, empty for every schema", Default: ""},
		{Name: "name", Desc: kind + " name pattern, empty for every " + kind, Default: ""},
		{Name: "with_system", Desc: "include the objects DuckDB ships with itself", Default: false},
	}
}

func schemaParentName(kind string) []dbmeta.Param {
	return append([]dbmeta.Param{
		{Name: "parent", Desc: "table name pattern, empty for every table", Default: ""},
	}, schemaNameSystem(kind)...)
}

// always wraps SQL that is the same on every release, which is all of it.
func always(query string) dbmeta.Choice { return dbmeta.Choice{{Query: query}} }
