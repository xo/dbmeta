package mysql

import (
	"database/sql"

	"github.com/xo/dbmeta"
)

// Relations and the detail of one: schemas, tables, columns, indexes,
// constraints, triggers, sequences and partitions.

func registerRelations() {
	// Schemas and databases are the same object in MariaDB. Both queries read
	// SCHEMATA, and a caller asking for either gets the same rows under a
	// different shape. Reporting one as unsupported would be wrong, because
	// the database does have the concept, it simply does not separate them.
	dbmeta.Schemas.Register(dbmeta.MySQL, &dbmeta.Binding[dbmeta.Schema]{
		Stmt: dbmeta.Stmt{
			{{SQL: `SELECT s.catalog_name AS "catalog"`}},
			{{SQL: `, s.schema_name AS "name"`}},
			// MariaDB records no owner for a schema
			{{SQL: `, '' AS "owner"`}},
			{{SQL: `, NULL AS "comment"`}},
			{{SQL: `FROM information_schema.SCHEMATA s`}},
			{{SQL: `WHERE (@with_system OR s.schema_name NOT IN (` + systemSchemas + `))`}},
			{{SQL: `AND (@name = '' OR s.schema_name LIKE @name)`}},
			{{SQL: `ORDER BY 2`}},
		},
		Fields: []dbmeta.Field{
			{Name: "catalog"}, {Name: "name"},
			{Name: "owner", Desc: "always empty: MariaDB records no owner for a schema"},
			{Name: "comment", Desc: "always absent: MariaDB has no comment on a schema"},
		},
		Params: nameSystem("schema"),
		Scan: func(rows *sql.Rows) (dbmeta.Schema, error) {
			var v dbmeta.Schema
			err := rows.Scan(&v.Catalog, &v.Name, &v.Owner, &v.Comment)
			return v, err
		},
	})

	dbmeta.Databases.Register(dbmeta.MySQL, &dbmeta.Binding[dbmeta.Database]{
		Stmt: dbmeta.Stmt{
			{{SQL: `SELECT s.schema_name AS "name"`}},
			{{SQL: `, '' AS "owner"`}},
			{{SQL: `, s.default_character_set_name AS "encoding"`}},
			{{SQL: `, s.default_collation_name AS "collate"`}},
			{{SQL: `, s.default_collation_name AS "ctype"`}},
			{{SQL: `, NULL AS "access"`}},
			{{SQL: `, NULL AS "tablespace"`}},
			{{SQL: `, '' AS "size"`}},
			{{SQL: `, NULL AS "comment"`}},
			{{SQL: `FROM information_schema.SCHEMATA s`}},
			{{SQL: `WHERE (@with_system OR s.schema_name NOT IN (` + systemSchemas + `))`}},
			{{SQL: `AND (@name = '' OR s.schema_name LIKE @name)`}},
			{{SQL: `ORDER BY 1`}},
		},
		Fields: fields("name", "owner", "encoding", "collate", "ctype",
			"access", "tablespace", "size", "comment"),
		Params: nameSystem("database"),
		Scan: func(rows *sql.Rows) (dbmeta.Database, error) {
			var v dbmeta.Database
			err := rows.Scan(&v.Name, &v.Owner, &v.Encoding, &v.Collate, &v.CType,
				&v.Access, &v.Tablespace, &v.Size, &v.Comment)
			return v, err
		},
	})

	// A comment lives on the object here, in TABLE_COMMENT, rather than in a
	// catalog of comments. MariaDB writes an empty string for no comment, not
	// NULL, so the query turns that back into NULL: an absent comment and an
	// empty one are different answers and docs/NULLS.md forbids collapsing them.
	dbmeta.Tables.Register(dbmeta.MySQL, &dbmeta.Binding[dbmeta.Table]{
		Stmt: dbmeta.Stmt{
			{{SQL: `SELECT t.table_catalog AS "catalog"`}},
			{{SQL: `, t.table_schema AS "schema"`}},
			{{SQL: `, t.table_name AS "name"`}},
			{{SQL: `, CASE t.table_type WHEN 'BASE TABLE' THEN 'table'` +
				` WHEN 'VIEW' THEN 'view'` +
				` WHEN 'SEQUENCE' THEN 'sequence'` +
				` WHEN 'SYSTEM VIEW' THEN 'view'` +
				` ELSE LOWER(t.table_type) END AS "type"`}},
			{{SQL: `, NULLIF(t.table_comment, '') AS "comment"`}},
			{{SQL: `FROM information_schema.TABLES t`}},
			{{SQL: `WHERE (@with_system OR t.table_schema NOT IN (` + systemSchemas + `))`}},
			{{SQL: `AND (@schema = '' OR t.table_schema LIKE @schema)`}},
			{{SQL: `AND (@name = '' OR t.table_name LIKE @name)`}},
			{{SQL: `ORDER BY 2, 3`}},
		},
		Fields: fields("catalog", "schema", "name", "type", "comment"),
		Params: schemaNameSystem("table"),
		Scan: func(rows *sql.Rows) (dbmeta.Table, error) {
			var v dbmeta.Table
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Name, &v.Type, &v.Comment)
			return v, err
		},
	})

	// column_type carries the declared type, such as varchar(255), where
	// data_type carries only varchar. psql prints the declared type, so that
	// is what this reports.
	dbmeta.Columns.Register(dbmeta.MySQL, &dbmeta.Binding[dbmeta.Column]{
		Stmt: dbmeta.Stmt{
			{{SQL: `SELECT c.table_catalog AS "catalog"`}},
			{{SQL: `, c.table_schema AS "schema"`}},
			{{SQL: `, c.table_name AS "table"`}},
			{{SQL: `, c.column_name AS "name"`}},
			{{SQL: `, c.ordinal_position AS "ordinal"`}},
			{{SQL: `, c.column_type AS "data_type"`}},
			{{SQL: `, c.is_nullable = 'YES' AS "nullable"`}},
			{{SQL: `, c.column_default AS "default"`}},
			// Free: information_schema.COLUMNS already says which columns are
			// in the primary key, so this costs nothing. See D47.
			{{SQL: `, c.column_key = 'PRI' AS "primary_key"`}},
			// auto_increment is the closest thing to an identity column
			{{SQL: `, CASE WHEN c.extra LIKE '%auto_increment%' THEN 'a' ELSE NULL END AS "identity"`}},
			{{SQL: `, CASE WHEN c.extra LIKE '%GENERATED%' THEN 's' ELSE NULL END AS "generated"`}},
			{{SQL: `, NULLIF(c.column_comment, '') AS "comment"`}},
			{{SQL: `FROM information_schema.COLUMNS c`}},
			{{SQL: `WHERE (@with_system OR c.table_schema NOT IN (` + systemSchemas + `))`}},
			{{SQL: `AND (@schema = '' OR c.table_schema LIKE @schema)`}},
			{{SQL: `AND (@parent = '' OR c.table_name LIKE @parent)`}},
			{{SQL: `AND (@name = '' OR c.column_name LIKE @name)`}},
			{{SQL: `ORDER BY 2, 3, 5`}},
		},
		Fields: fields("catalog", "schema", "table", "name", "ordinal",
			"data_type", "nullable", "default", "primary_key", "identity", "generated", "comment"),
		Params: schemaParentName("column"),
		Scan: func(rows *sql.Rows) (dbmeta.Column, error) {
			var v dbmeta.Column
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Table, &v.Name, &v.Ordinal,
				&v.DataType, &v.Nullable, &v.Default, &v.PrimaryKey, &v.Identity, &v.Generated,
				&v.Comment)
			return v, err
		},
	})

	// STATISTICS has one row per index column, so an index listing groups it.
	dbmeta.Indexes.Register(dbmeta.MySQL, &dbmeta.Binding[dbmeta.Index]{
		Stmt: dbmeta.Stmt{
			{{SQL: `SELECT s.table_catalog AS "catalog"`}},
			{{SQL: `, s.table_schema AS "schema"`}},
			{{SQL: `, s.table_name AS "table"`}},
			{{SQL: `, s.index_name AS "name"`}},
			{{SQL: `, MIN(s.index_type) AS "type"`}},
			{{SQL: `, MIN(s.non_unique) = 0 AS "unique"`}},
			{{SQL: `, MIN(s.index_name) = 'PRIMARY' AS "primary"`}},
			{{SQL: `, NULLIF(MIN(s.index_comment), '') AS "comment"`}},
			{{SQL: `FROM information_schema.STATISTICS s`}},
			{{SQL: `WHERE (@with_system OR s.table_schema NOT IN (` + systemSchemas + `))`}},
			{{SQL: `AND (@schema = '' OR s.table_schema LIKE @schema)`}},
			{{SQL: `AND (@parent = '' OR s.table_name LIKE @parent)`}},
			{{SQL: `AND (@name = '' OR s.index_name LIKE @name)`}},
			{{SQL: `GROUP BY 1, 2, 3, 4`}},
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

	dbmeta.IndexColumns.Register(dbmeta.MySQL, &dbmeta.Binding[dbmeta.IndexColumn]{
		Stmt: dbmeta.Stmt{
			{{SQL: `SELECT s.table_schema AS "schema"`}},
			{{SQL: `, s.table_name AS "table"`}},
			{{SQL: `, s.index_name AS "index"`}},
			{{SQL: `, s.column_name AS "name"`}},
			{{SQL: `, s.seq_in_index AS "ordinal"`}},
			// MariaDB's STATISTICS has no expression column: a functional
			// index records only the column it was built from
			{{SQL: `, NULL AS "expression"`}},
			{{SQL: `, s.collation = 'D' AS "descending"`}},
			{{SQL: `FROM information_schema.STATISTICS s`}},
			{{SQL: `WHERE (@with_system OR s.table_schema NOT IN (` + systemSchemas + `))`}},
			{{SQL: `AND (@schema = '' OR s.table_schema LIKE @schema)`}},
			{{SQL: `AND (@parent = '' OR s.table_name LIKE @parent)`}},
			{{SQL: `AND (@name = '' OR s.index_name LIKE @name)`}},
			{{SQL: `ORDER BY 1, 2, 3, 5`}},
		},
		Fields: []dbmeta.Field{
			{Name: "schema"}, {Name: "table"}, {Name: "index"}, {Name: "name"},
			{Name: "ordinal"},
			{Name: "expression", Desc: "always absent: MariaDB records no index expression"},
			{Name: "descending"},
		},
		Params: schemaParentName("index"),
		Scan: func(rows *sql.Rows) (dbmeta.IndexColumn, error) {
			var v dbmeta.IndexColumn
			err := rows.Scan(&v.Schema, &v.Table, &v.Index, &v.Name, &v.Ordinal,
				&v.Expression, &v.Descending)
			return v, err
		},
	})

	// CHECK_CONSTRAINTS arrived in MariaDB 10.2 and in MySQL 8.0.16. Neither
	// number tells you anything about the other product, so each one is a
	// fragment gating on its own key. Below both the check clause has no
	// source, so it is padded with NULL rather than an empty string.
	dbmeta.Constraints.Register(dbmeta.MySQL, &dbmeta.Binding[dbmeta.Constraint]{
		Stmt: dbmeta.Stmt{
			{{SQL: `SELECT t.table_schema AS "schema"`}},
			{{SQL: `, t.table_name AS "table"`}},
			{{SQL: `, t.constraint_name AS "name"`}},
			{{SQL: `, LOWER(t.constraint_type) AS "type"`}},
			{
				{SQL: `, NULL AS "definition"`},
				frag(mariaCheck, `, k.check_clause AS "definition"`),
				frag(mysqlCheck, `, k.check_clause AS "definition"`),
			},
			// MariaDB has no deferred constraints at all
			{{SQL: `, FALSE AS "deferrable"`}},
			{{SQL: `, FALSE AS "deferred"`}},
			{{SQL: `, NULL AS "comment"`}},
			{{SQL: `FROM information_schema.TABLE_CONSTRAINTS t`}},
			{
				{SQL: ``},
				frag(mariaCheck, mariaCheckJoin),
				frag(mysqlCheck, mysqlCheckJoin),
			},
			{{SQL: `WHERE (@with_system OR t.table_schema NOT IN (` + systemSchemas + `))`}},
			{{SQL: `AND (@schema = '' OR t.table_schema LIKE @schema)`}},
			{{SQL: `AND (@parent = '' OR t.table_name LIKE @parent)`}},
			{{SQL: `AND (@name = '' OR t.constraint_name LIKE @name)`}},
			{{SQL: `ORDER BY 1, 2, 3`}},
		},
		Fields: []dbmeta.Field{
			{Name: "schema"}, {Name: "table"}, {Name: "name"}, {Name: "type"},
			{
				Name: "definition",
				Desc: "the check clause, for a check constraint",
				Min:  mariaCheck.Min, Key: mariaCheck.Key,
				Also: []dbmeta.Gate{mysqlCheck},
			},
			{Name: "deferrable", Desc: "always false: MariaDB has no deferred constraints"},
			{Name: "deferred", Desc: "always false: MariaDB has no deferred constraints"},
			{Name: "comment"},
		},
		Params: schemaParentName("constraint"),
		Scan: func(rows *sql.Rows) (dbmeta.Constraint, error) {
			var v dbmeta.Constraint
			err := rows.Scan(&v.Schema, &v.Table, &v.Name, &v.Type, &v.Definition,
				&v.Deferrable, &v.Deferred, &v.Comment)
			return v, err
		},
	})

	dbmeta.Triggers.Register(dbmeta.MySQL, &dbmeta.Binding[dbmeta.Trigger]{
		Stmt: dbmeta.Stmt{
			{{SQL: `SELECT t.trigger_schema AS "schema"`}},
			{{SQL: `, t.event_object_table AS "table"`}},
			{{SQL: `, t.trigger_name AS "name"`}},
			// a MariaDB trigger cannot be disabled
			{{SQL: `, 'enabled' AS "enabled"`}},
			{{SQL: `, CONCAT(t.action_timing, ' ', t.event_manipulation, ' ', t.action_statement) AS "definition"`}},
			{{SQL: `, NULL AS "comment"`}},
			{{SQL: `FROM information_schema.TRIGGERS t`}},
			{{SQL: `WHERE (@with_system OR t.trigger_schema NOT IN (` + systemSchemas + `))`}},
			{{SQL: `AND (@schema = '' OR t.trigger_schema LIKE @schema)`}},
			{{SQL: `AND (@parent = '' OR t.event_object_table LIKE @parent)`}},
			{{SQL: `AND (@name = '' OR t.trigger_name LIKE @name)`}},
			{{SQL: `ORDER BY 1, 2, 3`}},
		},
		Fields: []dbmeta.Field{
			{Name: "schema"}, {Name: "table"}, {Name: "name"},
			{Name: "enabled", Desc: "always enabled: a MariaDB trigger cannot be disabled"},
			{Name: "definition"}, {Name: "comment"},
		},
		Params: schemaParentName("trigger"),
		Scan: func(rows *sql.Rows) (dbmeta.Trigger, error) {
			var v dbmeta.Trigger
			err := rows.Scan(&v.Schema, &v.Table, &v.Name, &v.Enabled, &v.Definition, &v.Comment)
			return v, err
		},
	})

	// Sequences arrived in MariaDB 10.3, but the view that lists them arrived
	// in 11.5. Verified by running 10.6, 10.11 and 11.4, which have sequences
	// and no information_schema.SEQUENCES, against 11.5 which has both. The
	// gate is on the view rather than on the feature.
	//
	// MySQL has neither. Below the gate the query is refused rather than
	// answered empty, because there is nothing to read.
	dbmeta.Sequences.Register(dbmeta.MySQL, &dbmeta.Binding[dbmeta.Sequence]{
		Stmt: dbmeta.Stmt{
			{frag(mariaSeq, `SELECT s.sequence_schema AS "schema"`)},
			{frag(mariaSeq, `, s.sequence_name AS "name"`)},
			{frag(mariaSeq, `, s.data_type AS "data_type"`)},
			{frag(mariaSeq, `, s.start_value AS "start"`)},
			{frag(mariaSeq, `, s.minimum_value AS "minimum"`)},
			{frag(mariaSeq, `, s.maximum_value AS "maximum"`)},
			{frag(mariaSeq, `, s.increment AS "increment"`)},
			{frag(mariaSeq, `, s.cycle_option = 1 AS "cycles"`)},
			{frag(mariaSeq, `, '' AS "owned_by"`)},
			{frag(mariaSeq, `, NULL AS "comment"`)},
			{frag(mariaSeq, `FROM information_schema.SEQUENCES s`)},
			{frag(mariaSeq, `WHERE (@with_system OR s.sequence_schema NOT IN (`+systemSchemas+`))`)},
			{frag(mariaSeq, `AND (@schema = '' OR s.sequence_schema LIKE @schema)`)},
			{frag(mariaSeq, `AND (@name = '' OR s.sequence_name LIKE @name)`)},
			{frag(mariaSeq, `ORDER BY 1, 2`)},
		},
		Fields: []dbmeta.Field{
			seqField("schema"), seqField("name"), seqField("data_type"),
			seqField("start"), seqField("minimum"), seqField("maximum"),
			seqField("increment"), seqField("cycles"), seqField("owned_by"),
			seqField("comment"),
		},
		Params: schemaNameSystem("sequence"),
		Scan: func(rows *sql.Rows) (dbmeta.Sequence, error) {
			var v dbmeta.Sequence
			err := rows.Scan(&v.Schema, &v.Name, &v.DataType, &v.Start, &v.Minimum,
				&v.Maximum, &v.Increment, &v.Cycles, &v.OwnedBy, &v.Comment)
			return v, err
		},
	})

	dbmeta.PartitionedTables.Register(dbmeta.MySQL, &dbmeta.Binding[dbmeta.PartitionedTable]{
		Stmt: dbmeta.Stmt{
			{{SQL: `SELECT p.table_schema AS "schema"`}},
			{{SQL: `, p.table_name AS "name"`}},
			{{SQL: `, '' AS "owner"`}},
			{{SQL: `, 'table' AS "type"`}},
			{{SQL: `, '' AS "parent"`}},
			{{SQL: `, LOWER(MIN(p.partition_method)) AS "strategy"`}},
			{{SQL: `, MIN(p.partition_expression) AS "expression"`}},
			{{SQL: `, NULL AS "comment"`}},
			{{SQL: `FROM information_schema.PARTITIONS p`}},
			{{SQL: `WHERE p.partition_name IS NOT NULL`}},
			{{SQL: `AND (@with_system OR p.table_schema NOT IN (` + systemSchemas + `))`}},
			{{SQL: `AND (@schema = '' OR p.table_schema LIKE @schema)`}},
			{{SQL: `AND (@name = '' OR p.table_name LIKE @name)`}},
			{{SQL: `GROUP BY 1, 2`}},
			{{SQL: `ORDER BY 1, 2`}},
		},
		Fields: fields("schema", "name", "owner", "type", "parent", "strategy", "expression", "comment"),
		Params: schemaNameSystem("partitioned table"),
		Scan: func(rows *sql.Rows) (dbmeta.PartitionedTable, error) {
			var v dbmeta.PartitionedTable
			err := rows.Scan(&v.Schema, &v.Name, &v.Owner, &v.Type, &v.Parent,
				&v.Strategy, &v.Expression, &v.Comment)
			return v, err
		},
	})

	// A comment is a column on the object rather than a catalog of its own, so
	// this gathers the ones that are set.
	dbmeta.Comments.Register(dbmeta.MySQL, &dbmeta.Binding[dbmeta.Comment]{
		Stmt: dbmeta.Stmt{
			{{SQL: `SELECT t.table_schema AS "schema"`}},
			{{SQL: `, t.table_name AS "name"`}},
			{{SQL: `, CASE t.table_type WHEN 'BASE TABLE' THEN 'table' ELSE LOWER(t.table_type) END AS "type"`}},
			{{SQL: `, t.table_comment AS "comment"`}},
			{{SQL: `FROM information_schema.TABLES t`}},
			{{SQL: `WHERE t.table_comment <> ''`}},
			{{SQL: `AND (@with_system OR t.table_schema NOT IN (` + systemSchemas + `))`}},
			{{SQL: `AND (@schema = '' OR t.table_schema LIKE @schema)`}},
			{{SQL: `AND (@name = '' OR t.table_name LIKE @name)`}},
			{{SQL: `ORDER BY 1, 2`}},
		},
		Fields: fields("schema", "name", "type", "comment"),
		Params: schemaNameSystem("object"),
		Scan: func(rows *sql.Rows) (dbmeta.Comment, error) {
			var v dbmeta.Comment
			err := rows.Scan(&v.Schema, &v.Name, &v.Type, &v.Comment)
			return v, err
		},
	})
}

// The two products reach the check clause through views of different shape.
// MariaDB records the table on CHECK_CONSTRAINTS and MySQL does not, so MySQL
// matches on the schema and the constraint name alone. A constraint name is
// unique within a schema in MySQL, so that is enough.
const (
	mariaCheckJoin = `LEFT JOIN information_schema.CHECK_CONSTRAINTS k` +
		` ON k.constraint_schema = t.constraint_schema` +
		` AND k.table_name = t.table_name` +
		` AND k.constraint_name = t.constraint_name`
	mysqlCheckJoin = `LEFT JOIN information_schema.CHECK_CONSTRAINTS k` +
		` ON k.constraint_schema = t.constraint_schema` +
		` AND k.constraint_name = t.constraint_name`
)

// seqField declares one column of the sequence query. Every one of them needs
// MariaDB 11.5, and MySQL has no sequences at any release.
func seqField(name string) dbmeta.Field {
	return dbmeta.Field{Name: name, Min: mariaSeq.Min, Key: mariaSeq.Key}
}
