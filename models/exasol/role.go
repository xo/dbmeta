package exasol

import (
	"database/sql"

	"github.com/xo/dbmeta"
)

func registerRoles() {
	// \du and \dg. Exasol keeps users and roles in two views and both are
	// principals, so this is one statement over both and can_login is what
	// separates them.
	//
	// EXA_ALL_USERS lists the users the current user can see, which for an
	// ordinary user is itself, and EXA_ALL_ROLES lists every role.
	dbmeta.Roles.Register(dbmeta.Exasol, &dbmeta.Binding[dbmeta.Role]{
		Stmt: dbmeta.Stmt{
			always(`SELECT u.USER_NAME AS "name"`),
			// Exasol has no superuser flag. SYS is created with the
			// database and holds every privilege without a grant, which is
			// what a superuser is.
			always(`, CASE WHEN u.USER_NAME = 'SYS' THEN TRUE ELSE FALSE END AS "superuser"`),
			always(`, FALSE AS "create_role"`),
			always(`, FALSE AS "create_db"`),
			always(`, TRUE AS "can_login"`),
			always(`, FALSE AS "replication"`),
			always(`, FALSE AS "bypass_rls"`),
			always(`, TRUE AS "inherit"`),
			always(`, CAST(0 AS BIGINT) AS "conn_limit"`),
			always(`, CAST(NULL AS VARCHAR(1)) AS "valid_until"`),
			always(`, '' AS "member_of"`),
			always(`, u.USER_COMMENT AS "comment"`),
			always(`FROM EXA_ALL_USERS u`),
			always(`WHERE ` + like(`u.USER_NAME`, `@name`)),
			always(`UNION ALL`),
			always(`SELECT r.ROLE_NAME`),
			always(`, FALSE, FALSE, FALSE, FALSE, FALSE, FALSE, TRUE`),
			always(`, CAST(0 AS BIGINT)`),
			always(`, CAST(NULL AS VARCHAR(1))`),
			always(`, ''`),
			always(`, r.ROLE_COMMENT`),
			always(`FROM EXA_ALL_ROLES r`),
			always(`WHERE ` + like(`r.ROLE_NAME`, `@name`)),
			always(`ORDER BY 1`),
		},
		Fields: []dbmeta.Field{
			{Name: "name"},
			{Name: "superuser", Desc: "true for SYS alone. Exasol records no superuser flag, and SYS is the user the database is created with and holds every privilege"},
			{Name: "create_role", Desc: "always false: the right to create a role is a system privilege and is not on the principal"},
			{Name: "create_db", Desc: "always false: Exasol has one database and no right to create another"},
			{Name: "can_login", Desc: "true for a user and false for a role, which is what separates the two halves of this result"},
			{Name: "replication", Desc: "always false: Exasol has no replication right"},
			{Name: "bypass_rls", Desc: "always false: Exasol has no row level security"},
			{Name: "inherit", Desc: "always true: a granted role applies to its members"},
			{Name: "conn_limit", Desc: "always zero: Exasol sets no per principal connection limit. A consumer group limits resources rather than connections"},
			{Name: "valid_until", Desc: "always absent: EXA_ALL_USERS carries no expiry, and EXA_DBA_USERS, which does, is for an administrator alone"},
			{Name: "member_of", Desc: "always empty: RoleGrants reads membership"},
			{Name: "comment", Desc: "from COMMENT ON USER or COMMENT ON ROLE. SYS, PUBLIC and DBA carry a description the engine wrote"},
		},
		Params: nameOnly("principal"),
		Scan: func(rows *sql.Rows) (dbmeta.Role, error) {
			var v dbmeta.Role
			err := rows.Scan(empty(&v.Name), &v.Superuser, &v.CreateRole, &v.CreateDB,
				&v.CanLogin, &v.Replication, &v.BypassRLS, &v.Inherit, &v.ConnLimit,
				&v.ValidUntil, empty(&v.MemberOf), &v.Comment)
			return v, err
		},
	})

	// \drg. EXA_DBA_ROLE_PRIVS is every role grant in the database, and it
	// needs SELECT ANY DICTIONARY. The views an ordinary user can read list
	// only the grants that user holds, which would hide every other
	// member's grants from an administrator as well. So this reads the
	// whole list and a lesser principal is refused it. test/parity_test.go
	// records that.
	dbmeta.RoleGrants.Register(dbmeta.Exasol, &dbmeta.Binding[dbmeta.RoleGrant]{
		Stmt: dbmeta.Stmt{
			always(`SELECT g.GRANTEE AS "role"`),
			always(`, g.GRANTED_ROLE AS "member_of"`),
			always(`, CAST(NULL AS VARCHAR(1)) AS "grantor"`),
			always(`, g.ADMIN_OPTION AS "admin"`),
			always(`, TRUE AS "inherit"`),
			always(`, TRUE AS "set"`),
			always(`FROM EXA_DBA_ROLE_PRIVS g`),
			always(`WHERE ` + like(`g.GRANTEE`, `@name`)),
			always(`ORDER BY g.GRANTEE, g.GRANTED_ROLE`),
		},
		Fields: []dbmeta.Field{
			{Name: "role", Desc: "the grantee, which is a user or another role"},
			{Name: "member_of", Desc: "the role granted"},
			{Name: "grantor", Desc: "always absent: Exasol does not record who granted a role"},
			{Name: "admin", Desc: "from ADMIN_OPTION"},
			{Name: "inherit", Desc: "always true: a granted role applies to its members"},
			{Name: "set", Desc: "always true: every granted role is active in a session, and there is no SET ROLE"},
		},
		Params: nameOnly("grantee"),
		Scan: func(rows *sql.Rows) (dbmeta.RoleGrant, error) {
			var v dbmeta.RoleGrant
			err := rows.Scan(empty(&v.Role), empty(&v.MemberOf), &v.Grantor, &v.Admin, &v.Inherit, &v.Set)
			return v, err
		},
	})

	// \dp. One row per object, with the grants folded into a string the way
	// psql prints them. EXA_ALL_OBJ_PRIVS is the grants the current user
	// made, received or owns the object of, and every user can read it.
	dbmeta.Privileges.Register(dbmeta.Exasol, &dbmeta.Binding[dbmeta.Privilege]{
		Stmt: dbmeta.Stmt{
			always(`SELECT p.OBJECT_SCHEMA AS "schema"`),
			always(`, p.OBJECT_NAME AS "name"`),
			always(`, LOWER(p.OBJECT_TYPE) AS "type"`),
			always(`, GROUP_CONCAT(p.GRANTEE || '=' || p.PRIVILEGE` +
				` ORDER BY p.GRANTEE, p.PRIVILEGE SEPARATOR ', ') AS "access"`),
			always(`, '' AS "column_access"`),
			always(`, '' AS "policies"`),
			always(`FROM EXA_ALL_OBJ_PRIVS p`),
			always(`WHERE ` + like(`p.OBJECT_SCHEMA`, `@schema`)),
			always(`AND ` + like(`p.OBJECT_NAME`, `@name`)),
			always(`GROUP BY p.OBJECT_SCHEMA, p.OBJECT_NAME, p.OBJECT_TYPE`),
			always(`ORDER BY p.OBJECT_SCHEMA, p.OBJECT_NAME`),
		},
		Fields: []dbmeta.Field{
			{Name: "schema", Desc: "absent for an object that belongs to the database rather than to a schema, such as a schema itself"},
			{Name: "name"},
			{Name: "type", Desc: "the kind of object, lower cased, such as table, view, function, script or schema"},
			{Name: "access", Desc: "grantee=privilege, comma separated"},
			{Name: "column_access", Desc: "always empty: Exasol grants on a whole object and has no column grant"},
			{Name: "policies", Desc: "always empty: Exasol has no row level security"},
		},
		Params: []dbmeta.Param{
			{Name: "schema", Desc: "schema name pattern, empty for every schema", Default: ""},
			{Name: "name", Desc: "object name pattern, empty for every one", Default: ""},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Privilege, error) {
			var v dbmeta.Privilege
			err := rows.Scan(empty(&v.Schema), empty(&v.Name), empty(&v.Type), &v.Access, empty(&v.ColumnAccess), empty(&v.Policies))
			return v, err
		},
	})
}
