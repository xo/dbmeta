package databend

import (
	"database/sql"

	"github.com/xo/dbmeta"
)

// systemDatabases are the databases Databend keeps for itself.
const systemDatabases = `'system', 'information_schema'`

// notSystem hides the system databases for the column col unless the caller
// asks for them.
func notSystem(col string) string {
	return `(@with_system OR ` + col + ` NOT IN (` + systemDatabases + `))`
}

// like is the filter that matches col to the pattern in param, and passes
// when the pattern is empty.
func like(col, param string) string {
	return `(` + param + ` = '' OR ` + col + ` LIKE ` + param + `)`
}

func always(query string) dbmeta.Choice { return dbmeta.Choice{{Query: query}} }

func fields(names ...string) []dbmeta.Field { return dbmeta.Fields(names...) }

func schemaNameSystem(kind string) []dbmeta.Param {
	return []dbmeta.Param{
		{Name: "schema", Desc: "database name pattern, empty for every database", Default: ""},
		{Name: "name", Desc: kind + " name pattern, empty for every " + kind, Default: ""},
		{Name: "with_system", Desc: "include the databases Databend keeps for itself", Default: false},
	}
}

// childParams are the parameters of a kind whose objects belong to a table.
func childParams(kind string) []dbmeta.Param {
	return append([]dbmeta.Param{
		{Name: "parent", Desc: "table name pattern, empty for every table", Default: ""},
	}, schemaNameSystem(kind)...)
}

// nameSystem are the parameters of a kind that belongs to no database.
func nameSystem(kind, system string) []dbmeta.Param {
	return []dbmeta.Param{
		{Name: "schema", Desc: "database name pattern. A " + kind + " belongs to no database, so its schema is empty", Default: ""},
		{Name: "name", Desc: kind + " name pattern, empty for every " + kind, Default: ""},
		{Name: "with_system", Desc: system, Default: false},
	}
}

// tableType is the word for the kind of the table t, from its engine.
const tableType = `CASE WHEN t.engine = 'FUSE' THEN 'table' WHEN t.engine = 'VIEW' THEN 'view'` +
	` WHEN t.engine = 'MATERIALIZED_VIEW' THEN 'materialized view'` +
	` WHEN t.engine LIKE 'System%' THEN 'system table'` +
	` ELSE lower(t.engine) || ' table' END`

// register registers every statement the model answers.
func register() {
	registerRelations()
	registerRoutines()
	registerServer()
}

func registerRelations() {
	// \dn. A database is the only namespace, so it is the schema. A system
	// database has no owner.
	dbmeta.Schemas.Register(dbmeta.Databend, &dbmeta.Binding[dbmeta.Schema]{
		Stmt: dbmeta.Stmt{
			always(`SELECT d.catalog AS "catalog"`),
			always(`, d.name AS "name"`),
			always(`, d.owner AS "owner"`),
			always(`, NULL AS "comment"`),
			always(`FROM system.databases d`),
			always(`WHERE ` + notSystem("d.name")),
			always(`AND ` + like("d.name", "@name")),
			always(`ORDER BY 2`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog"}, {Name: "name"},
			{Name: "owner", Desc: "the role that owns the database, and empty for one Databend made"},
			{Name: "comment", Desc: "always absent: a database takes no comment"},
		},
		Params: []dbmeta.Param{
			{Name: "name", Desc: "database name pattern, empty for every database", Default: ""},
			{Name: "with_system", Desc: "include the databases Databend keeps for itself", Default: false},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Schema, error) {
			var v dbmeta.Schema
			err := rows.Scan(&v.Catalog, &v.Name, dbmeta.NullAsEmpty(&v.Owner), &v.Comment)
			return v, err
		},
	})

	// \l. The same rows as Schemas, in the shape of a database.
	dbmeta.Databases.Register(dbmeta.Databend, &dbmeta.Binding[dbmeta.Database]{
		Stmt: dbmeta.Stmt{
			always(`SELECT d.name AS "name"`),
			always(`, d.owner AS "owner"`),
			always(`, '' AS "encoding"`),
			always(`, '' AS "collate"`),
			always(`, '' AS "ctype"`),
			always(`, NULL AS "access"`),
			always(`, NULL AS "tablespace"`),
			always(`, NULL AS "size"`),
			always(`, NULL AS "comment"`),
			always(`FROM system.databases d`),
			always(`WHERE ` + notSystem("d.name")),
			always(`AND ` + like("d.name", "@name")),
			always(`ORDER BY 1`),
		},
		Fields: []dbmeta.Field{
			{Name: "name"},
			{Name: "owner", Desc: "the role that owns the database, and empty for one Databend made"},
			{Name: "encoding", Desc: "always empty: a database records no encoding, and every string is UTF-8"},
			{Name: "collate", Desc: "always empty: a database records no collation"},
			{Name: "ctype", Desc: "always empty: a database records no collation"},
			{Name: "access", Desc: "always absent: a grant is listed only by show_grants for one role"},
			{Name: "tablespace", Desc: "always absent: Databend has no tablespace"},
			{Name: "size", Desc: "always absent: system.databases records no size"},
			{Name: "comment", Desc: "always absent: a database takes no comment"},
		},
		Params: []dbmeta.Param{
			{Name: "name", Desc: "database name pattern, empty for every database", Default: ""},
			{Name: "with_system", Desc: "include the databases Databend keeps for itself", Default: false},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Database, error) {
			var v dbmeta.Database
			err := rows.Scan(&v.Name, dbmeta.NullAsEmpty(&v.Owner), &v.Encoding, &v.Collate, &v.CType,
				&v.Access, &v.Tablespace, &v.Size, &v.Comment)
			return v, err
		},
	})

	// \dt, \dv and \dm. system.tables lists a view too, with the engine VIEW.
	dbmeta.Tables.Register(dbmeta.Databend, &dbmeta.Binding[dbmeta.Table]{
		Stmt: dbmeta.Stmt{
			always(`SELECT t.catalog AS "catalog"`),
			always(`, t.database AS "schema"`),
			always(`, t.name AS "name"`),
			always(`, ` + tableType + ` AS "type"`),
			always(`, NULLIF(t.comment, '') AS "comment"`),
			always(`FROM system.tables t`),
			always(`WHERE ` + notSystem("t.database")),
			always(`AND ` + like("t.database", "@schema")),
			always(`AND ` + like("t.name", "@name")),
			always(`AND (@types = '' OR ` + dbmeta.InList(`@types`, tableType) + `)`),
			always(`ORDER BY 2, 3`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog"}, {Name: "schema"}, {Name: "name"},
			{Name: "type", Desc: "table for the FUSE engine, view, materialized view, system table, and the engine's name and table for any other, such as iceberg table"},
			{Name: "comment", Desc: "the COMMENT of the table, and absent where there is none"},
		},
		Params: append(schemaNameSystem("table"), dbmeta.TypesParam()),
		Scan: func(rows *sql.Rows) (dbmeta.Table, error) {
			var v dbmeta.Table
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Name, &v.Type, &v.Comment)
			return v, err
		},
	})

	// \d NAME. system.columns has no position, and information_schema
	// reports 1 for every column, so the ordinal is counted in the order
	// system.columns lists a table's columns, which is the order they were
	// declared in. A stream lists the columns of its table, and a stream is
	// not a table, so only a table's columns are read.
	dbmeta.Columns.Register(dbmeta.Databend, &dbmeta.Binding[dbmeta.Column]{
		Stmt: dbmeta.Stmt{
			always(`SELECT t.catalog AS "catalog"`),
			always(`, c.database AS "schema"`),
			always(`, c.table AS "table"`),
			always(`, c.name AS "name"`),
			always(`, row_number() OVER (PARTITION BY c.database, c.table) AS "ordinal"`),
			always(`, c.data_type AS "data_type"`),
			always(`, c.is_nullable = 'YES' AS "nullable"`),
			always(`, CASE WHEN c.default_kind = 'DEFAULT' THEN c.default_expression END AS "default"`),
			always(`, false AS "primary_key"`),
			always(`, NULL AS "identity"`),
			always(`, NULL AS "generated"`),
			always(`, NULLIF(c.comment, '') AS "comment"`),
			always(`, NULL AS "collation"`),
			always(`FROM system.columns c`),
			always(`JOIN system.tables t ON t.database = c.database AND t.name = c.table`),
			always(`WHERE ` + notSystem("c.database")),
			always(`AND ` + like("c.database", "@schema")),
			always(`AND ` + like("c.table", "@parent")),
			always(`AND ` + like("c.name", "@name")),
			always(`ORDER BY 2, 3, 5`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog"}, {Name: "schema"}, {Name: "table"}, {Name: "name"},
			{Name: "ordinal", Desc: "the position in the order system.columns lists a table's columns, which is the order they were declared in, from 1"},
			{Name: "data_type", Desc: "the type as Databend writes it, such as VARCHAR or DECIMAL(10, 2)"},
			{Name: "nullable"},
			{Name: "default", Desc: "the DEFAULT expression, and absent where there is none"},
			{Name: "primary_key", Desc: "always false: Databend has no primary key"},
			{Name: "identity", Desc: "always absent: system.columns does not say that a column is AUTOINCREMENT"},
			{Name: "generated", Desc: "always absent: system.columns does not say that a column is computed"},
			{Name: "comment"},
			{Name: "collation", Desc: "always absent: Databend has no collation on a column"},
		},
		Params: childParams("column"),
		Scan: func(rows *sql.Rows) (dbmeta.Column, error) {
			var v dbmeta.Column
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Table, &v.Name, &v.Ordinal,
				&v.DataType, &v.Nullable, &v.Default, &v.PrimaryKey, &v.Identity,
				&v.Generated, &v.Comment, &v.Collation)
			return v, err
		},
	})

	// \di. An inverted, ngram or vector index belongs to a table. An
	// aggregating index belongs to a query and names no table, and its
	// table is empty. None enforces anything.
	dbmeta.Indexes.Register(dbmeta.Databend, &dbmeta.Binding[dbmeta.Index]{
		Stmt: dbmeta.Stmt{
			always(`SELECT 'default' AS "catalog"`),
			always(`, i.database AS "schema"`),
			always(`, i."table" AS "table"`),
			always(`, i.name AS "name"`),
			always(`, lower(i.type) AS "type"`),
			always(`, false AS "unique"`),
			always(`, false AS "primary"`),
			always(`, NULL AS "comment"`),
			always(`FROM system.indexes i`),
			always(`WHERE ` + notSystem("i.database")),
			always(`AND ` + like("i.database", "@schema")),
			always(`AND ` + like("i.\"table\"", "@parent")),
			always(`AND ` + like("i.name", "@name")),
			always(`ORDER BY 2, 3, 4`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "always default, the one catalog a connection reaches"},
			{Name: "schema"},
			{Name: "table", Desc: "the table, and empty for an aggregating index, which belongs to a query"},
			{Name: "name"},
			{Name: "type", Desc: "inverted, ngram, vector or aggregating"},
			{Name: "unique", Desc: "always false: a Databend index enforces nothing"},
			{Name: "primary", Desc: "always false: Databend has no primary key"},
			{Name: "comment", Desc: "always absent: an index takes no comment"},
		},
		Params: childParams("index"),
		Scan: func(rows *sql.Rows) (dbmeta.Index, error) {
			var v dbmeta.Index
			err := rows.Scan(&v.Catalog, &v.Schema, dbmeta.NullAsEmpty(&v.Table), &v.Name, &v.Type,
				&v.Unique, &v.Primary, &v.Comment)
			return v, err
		},
	})

	// The columns of an index, which system.indexes records only as the
	// definition, such as book(a, b). The list between the brackets is split
	// and numbered in the order it was written. An aggregating index is
	// defined by a query and has no such list, so it is left out.
	dbmeta.IndexColumns.Register(dbmeta.Databend, &dbmeta.Binding[dbmeta.IndexColumn]{
		Stmt: dbmeta.Stmt{
			always(`SELECT x.database AS "schema"`),
			always(`, x.t AS "table"`),
			always(`, x.name AS "index"`),
			always(`, x.col AS "name"`),
			always(`, row_number() OVER (PARTITION BY x.database, x.t, x.name) AS "ordinal"`),
			always(`, NULL AS "expression"`),
			always(`, false AS "descending"`),
			always(`FROM (`),
			always(`  SELECT i.database, i."table" AS t, i.name, trim(unnest(split(` +
				`substr(i.definition, position('(' IN i.definition) + 1,` +
				` length(i.definition) - position('(' IN i.definition) - 1), ','))) AS col`),
			always(`  FROM system.indexes i`),
			always(`  WHERE i.type <> 'AGGREGATING'`),
			always(`  AND ` + notSystem("i.database")),
			always(`  AND ` + like("i.database", "@schema")),
			always(`  AND ` + like(`i."table"`, "@parent")),
			always(`  AND ` + like("i.name", "@name")),
			always(`) x`),
			always(`ORDER BY 1, 2, 3, 5`),
		},
		Fields: []dbmeta.Field{
			{Name: "schema"}, {Name: "table"}, {Name: "index"},
			{Name: "name", Desc: "the column, read from the definition of the index, such as book(a, b)"},
			{Name: "ordinal", Desc: "the position in the definition, from 1"},
			{Name: "expression", Desc: "always absent: a Databend index names columns"},
			{Name: "descending", Desc: "always false: a Databend index has no direction"},
		},
		Params: childParams("index"),
		Scan: func(rows *sql.Rows) (dbmeta.IndexColumn, error) {
			var v dbmeta.IndexColumn
			err := rows.Scan(&v.Schema, dbmeta.NullAsEmpty(&v.Table), &v.Index, &v.Name, &v.Ordinal,
				&v.Expression, &v.Descending)
			return v, err
		},
	})

	// The columns a CHECK names, which system.constraints records as a list
	// joined by commas, in the order of the table's columns.
	dbmeta.ConstraintColumns.Register(dbmeta.Databend, &dbmeta.Binding[dbmeta.ConstraintColumn]{
		Stmt: dbmeta.Stmt{
			always(`SELECT 'default' AS "catalog"`),
			always(`, x.database AS "schema"`),
			always(`, x.t AS "table"`),
			always(`, x.name AS "constraint"`),
			always(`, x.col AS "name"`),
			always(`, row_number() OVER (PARTITION BY x.database, x.t, x.name) AS "ordinal"`),
			always(`, NULL AS "foreign_catalog"`),
			always(`, NULL AS "foreign_schema"`),
			always(`, NULL AS "foreign_table"`),
			always(`, NULL AS "foreign_name"`),
			always(`FROM (`),
			always(`  SELECT k.database, k."table" AS t, k.name,` +
				` trim(unnest(split(k.constraint_column_names, ','))) AS col`),
			always(`  FROM system.constraints k`),
			always(`  WHERE ` + notSystem("k.database")),
			always(`  AND ` + like("k.database", "@schema")),
			always(`  AND ` + like(`k."table"`, "@parent")),
			always(`  AND ` + like("k.name", "@name")),
			always(`) x`),
			always(`ORDER BY 2, 3, 4, 6`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "always default, the one catalog a connection reaches"},
			{Name: "schema"}, {Name: "table"}, {Name: "constraint"},
			{Name: "name", Desc: "a column the CHECK names"},
			{Name: "ordinal", Desc: "the position in the list system.constraints records, which is the order of the table's columns, from 1"},
			{Name: "foreign_catalog", Desc: "always absent: Databend has no foreign key"},
			{Name: "foreign_schema", Desc: "always absent: Databend has no foreign key"},
			{Name: "foreign_table", Desc: "always absent: Databend has no foreign key"},
			{Name: "foreign_name", Desc: "always absent: Databend has no foreign key"},
		},
		Params: childParams("constraint"),
		Scan: func(rows *sql.Rows) (dbmeta.ConstraintColumn, error) {
			var v dbmeta.ConstraintColumn
			err := rows.Scan(&v.Catalog, dbmeta.NullAsEmpty(&v.Schema), dbmeta.NullAsEmpty(&v.Table), &v.Constraint, &v.Name, &v.Ordinal,
				&v.ForeignCatalog, &v.ForeignSchema, &v.ForeignTable, &v.ForeignName)
			return v, err
		},
	})

	// \d NAME, the constraint section. A Databend constraint is a CHECK.
	dbmeta.Constraints.Register(dbmeta.Databend, &dbmeta.Binding[dbmeta.Constraint]{
		Stmt: dbmeta.Stmt{
			always(`SELECT k.database AS "schema"`),
			always(`, k."table" AS "table"`),
			always(`, k.name AS "name"`),
			always(`, lower(k.type) AS "type"`),
			always(`, k.expression AS "definition"`),
			always(`, false AS "deferrable"`),
			always(`, false AS "deferred"`),
			always(`, NULL AS "comment"`),
			always(`FROM system.constraints k`),
			always(`WHERE ` + notSystem("k.database")),
			always(`AND ` + like("k.database", "@schema")),
			always(`AND ` + like("k.\"table\"", "@parent")),
			always(`AND ` + like("k.name", "@name")),
			always(`ORDER BY 1, 2, 3`),
		},
		Fields: []dbmeta.Field{
			{Name: "schema"}, {Name: "table"}, {Name: "name"},
			{Name: "type", Desc: "check, the only constraint Databend has"},
			{Name: "definition", Desc: "the condition of the CHECK"},
			{Name: "deferrable", Desc: "always false: Databend defers nothing"},
			{Name: "deferred", Desc: "always false: Databend defers nothing"},
			{Name: "comment", Desc: "always absent: a constraint takes no comment"},
		},
		Params: childParams("constraint"),
		Scan: func(rows *sql.Rows) (dbmeta.Constraint, error) {
			var v dbmeta.Constraint
			err := rows.Scan(dbmeta.NullAsEmpty(&v.Schema), dbmeta.NullAsEmpty(&v.Table), &v.Name, &v.Type,
				&v.Definition, &v.Deferrable, &v.Deferred, &v.Comment)
			return v, err
		},
	})

	// The definition of a view.
	dbmeta.Views.Register(dbmeta.Databend, &dbmeta.Binding[dbmeta.View]{
		Stmt: dbmeta.Stmt{
			always(`SELECT v.catalog AS "catalog"`),
			always(`, v.database AS "schema"`),
			always(`, v.name AS "name"`),
			always(`, v.view_query AS "definition"`),
			always(`, NULL AS "check_option"`),
			always(`, NULL AS "updatable"`),
			always(`, NULL AS "insertable"`),
			always(`, NULLIF(v.comment, '') AS "comment"`),
			always(`FROM system.views v`),
			always(`WHERE ` + notSystem("v.database")),
			always(`AND ` + like("v.database", "@schema")),
			always(`AND ` + like("v.name", "@name")),
			always(`ORDER BY 2, 3`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog"}, {Name: "schema"}, {Name: "name"},
			{Name: "definition", Desc: "the query of the view as Databend stores it"},
			{Name: "check_option", Desc: "always absent: Databend has no WITH CHECK OPTION"},
			{Name: "updatable", Desc: "always absent: system.views does not say"},
			{Name: "insertable", Desc: "always absent: system.views does not say"},
			{Name: "comment"},
		},
		Params: schemaNameSystem("view"),
		Scan: func(rows *sql.Rows) (dbmeta.View, error) {
			var v dbmeta.View
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Name, &v.Definition, &v.CheckOption,
				&v.Updatable, &v.Insertable, &v.Comment)
			return v, err
		},
	})

	// Every comment is a column on its object, so this gathers the tables
	// that have one, as the mysql model does.
	dbmeta.Comments.Register(dbmeta.Databend, &dbmeta.Binding[dbmeta.Comment]{
		Stmt: dbmeta.Stmt{
			always(`SELECT t.database AS "schema"`),
			always(`, t.name AS "name"`),
			always(`, ` + tableType + ` AS "type"`),
			always(`, t.comment AS "comment"`),
			always(`FROM system.tables t`),
			always(`WHERE t.comment <> ''`),
			always(`AND ` + notSystem("t.database")),
			always(`AND ` + like("t.database", "@schema")),
			always(`AND ` + like("t.name", "@name")),
			always(`ORDER BY 1, 2`),
		},
		Fields: fields("schema", "name", "type", "comment"),
		Params: schemaNameSystem("object"),
		Scan: func(rows *sql.Rows) (dbmeta.Comment, error) {
			var v dbmeta.Comment
			err := rows.Scan(&v.Schema, &v.Name, &v.Type, &v.Comment)
			return v, err
		},
	})

	// \d NAME, the statistics. ANALYZE TABLE fills them, and a table that
	// was never analyzed has a row with every value absent.
	dbmeta.ColumnStats.Register(dbmeta.Databend, &dbmeta.Binding[dbmeta.ColumnStat]{
		Stmt: dbmeta.Stmt{
			always(`SELECT 'default' AS "catalog"`),
			always(`, s.database AS "schema"`),
			always(`, s."table" AS "table"`),
			always(`, s.column_name AS "name"`),
			always(`, CAST(s.avg_size AS BIGINT) AS "avg_width"`),
			always(`, CASE WHEN s.stats_row_count > 0` +
				` THEN CAST(s.null_count AS DOUBLE) / s.stats_row_count END AS "null_frac"`),
			always(`, CAST(s.distinct_count AS DOUBLE) AS "distinct"`),
			always(`, s.min AS "min"`),
			always(`, s.max AS "max"`),
			always(`, NULL AS "mean"`),
			always(`, NULL AS "top_n"`),
			always(`, NULL AS "top_n_freqs"`),
			always(`FROM system.statistics s`),
			always(`WHERE ` + notSystem("s.database")),
			always(`AND ` + like("s.database", "@schema")),
			always(`AND ` + like("s.\"table\"", "@parent")),
			always(`AND ` + like("s.column_name", "@name")),
			always(`ORDER BY 2, 3, 4`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "always default, the one catalog a connection reaches"},
			{Name: "schema"}, {Name: "table"}, {Name: "name"},
			{Name: "avg_width", Desc: "the average size of a value in bytes"},
			{Name: "null_frac", Desc: "the NULL count over the row count when the statistics were taken"},
			{Name: "distinct", Desc: "the distinct count, which Databend keeps as a count"},
			{Name: "min"}, {Name: "max"},
			{Name: "mean", Desc: "always absent: Databend records no mean"},
			{Name: "top_n", Desc: "always absent: Databend records no most common values"},
			{Name: "top_n_freqs", Desc: "always absent, for the same reason as top_n"},
		},
		Params: childParams("column"),
		Scan: func(rows *sql.Rows) (dbmeta.ColumnStat, error) {
			var v dbmeta.ColumnStat
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Table, &v.Name, &v.AvgWidth,
				&v.NullFrac, &v.Distinct, &v.Min, &v.Max, &v.Mean, &v.TopN, &v.TopNFreqs)
			return v, err
		},
	})

	// \ds. A sequence belongs to the tenant, and only show_sequences()
	// lists one. interval is the increment, and current is the next value,
	// not the first, so the start is not known.
	dbmeta.Sequences.Register(dbmeta.Databend, &dbmeta.Binding[dbmeta.Sequence]{
		Stmt: dbmeta.Stmt{
			always(`SELECT '' AS "schema"`),
			always(`, s.name AS "name"`),
			always(`, NULL AS "data_type"`),
			always(`, NULL AS "start"`),
			always(`, NULL AS "minimum"`),
			always(`, NULL AS "maximum"`),
			always(`, CAST(s.interval AS STRING) AS "increment"`),
			always(`, NULL AS "cycles"`),
			always(`, '' AS "owned_by"`),
			always(`, s.comment AS "comment"`),
			always(`FROM show_sequences() s`),
			always(`WHERE ` + like("''", "@schema")),
			always(`AND ` + like("s.name", "@name")),
			always(`ORDER BY 2`),
		},
		Fields: []dbmeta.Field{
			{Name: "schema", Desc: "always empty: a sequence belongs to no database"},
			{Name: "name"},
			{Name: "data_type", Desc: "always absent: show_sequences does not say"},
			{Name: "start", Desc: "always absent: Databend keeps the next value and not the first"},
			{Name: "minimum", Desc: "always absent: a Databend sequence records no bound"},
			{Name: "maximum", Desc: "always absent: a Databend sequence records no bound"},
			{Name: "increment", Desc: "the interval of the sequence"},
			{Name: "cycles", Desc: "always absent: show_sequences does not say"},
			{Name: "owned_by", Desc: "always empty: a sequence belongs to no column"},
			{Name: "comment"},
		},
		Params: []dbmeta.Param{
			{Name: "schema", Desc: "database name pattern. A sequence belongs to no database, so its schema is empty", Default: ""},
			{Name: "name", Desc: "sequence name pattern, empty for every sequence", Default: ""},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Sequence, error) {
			var v dbmeta.Sequence
			err := rows.Scan(&v.Schema, &v.Name, &v.DataType, &v.Start, &v.Minimum,
				&v.Maximum, &v.Increment, &v.Cycles, &v.OwnedBy, &v.Comment)
			return v, err
		},
	})

	// The current database, as the schema a connection is in.
	dbmeta.CurrentSchema.Register(dbmeta.Databend, &dbmeta.Binding[dbmeta.Schema]{
		Stmt: dbmeta.Stmt{
			always(`SELECT d.catalog AS "catalog"`),
			always(`, d.name AS "name"`),
			always(`, d.owner AS "owner"`),
			always(`, NULL AS "comment"`),
			always(`FROM system.databases d`),
			always(`WHERE d.name = database()`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog"}, {Name: "name"},
			{Name: "owner", Desc: "the role that owns the database, and empty for one Databend made"},
			{Name: "comment", Desc: "always absent: a database takes no comment"},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Schema, error) {
			var v dbmeta.Schema
			err := rows.Scan(&v.Catalog, &v.Name, dbmeta.NullAsEmpty(&v.Owner), &v.Comment)
			return v, err
		},
	})
}
