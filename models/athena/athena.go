// Package athena holds the metadata queries for Amazon Athena, in the Trino
// based SQL of Athena engine version 3.
//
// Import it for its effect:
//
//	import _ "github.com/xo/dbmeta/models/athena"
//
// # What it reads
//
// It reads the hosted service. Athena has no catalog of its own. The AWS Glue
// Data Catalog holds the databases, tables and columns, and Athena shows them
// through the INFORMATION_SCHEMA of the catalog awsdatacatalog. The model reads
// that: schemata, tables, columns and views. The model calls the catalog the
// database, a Glue database a schema, and a Glue table a table, as Athena does.
//
// Athena refuses the rest. The catalog system, which every other Trino model
// reads, answers "Queries of this type are not supported". Roles and
// table_privileges answer NOT_SUPPORTED, and SHOW CATALOGS, SHOW SESSION and
// SHOW STATS are syntax errors. SHOW FUNCTIONS, SHOW CREATE TABLE, SHOW
// TBLPROPERTIES, SHOW PARTITIONS and DESCRIBE answer, and they are statements
// rather than relations, so a SELECT cannot filter or join them. The walk of
// D146 is not allowed here, so what only they carry is not answered.
// docs/COVERAGE.md says what that is.
//
// # What it answers
//
// 8 of the 65. Databases, schemas, tables, columns, views, partitioned tables,
// the current schema and the current user.
//
// A partitioned table is a Hive table with a partition column. Athena marks the
// column with extra_info of partition key in COLUMNS, and a table has one row
// for each partition column, as the Hive model gives it. An Iceberg table
// partitioned by a transform has no such mark and is not listed.
//
// A table has no comment in any relation that a SELECT reads, so Table.Comment is
// always absent. The comment of a column is in COLUMNS.
//
// # The driver binds
//
// The driver that dburl v0.50.0 names for Athena is github.com/xo/dbimp/athena.
// It binds a parameter with the ExecutionParameters of the service, which writes
// each value as a literal that the service parses, so a quote or a backslash in a
// filter stays what the caller wrote. The dialect uses the placeholder ? and
// sets no [dbmeta.Info.Literal]. The driver returns a NULL as nil. See D222 and
// D229.
//
// # The version
//
// Athena is a service, and the engine version belongs to the workgroup, which
// only the Athena API reports. No SQL statement reads it, and SELECT version()
// is refused with FUNCTION_NOT_FOUND. The model declares no version query, so the
// version is unknown. See D222.
package athena

import (
	"github.com/xo/dbmeta"
)

func init() {
	dbmeta.RegisterDialect(dbmeta.Athena, &dbmeta.Info{
		Syntax: dbmeta.Syntax{BlockComments: true},
		// usql strips the semicolon at the end of a statement for Athena, and
		// Athena refuses it.
		Terminator:  dbmeta.TerminatorStripped,
		Fold:        dbmeta.FoldLower,
		Placeholder: func(int) string { return "?" },
	})
	registerRelations()
	registerExtra()
}

// always is a fragment every release takes.
func always(query string) dbmeta.Choice { return dbmeta.Choice{{Query: query}} }

// like is the filter that matches col to the pattern in param, and passes when
// the pattern is empty.
func like(col, param string) string {
	return `(` + param + ` = '' OR ` + col + ` LIKE ` + param + `)`
}

// notSystem hides the schema information_schema, which Athena lists beside
// the Glue databases, unless the caller asks for it.
func notSystem(schema string) string {
	return `(@with_system = true OR ` + schema + ` <> 'information_schema')`
}

func schemaName(kind string) []dbmeta.Param {
	return []dbmeta.Param{
		{Name: "schema", Desc: "Glue database name pattern, empty for every database", Default: ""},
		{Name: "name", Desc: kind + " name pattern, empty for every " + kind, Default: ""},
		{Name: "with_system", Desc: "include information_schema, which Athena lists as a schema", Default: false},
	}
}

func schemaParentName(parent, kind string) []dbmeta.Param {
	return []dbmeta.Param{
		{Name: "schema", Desc: "Glue database name pattern, empty for every database", Default: ""},
		{Name: "parent", Desc: parent + " name pattern, empty for every " + parent, Default: ""},
		{Name: "name", Desc: kind + " name pattern, empty for every " + kind, Default: ""},
		{Name: "with_system", Desc: "include information_schema, which Athena lists as a schema", Default: false},
	}
}
