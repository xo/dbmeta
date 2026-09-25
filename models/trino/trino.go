// Package trino holds the metadata queries for Trino.
//
// Import it for its effect:
//
//	import _ "github.com/xo/dbmeta/models/trino"
//
// # A catalog is a real level here, and it is the first time
//
// Every other model in dbmeta returns an empty catalog or repeats the
// database name into it, because the products have two levels and psql has
// three. Trino has all three. A table is catalog.schema.name, a catalog is a
// configured connector rather than a database, and one server reaches many of
// them at once.
//
// So Trino is the only model that takes a catalog filter, and [dbmeta.Args]
// has carried the field all along for it.
//
// # system.jdbc, not information_schema
//
// Trino ships an information_schema inside every catalog and a system catalog
// beside them, and the two differ in reach. A query against
// memory.information_schema.tables sees the memory catalog and nothing else,
// and there is no way to name the catalog from a bind parameter, so a filter
// on a second catalog would return nothing rather than an answer. The tables
// under system.jdbc span every catalog the server has.
//
// system.jdbc is also the richer of the two. Its columns table carries the
// column comment in remarks, which information_schema.columns does not have a
// column for at all.
//
// Views are the exception and they have to be. No cross catalog source
// carries a view definition, so Views reads the information_schema of the
// session catalog and says so in its parameter description.
//
// # What it answers
//
// 13 of the 55. Catalogs as databases, schemas, tables, columns, views,
// comments, types, access methods, roles, role grants, privileges, the
// current schema and the current user.
//
// The floor is 476 and the fixture sets it: the memory connector refuses a
// NOT NULL column before that, so there is nothing to read the queries
// against. container/trino.go has the measurement. Both releases take the
// same statements and the model has no version fragment.
//
// Trino is a query engine rather than a store, so most of what is missing is
// missing because the engine has no such thing rather than because the
// catalog hides it. There are no indexes, no constraints, no sequences and no
// triggers in Trino at any release. docs/COVERAGE.md holds the rest.
package trino

import "github.com/xo/dbmeta"

// versionQuery reads the server version, which Trino reports as one number
// such as 483. It has no major and minor: the number is the release.
const versionQuery = `SELECT version()`

// parseVersion reads the one column versionQuery returns.
func parseVersion(cols []string) (dbmeta.VersionSet, error) {
	var s dbmeta.VersionSet
	if len(cols) != 1 {
		return s, dbmeta.ErrInvalidVersion
	}
	s.Set("", dbmeta.ParseVersion(cols[0]))
	s.Display = "Trino " + cols[0]
	return s, nil
}

func init() {
	dbmeta.RegisterDialect(dbmeta.Trino, &dbmeta.Info{
		// trino-go-client binds by position and writes a question mark. It
		// sends the statement through PREPARE and EXECUTE, and a parameter
		// repeated in the text is repeated in the values, which is what every
		// filter here does.
		Placeholder:    func(int) string { return "?" },
		VersionQuery:   versionQuery,
		VersionColumns: 1,
		ParseVersion:   parseVersion,
	})
	registerRelations()
	registerExtra()
}

// always is a fragment every release takes.
func always(query string) dbmeta.Choice { return dbmeta.Choice{{Query: query}} }

// systemSchemas are the schemas Trino puts inside every catalog.
const systemSchemas = `'information_schema'`

// systemCatalogs are the catalogs Trino serves itself rather than data.
//
// system holds the metadata this model reads and jmx exposes the JVM. tpch
// and tpcds are sample data generators rather than server internals, so they
// are left in: a person who starts the image and asks what is there should be
// shown them.
const systemCatalogs = `'system', 'jmx'`

// notSystem filters both out unless the caller asks for them. The columns are
// named by the caller, because system.jdbc spells them table_cat and
// table_schem and information_schema spells them out in full.
// The comparison is against true rather than 1, because Trino applies no
// implicit conversion between a boolean and an integer and refuses the
// statement outright. ClickHouse coerces the same expression, which is why
// every other model writes = 1.
func notSystem(catalog, schema string) string {
	return `(@with_system = true OR (` + catalog + ` NOT IN (` + systemCatalogs + `)` +
		` AND ` + schema + ` NOT IN (` + systemSchemas + `)))`
}

// catalogSchemaName is the filter set a cross catalog query takes.
func catalogSchemaName(kind string) []dbmeta.Param {
	return []dbmeta.Param{
		{Name: "catalog", Desc: "catalog name pattern, empty for every catalog", Default: ""},
		{Name: "schema", Desc: "schema name pattern, empty for every schema", Default: ""},
		{Name: "name", Desc: kind + " name pattern, empty for every one", Default: ""},
		{Name: "with_system", Desc: "include the catalogs and schemas Trino keeps for itself", Default: false},
	}
}
