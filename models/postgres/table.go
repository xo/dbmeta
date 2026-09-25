package postgres

import (
	"database/sql"

	"github.com/xo/dbmeta"
)

// The detail that \d name prints about one table. psql assembles all of it in
// describeOneTableDetails, which is the hardest entry point in describe.c and
// carries 21 of its 68 version gates. dbmeta splits it into the objects it
// describes, because a caller usually wants one of them.

func init() {
	registerIndexes()
	registerIndexColumns()
	registerConstraints()
	registerTriggers()
	registerSequences()
	registerPartitionedTables()
}

// registerIndexes backs \di and the index footer of \d name.
func registerIndexes() {
	dbmeta.Indexes.Register(dbmeta.PostgreSQL, &dbmeta.Binding[dbmeta.Index]{
		Stmt: dbmeta.Stmt{
			{{SQL: `SELECT current_database() AS "catalog"`}},
			{{SQL: `, n.nspname AS "schema"`}},
			{{SQL: `, t.relname AS "table"`}},
			{{SQL: `, c.relname AS "name"`}},
			{{SQL: `, am.amname AS "type"`}},
			{{SQL: `, i.indisunique AS "unique"`}},
			{{SQL: `, i.indisprimary AS "primary"`}},
			{{SQL: `, pg_catalog.obj_description(c.oid, 'pg_class') AS "comment"`}},
			{{SQL: `FROM pg_catalog.pg_index i`}},
			{{SQL: `JOIN pg_catalog.pg_class c ON c.oid = i.indexrelid`}},
			{{SQL: `JOIN pg_catalog.pg_class t ON t.oid = i.indrelid`}},
			{{SQL: `JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace`}},
			{{SQL: `JOIN pg_catalog.pg_am am ON am.oid = c.relam`}},
			{{SQL: `WHERE (@with_system OR (n.nspname !~ '^pg_' AND n.nspname <> 'information_schema'))`}},
			{{SQL: `AND (@schema = '' OR n.nspname LIKE @schema)`}},
			{{SQL: `AND (@parent = '' OR t.relname LIKE @parent)`}},
			{{SQL: `AND (@name = '' OR c.relname LIKE @name)`}},
			{{SQL: `ORDER BY 2, 3, 4`}},
		},
		Fields: fields("catalog", "schema", "table", "name", "type", "unique", "primary", "comment"),
		Params: schemaParentName("index"),
		Scan: func(rows *sql.Rows) (dbmeta.Index, error) {
			var v dbmeta.Index
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Table, &v.Name, &v.Type,
				&v.Unique, &v.Primary, &v.Comment)
			return v, err
		},
	})
}

// registerIndexColumns lists the columns of each index in index order, which
// psql prints as part of the index definition rather than as a table.
func registerIndexColumns() {
	dbmeta.IndexColumns.Register(dbmeta.PostgreSQL, &dbmeta.Binding[dbmeta.IndexColumn]{
		Stmt: dbmeta.Stmt{
			{{SQL: `SELECT n.nspname AS "schema"`}},
			{{SQL: `, t.relname AS "table"`}},
			{{SQL: `, c.relname AS "index"`}},
			{{SQL: `, a.attname AS "name"`}},
			{{SQL: `, k.ordinality AS "ordinal"`}},
			{{SQL: `, pg_catalog.pg_get_indexdef(i.indexrelid, k.ordinality::int, true) AS "expression"`}},
			{{SQL: `, pg_catalog.pg_index_column_has_property(i.indexrelid, k.ordinality::int, 'desc') AS "descending"`}},
			{{SQL: `FROM pg_catalog.pg_index i`}},
			{{SQL: `JOIN pg_catalog.pg_class c ON c.oid = i.indexrelid`}},
			{{SQL: `JOIN pg_catalog.pg_class t ON t.oid = i.indrelid`}},
			{{SQL: `JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace`}},
			{{SQL: `CROSS JOIN LATERAL pg_catalog.unnest(i.indkey) WITH ORDINALITY AS k(attnum, ordinality)`}},
			{{SQL: `LEFT JOIN pg_catalog.pg_attribute a ON a.attrelid = i.indrelid AND a.attnum = k.attnum`}},
			{{SQL: `WHERE (@with_system OR (n.nspname !~ '^pg_' AND n.nspname <> 'information_schema'))`}},
			{{SQL: `AND (@schema = '' OR n.nspname LIKE @schema)`}},
			{{SQL: `AND (@parent = '' OR t.relname LIKE @parent)`}},
			{{SQL: `AND (@name = '' OR c.relname LIKE @name)`}},
			{{SQL: `ORDER BY 1, 2, 3, 5`}},
		},
		Fields: fields("schema", "table", "index", "name", "ordinal", "expression", "descending"),
		Params: schemaParentName("index"),
		Scan: func(rows *sql.Rows) (dbmeta.IndexColumn, error) {
			var v dbmeta.IndexColumn
			err := rows.Scan(&v.Schema, &v.Table, &v.Index, &v.Name, &v.Ordinal,
				&v.Expression, &v.Descending)
			return v, err
		},
	})
}

// registerConstraints backs the constraint footers of \d name.
func registerConstraints() {
	dbmeta.Constraints.Register(dbmeta.PostgreSQL, &dbmeta.Binding[dbmeta.Constraint]{
		Stmt: dbmeta.Stmt{
			{{SQL: `SELECT n.nspname AS "schema"`}},
			{{SQL: `, t.relname AS "table"`}},
			{{SQL: `, r.conname AS "name"`}},
			{{SQL: `, CASE r.contype WHEN 'c' THEN 'check' WHEN 'f' THEN 'foreign key'` +
				` WHEN 'p' THEN 'primary key' WHEN 'u' THEN 'unique' WHEN 't' THEN 'trigger'` +
				` WHEN 'x' THEN 'exclusion' ELSE r.contype::text END AS "type"`}},
			{{SQL: `, pg_catalog.pg_get_constraintdef(r.oid, true) AS "definition"`}},
			{{SQL: `, r.condeferrable AS "deferrable"`}},
			{{SQL: `, r.condeferred AS "deferred"`}},
			{{SQL: `, pg_catalog.obj_description(r.oid, 'pg_constraint') AS "comment"`}},
			{{SQL: `FROM pg_catalog.pg_constraint r`}},
			{{SQL: `JOIN pg_catalog.pg_class t ON t.oid = r.conrelid`}},
			{{SQL: `JOIN pg_catalog.pg_namespace n ON n.oid = t.relnamespace`}},
			{{SQL: `WHERE r.conrelid <> 0`}},
			// contype n is a NOT NULL constraint, which release 18 records
			// here and every earlier release records only on the column. It
			// is left out on every release, so that the same schema answers
			// the same way whatever the server is. Column.Nullable is where a
			// caller reads it, and it is filled everywhere. See D49.
			{{SQL: `AND r.contype <> 'n'`}},
			{{SQL: `AND (@with_system OR (n.nspname !~ '^pg_' AND n.nspname <> 'information_schema'))`}},
			{{SQL: `AND (@schema = '' OR n.nspname LIKE @schema)`}},
			{{SQL: `AND (@parent = '' OR t.relname LIKE @parent)`}},
			{{SQL: `AND (@name = '' OR r.conname LIKE @name)`}},
			{{SQL: `ORDER BY 1, 2, 3`}},
		},
		Fields: fields("schema", "table", "name", "type", "definition", "deferrable", "deferred", "comment"),
		Params: schemaParentName("constraint"),
		Scan: func(rows *sql.Rows) (dbmeta.Constraint, error) {
			var v dbmeta.Constraint
			err := rows.Scan(&v.Schema, &v.Table, &v.Name, &v.Type, &v.Definition,
				&v.Deferrable, &v.Deferred, &v.Comment)
			return v, err
		},
	})
}

// registerTriggers backs the trigger footer of \d name.
func registerTriggers() {
	dbmeta.Triggers.Register(dbmeta.PostgreSQL, &dbmeta.Binding[dbmeta.Trigger]{
		Stmt: dbmeta.Stmt{
			{{SQL: `SELECT n.nspname AS "schema"`}},
			{{SQL: `, c.relname AS "table"`}},
			{{SQL: `, t.tgname AS "name"`}},
			{{SQL: `, CASE t.tgenabled WHEN 'O' THEN 'enabled' WHEN 'D' THEN 'disabled'` +
				` WHEN 'R' THEN 'replica' WHEN 'A' THEN 'always' ELSE '' END AS "enabled"`}},
			{{SQL: `, pg_catalog.pg_get_triggerdef(t.oid, true) AS "definition"`}},
			{{SQL: `, pg_catalog.obj_description(t.oid, 'pg_trigger') AS "comment"`}},
			{{SQL: `FROM pg_catalog.pg_trigger t`}},
			{{SQL: `JOIN pg_catalog.pg_class c ON c.oid = t.tgrelid`}},
			{{SQL: `JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace`}},
			{{SQL: `WHERE NOT t.tgisinternal`}},
			{{SQL: `AND (@with_system OR (n.nspname !~ '^pg_' AND n.nspname <> 'information_schema'))`}},
			{{SQL: `AND (@schema = '' OR n.nspname LIKE @schema)`}},
			{{SQL: `AND (@parent = '' OR c.relname LIKE @parent)`}},
			{{SQL: `AND (@name = '' OR t.tgname LIKE @name)`}},
			{{SQL: `ORDER BY 1, 2, 3`}},
		},
		Fields: fields("schema", "table", "name", "enabled", "definition", "comment"),
		Params: schemaParentName("trigger"),
		Scan: func(rows *sql.Rows) (dbmeta.Trigger, error) {
			var v dbmeta.Trigger
			err := rows.Scan(&v.Schema, &v.Table, &v.Name, &v.Enabled, &v.Definition, &v.Comment)
			return v, err
		},
	})
}

// registerSequences backs \ds and the sequence detail of \d name.
//
// pg_sequence arrived in release 10. Before it, the bounds lived in the
// sequence relation itself and could only be read by selecting from it, which
// a metadata query cannot do for every sequence at once. An older server
// therefore reports the name and owner with zero bounds.
func registerSequences() {
	dbmeta.Sequences.Register(dbmeta.PostgreSQL, &dbmeta.Binding[dbmeta.Sequence]{
		Stmt: dbmeta.Stmt{
			{{SQL: `SELECT n.nspname AS "schema"`}},
			{{SQL: `, c.relname AS "name"`}},
			{
				{SQL: `, NULL AS "data_type"`},
				{Min: v10, SQL: `, pg_catalog.format_type(s.seqtypid, NULL) AS "data_type"`},
			},
			{
				{SQL: `, NULL::bigint AS "start"`},
				{Min: v10, SQL: `, s.seqstart AS "start"`},
			},
			{
				{SQL: `, NULL::bigint AS "minimum"`},
				{Min: v10, SQL: `, s.seqmin AS "minimum"`},
			},
			{
				{SQL: `, NULL::bigint AS "maximum"`},
				{Min: v10, SQL: `, s.seqmax AS "maximum"`},
			},
			{
				{SQL: `, NULL::bigint AS "increment"`},
				{Min: v10, SQL: `, s.seqincrement AS "increment"`},
			},
			{
				{SQL: `, NULL::boolean AS "cycles"`},
				{Min: v10, SQL: `, s.seqcycle AS "cycles"`},
			},
			{{SQL: `, COALESCE((SELECT pg_catalog.quote_ident(dn.nspname) || '.' ||` +
				` pg_catalog.quote_ident(dc.relname) || '.' || da.attname` +
				` FROM pg_catalog.pg_depend d` +
				` JOIN pg_catalog.pg_class dc ON dc.oid = d.refobjid` +
				` JOIN pg_catalog.pg_namespace dn ON dn.oid = dc.relnamespace` +
				` JOIN pg_catalog.pg_attribute da ON da.attrelid = d.refobjid AND da.attnum = d.refobjsubid` +
				` WHERE d.objid = c.oid AND d.deptype IN ('a', 'i') LIMIT 1), '') AS "owned_by"`}},
			{{SQL: `, pg_catalog.obj_description(c.oid, 'pg_class') AS "comment"`}},
			{{SQL: `FROM pg_catalog.pg_class c`}},
			{{SQL: `JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace`}},
			{
				{SQL: ``},
				{Min: v10, SQL: `LEFT JOIN pg_catalog.pg_sequence s ON s.seqrelid = c.oid`},
			},
			{{SQL: `WHERE c.relkind = 'S'`}},
			{{SQL: `AND (@with_system OR (n.nspname !~ '^pg_' AND n.nspname <> 'information_schema'))`}},
			{{SQL: `AND (@schema = '' OR n.nspname LIKE @schema)`}},
			{{SQL: `AND (@name = '' OR c.relname LIKE @name)`}},
			{{SQL: `ORDER BY 1, 2`}},
		},
		Fields: []dbmeta.Field{
			{Name: "schema"}, {Name: "name"},
			{Name: "data_type", Min: v10}, {Name: "start", Min: v10},
			{Name: "minimum", Min: v10}, {Name: "maximum", Min: v10},
			{Name: "increment", Min: v10}, {Name: "cycles", Min: v10},
			{Name: "owned_by"}, {Name: "comment"},
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

// registerPartitionedTables backs \dP, from listPartitionedTables.
//
// Declarative partitioning arrived in release 10, so this reports the version
// as too old below it rather than an empty result.
func registerPartitionedTables() {
	dbmeta.PartitionedTables.Register(dbmeta.PostgreSQL, &dbmeta.Binding[dbmeta.PartitionedTable]{
		Stmt: dbmeta.Stmt{
			{{Min: v10, SQL: `SELECT n.nspname AS "schema"`}},
			{{Min: v10, SQL: `, c.relname AS "name"`}},
			{{Min: v10, SQL: `, pg_catalog.pg_get_userbyid(c.relowner) AS "owner"`}},
			{{Min: v10, SQL: `, CASE c.relkind WHEN 'p' THEN 'table' WHEN 'I' THEN 'index'` +
				` ELSE c.relkind::text END AS "type"`}},
			{{Min: v10, SQL: `, COALESCE((SELECT pn.nspname || '.' || pc.relname` +
				` FROM pg_catalog.pg_inherits h` +
				` JOIN pg_catalog.pg_class pc ON pc.oid = h.inhparent` +
				` JOIN pg_catalog.pg_namespace pn ON pn.oid = pc.relnamespace` +
				` WHERE h.inhrelid = c.oid), '') AS "parent"`}},
			{{Min: v10, SQL: `, SUBSTRING(pg_catalog.pg_get_partkeydef(c.oid) FROM '^[A-Za-z]+') AS "strategy"`}},
			{{Min: v10, SQL: `, pg_catalog.pg_get_partkeydef(c.oid) AS "expression"`}},
			{{Min: v10, SQL: `, pg_catalog.obj_description(c.oid, 'pg_class') AS "comment"`}},
			{{Min: v10, SQL: `FROM pg_catalog.pg_class c`}},
			{{Min: v10, SQL: `JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace`}},
			{{Min: v10, SQL: `WHERE c.relkind IN ('p', 'I')`}},
			{{Min: v10, SQL: `AND (@with_system OR (n.nspname !~ '^pg_' AND n.nspname <> 'information_schema'))`}},
			{{Min: v10, SQL: `AND (@schema = '' OR n.nspname LIKE @schema)`}},
			{{Min: v10, SQL: `AND (@name = '' OR c.relname LIKE @name)`}},
			{{Min: v10, SQL: `ORDER BY 1, 2`}},
		},
		Fields: []dbmeta.Field{
			{Name: "schema", Min: v10}, {Name: "name", Min: v10}, {Name: "owner", Min: v10},
			{Name: "type", Min: v10}, {Name: "parent", Min: v10}, {Name: "strategy", Min: v10},
			{Name: "expression", Min: v10}, {Name: "comment", Min: v10},
		},
		Params: schemaNameSystem("partitioned table"),
		Scan: func(rows *sql.Rows) (dbmeta.PartitionedTable, error) {
			var v dbmeta.PartitionedTable
			err := rows.Scan(&v.Schema, &v.Name, &v.Owner, &v.Type, &v.Parent,
				&v.Strategy, &v.Expression, &v.Comment)
			return v, err
		},
	})
}
