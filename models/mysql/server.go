package mysql

import (
	"database/sql"

	"github.com/xo/dbmeta"
)

// Routines, users and the server itself.

func registerRoutines() {
	dbmeta.Functions.Register(dbmeta.MySQL, &dbmeta.Binding[dbmeta.Function]{
		Stmt: dbmeta.Stmt{
			{{Query: `SELECT r.routine_catalog AS "catalog"`}},
			{{Query: `, r.routine_schema AS "schema"`}},
			{{Query: `, r.routine_name AS "name"`}},
			// Neither product overloads a routine, so the specific name is
			// the name. RoutineParameters joins on it all the same, because a
			// caller writes one join for every database.
			{{Query: `, r.specific_name AS "id"`}},
			{{Query: `, CASE r.routine_type WHEN 'PROCEDURE' THEN 'proc' ELSE 'func' END AS "kind"`}},
			{{Query: `, r.dtd_identifier AS "result_type"`}},
			{{Query: `, NULL AS "arg_types"`}},
			{{Query: `, LOWER(r.is_deterministic) AS "volatility"`}},
			{{Query: `, '' AS "parallel"`}},
			{{Query: `, r.definer AS "owner"`}},
			{{Query: `, LOWER(r.security_type) AS "security"`}},
			{{Query: `, NULL AS "access"`}},
			// Never external_language. MariaDB leaves it NULL for a SQL
			// routine and MySQL writes SQL, and the field is not nullable, so
			// reading it fails to scan on MariaDB. routine_body says SQL or
			// EXTERNAL on both.
			{{Query: `, LOWER(r.routine_body) AS "language"`}},
			{{Query: `, r.routine_definition AS "source"`}},
			{{Query: `, NULLIF(r.routine_comment, '') AS "comment"`}},
			{{Query: `FROM information_schema.ROUTINES r`}},
			{{Query: `WHERE (@with_system OR r.routine_schema NOT IN (` + systemSchemas + `))`}},
			{{Query: `AND (@schema = '' OR r.routine_schema LIKE @schema)`}},
			{{Query: `AND (@name = '' OR r.routine_name LIKE @name)`}},
			{{Query: `ORDER BY 2, 3`}},
		},
		Fields: []dbmeta.Field{
			{Name: "catalog"}, {Name: "schema"}, {Name: "name"},
			{Name: "id", Desc: "the specific name, which is the name here because neither product overloads"},
			{Name: "kind"},
			{Name: "result_type"},
			{Name: "arg_types", Desc: "always absent: MariaDB keeps parameters in their own view"},
			{Name: "volatility", Desc: "yes or no, from is_deterministic"},
			{Name: "parallel", Desc: "always empty: MariaDB has no parallel safety marking"},
			{Name: "owner", Desc: "the definer"},
			{Name: "security"}, {Name: "access"}, {Name: "language"},
			{Name: "source"}, {Name: "comment"},
		},
		Params: schemaNameSystem("routine"),
		Scan: func(rows *sql.Rows) (dbmeta.Function, error) {
			var v dbmeta.Function
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Name, &v.ID, &v.Kind, &v.ResultType,
				&v.ArgTypes, &v.Volatility, &v.Parallel, &v.Owner, &v.Security,
				&v.Access, &v.Language, &v.Source, &v.Comment)
			return v, err
		},
	})
}

func registerServer() {
	// A storage engine is the closest thing MariaDB has to an access method.
	// Both answer "how is this stored and searched", and psql prints the
	// access method of a table in \d, which is where a caller would look.
	dbmeta.AccessMethods.Register(dbmeta.MySQL, &dbmeta.Binding[dbmeta.AccessMethod]{
		Stmt: dbmeta.Stmt{
			{{Query: `SELECT e.engine AS "name"`}},
			{{Query: `, 'table' AS "type"`}},
			{{Query: `, e.support AS "handler"`}},
			{{Query: `, e.comment AS "comment"`}},
			{{Query: `FROM information_schema.ENGINES e`}},
			{{Query: `WHERE (@name = '' OR e.engine LIKE @name)`}},
			{{Query: `ORDER BY 1`}},
		},
		Fields: []dbmeta.Field{
			{Name: "name"},
			{Name: "type", Desc: "always table: a storage engine has no index form"},
			{Name: "handler", Desc: "the support level, such as DEFAULT or YES"},
			{Name: "comment"},
		},
		Params: []dbmeta.Param{{Name: "name", Desc: "engine name pattern, empty for every engine", Default: ""}},
		Scan: func(rows *sql.Rows) (dbmeta.AccessMethod, error) {
			var v dbmeta.AccessMethod
			err := rows.Scan(&v.Name, &v.Type, &v.Handler, &v.Comment)
			return v, err
		},
	})

	// A plugin is the closest thing to an extension. Both add capability to a
	// running server and both are listed rather than created in SQL.
	dbmeta.Extensions.Register(dbmeta.MySQL, &dbmeta.Binding[dbmeta.Extension]{
		Stmt: dbmeta.Stmt{
			{{Query: `SELECT p.plugin_name AS "name"`}},
			{{Query: `, p.plugin_version AS "version"`}},
			{{Query: `, '' AS "schema"`}},
			{{Query: `, NULLIF(p.plugin_description, '') AS "comment"`}},
			// ALL_PLUGINS lists what is installed and what could be. MySQL has
			// only PLUGINS, which lists what is installed. The filter on an
			// active status makes the two agree on what they return.
			{
				frag(onMaria, `FROM information_schema.ALL_PLUGINS p`),
				frag(onMySQL, `FROM information_schema.PLUGINS p`),
			},
			{{Query: `WHERE p.plugin_status = 'ACTIVE'`}},
			{{Query: `AND (@name = '' OR p.plugin_name LIKE @name)`}},
			{{Query: `ORDER BY 1`}},
		},
		Fields: []dbmeta.Field{
			{Name: "name"}, {Name: "version"},
			{Name: "schema", Desc: "always empty: a plugin belongs to the server, not a schema"},
			{Name: "comment"},
		},
		Params: []dbmeta.Param{{Name: "name", Desc: "plugin name pattern, empty for every plugin", Default: ""}},
		Scan: func(rows *sql.Rows) (dbmeta.Extension, error) {
			var v dbmeta.Extension
			err := rows.Scan(&v.Name, &v.Version, &v.Schema, &v.Comment)
			return v, err
		},
	})

	dbmeta.Collations.Register(dbmeta.MySQL, &dbmeta.Binding[dbmeta.Collation]{
		Stmt: dbmeta.Stmt{
			{{Query: `SELECT '' AS "schema"`}},
			{{Query: `, c.collation_name AS "name"`}},
			{{Query: `, NULL AS "provider"`}},
			{{Query: `, c.character_set_name AS "collate"`}},
			{{Query: `, c.character_set_name AS "ctype"`}},
			{{Query: `, NULL AS "locale"`}},
			{{Query: `, TRUE AS "deterministic"`}},
			{{Query: `, NULL AS "comment"`}},
			{{Query: `FROM information_schema.COLLATIONS c`}},
			{{Query: `WHERE (@name = '' OR c.collation_name LIKE @name)`}},
			{{Query: `ORDER BY 2`}},
		},
		Fields: []dbmeta.Field{
			{Name: "schema", Desc: "always empty: a collation is not in a schema here"},
			{Name: "name"},
			{Name: "provider", Desc: "always absent: MariaDB has one collation provider"},
			{Name: "collate", Desc: "the character set"},
			{Name: "ctype", Desc: "the character set"},
			{Name: "locale", Desc: "always absent: MariaDB has no ICU locale"},
			{Name: "deterministic", Desc: "always true: MariaDB has no other behaviour"},
			{Name: "comment"},
		},
		Params: nameSystem("collation"),
		Scan: func(rows *sql.Rows) (dbmeta.Collation, error) {
			var v dbmeta.Collation
			err := rows.Scan(&v.Schema, &v.Name, &v.Provider, &v.Collate, &v.CType,
				&v.Locale, &v.Deterministic, &v.Comment)
			return v, err
		},
	})

	dbmeta.Settings.Register(dbmeta.MySQL, &dbmeta.Binding[dbmeta.Setting]{
		Stmt: dbmeta.Stmt{
			{{Query: `SELECT LOWER(v.variable_name) AS "name"`}},
			// MariaDB keeps the value beside the metadata. MySQL 8 dropped
			// information_schema.SYSTEM_VARIABLES and split the two across
			// performance_schema, so the value is joined back on. MySQL 8 has
			// the value and nothing else, and MySQL 9 added the type and the
			// scope in variables_metadata.
			{
				frag(onMaria, `, v.global_value AS "value"`),
				frag(onMySQL, `, v.variable_value AS "value"`),
				frag(mysqlVarMeta, `, g.variable_value AS "value"`),
			},
			{
				frag(onMaria, `, LOWER(v.variable_type) AS "type"`),
				frag(onMySQL, `, NULL AS "type"`),
				frag(mysqlVarMeta, `, LOWER(v.data_type) AS "type"`),
			},
			{
				frag(onMaria, `, LOWER(v.variable_scope) AS "context"`),
				frag(onMySQL, `, NULL AS "context"`),
				frag(mysqlVarMeta, `, LOWER(v.variable_scope) AS "context"`),
			},
			{{Query: `, NULL AS "access"`}},
			{
				frag(onMaria, `FROM information_schema.SYSTEM_VARIABLES v`),
				frag(onMySQL, `FROM performance_schema.global_variables v`),
				frag(mysqlVarMeta, `FROM performance_schema.variables_metadata v`+
					` JOIN performance_schema.global_variables g`+
					` ON g.variable_name = v.variable_name`),
			},
			{{Query: `WHERE (@name = '' OR LOWER(v.variable_name) LIKE @name)`}},
			{{Query: `ORDER BY 1`}},
		},
		Fields: []dbmeta.Field{
			{Name: "name"}, {Name: "value"},
			{Name: "type", Also: []dbmeta.Gate{onMaria, mysqlVarMeta}},
			{
				Name: "context",
				Desc: "the variable scope, such as global or session",
				Also: []dbmeta.Gate{onMaria, mysqlVarMeta},
			},
			{Name: "access", Desc: "always absent: neither product grants on a variable"},
		},
		Params: []dbmeta.Param{{Name: "name", Desc: "variable name pattern, empty for every one", Default: ""}},
		Scan: func(rows *sql.Rows) (dbmeta.Setting, error) {
			var v dbmeta.Setting
			err := rows.Scan(&v.Name, &v.Value, &v.Type, &v.Context, &v.Access)
			return v, err
		},
	})

	// A user and a role are both rows of mysql.user here, so one query answers
	// for both and psql's \du and \dg land on the same place. What separates
	// them differs: MariaDB marks a role with is_role, and MySQL locks the
	// account instead and has no such column.
	dbmeta.Roles.Register(dbmeta.MySQL, &dbmeta.Binding[dbmeta.Role]{
		Stmt: dbmeta.Stmt{
			{{Query: `SELECT CONCAT(u.user, IF(u.host = '', '', CONCAT('@', u.host))) AS "name"`}},
			{{Query: `, u.super_priv = 'Y' AS "superuser"`}},
			{{Query: `, u.create_user_priv = 'Y' AS "create_role"`}},
			{{Query: `, u.create_priv = 'Y' AS "create_db"`}},
			{
				frag(onMaria, `, u.is_role = 'N' AS "can_login"`),
				frag(onMySQL, `, u.account_locked = 'N' AS "can_login"`),
			},
			{{Query: `, u.repl_slave_priv = 'Y' AS "replication"`}},
			{{Query: `, FALSE AS "bypass_rls"`}},
			{{Query: `, TRUE AS "inherit"`}},
			{{Query: `, CAST(u.max_user_connections AS SIGNED) AS "conn_limit"`}},
			{{Query: `, NULL AS "valid_until"`}},
			{{Query: `, '' AS "member_of"`}},
			{{Query: `, NULL AS "comment"`}},
			{{Query: `FROM mysql.user u`}},
			{{Query: `WHERE (@name = '' OR u.user LIKE @name)`}},
			{{Query: `ORDER BY 1`}},
		},
		Fields: []dbmeta.Field{
			{Name: "name", Desc: "the user, with its host where it has one"},
			{Name: "superuser"}, {Name: "create_role"}, {Name: "create_db"},
			{Name: "can_login", Desc: "false for a role, which cannot log in;" +
				" MySQL locks the account of a role, so a locked user reads the same way"},
			{Name: "replication"},
			{Name: "bypass_rls", Desc: "always false: MariaDB has no row level security"},
			{Name: "inherit", Desc: "always true: MariaDB has no other behaviour"},
			{Name: "conn_limit"},
			{Name: "valid_until", Desc: "always absent: not recorded in mysql.user"},
			{Name: "member_of", Desc: "always empty: read role_grants instead"},
			{Name: "comment"},
		},
		Params: nameSystem("role"),
		Scan: func(rows *sql.Rows) (dbmeta.Role, error) {
			var v dbmeta.Role
			err := rows.Scan(&v.Name, &v.Superuser, &v.CreateRole, &v.CreateDB, &v.CanLogin,
				&v.Replication, &v.BypassRLS, &v.Inherit, &v.ConnLimit, &v.ValidUntil,
				&v.MemberOf, &v.Comment)
			return v, err
		},
	})

	dbmeta.RoleGrants.Register(dbmeta.MySQL, &dbmeta.Binding[dbmeta.RoleGrant]{
		Stmt: dbmeta.Stmt{
			// The two products record a membership in different tables with
			// different column names. MariaDB names the member and the role it
			// holds. MySQL names the role it came from and the account it went
			// to, so the columns are read the other way round.
			{
				frag(onMaria, `SELECT CONCAT(m.user, IF(m.host = '', '', CONCAT('@', m.host))) AS "role"`),
				frag(onMySQL, `SELECT CONCAT(m.to_user, IF(m.to_host = '', '', CONCAT('@', m.to_host))) AS "role"`),
			},
			{
				frag(onMaria, `, m.role AS "member_of"`),
				frag(onMySQL, `, m.from_user AS "member_of"`),
			},
			{{Query: `, NULL AS "grantor"`}},
			{
				frag(onMaria, `, m.admin_option = 1 AS "admin"`),
				frag(onMySQL, `, m.with_admin_option = 'Y' AS "admin"`),
			},
			{{Query: `, TRUE AS "inherit"`}},
			{{Query: `, TRUE AS "set"`}},
			{
				frag(onMaria, `FROM mysql.roles_mapping m`),
				frag(onMySQL, `FROM mysql.role_edges m`),
			},
			{
				frag(onMaria, `WHERE (@name = '' OR m.user LIKE @name)`),
				frag(onMySQL, `WHERE (@name = '' OR m.to_user LIKE @name)`),
			},
			{{Query: `ORDER BY 1, 2`}},
		},
		Fields: []dbmeta.Field{
			{Name: "role"}, {Name: "member_of"},
			{Name: "grantor", Desc: "always absent: neither product records who granted it"},
			{Name: "admin"},
			{Name: "inherit", Desc: "always true: MariaDB has no other behaviour"},
			{Name: "set", Desc: "always true: MariaDB has no other behaviour"},
		},
		Params: []dbmeta.Param{{Name: "name", Desc: "member name pattern, empty for every one", Default: ""}},
		Scan: func(rows *sql.Rows) (dbmeta.RoleGrant, error) {
			var v dbmeta.RoleGrant
			err := rows.Scan(&v.Role, &v.MemberOf, &v.Grantor, &v.Admin, &v.Inherit, &v.Set)
			return v, err
		},
	})

	// One row per grant, where psql gathers the grants of an object into one
	// row. The standard shape is kept, because gathering needs a string
	// aggregate and the caller can do it.
	dbmeta.Privileges.Register(dbmeta.MySQL, &dbmeta.Binding[dbmeta.Privilege]{
		Stmt: dbmeta.Stmt{
			{{Query: `SELECT p.table_schema AS "schema"`}},
			{{Query: `, p.table_name AS "name"`}},
			{{Query: `, 'table' AS "type"`}},
			{{Query: `, CONCAT(p.grantee, '=', p.privilege_type) AS "access"`}},
			{{Query: `, NULL AS "column_access"`}},
			{{Query: `, NULL AS "policies"`}},
			{{Query: `FROM information_schema.TABLE_PRIVILEGES p`}},
			{{Query: `WHERE (@with_system OR p.table_schema NOT IN (` + systemSchemas + `))`}},
			{{Query: `AND (@schema = '' OR p.table_schema LIKE @schema)`}},
			{{Query: `AND (@name = '' OR p.table_name LIKE @name)`}},
			{{Query: `ORDER BY 1, 2`}},
		},
		Fields: []dbmeta.Field{
			{Name: "schema"}, {Name: "name"}, {Name: "type"},
			{Name: "access", Desc: "one grant per row, not the gathered list psql prints"},
			{Name: "column_access", Desc: "always absent: read column_privileges instead"},
			{Name: "policies", Desc: "always absent: MariaDB has no row level security"},
		},
		Params: schemaNameSystem("table"),
		Scan: func(rows *sql.Rows) (dbmeta.Privilege, error) {
			var v dbmeta.Privilege
			err := rows.Scan(&v.Schema, &v.Name, &v.Type, &v.Access, &v.ColumnAccess, &v.Policies)
			return v, err
		},
	})
}
