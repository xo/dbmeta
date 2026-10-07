package drill

import (
	"database/sql"

	"github.com/xo/dbmeta"
)

// catalogDesc describes the catalog field of every row.
const catalogDesc = "always DRILL, the one catalog Drill reports"

// ownerDesc describes the owner of a schema.
const ownerDesc = "always empty: a schema has no owner, and SCHEMATA holds the text <owner>, which the query turns into NULL"

// types is the filter on Table.Type. TABLE_TYPE is TABLE, VIEW or
// SYSTEM TABLE, and Table.Type spells each in lower case.
const types = `(@types = '' OR STRPOS(',' || @types || ',', ',' || LOWER(t.TABLE_TYPE) || ',') > 0)`

func scanSchema(rows *sql.Rows) (dbmeta.Schema, error) {
	var v dbmeta.Schema
	err := rows.Scan(&v.Catalog, &v.Name, dbmeta.NullAsEmpty(&v.Owner), &v.Comment)
	return v, err
}

func registerRelations() {
	// SCHEMATA lists a workspace of each file plugin, such as dfs.tmp, and
	// information_schema and sys.
	dbmeta.Schemas.Register(dbmeta.Drill, &dbmeta.Binding[dbmeta.Schema]{
		Stmt: dbmeta.Stmt{
			always("SELECT s.CATALOG_NAME AS `catalog`"),
			always(", s.SCHEMA_NAME AS `name`"),
			always(", NULLIF(s.SCHEMA_OWNER, '<owner>') AS `owner`"),
			always(", NULL AS `comment`"),
			always("FROM INFORMATION_SCHEMA.SCHEMATA s"),
			always("WHERE " + notSystem("s.SCHEMA_NAME")),
			always("AND " + like("s.SCHEMA_NAME", "@name")),
			always("ORDER BY 2"),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: catalogDesc},
			{Name: "name", Desc: "the plugin and the workspace, with the dot, such as dfs.tmp"},
			{Name: "owner", Desc: ownerDesc},
			{Name: "comment", Desc: "always absent: Drill has no COMMENT statement"},
		},
		Params: []dbmeta.Param{
			{Name: "name", Desc: "schema name pattern, empty for every schema", Default: ""},
			system,
		},
		Scan: scanSchema,
	})

	// CURRENT_SCHEMA is the empty string until a request names a default
	// schema, by defaultSchema or by USE. The join then finds no row, which
	// says that the session has no current schema. A row is never made up.
	dbmeta.CurrentSchema.Register(dbmeta.Drill, &dbmeta.Binding[dbmeta.Schema]{
		Stmt: dbmeta.Stmt{
			always("SELECT s.CATALOG_NAME AS `catalog`"),
			always(", s.SCHEMA_NAME AS `name`"),
			always(", NULLIF(s.SCHEMA_OWNER, '<owner>') AS `owner`"),
			always(", NULL AS `comment`"),
			always("FROM INFORMATION_SCHEMA.SCHEMATA s"),
			always("WHERE s.SCHEMA_NAME = CURRENT_SCHEMA"),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: catalogDesc},
			{Name: "name", Desc: "the default schema of the session. There is no row when the session has none, which is the case until the DSN names a schema or the session runs USE"},
			{Name: "owner", Desc: ownerDesc},
			{Name: "comment", Desc: "always absent: Drill has no COMMENT statement"},
		},
		Scan: scanSchema,
	})

	// A file table is listed only when the Metastore is on and ANALYZE TABLE
	// has run for it. A view and a system table are always listed.
	dbmeta.Tables.Register(dbmeta.Drill, &dbmeta.Binding[dbmeta.Table]{
		Stmt: dbmeta.Stmt{
			always("SELECT t.TABLE_CATALOG AS `catalog`"),
			always(", t.TABLE_SCHEMA AS `schema`"),
			always(", t.TABLE_NAME AS `name`"),
			always(", LOWER(t.TABLE_TYPE) AS `type`"),
			always(", NULL AS `comment`"),
			always("FROM INFORMATION_SCHEMA.`TABLES` t"),
			always("WHERE " + notSystem("t.TABLE_SCHEMA")),
			always("AND " + like("t.TABLE_SCHEMA", "@schema")),
			always("AND " + like("t.TABLE_NAME", "@name")),
			always("AND " + types),
			always("ORDER BY 2, 3"),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: catalogDesc},
			{Name: "schema", Desc: "the plugin and the workspace, such as dfs.tmp"},
			{Name: "name"},
			{Name: "type", Desc: "table, view or system table. A table is a file table that the Drill Metastore holds, so it is listed only when the Metastore is on and the table was analyzed"},
			{Name: "comment", Desc: "always absent: Drill has no COMMENT statement"},
		},
		Params: []dbmeta.Param{
			{Name: "schema", Desc: "schema name pattern, empty for every schema", Default: ""},
			{Name: "name", Desc: "table name pattern, empty for every table", Default: ""},
			system,
			dbmeta.TypesParam(),
		},
		Scan: func(rows *sql.Rows) (dbmeta.Table, error) {
			var v dbmeta.Table
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Name, &v.Type, &v.Comment)
			return v, err
		},
	})

	// The columns of a view read DATA_TYPE ANY and IS_NULLABLE YES, because
	// Drill knows no type until the view runs.
	dbmeta.Columns.Register(dbmeta.Drill, &dbmeta.Binding[dbmeta.Column]{
		Stmt: dbmeta.Stmt{
			always("SELECT c.TABLE_CATALOG AS `catalog`"),
			always(", c.TABLE_SCHEMA AS `schema`"),
			always(", c.TABLE_NAME AS `table`"),
			always(", c.COLUMN_NAME AS `name`"),
			always(", c.ORDINAL_POSITION AS `ordinal`"),
			always(", c.DATA_TYPE AS `data_type`"),
			always(", c.IS_NULLABLE = 'YES' AS `nullable`"),
			always(", c.COLUMN_DEFAULT AS `default`"),
			always(", false AS `primary_key`"),
			always(", NULL AS `identity`"),
			always(", NULL AS `generated`"),
			always(", NULL AS `comment`"),
			always(", NULL AS `collation`"),
			always("FROM INFORMATION_SCHEMA.COLUMNS c"),
			always("WHERE " + notSystem("c.TABLE_SCHEMA")),
			always("AND " + like("c.TABLE_SCHEMA", "@schema")),
			always("AND " + like("c.TABLE_NAME", "@parent")),
			always("AND " + like("c.COLUMN_NAME", "@name")),
			always("ORDER BY 2, 3, 5"),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: catalogDesc}, {Name: "schema"}, {Name: "table"}, {Name: "name"},
			{Name: "ordinal", Desc: "the position from 1"},
			{Name: "data_type", Desc: "the SQL type as Drill spells it, such as BIGINT, CHARACTER VARYING or DECIMAL. A column of a view reads ANY"},
			{Name: "nullable", Desc: "false for a column that is NOT NULL. A column of a view is always true, because Drill knows no nullability for it"},
			{Name: "default", Desc: "the default of the column. It is absent for every table that Drill can build, because Drill has no DEFAULT clause"},
			{Name: "primary_key", Desc: "always false: Drill has no primary key"},
			{Name: "identity", Desc: "always absent: Drill has no identity column"},
			{Name: "generated", Desc: "always absent: Drill has no generated column"},
			{Name: "comment", Desc: "always absent: Drill has no COMMENT statement"},
			{Name: "collation", Desc: "always absent: COLUMNS records no collation"},
		},
		Params: []dbmeta.Param{
			{Name: "schema", Desc: "schema name pattern, empty for every schema", Default: ""},
			{Name: "parent", Desc: "table name pattern, empty for every table", Default: ""},
			{Name: "name", Desc: "column name pattern, empty for every column", Default: ""},
			system,
		},
		Scan: func(rows *sql.Rows) (dbmeta.Column, error) {
			var v dbmeta.Column
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Table, &v.Name, &v.Ordinal, &v.DataType,
				&v.Nullable, &v.Default, &v.PrimaryKey, &v.Identity, &v.Generated, &v.Comment, &v.Collation)
			return v, err
		},
	})

	dbmeta.Views.Register(dbmeta.Drill, &dbmeta.Binding[dbmeta.View]{
		Stmt: dbmeta.Stmt{
			always("SELECT v.TABLE_CATALOG AS `catalog`"),
			always(", v.TABLE_SCHEMA AS `schema`"),
			always(", v.TABLE_NAME AS `name`"),
			always(", v.VIEW_DEFINITION AS `definition`"),
			always(", NULL AS `check_option`"),
			always(", false AS `updatable`"),
			always(", false AS `insertable`"),
			always(", NULL AS `comment`"),
			always("FROM INFORMATION_SCHEMA.VIEWS v"),
			always("WHERE " + notSystem("v.TABLE_SCHEMA")),
			always("AND " + like("v.TABLE_SCHEMA", "@schema")),
			always("AND " + like("v.TABLE_NAME", "@name")),
			always("ORDER BY 2, 3"),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: catalogDesc}, {Name: "schema"}, {Name: "name"},
			{Name: "definition", Desc: "the SELECT statement of the view, with its names in backticks"},
			{Name: "check_option", Desc: "always absent: Drill has no WITH CHECK OPTION"},
			{Name: "updatable", Desc: "always false: no Drill view can be updated, because Drill has no UPDATE"},
			{Name: "insertable", Desc: "always false: no Drill view accepts an insert, because Drill has no INSERT"},
			{Name: "comment", Desc: "always absent: Drill has no COMMENT statement"},
		},
		Params: []dbmeta.Param{
			{Name: "schema", Desc: "schema name pattern, empty for every schema", Default: ""},
			{Name: "name", Desc: "view name pattern, empty for every view", Default: ""},
			system,
		},
		Scan: func(rows *sql.Rows) (dbmeta.View, error) {
			var v dbmeta.View
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Name, &v.Definition,
				&v.CheckOption, &v.Updatable, &v.Insertable, &v.Comment)
			return v, err
		},
	})

	// COLUMNS holds the statistics that ANALYZE TABLE collects into the
	// Metastore, and NUM_NULLS is NULL for a column that was never analyzed.
	// The row count is on TABLES. The planner of Drill takes NUM_NULLS as a
	// column that is never NULL and folds IS NOT NULL to true, so the test is
	// a comparison, which is false for NULL.
	dbmeta.ColumnStats.Register(dbmeta.Drill, &dbmeta.Binding[dbmeta.ColumnStat]{
		Stmt: dbmeta.Stmt{
			always("SELECT c.TABLE_CATALOG AS `catalog`"),
			always(", c.TABLE_SCHEMA AS `schema`"),
			always(", c.TABLE_NAME AS `table`"),
			always(", c.COLUMN_NAME AS `name`"),
			always(", NULL AS `avg_width`"),
			always(", CASE WHEN t.NUM_ROWS > 0 THEN CAST(c.NUM_NULLS AS DOUBLE) / t.NUM_ROWS END AS `null_frac`"),
			always(", c.NDV AS `distinct`"),
			always(", c.MIN_VAL AS `min`"),
			always(", c.MAX_VAL AS `max`"),
			always(", NULL AS `mean`"),
			always(", NULL AS `top_n`"),
			always(", NULL AS `top_n_freqs`"),
			always("FROM INFORMATION_SCHEMA.COLUMNS c"),
			always("LEFT JOIN INFORMATION_SCHEMA.`TABLES` t"),
			always("ON t.TABLE_SCHEMA = c.TABLE_SCHEMA AND t.TABLE_NAME = c.TABLE_NAME"),
			always("WHERE c.NUM_NULLS >= 0"),
			always("AND " + notSystem("c.TABLE_SCHEMA")),
			always("AND " + like("c.TABLE_SCHEMA", "@schema")),
			always("AND " + like("c.TABLE_NAME", "@parent")),
			always("AND " + like("c.COLUMN_NAME", "@name")),
			always("ORDER BY 2, 3, 4"),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: catalogDesc}, {Name: "schema"}, {Name: "table"}, {Name: "name"},
			{Name: "avg_width", Desc: "always absent: Drill records the declared width of a column and no average"},
			{Name: "null_frac", Desc: "NUM_NULLS over the row count of the table, from ANALYZE TABLE. It is absent when the row count is zero"},
			{Name: "distinct", Desc: "NDV. It is absent on 1.21.2 and 1.22.0, which collect no distinct count"},
			{Name: "min", Desc: "the smallest value as text. A DATE reads as the milliseconds since 1970-01-01"},
			{Name: "max", Desc: "the largest value as text, in the same form as min"},
			{Name: "mean", Desc: "always absent: Drill records no mean"},
			{Name: "top_n", Desc: "always absent: Drill records no most common values"},
			{Name: "top_n_freqs", Desc: "always absent, for the same reason as top_n"},
		},
		Params: []dbmeta.Param{
			{Name: "schema", Desc: "schema name pattern, empty for every schema", Default: ""},
			{Name: "parent", Desc: "table name pattern, empty for every table", Default: ""},
			{Name: "name", Desc: "column name pattern, empty for every column", Default: ""},
			system,
		},
		Scan: func(rows *sql.Rows) (dbmeta.ColumnStat, error) {
			var v dbmeta.ColumnStat
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Table, &v.Name, &v.AvgWidth,
				&v.NullFrac, &v.Distinct, &v.Min, &v.Max, &v.Mean, &v.TopN, &v.TopNFreqs)
			return v, err
		},
	})

	// CATALOGS has the one row DRILL. The Databases kind is the catalog, as
	// it is on Trino.
	dbmeta.Databases.Register(dbmeta.Drill, &dbmeta.Binding[dbmeta.Database]{
		Stmt: dbmeta.Stmt{
			always("SELECT c.CATALOG_NAME AS `name`"),
			always(", '' AS `owner`"),
			always(", '' AS `encoding`"),
			always(", '' AS `collate`"),
			always(", '' AS `ctype`"),
			always(", NULL AS `access`"),
			always(", NULL AS `tablespace`"),
			always(", NULL AS `size`"),
			always(", c.CATALOG_DESCRIPTION AS `comment`"),
			always("FROM INFORMATION_SCHEMA.CATALOGS c"),
			always("WHERE " + like("c.CATALOG_NAME", "@name")),
			always("ORDER BY 1"),
		},
		Fields: []dbmeta.Field{
			{Name: "name", Desc: "always DRILL"},
			{Name: "owner", Desc: "always empty: a catalog records no owner"},
			{Name: "encoding", Desc: "always empty: Drill records no encoding"},
			{Name: "collate", Desc: "always empty: a catalog has no collation"},
			{Name: "ctype", Desc: "always empty: a catalog has no character type"},
			{Name: "access", Desc: "always absent: Drill grants nothing on a catalog"},
			{Name: "tablespace", Desc: "always absent: Drill stores nothing of its own"},
			{Name: "size", Desc: "always absent: the size belongs to what the storage plugins reach"},
			{Name: "comment", Desc: "the description of the catalog, which is The internal metadata used by Drill"},
		},
		Params: []dbmeta.Param{
			{Name: "name", Desc: "catalog name pattern, empty for every catalog", Default: ""},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Database, error) {
			var v dbmeta.Database
			err := rows.Scan(&v.Name, &v.Owner, &v.Encoding, &v.Collate, &v.CType,
				&v.Access, &v.Tablespace, &v.Size, &v.Comment)
			return v, err
		},
	})
}
