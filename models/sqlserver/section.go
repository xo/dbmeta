package sqlserver

import (
	"database/sql"

	"github.com/xo/dbmeta"
)

// Partitions and Policies, which D199 added for PostgreSQL and D206 answers
// here.

// boundary is the text of the value of a partition boundary, which SQL Server
// keeps as a sql_variant. A date or a time is written the ISO way, because the
// default conversion writes "Jan  1 2020". Every other type is written as it
// is.
func boundary(alias string) string {
	return `CONVERT(nvarchar(4000), ` + alias + `.value, CASE WHEN` +
		` SQL_VARIANT_PROPERTY(` + alias + `.value, 'BaseType') IN ('date', 'datetime',` +
		` 'datetime2', 'smalldatetime', 'datetimeoffset', 'time') THEN 126 ELSE 0 END)`
}

func registerSections() {
	// The partitions of a table. A partition has a number and no name, so the
	// number is the name. The boundaries are in the partition function, one
	// fewer than the partitions, and the first partition has no lower bound
	// and the last has no upper one.
	//
	// RANGE RIGHT puts the boundary value in the partition above it, and
	// RANGE LEFT in the partition below it, so the interval is half open on
	// the side that says so: [lower, upper) for right, and (lower, upper] for
	// left. An end that has no boundary is written MINVALUE or MAXVALUE.
	//
	// Only the heap or the clustered index is read, because an index that is
	// aligned with the table has the same partitions, and an index that is not
	// aligned is not a partition of the table. A partitioned table of SQL
	// Server has one level, so Partitioned is false.
	dbmeta.Partitions.Register(dbmeta.SQLServer, &dbmeta.Binding[dbmeta.Partition]{
		Stmt: dbmeta.Stmt{
			always(`SELECT s.name AS "schema"`),
			always(`, t.name AS "table"`),
			always(`, s.name AS "partition_schema"`),
			always(`, CAST(p.partition_number AS nvarchar(10)) AS "partition"`),
			always(`, 'partition' AS "type"`),
			always(`, CASE WHEN pf.boundary_value_on_right = 1 THEN '[' ELSE '(' END` +
				` + COALESCE(` + boundary("lo") + `, 'MINVALUE') + ', ' + COALESCE(` + boundary("hi") + `, 'MAXVALUE')` +
				` + CASE WHEN pf.boundary_value_on_right = 1 THEN ')' ELSE ']' END AS "bound"`),
			always(`, CAST(0 AS bit) AS "partitioned"`),
			always(`, CAST(0 AS bit) AS "detach_pending"`),
			always(`FROM sys.partitions p`),
			always(`JOIN sys.tables t ON t.object_id = p.object_id`),
			always(`JOIN sys.schemas s ON s.schema_id = t.schema_id`),
			always(`JOIN sys.indexes i ON i.object_id = p.object_id AND i.index_id = p.index_id`),
			always(`JOIN sys.partition_schemes ps ON ps.data_space_id = i.data_space_id`),
			always(`JOIN sys.partition_functions pf ON pf.function_id = ps.function_id`),
			always(`LEFT JOIN sys.partition_range_values lo`),
			always(`  ON lo.function_id = pf.function_id AND lo.boundary_id = p.partition_number - 1`),
			always(`LEFT JOIN sys.partition_range_values hi`),
			always(`  ON hi.function_id = pf.function_id AND hi.boundary_id = p.partition_number`),
			always(`WHERE p.index_id IN (0, 1)`),
			always(`AND ` + notSystem),
			always(`AND (@schema = '' OR s.name LIKE @schema)`),
			always(`AND (@parent = '' OR t.name LIKE @parent)`),
			always(`AND (@partition_schema = '' OR s.name LIKE @partition_schema)`),
			always(`AND (@name = '' OR CAST(p.partition_number AS nvarchar(10)) LIKE @name)`),
			always(`ORDER BY 1, 2, p.partition_number`),
		},
		Fields: []dbmeta.Field{
			{Name: "schema", Desc: "schema of the partitioned table"},
			{Name: "table", Desc: "the partitioned table"},
			{Name: "partition_schema", Desc: "the same schema: a partition is in the schema of its table"},
			{Name: "partition", Desc: "the number of the partition, from one, because a partition has no name"},
			{Name: "type", Desc: "always partition"},
			{
				Name: "bound",
				Desc: "the interval of the partition, such as [2020-01-01, 2021-01-01) for RANGE RIGHT and (10, 20] for RANGE LEFT, with MINVALUE and MAXVALUE for an end that has no boundary",
			},
			{Name: "partitioned", Desc: "always false: SQL Server has one level of partitions"},
			{Name: "detach_pending", Desc: "always false: SQL Server has no detach that can wait"},
		},
		Params: []dbmeta.Param{
			{Name: "schema", Desc: "schema name pattern of the partitioned table, empty for every schema", Default: ""},
			{Name: "parent", Desc: "partitioned table name pattern, empty for every one", Default: ""},
			{Name: "partition_schema", Desc: "schema name pattern of the partition, empty for every schema", Default: ""},
			{Name: "name", Desc: "partition number pattern, empty for every partition", Default: ""},
			{Name: "with_system", Desc: "include the objects SQL Server keeps for itself", Default: false},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Partition, error) {
			var v dbmeta.Partition
			err := rows.Scan(&v.Schema, &v.Table, &v.PartitionSchema, &v.Partition, &v.Type,
				&v.Bound, &v.Partitioned, &v.DetachPending)
			return v, err
		},
	})

	// The row level security policies. SQL Server calls the whole set of
	// predicates of one security policy by the policy's name, and each
	// predicate is on one table. So a row is one predicate and a policy that
	// has several is several rows of the same name.
	//
	// A filter predicate hides a row from every read, which includes the read
	// that an UPDATE or a DELETE makes first, and has no counterpart for
	// INSERT. It is reported as select, with the expression as Using, because
	// it is the expression of a row that can be seen. A block predicate is
	// named for the moment it is checked. AFTER INSERT and AFTER UPDATE check
	// the new row, which is WithCheck. BEFORE UPDATE and BEFORE DELETE check the
	// existing row, which is Using. The predicate is a call to an inline
	// function, as SQL Server stores it.
	//
	// Every predicate restricts, so Permissive is false. A policy has no role
	// list, because it applies to every user and to the owner too, so Roles is
	// absent as it is for public. A policy that is switched off with
	// STATE = OFF restricts nothing and is not a row, because Policy has no
	// field to say that it is off.
	dbmeta.Policies.Register(dbmeta.SQLServer, &dbmeta.Binding[dbmeta.Policy]{
		Stmt: dbmeta.Stmt{
			{{Min: v13, Query: `SELECT s.name AS "schema"`}},
			{{Min: v13, Query: `, o.name AS "table"`}},
			{{Min: v13, Query: `, pol.name AS "name"`}},
			{{Min: v13, Query: `, CASE WHEN sp.predicate_type_desc = 'FILTER' THEN 'select'` +
				` WHEN sp.operation = 1 THEN 'insert' WHEN sp.operation IN (2, 3) THEN 'update'` +
				` WHEN sp.operation = 4 THEN 'delete' END AS "command"`}},
			{{Min: v13, Query: `, CAST(0 AS bit) AS "permissive"`}},
			{{Min: v13, Query: `, CASE WHEN sp.predicate_type_desc = 'FILTER' OR sp.operation IN (3, 4)` +
				` THEN sp.predicate_definition END AS "using"`}},
			{{Min: v13, Query: `, CASE WHEN sp.predicate_type_desc = 'BLOCK' AND sp.operation IN (1, 2)` +
				` THEN sp.predicate_definition END AS "with_check"`}},
			{{Min: v13, Query: `, ` + commentOn("pol.object_id") + ` AS "comment"`}},
			{{Min: v13, Query: `FROM sys.security_predicates sp`}},
			{{Min: v13, Query: `JOIN sys.security_policies pol ON pol.object_id = sp.object_id`}},
			{{Min: v13, Query: `JOIN sys.objects o ON o.object_id = sp.target_object_id`}},
			{{Min: v13, Query: `JOIN sys.schemas s ON s.schema_id = o.schema_id`}},
			{{Min: v13, Query: `WHERE pol.is_enabled = 1`}},
			{{Min: v13, Query: `AND ` + notSystem}},
			{{Min: v13, Query: `AND (@schema = '' OR s.name LIKE @schema)`}},
			{{Min: v13, Query: `AND (@parent = '' OR o.name LIKE @parent)`}},
			{{Min: v13, Query: `AND (@name = '' OR pol.name LIKE @name)`}},
			{{Min: v13, Query: `ORDER BY 1, 2, 3, 4`}},
		},
		Fields: []dbmeta.Field{
			{Name: "schema", Min: v13}, {Name: "table", Min: v13}, {Name: "name", Min: v13},
			{Name: "command", Desc: "select for a filter predicate, and insert, update or delete for a block predicate", Min: v13},
			{Name: "permissive", Desc: "always false: every security predicate restricts", Min: v13},
			{Name: "using", Desc: "the predicate that checks an existing row, which is a filter, BEFORE UPDATE or BEFORE DELETE", Min: v13},
			{Name: "with_check", Desc: "the predicate that checks a new row, which is AFTER INSERT or AFTER UPDATE", Min: v13},
			{Name: "comment", Min: v13},
		},
		Params: schemaParentName("policy"),
		Scan: func(rows *sql.Rows) (dbmeta.Policy, error) {
			var v dbmeta.Policy
			err := rows.Scan(&v.Schema, &v.Table, &v.Name, &v.Command, &v.Permissive,
				&v.Using, &v.WithCheck, &v.Comment)
			return v, err
		},
	})
}
