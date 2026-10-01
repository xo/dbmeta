package vertica

import (
	"database/sql"

	"github.com/xo/dbmeta"
)

func registerRoles() {
	// \du and \dg. Vertica keeps users and roles in two views and both are
	// principals, so this is one statement over both and can_login is what
	// separates them.
	dbmeta.Roles.Register(dbmeta.Vertica, &dbmeta.Binding[dbmeta.Role]{
		Stmt: dbmeta.Stmt{
			always(`SELECT u.user_name AS "name"`),
			always(`, u.is_super_user AS "superuser"`),
			always(`, FALSE AS "create_role"`),
			always(`, FALSE AS "create_db"`),
			always(`, NOT u.is_locked AS "can_login"`),
			always(`, FALSE AS "replication"`),
			always(`, FALSE AS "bypass_rls"`),
			always(`, TRUE AS "inherit"`),
			// A limit is a number or the word unlimited, and unlimited is
			// read as -1, which is what PostgreSQL's catalog calls no limit.
			since(v91, `, CASE WHEN u.max_connections = 'unlimited' THEN -1`+
				` ELSE CAST(u.max_connections AS INTEGER) END AS "conn_limit"`,
				`, CAST(-1 AS INTEGER) AS "conn_limit"`),
			always(`, CAST(NULL AS VARCHAR) AS "valid_until"`),
			always(`, '' AS "member_of"`),
			always(`, ` + comment("USER", "''", "u.user_name") + ` AS "comment"`),
			always(`FROM v_catalog.users u`),
			always(`WHERE ` + like(`u.user_name`, `@name`)),
			always(`UNION ALL`),
			always(`SELECT r.name, FALSE, FALSE, FALSE, FALSE, FALSE, FALSE, TRUE`),
			always(`, CAST(-1 AS INTEGER), CAST(NULL AS VARCHAR), ''`),
			always(`, ` + comment("ROLE", "''", "r.name")),
			always(`FROM v_catalog.roles r`),
			always(`WHERE ` + like(`r.name`, `@name`)),
			always(`ORDER BY 1`),
		},
		Fields: []dbmeta.Field{
			{Name: "name"},
			{Name: "superuser", Desc: "from is_super_user, which is true for dbadmin, the user the database is created with. PSEUDOSUPERUSER is a role and is not reported here"},
			{Name: "create_role", Desc: "always false: the right to create a role belongs to a superuser or to the USERADMIN role, and is not on the principal"},
			{Name: "create_db", Desc: "always false: a Vertica cluster runs one database and no user creates another"},
			{Name: "can_login", Desc: "true for a user whose account is not locked, and false for a role"},
			{Name: "replication", Desc: "always false: Vertica has no replication right"},
			{Name: "bypass_rls", Desc: "always false: an access policy applies to every principal it names, and no right bypasses it"},
			{Name: "inherit", Desc: "always true. A granted role is active only once SET ROLE enables it or ALTER USER makes it a default, and a role granted to a role follows whenever that role is active"},
			{Name: "conn_limit", Desc: "the user's MAXCONNECTIONS, and -1 for unlimited or for a role. 7.2 does not record it, so it reads -1 there"},
			{Name: "valid_until", Desc: "always absent: Vertica expires a password through a profile rather than on the user"},
			{Name: "member_of", Desc: "always empty: RoleGrants reads membership"},
			{Name: "comment", Desc: "from COMMENT ON USER or COMMENT ON ROLE"},
		},
		Params: nameOnly("principal"),
		Scan: func(rows *sql.Rows) (dbmeta.Role, error) {
			var v dbmeta.Role
			err := rows.Scan(&v.Name, &v.Superuser, &v.CreateRole, &v.CreateDB,
				&v.CanLogin, &v.Replication, &v.BypassRLS, &v.Inherit, &v.ConnLimit,
				&v.ValidUntil, &v.MemberOf, &v.Comment)
			return v, err
		},
	})

	// \drds. A user can carry its own value for a parameter, set with ALTER
	// USER ... SET, which is what ALTER ROLE ... SET records in PostgreSQL.
	// user_configuration_parameters is measured on 25.1 and absent on 10.1.
	dbmeta.RoleSettings.Register(dbmeta.Vertica, &dbmeta.Binding[dbmeta.RoleSetting]{
		Stmt: dbmeta.Stmt{
			{{Min: v251, Query: `SELECT u.user_name AS "role"`}},
			always(`, '' AS "database"`),
			always(`, LISTAGG(u.parameter_name || '=' || u.current_value` +
				` USING PARAMETERS separator = ', ') AS "settings"`),
			always(`FROM v_catalog.user_configuration_parameters u`),
			always(`WHERE ` + like(`u.user_name`, `@name`)),
			always(`GROUP BY u.user_name`),
			always(`ORDER BY u.user_name`),
		},
		Fields: []dbmeta.Field{
			{Name: "role", Desc: "the user. A Vertica role carries no parameters, so every row names a user"},
			{Name: "database", Desc: "always empty: a Vertica cluster runs one database, so a setting applies in it"},
			{Name: "settings", Desc: "parameter=value, comma separated"},
		},
		Params: nameOnly("user"),
		Scan: func(rows *sql.Rows) (dbmeta.RoleSetting, error) {
			var v dbmeta.RoleSetting
			err := rows.Scan(&v.Role, &v.Database, &v.Settings)
			return v, err
		},
	})

	// \drg. A role grant is a row in v_catalog.grants whose object is a role.
	dbmeta.RoleGrants.Register(dbmeta.Vertica, &dbmeta.Binding[dbmeta.RoleGrant]{
		Stmt: dbmeta.Stmt{
			always(`SELECT g.grantee AS "role"`),
			always(`, g.object_name AS "member_of"`),
			always(`, g.grantor AS "grantor"`),
			always(`, CASE WHEN g.privileges_description LIKE '%*%' THEN TRUE ELSE FALSE END AS "admin"`),
			always(`, TRUE AS "inherit"`),
			always(`, TRUE AS "set"`),
			always(`FROM v_catalog.grants g`),
			always(`WHERE g.object_type = 'ROLE'`),
			always(`AND ` + like(`g.grantee`, `@name`)),
			always(`ORDER BY g.grantee, g.object_name`),
		},
		Fields: []dbmeta.Field{
			{Name: "role", Desc: "the grantee, which is a user or another role"},
			{Name: "member_of", Desc: "the role granted"},
			{Name: "grantor"},
			{Name: "admin", Desc: "true where the grant carries WITH ADMIN OPTION, which Vertica marks with an asterisk"},
			{Name: "inherit", Desc: "always true: see Roles for when a granted role is active"},
			{Name: "set", Desc: "always true: a grantee can enable a granted role with SET ROLE"},
		},
		Params: nameOnly("grantee"),
		Scan: func(rows *sql.Rows) (dbmeta.RoleGrant, error) {
			var v dbmeta.RoleGrant
			err := rows.Scan(&v.Role, &v.MemberOf, &v.Grantor, &v.Admin, &v.Inherit, &v.Set)
			return v, err
		},
	})

	// \dp. One row per object with its grants folded together, from 10.1,
	// where LISTAGG arrived. Before it there is no string aggregate, so an
	// object granted to several principals has a row for each.
	dbmeta.Privileges.Register(dbmeta.Vertica, &dbmeta.Binding[dbmeta.Privilege]{
		Stmt: dbmeta.Stmt{
			always(`SELECT g.object_schema AS "schema"`),
			always(`, g.object_name AS "name"`),
			always(`, LOWER(g.object_type) AS "type"`),
			since(v101, `, LISTAGG(g.grantee || '=' || g.privileges_description`+
				` USING PARAMETERS separator = ', ') AS "access"`,
				`, g.grantee || '=' || g.privileges_description AS "access"`),
			always(`, CAST(NULL AS VARCHAR) AS "column_access"`),
			since(v101, `, COALESCE(a.policies, '') AS "policies"`, `, CAST(NULL AS VARCHAR) AS "policies"`),
			always(`FROM v_catalog.grants g`),
			// Vertica refuses a subquery beside GROUP BY, so the policies
			// are folded per table first and joined. A table's name here is
			// schema qualified.
			since(v101, `LEFT JOIN (SELECT a.table_name, LISTAGG(a.column_name || ': ' || a.expression`+
				` USING PARAMETERS separator = '; ') AS policies`+
				` FROM v_catalog.access_policy a GROUP BY a.table_name) a`+
				` ON a.table_name = g.object_schema || '.' || g.object_name`, ``),
			always(`WHERE g.object_type NOT IN ('ROLE', 'RESOURCEPOOL')`),
			always(`AND ` + like(`COALESCE(g.object_schema, '')`, `@schema`)),
			always(`AND ` + like(`g.object_name`, `@name`)),
			since(v101, `GROUP BY g.object_schema, g.object_name, g.object_type, a.policies`, ``),
			always(`ORDER BY 1, 2`),
		},
		Fields: []dbmeta.Field{
			{Name: "schema", Desc: "absent for an object that belongs to the database, such as a schema"},
			{Name: "name"},
			{Name: "type", Desc: "the kind of object, lower cased, such as table, view, schema, sequence or procedure"},
			{Name: "access", Desc: "grantee=privileges, where an asterisk after a privilege means WITH GRANT OPTION. From 10.1 every grantee on an object is on one row, comma separated. Before 10.1 there is a row per grantee"},
			{Name: "column_access", Desc: "always absent: Vertica grants on a whole object and has no column grant. A column access policy is in policies"},
			{Name: "policies", Min: v101, Desc: "the access policies on a table, column: expression, separated by semicolons, from 10.1. A row policy names no column. Absent before 10.1, which has no string aggregate to fold them with"},
		},
		Params: []dbmeta.Param{
			{Name: "schema", Desc: "schema name pattern, empty for every schema", Default: ""},
			{Name: "name", Desc: "object name pattern, empty for every one", Default: ""},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Privilege, error) {
			var v dbmeta.Privilege
			err := rows.Scan(&v.Schema, &v.Name, &v.Type, &v.Access, &v.ColumnAccess, &v.Policies)
			return v, err
		},
	})
}
