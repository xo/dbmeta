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
			always(`, toInt64(i.data_compressed_bytes + i.marks_bytes) AS "size"`),
			always(`, concat('granularity=', toString(i.granularity)) AS "options"`),
			always(`, i.type_full AS "using"`),
			always(`FROM system.data_skipping_indices i`),
			always(`WHERE ` + notSystem("i.database")),
			always(`AND (@schema = '' OR i.database LIKE @schema)`),
			always(`AND (@parent = '' OR i.table LIKE @parent)`),
			always(`AND (@name = '' OR i.name LIKE @name)`),
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
			{
				Name: "size",
				Desc: "the compressed data and the marks of the index, in bytes. It is 0 for a table with no rows",
			},
			{Name: "options", Desc: "the granularity, as granularity=N"},
			{
				Name: "using",
				Desc: "the index type with its arguments, such as bloom_filter(0.025), which is type_full",
			},
		},
		Params: childParams("index"),
		Scan: func(rows *sql.Rows) (dbmeta.Index, error) {
			var v dbmeta.Index
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Table, &v.Name, &v.Type,
				&v.Unique, &v.Primary, &v.Comment, &v.Size, &v.Options, &v.Using)
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
			always(`AND (@parent = '' OR i.table LIKE @parent)`),
			always(`AND (@name = '' OR i.name LIKE @name)`),
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
		Params: childParams("index"),
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
			// An ASSUME constraint is a promise to the optimizer and the
			// server checks nothing. A CHECK constraint is checked on insert.
			{{Min: v268, Query: `, toString(c.type) = 'CHECK' AS "enforced"`}},
			{{Min: v268, Query: `FROM system.constraints c`}},
			{{Min: v268, Query: `WHERE ` + notSystem("c.database")}},
			{{Min: v268, Query: `AND (@schema = '' OR c.database LIKE @schema)`}},
			{{Min: v268, Query: `AND (@parent = '' OR c.table LIKE @parent)`}},
			{{Min: v268, Query: `AND (@name = '' OR c.name LIKE @name)`}},
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
			{Name: "enforced", Desc: "true for a check constraint, which ClickHouse tests on insert, and false for an assume constraint, which it never tests"},
		},
		Params: childParams("constraint"),
		Scan: func(rows *sql.Rows) (dbmeta.Constraint, error) {
			var v dbmeta.Constraint
			err := rows.Scan(&v.Schema, &v.Table, &v.Name, &v.Type, &v.Definition,
				&v.Deferrable, &v.Deferred, &v.Comment, &v.Enforced)
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
				always(`, NULL AS "table"`),
				always(`, t.engine AS "access_method"`),
				// A ClickHouse partition is not a table and has no level
				// below it, so the two sizes are the same number.
				always(`, toInt64(t.total_bytes) AS "direct_size"`),
				always(`, toInt64(t.total_bytes) AS "total_size"`),
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
				{Name: "table", Desc: "always absent: ClickHouse has no partitioned index"},
				{Name: "access_method", Desc: "the table engine"},
				{
					Name: "direct_size",
					Desc: "total_bytes of the table, which is the compressed bytes of its active parts",
				},
				{Name: "total_size", Desc: "the same number: a partition is not a table, so there is no second level"},
			},
			Params: schemaNameSystem("table"),
			Scan: func(rows *sql.Rows) (dbmeta.PartitionedTable, error) {
				var v dbmeta.PartitionedTable
				err := rows.Scan(&v.Schema, &v.Name, &v.Owner, &v.Type, &v.Parent,
					&v.Strategy, &v.Expression, &v.Comment, &v.Table, &v.AccessMethod,
					&v.DirectSize, &v.TotalSize)
				return v, err
			},
		})

	// The partitions of a table. A ClickHouse partition is not a table, so
	// the type is partition, and the name is the partition id, which is the
	// name of the directory prefix of its parts. The bound is the value of
	// the partition expression, as system.parts prints it. A table with no
	// PARTITION BY has the one partition all, and it is left out. The rows
	// are in system.parts and the Partition kind has no field for them.
	// system.parts is closed to a user who has no grant on it, as
	// system.data_skipping_indices is.
	dbmeta.Partitions.Register(dbmeta.ClickHouse, &dbmeta.Binding[dbmeta.Partition]{
		Stmt: dbmeta.Stmt{
			always(`SELECT p.database AS "schema"`),
			always(`, p.table AS "table"`),
			always(`, p.database AS "partition_schema"`),
			always(`, p.partition_id AS "partition"`),
			always(`, 'partition' AS "type"`),
			always(`, any(p.partition) AS "bound"`),
			always(`, NULL AS "constraint"`),
			always(`, 0 AS "partitioned"`),
			always(`, 0 AS "detach_pending"`),
			always(`, NULL AS "table_visible"`),
			always(`, NULL AS "partition_visible"`),
			always(`FROM system.parts p`),
			always(`WHERE p.active AND p.partition_id != 'all'`),
			always(`AND ` + notSystem("p.database")),
			always(`AND (@schema = '' OR p.database LIKE @schema)`),
			always(`AND (@parent = '' OR p.table LIKE @parent)`),
			always(`AND (@partition_schema = '' OR p.database LIKE @partition_schema)`),
			always(`AND (@name = '' OR p.partition_id LIKE @name)`),
			always(`GROUP BY p.database, p.table, p.partition_id`),
			always(`ORDER BY p.database, p.table, p.partition_id`),
		},
		Fields: []dbmeta.Field{
			{Name: "schema", Desc: "database of the partitioned table"},
			{Name: "table", Desc: "the partitioned table"},
			{Name: "partition_schema", Desc: "the same database: a partition is in no database of its own"},
			{Name: "partition", Desc: "the partition id, such as 202501"},
			{Name: "type", Desc: "always partition: a ClickHouse partition is not a table"},
			{
				Name: "bound",
				Desc: "the value of the partition expression, as system.parts prints it, such as 202501 or ('a', 1)",
			},
			{Name: "constraint", Desc: "always absent: ClickHouse makes no constraint of a partition value"},
			{Name: "partitioned", Desc: "always false: a partition has no partitions"},
			{Name: "detach_pending", Desc: "always false: ClickHouse detaches at once, and a detached part is in system.detached_parts"},
			{Name: "table_visible", Desc: "always absent: ClickHouse reports no search path"},
			{Name: "partition_visible", Desc: "always absent, for the same reason"},
		},
		Params: []dbmeta.Param{
			{Name: "schema", Desc: "database name pattern of the table, empty for every database", Default: ""},
			{Name: "parent", Desc: "table name pattern, empty for every table", Default: ""},
			{Name: "partition_schema", Desc: "database name pattern of the partition, empty for every database", Default: ""},
			{Name: "name", Desc: "partition id pattern, empty for every partition", Default: ""},
			{Name: "with_system", Desc: "include the databases ClickHouse keeps for itself", Default: false},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Partition, error) {
			var v dbmeta.Partition
			err := rows.Scan(&v.Schema, &v.Table, &v.PartitionSchema, &v.Partition, &v.Type,
				&v.Bound, &v.Constraint, &v.Partitioned, &v.DetachPending,
				&v.TableVisible, &v.PartitionVisible)
			return v, err
		},
	})

	// \dp row policies. A ClickHouse row policy filters what a role reads
	// from a table, and it is the analogue of a PostgreSQL policy. It has a
	// filter for SELECT and none for a write, so the command is always
	// select and WITH CHECK is absent. system.row_policies needs a grant.
	dbmeta.Policies.Register(dbmeta.ClickHouse, &dbmeta.Binding[dbmeta.Policy]{
		Stmt: dbmeta.Stmt{
			always(`SELECT r.database AS "schema"`),
			always(`, r.table AS "table"`),
			always(`, r.short_name AS "name"`),
			always(`, 'select' AS "command"`),
			always(`, NOT r.is_restrictive AS "permissive"`),
			always(`, multiIf(r.apply_to_all AND empty(r.apply_to_except), NULL` +
				`, r.apply_to_all, concat('ALL EXCEPT ', arrayStringConcat(r.apply_to_except, ','))` +
				`, arrayStringConcat(r.apply_to_list, ',')) AS "roles"`),
			always(`, r.select_filter AS "using"`),
			always(`, NULL AS "with_check"`),
			always(`, NULL AS "comment"`),
			always(`, NULL AS "enabled"`),
			always(`FROM system.row_policies r`),
			always(`WHERE ` + notSystem("r.database")),
			always(`AND (@schema = '' OR r.database LIKE @schema)`),
			always(`AND (@parent = '' OR r.table LIKE @parent)`),
			always(`AND (@name = '' OR r.short_name LIKE @name)`),
			always(`ORDER BY r.database, r.table, r.short_name`),
		},
		Fields: []dbmeta.Field{
			{Name: "schema"}, {Name: "table"}, {Name: "name", Desc: "the short name, which is the name before ON"},
			{Name: "command", Desc: "always select: a ClickHouse row policy filters reads only"},
			{Name: "permissive", Desc: "false for AS RESTRICTIVE"},
			{
				Name: "roles",
				Desc: "the roles the policy applies to, joined by a comma, and ALL EXCEPT followed by the roles for TO ALL EXCEPT. Absent for TO ALL",
			},
			{Name: "using", Desc: "the filter expression, as ClickHouse prints it"},
			{Name: "with_check", Desc: "always absent: a ClickHouse policy has no check on a write"},
			{Name: "comment", Desc: "always absent"},
			{Name: "enabled", Desc: "always absent: a ClickHouse row policy has no switch"},
		},
		Params: childParams("policy"),
		Scan: func(rows *sql.Rows) (dbmeta.Policy, error) {
			var v dbmeta.Policy
			err := rows.Scan(&v.Schema, &v.Table, &v.Name, &v.Command, &v.Permissive,
				&v.Roles, &v.Using, &v.WithCheck, &v.Comment, &v.Enabled)
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
			always(`, NULL AS "owner"`),
			always(`, NULL AS "access"`),
			always(`, nullIf(f.alias_to, '') AS "comment"`),
			always(`, NULL AS "size"`),
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
			{Name: "elements", Desc: "always empty: a parameterized type is written inline"},
			{Name: "owner", Desc: "always absent: nobody owns a built in type"},
			{Name: "access", Desc: "always absent: a type is not grantable"},
			{
				Name: "comment",
				Desc: "the type this one is an alias to, absent when it is not an" +
					" alias. ClickHouse records no comment on a type",
			},
			{Name: "size", Desc: "always absent: system.data_type_families records no length"},
		},
		Params: []dbmeta.Param{
			{Name: "name", Desc: "type name pattern, empty for every one", Default: ""},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Type, error) {
			var v dbmeta.Type
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Name, &v.Internal, &v.Kind,
				&v.Elements, &v.Owner, &v.Access, &v.Comment, &v.Size)
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
			always(`, NULL AS "rules"`),
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
			{Name: "rules", Desc: "always absent: system.collations records no tailoring rules"},
		},
		Params: []dbmeta.Param{
			{Name: "name", Desc: "collation name pattern, empty for every one", Default: ""},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Collation, error) {
			var v dbmeta.Collation
			err := rows.Scan(&v.Schema, &v.Name, &v.Provider, &v.Collate, &v.CType,
				&v.Locale, &v.Deterministic, &v.Comment, &v.Rules)
			return v, err
		},
	})
}
