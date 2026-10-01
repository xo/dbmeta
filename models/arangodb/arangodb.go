// Package arangodb holds the metadata queries for ArangoDB, in AQL.
//
// Import it for its effect:
//
//	import _ "github.com/xo/dbmeta/models/arangodb"
//
// # What AQL can read
//
// ArangoDB has no information_schema and no catalog of tables. The HTTP API
// describes each collection, index, view, graph, analyzer and database, and
// AQL reaches only a part of that. The model reads what one AQL statement
// reads, which is the function COLLECTIONS(), the function SCHEMA_GET, the
// system collection _aqlfunctions, and the functions CURRENT_DATABASE() and
// CURRENT_USER(). Every query here is one statement over the whole database.
// D146 allows a walk for Impala alone, so a kind that needs one HTTP call for
// each collection is not answered. See D163.
//
// # The mapping
//
// A database is the schema, as it is on MySQL and ClickHouse, and the catalog
// is empty. A connection names one database in its path, and AQL cannot reach
// another one, so Schemas and CurrentSchema return that database alone. Ken
// chose this on 2026-10-02 (D168). A collection is the table. A collection's
// JSON schema rule, when it has one, gives the columns and one check
// constraint. A user defined AQL function is a function.
//
// # A document has no fixed shape
//
// ArangoDB does not keep the attributes of the documents in a collection. A
// document attribute is a column here only when the collection's JSON
// schema rule names it, because any other source reads the documents rather
// than a catalog, and D47 leaves such a kind unanswered.
//
// # What it answers
//
// 7 of the 56. The database of the connection as the schema and the current
// schema, collections as tables, the properties of a schema rule as columns,
// a schema rule as a check constraint, user defined functions and the
// current user.
//
// # What is missing
//
// 49 kinds. AQL cannot list the databases, the indexes, the views, the
// analyzers or the users, which only the HTTP API describes. COLLECTIONS()
// does not say whether a collection holds documents or edges. There is no
// sequence, no trigger, no user defined type and no comment. A graph is
// readable in _graphs and an analyzer in _analyzers, and neither fits a kind
// here. docs/COVERAGE.md holds the rest.
package arangodb

import (
	"strconv"
	"strings"

	"github.com/xo/dbmeta"
)

// versionQuery reads the release, such as 3.12.12.
//
// It is the statement usql's arangodb driver runs, and both print ArangoDB
// before the release. See docs/USQL.md.
const versionQuery = `RETURN VERSION()`

// parseVersion reads the one column versionQuery returns.
func parseVersion(cols []string) (dbmeta.VersionSet, error) {
	var s dbmeta.VersionSet
	if len(cols) != 1 || cols[0] == "" {
		return s, dbmeta.ErrInvalidVersion
	}
	ver := dbmeta.ParseVersion(strings.TrimSpace(cols[0]))
	if ver.Unknown {
		return s, dbmeta.ErrInvalidVersion
	}
	s.Set("", ver)
	s.Display = "ArangoDB " + cols[0]
	return s, nil
}

func init() {
	dbmeta.RegisterDialect(dbmeta.ArangoDB, &dbmeta.Info{
		// AQL has // and /* */ comments and names in backticks, which is
		// what dbimp's driver parses. The fold is measured by
		// scanEveryQuery: AQL keeps the case of a name (D143).
		Syntax: dbmeta.Syntax{BlockComments: true, SlashComments: true, Backticks: true},
		// The driver binds a positional value at @1, @2 and so on.
		Placeholder:    func(n int) string { return "@" + strconv.Itoa(n) },
		VersionQuery:   versionQuery,
		VersionColumns: 1,
		ParseVersion:   parseVersion,
	})
	registerRelations()
	registerRoutines()
}

// always is a fragment that every release takes. dbrun starts one release,
// and nothing here differs between releases.
func always(query string) dbmeta.Choice { return dbmeta.Choice{{Query: query}} }

// like builds a filter that matches everything when the parameter is empty.
// AQL's LIKE is SQL's, with % and _, and it compares case.
func like(expr, param string) string {
	return `(` + param + ` == '' OR ` + expr + ` LIKE ` + param + `)`
}

// noSchema is the filter on the schema parameter. A database is the schema,
// and AQL reaches only the database of the connection, so only a pattern
// that matches its name returns rows.
var noSchema = like(`CURRENT_DATABASE()`, `@schema`)

// schemaParam is the schema parameter of every query that takes one.
var schemaParam = dbmeta.Param{
	Name: "schema",
	Desc: "database name pattern, empty for the database of the connection," +
		" which is the only one AQL reaches",
	Default: "",
}

// schemaDesc describes the schema field of every row.
const schemaDesc = "the database of the connection, which is the only one AQL reaches"

// catalogDesc describes the catalog field of every row.
const catalogDesc = "always empty: ArangoDB has nothing above a database"
