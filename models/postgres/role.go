package postgres

import (
	"database/sql"

	"github.com/xo/dbmeta"
)

// Roles, grants and the objects reached through a foreign data wrapper.

func init() {
	registerRoles()
	registerRoleSettings()
	registerRoleGrants()
	registerPrivileges()
	registerDefaultACLs()
	registerForeignDataWrappers()
	registerForeignServers()
	registerUserMappings()
	registerForeignTables()
}

// registerRoles backs \du and \dg, from describeRoles.
//
// rolbypassrls arrived in release 9.5, which is below the floor, so nothing
// here is gated.
func registerRoles() {
	dbmeta.Roles.Register(dbmeta.PostgreSQL, &dbmeta.Binding[dbmeta.Role]{
		Stmt: dbmeta.Stmt{
			{{Query: `SELECT r.rolname AS "name"`}},
			{{Query: `, r.rolsuper AS "superuser"`}},
			{{Query: `, r.rolcreaterole AS "create_role"`}},
			{{Query: `, r.rolcreatedb AS "create_db"`}},
			{{Query: `, r.rolcanlogin AS "can_login"`}},
			{{Query: `, r.rolreplication AS "replication"`}},
			{{Query: `, r.rolbypassrls AS "bypass_rls"`}},
			{{Query: `, r.rolinherit AS "inherit"`}},
			{{Query: `, r.rolconnlimit AS "conn_limit"`}},
			{{Query: `, r.rolvaliduntil::text AS "valid_until"`}},
			{{Query: `, COALESCE((SELECT pg_catalog.string_agg(b.rolname, ', ' ORDER BY b.rolname)` +
				` FROM pg_catalog.pg_auth_members m` +
				` JOIN pg_catalog.pg_roles b ON m.roleid = b.oid` +
				` WHERE m.member = r.oid), '') AS "member_of"`}},
			{{Query: `, pg_catalog.shobj_description(r.oid, 'pg_authid') AS "comment"`}},
			{{Query: `FROM pg_catalog.pg_roles r`}},
			{{Query: `WHERE (@with_system OR r.rolname !~ '^pg_')`}},
			{{Query: `AND (@name = '' OR r.rolname LIKE @name)`}},
			{{Query: `ORDER BY 1`}},
		},
		Fields: fields("name", "superuser", "create_role", "create_db", "can_login",
			"replication", "bypass_rls", "inherit", "conn_limit", "valid_until",
			"member_of", "comment"),
		Params: []dbmeta.Param{
			{Name: "name", Desc: "role name pattern, empty for every role", Default: ""},
			{Name: "with_system", Desc: "include the roles PostgreSQL keeps for itself", Default: false},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Role, error) {
			var v dbmeta.Role
			err := rows.Scan(&v.Name, &v.Superuser, &v.CreateRole, &v.CreateDB, &v.CanLogin,
				&v.Replication, &v.BypassRLS, &v.Inherit, &v.ConnLimit, &v.ValidUntil,
				&v.MemberOf, &v.Comment)
			return v, err
		},
	})
}

// registerRoleSettings backs \drds, from listDbRoleSettings.
func registerRoleSettings() {
	dbmeta.RoleSettings.Register(dbmeta.PostgreSQL, &dbmeta.Binding[dbmeta.RoleSetting]{
		Stmt: dbmeta.Stmt{
			{{Query: `SELECT r.rolname AS "role"`}},
			{{Query: `, d.datname AS "database"`}},
			{{Query: `, pg_catalog.array_to_string(s.setconfig, E'\n') AS "settings"`}},
			{{Query: `FROM pg_catalog.pg_db_role_setting s`}},
			{{Query: `LEFT JOIN pg_catalog.pg_database d ON d.oid = s.setdatabase`}},
			{{Query: `LEFT JOIN pg_catalog.pg_roles r ON r.oid = s.setrole`}},
			{{Query: `WHERE (@name = '' OR COALESCE(r.rolname, '') LIKE @name)`}},
			{{Query: `AND (@database = '' OR COALESCE(d.datname, '') LIKE @database)`}},
			{{Query: `ORDER BY 1, 2`}},
		},
		Fields: fields("role", "database", "settings"),
		Params: []dbmeta.Param{
			{Name: "name", Desc: "role name pattern, empty for every role", Default: ""},
			{Name: "database", Desc: "database name pattern, empty for every database", Default: ""},
		},
		Scan: func(rows *sql.Rows) (dbmeta.RoleSetting, error) {
			var v dbmeta.RoleSetting
			err := rows.Scan(&v.Role, &v.Database, &v.Settings)
			return v, err
		},
	})
}

// registerRoleGrants backs \drg, from describeRoleGrants.
//
// pg_auth_members gained the grantor, inherit_option and set_option columns in
// release 16, so an older server reports the defaults that applied before
// them: membership inherited and settable, with no recorded grantor.
func registerRoleGrants() {
	dbmeta.RoleGrants.Register(dbmeta.PostgreSQL, &dbmeta.Binding[dbmeta.RoleGrant]{
		Stmt: dbmeta.Stmt{
			{{Query: `SELECT m.rolname AS "role"`}},
			{{Query: `, r.rolname AS "member_of"`}},
			{
				{Query: `, NULL AS "grantor"`},
				{Min: v16, Query: `, g.rolname AS "grantor"`},
			},
			{{Query: `, a.admin_option AS "admin"`}},
			{
				{Query: `, true AS "inherit"`},
				{Min: v16, Query: `, a.inherit_option AS "inherit"`},
			},
			{
				{Query: `, true AS "set"`},
				{Min: v16, Query: `, a.set_option AS "set"`},
			},
			{{Query: `FROM pg_catalog.pg_auth_members a`}},
			{{Query: `JOIN pg_catalog.pg_roles m ON m.oid = a.member`}},
			{{Query: `JOIN pg_catalog.pg_roles r ON r.oid = a.roleid`}},
			{
				{Query: ``},
				{Min: v16, Query: `LEFT JOIN pg_catalog.pg_roles g ON g.oid = a.grantor`},
			},
			{{Query: `WHERE (@name = '' OR m.rolname LIKE @name)`}},
			{{Query: `ORDER BY 1, 2`}},
		},
		Fields: []dbmeta.Field{
			{Name: "role"}, {Name: "member_of"},
			{Name: "grantor", Desc: "role that granted the membership", Min: v16},
			{Name: "admin"},
			{Name: "inherit", Desc: "whether the member inherits the privileges. Always true below release 16, which had no other behaviour"},
			{Name: "set", Desc: "whether the member may SET ROLE to it. Always true below release 16, which had no other behaviour"},
		},
		Params: []dbmeta.Param{{Name: "name", Desc: "member role name pattern, empty for every role", Default: ""}},
		Scan: func(rows *sql.Rows) (dbmeta.RoleGrant, error) {
			var v dbmeta.RoleGrant
			err := rows.Scan(&v.Role, &v.MemberOf, &v.Grantor, &v.Admin, &v.Inherit, &v.Set)
			return v, err
		},
	})
}

// registerPrivileges backs \z and \dp, from permissionsList.
func registerPrivileges() {
	dbmeta.Privileges.Register(dbmeta.PostgreSQL, &dbmeta.Binding[dbmeta.Privilege]{
		Stmt: dbmeta.Stmt{
			{{Query: `SELECT n.nspname AS "schema"`}},
			{{Query: `, c.relname AS "name"`}},
			{{Query: `, CASE c.relkind WHEN 'r' THEN 'table' WHEN 'p' THEN 'table'` +
				` WHEN 'v' THEN 'view' WHEN 'm' THEN 'materialized view'` +
				` WHEN 'S' THEN 'sequence' WHEN 'f' THEN 'foreign table'` +
				` ELSE c.relkind::text END AS "type"`}},
			{{Query: `, pg_catalog.array_to_string(c.relacl, E'\n') AS "access"`}},
			{{Query: `, COALESCE((SELECT pg_catalog.string_agg(a.attname || ':' ||` +
				` pg_catalog.array_to_string(a.attacl, ','), E'\n' ORDER BY a.attnum)` +
				` FROM pg_catalog.pg_attribute a` +
				` WHERE a.attrelid = c.oid AND a.attnum > 0 AND a.attacl IS NOT NULL), '') AS "column_access"`}},
			{{Query: `, COALESCE((SELECT pg_catalog.string_agg(p.polname, ', ' ORDER BY p.polname)` +
				` FROM pg_catalog.pg_policy p WHERE p.polrelid = c.oid), '') AS "policies"`}},
			{{Query: `FROM pg_catalog.pg_class c`}},
			{{Query: `JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace`}},
			{{Query: `WHERE c.relkind IN ('r', 'p', 'v', 'm', 'S', 'f')`}},
			{{Query: `AND (@with_system OR (n.nspname !~ '^pg_' AND n.nspname <> 'information_schema'))`}},
			{{Query: `AND (@schema = '' OR n.nspname LIKE @schema)`}},
			{{Query: `AND (@name = '' OR c.relname LIKE @name)`}},
			{{Query: `ORDER BY 1, 2`}},
		},
		Fields: fields("schema", "name", "type", "access", "column_access", "policies"),
		Params: schemaNameSystem("relation"),
		Scan: func(rows *sql.Rows) (dbmeta.Privilege, error) {
			var v dbmeta.Privilege
			err := rows.Scan(&v.Schema, &v.Name, &v.Type, &v.Access, &v.ColumnAccess, &v.Policies)
			return v, err
		},
	})
}

// registerDefaultACLs backs \ddp, from listDefaultACLs.
func registerDefaultACLs() {
	dbmeta.DefaultACLs.Register(dbmeta.PostgreSQL, &dbmeta.Binding[dbmeta.DefaultACL]{
		Stmt: dbmeta.Stmt{
			{{Query: `SELECT pg_catalog.pg_get_userbyid(d.defaclrole) AS "owner"`}},
			{{Query: `, n.nspname AS "schema"`}},
			{{Query: `, CASE d.defaclobjtype WHEN 'r' THEN 'table' WHEN 'S' THEN 'sequence'` +
				` WHEN 'f' THEN 'function' WHEN 'T' THEN 'type' WHEN 'n' THEN 'schema'` +
				` ELSE d.defaclobjtype::text END AS "type"`}},
			{{Query: `, pg_catalog.array_to_string(d.defaclacl, E'\n') AS "access"`}},
			{{Query: `FROM pg_catalog.pg_default_acl d`}},
			{{Query: `LEFT JOIN pg_catalog.pg_namespace n ON n.oid = d.defaclnamespace`}},
			{{Query: `WHERE (@schema = '' OR COALESCE(n.nspname, '') LIKE @schema)`}},
			{{Query: `ORDER BY 1, 2, 3`}},
		},
		Fields: fields("owner", "schema", "type", "access"),
		Params: []dbmeta.Param{{Name: "schema", Desc: "schema name pattern, empty for every schema", Default: ""}},
		Scan: func(rows *sql.Rows) (dbmeta.DefaultACL, error) {
			var v dbmeta.DefaultACL
			err := rows.Scan(&v.Owner, &v.Schema, &v.Type, &v.Access)
			return v, err
		},
	})
}

// registerForeignDataWrappers backs \dew, from listForeignDataWrappers.
func registerForeignDataWrappers() {
	dbmeta.ForeignDataWrappers.Register(dbmeta.PostgreSQL, &dbmeta.Binding[dbmeta.ForeignDataWrapper]{
		Stmt: dbmeta.Stmt{
			{{Query: `SELECT w.fdwname AS "name"`}},
			{{Query: `, pg_catalog.pg_get_userbyid(w.fdwowner) AS "owner"`}},
			{{Query: `, w.fdwhandler::pg_catalog.regproc::text AS "handler"`}},
			{{Query: `, w.fdwvalidator::pg_catalog.regproc::text AS "validator"`}},
			{{Query: `, pg_catalog.array_to_string(w.fdwacl, E'\n') AS "access"`}},
			{{Query: `, pg_catalog.array_to_string(w.fdwoptions, ', ') AS "options"`}},
			{{Query: `, pg_catalog.obj_description(w.oid, 'pg_foreign_data_wrapper') AS "comment"`}},
			{{Query: `FROM pg_catalog.pg_foreign_data_wrapper w`}},
			{{Query: `WHERE (@name = '' OR w.fdwname LIKE @name)`}},
			{{Query: `ORDER BY 1`}},
		},
		Fields: fields("name", "owner", "handler", "validator", "access", "options", "comment"),
		Params: []dbmeta.Param{{Name: "name", Desc: "wrapper name pattern, empty for every wrapper", Default: ""}},
		Scan: func(rows *sql.Rows) (dbmeta.ForeignDataWrapper, error) {
			var v dbmeta.ForeignDataWrapper
			err := rows.Scan(&v.Name, &v.Owner, &v.Handler, &v.Validator, &v.Access, &v.Options, &v.Comment)
			return v, err
		},
	})
}

// registerForeignServers backs \des, from listForeignServers.
func registerForeignServers() {
	dbmeta.ForeignServers.Register(dbmeta.PostgreSQL, &dbmeta.Binding[dbmeta.ForeignServer]{
		Stmt: dbmeta.Stmt{
			{{Query: `SELECT s.srvname AS "name"`}},
			{{Query: `, pg_catalog.pg_get_userbyid(s.srvowner) AS "owner"`}},
			{{Query: `, w.fdwname AS "wrapper"`}},
			{{Query: `, s.srvtype AS "type"`}},
			{{Query: `, s.srvversion AS "version"`}},
			{{Query: `, pg_catalog.array_to_string(s.srvacl, E'\n') AS "access"`}},
			{{Query: `, pg_catalog.array_to_string(s.srvoptions, ', ') AS "options"`}},
			{{Query: `, pg_catalog.obj_description(s.oid, 'pg_foreign_server') AS "comment"`}},
			{{Query: `FROM pg_catalog.pg_foreign_server s`}},
			{{Query: `JOIN pg_catalog.pg_foreign_data_wrapper w ON w.oid = s.srvfdw`}},
			{{Query: `WHERE (@name = '' OR s.srvname LIKE @name)`}},
			{{Query: `ORDER BY 1`}},
		},
		Fields: fields("name", "owner", "wrapper", "type", "version", "access", "options", "comment"),
		Params: []dbmeta.Param{{Name: "name", Desc: "server name pattern, empty for every server", Default: ""}},
		Scan: func(rows *sql.Rows) (dbmeta.ForeignServer, error) {
			var v dbmeta.ForeignServer
			err := rows.Scan(&v.Name, &v.Owner, &v.Wrapper, &v.Type, &v.Version,
				&v.Access, &v.Options, &v.Comment)
			return v, err
		},
	})
}

// registerUserMappings backs \deu, from listUserMappings.
func registerUserMappings() {
	dbmeta.UserMappings.Register(dbmeta.PostgreSQL, &dbmeta.Binding[dbmeta.UserMapping]{
		Stmt: dbmeta.Stmt{
			{{Query: `SELECT um.srvname AS "server"`}},
			{{Query: `, um.usename AS "name"`}},
			{{Query: `, pg_catalog.array_to_string(um.umoptions, ', ') AS "options"`}},
			{{Query: `FROM pg_catalog.pg_user_mappings um`}},
			{{Query: `WHERE (@name = '' OR COALESCE(um.usename, '') LIKE @name)`}},
			{{Query: `AND (@server = '' OR um.srvname LIKE @server)`}},
			{{Query: `ORDER BY 1, 2`}},
		},
		Fields: fields("server", "name", "options"),
		Params: []dbmeta.Param{
			{Name: "name", Desc: "user name pattern, empty for every mapping", Default: ""},
			{Name: "server", Desc: "server name pattern, empty for every server", Default: ""},
		},
		Scan: func(rows *sql.Rows) (dbmeta.UserMapping, error) {
			var v dbmeta.UserMapping
			err := rows.Scan(&v.Server, &v.Name, &v.Options)
			return v, err
		},
	})
}

// registerForeignTables backs \det, from listForeignTables.
func registerForeignTables() {
	dbmeta.ForeignTables.Register(dbmeta.PostgreSQL, &dbmeta.Binding[dbmeta.ForeignTable]{
		Stmt: dbmeta.Stmt{
			{{Query: `SELECT n.nspname AS "schema"`}},
			{{Query: `, c.relname AS "name"`}},
			{{Query: `, s.srvname AS "server"`}},
			{{Query: `, pg_catalog.array_to_string(ft.ftoptions, ', ') AS "options"`}},
			{{Query: `, pg_catalog.obj_description(c.oid, 'pg_class') AS "comment"`}},
			{{Query: `FROM pg_catalog.pg_foreign_table ft`}},
			{{Query: `JOIN pg_catalog.pg_class c ON c.oid = ft.ftrelid`}},
			{{Query: `JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace`}},
			{{Query: `JOIN pg_catalog.pg_foreign_server s ON s.oid = ft.ftserver`}},
			{{Query: `WHERE (@schema = '' OR n.nspname LIKE @schema)`}},
			{{Query: `AND (@name = '' OR c.relname LIKE @name)`}},
			{{Query: `ORDER BY 1, 2`}},
		},
		Fields: fields("schema", "name", "server", "options", "comment"),
		Params: []dbmeta.Param{
			{Name: "schema", Desc: "schema name pattern, empty for every schema", Default: ""},
			{Name: "name", Desc: "table name pattern, empty for every foreign table", Default: ""},
		},
		Scan: func(rows *sql.Rows) (dbmeta.ForeignTable, error) {
			var v dbmeta.ForeignTable
			err := rows.Scan(&v.Schema, &v.Name, &v.Server, &v.Options, &v.Comment)
			return v, err
		},
	})
}
