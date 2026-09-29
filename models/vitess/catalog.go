package vitess

import (
	"database/sql"

	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/models/mysql"
)

// vitessSystemSchemas are the schemas Vitess keeps for itself: MySQL's, and
// _vt, where a tablet keeps its own state. The mysql model holds the same
// list for the statements it shares.
const vitessSystemSchemas = `'mysql', 'information_schema', 'performance_schema', 'sys', '_vt'`

// sequenceComment is the comment that makes a table a sequence. vttablet
// serves a table with this comment as a sequence, and the table holds the
// next value and the size of the block a vtgate takes at a time.
const sequenceComment = `vitess_sequence`

func always(query string) dbmeta.Choice { return dbmeta.Choice{{Query: query}} }

// registerOwn registers the statements that read what only Vitess has.
func registerOwn() {
	// A Vitess sequence is a table with the comment vitess_sequence and the
	// columns id, next_id and cache, so one statement over
	// information_schema lists them, which vtgate passes. The table holds the
	// next value and not the first one, so the start is not known. NEXT VALUE
	// gave 1100, 1101 and 1102 on 24.0.3, on 2026-09-30, so the increment is
	// 1. Nothing in information_schema says which column a sequence fills,
	// because that is in the VSchema.
	dbmeta.Sequences.Register(dbmeta.Vitess, &dbmeta.Binding[dbmeta.Sequence]{
		Stmt: dbmeta.Stmt{
			always(`SELECT ` + mysql.Keyspace("t.table_schema") + ` AS "schema"`),
			always(`, t.table_name AS "name"`),
			always(`, c.data_type AS "data_type"`),
			always(`, NULL AS "start"`),
			always(`, NULL AS "minimum"`),
			always(`, NULL AS "maximum"`),
			always(`, '1' AS "increment"`),
			always(`, NULL AS "cycles"`),
			always(`, '' AS "owned_by"`),
			always(`, t.table_comment AS "comment"`),
			always(`FROM information_schema.tables t`),
			always(`LEFT JOIN information_schema.columns c ON c.table_schema = t.table_schema`),
			always(`AND c.table_name = t.table_name AND c.column_name = 'next_id'`),
			always(`WHERE t.table_comment = '` + sequenceComment + `'`),
			always(`AND (@with_system OR t.table_schema NOT IN (` + vitessSystemSchemas + `))`),
			always(`AND (@schema = '' OR ` + mysql.Keyspace("t.table_schema") + ` LIKE @schema)`),
			always(`AND (@name = '' OR t.table_name LIKE @name)`),
			always(`ORDER BY 1, 2`),
		},
		Fields: []dbmeta.Field{
			{Name: "schema"}, {Name: "name"},
			{Name: "data_type", Desc: "the type of the column next_id, and NULL when the table has no such column"},
			{Name: "start", Desc: "always NULL: the table holds the next value and not the first"},
			{Name: "minimum", Desc: "always NULL: a Vitess sequence records no bound"},
			{Name: "maximum", Desc: "always NULL: a Vitess sequence records no bound"},
			{Name: "increment", Desc: "always 1: a Vitess sequence counts by one"},
			{Name: "cycles", Desc: "always NULL: nothing records whether a Vitess sequence wraps"},
			{Name: "owned_by", Desc: "always empty: the VSchema names the column a sequence fills"},
			{Name: "comment", Desc: "the table's comment, which is vitess_sequence on every sequence"},
		},
		Params: []dbmeta.Param{
			{Name: "schema", Desc: "keyspace name pattern, empty for every keyspace", Default: ""},
			{Name: "name", Desc: "sequence name pattern, empty for every sequence", Default: ""},
			{Name: "with_system", Desc: "include the schemas Vitess keeps for itself", Default: false},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Sequence, error) {
			var v dbmeta.Sequence
			err := rows.Scan(&v.Schema, &v.Name, &v.DataType, &v.Start, &v.Minimum,
				&v.Maximum, &v.Increment, &v.Cycles, &v.OwnedBy, &v.Comment)
			return v, err
		},
	})
}
