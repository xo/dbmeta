package tidb

import (
	"database/sql"

	"github.com/xo/dbmeta"
)

// tidbSystemSchemas are the schemas TiDB keeps for itself, spelled as TiDB
// spells them. The mysql model holds the same list for the statements it
// shares.
const tidbSystemSchemas = `'mysql', 'INFORMATION_SCHEMA', 'PERFORMANCE_SCHEMA', 'METRICS_SCHEMA', 'sys'`

func always(query string) dbmeta.Choice { return dbmeta.Choice{{Query: query}} }

// v85 is TiDB 8.5, the first release measured to fill
// information_schema.TABLE_PRIVILEGES. 7.5.8 and 8.1.2 keep a grant in
// mysql.tables_priv and list nothing in the view, measured on 2026-09-29.
var v85 = dbmeta.V(8, 5)

// from85 is a fragment that TiDB 8.5 and later take.
func from85(query string) dbmeta.Choice {
	return dbmeta.Choice{{Key: Release, Min: v85, Query: query}}
}

func schemaNameSystem(kind string) []dbmeta.Param {
	return []dbmeta.Param{
		{Name: "schema", Desc: "schema name pattern, empty for every schema", Default: ""},
		{Name: "name", Desc: kind + " name pattern, empty for every " + kind, Default: ""},
		{Name: "with_system", Desc: "include the schemas TiDB keeps for itself", Default: false},
	}
}

// registerOwn registers the statements that read TiDB's own catalog, where
// the mysql model's statement reads a table TiDB does not have.
func registerOwn() {
	// information_schema.variables_info lists every system variable with
	// its scope, which the mysql model reads from performance_schema, a
	// table TiDB does not have. A variable TiDB accepts and ignores is
	// marked IS_NOOP, and is listed like any other.
	dbmeta.Settings.Register(dbmeta.TiDB, &dbmeta.Binding[dbmeta.Setting]{
		Stmt: dbmeta.Stmt{
			always(`SELECT v.variable_name AS "name"`),
			always(`, v.current_value AS "value"`),
			always(`, NULL AS "type"`),
			always(`, LOWER(v.variable_scope) AS "context"`),
			always(`, NULL AS "access"`),
			always(`FROM information_schema.variables_info v`),
			always(`WHERE (@name = '' OR v.variable_name LIKE @name)`),
			always(`ORDER BY 1`),
		},
		Fields: []dbmeta.Field{
			{Name: "name"},
			{Name: "value", Desc: "the value the server has now"},
			{Name: "type", Desc: "always absent: TiDB records no type for a variable"},
			{Name: "context", Desc: "where the variable is set, such as session,global, global, instance or none"},
			{Name: "access", Desc: "always absent: a variable is not granted"},
		},
		Params: []dbmeta.Param{{Name: "name", Desc: "variable name pattern, empty for every variable", Default: ""}},
		Scan: func(rows *sql.Rows) (dbmeta.Setting, error) {
			var v dbmeta.Setting
			err := rows.Scan(&v.Name, &v.Value, &v.Type, &v.Context, &v.Access)
			return v, err
		},
	})

	// The mysql model's statement, from 8.5, where the view has rows. Before
	// it the query is too old rather than answering no rows, because the
	// grants are there and the view does not list them.
	dbmeta.Privileges.Register(dbmeta.TiDB, &dbmeta.Binding[dbmeta.Privilege]{
		Stmt: dbmeta.Stmt{
			from85(`SELECT p.table_schema AS "schema"`),
			from85(`, p.table_name AS "name"`),
			from85(`, 'table' AS "type"`),
			from85(`, CONCAT(p.grantee, '=', p.privilege_type) AS "access"`),
			from85(`, NULL AS "column_access"`),
			from85(`, NULL AS "policies"`),
			from85(`FROM information_schema.TABLE_PRIVILEGES p`),
			from85(`WHERE (@with_system OR p.table_schema NOT IN (` + tidbSystemSchemas + `))`),
			from85(`AND (@schema = '' OR p.table_schema LIKE @schema)`),
			from85(`AND (@name = '' OR p.table_name LIKE @name)`),
			from85(`ORDER BY 1, 2`),
		},
		Fields: []dbmeta.Field{
			{Name: "schema"}, {Name: "name"}, {Name: "type"},
			{Name: "access", Desc: "one grant per row, not the gathered list psql prints"},
			{Name: "column_access", Desc: "always absent: read column_privileges instead"},
			{Name: "policies", Desc: "always absent: TiDB has no row level security"},
		},
		Params: schemaNameSystem("table"),
		Scan: func(rows *sql.Rows) (dbmeta.Privilege, error) {
			var v dbmeta.Privilege
			err := rows.Scan(&v.Schema, &v.Name, &v.Type, &v.Access, &v.ColumnAccess, &v.Policies)
			return v, err
		},
	})

	// A TiDB sequence is always a bigint. The mysql model reads sequences
	// only on MariaDB, whose view has other columns. TiDB writes an empty
	// comment where there is none.
	dbmeta.Sequences.Register(dbmeta.TiDB, &dbmeta.Binding[dbmeta.Sequence]{
		Stmt: dbmeta.Stmt{
			always(`SELECT s.sequence_schema AS "schema"`),
			always(`, s.sequence_name AS "name"`),
			always(`, 'bigint' AS "data_type"`),
			always(`, CAST(s.start AS CHAR) AS "start"`),
			always(`, CAST(s.min_value AS CHAR) AS "minimum"`),
			always(`, CAST(s.max_value AS CHAR) AS "maximum"`),
			always(`, CAST(s.increment AS CHAR) AS "increment"`),
			always(`, s.cycle = 1 AS "cycles"`),
			always(`, '' AS "owned_by"`),
			always(`, NULLIF(s.comment, '') AS "comment"`),
			always(`FROM information_schema.sequences s`),
			always(`WHERE (@with_system OR s.sequence_schema NOT IN (` + tidbSystemSchemas + `))`),
			always(`AND (@schema = '' OR s.sequence_schema LIKE @schema)`),
			always(`AND (@name = '' OR s.sequence_name LIKE @name)`),
			always(`ORDER BY 1, 2`),
		},
		Fields: []dbmeta.Field{
			{Name: "schema"}, {Name: "name"},
			{Name: "data_type", Desc: "always bigint: a TiDB sequence has no other type"},
			{Name: "start"}, {Name: "minimum"}, {Name: "maximum"}, {Name: "increment"}, {Name: "cycles"},
			{Name: "owned_by", Desc: "always empty: a TiDB sequence belongs to no column"},
			{Name: "comment"},
		},
		Params: schemaNameSystem("sequence"),
		Scan: func(rows *sql.Rows) (dbmeta.Sequence, error) {
			var v dbmeta.Sequence
			err := rows.Scan(&v.Schema, &v.Name, &v.DataType, &v.Start, &v.Minimum,
				&v.Maximum, &v.Increment, &v.Cycles, &v.OwnedBy, &v.Comment)
			return v, err
		},
	})
}
