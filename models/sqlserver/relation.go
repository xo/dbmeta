package sqlserver

import (
	"database/sql"

	"github.com/xo/dbmeta"
)

// Schemas, tables, columns, indexes, constraints, sequences, views and
// triggers.

func registerRelations() {
	dbmeta.Schemas.Register(dbmeta.SQLServer, &dbmeta.Binding[dbmeta.Schema]{
		Stmt: dbmeta.Stmt{
			always(`SELECT DB_NAME() AS "catalog"`),
			always(`, s.name AS "name"`),
			always(`, COALESCE(p.name, '') AS "owner"`),
			always(`, NULL AS "comment"`),
			always(`FROM sys.schemas s`),
			always(`LEFT JOIN sys.database_principals p ON p.principal_id = s.principal_id`),
			always(`WHERE ` + notSystem),
			always(`AND (@name = '' OR s.name LIKE @name)`),
			always(`ORDER BY s.name`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "the database in use: a SQL Server schema belongs to one"},
			{Name: "name"},
			{Name: "owner", Desc: "the principal that owns the schema"},
			{Name: "comment", Desc: "always absent: SQL Server records no extended property on a schema this way"},
		},
		Params: []dbmeta.Param{
			{Name: "name", Desc: "schema name pattern, empty for every schema", Default: ""},
			{Name: "with_system", Desc: "include the schemas SQL Server keeps for itself", Default: false},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Schema, error) {
			var v dbmeta.Schema
			err := rows.Scan(&v.Catalog, &v.Name, &v.Owner, &v.Comment)
			return v, err
		},
	})

	// \l. A caller sees the databases it may see, which is every one for a
	// sysadmin and its own for anybody else.
	dbmeta.Databases.Register(dbmeta.SQLServer, &dbmeta.Binding[dbmeta.Database]{
		Stmt: dbmeta.Stmt{
			always(`SELECT d.name AS "name"`),
			always(`, COALESCE(SUSER_SNAME(d.owner_sid), '') AS "owner"`),
			always(`, CASE WHEN d.collation_name LIKE '%UTF8%' THEN 'UTF8' ELSE 'UCS-2' END AS "encoding"`),
			always(`, COALESCE(d.collation_name, '') AS "collate"`),
			always(`, COALESCE(d.collation_name, '') AS "ctype"`),
			always(`, NULL AS "access"`),
			always(`, NULL AS "tablespace"`),
			always(`, '' AS "size"`),
			always(`, NULL AS "comment"`),
			always(`FROM sys.databases d`),
			always(`WHERE (@with_system = 1 OR d.database_id > 4)`),
			always(`AND (@name = '' OR d.name LIKE @name)`),
			always(`ORDER BY d.name`),
		},
		Fields: []dbmeta.Field{
			{Name: "name"},
			{Name: "owner", Desc: "empty where the caller may not resolve the owner's login"},
			{
				Name: "encoding",
				Desc: "UTF8 for a UTF-8 collation and UCS-2 otherwise, which is what nvarchar stores",
			},
			{Name: "collate", Desc: "the database collation, which covers both collate and ctype here"},
			{Name: "ctype", Desc: "the same value: SQL Server has one collation rather than two"},
			{Name: "access", Desc: "always absent: read privileges instead"},
			{Name: "tablespace", Desc: "always absent: a filegroup belongs to a database rather than the other way round"},
			{Name: "size", Desc: "always empty: a size needs sys.master_files and a per database read"},
			{Name: "comment", Desc: "always absent"},
		},
		Params: []dbmeta.Param{
			{Name: "name", Desc: "database name pattern, empty for every one", Default: ""},
			{
				Name:    "with_system",
				Desc:    "include master, tempdb, model and msdb, which have ids 1 to 4",
				Default: false,
			},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Database, error) {
			var v dbmeta.Database
			err := rows.Scan(&v.Name, &v.Owner, &v.Encoding, &v.Collate, &v.CType,
				&v.Access, &v.Tablespace, &v.Size, &v.Comment)
			return v, err
		},
	})

	// \d, \dt, \dv. Tables and views are separate catalog views here, so they
	// are unioned, which is what psql lists together.
	dbmeta.Tables.Register(dbmeta.SQLServer, &dbmeta.Binding[dbmeta.Table]{
		Stmt: dbmeta.Stmt{
			always(`SELECT DB_NAME() AS "catalog"`),
			always(`, s.name AS "schema"`),
			always(`, t.name AS "name"`),
			// temporal_type and is_external arrived in 2016. An older server
			// has neither kind of table, so the padded alternative is not
			// missing anything it could have reported.
			{
				{Query: `, 'table' AS "type"`},
				{Min: v13, Query: `, CASE WHEN t.temporal_type = 2 THEN 'system versioned table'` +
					` WHEN t.is_external = 1 THEN 'external table' ELSE 'table' END AS "type"`},
			},
			always(`, ` + commentOn("t.object_id") + ` AS "comment"`),
			always(`FROM sys.tables t`),
			always(`JOIN sys.schemas s ON s.schema_id = t.schema_id`),
			always(`WHERE ` + notSystem),
			always(`AND (@schema = '' OR s.name LIKE @schema)`),
			always(`AND (@name = '' OR t.name LIKE @name)`),
			always(`UNION ALL`),
			always(`SELECT DB_NAME(), s.name, v.name, 'view', ` + commentOn("v.object_id")),
			always(`FROM sys.views v`),
			always(`JOIN sys.schemas s ON s.schema_id = v.schema_id`),
			always(`WHERE ` + notSystem),
			always(`AND (@schema = '' OR s.name LIKE @schema)`),
			always(`AND (@name = '' OR v.name LIKE @name)`),
			always(`ORDER BY 2, 3`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog"}, {Name: "schema"}, {Name: "name"},
			{
				Name: "type",
				Desc: "table, view, external table, or system versioned table for a temporal one",
			},
			{Name: "comment", Desc: "the MS_Description extended property, which is what SQL Server has instead of a comment"},
		},
		Params: schemaNameSystem("table"),
		Scan: func(rows *sql.Rows) (dbmeta.Table, error) {
			var v dbmeta.Table
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Name, &v.Type, &v.Comment)
			return v, err
		},
	})

	// \d name. sys.columns covers a view's columns as well as a table's, so
	// this needs no union.
	dbmeta.Columns.Register(dbmeta.SQLServer, &dbmeta.Binding[dbmeta.Column]{
		Stmt: dbmeta.Stmt{
			always(`SELECT DB_NAME() AS "catalog"`),
			always(`, s.name AS "schema"`),
			always(`, o.name AS "table"`),
			always(`, c.name AS "name"`),
			always(`, c.column_id AS "ordinal"`),
			// The declared type with its length, precision and scale, which
			// is what a person reads and what psql prints.
			always(`, ty.name + CASE` +
				` WHEN ty.name IN ('varchar', 'char', 'varbinary', 'binary')` +
				`  THEN '(' + CASE WHEN c.max_length = -1 THEN 'max'` +
				`   ELSE CAST(c.max_length AS varchar(8)) END + ')'` +
				` WHEN ty.name IN ('nvarchar', 'nchar')` +
				`  THEN '(' + CASE WHEN c.max_length = -1 THEN 'max'` +
				`   ELSE CAST(c.max_length / 2 AS varchar(8)) END + ')'` +
				` WHEN ty.name IN ('decimal', 'numeric')` +
				`  THEN '(' + CAST(c.precision AS varchar(8)) + ',' + CAST(c.scale AS varchar(8)) + ')'` +
				` ELSE '' END AS "data_type"`),
			always(`, c.is_nullable AS "nullable"`),
			always(`, d.definition AS "default"`),
			// One more join, to the primary key index of the object.
			always(`, CAST(CASE WHEN EXISTS (SELECT 1 FROM sys.index_columns ic` +
				` JOIN sys.indexes i ON i.object_id = ic.object_id AND i.index_id = ic.index_id` +
				` WHERE i.is_primary_key = 1 AND ic.object_id = c.object_id` +
				` AND ic.column_id = c.column_id) THEN 1 ELSE 0 END AS bit) AS "primary_key"`),
			always(`, CASE WHEN c.is_identity = 1 THEN 'identity' ELSE NULL END AS "identity"`),
			always(`, CASE WHEN c.is_computed = 1 THEN` +
				` CASE WHEN cc.is_persisted = 1 THEN 'stored' ELSE 'virtual' END` +
				` ELSE NULL END AS "generated"`),
			always(`, ` + columnComment("c.object_id", "c.column_id") + ` AS "comment"`),
			always(`FROM sys.columns c`),
			always(`JOIN sys.objects o ON o.object_id = c.object_id`),
			always(`JOIN sys.schemas s ON s.schema_id = o.schema_id`),
			always(`JOIN sys.types ty ON ty.user_type_id = c.user_type_id`),
			always(`LEFT JOIN sys.default_constraints d ON d.object_id = c.default_object_id`),
			always(`LEFT JOIN sys.computed_columns cc` +
				` ON cc.object_id = c.object_id AND cc.column_id = c.column_id`),
			always(`WHERE o.type IN ('U', 'V')`),
			always(`AND ` + notSystem),
			always(`AND (@schema = '' OR s.name LIKE @schema)`),
			always(`AND (@parent = '' OR o.name LIKE @parent)`),
			always(`AND (@name = '' OR c.name LIKE @name)`),
			always(`ORDER BY 2, 3, 5`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog"}, {Name: "schema"}, {Name: "table"}, {Name: "name"},
			{Name: "ordinal", Desc: "one based, which is how column_id counts"},
			{
				Name: "data_type",
				Desc: "the declared type with its length, precision and scale. An nvarchar length is halved, because max_length counts bytes",
			},
			{Name: "nullable"},
			{Name: "default", Desc: "the default expression as SQL Server stores it, in brackets"},
			{Name: "primary_key"},
			{Name: "identity", Desc: "identity for an IDENTITY column, and absent otherwise"},
			{Name: "generated", Desc: "stored for a persisted computed column, virtual for one computed on read"},
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

	dbmeta.Indexes.Register(dbmeta.SQLServer, &dbmeta.Binding[dbmeta.Index]{
		Stmt: dbmeta.Stmt{
			always(`SELECT DB_NAME() AS "catalog"`),
			always(`, s.name AS "schema"`),
			always(`, o.name AS "table"`),
			always(`, i.name AS "name"`),
			always(`, LOWER(i.type_desc) AS "type"`),
			always(`, i.is_unique AS "unique"`),
			always(`, i.is_primary_key AS "primary"`),
			always(`, ` + commentOn("i.object_id") + ` AS "comment"`),
			always(`FROM sys.indexes i`),
			always(`JOIN sys.objects o ON o.object_id = i.object_id`),
			always(`JOIN sys.schemas s ON s.schema_id = o.schema_id`),
			// index_id 0 is the heap, which is the absence of an index
			always(`WHERE i.index_id > 0 AND i.name IS NOT NULL`),
			always(`AND ` + notSystem),
			always(`AND (@schema = '' OR s.name LIKE @schema)`),
			always(`AND (@parent = '' OR o.name LIKE @parent)`),
			always(`AND (@name = '' OR i.name LIKE @name)`),
			always(`ORDER BY 2, 3, 4`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog"}, {Name: "schema"}, {Name: "table"}, {Name: "name"},
			{
				Name: "type",
				Desc: "clustered, nonclustered, xml, spatial, clustered columnstore or nonclustered columnstore",
			},
			{Name: "unique"}, {Name: "primary"},
			{Name: "comment", Desc: "the extended property on the table, because SQL Server puts none on an index"},
		},
		Params: schemaParentName("index"),
		Scan: func(rows *sql.Rows) (dbmeta.Index, error) {
			var v dbmeta.Index
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Table, &v.Name, &v.Type,
				&v.Unique, &v.Primary, &v.Comment)
			return v, err
		},
	})

	dbmeta.IndexColumns.Register(dbmeta.SQLServer, &dbmeta.Binding[dbmeta.IndexColumn]{
		Stmt: dbmeta.Stmt{
			always(`SELECT s.name AS "schema"`),
			always(`, o.name AS "table"`),
			always(`, i.name AS "index"`),
			always(`, c.name AS "name"`),
			always(`, ic.key_ordinal AS "ordinal"`),
			// An included column is not part of the key and SQL Server gives
			// it key_ordinal 0, which is reported rather than hidden.
			always(`, CASE WHEN ic.is_included_column = 1 THEN 'included' ELSE NULL END AS "expression"`),
			always(`, ic.is_descending_key AS "descending"`),
			always(`FROM sys.index_columns ic`),
			always(`JOIN sys.indexes i ON i.object_id = ic.object_id AND i.index_id = ic.index_id`),
			always(`JOIN sys.objects o ON o.object_id = ic.object_id`),
			always(`JOIN sys.schemas s ON s.schema_id = o.schema_id`),
			always(`JOIN sys.columns c ON c.object_id = ic.object_id AND c.column_id = ic.column_id`),
			always(`WHERE i.name IS NOT NULL`),
			always(`AND ` + notSystem),
			always(`AND (@schema = '' OR s.name LIKE @schema)`),
			always(`AND (@parent = '' OR o.name LIKE @parent)`),
			always(`AND (@name = '' OR i.name LIKE @name)`),
			always(`ORDER BY 1, 2, 3, 5`),
		},
		Fields: []dbmeta.Field{
			{Name: "schema"}, {Name: "table"}, {Name: "index"}, {Name: "name"},
			{Name: "ordinal", Desc: "one based position in the key, and zero for an included column"},
			{
				Name: "expression",
				Desc: "the word included for a column the index carries and does not sort by. SQL Server has no expression index",
			},
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

	registerConstraints()

	// sys.sequences arrived in 2012, and so did CREATE SEQUENCE. An older
	// server has no sequences at all, which is why the whole statement is
	// gated rather than padded.
	dbmeta.Sequences.Register(dbmeta.SQLServer, &dbmeta.Binding[dbmeta.Sequence]{
		Stmt: dbmeta.Stmt{
			{{Min: v11, Query: `SELECT s.name AS "schema"`}},
			{{Min: v11, Query: `, q.name AS "name"`}},
			{{Min: v11, Query: `, ty.name AS "data_type"`}},
			{{Min: v11, Query: `, CAST(q.start_value AS bigint) AS "start"`}},
			{{Min: v11, Query: `, CAST(q.minimum_value AS bigint) AS "minimum"`}},
			{{Min: v11, Query: `, CAST(q.maximum_value AS bigint) AS "maximum"`}},
			{{Min: v11, Query: `, CAST(q.increment AS bigint) AS "increment"`}},
			{{Min: v11, Query: `, q.is_cycling AS "cycles"`}},
			{{Min: v11, Query: `, '' AS "owned_by"`}},
			{{Min: v11, Query: `, ` + commentOn("q.object_id") + ` AS "comment"`}},
			{{Min: v11, Query: `FROM sys.sequences q`}},
			{{Min: v11, Query: `JOIN sys.schemas s ON s.schema_id = q.schema_id`}},
			{{Min: v11, Query: `JOIN sys.types ty ON ty.user_type_id = q.user_type_id`}},
			{{Min: v11, Query: `WHERE ` + notSystem}},
			{{Min: v11, Query: `AND (@schema = '' OR s.name LIKE @schema)`}},
			{{Min: v11, Query: `AND (@name = '' OR q.name LIKE @name)`}},
			{{Min: v11, Query: `ORDER BY 1, 2`}},
		},
		Fields: []dbmeta.Field{
			{Name: "schema"}, {Name: "name"},
			{Name: "data_type", Desc: "the type the sequence was declared with, which SQL Server records"},
			{Name: "start"}, {Name: "minimum"}, {Name: "maximum"}, {Name: "increment"},
			{Name: "cycles"},
			{
				Name: "owned_by",
				Desc: "always empty: a SQL Server sequence is independent of the column that reads it",
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

	dbmeta.Views.Register(dbmeta.SQLServer, &dbmeta.Binding[dbmeta.View]{
		Stmt: dbmeta.Stmt{
			always(`SELECT DB_NAME() AS "catalog"`),
			always(`, s.name AS "schema"`),
			always(`, v.name AS "name"`),
			always(`, COALESCE(m.definition, '') AS "definition"`),
			always(`, CASE WHEN v.with_check_option = 1 THEN 'cascaded' ELSE 'none' END AS "check_option"`),
			always(`, NULL AS "updatable"`),
			always(`, NULL AS "insertable"`),
			always(`, ` + commentOn("v.object_id") + ` AS "comment"`),
			always(`FROM sys.views v`),
			always(`JOIN sys.schemas s ON s.schema_id = v.schema_id`),
			always(`LEFT JOIN sys.sql_modules m ON m.object_id = v.object_id`),
			always(`WHERE ` + notSystem),
			always(`AND (@schema = '' OR s.name LIKE @schema)`),
			always(`AND (@name = '' OR v.name LIKE @name)`),
			always(`ORDER BY 2, 3`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog"}, {Name: "schema"}, {Name: "name"},
			{
				Name: "definition",
				Desc: "the whole CREATE VIEW statement as written. It is empty where the view was created WITH ENCRYPTION or where the caller may not see the definition",
			},
			{Name: "check_option", Desc: "cascaded for WITH CHECK OPTION, and none otherwise"},
			{
				Name: "updatable",
				Desc: "always absent: SQL Server decides at statement time and publishes no flag",
			},
			{Name: "insertable", Desc: "always absent, for the same reason"},
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

	// \d name. A trigger on a table, which is parent_class 1. A trigger on the
	// database is an event trigger and is registered separately.
	dbmeta.Triggers.Register(dbmeta.SQLServer, &dbmeta.Binding[dbmeta.Trigger]{
		Stmt: dbmeta.Stmt{
			always(`SELECT s.name AS "schema"`),
			always(`, o.name AS "table"`),
			always(`, tr.name AS "name"`),
			always(`, CASE WHEN tr.is_disabled = 1 THEN 'disabled' ELSE 'enabled' END AS "enabled"`),
			always(`, COALESCE(m.definition, '') AS "definition"`),
			always(`, ` + commentOn("tr.object_id") + ` AS "comment"`),
			always(`FROM sys.triggers tr`),
			always(`JOIN sys.objects o ON o.object_id = tr.parent_id`),
			always(`JOIN sys.schemas s ON s.schema_id = o.schema_id`),
			always(`LEFT JOIN sys.sql_modules m ON m.object_id = tr.object_id`),
			always(`WHERE tr.parent_class = 1`),
			always(`AND ` + notSystem),
			always(`AND (@schema = '' OR s.name LIKE @schema)`),
			always(`AND (@parent = '' OR o.name LIKE @parent)`),
			always(`AND (@name = '' OR tr.name LIKE @name)`),
			always(`ORDER BY 1, 2, 3`),
		},
		Fields: []dbmeta.Field{
			{Name: "schema"}, {Name: "table"}, {Name: "name"},
			{Name: "enabled", Desc: "enabled or disabled: SQL Server can disable one trigger"},
			{
				Name: "definition",
				Desc: "the CREATE TRIGGER statement, empty where it was encrypted or the caller may not see it",
			},
			{Name: "comment"},
		},
		Params: schemaParentName("trigger"),
		Scan: func(rows *sql.Rows) (dbmeta.Trigger, error) {
			var v dbmeta.Trigger
			err := rows.Scan(&v.Schema, &v.Table, &v.Name, &v.Enabled, &v.Definition, &v.Comment)
			return v, err
		},
	})
}

// registerConstraints backs the constraint footers of \d name, and the kind
// that reports their columns.
//
// SQL Server keeps each kind in its own catalog view, so both queries union
// them. A NOT NULL is not among them, which is the one place SQL Server agrees
// with what D49 decided for everybody else: nullability is on the column.
func registerConstraints() {
	dbmeta.Constraints.Register(dbmeta.SQLServer, &dbmeta.Binding[dbmeta.Constraint]{
		Stmt: dbmeta.Stmt{
			// a primary key or a unique constraint, which SQL Server records
			// as a key constraint backed by an index
			always(`SELECT s.name AS "schema"`),
			always(`, o.name AS "table"`),
			always(`, k.name AS "name"`),
			always(`, CASE k.type WHEN 'PK' THEN 'primary key' ELSE 'unique' END AS "type"`),
			always(`, NULL AS "definition"`),
			always(`, CAST(0 AS bit) AS "deferrable"`),
			always(`, CAST(0 AS bit) AS "deferred"`),
			always(`, ` + commentOn("k.object_id") + ` AS "comment"`),
			always(`FROM sys.key_constraints k`),
			always(`JOIN sys.objects o ON o.object_id = k.parent_object_id`),
			always(`JOIN sys.schemas s ON s.schema_id = o.schema_id`),
			always(`WHERE ` + notSystem),
			always(`AND (@schema = '' OR s.name LIKE @schema)`),
			always(`AND (@parent = '' OR o.name LIKE @parent)`),
			always(`AND (@name = '' OR k.name LIKE @name)`),

			always(`UNION ALL`),
			always(`SELECT s.name, o.name, f.name, 'foreign key', NULL,` +
				` CAST(0 AS bit), CAST(0 AS bit), ` + commentOn("f.object_id")),
			always(`FROM sys.foreign_keys f`),
			always(`JOIN sys.objects o ON o.object_id = f.parent_object_id`),
			always(`JOIN sys.schemas s ON s.schema_id = o.schema_id`),
			always(`WHERE ` + notSystem),
			always(`AND (@schema = '' OR s.name LIKE @schema)`),
			always(`AND (@parent = '' OR o.name LIKE @parent)`),
			always(`AND (@name = '' OR f.name LIKE @name)`),

			always(`UNION ALL`),
			always(`SELECT s.name, o.name, c.name, 'check', c.definition,` +
				` CAST(0 AS bit), CAST(0 AS bit), ` + commentOn("c.object_id")),
			always(`FROM sys.check_constraints c`),
			always(`JOIN sys.objects o ON o.object_id = c.parent_object_id`),
			always(`JOIN sys.schemas s ON s.schema_id = o.schema_id`),
			always(`WHERE ` + notSystem),
			always(`AND (@schema = '' OR s.name LIKE @schema)`),
			always(`AND (@parent = '' OR o.name LIKE @parent)`),
			always(`AND (@name = '' OR c.name LIKE @name)`),

			// A default is a named constraint here, which no other database
			// models. It is reported, because a caller reading \d name would
			// see it in the DDL and Column.Default holds the same expression.
			always(`UNION ALL`),
			always(`SELECT s.name, o.name, d.name, 'default', d.definition,` +
				` CAST(0 AS bit), CAST(0 AS bit), ` + commentOn("d.object_id")),
			always(`FROM sys.default_constraints d`),
			always(`JOIN sys.objects o ON o.object_id = d.parent_object_id`),
			always(`JOIN sys.schemas s ON s.schema_id = o.schema_id`),
			always(`WHERE ` + notSystem),
			always(`AND (@schema = '' OR s.name LIKE @schema)`),
			always(`AND (@parent = '' OR o.name LIKE @parent)`),
			always(`AND (@name = '' OR d.name LIKE @name)`),
			always(`ORDER BY 1, 2, 4, 3`),
		},
		Fields: []dbmeta.Field{
			{Name: "schema"}, {Name: "table"},
			{
				Name: "name",
				Desc: "SQL Server generates a name where the DDL gave none, such as PK__author__86516BCF",
			},
			{
				Name: "type",
				Desc: "primary key, unique, foreign key, check, or default. A default is a named constraint here and nowhere else",
			},
			{
				Name: "definition",
				Desc: "the expression, for a check or a default. Absent for a key, whose columns are in constraint_columns",
			},
			{Name: "deferrable", Desc: "always false: SQL Server has no deferred constraint"},
			{Name: "deferred", Desc: "always false, for the same reason"},
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

	// The columns of a key come from the index that backs it, and the columns
	// of a foreign key from their own catalog view, which carries the
	// referenced side in the same row.
	dbmeta.ConstraintColumns.Register(dbmeta.SQLServer, &dbmeta.Binding[dbmeta.ConstraintColumn]{
		Stmt: dbmeta.Stmt{
			always(`SELECT DB_NAME() AS "catalog"`),
			always(`, s.name AS "schema"`),
			always(`, o.name AS "table"`),
			always(`, k.name AS "constraint"`),
			always(`, c.name AS "name"`),
			always(`, CAST(ic.key_ordinal AS bigint) AS "ordinal"`),
			always(`, NULL AS "foreign_catalog"`),
			always(`, NULL AS "foreign_schema"`),
			always(`, NULL AS "foreign_table"`),
			always(`, NULL AS "foreign_name"`),
			always(`FROM sys.key_constraints k`),
			always(`JOIN sys.objects o ON o.object_id = k.parent_object_id`),
			always(`JOIN sys.schemas s ON s.schema_id = o.schema_id`),
			always(`JOIN sys.index_columns ic` +
				` ON ic.object_id = k.parent_object_id AND ic.index_id = k.unique_index_id`),
			always(`JOIN sys.columns c ON c.object_id = ic.object_id AND c.column_id = ic.column_id`),
			always(`WHERE ic.is_included_column = 0`),
			always(`AND ` + notSystem),
			always(`AND (@schema = '' OR s.name LIKE @schema)`),
			always(`AND (@parent = '' OR o.name LIKE @parent)`),
			always(`AND (@name = '' OR k.name LIKE @name)`),

			always(`UNION ALL`),
			always(`SELECT DB_NAME(), s.name, o.name, f.name, c.name,` +
				` CAST(fc.constraint_column_id AS bigint),` +
				` DB_NAME(), rs.name, ro.name, rc.name`),
			always(`FROM sys.foreign_keys f`),
			always(`JOIN sys.foreign_key_columns fc ON fc.constraint_object_id = f.object_id`),
			always(`JOIN sys.objects o ON o.object_id = fc.parent_object_id`),
			always(`JOIN sys.schemas s ON s.schema_id = o.schema_id`),
			always(`JOIN sys.columns c ON c.object_id = fc.parent_object_id AND c.column_id = fc.parent_column_id`),
			always(`JOIN sys.objects ro ON ro.object_id = fc.referenced_object_id`),
			always(`JOIN sys.schemas rs ON rs.schema_id = ro.schema_id`),
			always(`JOIN sys.columns rc` +
				` ON rc.object_id = fc.referenced_object_id AND rc.column_id = fc.referenced_column_id`),
			always(`WHERE ` + notSystem),
			always(`AND (@schema = '' OR s.name LIKE @schema)`),
			always(`AND (@parent = '' OR o.name LIKE @parent)`),
			always(`AND (@name = '' OR f.name LIKE @name)`),
			always(`ORDER BY 2, 3, 4, 6`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog"}, {Name: "schema"}, {Name: "table"}, {Name: "constraint"},
			{Name: "name", Desc: "the column the constraint is on"},
			{Name: "ordinal", Desc: "one based position within the constraint"},
			{Name: "foreign_catalog", Desc: "set for a foreign key: SQL Server cannot reference another database"},
			{Name: "foreign_schema"}, {Name: "foreign_table"},
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
