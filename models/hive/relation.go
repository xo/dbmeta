package hive

import (
	"database/sql"
	"strconv"

	"github.com/xo/dbmeta"
)

// tableColumns joins a table to its columns. Hive reaches them through two
// hops that exist for sharing rather than for description: a table names a
// storage descriptor, and the descriptor names a column descriptor, which is
// what the columns hang off. A view has the same chain.
const tableColumns = `JOIN sys.SDS s ON s.SD_ID = t.SD_ID` +
	` JOIN sys.COLUMNS_V2 c ON c.CD_ID = s.CD_ID`

// tableComment reads the comment, which Hive keeps as a table property
// rather than as a column of its own. One bounded read per row: a table has
// a handful of properties whatever the size of the metastore.
const tableComment = `(SELECT p.PARAM_VALUE FROM sys.TABLE_PARAMS p` +
	` WHERE p.TBL_ID = t.TBL_ID AND p.PARAM_KEY = 'comment')`

func registerRelations() {
	registerTables()
	registerColumns()
	registerConstraints()
}

func registerTables() {
	// \dn. A Hive database is the only namespace there is, so it is the
	// schema. There is no level above it in the metastore this reads.
	dbmeta.Schemas.Register(dbmeta.Hive, &dbmeta.Binding[dbmeta.Schema]{
		Stmt: dbmeta.Stmt{
			always(`SELECT '' AS "catalog"`),
			always(`, d.NAME AS "name"`),
			always(`, d.OWNER_NAME AS "owner"`),
			always(`, (SELECT p.PARAM_VALUE FROM sys.DATABASE_PARAMS p` +
				` WHERE p.DB_ID = d.DB_ID AND p.PARAM_KEY = 'comment') AS "comment"`),
			always(`FROM sys.DBS d`),
			always(`WHERE ` + notSystem),
			always(`AND ` + like(`d.NAME`, `@name`)),
			always(`ORDER BY d.NAME`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "always empty: the metastore this reads has no level above a database"},
			{Name: "name", Desc: "the Hive database, which is the only namespace Hive has"},
			{Name: "owner", Desc: "from DBS.OWNER_NAME"},
			{Name: "comment", Desc: "from the database property named comment, which is where Hive keeps one"},
		},
		Params: []dbmeta.Param{
			{Name: "name", Desc: "database name pattern, empty for every database", Default: ""},
			{Name: "with_system", Desc: "include sys and information_schema, which Hive keeps for itself", Default: false},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Schema, error) {
			var v dbmeta.Schema
			err := rows.Scan(&v.Catalog, &v.Name, &v.Owner, &v.Comment)
			return v, err
		},
	})

	// \dt and \dv. TBLS holds both and TBL_TYPE separates them.
	dbmeta.Tables.Register(dbmeta.Hive, &dbmeta.Binding[dbmeta.Table]{
		Stmt: dbmeta.Stmt{
			always(`SELECT '' AS "catalog"`),
			always(`, d.NAME AS "schema"`),
			always(`, t.TBL_NAME AS "name"`),
			always(`, CASE t.TBL_TYPE` +
				` WHEN 'MANAGED_TABLE' THEN 'table'` +
				` WHEN 'EXTERNAL_TABLE' THEN 'external table'` +
				` WHEN 'VIRTUAL_VIEW' THEN 'view'` +
				` WHEN 'MATERIALIZED_VIEW' THEN 'materialized view'` +
				` ELSE LOWER(t.TBL_TYPE) END AS "type"`),
			always(`, ` + tableComment + ` AS "comment"`),
			always(`FROM sys.TBLS t JOIN sys.DBS d ON d.DB_ID = t.DB_ID`),
			always(`WHERE ` + notSystem),
			always(`AND ` + like(`d.NAME`, `@schema`)),
			always(`AND ` + like(`t.TBL_NAME`, `@name`)),
			always(`ORDER BY d.NAME, t.TBL_NAME`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "always empty: the metastore has no level above a database"},
			{Name: "schema"}, {Name: "name"},
			{Name: "type", Desc: "table, external table, view or materialized view. Hive records which in TBL_TYPE and an external table is a first class kind here"},
			{Name: "comment", Desc: "from the table property named comment"},
		},
		Params: schemaAndName("table"),
		Scan: func(rows *sql.Rows) (dbmeta.Table, error) {
			var v dbmeta.Table
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Name, &v.Type, &v.Comment)
			return v, err
		},
	})

	// \dv in full. Hive keeps two texts for a view: the one that was typed
	// and the one with names resolved. The expanded one is what a reader
	// wants, and the original is the fallback for a view that has none.
	dbmeta.Views.Register(dbmeta.Hive, &dbmeta.Binding[dbmeta.View]{
		Stmt: dbmeta.Stmt{
			always(`SELECT '' AS "catalog"`),
			always(`, d.NAME AS "schema"`),
			always(`, t.TBL_NAME AS "name"`),
			always(`, COALESCE(t.VIEW_EXPANDED_TEXT, t.VIEW_ORIGINAL_TEXT, '') AS "definition"`),
			always(`, CAST(NULL AS string) AS "check_option"`),
			always(`, FALSE AS "updatable"`),
			always(`, FALSE AS "insertable"`),
			always(`, ` + tableComment + ` AS "comment"`),
			always(`FROM sys.TBLS t JOIN sys.DBS d ON d.DB_ID = t.DB_ID`),
			always(`WHERE t.TBL_TYPE IN ('VIRTUAL_VIEW', 'MATERIALIZED_VIEW')`),
			always(`AND ` + notSystem),
			always(`AND ` + like(`d.NAME`, `@schema`)),
			always(`AND ` + like(`t.TBL_NAME`, `@name`)),
			always(`ORDER BY d.NAME, t.TBL_NAME`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "always empty: the metastore has no level above a database"},
			{Name: "schema"}, {Name: "name"},
			{Name: "definition", Desc: "the expanded text, with names resolved, and the original text where there is no expanded one"},
			{Name: "check_option", Desc: "always absent: Hive has no WITH CHECK OPTION"},
			{Name: "updatable", Desc: "always false: a Hive view is read only"},
			{Name: "insertable", Desc: "always false, for the same reason"},
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

	// \dP. A Hive partition column is not a column of the table: it is in
	// PARTITION_KEYS and it does not appear in COLUMNS_V2 at all.
	dbmeta.PartitionedTables.Register(dbmeta.Hive, &dbmeta.Binding[dbmeta.PartitionedTable]{
		Stmt: dbmeta.Stmt{
			always(`SELECT d.NAME AS "schema"`),
			always(`, t.TBL_NAME AS "name"`),
			always(`, t.OWNER AS "owner"`),
			always(`, 'table' AS "type"`),
			always(`, '' AS "parent"`),
			always(`, 'list' AS "strategy"`),
			always(`, k.PKEY_NAME AS "expression"`),
			always(`, k.PKEY_COMMENT AS "comment"`),
			always(`FROM sys.PARTITION_KEYS k`),
			always(`JOIN sys.TBLS t ON t.TBL_ID = k.TBL_ID`),
			always(`JOIN sys.DBS d ON d.DB_ID = t.DB_ID`),
			always(`WHERE ` + notSystem),
			always(`AND ` + like(`d.NAME`, `@schema`)),
			always(`AND ` + like(`t.TBL_NAME`, `@name`)),
			always(`ORDER BY d.NAME, t.TBL_NAME, k.INTEGER_IDX`),
		},
		Fields: []dbmeta.Field{
			{Name: "schema"}, {Name: "name"},
			{Name: "owner", Desc: "from TBLS.OWNER"},
			{Name: "type", Desc: "always table: Hive partitions a table and nothing else"},
			{Name: "parent", Desc: "always empty: a Hive partition is a directory rather than a table, so there is no child relation to name a parent for"},
			{Name: "strategy", Desc: "always list: Hive partitions by the value of a column and has no range or hash form"},
			{Name: "expression", Desc: "the partition column. A table partitioned by two columns has two rows here, in declaration order, because a partition key is not a column of the table and does not appear in Columns"},
			{Name: "comment", Desc: "the comment on the partition key, which Hive records separately from a column comment"},
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

// schemaAndName is the filter set most queries here take.
func schemaAndName(kind string) []dbmeta.Param {
	return []dbmeta.Param{
		{Name: "schema", Desc: "database name pattern, empty for every database", Default: ""},
		{Name: "name", Desc: kind + " name pattern, empty for every one", Default: ""},
		{Name: "with_system", Desc: "include sys and information_schema", Default: false},
	}
}

// parentAndName is the filter set for a child of a table.
func parentAndName(kind string) []dbmeta.Param {
	return []dbmeta.Param{
		{Name: "schema", Desc: "database name pattern, empty for every database", Default: ""},
		{Name: "parent", Desc: "table name pattern, empty for every table", Default: ""},
		{Name: "name", Desc: kind + " name pattern, empty for every one", Default: ""},
		{Name: "with_system", Desc: "include sys and information_schema", Default: false},
	}
}

func registerColumns() {
	// The columns of a table or a view. A partition key is not here: Hive
	// keeps those in PARTITION_KEYS and PartitionedTables reads them.
	dbmeta.Columns.Register(dbmeta.Hive, &dbmeta.Binding[dbmeta.Column]{
		Stmt: dbmeta.Stmt{
			always(`SELECT '' AS "catalog"`),
			always(`, d.NAME AS "schema"`),
			always(`, t.TBL_NAME AS "table"`),
			always(`, c.COLUMN_NAME AS "name"`),
			always(`, c.INTEGER_IDX + 1 AS "ordinal"`),
			always(`, c.TYPE_NAME AS "data_type"`),
			// Hive records nullability, a default and a key as
			// constraints rather than as properties of the column, so
			// each is a join rather than a column read. They are joins
			// rather than correlated subqueries because Hive does not
			// take a correlated scalar subquery in a select list.
			always(`, nn.CONSTRAINT_NAME IS NULL AS "nullable"`),
			always(`, df.DEFAULT_VALUE AS "default"`),
			always(`, pk.CONSTRAINT_NAME IS NOT NULL AS "primary_key"`),
			always(`, '' AS "identity"`),
			always(`, '' AS "generated"`),
			always(`, c.COMMENT AS "comment"`),
			always(`FROM sys.TBLS t JOIN sys.DBS d ON d.DB_ID = t.DB_ID ` + tableColumns),
			always(`LEFT JOIN ` + constraintOn(`nn`, 3)),
			always(`LEFT JOIN ` + constraintOn(`df`, 4)),
			always(`LEFT JOIN ` + constraintOn(`pk`, 0)),
			always(`WHERE ` + notSystem),
			always(`AND ` + like(`d.NAME`, `@schema`)),
			always(`AND ` + like(`t.TBL_NAME`, `@parent`)),
			always(`AND ` + like(`c.COLUMN_NAME`, `@name`)),
			always(`ORDER BY d.NAME, t.TBL_NAME, c.INTEGER_IDX`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "always empty: the metastore has no level above a database"},
			{Name: "schema"}, {Name: "table"}, {Name: "name"},
			{Name: "ordinal", Desc: "from INTEGER_IDX, which counts from zero, plus one"},
			{Name: "data_type", Desc: "the Hive type as written, including a nested one such as array<string> or struct<a:int>"},
			{Name: "nullable", Desc: "false only where a NOT NULL constraint names the column. Hive records nullability as a constraint rather than on the column, and a column with no constraint is nullable"},
			{Name: "default", Desc: "from the DEFAULT constraint, and absent where there is none"},
			{Name: "primary_key"},
			{Name: "identity", Desc: "always empty: Hive has no identity column"},
			{Name: "generated", Desc: "always empty: Hive has no generated column"},
			{Name: "comment", Desc: "from COLUMNS_V2.COMMENT, which is the one comment Hive stores on the row itself rather than as a property"},
		},
		Params: []dbmeta.Param{
			{Name: "schema", Desc: "database name pattern, empty for every database", Default: ""},
			{Name: "parent", Desc: "table name pattern, empty for every table", Default: ""},
			{Name: "name", Desc: "column name pattern, empty for every column", Default: ""},
			{Name: "with_system", Desc: "include sys and information_schema", Default: false},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Column, error) {
			var v dbmeta.Column
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Table, &v.Name, &v.Ordinal,
				&v.DataType, &v.Nullable, &v.Default, &v.PrimaryKey, &v.Identity,
				&v.Generated, &v.Comment)
			return v, err
		},
	})

	// \ss. Hive gathers these with ANALYZE TABLE and a column that has
	// never been analyzed has no row, which is what this kind says.
	dbmeta.ColumnStats.Register(dbmeta.Hive, &dbmeta.Binding[dbmeta.ColumnStat]{
		Stmt: dbmeta.Stmt{
			always(`SELECT '' AS "catalog"`),
			always(`, d.NAME AS "schema"`),
			always(`, t.TBL_NAME AS "table"`),
			always(`, cs.COLUMN_NAME AS "name"`),
			always(`, CAST(cs.AVG_COL_LEN AS bigint) AS "avg_width"`),
			always(`, CAST(NULL AS double) AS "null_frac"`),
			always(`, CAST(cs.NUM_DISTINCTS AS double) AS "distinct"`),
			always(`, CAST(cs.LONG_LOW_VALUE AS string) AS "min"`),
			always(`, CAST(cs.LONG_HIGH_VALUE AS string) AS "max"`),
			always(`, CAST(NULL AS string) AS "mean"`),
			always(`, CAST(NULL AS string) AS "top_n"`),
			always(`, CAST(NULL AS string) AS "top_n_freqs"`),
			always(`FROM sys.TAB_COL_STATS cs`),
			always(`JOIN sys.TBLS t ON t.TBL_ID = cs.TBL_ID`),
			always(`JOIN sys.DBS d ON d.DB_ID = t.DB_ID`),
			always(`WHERE ` + notSystem),
			always(`AND ` + like(`d.NAME`, `@schema`)),
			always(`AND ` + like(`t.TBL_NAME`, `@parent`)),
			always(`AND ` + like(`cs.COLUMN_NAME`, `@name`)),
			always(`ORDER BY d.NAME, t.TBL_NAME, cs.COLUMN_NAME`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "always empty: the metastore has no level above a database"},
			{Name: "schema"}, {Name: "table"}, {Name: "name"},
			{Name: "avg_width", Desc: "from AVG_COL_LEN, which Hive records for a string column and leaves absent for a fixed width one"},
			{Name: "null_frac", Desc: "always absent: Hive counts nulls in NUM_NULLS and records no row count beside it, so the fraction cannot be formed from this row alone"},
			{Name: "distinct", Desc: "from NUM_DISTINCTS, a count rather than a fraction, and an estimate rather than exact"},
			{Name: "min", Desc: "the low value, for an integer column. Hive keeps a separate low value per type family and this reads the integer one, so a string or decimal column reports none"},
			{Name: "max", Desc: "the high value, on the same terms as min"},
			{Name: "mean", Desc: "always absent: Hive computes no mean"},
			{Name: "top_n", Desc: "always absent: Hive keeps no most common value list"},
			{Name: "top_n_freqs", Desc: "always absent, for the same reason"},
		},
		Params: []dbmeta.Param{
			{Name: "schema", Desc: "database name pattern, empty for every database", Default: ""},
			{Name: "parent", Desc: "table name pattern, empty for every table", Default: ""},
			{Name: "name", Desc: "column name pattern. Only a column that ANALYZE TABLE has reached appears", Default: ""},
			{Name: "with_system", Desc: "include sys and information_schema", Default: false},
		},
		Scan: func(rows *sql.Rows) (dbmeta.ColumnStat, error) {
			var v dbmeta.ColumnStat
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Table, &v.Name, &v.AvgWidth,
				&v.NullFrac, &v.Distinct, &v.Min, &v.Max, &v.Mean, &v.TopN, &v.TopNFreqs)
			return v, err
		},
	})
}

// constraintKind turns Hive's numeric constraint type into the word this
// project uses. The numbers are the metastore's own and there is no name
// column beside them.
const constraintKind = `CASE k.CONSTRAINT_TYPE` +
	` WHEN 0 THEN 'primary key' WHEN 1 THEN 'foreign key'` +
	` WHEN 2 THEN 'unique' WHEN 3 THEN 'not null'` +
	` WHEN 4 THEN 'default' WHEN 5 THEN 'check'` +
	` ELSE CAST(k.CONSTRAINT_TYPE AS string) END`

// keyConstraints normalizes KEY_CONSTRAINTS so that one set of columns names
// the table a constraint is on, whatever kind it is.
//
// Hive stores the two sides the other way round from how the names read, and
// only a foreign key has both. Measured on 4.2.1, with tables 115 and 116:
//
//	kp_pk type=0  child(tbl=0)    parent(tbl=115)
//	kp_fk type=1  child(tbl=116)  parent(tbl=115)
//
// So for a foreign key the child is the table that has the constraint and
// the parent is the table it points at. For every other kind the parent is
// the table that has it and the child columns are zero rather than null,
// which is worse: joining on them silently finds nothing instead of
// dropping the row.
//
// Reading child as "the table this is on" is the obvious mistake and it was
// made here first. It leaves every primary key, unique, not null and default
// invisible while foreign keys work, so the queries look right on a schema
// that has foreign keys in it.
const keyConstraints = `(SELECT CONSTRAINT_NAME, CONSTRAINT_TYPE, POSITION, DEFAULT_VALUE` +
	`, CASE WHEN CONSTRAINT_TYPE = 1 THEN CHILD_TBL_ID ELSE PARENT_TBL_ID END AS OWNER_TBL_ID` +
	`, CASE WHEN CONSTRAINT_TYPE = 1 THEN CHILD_CD_ID ELSE PARENT_CD_ID END AS OWNER_CD_ID` +
	`, CASE WHEN CONSTRAINT_TYPE = 1 THEN CHILD_INTEGER_IDX ELSE PARENT_INTEGER_IDX END AS OWNER_IDX` +
	`, CASE WHEN CONSTRAINT_TYPE = 1 THEN PARENT_TBL_ID END AS REF_TBL_ID` +
	`, CASE WHEN CONSTRAINT_TYPE = 1 THEN PARENT_CD_ID END AS REF_CD_ID` +
	`, CASE WHEN CONSTRAINT_TYPE = 1 THEN PARENT_INTEGER_IDX END AS REF_IDX` +
	` FROM sys.KEY_CONSTRAINTS) k`

func registerConstraints() {
	// Hive 3.0 added constraints and none of them is enforced: they are
	// declarations a planner may use. KEY_CONSTRAINTS holds one row per
	// column, so this groups them.
	dbmeta.Constraints.Register(dbmeta.Hive, &dbmeta.Binding[dbmeta.Constraint]{
		Stmt: dbmeta.Stmt{
			always(`SELECT d.NAME AS "schema"`),
			always(`, t.TBL_NAME AS "table"`),
			always(`, k.CONSTRAINT_NAME AS "name"`),
			always(`, ` + constraintKind + ` AS "type"`),
			always(`, CAST(NULL AS string) AS "definition"`),
			always(`, FALSE AS "deferrable"`),
			always(`, FALSE AS "deferred"`),
			always(`, CAST(NULL AS string) AS "comment"`),
			always(`FROM ` + keyConstraints),
			always(`JOIN sys.TBLS t ON t.TBL_ID = k.OWNER_TBL_ID`),
			always(`JOIN sys.DBS d ON d.DB_ID = t.DB_ID`),
			always(`WHERE ` + notSystem),
			always(`AND ` + like(`d.NAME`, `@schema`)),
			always(`AND ` + like(`t.TBL_NAME`, `@parent`)),
			always(`AND ` + like(`k.CONSTRAINT_NAME`, `@name`)),
			always(`GROUP BY d.NAME, t.TBL_NAME, k.CONSTRAINT_NAME, k.CONSTRAINT_TYPE`),
			always(`ORDER BY d.NAME, t.TBL_NAME, k.CONSTRAINT_NAME`),
		},
		Fields: []dbmeta.Field{
			{Name: "schema"}, {Name: "table"}, {Name: "name"},
			{Name: "type", Desc: "primary key, foreign key, unique, not null, default or check, from the numeric CONSTRAINT_TYPE. Hive records NOT NULL and DEFAULT as constraints where PostgreSQL records them on the column"},
			{Name: "definition", Desc: "always absent: Hive stores no expression for a check constraint in the metastore"},
			{Name: "deferrable", Desc: "always false. No Hive constraint is enforced at all, so there is nothing to defer"},
			{Name: "deferred", Desc: "always false, for the same reason"},
			{Name: "comment", Desc: "always absent: Hive records no comment on a constraint"},
		},
		Params: parentAndName("constraint"),
		Scan: func(rows *sql.Rows) (dbmeta.Constraint, error) {
			var v dbmeta.Constraint
			err := rows.Scan(&v.Schema, &v.Table, &v.Name, &v.Type, &v.Definition,
				&v.Deferrable, &v.Deferred, &v.Comment)
			return v, err
		},
	})

	// The columns of each constraint. KEY_CONSTRAINTS names a column by
	// its position rather than by its name, so reaching the name is a
	// join back to the column list on both sides of a foreign key.
	dbmeta.ConstraintColumns.Register(dbmeta.Hive, &dbmeta.Binding[dbmeta.ConstraintColumn]{
		Stmt: dbmeta.Stmt{
			always(`SELECT '' AS "catalog"`),
			always(`, d.NAME AS "schema"`),
			always(`, t.TBL_NAME AS "table"`),
			always(`, k.CONSTRAINT_NAME AS "constraint"`),
			always(`, c.COLUMN_NAME AS "name"`),
			always(`, CAST(k.POSITION AS bigint) AS "ordinal"`),
			always(`, CASE WHEN pt.TBL_NAME IS NULL THEN NULL ELSE '' END AS "foreign_catalog"`),
			always(`, pd.NAME AS "foreign_schema"`),
			always(`, pt.TBL_NAME AS "foreign_table"`),
			always(`, pc.COLUMN_NAME AS "foreign_name"`),
			always(`FROM ` + keyConstraints),
			always(`JOIN sys.TBLS t ON t.TBL_ID = k.OWNER_TBL_ID`),
			always(`JOIN sys.DBS d ON d.DB_ID = t.DB_ID`),
			always(`JOIN sys.COLUMNS_V2 c ON c.CD_ID = k.OWNER_CD_ID` +
				` AND c.INTEGER_IDX = k.OWNER_IDX`),
			// The referenced side exists only for a foreign key, so every
			// join to it is an outer one.
			always(`LEFT JOIN sys.TBLS pt ON pt.TBL_ID = k.REF_TBL_ID`),
			always(`LEFT JOIN sys.DBS pd ON pd.DB_ID = pt.DB_ID`),
			always(`LEFT JOIN sys.COLUMNS_V2 pc ON pc.CD_ID = k.REF_CD_ID` +
				` AND pc.INTEGER_IDX = k.REF_IDX`),
			always(`WHERE ` + notSystem),
			always(`AND ` + like(`d.NAME`, `@schema`)),
			always(`AND ` + like(`t.TBL_NAME`, `@parent`)),
			always(`AND ` + like(`k.CONSTRAINT_NAME`, `@name`)),
			always(`ORDER BY d.NAME, t.TBL_NAME, k.CONSTRAINT_NAME, k.POSITION`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "always empty: the metastore has no level above a database"},
			{Name: "schema"}, {Name: "table"}, {Name: "constraint"}, {Name: "name"},
			{Name: "ordinal", Desc: "from POSITION, which counts from one"},
			{Name: "foreign_catalog", Desc: "empty for a foreign key and absent for every other kind"},
			{Name: "foreign_schema", Desc: "absent unless this is a foreign key"},
			{Name: "foreign_table", Desc: "absent unless this is a foreign key"},
			{Name: "foreign_name", Desc: "absent unless this is a foreign key. Hive names the referenced column by its position, so this is a join back to the column list rather than a name the metastore stores"},
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

// constraintOn is an outer join to the constraints of one kind, used where a
// column property is a constraint in Hive rather than a column.
func constraintOn(alias string, kind int) string {
	return `(SELECT CONSTRAINT_NAME, DEFAULT_VALUE` +
		`, CASE WHEN CONSTRAINT_TYPE = 1 THEN CHILD_TBL_ID ELSE PARENT_TBL_ID END AS OWNER_TBL_ID` +
		`, CASE WHEN CONSTRAINT_TYPE = 1 THEN CHILD_INTEGER_IDX ELSE PARENT_INTEGER_IDX END AS OWNER_IDX` +
		` FROM sys.KEY_CONSTRAINTS WHERE CONSTRAINT_TYPE = ` + strconv.Itoa(kind) + `) ` + alias +
		` ON ` + alias + `.OWNER_TBL_ID = t.TBL_ID AND ` + alias + `.OWNER_IDX = c.INTEGER_IDX`
}
