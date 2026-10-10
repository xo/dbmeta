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
	// different shape. Reporting one as unsupported is wrong, because
	// the database does have the concept. It does not separate the two.
	dbmeta.Schemas.Register(dbmeta.MySQL, &dbmeta.Binding[dbmeta.Schema]{
		Stmt: dbmeta.Stmt{
			{{Query: `SELECT s.catalog_name AS "catalog"`}},
			schemaAs(`, `, "s.schema_name", "name"),
			// MariaDB records no owner for a schema
			{{Query: `, '' AS "owner"`}},
			{{Query: `, NULL AS "comment"`}},
			{{Query: `FROM information_schema.SCHEMATA s`}},
			notSystem("WHERE", "s.schema_name"),
			schemaLike("name", "s.schema_name"),
			{{Query: `ORDER BY 2`}},
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
			schemaAs(`SELECT `, "s.schema_name", "name"),
			{{Query: `, '' AS "owner"`}},
			{{Query: `, s.default_character_set_name AS "encoding"`}},
			{{Query: `, s.default_collation_name AS "collate"`}},
			{{Query: `, s.default_collation_name AS "ctype"`}},
			{{Query: `, NULL AS "access"`}},
			{{Query: `, NULL AS "tablespace"`}},
			{{Query: `, NULL AS "size"`}},
			{{Query: `, NULL AS "comment"`}},
			{{Query: `FROM information_schema.SCHEMATA s`}},
			notSystem("WHERE", "s.schema_name"),
			schemaLike("name", "s.schema_name"),
			{{Query: `ORDER BY 1`}},
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
	// Both products write the word VIEW there for every view, and neither lets
	// a view have a comment, so a view's comment is absent. usql found a view
	// reported with the comment VIEW on 2026-09-30.
	dbmeta.Tables.Register(dbmeta.MySQL, &dbmeta.Binding[dbmeta.Table]{
		Stmt: dbmeta.Stmt{
			{{Query: `SELECT t.table_catalog AS "catalog"`}},
			schemaAs(`, `, "t.table_schema", "schema"),
			{{Query: `, t.table_name AS "name"`}},
			{{Query: `, ` + tableType + ` AS "type"`}},
			{{Query: `, ` + tableComment + ` AS "comment"`}},
			// A temporary table is listed only by MariaDB and only to the
			// session that made it, with the type TEMPORARY. Everything else
			// that either product lists is permanent. See D205.
			// TiDB lists a global temporary table as a base table and no
			// local one, so it has no source and answers absent.
			// SingleStore lists the temporary table of the session too, with
			// the type TEMPORARY TABLE.
			{
				{Query: `, CASE WHEN t.table_type IN ('TEMPORARY', 'TEMPORARY TABLE') THEN 'temporary' ELSE 'permanent' END AS "persistence"`},
				{Key: TiDB, Query: `, NULL AS "persistence"`},
			},
			// ENGINE, DATA_LENGTH, INDEX_LENGTH and TABLE_ROWS are NULL for a
			// view. The size is the data and the indexes, in bytes.
			// SingleStore has one engine, which it names MemSQL, and keeps the
			// choice that matters in STORAGE_TYPE: COLUMNSTORE, or
			// INMEMORY_ROWSTORE. That is the access method.
			{
				{Query: `, t.engine AS "access_method"`},
				{Key: MemSQL, Query: `, CASE WHEN t.table_type = 'VIEW' THEN NULL ELSE t.storage_type END AS "access_method"`},
			},
			// SingleStore answers 0 for a view, where MySQL and MariaDB
			// answer NULL, so a view is named here.
			{{Query: `, CASE WHEN t.table_type = 'VIEW' THEN NULL ELSE CAST(t.data_length + t.index_length AS SIGNED) END AS "size"`}},
			{{Query: `, CASE WHEN t.table_type = 'VIEW' THEN NULL ELSE CAST(t.table_rows AS SIGNED) END AS "rows"`}},
			{{Query: `, NULLIF(t.create_options, '') AS "options"`}},
			{{Query: `FROM information_schema.TABLES t`}},
			notSystem("WHERE", "t.table_schema"),
			schemaLike("schema", "t.table_schema"),
			{{Query: `AND (@name = '' OR t.table_name LIKE @name)`}},
			{{Query: `AND (@types = '' OR CONCAT(',', @types, ',') LIKE CONCAT('%,', ` + tableType + `, ',%'))`}},
			{{Query: `ORDER BY 2, 3`}},
		},
		Fields: []dbmeta.Field{
			{Name: "catalog"}, {Name: "schema"}, {Name: "name"}, {Name: "type"}, {Name: "comment"},
			{Name: "persistence", Desc: "permanent, or temporary for a temporary table of this session, which only MariaDB lists. Absent on TiDB"},
			{Name: "access_method", Desc: "the storage engine, absent for a view"},
			{Name: "size", Desc: "DATA_LENGTH plus INDEX_LENGTH in bytes, absent for a view"},
			{Name: "rows", Desc: "TABLE_ROWS, an estimate for InnoDB, absent for a view"},
			{Name: "options", Desc: "CREATE_OPTIONS, such as row_format=COMPRESSED, separated by a space, absent when none is set"},
		},
		Params: append(schemaNameSystem("table"), dbmeta.TypesParam()),
		Scan: func(rows *sql.Rows) (dbmeta.Table, error) {
			var v dbmeta.Table
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Name, &v.Type, &v.Comment,
				&v.Persistence, &v.AccessMethod, &v.Size, &v.Rows, &v.Options)
			return v, err
		},
	})

	// column_type carries the declared type, such as varchar(255), where
	// data_type carries only varchar. psql prints the declared type, so that
	// is what this reports.
	dbmeta.Columns.Register(dbmeta.MySQL, &dbmeta.Binding[dbmeta.Column]{
		Stmt: dbmeta.Stmt{
			{{Query: `SELECT c.table_catalog AS "catalog"`}},
			schemaAs(`, `, "c.table_schema", "schema"),
			{{Query: `, c.table_name AS "table"`}},
			{{Query: `, c.column_name AS "name"`}},
			{{Query: `, c.ordinal_position AS "ordinal"`}},
			{{Query: `, c.column_type AS "data_type"`}},
			{{Query: `, c.is_nullable = 'YES' AS "nullable"`}},
			{{Query: `, c.column_default AS "default"`}},
			// Free: information_schema.COLUMNS already says which columns are
			// in the primary key, so this costs nothing. See D47.
			{{Query: `, c.column_key = 'PRI' AS "primary_key"`}},
			// auto_increment is the closest thing to an identity column
			{{Query: `, CASE WHEN c.extra LIKE '%auto_increment%' THEN 'a' ELSE NULL END AS "identity"`}},
			{{Query: `, CASE WHEN c.extra LIKE '%GENERATED%' THEN 's' ELSE NULL END AS "generated"`}},
			{{Query: `, NULLIF(c.column_comment, '') AS "comment"`}},
			{{Query: `, c.collation_name AS "collation"`}},
			// MariaDB 10.3 can compress a column and marks it in COLUMN_TYPE
			// with a comment that older servers read as nothing. zlib is the
			// only method. MySQL has no column compression, and compresses a
			// page or a table, which Tables.Options shows. See D205.
			{
				{Query: `, NULL AS "compression"`},
				frag(mariaAgg, `, CASE WHEN c.column_type LIKE '%/*M!100301 COMPRESSED*/%' THEN 'zlib' END AS "compression"`),
			},
			{{Query: `FROM information_schema.COLUMNS c`}},
			notSystem("WHERE", "c.table_schema"),
			schemaLike("schema", "c.table_schema"),
			{{Query: `AND (@parent = '' OR c.table_name LIKE @parent)`}},
			{{Query: `AND (@name = '' OR c.column_name LIKE @name)`}},
			{{Query: `ORDER BY 2, 3, 5`}},
		},
		Fields: []dbmeta.Field{
			{Name: "catalog"}, {Name: "schema"}, {Name: "table"}, {Name: "name"}, {Name: "ordinal"},
			{Name: "data_type"}, {Name: "nullable"}, {Name: "default"}, {Name: "primary_key"},
			{Name: "identity"}, {Name: "generated"}, {Name: "comment"}, {Name: "collation"},
			{
				Name: "compression", Desc: "zlib for a compressed column, which only MariaDB has",
				Min: mariaAgg.Min, Key: mariaAgg.Key,
			},
		},
		Params: schemaParentName("column"),
		Scan: func(rows *sql.Rows) (dbmeta.Column, error) {
			var v dbmeta.Column
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Table, &v.Name, &v.Ordinal,
				&v.DataType, &v.Nullable, &v.Default, &v.PrimaryKey, &v.Identity, &v.Generated,
				&v.Comment, &v.Collation, &v.Compression)
			return v, err
		},
	})

	// STATISTICS has one row per index column, so an index listing groups it.
	dbmeta.Indexes.Register(dbmeta.MySQL, &dbmeta.Binding[dbmeta.Index]{
		Stmt: dbmeta.Stmt{
			{{Query: `SELECT s.table_catalog AS "catalog"`}},
			schemaAs(`, `, "s.table_schema", "schema"),
			{{Query: `, s.table_name AS "table"`}},
			{{Query: `, s.index_name AS "name"`}},
			{{Query: `, MIN(s.index_type) AS "type"`}},
			{{Query: `, MIN(s.non_unique) = 0 AS "unique"`}},
			{{Query: `, MIN(s.index_name) = 'PRIMARY' AS "primary"`}},
			{{Query: `, NULLIF(MIN(s.index_comment), '') AS "comment"`}},
			// BTREE, HASH, FULLTEXT, SPATIAL or RTREE, which is what follows
			// USING in the statement that made the index. See D205.
			{{Query: `, MIN(s.index_type) AS "using"`}},
			{{Query: `FROM information_schema.STATISTICS s`}},
			notSystem("WHERE", "s.table_schema"),
			schemaLike("schema", "s.table_schema"),
			{{Query: `AND (@parent = '' OR s.table_name LIKE @parent)`}},
			{{Query: `AND (@name = '' OR s.index_name LIKE @name)`}},
			{{Query: `GROUP BY 1, 2, 3, 4`}},
			{{Query: `ORDER BY 2, 3, 4`}},
		},
		Fields: fields("catalog", "schema", "table", "name", "type", "unique", "primary", "comment", "using"),
		Params: schemaParentName("index"),
		Scan: func(rows *sql.Rows) (dbmeta.Index, error) {
			var v dbmeta.Index
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Table, &v.Name, &v.Type,
				&v.Unique, &v.Primary, &v.Comment, &v.Using)
			return v, err
		},
	})

	dbmeta.IndexColumns.Register(dbmeta.MySQL, &dbmeta.Binding[dbmeta.IndexColumn]{
		Stmt: dbmeta.Stmt{
			schemaAs(`SELECT `, "s.table_schema", "schema"),
			{{Query: `, s.table_name AS "table"`}},
			{{Query: `, s.index_name AS "index"`}},
			{{Query: `, s.column_name AS "name"`}},
			{{Query: `, s.seq_in_index AS "ordinal"`}},
			// MariaDB's STATISTICS has no expression column: a functional
			// index records only the column it was built from
			{{Query: `, NULL AS "expression"`}},
			// COLLATION is NULL for an index that keeps no order, such as a hash
			// or a full text index, and no key of it is descending.
			{{Query: `, s.collation <=> 'D' AS "descending"`}},
			{{Query: `FROM information_schema.STATISTICS s`}},
			notSystem("WHERE", "s.table_schema"),
			schemaLike("schema", "s.table_schema"),
			{{Query: `AND (@parent = '' OR s.table_name LIKE @parent)`}},
			{{Query: `AND (@name = '' OR s.index_name LIKE @name)`}},
			// SingleStore lists a shard key as an index of the type SHARD,
			// and a key that is also the shard key twice, once under each
			// type. The second row of such a key is left out, and a shard key
			// that is no other index keeps its rows. See D141.
			{
				{Query: ``},
				{Key: MemSQL, Query: `AND (s.index_type <> 'SHARD' OR NOT EXISTS (SELECT 1` +
					` FROM information_schema.STATISTICS o WHERE o.table_schema = s.table_schema` +
					` AND o.table_name = s.table_name AND o.index_name = s.index_name` +
					` AND o.index_type <> 'SHARD'))`},
			},
			{{Query: `ORDER BY 1, 2, 3, 5`}},
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
			schemaAs(`SELECT `, "t.table_schema", "schema"),
			{{Query: `, t.table_name AS "table"`}},
			{{Query: `, t.constraint_name AS "name"`}},
			// SingleStore reports the primary key of a columnstore table as
			// UNIQUE, with the name PRIMARY, measured on 9.1.1. See D141.
			{
				{Query: `, LOWER(t.constraint_type) AS "type"`},
				{Key: MemSQL, Query: `, CASE WHEN t.constraint_name = 'PRIMARY' THEN 'primary key'` +
					` ELSE LOWER(t.constraint_type) END AS "type"`},
			},
			{
				{Query: `, NULL AS "definition"`},
				frag(mariaCheck, `, k.check_clause AS "definition"`),
				frag(mysqlCheck, `, k.check_clause AS "definition"`),
			},
			// MariaDB has no deferred constraints at all
			{{Query: `, FALSE AS "deferrable"`}},
			{{Query: `, FALSE AS "deferred"`}},
			{{Query: `, NULL AS "comment"`}},
			// MySQL records whether it checks a constraint from 8.0.16, in
			// TABLE_CONSTRAINTS. Below that it stays absent. MariaDB has no
			// such column and cannot disable a constraint it records, so
			// every one is enforced and the value is a genuine true, not an
			// unknown one. See D205 and D212.
			{
				{Query: `, NULL AS "enforced"`},
				frag(onMaria, `, TRUE AS "enforced"`),
				frag(mysqlCheck, `, t.enforced = 'YES' AS "enforced"`),
			},
			{{Query: `FROM information_schema.TABLE_CONSTRAINTS t`}},
			{
				{Query: ``},
				frag(mariaCheck, mariaCheckJoin),
				frag(mysqlCheck, mysqlCheckJoin),
			},
			notSystem("WHERE", "t.table_schema"),
			schemaLike("schema", "t.table_schema"),
			{{Query: `AND (@parent = '' OR t.table_name LIKE @parent)`}},
			{{Query: `AND (@name = '' OR t.constraint_name LIKE @name)`}},
			{{Query: `ORDER BY 1, 2, 3`}},
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
			{
				Name: "enforced", Desc: "whether the server checks the constraint. MySQL reads it from 8.0.16. MariaDB has no column and enforces every constraint it records, so it is always true there (D212)",
				Key:  onMaria.Key,
				Also: []dbmeta.Gate{mysqlCheck},
			},
		},
		Params: schemaParentName("constraint"),
		Scan: func(rows *sql.Rows) (dbmeta.Constraint, error) {
			var v dbmeta.Constraint
			err := rows.Scan(&v.Schema, &v.Table, &v.Name, &v.Type, &v.Definition,
				&v.Deferrable, &v.Deferred, &v.Comment, &v.Enforced)
			return v, err
		},
	})

	dbmeta.Triggers.Register(dbmeta.MySQL, &dbmeta.Binding[dbmeta.Trigger]{
		Stmt: dbmeta.Stmt{
			schemaAs(`SELECT `, "t.trigger_schema", "schema"),
			{{Query: `, t.event_object_table AS "table"`}},
			{{Query: `, t.trigger_name AS "name"`}},
			// a MariaDB trigger cannot be disabled
			{{Query: `, 'enabled' AS "enabled"`}},
			{{Query: `, CONCAT(t.action_timing, ' ', t.event_manipulation, ' ', t.action_statement) AS "definition"`}},
			{{Query: `, NULL AS "comment"`}},
			{{Query: `FROM information_schema.TRIGGERS t`}},
			notSystem("WHERE", "t.trigger_schema"),
			schemaLike("schema", "t.trigger_schema"),
			{{Query: `AND (@parent = '' OR t.event_object_table LIKE @parent)`}},
			{{Query: `AND (@name = '' OR t.trigger_name LIKE @name)`}},
			{{Query: `ORDER BY 1, 2, 3`}},
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
			schemaAs(`SELECT `, "p.table_schema", "schema"),
			{{Query: `, p.table_name AS "name"`}},
			{{Query: `, '' AS "owner"`}},
			{{Query: `, 'table' AS "type"`}},
			{{Query: `, '' AS "parent"`}},
			{{Query: `, LOWER(MIN(p.partition_method)) AS "strategy"`}},
			{{Query: `, MIN(p.partition_expression) AS "expression"`}},
			{{Query: `, NULL AS "comment"`}},
			// The size is what every partition takes. A subpartition has a
			// row of its own in PARTITIONS and a partition is the sum of its
			// subpartitions, so one sum is both the partitions one level
			// below and every partition below. See D205.
			{{Query: `, CAST(SUM(p.data_length + p.index_length) AS SIGNED) AS "direct_size"`}},
			{{Query: `, CAST(SUM(p.data_length + p.index_length) AS SIGNED) AS "total_size"`}},
			{{Query: `FROM information_schema.PARTITIONS p`}},
			{{Query: `WHERE p.partition_name IS NOT NULL`}},
			notSystem("AND", "p.table_schema"),
			schemaLike("schema", "p.table_schema"),
			{{Query: `AND (@name = '' OR p.table_name LIKE @name)`}},
			{{Query: `GROUP BY 1, 2`}},
			{{Query: `ORDER BY 1, 2`}},
		},
		Fields: fields("schema", "name", "owner", "type", "parent", "strategy", "expression", "comment",
			"direct_size", "total_size"),
		Params: schemaNameSystem("partitioned table"),
		Scan: func(rows *sql.Rows) (dbmeta.PartitionedTable, error) {
			var v dbmeta.PartitionedTable
			err := rows.Scan(&v.Schema, &v.Name, &v.Owner, &v.Type, &v.Parent,
				&v.Strategy, &v.Expression, &v.Comment, &v.DirectSize, &v.TotalSize)
			return v, err
		},
	})

	// PARTITIONS has one row for each partition, or for each subpartition of
	// a table that has them. The partitions of a table that has subpartitions
	// are the same rows with the subpartitions folded away, and the
	// subpartitions are rows of their own with the type subpartition. The
	// bound is PARTITION_DESCRIPTION: the upper limit of a range partition,
	// the values of a list partition, and absent for a hash or key partition,
	// which has none. See D205.
	dbmeta.Partitions.Register(dbmeta.MySQL, &dbmeta.Binding[dbmeta.Partition]{
		Stmt: dbmeta.Stmt{
			{{Query: `SELECT x.schema AS "schema", x.tbl AS "table", x.schema AS "partition_schema"`}},
			{{Query: `, x.part AS "partition", x.kind AS "type", x.bound AS "bound"`}},
			{{Query: `, x.has_subpartitions = 1 AS "partitioned"`}},
			{{Query: `FROM (`}},
			schemaAs(`SELECT `, "p.table_schema", "schema"),
			{{Query: `, p.table_name AS tbl, p.partition_name AS part, 'partition' AS kind`}},
			{{Query: `, p.partition_description AS bound`}},
			{{Query: `, MAX(p.subpartition_name IS NOT NULL) AS has_subpartitions`}},
			{{Query: `, MIN(p.partition_ordinal_position) AS o1, 0 AS o2`}},
			{{Query: `FROM information_schema.PARTITIONS p`}},
			{{Query: `WHERE p.partition_name IS NOT NULL`}},
			notSystem("AND", "p.table_schema"),
			schemaLike("schema", "p.table_schema"),
			{{Query: `AND (@parent = '' OR p.table_name LIKE @parent)`}},
			{{Query: `AND (@name = '' OR p.partition_name LIKE @name)`}},
			{{Query: `GROUP BY p.table_schema, p.table_name, p.partition_name, p.partition_description`}},
			{{Query: `UNION ALL`}},
			schemaAs(`SELECT `, "p.table_schema", "schema"),
			{{Query: `, p.table_name, p.subpartition_name, 'subpartition'`}},
			{{Query: `, NULL, 0`}},
			{{Query: `, p.partition_ordinal_position, p.subpartition_ordinal_position`}},
			{{Query: `FROM information_schema.PARTITIONS p`}},
			{{Query: `WHERE p.subpartition_name IS NOT NULL`}},
			notSystem("AND", "p.table_schema"),
			schemaLike("schema", "p.table_schema"),
			{{Query: `AND (@parent = '' OR p.table_name LIKE @parent)`}},
			{{Query: `AND (@name = '' OR p.subpartition_name LIKE @name)`}},
			{{Query: `) x`}},
			{{Query: `ORDER BY x.schema, x.tbl, x.o1, x.o2`}},
		},
		Fields: fields("schema", "table", "partition_schema", "partition", "type", "bound", "partitioned"),
		Params: schemaParentName("partition"),
		Scan: func(rows *sql.Rows) (dbmeta.Partition, error) {
			var v dbmeta.Partition
			err := rows.Scan(&v.Schema, &v.Table, &v.PartitionSchema, &v.Partition,
				&v.Type, &v.Bound, &v.Partitioned)
			return v, err
		},
	})

	// A comment is a column on the object rather than a catalog of its own, so
	// this gathers the ones that are set.
	dbmeta.Comments.Register(dbmeta.MySQL, &dbmeta.Binding[dbmeta.Comment]{
		Stmt: dbmeta.Stmt{
			schemaAs(`SELECT `, "t.table_schema", "schema"),
			{{Query: `, t.table_name AS "name"`}},
			{{Query: `, CASE t.table_type WHEN 'BASE TABLE' THEN 'table' ELSE LOWER(t.table_type) END AS "type"`}},
			{{Query: `, t.table_comment AS "comment"`}},
			{{Query: `FROM information_schema.TABLES t`}},
			{{Query: `WHERE t.table_comment <> '' AND t.table_type <> 'VIEW'`}},
			notSystem("AND", "t.table_schema"),
			schemaLike("schema", "t.table_schema"),
			{{Query: `AND (@name = '' OR t.table_name LIKE @name)`}},
			{{Query: `ORDER BY 1, 2`}},
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
