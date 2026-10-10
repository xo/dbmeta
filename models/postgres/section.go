package postgres

import (
	"database/sql"

	"github.com/xo/dbmeta"
)

// The kinds that the sections of \d+ name read and that no earlier kind
// carried. Each one is a child of a table with flat rows, which D47 requires,
// and each takes the parent filter, so that a caller describing one table
// reads only that table. See D199.

func init() {
	registerPartitions()
	registerInherits()
	registerPolicies()
	registerRules()
	registerNotNulls()
}

// Releases that a fragment gates on and that no other file needs.
var (
	// v95 is the release that brought row level security.
	v95 = dbmeta.V(9, 5)
	v18 = dbmeta.V(18)
)

// registerPartitions backs the Partitions list of \d+ name and the Partition
// of, Partition constraint and Partition key lines.
//
// pg_inherits holds one row for each parent and child, and relispartition says
// that the child is a partition. Declarative partitioning arrived in release
// 10, so this is refused below it, as PartitionedTables is.
func registerPartitions() {
	dbmeta.Partitions.Register(dbmeta.PostgreSQL, &dbmeta.Binding[dbmeta.Partition]{
		Stmt: dbmeta.Stmt{
			{{Min: v10, Query: `SELECT pn.nspname AS "schema"`}},
			{{Min: v10, Query: `, p.relname AS "table"`}},
			{{Min: v10, Query: `, cn.nspname AS "partition_schema"`}},
			{{Min: v10, Query: `, c.relname AS "partition"`}},
			{{Min: v10, Query: `, ` + relationType + ` AS "type"`}},
			{{Min: v10, Query: `, pg_catalog.pg_get_expr(c.relpartbound, c.oid) AS "bound"`}},
			{{Min: v10, Query: `, pg_catalog.pg_get_partition_constraintdef(c.oid) AS "constraint"`}},
			{{Min: v10, Query: `, c.relkind = 'p' AS "partitioned"`}},
			// inhdetachpending arrived in release 14
			{
				{Min: v10, Query: `, false AS "detach_pending"`},
				{Min: v14, Query: `, h.inhdetachpending AS "detach_pending"`},
			},
			// whether psql prints the names with no schema. See D201.
			{{Min: v10, Query: `, pg_catalog.pg_table_is_visible(p.oid) AS "table_visible"`}},
			{{Min: v10, Query: `, pg_catalog.pg_table_is_visible(c.oid) AS "partition_visible"`}},
			{{Min: v10, Query: `FROM pg_catalog.pg_inherits h`}},
			{{Min: v10, Query: `JOIN pg_catalog.pg_class c ON c.oid = h.inhrelid`}},
			{{Min: v10, Query: `JOIN pg_catalog.pg_namespace cn ON cn.oid = c.relnamespace`}},
			{{Min: v10, Query: `JOIN pg_catalog.pg_class p ON p.oid = h.inhparent`}},
			{{Min: v10, Query: `JOIN pg_catalog.pg_namespace pn ON pn.oid = p.relnamespace`}},
			{{Min: v10, Query: `WHERE c.relispartition`}},
			{{Min: v10, Query: `AND c.relkind NOT IN ('i', 'I')`}},
			{{Min: v10, Query: `AND (@with_system OR (pn.nspname !~ '^pg_' AND pn.nspname <> 'information_schema'))`}},
			{{Min: v10, Query: `AND (@schema = '' OR pn.nspname LIKE @schema)`}},
			{{Min: v10, Query: `AND (@parent = '' OR p.relname LIKE @parent)`}},
			{{Min: v10, Query: `AND (@partition_schema = '' OR cn.nspname LIKE @partition_schema)`}},
			{{Min: v10, Query: `AND (@name = '' OR c.relname LIKE @name)`}},
			{{Min: v10, Query: `ORDER BY 1, 2, 3, 4`}},
		},
		Fields: []dbmeta.Field{
			{Name: "schema", Desc: "schema of the partitioned table", Min: v10},
			{Name: "table", Desc: "the partitioned table", Min: v10},
			{Name: "partition_schema", Desc: "schema of the partition", Min: v10},
			{Name: "partition", Desc: "the partition", Min: v10},
			{Name: "type", Desc: "relation type of the partition", Min: v10},
			{Name: "bound", Desc: "what follows FOR VALUES, or DEFAULT", Min: v10},
			{Name: "constraint", Desc: "the implicit constraint the bound makes", Min: v10},
			{Name: "partitioned", Desc: "whether the partition has partitions of its own", Min: v10},
			{Name: "detach_pending", Desc: "whether a DETACH CONCURRENTLY did not finish. Always false below release 14", Min: v10},
			{Name: "table_visible", Desc: "the parent is on the search path of the session", Min: v10},
			{Name: "partition_visible", Desc: "the partition is on the search path of the session", Min: v10},
		},
		Params: []dbmeta.Param{
			{Name: "schema", Desc: "schema name pattern of the partitioned table, empty for every schema", Default: ""},
			{Name: "parent", Desc: "partitioned table name pattern, empty for every one", Default: ""},
			{Name: "partition_schema", Desc: "schema name pattern of the partition, empty for every schema", Default: ""},
			{Name: "name", Desc: "partition name pattern, empty for every partition", Default: ""},
			{Name: "with_system", Desc: "include the objects PostgreSQL keeps for itself", Default: false},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Partition, error) {
			var v dbmeta.Partition
			err := rows.Scan(&v.Schema, &v.Table, &v.PartitionSchema, &v.Partition, &v.Type,
				&v.Bound, &v.Constraint, &v.Partitioned, &v.DetachPending,
				&v.TableVisible, &v.PartitionVisible)
			return v, err
		},
	})
}

// registerInherits backs Inherits and Child tables of \d+ name.
//
// A row is one parent of one child. It reads every child of the parent when the
// caller names the parent, and every parent of the child when the caller names
// the child. An index is left out, because a partitioned index has children
// that psql never calls inheritance.
func registerInherits() {
	dbmeta.Inherits.Register(dbmeta.PostgreSQL, &dbmeta.Binding[dbmeta.Inherit]{
		Stmt: dbmeta.Stmt{
			{{Query: `SELECT cn.nspname AS "schema"`}},
			{{Query: `, c.relname AS "name"`}},
			{{Query: `, ` + relationType + ` AS "type"`}},
			{{Query: `, pn.nspname AS "parent_schema"`}},
			{{Query: `, p.relname AS "parent"`}},
			{{Query: `, h.inhseqno AS "ordinal"`}},
			// relispartition arrived in release 10. Before it no table is a
			// partition, so false is the answer and not a stand in.
			{
				{Query: `, false AS "partition"`},
				{Min: v10, Query: `, c.relispartition AS "partition"`},
			},
			// whether psql prints the names with no schema. See D201.
			{{Query: `, pg_catalog.pg_table_is_visible(c.oid) AS "visible"`}},
			{{Query: `, pg_catalog.pg_table_is_visible(p.oid) AS "parent_visible"`}},
			{{Query: `FROM pg_catalog.pg_inherits h`}},
			{{Query: `JOIN pg_catalog.pg_class c ON c.oid = h.inhrelid`}},
			{{Query: `JOIN pg_catalog.pg_namespace cn ON cn.oid = c.relnamespace`}},
			{{Query: `JOIN pg_catalog.pg_class p ON p.oid = h.inhparent`}},
			{{Query: `JOIN pg_catalog.pg_namespace pn ON pn.oid = p.relnamespace`}},
			{{Query: `WHERE c.relkind NOT IN ('i', 'I')`}},
			{{Query: `AND (@with_system OR (cn.nspname !~ '^pg_' AND cn.nspname <> 'information_schema'))`}},
			{{Query: `AND (@schema = '' OR cn.nspname LIKE @schema)`}},
			{{Query: `AND (@name = '' OR c.relname LIKE @name)`}},
			{{Query: `AND (@parent_schema = '' OR pn.nspname LIKE @parent_schema)`}},
			{{Query: `AND (@parent = '' OR p.relname LIKE @parent)`}},
			{{Query: `ORDER BY 1, 2, 6`}},
		},
		Fields: []dbmeta.Field{
			{Name: "schema", Desc: "schema of the child"},
			{Name: "name", Desc: "the child table"},
			{Name: "type", Desc: "relation type of the child"},
			{Name: "parent_schema", Desc: "schema of the parent"},
			{Name: "parent", Desc: "the table the child inherits from"},
			{Name: "ordinal", Desc: "position of the parent among the parents of the child"},
			{Name: "partition", Desc: "whether the child is a partition of the parent. Always false below release 10"},
			{Name: "visible", Desc: "the child is on the search path of the session"},
			{Name: "parent_visible", Desc: "the parent is on the search path of the session"},
		},
		Params: []dbmeta.Param{
			{Name: "schema", Desc: "schema name pattern of the child, empty for every schema", Default: ""},
			{Name: "name", Desc: "child table name pattern, empty for every child", Default: ""},
			{Name: "parent_schema", Desc: "schema name pattern of the parent, empty for every schema", Default: ""},
			{Name: "parent", Desc: "parent table name pattern, empty for every parent", Default: ""},
			{Name: "with_system", Desc: "include the objects PostgreSQL keeps for itself", Default: false},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Inherit, error) {
			var v dbmeta.Inherit
			err := rows.Scan(&v.Schema, &v.Name, &v.Type, &v.ParentSchema, &v.Parent,
				&v.Ordinal, &v.Partition, &v.Visible, &v.ParentVisible)
			return v, err
		},
	})
}

// registerPolicies backs Policies of \d+ name.
//
// Row level security arrived in release 9.5, and a restrictive policy in
// release 10. A policy of an earlier release is permissive, which is the only
// kind it had.
func registerPolicies() {
	dbmeta.Policies.Register(dbmeta.PostgreSQL, &dbmeta.Binding[dbmeta.Policy]{
		Stmt: dbmeta.Stmt{
			{{Min: v95, Query: `SELECT n.nspname AS "schema"`}},
			{{Min: v95, Query: `, c.relname AS "table"`}},
			{{Min: v95, Query: `, pol.polname AS "name"`}},
			{{Min: v95, Query: `, CASE pol.polcmd WHEN '*' THEN 'all' WHEN 'r' THEN 'select'` +
				` WHEN 'a' THEN 'insert' WHEN 'w' THEN 'update' WHEN 'd' THEN 'delete'` +
				` ELSE pol.polcmd::text END AS "command"`}},
			{
				{Min: v95, Query: `, true AS "permissive"`},
				{Min: v10, Query: `, pol.polpermissive AS "permissive"`},
			},
			// the role list holds the oid 0 for public
			{{Min: v95, Query: `, CASE WHEN pol.polroles = '{0}' THEN NULL ELSE pg_catalog.array_to_string(ARRAY(` +
				`SELECT r.rolname FROM pg_catalog.pg_roles r WHERE r.oid = ANY (pol.polroles) ORDER BY 1), ',') END AS "roles"`}},
			{{Min: v95, Query: `, pg_catalog.pg_get_expr(pol.polqual, pol.polrelid) AS "using"`}},
			{{Min: v95, Query: `, pg_catalog.pg_get_expr(pol.polwithcheck, pol.polrelid) AS "with_check"`}},
			{{Min: v95, Query: `, pg_catalog.obj_description(pol.oid, 'pg_policy') AS "comment"`}},
			// PostgreSQL has no switch for one policy. Row security is on or
			// off for the whole table, which is Table.RowSecurity.
			{{Min: v95, Query: `, NULL::boolean AS "enabled"`}},
			{{Min: v95, Query: `FROM pg_catalog.pg_policy pol`}},
			{{Min: v95, Query: `JOIN pg_catalog.pg_class c ON c.oid = pol.polrelid`}},
			{{Min: v95, Query: `JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace`}},
			{{Min: v95, Query: `WHERE ` + notSystemSchema}},
			{{Min: v95, Query: `AND (@schema = '' OR n.nspname LIKE @schema)`}},
			{{Min: v95, Query: `AND (@parent = '' OR c.relname LIKE @parent)`}},
			{{Min: v95, Query: `AND (@name = '' OR pol.polname LIKE @name)`}},
			{{Min: v95, Query: `ORDER BY 1, 2, 3`}},
		},
		Fields: []dbmeta.Field{
			{Name: "schema", Min: v95}, {Name: "table", Min: v95}, {Name: "name", Min: v95},
			{Name: "command", Desc: "all, select, insert, update or delete", Min: v95},
			{Name: "permissive", Desc: "false for a restrictive policy. Always true below release 10", Min: v95},
			{Name: "roles", Desc: "the roles the policy applies to, joined by a comma, absent for public", Min: v95},
			{Name: "using", Desc: "the USING expression, absent when there is none", Min: v95},
			{Name: "with_check", Desc: "the WITH CHECK expression, absent when there is none", Min: v95},
			{Name: "comment", Min: v95},
			{Name: "enabled", Desc: "always absent: PostgreSQL switches row security for a table, never one policy. See Table.RowSecurity", Min: v95},
		},
		Params: schemaParentName("policy"),
		Scan: func(rows *sql.Rows) (dbmeta.Policy, error) {
			var v dbmeta.Policy
			err := rows.Scan(&v.Schema, &v.Table, &v.Name, &v.Command, &v.Permissive,
				&v.Roles, &v.Using, &v.WithCheck, &v.Comment, &v.Enabled)
			return v, err
		},
	})
}

// registerRules backs Rules of \d+ name.
//
// psql leaves out the rule named _RETURN, which is the way a view holds its
// definition, and so does this. Views reads it.
func registerRules() {
	dbmeta.Rules.Register(dbmeta.PostgreSQL, &dbmeta.Binding[dbmeta.Rule]{
		Stmt: dbmeta.Stmt{
			{{Query: `SELECT n.nspname AS "schema"`}},
			{{Query: `, c.relname AS "table"`}},
			{{Query: `, r.rulename AS "name"`}},
			{{Query: `, CASE r.ev_type WHEN '1' THEN 'select' WHEN '2' THEN 'update'` +
				` WHEN '3' THEN 'insert' WHEN '4' THEN 'delete' ELSE r.ev_type::text END AS "event"`}},
			{{Query: `, CASE r.ev_enabled WHEN 'O' THEN 'enabled' WHEN 'D' THEN 'disabled'` +
				` WHEN 'R' THEN 'replica' WHEN 'A' THEN 'always' ELSE '' END AS "enabled"`}},
			{{Query: `, r.is_instead AS "instead"`}},
			{{Query: `, trim(trailing ';' FROM pg_catalog.pg_get_ruledef(r.oid, true)) AS "definition"`}},
			{{Query: `, pg_catalog.obj_description(r.oid, 'pg_rewrite') AS "comment"`}},
			{{Query: `FROM pg_catalog.pg_rewrite r`}},
			{{Query: `JOIN pg_catalog.pg_class c ON c.oid = r.ev_class`}},
			{{Query: `JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace`}},
			{{Query: `WHERE r.rulename <> '_RETURN'`}},
			{{Query: `AND ` + notSystemSchema}},
			{{Query: `AND (@schema = '' OR n.nspname LIKE @schema)`}},
			{{Query: `AND (@parent = '' OR c.relname LIKE @parent)`}},
			{{Query: `AND (@name = '' OR r.rulename LIKE @name)`}},
			{{Query: `ORDER BY 1, 2, 3`}},
		},
		Fields: fields("schema", "table", "name", "event", "enabled", "instead", "definition", "comment"),
		Params: schemaParentName("rule"),
		Scan: func(rows *sql.Rows) (dbmeta.Rule, error) {
			var v dbmeta.Rule
			err := rows.Scan(&v.Schema, &v.Table, &v.Name, &v.Event, &v.Enabled,
				&v.Instead, &v.Definition, &v.Comment)
			return v, err
		},
	})
}

// registerNotNulls backs Not-null constraints of \d+ name in psql 18.
//
// Release 18 records a NOT NULL in pg_constraint with the type n. Constraints
// leaves it out on every release, for the reason D49 gives, and this kind is
// where it is. A server below 18 refuses it as too old, because no release
// below 18 has a name for the constraint to read.
func registerNotNulls() {
	dbmeta.NotNulls.Register(dbmeta.PostgreSQL, &dbmeta.Binding[dbmeta.NotNull]{
		Stmt: dbmeta.Stmt{
			{{Min: v18, Query: `SELECT n.nspname AS "schema"`}},
			{{Min: v18, Query: `, t.relname AS "table"`}},
			{{Min: v18, Query: `, r.conname AS "name"`}},
			{{Min: v18, Query: `, a.attname AS "column"`}},
			{{Min: v18, Query: `, r.connoinherit AS "no_inherit"`}},
			{{Min: v18, Query: `, r.conislocal AS "local"`}},
			{{Min: v18, Query: `, r.coninhcount <> 0 AS "inherited"`}},
			{{Min: v18, Query: `, r.convalidated AS "validated"`}},
			{{Min: v18, Query: `FROM pg_catalog.pg_constraint r`}},
			{{Min: v18, Query: `JOIN pg_catalog.pg_class t ON t.oid = r.conrelid`}},
			{{Min: v18, Query: `JOIN pg_catalog.pg_namespace n ON n.oid = t.relnamespace`}},
			{{Min: v18, Query: `JOIN pg_catalog.pg_attribute a ON a.attrelid = r.conrelid AND a.attnum = r.conkey[1]`}},
			{{Min: v18, Query: `WHERE r.contype = 'n'`}},
			{{Min: v18, Query: `AND ` + notSystemSchema}},
			{{Min: v18, Query: `AND (@schema = '' OR n.nspname LIKE @schema)`}},
			{{Min: v18, Query: `AND (@parent = '' OR t.relname LIKE @parent)`}},
			{{Min: v18, Query: `AND (@name = '' OR r.conname LIKE @name)`}},
			{{Min: v18, Query: `ORDER BY 1, 2, a.attnum`}},
		},
		Fields: []dbmeta.Field{
			{Name: "schema", Min: v18}, {Name: "table", Min: v18}, {Name: "name", Min: v18},
			{Name: "column", Desc: "the column the constraint is on", Min: v18},
			{Name: "no_inherit", Desc: "whether the constraint is NO INHERIT", Min: v18},
			{Name: "local", Desc: "whether the constraint was written on this table", Min: v18},
			{Name: "inherited", Desc: "whether a parent table holds it too", Min: v18},
			{Name: "validated", Desc: "false for a constraint added NOT VALID", Min: v18},
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
