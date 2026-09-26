package hana

import (
	"database/sql"

	"github.com/xo/dbmeta"
)

func registerRelations() {
	registerTables()
	registerColumns()
	registerIndexes()
	registerConstraints()
	registerTriggers()
}

func registerTables() {
	// \dn.
	dbmeta.Schemas.Register(dbmeta.HANA, &dbmeta.Binding[dbmeta.Schema]{
		Stmt: dbmeta.Stmt{
			always(`SELECT '' AS "catalog"`),
			always(`, s.SCHEMA_NAME AS "name"`),
			always(`, s.SCHEMA_OWNER AS "owner"`),
			always(`, CAST(NULL AS NVARCHAR(1)) AS "comment"`),
			always(`FROM SYS.SCHEMAS s`),
			always(`WHERE ` + notSystem(`s.SCHEMA_NAME`)),
			always(`AND ` + like(`s.SCHEMA_NAME`, `@name`)),
			always(`ORDER BY s.SCHEMA_NAME`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "always empty: a HANA connection reaches one tenant database and the catalog level is the tenant itself"},
			{Name: "name"},
			{Name: "owner", Desc: "from SCHEMA_OWNER"},
			{Name: "comment", Desc: "always absent: COMMENT ON has no schema form in HANA"},
		},
		Params: []dbmeta.Param{
			{Name: "name", Desc: "schema name pattern, empty for every schema", Default: ""},
			{Name: "with_system", Desc: "include the schemas SAP HANA keeps for itself", Default: false},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Schema, error) {
			var v dbmeta.Schema
			err := rows.Scan(&v.Catalog, &v.Name, &v.Owner, &v.Comment)
			return v, err
		},
	})

	// \dt and \dv. HANA keeps tables and views in two views, so this is one
	// statement over both and the type column separates them.
	dbmeta.Tables.Register(dbmeta.HANA, &dbmeta.Binding[dbmeta.Table]{
		Stmt: dbmeta.Stmt{
			always(`SELECT '' AS "catalog"`),
			always(`, t.SCHEMA_NAME AS "schema"`),
			always(`, t.TABLE_NAME AS "name"`),
			// A HANA table is stored by row or by column and the choice is
			// per table, so the word says which rather than saying table.
			always(`, CASE WHEN t.IS_TEMPORARY = 'TRUE' THEN 'temporary table'` +
				` WHEN t.TABLE_TYPE = 'COLUMN' THEN 'column table'` +
				` ELSE 'row table' END AS "type"`),
			always(`, t.COMMENTS AS "comment"`),
			always(`FROM SYS.TABLES t`),
			always(`WHERE ` + notSystem(`t.SCHEMA_NAME`)),
			always(`AND ` + like(`t.SCHEMA_NAME`, `@schema`)),
			always(`AND ` + like(`t.TABLE_NAME`, `@name`)),
			always(`UNION ALL`),
			always(`SELECT '', v.SCHEMA_NAME, v.VIEW_NAME, 'view', v.COMMENTS`),
			always(`FROM SYS.VIEWS v`),
			always(`WHERE ` + notSystem(`v.SCHEMA_NAME`)),
			always(`AND ` + like(`v.SCHEMA_NAME`, `@schema`)),
			always(`AND ` + like(`v.VIEW_NAME`, `@name`)),
			always(`ORDER BY 2, 3`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "always empty: a connection reaches one tenant database"},
			{Name: "schema"},
			{Name: "name"},
			{Name: "type", Desc: "row table, column table, temporary table or view. HANA stores a table by row or by column and records which, so the word says which rather than saying table"},
			{Name: "comment", Desc: "from COMMENTS, which COMMENT ON writes"},
		},
		Params: schemaAndName("table"),
		Scan: func(rows *sql.Rows) (dbmeta.Table, error) {
			var v dbmeta.Table
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Name, &v.Type, &v.Comment)
			return v, err
		},
	})

	// \dv in full.
	dbmeta.Views.Register(dbmeta.HANA, &dbmeta.Binding[dbmeta.View]{
		Stmt: dbmeta.Stmt{
			always(`SELECT '' AS "catalog"`),
			always(`, v.SCHEMA_NAME AS "schema"`),
			always(`, v.VIEW_NAME AS "name"`),
			always(`, v.DEFINITION AS "definition"`),
			always(`, CASE WHEN v.HAS_CHECK_OPTION = 'TRUE' THEN 'cascaded'` +
				` ELSE 'none' END AS "check_option"`),
			always(`, ` + no(`v.IS_READ_ONLY`) + ` AS "updatable"`),
			always(`, ` + no(`v.IS_READ_ONLY`) + ` AS "insertable"`),
			always(`, v.COMMENTS AS "comment"`),
			always(`FROM SYS.VIEWS v`),
			always(`WHERE ` + notSystem(`v.SCHEMA_NAME`)),
			always(`AND ` + like(`v.SCHEMA_NAME`, `@schema`)),
			always(`AND ` + like(`v.VIEW_NAME`, `@name`)),
			always(`ORDER BY v.SCHEMA_NAME, v.VIEW_NAME`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "always empty: a connection reaches one tenant database"},
			{Name: "schema"}, {Name: "name"},
			{Name: "definition", Desc: "the CREATE VIEW text, which HANA stores in full"},
			{Name: "check_option", Desc: "cascaded or none. HANA records whether a view has one and not which kind"},
			{Name: "updatable", Desc: "from IS_READ_ONLY, inverted"},
			{Name: "insertable", Desc: "the same as updatable: HANA records one flag for both"},
			{Name: "comment"},
		},
		Params: schemaAndName("view"),
		Scan: func(rows *sql.Rows) (dbmeta.View, error) {
			var v dbmeta.View
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Name, &v.Definition,
				&v.CheckOption, &v.Updatable, &v.Insertable, &v.Comment)
			return v, err
		},
	})

	// \ds.
	dbmeta.Sequences.Register(dbmeta.HANA, &dbmeta.Binding[dbmeta.Sequence]{
		Stmt: dbmeta.Stmt{
			always(`SELECT q.SCHEMA_NAME AS "schema"`),
			always(`, q.SEQUENCE_NAME AS "name"`),
			always(`, 'BIGINT' AS "data_type"`),
			always(`, q.START_NUMBER AS "start"`),
			always(`, q.MIN_VALUE AS "minimum"`),
			always(`, q.MAX_VALUE AS "maximum"`),
			always(`, q.INCREMENT_BY AS "increment"`),
			always(`, ` + yes(`q.IS_CYCLED`) + ` AS "cycles"`),
			always(`, '' AS "owned_by"`),
			always(`, CAST(NULL AS NVARCHAR(1)) AS "comment"`),
			always(`FROM SYS.SEQUENCES q`),
			always(`WHERE ` + notSystem(`q.SCHEMA_NAME`)),
			always(`AND ` + like(`q.SCHEMA_NAME`, `@schema`)),
			always(`AND ` + like(`q.SEQUENCE_NAME`, `@name`)),
			always(`ORDER BY q.SCHEMA_NAME, q.SEQUENCE_NAME`),
		},
		Fields: []dbmeta.Field{
			{Name: "schema"}, {Name: "name"},
			{Name: "data_type", Desc: "always BIGINT: every HANA sequence is 64 bit"},
			{Name: "start"}, {Name: "minimum"}, {Name: "maximum"}, {Name: "increment"},
			{Name: "cycles"},
			{Name: "owned_by", Desc: "always empty: an identity column carries its own generator and the sequence does not name a column"},
			{Name: "comment", Desc: "always absent: COMMENT ON has no sequence form"},
		},
		Params: schemaAndName("sequence"),
		Scan: func(rows *sql.Rows) (dbmeta.Sequence, error) {
			var v dbmeta.Sequence
			err := rows.Scan(&v.Schema, &v.Name, &v.DataType, &v.Start, &v.Minimum,
				&v.Maximum, &v.Increment, &v.Cycles, &v.OwnedBy, &v.Comment)
			return v, err
		},
	})

	// \dP. HANA records the partitioning of a table as up to three levels,
	// each an expression and a kind, on one row.
	dbmeta.PartitionedTables.Register(dbmeta.HANA, &dbmeta.Binding[dbmeta.PartitionedTable]{
		Stmt: dbmeta.Stmt{
			always(`SELECT p.SCHEMA_NAME AS "schema"`),
			always(`, p.TABLE_NAME AS "name"`),
			always(`, '' AS "owner"`),
			always(`, 'table' AS "type"`),
			always(`, '' AS "parent"`),
			always(`, p.LEVEL_1_TYPE AS "strategy"`),
			always(`, p.LEVEL_1_EXPRESSION AS "expression"`),
			always(`, CAST(NULL AS NVARCHAR(1)) AS "comment"`),
			always(`FROM SYS.PARTITIONED_TABLES p`),
			always(`WHERE ` + notSystem(`p.SCHEMA_NAME`)),
			always(`AND ` + like(`p.SCHEMA_NAME`, `@schema`)),
			always(`AND ` + like(`p.TABLE_NAME`, `@name`)),
			always(`ORDER BY p.SCHEMA_NAME, p.TABLE_NAME`),
		},
		Fields: []dbmeta.Field{
			{Name: "schema"}, {Name: "name"},
			{Name: "owner", Desc: "always empty: PARTITIONED_TABLES records no owner and Tables carries the table"},
			{Name: "type", Desc: "always table: HANA partitions a table and nothing else"},
			{Name: "parent", Desc: "always empty: a HANA partition is not a table of its own, so there is no parent to name"},
			{Name: "strategy", Desc: "the first level kind, such as HASH, RANGE or ROUNDROBIN"},
			{Name: "expression", Desc: "the first level expression. HANA allows three levels and this is the first, because the kind has one column for each"},
			{Name: "comment", Desc: "always absent: Tables carries the table comment"},
		},
		Params: schemaAndName("table"),
		Scan: func(rows *sql.Rows) (dbmeta.PartitionedTable, error) {
			var v dbmeta.PartitionedTable
			err := rows.Scan(&v.Schema, &v.Name, &v.Owner, &v.Type, &v.Parent,
				&v.Strategy, &v.Expression, &v.Comment)
			return v, err
		},
	})
}

// hanaType assembles the SQL type name from the parts HANA records.
//
// TABLE_COLUMNS stores the name, the length and the scale in three columns,
// so a decimal is DECIMAL with 10 and 2 beside it rather than DECIMAL(10,2).
func hanaType(p string) string {
	return `CASE WHEN ` + p + `.DATA_TYPE_NAME IN ('DECIMAL', 'SMALLDECIMAL')` +
		` AND ` + p + `.SCALE IS NOT NULL` +
		` THEN ` + p + `.DATA_TYPE_NAME || '(' || ` + p + `.LENGTH || ',' || ` + p + `.SCALE || ')'` +
		` WHEN ` + p + `.DATA_TYPE_NAME IN ('VARCHAR', 'NVARCHAR', 'CHAR', 'NCHAR',` +
		` 'VARBINARY', 'ALPHANUM', 'SHORTTEXT')` +
		` THEN ` + p + `.DATA_TYPE_NAME || '(' || ` + p + `.LENGTH || ')'` +
		` ELSE ` + p + `.DATA_TYPE_NAME END`
}

func registerColumns() {
	// The columns of a table. A view's columns are in SYS.VIEW_COLUMNS,
	// which is the same shape, so this is one statement over both and a
	// caller sees a view's columns the way it sees a table's.
	dbmeta.Columns.Register(dbmeta.HANA, &dbmeta.Binding[dbmeta.Column]{
		Stmt: dbmeta.Stmt{
			always(`SELECT '' AS "catalog"`),
			always(`, c.SCHEMA_NAME AS "schema"`),
			always(`, c.TABLE_NAME AS "table"`),
			always(`, c.COLUMN_NAME AS "name"`),
			always(`, c.POSITION AS "ordinal"`),
			always(`, ` + hanaType("c") + ` AS "data_type"`),
			always(`, ` + yes(`c.IS_NULLABLE`) + ` AS "nullable"`),
			always(`, c.DEFAULT_VALUE AS "default"`),
			// One bounded read of the constraint view, which names a
			// handful of columns whatever the size of the catalog.
			always(`, CASE WHEN EXISTS (SELECT 1 FROM SYS.CONSTRAINTS k` +
				` WHERE k.SCHEMA_NAME = c.SCHEMA_NAME AND k.TABLE_NAME = c.TABLE_NAME` +
				` AND k.COLUMN_NAME = c.COLUMN_NAME AND k.IS_PRIMARY_KEY = 'TRUE')` +
				` THEN TRUE ELSE FALSE END AS "primary_key"`),
			// HANA records an identity column as a generation type rather
			// than as a flag, and the same column carries a computed one.
			always(`, CASE WHEN c.GENERATION_TYPE = 'ALWAYS AS IDENTITY' THEN 'always'` +
				` WHEN c.GENERATION_TYPE = 'BY DEFAULT AS IDENTITY' THEN 'by default'` +
				` ELSE '' END AS "identity"`),
			always(`, CASE WHEN c.GENERATED_ALWAYS_AS IS NOT NULL THEN 'stored'` +
				` ELSE '' END AS "generated"`),
			always(`, c.COMMENTS AS "comment"`),
			always(`FROM SYS.TABLE_COLUMNS c`),
			always(`WHERE ` + notSystem(`c.SCHEMA_NAME`)),
			always(`AND ` + like(`c.SCHEMA_NAME`, `@schema`)),
			always(`AND ` + like(`c.TABLE_NAME`, `@parent`)),
			always(`AND ` + like(`c.COLUMN_NAME`, `@name`)),
			always(`UNION ALL`),
			always(`SELECT '', w.SCHEMA_NAME, w.VIEW_NAME, w.COLUMN_NAME, w.POSITION`),
			always(`, ` + hanaType("w")),
			always(`, ` + yes(`w.IS_NULLABLE`)),
			always(`, w.DEFAULT_VALUE`),
			always(`, FALSE`),
			always(`, ''`),
			always(`, ''`),
			always(`, w.COMMENTS`),
			always(`FROM SYS.VIEW_COLUMNS w`),
			always(`WHERE ` + notSystem(`w.SCHEMA_NAME`)),
			always(`AND ` + like(`w.SCHEMA_NAME`, `@schema`)),
			always(`AND ` + like(`w.VIEW_NAME`, `@parent`)),
			always(`AND ` + like(`w.COLUMN_NAME`, `@name`)),
			always(`ORDER BY 2, 3, 5`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "always empty: a connection reaches one tenant database"},
			{Name: "schema"}, {Name: "table"}, {Name: "name"},
			{Name: "ordinal", Desc: "from POSITION, which counts from one"},
			{Name: "data_type", Desc: "assembled from the type name, length and scale, which HANA stores in three columns"},
			{Name: "nullable"},
			{Name: "default", Desc: "the value alone, without the word DEFAULT"},
			{Name: "primary_key", Desc: "always false for a view column: HANA records no key on a view"},
			{Name: "identity", Desc: "always or by default, and empty when the column is not an identity"},
			{Name: "generated", Desc: "stored for a GENERATED ALWAYS AS column, and empty otherwise"},
			{Name: "comment"},
		},
		Params: []dbmeta.Param{
			{Name: "schema", Desc: "schema name pattern, empty for every schema", Default: ""},
			{Name: "parent", Desc: "table or view name pattern, empty for every one", Default: ""},
			{Name: "name", Desc: "column name pattern, empty for every column", Default: ""},
			{Name: "with_system", Desc: "include the schemas SAP HANA keeps for itself", Default: false},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Column, error) {
			var v dbmeta.Column
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Table, &v.Name, &v.Ordinal,
				&v.DataType, &v.Nullable, &v.Default, &v.PrimaryKey, &v.Identity,
				&v.Generated, &v.Comment)
			return v, err
		},
	})
}

func registerIndexes() {
	// \di. HANA names the constraint that owns an index in the CONSTRAINT
	// column, which is NULL for an index somebody created.
	dbmeta.Indexes.Register(dbmeta.HANA, &dbmeta.Binding[dbmeta.Index]{
		Stmt: dbmeta.Stmt{
			always(`SELECT '' AS "catalog"`),
			always(`, i.SCHEMA_NAME AS "schema"`),
			always(`, i.TABLE_NAME AS "table"`),
			always(`, i.INDEX_NAME AS "name"`),
			always(`, LOWER(i.INDEX_TYPE) AS "type"`),
			always(`, CASE WHEN i.INDEX_TYPE LIKE '%UNIQUE%' THEN TRUE ELSE FALSE END AS "unique"`),
			always(`, CASE WHEN i.CONSTRAINT = 'PRIMARY KEY' THEN TRUE ELSE FALSE END AS "primary"`),
			always(`, CAST(NULL AS NVARCHAR(1)) AS "comment"`),
			always(`FROM SYS.INDEXES i`),
			always(`WHERE ` + notSystem(`i.SCHEMA_NAME`)),
			always(`AND ` + like(`i.SCHEMA_NAME`, `@schema`)),
			always(`AND ` + like(`i.TABLE_NAME`, `@parent`)),
			always(`AND ` + like(`i.INDEX_NAME`, `@name`)),
			always(`ORDER BY i.SCHEMA_NAME, i.TABLE_NAME, i.INDEX_NAME`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "always empty: a connection reaches one tenant database"},
			{Name: "schema"}, {Name: "table"}, {Name: "name"},
			{Name: "type", Desc: "the HANA index kind, lower cased, such as btree, cpbtree or inverted value"},
			{Name: "unique", Desc: "from the index kind, which spells uniqueness into the name rather than carrying a flag"},
			{Name: "primary", Desc: "true when a PRIMARY KEY constraint owns this index"},
			{Name: "comment", Desc: "always absent: COMMENT ON has no index form"},
		},
		Params: parentAndName("index"),
		Scan: func(rows *sql.Rows) (dbmeta.Index, error) {
			var v dbmeta.Index
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Table, &v.Name, &v.Type,
				&v.Unique, &v.Primary, &v.Comment)
			return v, err
		},
	})

	dbmeta.IndexColumns.Register(dbmeta.HANA, &dbmeta.Binding[dbmeta.IndexColumn]{
		Stmt: dbmeta.Stmt{
			always(`SELECT c.SCHEMA_NAME AS "schema"`),
			always(`, c.TABLE_NAME AS "table"`),
			always(`, c.INDEX_NAME AS "index"`),
			always(`, c.COLUMN_NAME AS "name"`),
			always(`, c.POSITION AS "ordinal"`),
			always(`, CAST(NULL AS NVARCHAR(1)) AS "expression"`),
			always(`, ` + no(`c.ASCENDING_ORDER`) + ` AS "descending"`),
			always(`FROM SYS.INDEX_COLUMNS c`),
			always(`WHERE ` + notSystem(`c.SCHEMA_NAME`)),
			always(`AND ` + like(`c.SCHEMA_NAME`, `@schema`)),
			always(`AND ` + like(`c.TABLE_NAME`, `@parent`)),
			always(`AND ` + like(`c.INDEX_NAME`, `@name`)),
			always(`ORDER BY c.SCHEMA_NAME, c.TABLE_NAME, c.INDEX_NAME, c.POSITION`),
		},
		Fields: []dbmeta.Field{
			{Name: "schema"}, {Name: "table"}, {Name: "index"}, {Name: "name"},
			{Name: "ordinal", Desc: "from POSITION, which counts from one"},
			{Name: "expression", Desc: "always absent: HANA indexes columns and has no expression index"},
			{Name: "descending", Desc: "from ASCENDING_ORDER, inverted"},
		},
		Params: parentAndName("index"),
		Scan: func(rows *sql.Rows) (dbmeta.IndexColumn, error) {
			var v dbmeta.IndexColumn
			err := rows.Scan(&v.Schema, &v.Table, &v.Index, &v.Name, &v.Ordinal,
				&v.Expression, &v.Descending)
			return v, err
		},
	})
}

// constraintKind reads the kind from the two flags and the check text, which
// is how SYS.CONSTRAINTS records it. There is no type column.
//
// A primary key reports both flags, so the order matters. A check reports
// neither and carries the condition instead, and its COLUMN_NAME is NULL,
// because a HANA check belongs to the table rather than to a column.
const constraintKind = `CASE WHEN k.IS_PRIMARY_KEY = 'TRUE' THEN 'primary key'` +
	` WHEN k.CHECK_CONDITION IS NOT NULL THEN 'check'` +
	` WHEN k.IS_UNIQUE_KEY = 'TRUE' THEN 'unique'` +
	` ELSE 'not null' END`

func registerConstraints() {
	// The constraints on a table. SYS.CONSTRAINTS holds one row per column,
	// so this groups them, and a foreign key is not in it at all: those are
	// in SYS.REFERENTIAL_CONSTRAINTS and arrive as the second arm.
	dbmeta.Constraints.Register(dbmeta.HANA, &dbmeta.Binding[dbmeta.Constraint]{
		Stmt: dbmeta.Stmt{
			always(`SELECT k.SCHEMA_NAME AS "schema"`),
			always(`, k.TABLE_NAME AS "table"`),
			always(`, k.CONSTRAINT_NAME AS "name"`),
			always(`, ` + constraintKind + ` AS "type"`),
			always(`, k.CHECK_CONDITION AS "definition"`),
			always(`, FALSE AS "deferrable"`),
			always(`, FALSE AS "deferred"`),
			always(`, CAST(NULL AS NVARCHAR(1)) AS "comment"`),
			always(`FROM SYS.CONSTRAINTS k`),
			always(`WHERE ` + notSystem(`k.SCHEMA_NAME`)),
			always(`AND ` + like(`k.SCHEMA_NAME`, `@schema`)),
			always(`AND ` + like(`k.TABLE_NAME`, `@parent`)),
			always(`AND ` + like(`k.CONSTRAINT_NAME`, `@name`)),
			// One row per column becomes one row per constraint. The check
			// text is grouped by rather than aggregated, because it is the
			// same string on every row of one constraint.
			always(`GROUP BY k.SCHEMA_NAME, k.TABLE_NAME, k.CONSTRAINT_NAME` +
				`, k.IS_PRIMARY_KEY, k.IS_UNIQUE_KEY, k.CHECK_CONDITION`),
			always(`UNION ALL`),
			always(`SELECT r.SCHEMA_NAME, r.TABLE_NAME, r.CONSTRAINT_NAME`),
			always(`, 'foreign key'`),
			always(`, CAST(NULL AS NVARCHAR(5000))`),
			always(`, FALSE, FALSE`),
			always(`, CAST(NULL AS NVARCHAR(1))`),
			always(`FROM SYS.REFERENTIAL_CONSTRAINTS r`),
			always(`WHERE ` + notSystem(`r.SCHEMA_NAME`)),
			always(`AND ` + like(`r.SCHEMA_NAME`, `@schema`)),
			always(`AND ` + like(`r.TABLE_NAME`, `@parent`)),
			always(`AND ` + like(`r.CONSTRAINT_NAME`, `@name`)),
			always(`GROUP BY r.SCHEMA_NAME, r.TABLE_NAME, r.CONSTRAINT_NAME`),
			always(`ORDER BY 1, 2, 3`),
		},
		Fields: []dbmeta.Field{
			{Name: "schema"}, {Name: "table"},
			{Name: "name", Desc: "a primary key declared inline gets a generated name such as _SYS_TREE_CS_#165387_#0_#P1, because HANA names it rather than the statement"},
			{Name: "type", Desc: "primary key, foreign key, unique or check, derived from two flags and the check text. SYS.CONSTRAINTS has no type column"},
			{Name: "definition", Desc: "the CHECK text, and absent for every other kind"},
			{Name: "deferrable", Desc: "always false: HANA defers no constraint"},
			{Name: "deferred", Desc: "always false, for the same reason"},
			{Name: "comment", Desc: "always absent: COMMENT ON has no constraint form"},
		},
		Params: parentAndName("constraint"),
		Scan: func(rows *sql.Rows) (dbmeta.Constraint, error) {
			var v dbmeta.Constraint
			err := rows.Scan(&v.Schema, &v.Table, &v.Name, &v.Type, &v.Definition,
				&v.Deferrable, &v.Deferred, &v.Comment)
			return v, err
		},
	})

	// The columns of each constraint, and the column each foreign key points
	// at. HANA records the target on the row itself, so the foreign key arm
	// needs no join at all, which is unusual: every other model here reaches
	// it through the referenced constraint.
	dbmeta.ConstraintColumns.Register(dbmeta.HANA, &dbmeta.Binding[dbmeta.ConstraintColumn]{
		Stmt: dbmeta.Stmt{
			always(`SELECT '' AS "catalog"`),
			always(`, k.SCHEMA_NAME AS "schema"`),
			always(`, k.TABLE_NAME AS "table"`),
			always(`, k.CONSTRAINT_NAME AS "constraint"`),
			always(`, k.COLUMN_NAME AS "name"`),
			always(`, k.POSITION AS "ordinal"`),
			always(`, CAST(NULL AS NVARCHAR(1)) AS "foreign_catalog"`),
			always(`, CAST(NULL AS NVARCHAR(256)) AS "foreign_schema"`),
			always(`, CAST(NULL AS NVARCHAR(256)) AS "foreign_table"`),
			always(`, CAST(NULL AS NVARCHAR(256)) AS "foreign_name"`),
			always(`FROM SYS.CONSTRAINTS k`),
			// A check names no column: HANA records the condition on the
			// table and leaves COLUMN_NAME and POSITION NULL. Those rows
			// would arrive here as a column with no name.
			always(`WHERE k.COLUMN_NAME IS NOT NULL`),
			always(`AND ` + notSystem(`k.SCHEMA_NAME`)),
			always(`AND ` + like(`k.SCHEMA_NAME`, `@schema`)),
			always(`AND ` + like(`k.TABLE_NAME`, `@parent`)),
			always(`AND ` + like(`k.CONSTRAINT_NAME`, `@name`)),
			always(`UNION ALL`),
			always(`SELECT '', r.SCHEMA_NAME, r.TABLE_NAME, r.CONSTRAINT_NAME`),
			always(`, r.COLUMN_NAME, r.POSITION`),
			always(`, '', r.REFERENCED_SCHEMA_NAME`),
			always(`, r.REFERENCED_TABLE_NAME, r.REFERENCED_COLUMN_NAME`),
			always(`FROM SYS.REFERENTIAL_CONSTRAINTS r`),
			always(`WHERE ` + notSystem(`r.SCHEMA_NAME`)),
			always(`AND ` + like(`r.SCHEMA_NAME`, `@schema`)),
			always(`AND ` + like(`r.TABLE_NAME`, `@parent`)),
			always(`AND ` + like(`r.CONSTRAINT_NAME`, `@name`)),
			always(`ORDER BY 2, 3, 4, 6`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "always empty: a connection reaches one tenant database"},
			{Name: "schema"}, {Name: "table"}, {Name: "constraint"}, {Name: "name"},
			{Name: "ordinal", Desc: "from POSITION, which counts from one"},
			{Name: "foreign_catalog", Desc: "empty for a foreign key and absent for every other kind"},
			{Name: "foreign_schema", Desc: "absent unless this is a foreign key"},
			{Name: "foreign_table", Desc: "absent unless this is a foreign key"},
			{Name: "foreign_name", Desc: "absent unless this is a foreign key"},
		},
		Params: parentAndName("constraint"),
		Scan: func(rows *sql.Rows) (dbmeta.ConstraintColumn, error) {
			var v dbmeta.ConstraintColumn
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Table, &v.Constraint, &v.Name,
				&v.Ordinal, &v.ForeignCatalog, &v.ForeignSchema, &v.ForeignTable, &v.ForeignName)
			return v, err
		},
	})
}

func registerTriggers() {
	dbmeta.Triggers.Register(dbmeta.HANA, &dbmeta.Binding[dbmeta.Trigger]{
		Stmt: dbmeta.Stmt{
			always(`SELECT t.SUBJECT_TABLE_SCHEMA AS "schema"`),
			always(`, t.SUBJECT_TABLE_NAME AS "table"`),
			always(`, t.TRIGGER_NAME AS "name"`),
			always(`, CASE WHEN t.IS_ENABLED = 'TRUE' THEN 'enabled'` +
				` ELSE 'disabled' END AS "enabled"`),
			always(`, t.DEFINITION AS "definition"`),
			always(`, CAST(NULL AS NVARCHAR(1)) AS "comment"`),
			always(`FROM SYS.TRIGGERS t`),
			always(`WHERE ` + notSystem(`t.SUBJECT_TABLE_SCHEMA`)),
			always(`AND ` + like(`t.SUBJECT_TABLE_SCHEMA`, `@schema`)),
			always(`AND ` + like(`t.SUBJECT_TABLE_NAME`, `@parent`)),
			always(`AND ` + like(`t.TRIGGER_NAME`, `@name`)),
			always(`ORDER BY t.SUBJECT_TABLE_SCHEMA, t.SUBJECT_TABLE_NAME, t.TRIGGER_NAME`),
		},
		Fields: []dbmeta.Field{
			{Name: "schema", Desc: "the schema of the table the trigger is on, which is what a caller groups by. A HANA trigger has a schema of its own and it is always the table's"},
			{Name: "table"}, {Name: "name"},
			{Name: "enabled", Desc: "from IS_ENABLED"},
			{Name: "definition", Desc: "the CREATE TRIGGER text, which HANA stores in full"},
			{Name: "comment", Desc: "always absent: COMMENT ON has no trigger form"},
		},
		Params: parentAndName("trigger"),
		Scan: func(rows *sql.Rows) (dbmeta.Trigger, error) {
			var v dbmeta.Trigger
			err := rows.Scan(&v.Schema, &v.Table, &v.Name, &v.Enabled, &v.Definition, &v.Comment)
			return v, err
		},
	})
}
