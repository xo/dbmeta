package clickhouse

import (
	"database/sql"

	"github.com/xo/dbmeta"
)

func registerExtra() {
	// \di. A ClickHouse index is a data skipping index: it holds a summary
	// per granule and lets the reader skip granules that cannot match. It
	// does not point at rows and it enforces nothing.
	dbmeta.Indexes.Register(dbmeta.ClickHouse, &dbmeta.Binding[dbmeta.Index]{
		Stmt: dbmeta.Stmt{
			always(`SELECT '' AS "catalog"`),
			always(`, i.database AS "schema"`),
			always(`, i.table AS "table"`),
			always(`, i.name AS "name"`),
			always(`, i.type AS "type"`),
			always(`, 0 AS "unique"`),
			always(`, 0 AS "primary"`),
			always(`, NULL AS "comment"`),
			always(`FROM system.data_skipping_indices i`),
			always(`WHERE ` + notSystem("i.database")),
			always(`AND (@schema = '' OR i.database LIKE @schema)`),
			always(`AND (@name = '' OR i.table LIKE @name)`),
			always(`ORDER BY i.database, i.table, i.name`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "always empty: ClickHouse has nothing above a database"},
			{Name: "schema"}, {Name: "table"}, {Name: "name"},
			{Name: "type", Desc: "the skipping index type, such as minmax, set or bloom_filter"},
			{Name: "unique", Desc: "always false: a skipping index enforces nothing"},
			{
				Name: "primary",
				Desc: "always false: the primary key is the sorting order of the" +
					" table rather than an index, and the columns query flags it",
			},
			{Name: "comment", Desc: "always absent: an index carries no comment"},
		},
		Params: schemaNameSystem("table"),
		Scan: func(rows *sql.Rows) (dbmeta.Index, error) {
			var v dbmeta.Index
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Table, &v.Name, &v.Type,
				&v.Unique, &v.Primary, &v.Comment)
			return v, err
		},
	})

	// The expression an index is on. ClickHouse keeps it as written rather
	// than as a column list, because a skipping index is usually on an
	// expression and not on a bare column.
	dbmeta.IndexColumns.Register(dbmeta.ClickHouse, &dbmeta.Binding[dbmeta.IndexColumn]{
		Stmt: dbmeta.Stmt{
			always(`SELECT i.database AS "schema"`),
			always(`, i.table AS "table"`),
			always(`, i.name AS "index"`),
			always(`, i.expr AS "name"`),
			always(`, 1 AS "ordinal"`),
			always(`, i.expr AS "expression"`),
			always(`, 0 AS "descending"`),
			always(`FROM system.data_skipping_indices i`),
			always(`WHERE ` + notSystem("i.database")),
			always(`AND (@schema = '' OR i.database LIKE @schema)`),
			always(`AND (@name = '' OR i.table LIKE @name)`),
			always(`ORDER BY i.database, i.table, i.name`),
		},
		Fields: []dbmeta.Field{
			{Name: "schema"}, {Name: "table"}, {Name: "index"},
			{
				Name: "name",
				Desc: "the expression the index is on, which is the only form" +
					" ClickHouse records it in",
			},
			{
				Name: "ordinal",
				Desc: "always 1: the expression is stored whole rather than as a" +
					" list of columns to number",
			},
			{Name: "expression", Desc: "the same text, so a caller reading either finds it"},
			{Name: "descending", Desc: "always false: a skipping index has no direction"},
		},
		Params: schemaNameSystem("table"),
		Scan: func(rows *sql.Rows) (dbmeta.IndexColumn, error) {
			var v dbmeta.IndexColumn
			err := rows.Scan(&v.Schema, &v.Table, &v.Index, &v.Name, &v.Ordinal,
				&v.Expression, &v.Descending)
			return v, err
		},
	})

	// The CHECK constraints on a table. Every fragment gates on 26.8, where
	// system.constraints was first seen, so an older server reports that it
	// is too old rather than failing on a missing table.
	dbmeta.Constraints.Register(dbmeta.ClickHouse, &dbmeta.Binding[dbmeta.Constraint]{
		Stmt: dbmeta.Stmt{
			{{Min: v268, Query: `SELECT c.database AS "schema"`}},
			{{Min: v268, Query: `, c.table AS "table"`}},
			{{Min: v268, Query: `, c.name AS "name"`}},
			{{Min: v268, Query: `, lower(toString(c.type)) AS "type"`}},
			{{Min: v268, Query: `, c.expression AS "definition"`}},
			{{Min: v268, Query: `, 0 AS "deferrable"`}},
			{{Min: v268, Query: `, 0 AS "deferred"`}},
			{{Min: v268, Query: `, NULL AS "comment"`}},
			{{Min: v268, Query: `FROM system.constraints c`}},
			{{Min: v268, Query: `WHERE ` + notSystem("c.database")}},
			{{Min: v268, Query: `AND (@schema = '' OR c.database LIKE @schema)`}},
			{{Min: v268, Query: `AND (@name = '' OR c.table LIKE @name)`}},
			{{Min: v268, Query: `ORDER BY c.database, c.table, c.name`}},
		},
		Fields: []dbmeta.Field{
			{Name: "schema"}, {Name: "table"}, {Name: "name"},
			{
				Name: "type",
				Desc: "check or assume: ClickHouse has no foreign key, no unique" +
					" constraint and no primary key constraint",
			},
			{Name: "definition", Desc: "the expression the constraint asserts"},
			{Name: "deferrable", Desc: "always false: ClickHouse has no deferrable constraint"},
			{Name: "deferred", Desc: "always false, for the same reason"},
			{Name: "comment", Desc: "always absent: a constraint carries no comment"},
		},
		Params: schemaNameSystem("table"),
		Scan: func(rows *sql.Rows) (dbmeta.Constraint, error) {
			var v dbmeta.Constraint
			err := rows.Scan(&v.Schema, &v.Table, &v.Name, &v.Type, &v.Definition,
				&v.Deferrable, &v.Deferred, &v.Comment)
			return v, err
		},
	})

	// The comments on a table and on a column, in one list. ClickHouse keeps
	// each beside the object, and UNION ALL puts them together.
	dbmeta.Comments.Register(dbmeta.ClickHouse, &dbmeta.Binding[dbmeta.Comment]{
		Stmt: dbmeta.Stmt{
			always(`SELECT t.database AS "schema"`),
			always(`, t.name AS "name"`),
			always(`, 'table' AS "type"`),
			always(`, t.comment AS "comment"`),
			always(`FROM system.tables t`),
			always(`WHERE t.comment != ''`),
			always(`AND ` + notSystem("t.database")),
			always(`AND (@schema = '' OR t.database LIKE @schema)`),
			always(`AND (@name = '' OR t.name LIKE @name)`),
			always(`UNION ALL`),
			always(`SELECT c.database`),
			always(`, concat(c.table, '.', c.name)`),
			always(`, 'column'`),
			always(`, c.comment`),
			always(`FROM system.columns c`),
			always(`WHERE c.comment != ''`),
			always(`AND ` + notSystem("c.database")),
			always(`AND (@schema = '' OR c.database LIKE @schema)`),
			always(`AND (@name = '' OR c.table LIKE @name)`),
			always(`ORDER BY 1, 3, 2`),
		},
		Fields: []dbmeta.Field{
			{Name: "schema"},
			{Name: "name", Desc: "the table name, or table.column for a column comment"},
			{Name: "type"},
			{Name: "comment"},
		},
		Params: schemaNameSystem("table"),
		Scan: func(rows *sql.Rows) (dbmeta.Comment, error) {
			var v dbmeta.Comment
			err := rows.Scan(&v.Schema, &v.Name, &v.Type, &v.Comment)
			return v, err
		},
	})

	// \dP. A table with a partition key, and the key it is by.
	dbmeta.PartitionedTables.Register(dbmeta.ClickHouse,
		&dbmeta.Binding[dbmeta.PartitionedTable]{
			Stmt: dbmeta.Stmt{
				always(`SELECT t.database AS "schema"`),
				always(`, t.name AS "name"`),
				always(`, '' AS "owner"`),
				always(`, 'table' AS "type"`),
				always(`, '' AS "parent"`),
				always(`, 'key' AS "strategy"`),
				always(`, t.partition_key AS "expression"`),
				always(`, nullIf(t.comment, '') AS "comment"`),
				always(`FROM system.tables t`),
				always(`WHERE t.partition_key != ''`),
				always(`AND ` + notSystem("t.database")),
				always(`AND (@schema = '' OR t.database LIKE @schema)`),
				always(`AND (@name = '' OR t.name LIKE @name)`),
				always(`ORDER BY t.database, t.name`),
			},
			Fields: []dbmeta.Field{
				{Name: "schema"}, {Name: "name"},
				{Name: "owner", Desc: "always empty: a table records no owner"},
				{Name: "type", Desc: "always table: ClickHouse partitions no other kind"},
				{
					Name: "parent",
					Desc: "always empty: a ClickHouse partition is not a table, so a" +
						" partitioned table has no parent to name",
				},
				{
					Name: "strategy",
					Desc: "always key: PARTITION BY takes an expression and there is" +
						" no range, list or hash to choose between",
				},
				{Name: "expression", Desc: "the PARTITION BY expression"},
				{Name: "comment"},
			},
			Params: schemaNameSystem("table"),
			Scan: func(rows *sql.Rows) (dbmeta.PartitionedTable, error) {
				var v dbmeta.PartitionedTable
				err := rows.Scan(&v.Schema, &v.Name, &v.Owner, &v.Type, &v.Parent,
					&v.Strategy, &v.Expression, &v.Comment)
				return v, err
			},
		})

	// \dT. The data types the server knows, which are built in: ClickHouse
	// has no CREATE TYPE.
	dbmeta.Types.Register(dbmeta.ClickHouse, &dbmeta.Binding[dbmeta.Type]{
		Stmt: dbmeta.Stmt{
			always(`SELECT '' AS "catalog"`),
			always(`, '' AS "schema"`),
			always(`, f.name AS "name"`),
			always(`, f.name AS "internal"`),
			always(`, 'base' AS "kind"`),
			always(`, '' AS "elements"`),
			always(`, '' AS "owner"`),
			always(`, NULL AS "access"`),
			always(`, nullIf(f.alias_to, '') AS "comment"`),
			always(`FROM system.data_type_families f`),
			always(`WHERE (@name = '' OR f.name LIKE @name)`),
			always(`ORDER BY f.name`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "always empty: a type belongs to the server rather than a database"},
			{Name: "schema", Desc: "always empty, for the same reason"},
			{Name: "name"},
			{Name: "internal", Desc: "the same as the name: a type has one name here"},
			{
				Name: "kind",
				Desc: "always base: every ClickHouse type is built in, and Enum," +
					" Array and Tuple are spelled inside a column's type rather" +
					" than declared",
			},
			{Name: "elements", Desc: "always empty: a parameterised type is written inline"},
			{Name: "owner", Desc: "always empty: nobody owns a built in type"},
			{Name: "access", Desc: "always absent: a type is not grantable"},
			{
				Name: "comment",
				Desc: "the type this one is an alias to, absent when it is not an" +
					" alias. ClickHouse records no comment on a type",
			},
		},
		Params: []dbmeta.Param{
			{Name: "name", Desc: "type name pattern, empty for every one", Default: ""},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Type, error) {
			var v dbmeta.Type
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Name, &v.Internal, &v.Kind,
				&v.Elements, &v.Owner, &v.Access, &v.Comment)
			return v, err
		},
	})

	// \dO. The collations the server was built with.
	dbmeta.Collations.Register(dbmeta.ClickHouse, &dbmeta.Binding[dbmeta.Collation]{
		Stmt: dbmeta.Stmt{
			always(`SELECT '' AS "schema"`),
			always(`, c.name AS "name"`),
			always(`, 'icu' AS "provider"`),
			always(`, c.name AS "collate"`),
			always(`, c.name AS "ctype"`),
			always(`, nullIf(c.language, '') AS "locale"`),
			always(`, 1 AS "deterministic"`),
			always(`, NULL AS "comment"`),
			always(`FROM system.collations c`),
			always(`WHERE (@name = '' OR c.name LIKE @name)`),
			always(`ORDER BY c.name`),
		},
		Fields: []dbmeta.Field{
			{Name: "schema", Desc: "always empty: a collation belongs to the server"},
			{Name: "name"},
			{Name: "provider", Desc: "always icu, which is the only one ClickHouse uses"},
			{Name: "collate", Desc: "the same as the name: ClickHouse keeps one name"},
			{Name: "ctype", Desc: "the same as the name, for the same reason"},
			{Name: "locale", Desc: "the language, where the collation names one"},
			{Name: "deterministic", Desc: "always true: ClickHouse has no non deterministic collation"},
			{Name: "comment", Desc: "always absent: a collation carries no comment"},
		},
		Params: []dbmeta.Param{
			{Name: "name", Desc: "collation name pattern, empty for every one", Default: ""},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Collation, error) {
			var v dbmeta.Collation
			err := rows.Scan(&v.Schema, &v.Name, &v.Provider, &v.Collate, &v.CType,
				&v.Locale, &v.Deterministic, &v.Comment)
			return v, err
		},
	})
}
