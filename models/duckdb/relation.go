package duckdb

import (
	"database/sql"

	"github.com/xo/dbmeta"
)

// Schemas, tables, columns, indexes, constraints, sequences and views.

func registerRelations() {
	dbmeta.Schemas.Register(dbmeta.DuckDB, &dbmeta.Binding[dbmeta.Schema]{
		Stmt: dbmeta.Stmt{
			always(`SELECT s.database_name AS "catalog"`),
			always(`, s.schema_name AS "name"`),
			always(`, '' AS "owner"`),
			always(`, s.comment AS "comment"`),
			always(`FROM duckdb_schemas() s`),
			userSchemas("WHERE", "s"),
			always(`AND (@name = '' OR s.schema_name LIKE @name)`),
			always(`ORDER BY 1, 2`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "the attached database the schema is in"},
			{Name: "name"},
			{Name: "owner", Desc: "always empty: DuckDB has no users"},
			{Name: "comment"},
		},
		Params: []dbmeta.Param{
			{Name: "name", Desc: "schema name pattern, empty for every schema", Default: ""},
			{Name: "with_system", Desc: "include the schemas DuckDB ships with itself", Default: false},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Schema, error) {
			var v dbmeta.Schema
			err := rows.Scan(&v.Catalog, &v.Name, &v.Owner, &v.Comment)
			return v, err
		},
	})

	// \l. An attached database is what DuckDB calls a database, and the path
	// is the file behind it, which is empty for one held in memory.
	dbmeta.Databases.Register(dbmeta.DuckDB, &dbmeta.Binding[dbmeta.Database]{
		Stmt: dbmeta.Stmt{
			always(`SELECT d.database_name AS "name"`),
			always(`, '' AS "owner"`),
			always(`, 'UTF8' AS "encoding"`),
			always(`, 'BINARY' AS "collate"`),
			always(`, 'BINARY' AS "ctype"`),
			always(`, NULL AS "access"`),
			always(`, NULLIF(d.path, '') AS "tablespace"`),
			always(`, NULL AS "size"`),
			always(`, d.comment AS "comment"`),
			always(`FROM duckdb_databases() d`),
			internalOf("WHERE", "d"),
			always(`AND (@name = '' OR d.database_name LIKE @name)`),
			always(`ORDER BY 1`),
		},
		Fields: []dbmeta.Field{
			{Name: "name"},
			{Name: "owner", Desc: "always empty: DuckDB has no users"},
			{Name: "encoding", Desc: "always UTF8: DuckDB stores text as UTF-8 and offers no other"},
			{Name: "collate", Desc: "always BINARY unless a column says otherwise"},
			{Name: "ctype", Desc: "always BINARY, for the same reason"},
			{Name: "access", Desc: "always absent: DuckDB has no grants"},
			{
				Name: "tablespace",
				Desc: "the file this database is stored in, absent for one held in memory",
			},
			{Name: "size", Desc: "always absent: DuckDB publishes no database size"},
			{Name: "comment"},
		},
		Params: []dbmeta.Param{
			{Name: "name", Desc: "database name pattern, empty for every one", Default: ""},
			{Name: "with_system", Desc: "include the databases DuckDB attaches itself", Default: false},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Database, error) {
			var v dbmeta.Database
			err := rows.Scan(&v.Name, &v.Owner, &v.Encoding, &v.Collate, &v.CType,
				&v.Access, &v.Tablespace, &v.Size, &v.Comment)
			return v, err
		},
	})

	// \d, \dt, \dv. Tables and views live in different catalog functions, so
	// they are unioned here, which is what psql shows in one listing.
	dbmeta.Tables.Register(dbmeta.DuckDB, &dbmeta.Binding[dbmeta.Table]{
		Stmt: dbmeta.Stmt{
			always(`SELECT t.database_name AS "catalog"`),
			always(`, t.schema_name AS "schema"`),
			always(`, t.table_name AS "name"`),
			always(`, CASE WHEN t.temporary THEN 'temporary table' ELSE 'table' END AS "type"`),
			always(`, t.comment AS "comment"`),
			always(`, CASE WHEN t.temporary THEN 'temporary' ELSE 'permanent' END AS "persistence"`),
			always(`, CAST(t.estimated_size AS BIGINT) AS "rows"`),
			always(`FROM duckdb_tables() t`),
			internalOf("WHERE", "t"),
			always(`AND (@schema = '' OR t.schema_name LIKE @schema)`),
			always(`AND (@name = '' OR t.table_name LIKE @name)`),
			// The cast types the parameter, because DuckDB cannot infer the
			// type of a bare one in a || chain when it prepares the statement.
			always(`AND (@types = '' OR ` + dbmeta.InList(`CAST(@types AS VARCHAR)`, `CASE WHEN t.temporary THEN 'temporary table' ELSE 'table' END`) + `)`),
			always(`UNION ALL`),
			always(`SELECT v.database_name, v.schema_name, v.view_name`),
			always(`, CASE WHEN v.temporary THEN 'temporary view' ELSE 'view' END`),
			always(`, v.comment`),
			always(`, CAST(NULL AS VARCHAR)`),
			always(`, CAST(NULL AS BIGINT)`),
			always(`FROM duckdb_views() v`),
			internalOf("WHERE", "v"),
			always(`AND (@schema = '' OR v.schema_name LIKE @schema)`),
			always(`AND (@name = '' OR v.view_name LIKE @name)`),
			always(`AND (@types = '' OR ` + dbmeta.InList(`CAST(@types AS VARCHAR)`, `CASE WHEN v.temporary THEN 'temporary view' ELSE 'view' END`) + `)`),
			always(`ORDER BY 1, 2, 3`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog"}, {Name: "schema"}, {Name: "name"},
			{Name: "type", Desc: "table or view, with temporary in front where it is one"},
			{Name: "comment"},
			{Name: "persistence", Desc: "permanent or temporary. Absent for a view"},
			{
				Name: "rows",
				Desc: "estimated_size of duckdb_tables, which is DuckDB's own estimate of the rows and not a count. Absent for a view",
			},
		},
		Params: append(schemaNameSystem("table"), dbmeta.TypesParam()),
		Scan: func(rows *sql.Rows) (dbmeta.Table, error) {
			var v dbmeta.Table
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Name, &v.Type, &v.Comment,
				&v.Persistence, &v.Rows)
			return v, err
		},
	})

	// \d name. duckdb_columns covers a view's columns as well as a table's,
	// so this needs no union.
	dbmeta.Columns.Register(dbmeta.DuckDB, &dbmeta.Binding[dbmeta.Column]{
		Stmt: dbmeta.Stmt{
			always(`SELECT c.database_name AS "catalog"`),
			always(`, c.schema_name AS "schema"`),
			always(`, c.table_name AS "table"`),
			always(`, c.column_name AS "name"`),
			always(`, c.column_index AS "ordinal"`),
			always(`, c.data_type AS "data_type"`),
			always(`, c.is_nullable AS "nullable"`),
			// A generated column keeps its expression apart from the default of a column.
			always(`, CASE WHEN c.is_generated THEN c.generation_expression ELSE c.column_default END AS "default"`),
			// One more read of the constraint catalog, which is small and in
			// memory. See D47.
			always(`, EXISTS (SELECT 1 FROM duckdb_constraints() k` +
				` WHERE k.table_oid = c.table_oid AND k.constraint_type = 'PRIMARY KEY'` +
				` AND LIST_CONTAINS(k.constraint_column_names, c.column_name)) AS "primary_key"`),
			always(`, NULL AS "identity"`),
			// DuckDB computes a generated column when it is read, and has no stored one.
			always(`, CASE WHEN c.is_generated THEN 'virtual' END AS "generated"`),
			always(`, c.comment AS "comment"`),
			always(`, CAST(NULL AS VARCHAR) AS "collation"`),
			always(`FROM duckdb_columns() c`),
			internalOf("WHERE", "c"),
			always(`AND (@schema = '' OR c.schema_name LIKE @schema)`),
			always(`AND (@parent = '' OR c.table_name LIKE @parent)`),
			always(`AND (@name = '' OR c.column_name LIKE @name)`),
			always(`ORDER BY 2, 3, 5`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog"}, {Name: "schema"}, {Name: "table"}, {Name: "name"},
			{Name: "ordinal", Desc: "one based, which is how duckdb_columns counts"},
			{Name: "data_type"},
			{Name: "nullable"},
			{Name: "default", Desc: "the default expression as written, or the expression of a generated column"},
			{Name: "primary_key"},
			{
				Name: "identity",
				Desc: "always absent: DuckDB has no identity column, and a column that defaults to nextval is a default rather than a kind",
			},
			{
				Name: "generated",
				Desc: "virtual for a generated column, which DuckDB computes when it reads it, and absent for any other",
			},
			{Name: "comment"},
			{Name: "collation", Desc: "always absent: duckdb_columns does not record a collation"},
		},
		Params: schemaParentName("column"),
		Scan: func(rows *sql.Rows) (dbmeta.Column, error) {
			var v dbmeta.Column
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Table, &v.Name, &v.Ordinal,
				&v.DataType, &v.Nullable, &v.Default, &v.PrimaryKey, &v.Identity,
				&v.Generated, &v.Comment, &v.Collation)
			return v, err
		},
	})

	// \di. A UNIQUE or PRIMARY KEY constraint builds an index, and DuckDB
	// does not list it in duckdb_indexes, so the second half of the statement
	// reads it from duckdb_constraints. That index has no name and no CREATE
	// INDEX text, so it takes the name of the constraint and an absent
	// definition, and constraint_type says which constraint owns it.
	dbmeta.Indexes.Register(dbmeta.DuckDB, &dbmeta.Binding[dbmeta.Index]{
		Stmt: dbmeta.Stmt{
			always(`SELECT x."catalog", x."schema", x."table", x."name", x."type", x."unique", x."primary"`),
			always(`, x."comment", x."persistence", x."definition", x."using", x."constraint_type"`),
			always(`FROM (`),
			always(`SELECT i.database_name AS "catalog"`),
			always(`, i.schema_name AS "schema"`),
			always(`, i.table_name AS "table"`),
			always(`, i.index_name AS "name"`),
			always(`, 'art' AS "type"`),
			always(`, i.is_unique AS "unique"`),
			always(`, i.is_primary AS "primary"`),
			always(`, i.comment AS "comment"`),
			// An index on a temporary table is in the temp database.
			always(`, CASE WHEN i.database_name = 'temp' THEN 'temporary' ELSE 'permanent' END AS "persistence"`),
			always(`, i.sql AS "definition"`),
			always(`, 'art' AS "using"`),
			always(`, NULL AS "constraint_type"`),
			always(`FROM duckdb_indexes() i`),
			always(`UNION ALL`),
			always(`SELECT k.database_name`),
			always(`, k.schema_name`),
			always(`, k.table_name`),
			always(`, k.constraint_name`),
			always(`, 'art'`),
			always(`, TRUE`),
			always(`, k.constraint_type = 'PRIMARY KEY'`),
			always(`, NULL`),
			always(`, CASE WHEN k.database_name = 'temp' THEN 'temporary' ELSE 'permanent' END`),
			always(`, NULL`),
			always(`, 'art'`),
			always(`, CASE k.constraint_type WHEN 'PRIMARY KEY' THEN 'p' ELSE 'u' END`),
			always(`FROM duckdb_constraints() k`),
			always(`WHERE k.constraint_type IN ('PRIMARY KEY', 'UNIQUE')`),
			always(`) x`),
			always(`WHERE (@schema = '' OR x."schema" LIKE @schema)`),
			always(`AND (@parent = '' OR x."table" LIKE @parent)`),
			always(`AND (@name = '' OR x."name" LIKE @name)`),
			always(`AND (@with_system OR x."schema" NOT IN ('information_schema', 'pg_catalog'))`),
			always(`ORDER BY 2, 3, 4`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog"}, {Name: "schema"}, {Name: "table"}, {Name: "name"},
			{
				Name: "type",
				Desc: "always art, the adaptive radix tree, which is the only index DuckDB builds",
			},
			{Name: "unique"}, {Name: "primary"},
			{Name: "comment"},
			{Name: "persistence", Desc: "temporary for an index on a temporary table, which is in the temp database, and permanent otherwise"},
			{Name: "definition", Desc: "the CREATE INDEX statement, as DuckDB renders it, and absent for the index of a constraint, which has none"},
			{Name: "using", Desc: "always art"},
			{Name: "constraint_type", Desc: "p or u for the index of a primary key or a unique constraint, which takes the name of the constraint, and absent for any other index"},
		},
		Params: schemaParentName("index"),
		Scan: func(rows *sql.Rows) (dbmeta.Index, error) {
			var v dbmeta.Index
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Table, &v.Name, &v.Type,
				&v.Unique, &v.Primary, &v.Comment, &v.Persistence, &v.Definition, &v.Using,
				&v.ConstraintType)
			return v, err
		},
	})

	registerIndexColumns()

	registerConstraints()
	registerNotNulls()

	dbmeta.Sequences.Register(dbmeta.DuckDB, &dbmeta.Binding[dbmeta.Sequence]{
		Stmt: dbmeta.Stmt{
			always(`SELECT s.schema_name AS "schema"`),
			always(`, s.sequence_name AS "name"`),
			always(`, 'BIGINT' AS "data_type"`),
			always(`, s.start_value AS "start"`),
			always(`, s.min_value AS "minimum"`),
			always(`, s.max_value AS "maximum"`),
			always(`, s.increment_by AS "increment"`),
			always(`, s.cycle AS "cycles"`),
			always(`, '' AS "owned_by"`),
			always(`, s.comment AS "comment"`),
			always(`, CAST(s.last_value AS VARCHAR) AS "last_value"`),
			always(`FROM duckdb_sequences() s`),
			always(`WHERE (@with_system OR TRUE)`),
			always(`AND (@schema = '' OR s.schema_name LIKE @schema)`),
			always(`AND (@name = '' OR s.sequence_name LIKE @name)`),
			always(`ORDER BY 1, 2`),
		},
		Fields: []dbmeta.Field{
			{Name: "schema"}, {Name: "name"},
			{
				Name: "data_type",
				Desc: "always BIGINT: every DuckDB sequence is one and the catalog does not record a type",
			},
			{Name: "start"}, {Name: "minimum"}, {Name: "maximum"}, {Name: "increment"},
			{Name: "cycles"},
			{
				Name: "owned_by",
				Desc: "always empty: a DuckDB sequence is independent of the column that reads it",
			},
			{Name: "comment"},
			{Name: "last_value", Desc: "the last value that the sequence gave, absent when it gave none"},
		},
		Params: schemaNameSystem("sequence"),
		Scan: func(rows *sql.Rows) (dbmeta.Sequence, error) {
			var v dbmeta.Sequence
			err := rows.Scan(&v.Schema, &v.Name, &v.DataType, &v.Start, &v.Minimum,
				&v.Maximum, &v.Increment, &v.Cycles, &v.OwnedBy, &v.Comment, &v.LastValue)
			return v, err
		},
	})

	dbmeta.Views.Register(dbmeta.DuckDB, &dbmeta.Binding[dbmeta.View]{
		Stmt: dbmeta.Stmt{
			always(`SELECT v.database_name AS "catalog"`),
			always(`, v.schema_name AS "schema"`),
			always(`, v.view_name AS "name"`),
			always(`, v.sql AS "definition"`),
			always(`, NULL AS "check_option"`),
			always(`, FALSE AS "updatable"`),
			always(`, FALSE AS "insertable"`),
			always(`, v.comment AS "comment"`),
			always(`FROM duckdb_views() v`),
			internalOf("WHERE", "v"),
			always(`AND (@schema = '' OR v.schema_name LIKE @schema)`),
			always(`AND (@name = '' OR v.view_name LIKE @name)`),
			always(`ORDER BY 2, 3`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog"}, {Name: "schema"}, {Name: "name"},
			{Name: "definition", Desc: "the whole CREATE VIEW statement, as DuckDB stores it"},
			{Name: "check_option", Desc: "always absent: DuckDB has no check option"},
			{Name: "updatable", Desc: "always false: a DuckDB view is read only"},
			{Name: "insertable", Desc: "always false, for the same reason"},
			{Name: "comment"},
		},
		Params: schemaNameSystem("view"),
		Scan: func(rows *sql.Rows) (dbmeta.View, error) {
			var v dbmeta.View
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Name, &v.Definition,
				&v.CheckOption, &v.Updatable, &v.Insertable, &v.Comment)
			return v, err
		},
	})

	// \dd. DuckDB puts a comment on the object rather than in a catalog of
	// comments, so this gathers them from the five catalogs that carry one.
	dbmeta.Comments.Register(dbmeta.DuckDB, &dbmeta.Binding[dbmeta.Comment]{
		Stmt: dbmeta.Stmt{
			always(`SELECT t.schema_name AS "schema", t.table_name AS "name"`),
			always(`, 'table' AS "type", t.comment AS "comment"`),
			always(`FROM duckdb_tables() t WHERE t.comment IS NOT NULL`),
			internalOf("AND", "t"),
			always(`AND (@schema = '' OR t.schema_name LIKE @schema)`),
			always(`AND (@name = '' OR t.table_name LIKE @name)`),
			always(`UNION ALL SELECT c.schema_name, c.table_name || '.' || c.column_name, 'column', c.comment`),
			always(`FROM duckdb_columns() c WHERE c.comment IS NOT NULL`),
			internalOf("AND", "c"),
			always(`AND (@schema = '' OR c.schema_name LIKE @schema)`),
			always(`AND (@name = '' OR c.column_name LIKE @name)`),
			always(`UNION ALL SELECT v.schema_name, v.view_name, 'view', v.comment`),
			always(`FROM duckdb_views() v WHERE v.comment IS NOT NULL`),
			internalOf("AND", "v"),
			always(`AND (@schema = '' OR v.schema_name LIKE @schema)`),
			always(`AND (@name = '' OR v.view_name LIKE @name)`),
			always(`UNION ALL SELECT s.schema_name, s.sequence_name, 'sequence', s.comment`),
			always(`FROM duckdb_sequences() s WHERE s.comment IS NOT NULL`),
			always(`AND (@schema = '' OR s.schema_name LIKE @schema)`),
			always(`AND (@name = '' OR s.sequence_name LIKE @name)`),
			always(`UNION ALL SELECT y.schema_name, y.type_name, 'type', y.comment`),
			always(`FROM duckdb_types() y WHERE y.comment IS NOT NULL`),
			internalOf("AND", "y"),
			always(`AND (@schema = '' OR y.schema_name LIKE @schema)`),
			always(`AND (@name = '' OR y.type_name LIKE @name)`),
			always(`ORDER BY 1, 3, 2`),
		},
		Fields: []dbmeta.Field{
			{Name: "schema"},
			{Name: "name", Desc: "the object, and table.column for a column"},
			{Name: "type", Desc: "table, column, view, sequence or type"},
			{Name: "comment"},
		},
		Params: schemaNameSystem("object"),
		Scan: func(rows *sql.Rows) (dbmeta.Comment, error) {
			var v dbmeta.Comment
			err := rows.Scan(&v.Schema, &v.Name, &v.Type, &v.Comment)
			return v, err
		},
	})
}

// notNullFilter leaves out the NOT NULL rows DuckDB records in its constraint
// catalog.
//
// D49 filters them on PostgreSQL 18 because earlier releases record a NOT NULL
// only on the column, so reporting it made the same schema answer differently
// by release. DuckDB always records it, so that reason does not apply, and a
// second one does: PostgreSQL never reports a NOT NULL constraint and DuckDB
// records one, so without the filter the same schema answers differently by
// database. Column nullable carries the fact in both.
//
// DeepSeek argued the other way, that duckdb_constraints is DuckDB's own
// catalog and hiding a row loses information. The information is not lost, and
// the constraint name is the only thing that is, which D49 already records as
// a known gap on PostgreSQL 18.
const notNullFilter = `AND k.constraint_type <> 'NOT NULL'`

// registerConstraints backs the constraint footers of \d name, and the kind
// that reports their columns.
func registerConstraints() {
	dbmeta.Constraints.Register(dbmeta.DuckDB, &dbmeta.Binding[dbmeta.Constraint]{
		Stmt: dbmeta.Stmt{
			always(`SELECT k.schema_name AS "schema"`),
			always(`, k.table_name AS "table"`),
			always(`, k.constraint_name AS "name"`),
			always(`, LOWER(k.constraint_type) AS "type"`),
			always(`, k.constraint_text AS "definition"`),
			always(`, FALSE AS "deferrable"`),
			always(`, FALSE AS "deferred"`),
			always(`, NULL AS "comment"`),
			always(`FROM duckdb_constraints() k`),
			always(`WHERE TRUE ` + notNullFilter),
			always(`AND (@with_system OR NOT k.schema_name IN ('information_schema', 'pg_catalog'))`),
			always(`AND (@schema = '' OR k.schema_name LIKE @schema)`),
			always(`AND (@parent = '' OR k.table_name LIKE @parent)`),
			always(`AND (@name = '' OR k.constraint_name LIKE @name)`),
			always(`ORDER BY 1, 2, 3`),
		},
		Fields: []dbmeta.Field{
			{Name: "schema"}, {Name: "table"}, {Name: "name"},
			{
				Name: "type",
				Desc: "primary key, unique, foreign key or check. A NOT NULL is never listed: read Column.Nullable, which every database fills. See D49",
			},
			{Name: "definition", Desc: "the constraint as DuckDB renders it"},
			{Name: "deferrable", Desc: "always false: DuckDB has no deferred constraints"},
			{Name: "deferred", Desc: "always false, for the same reason"},
			{Name: "comment", Desc: "always absent: DuckDB has no comment on a constraint"},
		},
		Params: schemaParentName("constraint"),
		Scan: func(rows *sql.Rows) (dbmeta.Constraint, error) {
			var v dbmeta.Constraint
			err := rows.Scan(&v.Schema, &v.Table, &v.Name, &v.Type, &v.Definition,
				&v.Deferrable, &v.Deferred, &v.Comment)
			return v, err
		},
	})

	// The columns of a constraint and the columns they point at are two lists
	// in the same row, in the same order. Unnesting one with ordinality and
	// indexing the other by it keeps a composite key together, which is the
	// same shape the PostgreSQL model uses on conkey and confkey.
	dbmeta.ConstraintColumns.Register(dbmeta.DuckDB, &dbmeta.Binding[dbmeta.ConstraintColumn]{
		Stmt: dbmeta.Stmt{
			always(`SELECT k.database_name AS "catalog"`),
			always(`, k.schema_name AS "schema"`),
			always(`, k.table_name AS "table"`),
			always(`, k.constraint_name AS "constraint"`),
			always(`, e.col AS "name"`),
			always(`, e.ordinality AS "ordinal"`),
			always(`, CASE WHEN k.referenced_table IS NULL THEN NULL` +
				` ELSE k.database_name END AS "foreign_catalog"`),
			always(`, CASE WHEN k.referenced_table IS NULL THEN NULL` +
				` ELSE k.schema_name END AS "foreign_schema"`),
			always(`, k.referenced_table AS "foreign_table"`),
			always(`, k.referenced_column_names[e.ordinality] AS "foreign_name"`),
			always(`FROM duckdb_constraints() k,` +
				` unnest(k.constraint_column_names) WITH ORDINALITY AS e(col, ordinality)`),
			always(`WHERE TRUE ` + notNullFilter),
			always(`AND (@with_system OR NOT k.schema_name IN ('information_schema', 'pg_catalog'))`),
			always(`AND (@schema = '' OR k.schema_name LIKE @schema)`),
			always(`AND (@parent = '' OR k.table_name LIKE @parent)`),
			always(`AND (@name = '' OR k.constraint_name LIKE @name)`),
			always(`ORDER BY 2, 3, 4, 6`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog"}, {Name: "schema"}, {Name: "table"}, {Name: "constraint"},
			{Name: "name", Desc: "the column the constraint is on"},
			{Name: "ordinal", Desc: "one based position within the constraint"},
			{Name: "foreign_catalog", Desc: "set for a foreign key, absent otherwise"},
			{
				Name: "foreign_schema",
				Desc: "set for a foreign key. DuckDB names only the referenced table, so this is the schema of the referring one",
			},
			{Name: "foreign_table"},
			{Name: "foreign_name", Desc: "the column this one points at, in the same position of the key"},
		},
		Params: schemaParentName("constraint"),
		Scan: func(rows *sql.Rows) (dbmeta.ConstraintColumn, error) {
			var v dbmeta.ConstraintColumn
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Table, &v.Constraint, &v.Name,
				&v.Ordinal, &v.ForeignCatalog, &v.ForeignSchema, &v.ForeignTable, &v.ForeignName)
			return v, err
		},
	})
}

// registerNotNulls answers NotNulls. DuckDB records each NOT NULL in
// duckdb_constraints and names it, such as tt_a_not_null. The Constraints query
// leaves the row out on purpose, because PostgreSQL before release 18 reports
// no constraint for it (D49), and this kind is the place for the name. A NOT
// NULL is on one column, so the list of columns has one element. DuckDB has no
// inheritance and no NOT VALID, so the three flags are fixed.
func registerNotNulls() {
	dbmeta.NotNulls.Register(dbmeta.DuckDB, &dbmeta.Binding[dbmeta.NotNull]{
		Stmt: dbmeta.Stmt{
			always(`SELECT k.schema_name AS "schema"`),
			always(`, k.table_name AS "table"`),
			always(`, k.constraint_name AS "name"`),
			always(`, k.constraint_column_names[1] AS "column"`),
			always(`, FALSE AS "no_inherit"`),
			always(`, TRUE AS "local"`),
			always(`, FALSE AS "inherited"`),
			always(`, TRUE AS "validated"`),
			always(`FROM duckdb_constraints() k`),
			always(`WHERE k.constraint_type = 'NOT NULL'`),
			always(`AND (@with_system OR NOT k.schema_name IN ('information_schema', 'pg_catalog'))`),
			always(`AND (@schema = '' OR k.schema_name LIKE @schema)`),
			always(`AND (@parent = '' OR k.table_name LIKE @parent)`),
			always(`AND (@name = '' OR k.constraint_name LIKE @name)`),
			always(`ORDER BY 1, 2, 3`),
		},
		Fields: []dbmeta.Field{
			{Name: "schema"}, {Name: "table"},
			{Name: "name", Desc: "the name DuckDB gives it, such as t_a_not_null"},
			{Name: "column", Desc: "the column the constraint is on"},
			{Name: "no_inherit", Desc: "always false: DuckDB has no inheritance"},
			{Name: "local", Desc: "always true, for the same reason"},
			{Name: "inherited", Desc: "always false, for the same reason"},
			{Name: "validated", Desc: "always true: DuckDB has no NOT VALID"},
		},
		Params: schemaParentName("constraint"),
		Scan: func(rows *sql.Rows) (dbmeta.NotNull, error) {
			var v dbmeta.NotNull
			err := rows.Scan(&v.Schema, &v.Table, &v.Name, &v.Column, &v.NoInherit,
				&v.Local, &v.Inherited, &v.Validated)
			return v, err
		},
	})
}

// registerIndexColumns answers IndexColumns, which is the kind that \d name
// of an index prints.
//
// duckdb_indexes.expressions has the type VARCHAR and prints as a list, such
// as ['"name"', '(upper("name"))']. Unnest refuses the text, but a cast to
// VARCHAR[] reads it as a list and keeps an element whole when it holds a
// comma, such as concat(a, b), or a quote. DuckDB writes a column bare or in
// double quotes, and anything it computes inside parentheses, so an element
// that starts with a parenthesis is an expression and any other is a column.
//
// The index of a primary key or a unique constraint is not in duckdb_indexes.
// Its columns are in duckdb_constraints, and the Indexes query lists it under
// the name of the constraint.
func registerIndexColumns() {
	dbmeta.IndexColumns.Register(dbmeta.DuckDB, &dbmeta.Binding[dbmeta.IndexColumn]{
		Stmt: dbmeta.Stmt{
			always(`SELECT x."schema", x."table", x."index"`),
			always(`, CASE WHEN x.item LIKE '(%' THEN NULL`),
			always(`  WHEN x.item LIKE '"%"' THEN replace(substr(x.item, 2, length(x.item) - 2), '""', '"')`),
			always(`  ELSE x.item END AS "name"`),
			always(`, x.ordinal AS "ordinal"`),
			always(`, CASE WHEN x.item LIKE '(%' THEN x.item ELSE NULL END AS "expression"`),
			always(`, NULL AS "descending"`),
			always(`FROM (`),
			always(`SELECT i.schema_name AS "schema", i.table_name AS "table", i.index_name AS "index"`),
			always(`, e.item AS item, e.ordinal AS ordinal`),
			always(`FROM duckdb_indexes() i,`),
			always(` unnest(CAST(i.expressions AS VARCHAR[])) WITH ORDINALITY AS e(item, ordinal)`),
			always(`UNION ALL`),
			always(`SELECT k.schema_name, k.table_name, k.constraint_name`),
			always(`, CASE WHEN e.col SIMILAR TO '[A-Za-z_][A-Za-z0-9_]*' THEN e.col`),
			always(`  ELSE '"' || replace(e.col, '"', '""') || '"' END`),
			always(`, e.ordinality`),
			always(`FROM duckdb_constraints() k,`),
			always(` unnest(k.constraint_column_names) WITH ORDINALITY AS e(col, ordinality)`),
			always(`WHERE k.constraint_type IN ('PRIMARY KEY', 'UNIQUE')`),
			always(`) x`),
			always(`WHERE (@schema = '' OR x."schema" LIKE @schema)`),
			always(`AND (@parent = '' OR x."table" LIKE @parent)`),
			always(`AND (@name = '' OR x."index" LIKE @name)`),
			always(`AND (@with_system OR x."schema" NOT IN ('information_schema', 'pg_catalog'))`),
			always(`ORDER BY 1, 2, 3, 5`),
		},
		Fields: []dbmeta.Field{
			{Name: "schema"}, {Name: "table"}, {Name: "index"},
			{Name: "name", Desc: "absent for a column the index computes rather than stores"},
			{Name: "ordinal", Desc: "one based position within the index"},
			{Name: "expression", Desc: "the expression, in the parentheses DuckDB writes it in, and absent for a column"},
			{Name: "descending", Desc: "always absent: duckdb_indexes does not publish the direction"},
		},
		Params: schemaParentName("index"),
		Scan: func(rows *sql.Rows) (dbmeta.IndexColumn, error) {
			var v dbmeta.IndexColumn
			err := rows.Scan(&v.Schema, &v.Table, &v.Index, &v.Name, &v.Ordinal,
				&v.Expression, &v.Descending)
			return v, err
		},
	})
}
