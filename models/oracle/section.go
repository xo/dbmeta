package oracle

import (
	"database/sql"

	"github.com/xo/dbmeta"
)

// Partitions, Policies and NotNulls, which D199 added for PostgreSQL and D206
// answers here.

func registerSections() {
	// The partitions of a table, one row each. A table that is partitioned
	// again into subpartitions has a row for each partition, and Partitioned
	// says so. The subpartitions are not rows, because ALL_TAB_SUBPARTITIONS has
	// its own high value and a LONG cannot be read through a UNION.
	//
	// Bound is HIGH_VALUE as the dictionary keeps it, which is the exclusive
	// upper bound of a range partition, such as MAXVALUE or
	// TO_DATE(' 2020-01-01 00:00:00', ...), and the values of a list partition.
	// It is absent for a hash partition. HIGH_VALUE is a LONG, which cannot be
	// joined to a text or searched, so it is returned as it is.
	dbmeta.Partitions.Register(dbmeta.Oracle, &dbmeta.Binding[dbmeta.Partition]{
		Stmt: dbmeta.Stmt{
			always(`SELECT p.table_owner AS "schema"`),
			always(`, p.table_name AS "table"`),
			always(`, p.table_owner AS "partition_schema"`),
			always(`, p.partition_name AS "partition"`),
			always(`, 'partition' AS "type"`),
			always(`, p.high_value AS "bound"`),
			always(`, CASE WHEN p.subpartition_count > 0 THEN 1 ELSE 0 END AS "partitioned"`),
			always(`, 0 AS "detach_pending"`),
			always(`FROM all_tab_partitions p`),
			notSystem("WHERE", "p.table_owner"),
			always(`AND (@schema IS NULL OR p.table_owner LIKE @schema)`),
			always(`AND (@parent IS NULL OR p.table_name LIKE @parent)`),
			always(`AND (@partition_schema IS NULL OR p.table_owner LIKE @partition_schema)`),
			always(`AND (@name IS NULL OR p.partition_name LIKE @name)`),
			always(`ORDER BY p.table_owner, p.table_name, p.partition_position`),
		},
		Fields: []dbmeta.Field{
			{Name: "schema", Desc: "schema of the partitioned table"},
			{Name: "table", Desc: "the partitioned table"},
			{Name: "partition_schema", Desc: "the same schema: a partition is in the schema of its table"},
			{Name: "partition", Desc: "the partition"},
			{Name: "type", Desc: "always partition"},
			{
				Name: "bound",
				Desc: "HIGH_VALUE as the dictionary keeps it: the exclusive upper bound of a range partition, the values of a list partition, and absent for a hash partition",
			},
			{Name: "partitioned", Desc: "true when the partition has subpartitions"},
			{Name: "detach_pending", Desc: "always false: Oracle has no detach that can wait"},
		},
		Params: []dbmeta.Param{
			{Name: "schema", Desc: "schema name pattern of the partitioned table, empty for every schema", Default: ""},
			{Name: "parent", Desc: "partitioned table name pattern, empty for every one", Default: ""},
			{Name: "partition_schema", Desc: "schema name pattern of the partition, empty for every schema", Default: ""},
			{Name: "name", Desc: "partition name pattern, empty for every partition", Default: ""},
			{Name: "with_system", Desc: "include the schemas Oracle keeps for itself", Default: false},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Partition, error) {
			var v dbmeta.Partition
			err := rows.Scan(&v.Schema, &v.Table, &v.PartitionSchema, &v.Partition, &v.Type,
				&v.Bound, &v.Partitioned, &v.DetachPending)
			return v, err
		},
	})

	// The virtual private database policies, which ALL_POLICIES lists for the
	// objects the user can reach. DBA_POLICIES needs a role and is not read.
	//
	// A policy applies to the statements it names with SEL, INS, UPD and DEL.
	// One policy for all four is one row with the command all. Otherwise there
	// is one row for each statement it names, because a row has one command.
	//
	// Oracle keeps no predicate. The policy names a function, and the function
	// returns the predicate each time a statement runs. So Using and WithCheck
	// hold the name of that function, owner and package included, which is the
	// expression the policy evaluates. CHK_OPT says that the policy also checks
	// a row that INSERT or UPDATE writes. Every predicate restricts, because
	// the predicates of several policies are joined with AND, so Permissive is
	// false. A policy has no role list, and applies to every user except one
	// with EXEMPT ACCESS POLICY, so Roles is absent. A policy that is disabled
	// restricts nothing and is not a row, because Policy has no field to say
	// that it is off.
	dbmeta.Policies.Register(dbmeta.Oracle, &dbmeta.Binding[dbmeta.Policy]{
		Stmt: dbmeta.Stmt{
			always(`SELECT p.object_owner AS "schema"`),
			always(`, p.object_name AS "table"`),
			always(`, p.policy_name AS "name"`),
			always(`, k.command AS "command"`),
			always(`, 0 AS "permissive"`),
			always(`, CASE WHEN k.command <> 'insert' THEN p.pf_owner || '.' ||`),
			always(`    CASE WHEN p.package IS NOT NULL THEN p.package || '.' END || p.function END AS "using"`),
			always(`, CASE WHEN k.command = 'insert' OR (p.chk_option = 'YES' AND k.command IN ('all', 'update'))`),
			always(`    THEN p.pf_owner || '.' || CASE WHEN p.package IS NOT NULL THEN p.package || '.' END`),
			always(`    || p.function END AS "with_check"`),
			always(`, NULL AS "comment"`),
			always(`FROM all_policies p`),
			always(`JOIN (SELECT 'all' AS command FROM dual UNION ALL SELECT 'select' FROM dual`),
			always(`  UNION ALL SELECT 'insert' FROM dual UNION ALL SELECT 'update' FROM dual`),
			always(`  UNION ALL SELECT 'delete' FROM dual) k`),
			always(`  ON (k.command = 'all' AND p.sel = 'YES' AND p.ins = 'YES' AND p.upd = 'YES' AND p.del = 'YES')`),
			always(`  OR (NOT (p.sel = 'YES' AND p.ins = 'YES' AND p.upd = 'YES' AND p.del = 'YES')`),
			always(`    AND ((k.command = 'select' AND p.sel = 'YES') OR (k.command = 'insert' AND p.ins = 'YES')`),
			always(`    OR (k.command = 'update' AND p.upd = 'YES') OR (k.command = 'delete' AND p.del = 'YES')))`),
			always(`WHERE p.enable = 'YES'`),
			notSystem("AND", "p.object_owner"),
			always(`AND (@schema IS NULL OR p.object_owner LIKE @schema)`),
			always(`AND (@parent IS NULL OR p.object_name LIKE @parent)`),
			always(`AND (@name IS NULL OR p.policy_name LIKE @name)`),
			always(`ORDER BY p.object_owner, p.object_name, p.policy_name, k.command`),
		},
		Fields: []dbmeta.Field{
			{Name: "schema"}, {Name: "table"}, {Name: "name"},
			{Name: "command", Desc: "all when the policy names every statement, and otherwise select, insert, update or delete, one row for each"},
			{Name: "permissive", Desc: "always false: the predicates of several policies are joined with AND"},
			{Name: "using", Desc: "the policy function, as owner.package.function, because Oracle keeps no predicate text. Absent for insert"},
			{Name: "with_check", Desc: "the same function for insert, and for update and all when the policy has the check option. Absent otherwise"},
			{Name: "comment", Desc: "always absent: Oracle records no comment on a policy"},
		},
		Params: childParams("table", "policy"),
		Scan: func(rows *sql.Rows) (dbmeta.Policy, error) {
			var v dbmeta.Policy
			err := rows.Scan(&v.Schema, &v.Table, &v.Name, &v.Command, &v.Permissive,
				&v.Using, &v.WithCheck, &v.Comment)
			return v, err
		},
	})

	// A NOT NULL is a check constraint in the dictionary, with the condition
	// "COLUMN" IS NOT NULL. Whether the name was generated or written does not
	// matter here, so both are rows. The condition is compared whole, because
	// a pattern also takes a check that ends in the same words.
	//
	// search_condition_vc arrived in 12c. Before it the condition is a LONG,
	// which no WHERE can read, so 11g is refused.
	dbmeta.NotNulls.Register(dbmeta.Oracle, &dbmeta.Binding[dbmeta.NotNull]{
		Stmt: dbmeta.Stmt{
			{{Min: v12, Query: `SELECT c.owner AS "schema"`}},
			{{Min: v12, Query: `, c.table_name AS "table"`}},
			{{Min: v12, Query: `, c.constraint_name AS "name"`}},
			{{Min: v12, Query: `, cc.column_name AS "column"`}},
			{{Min: v12, Query: `, 0 AS "no_inherit"`}},
			{{Min: v12, Query: `, 1 AS "local"`}},
			{{Min: v12, Query: `, 0 AS "inherited"`}},
			{{Min: v12, Query: `, CASE c.validated WHEN 'VALIDATED' THEN 1 ELSE 0 END AS "validated"`}},
			{{Min: v12, Query: `FROM all_constraints c`}},
			{{Min: v12, Query: `JOIN all_cons_columns cc ON cc.owner = c.owner AND cc.constraint_name = c.constraint_name`}},
			{{Min: v12, Query: `WHERE c.constraint_type = 'C'`}},
			{{Min: v12, Query: `AND c.search_condition_vc = '"' || cc.column_name || '" IS NOT NULL'`}},
			notSystem("AND", "c.owner"),
			{{Min: v12, Query: `AND (@schema IS NULL OR c.owner LIKE @schema)`}},
			{{Min: v12, Query: `AND (@parent IS NULL OR c.table_name LIKE @parent)`}},
			{{Min: v12, Query: `AND (@name IS NULL OR c.constraint_name LIKE @name)`}},
			{{Min: v12, Query: `ORDER BY c.owner, c.table_name, cc.column_name`}},
		},
		Fields: []dbmeta.Field{
			{Name: "schema", Min: v12}, {Name: "table", Min: v12}, {Name: "name", Min: v12},
			{Name: "column", Min: v12},
			{Name: "no_inherit", Desc: "always false: Oracle has no table inheritance", Min: v12},
			{Name: "local", Desc: "always true, for the same reason", Min: v12},
			{Name: "inherited", Desc: "always false, for the same reason", Min: v12},
			{Name: "validated", Desc: "false for a constraint that is NOT VALIDATED", Min: v12},
		},
		Params: childParams("table", "constraint"),
		Scan: func(rows *sql.Rows) (dbmeta.NotNull, error) {
			var v dbmeta.NotNull
			err := rows.Scan(&v.Schema, &v.Table, &v.Name, &v.Column, &v.NoInherit,
				&v.Local, &v.Inherited, &v.Validated)
			return v, err
		},
	})
}
