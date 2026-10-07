package avatica

import (
	"database/sql"

	"github.com/xo/dbmeta"
)

// HSQLDB gives the same label to two columns that are the same constant, so a
// result with FALSE twice names both of them by the last alias. The flags that
// are constants are written as comparisons that differ, which is a boolean
// with a label of its own.
func registerRoles() {
	// AUTHORIZATIONS lists every user and every role, and SYSTEM_USERS says
	// which user is an administrator. An ordinary user sees itself alone in
	// both. DBA is the role that holds every right, so it is the superuser.
	dbmeta.Roles.Register(dbmeta.Avatica, &dbmeta.Binding[dbmeta.Role]{
		Stmt: dbmeta.Stmt{
			always(`SELECT a.AUTHORIZATION_NAME AS "name"`),
			always(`, CASE WHEN a.AUTHORIZATION_TYPE = 'USER' THEN u.ADMIN ELSE a.AUTHORIZATION_NAME = 'DBA' END AS "superuser"`),
			always(`, CASE WHEN a.AUTHORIZATION_TYPE = 'USER' THEN u.ADMIN ELSE a.AUTHORIZATION_NAME = 'DBA' END AS "create_role"`),
			always(`, (1 = 2) AS "create_db"`),
			always(`, a.AUTHORIZATION_TYPE = 'USER' AS "can_login"`),
			always(`, (3 = 4) AS "replication"`),
			always(`, (5 = 6) AS "bypass_rls"`),
			always(`, (7 = 7) AS "inherit"`),
			always(`, CAST(0 AS BIGINT) AS "conn_limit"`),
			always(`, ` + text + ` AS "valid_until"`),
			always(`, COALESCE((SELECT GROUP_CONCAT(g.ROLE_NAME ORDER BY g.ROLE_NAME SEPARATOR ', ')` +
				` FROM INFORMATION_SCHEMA.ROLE_AUTHORIZATION_DESCRIPTORS g WHERE g.GRANTEE = a.AUTHORIZATION_NAME), '') AS "member_of"`),
			always(`, ` + text + ` AS "comment"`),
			always(`FROM INFORMATION_SCHEMA.AUTHORIZATIONS a`),
			always(`LEFT JOIN INFORMATION_SCHEMA.SYSTEM_USERS u ON u.USER_NAME = a.AUTHORIZATION_NAME`),
			always(`WHERE ` + like("a.AUTHORIZATION_NAME", "@name")),
			always(`ORDER BY 1`),
		},
		Fields: []dbmeta.Field{
			{Name: "name"},
			{Name: "superuser", Desc: "from the ADMIN column of SYSTEM_USERS for a user, and true for the role DBA alone. A user is an administrator when it holds DBA"},
			{Name: "create_role", Desc: "the same as superuser: CREATE ROLE needs DBA"},
			{Name: "create_db", Desc: "always false: HSQLDB has one database for each server and no CREATE DATABASE"},
			{Name: "can_login", Desc: "true for a user, and false for a role, which is what separates the two halves of this result"},
			{Name: "replication", Desc: "always false: HSQLDB has no replication right"},
			{Name: "bypass_rls", Desc: "always false: HSQLDB has no row level security"},
			{Name: "inherit", Desc: "always true: a granted role applies to its members"},
			{Name: "conn_limit", Desc: "always zero: HSQLDB sets no connection limit on a user"},
			{Name: "valid_until", Desc: "always absent: an HSQLDB user does not expire"},
			{Name: "member_of", Desc: "the roles granted directly to this one, comma separated. Empty where there are none. RoleGrants lists them one to a row"},
			{Name: "comment", Desc: "always absent: COMMENT ON takes no user and no role in HSQLDB"},
		},
		Params: []dbmeta.Param{
			{Name: "name", Desc: "principal name pattern, empty for every user and role", Default: ""},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Role, error) {
			var v dbmeta.Role
			err := rows.Scan(&v.Name, &v.Superuser, &v.CreateRole, &v.CreateDB,
				&v.CanLogin, &v.Replication, &v.BypassRLS, &v.Inherit, &v.ConnLimit,
				&v.ValidUntil, &v.MemberOf, &v.Comment)
			return v, err
		},
	})

	dbmeta.RoleGrants.Register(dbmeta.Avatica, &dbmeta.Binding[dbmeta.RoleGrant]{
		Stmt: dbmeta.Stmt{
			always(`SELECT g.GRANTEE AS "role"`),
			always(`, g.ROLE_NAME AS "member_of"`),
			always(`, g.GRANTOR AS "grantor"`),
			always(`, g.IS_GRANTABLE = 'YES' AS "admin"`),
			always(`, (1 = 1) AS "inherit"`),
			always(`, (2 = 2) AS "set"`),
			always(`FROM INFORMATION_SCHEMA.ROLE_AUTHORIZATION_DESCRIPTORS g`),
			always(`WHERE ` + like("g.GRANTEE", "@name")),
			always(`ORDER BY g.GRANTEE, g.ROLE_NAME`),
		},
		Fields: []dbmeta.Field{
			{Name: "role", Desc: "the grantee, which is a user or another role"},
			{Name: "member_of", Desc: "the role granted"},
			{Name: "grantor", Desc: "_SYSTEM for a grant the server made when it started, such as SA holding DBA"},
			{Name: "admin", Desc: "from IS_GRANTABLE, which is true for WITH ADMIN OPTION"},
			{Name: "inherit", Desc: "always true: a granted role applies to its members"},
			{Name: "set", Desc: "always true: HSQLDB enables a granted role for the session"},
		},
		Params: []dbmeta.Param{
			{Name: "name", Desc: "grantee name pattern, empty for every grantee", Default: ""},
		},
		Scan: func(rows *sql.Rows) (dbmeta.RoleGrant, error) {
			var v dbmeta.RoleGrant
			err := rows.Scan(&v.Role, &v.MemberOf, &v.Grantor, &v.Admin, &v.Inherit, &v.Set)
			return v, err
		},
	})

	registerPrivileges()
}

// grants folds the grants on one object into the string psql prints,
// grantee=privilege, comma separated.
func grants(alias, privilege string) string {
	return `GROUP_CONCAT(` + alias + `.GRANTEE || '=' || ` + alias + `.` + privilege +
		` ORDER BY ` + alias + `.GRANTEE, ` + alias + `.` + privilege + ` SEPARATOR ', ')`
}

func registerPrivileges() {
	// One row for each object, with the grants folded into a string. A table
	// or a view has its grants in TABLE_PRIVILEGES and the grants on its
	// columns in COLUMN_PRIVILEGES, which also repeats a grant on the whole
	// table for every column, so a column grant is only the one the table
	// does not carry. Both are read once and grouped, first by grantee and
	// privilege, where a group with no table row is a column grant alone, and
	// then by object. A join of the two views checks each column against the
	// table grants and took 93 seconds over 3,500 tables (D186). A routine has
	// ROUTINE_PRIVILEGES, and a sequence, a domain and a type have
	// USAGE_PRIVILEGES.
	dbmeta.Privileges.Register(dbmeta.Avatica, &dbmeta.Binding[dbmeta.Privilege]{
		Stmt: dbmeta.Stmt{
			always(`SELECT s.TABLE_SCHEMA AS "schema"`),
			always(`, s.TABLE_NAME AS "name"`),
			always(`, RTRIM(MAX(CASE WHEN v.TABLE_NAME IS NOT NULL THEN 'view' ELSE 'table' END)) AS "type"`),
			always(`, GROUP_CONCAT(CASE WHEN s.TN > 0 THEN s.GRANTEE || '=' || s.PRIV END` +
				` ORDER BY s.GRANTEE, s.PRIV SEPARATOR ', ') AS "access"`),
			always(`, GROUP_CONCAT(CASE WHEN s.TN = 0 THEN s.COLS END` +
				` ORDER BY s.GRANTEE, s.PRIV SEPARATOR ', ') AS "column_access"`),
			always(`, ` + text + ` AS "policies"`),
			always(`FROM (SELECT u.TABLE_SCHEMA, u.TABLE_NAME, u.GRANTEE, u.PRIV, SUM(u.T) AS TN,` +
				` GROUP_CONCAT(u.COL ORDER BY u.COL SEPARATOR ', ') AS COLS`),
			always(`FROM (SELECT TABLE_SCHEMA, TABLE_NAME, GRANTEE, PRIVILEGE_TYPE AS PRIV, 1 AS T, CAST(NULL AS VARCHAR(400)) AS COL` +
				` FROM INFORMATION_SCHEMA.TABLE_PRIVILEGES`),
			always(`UNION ALL SELECT TABLE_SCHEMA, TABLE_NAME, GRANTEE, PRIVILEGE_TYPE, 0,` +
				` COLUMN_NAME || ':' || GRANTEE || '=' || PRIVILEGE_TYPE FROM INFORMATION_SCHEMA.COLUMN_PRIVILEGES) u`),
			always(`GROUP BY u.TABLE_SCHEMA, u.TABLE_NAME, u.GRANTEE, u.PRIV) s`),
			always(`LEFT JOIN INFORMATION_SCHEMA.VIEWS v ON v.TABLE_SCHEMA = s.TABLE_SCHEMA AND v.TABLE_NAME = s.TABLE_NAME`),
			always(`WHERE ` + notSystem("s.TABLE_SCHEMA")),
			always(`AND ` + like("s.TABLE_SCHEMA", "@schema")),
			always(`AND ` + like("s.TABLE_NAME", "@name")),
			always(`GROUP BY s.TABLE_SCHEMA, s.TABLE_NAME`),
			always(`UNION ALL`),
			always(`SELECT r.ROUTINE_SCHEMA, r.ROUTINE_NAME, LOWER(m.ROUTINE_TYPE)`),
			always(`, ` + grants("r", "PRIVILEGE_TYPE") + `, ` + text + `, ` + text),
			always(`FROM INFORMATION_SCHEMA.ROUTINE_PRIVILEGES r`),
			always(`JOIN INFORMATION_SCHEMA.ROUTINES m ON m.SPECIFIC_SCHEMA = r.SPECIFIC_SCHEMA AND m.SPECIFIC_NAME = r.SPECIFIC_NAME`),
			always(`WHERE ` + notSystem("r.ROUTINE_SCHEMA")),
			always(`AND ` + like("r.ROUTINE_SCHEMA", "@schema")),
			always(`AND ` + like("r.ROUTINE_NAME", "@name")),
			always(`GROUP BY r.ROUTINE_SCHEMA, r.ROUTINE_NAME, m.ROUTINE_TYPE`),
			always(`UNION ALL`),
			always(`SELECT u.OBJECT_SCHEMA, u.OBJECT_NAME, LOWER(u.OBJECT_TYPE)`),
			always(`, ` + grants("u", "PRIVILEGE_TYPE") + `, ` + text + `, ` + text),
			always(`FROM INFORMATION_SCHEMA.USAGE_PRIVILEGES u`),
			always(`WHERE ` + notSystem("u.OBJECT_SCHEMA")),
			always(`AND ` + like("u.OBJECT_SCHEMA", "@schema")),
			always(`AND ` + like("u.OBJECT_NAME", "@name")),
			always(`GROUP BY u.OBJECT_SCHEMA, u.OBJECT_NAME, u.OBJECT_TYPE`),
			always(`ORDER BY 1, 2`),
		},
		Fields: []dbmeta.Field{
			{Name: "schema"}, {Name: "name"},
			{Name: "type", Desc: "table, view, function, procedure, sequence or domain, from the view the grant is in"},
			{Name: "access", Desc: "grantee=privilege, comma separated. The owner has a grant from the server, so an object always has some. Absent for an object that has only grants on columns"},
			{Name: "column_access", Desc: "column:grantee=privilege, comma separated, for a grant on a column that the table level grant does not carry. Absent where there is none, and for every object that is not a table"},
			{Name: "policies", Desc: "always absent: HSQLDB has no row level security"},
		},
		Params: schemaAndName("object"),
		Scan: func(rows *sql.Rows) (dbmeta.Privilege, error) {
			var v dbmeta.Privilege
			err := rows.Scan(&v.Schema, &v.Name, &v.Type, &v.Access, &v.ColumnAccess, &v.Policies)
			return v, err
		},
	})
}
