// Package bigquery holds the metadata queries for Google BigQuery, in the
// GoogleSQL dialect.
//
// Import it for its effect:
//
//	import _ "github.com/xo/dbmeta/models/bigquery"
//
// # What it reads
//
// It reads the hosted service. The emulator that dbrun can start answers a
// small part of INFORMATION_SCHEMA and is not what this model is written
// against. See D118 and D220.
//
// BigQuery has a project, which holds datasets, which hold tables. The model
// calls the project the catalog and a dataset a schema, as INFORMATION_SCHEMA
// does. Every view that this model reads is the view of one dataset, and the
// statements name it without a prefix, so they read the default dataset of the
// connection. The URL names it as bigquery://project/location/dataset. A model
// cannot name another dataset, because a dataset is part of the name of a view
// and a statement cannot bind a name.
//
// SCHEMATA lists every dataset of the project, and the model keeps the one that
// the tables of the connection name. It needs the role
// roles/bigquery.metadataViewer on the project. TABLE_STORAGE is not read.
// OBJECT_PRIVILEGES answers only a query that names one object, so it lists the
// grants on the dataset and not those of every table. See D226 and
// docs/COVERAGE.md.
//
// # What it answers
//
// 18 of the 65. Tables, columns, indexes, index columns, constraints,
// constraint columns, views, functions, aggregates, routine parameters,
// comments, partitioned tables, partitions, the current user, databases,
// schemas, the current schema and privileges.
//
// Four of those are an analogue, and docs/COVERAGE.md says why each is one. A
// search index and a vector index are indexes. A user defined aggregate
// function is an aggregate. A partition is the part of a table that a date or a
// number selects, and BigQuery names it with a table decorator. A project is a
// database, and Databases reports the one project that the session runs in,
// because BigQuery lists no other.
//
// BigQuery has primary keys and foreign keys, and it never enforces either.
// Constraints reports Enforced as false for each of them.
//
// The rest is absent or unreachable. BigQuery has no sequence, no trigger, no
// user defined type, no extension and no role, because access is by IAM. A row
// access policy and a data policy exist, and INFORMATION_SCHEMA has no view of
// either.
//
// # The version
//
// BigQuery is a service that has no release, and no function or view reports
// one. The model declares no version query, so the version is unknown. See
// D220.
//
// # The text NULL
//
// COLUMNS reports the text NULL, as a string, for a column with no default and
// for a column with no collation. The model turns that text into a NULL, which
// is what it means.
package bigquery

import (
	"github.com/xo/dbmeta"
)

func init() {
	dbmeta.RegisterDialect(dbmeta.BigQuery, &dbmeta.Info{
		// GoogleSQL has block comments and hash comments, and quotes a name
		// with backticks.
		Syntax: dbmeta.Syntax{BlockComments: true, HashComments: true, Backticks: true},
		// The driver sends every parameter by position, and a position is a
		// question mark.
		Placeholder: func(int) string { return "?" },
	})
	registerRelations()
	registerDetail()
	registerExtra()
	registerSchemas()
}

// always is a fragment every release takes.
func always(query string) dbmeta.Choice { return dbmeta.Choice{{Query: query}} }

// like is the filter that matches col to the pattern in param, and passes when
// the pattern is empty.
func like(col, param string) string {
	return `(` + param + ` = '' OR ` + col + ` LIKE ` + param + `)`
}

// unquote turns col, a GoogleSQL string literal as OPTION_VALUE holds it, such
// as "a \"b\"", into the text it stands for. The literal is JSON that a parser
// reads, and a literal that is not falls back to itself.
func unquote(col string) string {
	return `COALESCE(SAFE.JSON_VALUE(SAFE.PARSE_JSON(` + col + `)), ` + col + `)`
}

// nullText turns the text NULL, which COLUMNS reports for a value it has none
// of, into a NULL.
func nullText(col string) string { return `NULLIF(` + col + `, 'NULL')` }

func schemaName(kind string) []dbmeta.Param {
	return []dbmeta.Param{
		{
			Name:    "schema",
			Desc:    "dataset name pattern, empty for every dataset. A statement reads the default dataset of the connection, so a pattern can only narrow that one",
			Default: "",
		},
		{Name: "name", Desc: kind + " name pattern, empty for every " + kind, Default: ""},
	}
}

func schemaParentName(parent, kind string) []dbmeta.Param {
	return append([]dbmeta.Param{
		{Name: "parent", Desc: parent + " name pattern, empty for every " + parent, Default: ""},
	}, schemaName(kind)...)
}

func nameOnly(kind string) []dbmeta.Param {
	return []dbmeta.Param{
		{Name: "name", Desc: kind + " name pattern, empty for every " + kind, Default: ""},
	}
}
