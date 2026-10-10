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
// # The driver writes the values, and this model does not let it
//
// The driver that dburl v0.49.0 names for Athena is github.com/uber/athenadriver.
// It binds a parameter by writing the value into the text of the statement, the
// way a MySQL driver does, with a backslash before a quote and before a backslash.
// Athena reads a backslash as itself, so a value with a quote or a backslash
// becomes a different value or a syntax error. So this model sets
// [dbmeta.Info.Literal] and renders each filter as a Trino literal. A statement
// then carries no parameter, and the driver binds nothing. See D222.
//
// The same driver refuses a NULL in a result unless the connection has the
// option missingAsNil=true, and it reads a cell it cannot convert as an error.
// A caller that opens the driver must set the option, or a column that is NULL
// stops the query. The tests set it.
//
// # The version
//
// Athena is a service, and the engine version belongs to the workgroup, which
// only the Athena API reports. No SQL statement reads it, and SELECT version()
// is refused with FUNCTION_NOT_FOUND. The model declares no version query, so the
// version is unknown. See D222.
package athena

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/xo/dbmeta"
)

func init() {
	dbmeta.RegisterDialect(dbmeta.Athena, &dbmeta.Info{
		Syntax: dbmeta.Syntax{BlockComments: true},
		// usql strips the semicolon at the end of a statement for Athena, and
		// Athena refuses it.
		Terminator: dbmeta.TerminatorStripped,
		Fold:       dbmeta.FoldLower,
		// Placeholder is never called, because the dialect writes literals.
		Placeholder: func(int) string {
			panic("athena: Placeholder is never used, because the driver cannot bind safely. See Info.Literal")
		},
		Literal: literal,
	})
	registerRelations()
	registerExtra()
}

// literal renders one filter value as a Trino literal.
//
// Trino reads a doubled quote as one quote and reads a backslash as itself, so
// a string needs one change. The driver of Uber writes a backslash before each
// quote, which Athena does not read as an escape, and that is why the model
// does not let the driver bind. See the package comment.
//
// It refuses a type that no query declares, and a string with a NUL.
func literal(v any) (string, error) {
	switch t := v.(type) {
	case string:
		if strings.ContainsRune(t, 0) {
			return "", fmt.Errorf("athena: %w: a filter cannot hold a NUL", dbmeta.ErrInvalidParam)
		}
		return "'" + strings.ReplaceAll(t, "'", "''") + "'", nil
	case bool:
		if t {
			return "true", nil
		}
		return "false", nil
	case int:
		return strconv.Itoa(t), nil
	case int64:
		return strconv.FormatInt(t, 10), nil
	}
	return "", fmt.Errorf("athena: %w: cannot render %T as a literal", dbmeta.ErrInvalidParam, v)
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
