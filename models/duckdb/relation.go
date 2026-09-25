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
			always(`WHERE ` + internalOf("s")),
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
			always(`, '' AS "size"`),
			always(`, d.comment AS "comment"`),
			always(`FROM duckdb_databases() d`),
			always(`WHERE ` + internalOf("d")),
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
			{Name: "size", Desc: "always empty: DuckDB publishes no database size"},
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
			always(`FROM duckdb_tables() t`),
			always(`WHERE ` + internalOf("t")),
			always(`AND (@schema = '' OR t.schema_name LIKE @schema)`),
			always(`AND (@name = '' OR t.table_name LIKE @name)`),
			always(`UNION ALL`),
			always(`SELECT v.database_name, v.schema_name, v.view_name`),
			always(`, CASE WHEN v.temporary THEN 'temporary view' ELSE 'view' END`),
			always(`, v.comment`),
			always(`FROM duckdb_views() v`),
			always(`WHERE ` + internalOf("v")),
			always(`AND (@schema = '' OR v.schema_name LIKE @schema)`),
			always(`AND (@name = '' OR v.view_name LIKE @name)`),
			always(`ORDER BY 1, 2, 3`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog"}, {Name: "schema"}, {Name: "name"},
			{Name: "type", Desc: "table or view, with temporary in front where it is one"},
			{Name: "comment"},
		},
		Params: schemaNameSystem("table"),
		Scan: func(rows *sql.Rows) (dbmeta.Table, error) {
			var v dbmeta.Table
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Name, &v.Type, &v.Comment)
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
			always(`, c.column_default AS "default"`),
			// One more read of the constraint catalog, which is small and in
			// memory. See D47.
			always(`, EXISTS (SELECT 1 FROM duckdb_constraints() k` +
				` WHERE k.table_oid = c.table_oid AND k.constraint_type = 'PRIMARY KEY'` +
				` AND LIST_CONTAINS(k.constraint_column_names, c.column_name)) AS "primary_key"`),
			always(`, NULL AS "identity"`),
			always(`, NULL AS "generated"`),
			always(`, c.comment AS "comment"`),
			always(`FROM duckdb_columns() c`),
			always(`WHERE ` + internalOf("c")),
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
			{Name: "default", Desc: "the default expression as written"},
			{Name: "primary_key"},
			{
				Name: "identity",
				Desc: "always absent: DuckDB has no identity column, and a column that defaults to nextval is a default rather than a kind",
			},
			{
				Name: "generated",
				Desc: "always absent: duckdb_columns does not say whether a column is generated",
			},
			{Name: "comment"},
		},
		Params: schemaParentName("column"),
		Scan: func(rows *sql.Rows) (dbmeta.Column, error) {
			var v dbmeta.Column
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Table, &v.Name, &v.Ordinal,
				&v.DataType, &v.Nullable, &v.Default, &v.PrimaryKey, &v.Identity,
				&v.Generated, &v.Comment)
			return v, err
		},
	})

	// \di. DuckDB creates an index for a UNIQUE or PRIMARY KEY constraint and
	// lists it here with is_primary set, the same way SQLite does.
	dbmeta.Indexes.Register(dbmeta.DuckDB, &dbmeta.Binding[dbmeta.Index]{
		Stmt: dbmeta.Stmt{
			always(`SELECT i.database_name AS "catalog"`),
			always(`, i.schema_name AS "schema"`),
			always(`, i.table_name AS "table"`),
			always(`, i.index_name AS "name"`),
			always(`, 'art' AS "type"`),
			always(`, i.is_unique AS "unique"`),
			always(`, i.is_primary AS "primary"`),
			always(`, i.comment AS "comment"`),
			always(`FROM duckdb_indexes() i`),
			always(`WHERE (@schema = '' OR i.schema_name LIKE @schema)`),
			always(`AND (@parent = '' OR i.table_name LIKE @parent)`),
			always(`AND (@name = '' OR i.index_name LIKE @name)`),
			always(`AND (@with_system OR TRUE)`),
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
		},
		Params: schemaParentName("index"),
		Scan: func(rows *sql.Rows) (dbmeta.Index, error) {
			var v dbmeta.Index
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Table, &v.Name, &v.Type,
				&v.Unique, &v.Primary, &v.Comment)
			return v, err
		},
	})

	// \d name is not answered. IndexColumns is unsupported, and this is the
	// one place DuckDB looks like it can answer and cannot.
	//
	// duckdb_indexes.expressions prints as [title] and its type is VARCHAR,
	// not VARCHAR[]. It is a rendering of a list rather than a list, so
	// unnest refuses it, and splitting the text on a comma breaks the moment
	// an index is on an expression that contains one, such as concat(a, b).
	// duckdb_constraints.constraint_column_names is a real VARCHAR[], which
	// is why ConstraintColumns works and this does not.
	//
	// The columns of a primary key or a unique constraint are readable from
	// ConstraintColumns. The columns of an index someone created are not
	// readable at all.

	registerConstraints()

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
		},
		Params: schemaNameSystem("sequence"),
		Scan: func(rows *sql.Rows) (dbmeta.Sequence, error) {
			var v dbmeta.Sequence
			err := rows.Scan(&v.Schema, &v.Name, &v.DataType, &v.Start, &v.Minimum,
				&v.Maximum, &v.Increment, &v.Cycles, &v.OwnedBy, &v.Comment)
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
			always(`WHERE ` + internalOf("v")),
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
			always(`AND ` + internalOf("t")),
			always(`AND (@schema = '' OR t.schema_name LIKE @schema)`),
			always(`AND (@name = '' OR t.table_name LIKE @name)`),
			always(`UNION ALL SELECT c.schema_name, c.table_name || '.' || c.column_name, 'column', c.comment`),
			always(`FROM duckdb_columns() c WHERE c.comment IS NOT NULL`),
			always(`AND ` + internalOf("c")),
			always(`AND (@schema = '' OR c.schema_name LIKE @schema)`),
			always(`AND (@name = '' OR c.column_name LIKE @name)`),
			always(`UNION ALL SELECT v.schema_name, v.view_name, 'view', v.comment`),
			always(`FROM duckdb_views() v WHERE v.comment IS NOT NULL`),
			always(`AND ` + internalOf("v")),
			always(`AND (@schema = '' OR v.schema_name LIKE @schema)`),
			always(`AND (@name = '' OR v.view_name LIKE @name)`),
			always(`UNION ALL SELECT s.schema_name, s.sequence_name, 'sequence', s.comment`),
			always(`FROM duckdb_sequences() s WHERE s.comment IS NOT NULL`),
			always(`AND (@schema = '' OR s.schema_name LIKE @schema)`),
			always(`AND (@name = '' OR s.sequence_name LIKE @name)`),
			always(`UNION ALL SELECT y.schema_name, y.type_name, 'type', y.comment`),
			always(`FROM duckdb_types() y WHERE y.comment IS NOT NULL`),
			always(`AND ` + internalOf("y")),
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
// would, so the same schema would answer differently by database. Column
// nullable carries the fact in both.
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
