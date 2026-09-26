// Package presto holds the metadata queries for Presto.
//
// Import it for its effect:
//
//	import _ "github.com/xo/dbmeta/models/presto"
//
// # Presto is not Trino, and this is not models/trino
//
// The two are the same program forked in 2019 and six years apart. They share
// system.jdbc exactly, and almost nothing else this package needs: Presto has
// no version() function, no system.metadata.table_comments, no queryable table
// comment at all, and neither current_catalog nor current_schema resolves.
// A shared dialect would branch on which product it was talking to rather than
// on a version, which is two dialects sharing a struct. D73 has the
// measurement.
//
// # A catalog is a real level here, and it is the first time
//
// Every other model in dbmeta returns an empty catalog or repeats the
// database name into it, because the products have two levels and psql has
// three. Presto has all three. A table is catalog.schema.name, a catalog is a
// configured connector rather than a database, and one server reaches many of
// them at once.
//
// So Presto and Trino are the only models that take a catalog filter, and
// [dbmeta.Args] has carried the field all along for them.
//
// # system.jdbc, not information_schema
//
// Presto ships an information_schema inside every catalog and a system catalog
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
// Presto's information_schema.columns is the richer of the two, carrying a
// comment and an extra_info that Trino's does not have, and it is still not
// used: it reaches one catalog and system.jdbc reaches them all.
//
// # What it answers
//
// 9 of the 55. Catalogs as databases, schemas, tables, columns, views, types,
// access methods, privileges and the current user.
//
// Four that models/trino answers are missing here and none is a gap in this
// model. Comments has no source: Presto accepts a COMMENT clause on CREATE
// TABLE, stores nothing, and shows nothing in SHOW CREATE TABLE. CurrentSchema
// has no expression: neither current_catalog nor current_schema resolves, and
// nothing in system.runtime carries the session. Roles and RoleGrants read the
// standard views and the memory connector raises NOT_SUPPORTED rather than
// answering nothing, which D34 says is not a query to offer.
//
// Presto is a query engine rather than a store, so most of what is missing is
// missing because the engine has no such thing rather than because the
// catalog hides it. There are no indexes, no constraints, no sequences and no
// triggers in Presto at any release. docs/COVERAGE.md holds the rest.
package presto

import "github.com/xo/dbmeta"

// versionQuery reads the server version.
//
// Presto has no version() function, which is the first thing that separates it
// from Trino, so this reads the coordinator's row instead. It answers
// something like 0.299-7d50721: the release, then the build it was cut from.
// usql reads the same table for both products.
const versionQuery = `SELECT node_version FROM system.runtime.nodes WHERE coordinator = true LIMIT 1`

// parseVersion reads the one column versionQuery returns.
func parseVersion(cols []string) (dbmeta.VersionSet, error) {
	var s dbmeta.VersionSet
	if len(cols) != 1 {
		return s, dbmeta.ErrInvalidVersion
	}
	s.Set("", dbmeta.ParseVersion(cols[0]))
	s.Display = "Presto " + cols[0]
	return s, nil
}

func init() {
	dbmeta.RegisterDialect(dbmeta.Presto, &dbmeta.Info{
		// presto-go-client binds by position and writes a question mark. It
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

// systemSchemas are the schemas Presto puts inside every catalog.
const systemSchemas = `'information_schema'`

// systemCatalogs are the catalogs Presto serves itself rather than data.
//
// system holds the metadata this model reads and jmx exposes the JVM. tpch
// and tpcds are sample data generators rather than server internals, so they
// are left in: a person who starts the image and asks what is there should be
// shown them.
const systemCatalogs = `'system', 'jmx'`

// notSystem filters both out unless the caller asks for them. The columns are
// named by the caller, because system.jdbc spells them table_cat and
// table_schem and information_schema spells them out in full.
// The comparison is against true rather than 1, because Presto applies no
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
		{Name: "with_system", Desc: "include the catalogs and schemas Presto keeps for itself", Default: false},
	}
}
