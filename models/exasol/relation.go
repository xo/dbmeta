package exasol

import (
	"database/sql"

	"github.com/xo/dbmeta"
)

func registerRelations() {
	registerTables()
	registerColumns()
	registerIndexes()
	registerIndexColumns()
	registerConstraints()
}

func registerTables() {
	// \dn. A virtual schema is a schema here too, and SCHEMA_IS_VIRTUAL is
	// what tells the two apart. ForeignServers reports the virtual ones
	// again, with their adapter.
	dbmeta.Schemas.Register(dbmeta.Exasol, &dbmeta.Binding[dbmeta.Schema]{
		Stmt: dbmeta.Stmt{
			always(`SELECT '' AS "catalog"`),
			always(`, s.SCHEMA_NAME AS "name"`),
			always(`, s.SCHEMA_OWNER AS "owner"`),
			always(`, s.SCHEMA_COMMENT AS "comment"`),
			always(`FROM EXA_ALL_SCHEMAS s`),
			always(`WHERE ` + like(`s.SCHEMA_NAME`, `@name`)),
			// The two schemas the system tables live in are not in
			// EXA_ALL_SCHEMAS. EXA_SYSCAT names them.
			always(`UNION ALL`),
			always(`SELECT DISTINCT '', c.SCHEMA_NAME, 'SYS', CAST(NULL AS VARCHAR(2000))`),
			always(`FROM EXA_SYSCAT c`),
			always(`WHERE ` + system + ` AND ` + like(`c.SCHEMA_NAME`, `@name`)),
			always(`ORDER BY 2`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "always empty: an Exasol connection reaches one database and has no catalog level"},
			{Name: "name"},
			{Name: "owner", Desc: "from SCHEMA_OWNER. SYS for the two system schemas"},
			{Name: "comment", Desc: "from COMMENT ON SCHEMA, and absent for the two system schemas"},
		},
		Params: []dbmeta.Param{
			{Name: "name", Desc: "schema name pattern, empty for every schema", Default: ""},
			{Name: "with_system", Desc: "include SYS and EXA_STATISTICS, which hold the system tables", Default: false},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Schema, error) {
			var v dbmeta.Schema
			err := rows.Scan(dbmeta.NullAsEmpty(&v.Catalog), dbmeta.NullAsEmpty(&v.Name), dbmeta.NullAsEmpty(&v.Owner), &v.Comment)
			return v, err
		},
	})

	// \dt and \dv. Exasol keeps tables and views in two views, and a
	// virtual table is in the table view with a flag, so this is one
	// statement over both and the type column separates them.
	dbmeta.Tables.Register(dbmeta.Exasol, &dbmeta.Binding[dbmeta.Table]{
		Stmt: dbmeta.Stmt{
			always(`SELECT '' AS "catalog"`),
			always(`, t.TABLE_SCHEMA AS "schema"`),
			always(`, t.TABLE_NAME AS "name"`),
			always(`, CASE WHEN t.TABLE_IS_VIRTUAL THEN 'virtual table'` +
				` ELSE 'table' END AS "type"`),
			always(`, t.TABLE_COMMENT AS "comment"`),
			always(`FROM EXA_ALL_TABLES t`),
			always(`WHERE ` + like(`t.TABLE_SCHEMA`, `@schema`)),
			always(`AND ` + like(`t.TABLE_NAME`, `@name`)),
			always(`AND (@types IS NULL OR ` + dbmeta.InList(`@types`, `CASE WHEN t.TABLE_IS_VIRTUAL THEN 'virtual table' ELSE 'table' END`) + `)`),
			always(`UNION ALL`),
			always(`SELECT '', v.VIEW_SCHEMA, v.VIEW_NAME, 'view', v.VIEW_COMMENT`),
			always(`FROM EXA_ALL_VIEWS v`),
			always(`WHERE ` + like(`v.VIEW_SCHEMA`, `@schema`)),
			always(`AND ` + like(`v.VIEW_NAME`, `@name`)),
			always(`AND (@types IS NULL OR ` + dbmeta.InList(`@types`, `'view'`) + `)`),
			always(`UNION ALL`),
			always(`SELECT '', c.SCHEMA_NAME, c.OBJECT_NAME, 'system table', c.OBJECT_COMMENT`),
			always(`FROM EXA_SYSCAT c`),
			always(`WHERE ` + system),
			always(`AND ` + like(`c.SCHEMA_NAME`, `@schema`)),
			always(`AND ` + like(`c.OBJECT_NAME`, `@name`)),
			always(`AND (@types IS NULL OR ` + dbmeta.InList(`@types`, `'system table'`) + `)`),
			always(`ORDER BY 2, 3`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "always empty: an Exasol connection reaches one database"},
			{Name: "schema"},
			{Name: "name"},
			{Name: "type", Desc: "table, virtual table, view or system table. A virtual table belongs to a virtual schema and its rows come from an adapter"},
			{Name: "comment", Desc: "from COMMENT ON TABLE, or the COMMENT IS clause of CREATE VIEW, which is the only way a view takes one. A system table carries the engine's own description"},
		},
		Params: append(schemaAndName("table"), dbmeta.TypesParam()),
		Scan: func(rows *sql.Rows) (dbmeta.Table, error) {
			var v dbmeta.Table
			err := rows.Scan(dbmeta.NullAsEmpty(&v.Catalog), dbmeta.NullAsEmpty(&v.Schema), dbmeta.NullAsEmpty(&v.Name), dbmeta.NullAsEmpty(&v.Type), &v.Comment)
			return v, err
		},
	})

	// \dv in full.
	dbmeta.Views.Register(dbmeta.Exasol, &dbmeta.Binding[dbmeta.View]{
		Stmt: dbmeta.Stmt{
			always(`SELECT '' AS "catalog"`),
			always(`, v.VIEW_SCHEMA AS "schema"`),
			always(`, v.VIEW_NAME AS "name"`),
			always(`, v.VIEW_TEXT AS "definition"`),
			always(`, 'none' AS "check_option"`),
			always(`, FALSE AS "updatable"`),
			always(`, FALSE AS "insertable"`),
			always(`, v.VIEW_COMMENT AS "comment"`),
			always(`FROM EXA_ALL_VIEWS v`),
			always(`WHERE ` + like(`v.VIEW_SCHEMA`, `@schema`)),
			always(`AND ` + like(`v.VIEW_NAME`, `@name`)),
			always(`ORDER BY v.VIEW_SCHEMA, v.VIEW_NAME`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "always empty: an Exasol connection reaches one database"},
			{Name: "schema"}, {Name: "name"},
			{Name: "definition", Desc: "the whole CREATE VIEW statement, which is what Exasol stores in VIEW_TEXT, rather than the query alone"},
			{Name: "check_option", Desc: "always none: Exasol has no WITH CHECK OPTION"},
			{Name: "updatable", Desc: "always false: an Exasol view cannot be the target of INSERT, UPDATE or DELETE"},
			{Name: "insertable", Desc: "always false, for the same reason"},
			{Name: "comment", Desc: "from the COMMENT IS clause of CREATE VIEW"},
		},
		Params: []dbmeta.Param{
			{Name: "schema", Desc: "schema name pattern, empty for every schema", Default: ""},
			{Name: "name", Desc: "view name pattern, empty for every view", Default: ""},
		},
		Scan: func(rows *sql.Rows) (dbmeta.View, error) {
			var v dbmeta.View
			err := rows.Scan(dbmeta.NullAsEmpty(&v.Catalog), dbmeta.NullAsEmpty(&v.Schema), dbmeta.NullAsEmpty(&v.Name), &v.Definition,
				&v.CheckOption, &v.Updatable, &v.Insertable, &v.Comment)
			return v, err
		},
	})

	// \dP. Exasol partitions a table by the values of one or more of its
	// columns. The table carries a flag and each column its position in
	// the key, so the expression is the key columns in order.
	dbmeta.PartitionedTables.Register(dbmeta.Exasol, &dbmeta.Binding[dbmeta.PartitionedTable]{
		Stmt: dbmeta.Stmt{
			always(`SELECT t.TABLE_SCHEMA AS "schema"`),
			always(`, t.TABLE_NAME AS "name"`),
			always(`, t.TABLE_OWNER AS "owner"`),
			always(`, 'table' AS "type"`),
			always(`, '' AS "parent"`),
			always(`, 'column' AS "strategy"`),
			// One bounded read of the table's own columns.
			always(`, (SELECT GROUP_CONCAT(c.COLUMN_NAME` +
				` ORDER BY c.COLUMN_PARTITION_KEY_ORDINAL_POSITION SEPARATOR ', ')` +
				` FROM EXA_ALL_COLUMNS c` +
				` WHERE c.COLUMN_SCHEMA = t.TABLE_SCHEMA AND c.COLUMN_TABLE = t.TABLE_NAME` +
				` AND c.COLUMN_PARTITION_KEY_ORDINAL_POSITION IS NOT NULL) AS "expression"`),
			always(`, t.TABLE_COMMENT AS "comment"`),
			always(`FROM EXA_ALL_TABLES t`),
			always(`WHERE t.TABLE_HAS_PARTITION_KEY`),
			always(`AND ` + like(`t.TABLE_SCHEMA`, `@schema`)),
			always(`AND ` + like(`t.TABLE_NAME`, `@name`)),
			always(`ORDER BY t.TABLE_SCHEMA, t.TABLE_NAME`),
		},
		Fields: []dbmeta.Field{
			{Name: "schema"}, {Name: "name"},
			{Name: "owner"},
			{Name: "type", Desc: "always table: Exasol partitions a table and nothing else"},
			{Name: "parent", Desc: "always empty: an Exasol partition is not a table of its own, so there is no parent to name"},
			{Name: "strategy", Desc: "always column: Exasol partitions by the values of the key columns and records no range, list or hash kind"},
			{Name: "expression", Desc: "the partition key columns, comma separated and in key order"},
			{Name: "comment", Desc: "the table's comment"},
		},
		Params: []dbmeta.Param{
			{Name: "schema", Desc: "schema name pattern, empty for every schema", Default: ""},
			{Name: "name", Desc: "table name pattern, empty for every table", Default: ""},
		},
		Scan: func(rows *sql.Rows) (dbmeta.PartitionedTable, error) {
			var v dbmeta.PartitionedTable
			err := rows.Scan(dbmeta.NullAsEmpty(&v.Schema), dbmeta.NullAsEmpty(&v.Name), dbmeta.NullAsEmpty(&v.Owner), dbmeta.NullAsEmpty(&v.Type), dbmeta.NullAsEmpty(&v.Parent),
				dbmeta.NullAsEmpty(&v.Strategy), dbmeta.NullAsEmpty(&v.Expression), &v.Comment)
			return v, err
		},
	})

	// Every comment in the database. Exasol puts the comment on the
	// object's own row, and EXA_ALL_OBJECTS carries it for every schema
	// object, so this is that view and the three principal and connection
	// views that are not in it.
	dbmeta.Comments.Register(dbmeta.Exasol, &dbmeta.Binding[dbmeta.Comment]{
		Stmt: dbmeta.Stmt{
			always(`SELECT CASE WHEN o.ROOT_TYPE = 'SCHEMA' THEN o.ROOT_NAME ELSE '' END AS "schema"`),
			always(`, o.OBJECT_NAME AS "name"`),
			always(`, LOWER(o.OBJECT_TYPE) AS "type"`),
			always(`, o.OBJECT_COMMENT AS "comment"`),
			always(`FROM EXA_ALL_OBJECTS o`),
			always(`WHERE o.OBJECT_COMMENT IS NOT NULL AND ` + like(`o.OBJECT_NAME`, `@name`)),
			always(`UNION ALL`),
			always(`SELECT '', u.USER_NAME, 'user', u.USER_COMMENT FROM EXA_ALL_USERS u`),
			always(`WHERE u.USER_COMMENT IS NOT NULL AND ` + like(`u.USER_NAME`, `@name`)),
			always(`UNION ALL`),
			always(`SELECT '', r.ROLE_NAME, 'role', r.ROLE_COMMENT FROM EXA_ALL_ROLES r`),
			always(`WHERE r.ROLE_COMMENT IS NOT NULL AND ` + like(`r.ROLE_NAME`, `@name`)),
			always(`UNION ALL`),
			always(`SELECT '', c.CONNECTION_NAME, 'connection', c.CONNECTION_COMMENT FROM EXA_ALL_CONNECTIONS c`),
			always(`WHERE c.CONNECTION_COMMENT IS NOT NULL AND ` + like(`c.CONNECTION_NAME`, `@name`)),
			always(`ORDER BY 3, 1, 2`),
		},
		Fields: []dbmeta.Field{
			{Name: "schema", Desc: "empty for a schema, a user, a role and a connection, which belong to the database rather than to a schema"},
			{Name: "name"},
			{Name: "type", Desc: "the kind of object, lower cased: schema, table, view, function, script, user, role or connection. A column comment is left out, because it needs two names to identify it and this kind has one, and Columns carries it"},
			{Name: "comment", Desc: "the comment. The users and roles Exasol creates itself, such as SYS, PUBLIC and DBA, carry a description the engine wrote"},
		},
		Params: nameOnly("object"),
		Scan: func(rows *sql.Rows) (dbmeta.Comment, error) {
			var v dbmeta.Comment
			err := rows.Scan(dbmeta.NullAsEmpty(&v.Schema), dbmeta.NullAsEmpty(&v.Name), dbmeta.NullAsEmpty(&v.Type), dbmeta.NullAsEmpty(&v.Comment))
			return v, err
		},
	})
}

func registerColumns() {
	// The columns of a table or a view. EXA_ALL_COLUMNS holds both, with
	// COLUMN_OBJECT_TYPE saying which, and EXA_SYS_COLUMNS is the same
	// shape for the system tables.
	dbmeta.Columns.Register(dbmeta.Exasol, &dbmeta.Binding[dbmeta.Column]{
		Stmt: dbmeta.Stmt{
			always(`SELECT '' AS "catalog"`),
			always(`, c.COLUMN_SCHEMA AS "schema"`),
			always(`, c.COLUMN_TABLE AS "table"`),
			always(`, c.COLUMN_NAME AS "name"`),
			always(`, CAST(c.COLUMN_ORDINAL_POSITION AS INTEGER) AS "ordinal"`),
			always(`, c.COLUMN_TYPE AS "data_type"`),
			// A view column has no nullability of its own and the catalog
			// records NULL for it. PostgreSQL reports a view column as
			// nullable, because nothing on the view forbids a NULL, and
			// that is what this reports too.
			always(`, CASE WHEN c.COLUMN_IS_NULLABLE = FALSE THEN FALSE ELSE TRUE END AS "nullable"`),
			always(`, c.COLUMN_DEFAULT AS "default"`),
			always(`, CASE WHEN k.COLUMN_NAME IS NULL THEN FALSE ELSE TRUE END AS "primary_key"`),
			always(`, CASE WHEN c.COLUMN_IDENTITY IS NOT NULL THEN 'by default'` +
				` ELSE '' END AS "identity"`),
			always(`, '' AS "generated"`),
			always(`, c.COLUMN_COMMENT AS "comment"`),
			always(`, NULL AS "collation"`),
			always(`FROM EXA_ALL_COLUMNS c`),
			// Exasol refuses a correlated EXISTS in a select list, so the
			// key columns are joined. A column is in at most one primary
			// key, so the join adds no rows.
			always(`LEFT JOIN EXA_ALL_CONSTRAINT_COLUMNS k` +
				` ON k.CONSTRAINT_SCHEMA = c.COLUMN_SCHEMA AND k.CONSTRAINT_TABLE = c.COLUMN_TABLE` +
				` AND k.COLUMN_NAME = c.COLUMN_NAME AND k.CONSTRAINT_TYPE = 'PRIMARY KEY'`),
			always(`WHERE ` + like(`c.COLUMN_SCHEMA`, `@schema`)),
			always(`AND ` + like(`c.COLUMN_TABLE`, `@parent`)),
			always(`AND ` + like(`c.COLUMN_NAME`, `@name`)),
			always(`UNION ALL`),
			always(`SELECT '', s.COLUMN_SCHEMA, s.COLUMN_TABLE, s.COLUMN_NAME`),
			always(`, CAST(s.COLUMN_ORDINAL_POSITION AS INTEGER), s.COLUMN_TYPE`),
			always(`, CASE WHEN s.COLUMN_IS_NULLABLE = FALSE THEN FALSE ELSE TRUE END`),
			always(`, s.COLUMN_DEFAULT, FALSE, '', '', s.COLUMN_COMMENT, NULL`),
			always(`FROM EXA_SYS_COLUMNS s`),
			always(`WHERE ` + system),
			always(`AND ` + like(`s.COLUMN_SCHEMA`, `@schema`)),
			always(`AND ` + like(`s.COLUMN_TABLE`, `@parent`)),
			always(`AND ` + like(`s.COLUMN_NAME`, `@name`)),
			always(`ORDER BY 2, 3, 5`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "always empty: an Exasol connection reaches one database"},
			{Name: "schema"}, {Name: "table"}, {Name: "name"},
			{Name: "ordinal", Desc: "from COLUMN_ORDINAL_POSITION, which counts from one"},
			{Name: "data_type", Desc: "the type as Exasol writes it, such as VARCHAR(128) UTF8. INTEGER is an alias and arrives as DECIMAL(18,0), which is what Exasol stores"},
			{Name: "nullable", Desc: "true for a view column, which Exasol records no nullability for, the way PostgreSQL reports one"},
			{Name: "default", Desc: "the default expression as written, such as 'plain' with its quotes"},
			{Name: "primary_key", Desc: "always false for a view column: Exasol records no key on a view"},
			{Name: "identity", Desc: "by default for an IDENTITY column, which accepts an explicit value as well as generating one, and empty otherwise"},
			{Name: "generated", Desc: "always empty: Exasol has no generated column"},
			{Name: "comment", Desc: "from COMMENT ON COLUMN. A system table column carries the engine's own description"},
			{Name: "collation", Desc: "always absent: Exasol has no collation on a column"},
		},
		Params: []dbmeta.Param{
			{Name: "schema", Desc: "schema name pattern, empty for every schema", Default: ""},
			{Name: "parent", Desc: "table or view name pattern, empty for every one", Default: ""},
			{Name: "name", Desc: "column name pattern, empty for every column", Default: ""},
			{Name: "with_system", Desc: "include the columns of the system tables", Default: false},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Column, error) {
			var v dbmeta.Column
			err := rows.Scan(dbmeta.NullAsEmpty(&v.Catalog), dbmeta.NullAsEmpty(&v.Schema), dbmeta.NullAsEmpty(&v.Table), dbmeta.NullAsEmpty(&v.Name), &v.Ordinal,
				dbmeta.NullAsEmpty(&v.DataType), &v.Nullable, &v.Default, &v.PrimaryKey, present(&v.Identity),
				present(&v.Generated), &v.Comment, &v.Collation)
			return v, err
		},
	})
}

func registerIndexes() {
	// \di. Exasol builds its own indices and nobody names one. The engine
	// creates one for a key and others as joins need them, and drops the
	// ones that go unused. The object id is the only identity an index has,
	// so it is the name, and REMARKS is Exasol's own description of it.
	dbmeta.Indexes.Register(dbmeta.Exasol, &dbmeta.Binding[dbmeta.Index]{
		Stmt: dbmeta.Stmt{
			always(`SELECT '' AS "catalog"`),
			always(`, i.INDEX_SCHEMA AS "schema"`),
			always(`, i.INDEX_TABLE AS "table"`),
			always(`, CAST(i.INDEX_OBJECT_ID AS VARCHAR(20)) AS "name"`),
			always(`, LOWER(i.INDEX_TYPE) AS "type"`),
			always(`, FALSE AS "unique"`),
			always(`, FALSE AS "primary"`),
			always(`, i.REMARKS AS "comment"`),
			always(`FROM EXA_ALL_INDICES i`),
			always(`WHERE ` + like(`i.INDEX_SCHEMA`, `@schema`)),
			always(`AND ` + like(`i.INDEX_TABLE`, `@parent`)),
			always(`AND ` + like(`CAST(i.INDEX_OBJECT_ID AS VARCHAR(20))`, `@name`)),
			always(`ORDER BY i.INDEX_SCHEMA, i.INDEX_TABLE, i.INDEX_OBJECT_ID`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "always empty: an Exasol connection reaches one database"},
			{Name: "schema"}, {Name: "table"},
			{Name: "name", Desc: "the index object id. Exasol builds its own indices and names none of them, so the id is the only thing that tells two apart"},
			{Name: "type", Desc: "global or local, which is whether the index spans the cluster or one node's share of the table"},
			{Name: "unique", Desc: "always false: an Exasol index enforces nothing. A primary key is checked by its constraint"},
			{Name: "primary", Desc: "always false: Exasol records no link from an index to the constraint it serves"},
			{Name: "comment", Desc: "from REMARKS, the engine's own description, such as GLOBAL INDEX (AUTHOR_ID), which names the columns"},
		},
		Params: parentAndName("index"),
		Scan: func(rows *sql.Rows) (dbmeta.Index, error) {
			var v dbmeta.Index
			err := rows.Scan(dbmeta.NullAsEmpty(&v.Catalog), dbmeta.NullAsEmpty(&v.Schema), dbmeta.NullAsEmpty(&v.Table), dbmeta.NullAsEmpty(&v.Name), dbmeta.NullAsEmpty(&v.Type),
				&v.Unique, &v.Primary, &v.Comment)
			return v, err
		},
	})
}

// indexList is the column list inside an index's REMARKS, which reads
// GLOBAL INDEX (COUNTRY,AREA): everything between the first opening
// parenthesis and the last closing one, so that a column named p(q) stays
// whole.
const indexList = `SUBSTR(i.REMARKS, INSTR(i.REMARKS, '(') + 1,` +
	` INSTR(i.REMARKS, ')', -1) - INSTR(i.REMARKS, '(') - 1)`

// indexPosition is where a column's name starts in its index's list, with a
// comma added at each end of both so that AREA does not match inside
// SUBAREA. Zero means the column is not in the index.
const indexPosition = `INSTR(',' || ` + indexList + ` || ',', ',' || c.COLUMN_NAME || ',')`

func registerIndexColumns() {
	// The columns of each index, which Exasol lists only inside REMARKS.
	//
	// The list is not split on its commas, because the names in it are not
	// quoted and a column can be called "a,b". It is matched against the
	// table's real columns instead, and each column's place in the list is
	// its position in the index. A name that is itself two other column
	// names joined by a comma matches twice, and nothing short of that
	// can be misread.
	dbmeta.IndexColumns.Register(dbmeta.Exasol, &dbmeta.Binding[dbmeta.IndexColumn]{
		Stmt: dbmeta.Stmt{
			always(`SELECT i.INDEX_SCHEMA AS "schema"`),
			always(`, i.INDEX_TABLE AS "table"`),
			always(`, CAST(i.INDEX_OBJECT_ID AS VARCHAR(20)) AS "index"`),
			always(`, c.COLUMN_NAME AS "name"`),
			always(`, CAST(RANK() OVER (PARTITION BY i.INDEX_OBJECT_ID` +
				` ORDER BY ` + indexPosition + `) AS INTEGER) AS "ordinal"`),
			always(`, CAST(NULL AS VARCHAR(1)) AS "expression"`),
			always(`, FALSE AS "descending"`),
			always(`FROM EXA_ALL_INDICES i`),
			always(`JOIN EXA_ALL_COLUMNS c ON c.COLUMN_SCHEMA = i.INDEX_SCHEMA` +
				` AND c.COLUMN_TABLE = i.INDEX_TABLE`),
			always(`WHERE ` + indexPosition + ` > 0`),
			always(`AND ` + like(`i.INDEX_SCHEMA`, `@schema`)),
			always(`AND ` + like(`i.INDEX_TABLE`, `@parent`)),
			always(`AND ` + like(`CAST(i.INDEX_OBJECT_ID AS VARCHAR(20))`, `@name`)),
			always(`ORDER BY 1, 2, i.INDEX_OBJECT_ID, 5`),
		},
		Fields: []dbmeta.Field{
			{Name: "schema"}, {Name: "table"},
			{Name: "index", Desc: "the index object id, which is what Indexes reports as its name"},
			{Name: "name"},
			{Name: "ordinal", Desc: "the column's position in the index, counting from one, read from the order REMARKS lists the columns in"},
			{Name: "expression", Desc: "always absent: Exasol indexes columns and has no expression index"},
			{Name: "descending", Desc: "always false: an Exasol index has no sort direction"},
		},
		Params: parentAndName("index"),
		Scan: func(rows *sql.Rows) (dbmeta.IndexColumn, error) {
			var v dbmeta.IndexColumn
			err := rows.Scan(dbmeta.NullAsEmpty(&v.Schema), dbmeta.NullAsEmpty(&v.Table), dbmeta.NullAsEmpty(&v.Index), &v.Name,
				&v.Ordinal, &v.Expression, &v.Descending)
			return v, err
		},
	})
}

func registerConstraints() {
	// The constraints on a table. Exasol has three kinds and names a not
	// null constraint itself when the statement did not.
	dbmeta.Constraints.Register(dbmeta.Exasol, &dbmeta.Binding[dbmeta.Constraint]{
		Stmt: dbmeta.Stmt{
			always(`SELECT k.CONSTRAINT_SCHEMA AS "schema"`),
			always(`, k.CONSTRAINT_TABLE AS "table"`),
			always(`, k.CONSTRAINT_NAME AS "name"`),
			always(`, LOWER(k.CONSTRAINT_TYPE) AS "type"`),
			always(`, CAST(NULL AS VARCHAR(1)) AS "definition"`),
			always(`, FALSE AS "deferrable"`),
			always(`, FALSE AS "deferred"`),
			always(`, CAST(NULL AS VARCHAR(1)) AS "comment"`),
			always(`FROM EXA_ALL_CONSTRAINTS k`),
			always(`WHERE ` + like(`k.CONSTRAINT_SCHEMA`, `@schema`)),
			always(`AND ` + like(`k.CONSTRAINT_TABLE`, `@parent`)),
			always(`AND ` + like(`k.CONSTRAINT_NAME`, `@name`)),
			always(`ORDER BY 1, 2, 3`),
		},
		Fields: []dbmeta.Field{
			{Name: "schema"}, {Name: "table"},
			{Name: "name", Desc: "a not null constraint declared without a name gets a generated one such as SYS_1327..., because Exasol names it rather than the statement"},
			{Name: "type", Desc: "primary key, foreign key or not null. Exasol has no unique or check constraint"},
			{Name: "definition", Desc: "always absent: Exasol keeps no constraint text. ConstraintColumns carries the columns"},
			{Name: "deferrable", Desc: "always false: Exasol defers no constraint"},
			{Name: "deferred", Desc: "always false, for the same reason"},
			{Name: "comment", Desc: "always absent: COMMENT ON has no constraint form"},
		},
		Params: parentAndName("constraint"),
		Scan: func(rows *sql.Rows) (dbmeta.Constraint, error) {
			var v dbmeta.Constraint
			err := rows.Scan(dbmeta.NullAsEmpty(&v.Schema), dbmeta.NullAsEmpty(&v.Table), dbmeta.NullAsEmpty(&v.Name), dbmeta.NullAsEmpty(&v.Type), &v.Definition,
				&v.Deferrable, &v.Deferred, &v.Comment)
			return v, err
		},
	})

	// The columns of each constraint, and the column each foreign key points
	// at. Exasol records the target on the row itself, the way HANA does.
	dbmeta.ConstraintColumns.Register(dbmeta.Exasol, &dbmeta.Binding[dbmeta.ConstraintColumn]{
		Stmt: dbmeta.Stmt{
			always(`SELECT '' AS "catalog"`),
			always(`, k.CONSTRAINT_SCHEMA AS "schema"`),
			always(`, k.CONSTRAINT_TABLE AS "table"`),
			always(`, k.CONSTRAINT_NAME AS "constraint"`),
			always(`, k.COLUMN_NAME AS "name"`),
			// A not null constraint covers one column and the catalog
			// leaves its position NULL, because there is no order to
			// record. One is the position that column genuinely has.
			always(`, CAST(CASE WHEN k.CONSTRAINT_TYPE = 'NOT NULL' THEN 1` +
				` ELSE k.ORDINAL_POSITION END AS INTEGER) AS "ordinal"`),
			always(`, CASE WHEN k.CONSTRAINT_TYPE = 'FOREIGN KEY' THEN ''` +
				` END AS "foreign_catalog"`),
			always(`, k.REFERENCED_SCHEMA AS "foreign_schema"`),
			always(`, k.REFERENCED_TABLE AS "foreign_table"`),
			always(`, k.REFERENCED_COLUMN AS "foreign_name"`),
			always(`FROM EXA_ALL_CONSTRAINT_COLUMNS k`),
			always(`WHERE ` + like(`k.CONSTRAINT_SCHEMA`, `@schema`)),
			always(`AND ` + like(`k.CONSTRAINT_TABLE`, `@parent`)),
			always(`AND ` + like(`k.CONSTRAINT_NAME`, `@name`)),
			always(`ORDER BY 2, 3, 4, 6`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "always empty: an Exasol connection reaches one database"},
			{Name: "schema"}, {Name: "table"}, {Name: "constraint"}, {Name: "name"},
			{Name: "ordinal", Desc: "the position in the key, counting from one. A not null constraint covers one column and reports one"},
			{Name: "foreign_catalog", Desc: "empty for a foreign key and absent for every other kind"},
			{Name: "foreign_schema", Desc: "absent unless this is a foreign key"},
			{Name: "foreign_table", Desc: "absent unless this is a foreign key"},
			{Name: "foreign_name", Desc: "absent unless this is a foreign key"},
		},
		Params: parentAndName("constraint"),
		Scan: func(rows *sql.Rows) (dbmeta.ConstraintColumn, error) {
			var v dbmeta.ConstraintColumn
			err := rows.Scan(dbmeta.NullAsEmpty(&v.Catalog), dbmeta.NullAsEmpty(&v.Schema), dbmeta.NullAsEmpty(&v.Table), dbmeta.NullAsEmpty(&v.Constraint), dbmeta.NullAsEmpty(&v.Name),
				&v.Ordinal, &v.ForeignCatalog, &v.ForeignSchema, &v.ForeignTable, &v.ForeignName)
			// The statement selects '' for a foreign key's catalog, and Exasol
			// reads that as NULL, so every row arrives with it absent. A row
			// with a foreign table is a foreign key and its catalog is the
			// empty string, which is what every other model reports.
			if v.ForeignTable.Valid {
				v.ForeignCatalog = sql.Null[string]{Valid: true}
			}
			return v, err
		},
	})
}
