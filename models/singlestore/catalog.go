package singlestore

import (
	"database/sql"

	"github.com/xo/dbmeta"
)

// systemSchemas are the schemas SingleStore keeps for itself. The mysql
// model holds the same list for the statements it shares.
const systemSchemas = `'information_schema', 'memsql', 'cluster'`

func always(query string) dbmeta.Choice { return dbmeta.Choice{{Query: query}} }

// like is the filter that matches col to the pattern in param, and passes
// when the pattern is empty.
func like(col, param string) string {
	return `(` + param + ` = '' OR ` + col + ` LIKE ` + param + `)`
}

// registerOwn registers the statements that read SingleStore's own views.
func registerOwn() {
	// information_schema.GLOBAL_VARIABLES, because SingleStore has no
	// performance_schema. It says whether a variable can be set while the
	// server runs, which is the context.
	dbmeta.Settings.Register(dbmeta.MemSQL, &dbmeta.Binding[dbmeta.Setting]{
		Stmt: dbmeta.Stmt{
			always(`SELECT LOWER(v.variable_name) AS "name"`),
			always(`, v.variable_value AS "value"`),
			always(`, NULL AS "type"`),
			always(`, CASE WHEN v.is_settable_at_runtime THEN 'runtime'` +
				` WHEN v.is_settable_at_startup THEN 'startup' ELSE 'none' END AS "context"`),
			always(`, NULL AS "access"`),
			always(`, NULL AS "display"`),
			always(`FROM information_schema.GLOBAL_VARIABLES v`),
			always(`WHERE ` + like("LOWER(v.variable_name)", "@name")),
			always(`ORDER BY 1`),
		},
		Fields: []dbmeta.Field{
			{Name: "name", Desc: "in lower case, as the mysql model reports a variable"},
			{Name: "value"},
			{Name: "type", Desc: "always absent: GLOBAL_VARIABLES records no type"},
			{Name: "context", Desc: "runtime for a variable that can be set while the server runs, startup for one set only when it starts, and none otherwise"},
			{Name: "access", Desc: "always absent: a variable has no grant of its own"},
			{Name: "display", Desc: "always absent: SingleStore shows a value in one form, which is value"},
		},
		Params: []dbmeta.Param{
			{Name: "name", Desc: "variable name pattern, empty for every variable", Default: ""},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Setting, error) {
			var v dbmeta.Setting
			err := rows.Scan(&v.Name, &v.Value, &v.Type, &v.Context, &v.Access, &v.Display)
			return v, err
		},
	})

	// A user can log in. A role holds privileges and a group holds roles,
	// and neither can log in. SingleStore grants a role to a group and a
	// group to a user.
	dbmeta.Roles.Register(dbmeta.MemSQL, &dbmeta.Binding[dbmeta.Role]{
		Stmt: dbmeta.Stmt{
			always(`SELECT CONCAT(u.user, '@', u.host) AS "name"`),
			always(`, u.user = 'root' AS "superuser"`),
			always(`, FALSE AS "create_role"`),
			always(`, FALSE AS "create_db"`),
			always(`, u.account_status = 'OPEN' AS "can_login"`),
			always(`, FALSE AS "replication"`),
			always(`, FALSE AS "bypass_rls"`),
			always(`, TRUE AS "inherit"`),
			always(`, -1 AS "conn_limit"`),
			always(`, CAST(u.password_expiration AS CHAR) AS "valid_until"`),
			always(`, '' AS "member_of"`),
			always(`, NULLIF(u.comment, '') AS "comment"`),
			always(`FROM information_schema.USERS u`),
			always(`WHERE ` + like("CONCAT(u.user, '@', u.host)", "@name")),
			always(`UNION ALL`),
			always(`SELECT r.role, FALSE, FALSE, FALSE, FALSE, FALSE, FALSE, TRUE, -1, NULL, '', NULL`),
			always(`FROM information_schema.ROLES r`),
			always(`WHERE ` + like("r.role", "@name")),
			always(`UNION ALL`),
			always(`SELECT g.groupname, FALSE, FALSE, FALSE, FALSE, FALSE, FALSE, TRUE, -1, NULL,`),
			always(`  COALESCE(x.roles, ''), NULL`),
			always(`FROM information_schema.GROUPS g`),
			// SingleStore refuses an ordered aggregate in a correlated
			// subquery, so the roles of each group are gathered once and
			// joined.
			always("LEFT JOIN (SELECT gr.`group` AS g, GROUP_CONCAT(gr.role ORDER BY gr.role SEPARATOR ', ') AS roles" +
				" FROM information_schema.GROUPS_ROLES gr GROUP BY gr.`group`) x ON x.g = g.groupname"),
			always(`WHERE ` + like("g.groupname", "@name")),
			always(`ORDER BY 1`),
		},
		Fields: []dbmeta.Field{
			{Name: "name", Desc: "a user as user@host, a role, or a group"},
			{Name: "superuser", Desc: "true for root, the user SingleStore makes with every privilege"},
			{Name: "create_role", Desc: "always false: it is a grant rather than a flag"},
			{Name: "create_db", Desc: "always false: it is a grant rather than a flag"},
			{Name: "can_login", Desc: "true for a user whose account is open, and false for a role and a group"},
			{Name: "replication", Desc: "always false: it is a grant rather than a flag"},
			{Name: "bypass_rls", Desc: "always false: SingleStore has no row level security"},
			{Name: "inherit", Desc: "always true: a granted role or group is always inherited"},
			{Name: "conn_limit", Desc: "always -1: a limit is set by a resource pool rather than on the user"},
			{Name: "valid_until", Desc: "when a user's password expires, and absent for one that does not"},
			{Name: "member_of", Desc: "the roles a group holds, joined by commas. USERS does not say which groups a user is in"},
			{Name: "comment", Desc: "the COMMENT of a user"},
		},
		Params: []dbmeta.Param{
			{Name: "name", Desc: "user, role or group name pattern, empty for every one", Default: ""},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Role, error) {
			var v dbmeta.Role
			err := rows.Scan(&v.Name, &v.Superuser, &v.CreateRole, &v.CreateDB,
				&v.CanLogin, &v.Replication, &v.BypassRLS, &v.Inherit, &v.ConnLimit,
				&v.ValidUntil, &v.MemberOf, &v.Comment)
			return v, err
		},
	})

	// A role granted to a group. SingleStore also grants a group to a user,
	// and no view lists that, so it is not here.
	dbmeta.RoleGrants.Register(dbmeta.MemSQL, &dbmeta.Binding[dbmeta.RoleGrant]{
		Stmt: dbmeta.Stmt{
			always("SELECT gr.`group` AS \"role\""),
			always(`, gr.role AS "member_of"`),
			always(`, NULL AS "grantor"`),
			always(`, FALSE AS "admin"`),
			always(`, TRUE AS "inherit"`),
			always(`, TRUE AS "set"`),
			always(`FROM information_schema.GROUPS_ROLES gr`),
			always(`WHERE ` + like("gr.role", "@name")),
			always(`ORDER BY 1, 2`),
		},
		Fields: []dbmeta.Field{
			{Name: "role", Desc: "the group the role was granted to. A group granted to a user is in no view"},
			{Name: "member_of", Desc: "the role that was granted"},
			{Name: "grantor", Desc: "always absent: SingleStore records no grantor"},
			{Name: "admin", Desc: "always false: GROUPS_ROLES records no admin option"},
			{Name: "inherit", Desc: "always true: a granted role is always inherited"},
			{Name: "set", Desc: "always true: a member holds every role of its groups"},
		},
		Params: []dbmeta.Param{
			{Name: "name", Desc: "granted role name pattern, empty for every one", Default: ""},
		},
		Scan: func(rows *sql.Rows) (dbmeta.RoleGrant, error) {
			var v dbmeta.RoleGrant
			err := rows.Scan(&v.Role, &v.MemberOf, &v.Grantor, &v.Admin, &v.Inherit, &v.Set)
			return v, err
		},
	})

	// A privilege is granted to a role, and ROLE_PRIVILEGES lists each one
	// with its database and table, where * is every one. TABLE_PRIVILEGES
	// and SCHEMA_PRIVILEGES are empty although a role holds a grant,
	// measured on 9.1.1. USAGE is what every role holds, and is not a grant.
	dbmeta.Privileges.Register(dbmeta.MemSQL, &dbmeta.Binding[dbmeta.Privilege]{
		Stmt: dbmeta.Stmt{
			always("SELECT NULLIF(p.`database`, '*') AS \"schema\""),
			always(", p.`table` AS \"name\""),
			always(", CASE WHEN p.`database` = '*' THEN 'global' WHEN p.`table` = '*' THEN 'schema' ELSE 'table' END AS \"type\""),
			always(", CONCAT(p.role, '=', p.privileges) AS \"access\""),
			always(`, NULL AS "column_access"`),
			always(`, NULL AS "policies"`),
			always(`FROM information_schema.ROLE_PRIVILEGES p`),
			always("WHERE p.privileges <> 'USAGE'"),
			always("AND (@with_system OR p.`database` NOT IN (" + systemSchemas + "))"),
			always("AND " + like("p.`database`", "@schema")),
			always("AND " + like("p.`table`", "@name")),
			always(`ORDER BY 1, 2, 4`),
		},
		Fields: []dbmeta.Field{
			{Name: "schema", Desc: "the database, and absent for a grant on every database"},
			{Name: "name", Desc: "the table, and * for a grant on every table of the database"},
			{Name: "type", Desc: "table, schema for a grant on a whole database, or global"},
			{Name: "access", Desc: "the role and what it can do, one grant per row. A grant to a user is in no view"},
			{Name: "column_access", Desc: "always absent: SingleStore has no grant on a column"},
			{Name: "policies", Desc: "always absent: SingleStore has no row level security"},
		},
		Params: []dbmeta.Param{
			{Name: "schema", Desc: "database name pattern, empty for every database", Default: ""},
			{Name: "name", Desc: "table name pattern, empty for every table", Default: ""},
			{Name: "with_system", Desc: "include the schemas SingleStore keeps for itself", Default: false},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Privilege, error) {
			var v dbmeta.Privilege
			err := rows.Scan(&v.Schema, &v.Name, &v.Type, &v.Access, &v.ColumnAccess, &v.Policies)
			return v, err
		},
	})

	// A user defined aggregate, which is four functions and a state type.
	dbmeta.Aggregates.Register(dbmeta.MemSQL, &dbmeta.Binding[dbmeta.Function]{
		Stmt: dbmeta.Stmt{
			always(`SELECT 'def' AS "catalog"`),
			always(`, a.aggregate_schema AS "schema"`),
			always(`, a.aggregate_name AS "name"`),
			always(`, NULL AS "id"`),
			always(`, 'agg' AS "kind"`),
			always(`, a.return_type AS "result_type"`),
			always(`, NULL AS "arg_types"`),
			always(`, '' AS "volatility"`),
			always(`, '' AS "parallel"`),
			always(`, a.definer AS "owner"`),
			always(`, 'definer' AS "security"`),
			always(`, NULL AS "access"`),
			always(`, 'psql' AS "language"`),
			always(`, CONCAT('INITIALIZE WITH ', a.initialize_function, ' ITERATE WITH ', a.iterate_function,` +
				` ' MERGE WITH ', a.merge_function, ' TERMINATE WITH ', a.terminate_function) AS "source"`),
			always(`, NULL AS "comment"`),
			always(`, NULL AS "definition"`),
			always(`FROM information_schema.AGGREGATE_FUNCTIONS a`),
			always(`WHERE (@with_system OR a.aggregate_schema NOT IN (` + systemSchemas + `))`),
			always(`AND ` + like("a.aggregate_schema", "@schema")),
			always(`AND ` + like("a.aggregate_name", "@name")),
			always(`ORDER BY 2, 3`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog"}, {Name: "schema"}, {Name: "name"},
			{Name: "id", Desc: "always absent: SingleStore does not overload a name"},
			{Name: "kind", Desc: "always agg"},
			{Name: "result_type"},
			{Name: "arg_types", Desc: "always absent: AGGREGATE_FUNCTIONS records how many arguments and not their types"},
			{Name: "volatility", Desc: "always empty: SingleStore does not record it"},
			{Name: "parallel", Desc: "always empty: SingleStore does not record it"},
			{Name: "owner", Desc: "the definer"},
			{Name: "security", Desc: "always definer: an aggregate runs with its definer's rights"},
			{Name: "access", Desc: "always absent: a grant is on the database, not the aggregate"},
			{Name: "language", Desc: "always psql, SingleStore's procedural language, in which the four functions are written"},
			{Name: "source", Desc: "the four functions that make the aggregate"},
			{Name: "comment", Desc: "always absent: an aggregate takes no comment"},
			{Name: "definition", Desc: "always absent: AGGREGATE_FUNCTIONS keeps the four functions, which are source, and no CREATE statement"},
		},
		Params: []dbmeta.Param{
			{Name: "schema", Desc: "schema name pattern, empty for every schema", Default: ""},
			{Name: "name", Desc: "aggregate name pattern, empty for every aggregate", Default: ""},
			{Name: "with_system", Desc: "include the schemas SingleStore keeps for itself", Default: false},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Function, error) {
			var v dbmeta.Function
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Name, &v.ID, &v.Kind, &v.ResultType,
				&v.ArgTypes, &v.Volatility, &v.Parallel, &v.Owner, &v.Security, &v.Access,
				&v.Language, &v.Source, &v.Comment, &v.Definition)
			return v, err
		},
	})

	// ANALYZE TABLE fills OPTIMIZER_STATISTICS. cardinality is the distinct
	// count, and a table never analyzed has its values absent.
	dbmeta.ColumnStats.Register(dbmeta.MemSQL, &dbmeta.Binding[dbmeta.ColumnStat]{
		Stmt: dbmeta.Stmt{
			always(`SELECT 'def' AS "catalog"`),
			always(`, s.database_name AS "schema"`),
			always(`, s.table_name AS "table"`),
			always(`, s.column_name AS "name"`),
			always(`, s.average_byte_length AS "avg_width"`),
			always(`, CASE WHEN s.row_count > 0 THEN s.null_count / s.row_count END AS "null_frac"`),
			always(`, s.cardinality AS "distinct"`),
			always(`, NULL AS "min"`),
			always(`, NULL AS "max"`),
			always(`, NULL AS "mean"`),
			always(`, NULL AS "top_n"`),
			always(`, NULL AS "top_n_freqs"`),
			always(`FROM information_schema.OPTIMIZER_STATISTICS s`),
			always(`WHERE (@with_system OR s.database_name NOT IN (` + systemSchemas + `))`),
			always(`AND ` + like("s.database_name", "@schema")),
			always(`AND ` + like("s.table_name", "@parent")),
			always(`AND ` + like("s.column_name", "@name")),
			always(`ORDER BY 2, 3, 4`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog"}, {Name: "schema"}, {Name: "table"}, {Name: "name"},
			{Name: "avg_width", Desc: "the average length of a value in bytes"},
			{Name: "null_frac", Desc: "the NULL count over the row count"},
			{Name: "distinct", Desc: "the cardinality, which is the distinct count"},
			{Name: "min", Desc: "always absent: the bounds are inside the histogram"},
			{Name: "max", Desc: "always absent: the bounds are inside the histogram"},
			{Name: "mean", Desc: "always absent: SingleStore records no mean"},
			{Name: "top_n", Desc: "always absent: SingleStore records no most common values"},
			{Name: "top_n_freqs", Desc: "always absent, for the same reason as top_n"},
		},
		Params: []dbmeta.Param{
			{Name: "parent", Desc: "table name pattern, empty for every table", Default: ""},
			{Name: "schema", Desc: "schema name pattern, empty for every schema", Default: ""},
			{Name: "name", Desc: "column name pattern, empty for every column", Default: ""},
			{Name: "with_system", Desc: "include the schemas SingleStore keeps for itself", Default: false},
		},
		Scan: func(rows *sql.Rows) (dbmeta.ColumnStat, error) {
			var v dbmeta.ColumnStat
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Table, &v.Name, &v.AvgWidth,
				&v.NullFrac, &v.Distinct, &v.Min, &v.Max, &v.Mean, &v.TopN, &v.TopNFreqs)
			return v, err
		},
	})

	// \dX. ANALYZE TABLE ... CORRELATE COLUMN declares how strongly one
	// column follows another, so that the optimizer does not treat a filter
	// on both as independent. That is what PostgreSQL's functional
	// dependency statistic does, so the kind is f. A correlation has no name.
	// See D149.
	dbmeta.ExtendedStats.Register(dbmeta.MemSQL, &dbmeta.Binding[dbmeta.ExtendedStat]{
		Stmt: dbmeta.Stmt{
			always(`SELECT c.DATABASE_NAME AS "schema"`),
			always(`, NULL AS "name"`),
			always(`, NULL AS "owner"`),
			always(`, c.TABLE_NAME AS "table"`),
			always(`, 'f' AS "kinds"`),
			always(`, NULL AS "comment"`),
			always(`, CONCAT(c.CORRELATED_COLUMN_NAME, ', ', c.CORRELLEE_COLUMN_NAME,` +
				` ' FROM ', c.DATABASE_NAME, '.', c.TABLE_NAME) AS "definition"`),
			always(`, FALSE AS "ndistinct"`),
			always(`, TRUE AS "dependencies"`),
			always(`, FALSE AS "mcv"`),
			always(`FROM information_schema.CORRELATED_COLUMN_STATISTICS c`),
			always(`WHERE (@with_system OR c.DATABASE_NAME NOT IN (` + systemSchemas + `))`),
			always(`AND ` + like("c.DATABASE_NAME", "@schema")),
			// a correlation has no name, so only the empty pattern matches
			always(`AND @name = ''`),
			always(`ORDER BY 1, 4, c.CORRELATED_COLUMN_NAME, c.CORRELLEE_COLUMN_NAME`),
		},
		Fields: []dbmeta.Field{
			{Name: "schema"},
			{Name: "name", Desc: "always absent: a correlation has no name"},
			{Name: "owner", Desc: "always absent: a correlation has no owner"},
			{Name: "table"},
			{Name: "kinds", Desc: "always f, for a functional dependency"},
			{Name: "comment", Desc: "always absent: a correlation takes no comment"},
			{Name: "definition", Desc: "the correlated column, then the column it follows, and their table"},
			{Name: "ndistinct", Desc: "always false: a correlation counts no distinct values"},
			{Name: "dependencies", Desc: "always true: a correlation is how strongly one column follows another"},
			{Name: "mcv", Desc: "always false: a correlation keeps no common values"},
		},
		Params: []dbmeta.Param{
			{Name: "schema", Desc: "schema name pattern, empty for every schema", Default: ""},
			{Name: "name", Desc: "statistics object name pattern. A correlation has no name, so only the empty pattern matches", Default: ""},
			{Name: "with_system", Desc: "include the schemas SingleStore keeps for itself", Default: false},
		},
		Scan: func(rows *sql.Rows) (dbmeta.ExtendedStat, error) {
			var v dbmeta.ExtendedStat
			err := rows.Scan(&v.Schema, &v.Name, &v.Owner, &v.Table, &v.Kinds, &v.Comment,
				&v.Definition, &v.Ndistinct, &v.Dependencies, &v.MCV)
			return v, err
		},
	})
}
