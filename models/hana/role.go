package hana

import (
	"database/sql"

	"github.com/xo/dbmeta"
)

func registerRoles() {
	// \du and \dg. HANA keeps users and roles in two views and both are
	// principals, so this is one statement over both and can_login is what
	// separates them.
	dbmeta.Roles.Register(dbmeta.HANA, &dbmeta.Binding[dbmeta.Role]{
		Stmt: dbmeta.Stmt{
			always(`SELECT u.USER_NAME AS "name"`),
			// HANA has no superuser flag. SYSTEM is the one the system is
			// created with and it is the nearest thing, so this names it
			// rather than reporting a right nobody records.
			always(`, CASE WHEN u.USER_NAME = 'SYSTEM' THEN TRUE ELSE FALSE END AS "superuser"`),
			always(`, FALSE AS "create_role"`),
			always(`, FALSE AS "create_db"`),
			always(`, ` + yes(`u.IS_PASSWORD_ENABLED`) + ` AS "can_login"`),
			always(`, FALSE AS "replication"`),
			always(`, FALSE AS "bypass_rls"`),
			always(`, TRUE AS "inherit"`),
			always(`, CAST(0 AS BIGINT) AS "conn_limit"`),
			always(`, CAST(u.VALID_UNTIL AS NVARCHAR(40)) AS "valid_until"`),
			always(`, '' AS "member_of"`),
			always(`, u.COMMENTS AS "comment"`),
			always(`FROM SYS.USERS u`),
			always(`WHERE ` + like(`u.USER_NAME`, `@name`)),
			always(`UNION ALL`),
			always(`SELECT r.ROLE_NAME`),
			always(`, FALSE, FALSE, FALSE, FALSE, FALSE, FALSE, TRUE`),
			always(`, CAST(0 AS BIGINT)`),
			always(`, CAST(NULL AS NVARCHAR(40))`),
			always(`, ''`),
			always(`, r.COMMENTS`),
			always(`FROM SYS.ROLES r`),
			always(`WHERE ` + like(`r.ROLE_NAME`, `@name`)),
			always(`ORDER BY 1`),
		},
		Fields: []dbmeta.Field{
			{Name: "name"},
			{Name: "superuser", Desc: "true for SYSTEM alone. HANA records no superuser flag, and SYSTEM is the user the database is created with"},
			{Name: "create_role", Desc: "always false: the right to create a role is a system privilege and is in Privileges rather than on the principal"},
			{Name: "create_db", Desc: "always false, for the same reason"},
			{Name: "can_login", Desc: "true for a user with a password and false for a role, which is what separates the two halves of this result"},
			{Name: "replication", Desc: "always false: HANA configures replication outside the catalog"},
			{Name: "bypass_rls", Desc: "always false: HANA restricts rows with an analytic privilege rather than with a per role bypass"},
			{Name: "inherit", Desc: "always true: a granted role applies to its members"},
			{Name: "conn_limit", Desc: "always zero: HANA sets no per principal connection limit"},
			{Name: "valid_until", Desc: "from VALID_UNTIL, and absent for a role and for a user that does not expire"},
			{Name: "member_of", Desc: "always empty: RoleGrants reads membership"},
			{Name: "comment", Desc: "from COMMENTS, which HANA records on both a user and a role"},
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

	// \drg.
	dbmeta.RoleGrants.Register(dbmeta.HANA, &dbmeta.Binding[dbmeta.RoleGrant]{
		Stmt: dbmeta.Stmt{
			always(`SELECT g.GRANTEE AS "role"`),
			always(`, g.ROLE_NAME AS "member_of"`),
			always(`, g.GRANTOR AS "grantor"`),
			always(`, ` + yes(`g.IS_GRANTABLE`) + ` AS "admin"`),
			always(`, TRUE AS "inherit"`),
			always(`, TRUE AS "set"`),
			always(`FROM SYS.GRANTED_ROLES g`),
			always(`WHERE ` + like(`g.GRANTEE`, `@name`)),
			always(`ORDER BY g.GRANTEE, g.ROLE_NAME`),
		},
		Fields: []dbmeta.Field{
			{Name: "role", Desc: "the grantee, which is a user or another role"},
			{Name: "member_of", Desc: "the role granted"},
			{Name: "grantor"},
			{Name: "admin", Desc: "from IS_GRANTABLE"},
			{Name: "inherit", Desc: "always true: a granted role applies to its members"},
			{Name: "set", Desc: "always true: HANA activates a granted role for the session"},
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

	// \dp. One row per object, with the grants folded into a string the way
	// psql prints them.
	dbmeta.Privileges.Register(dbmeta.HANA, &dbmeta.Binding[dbmeta.Privilege]{
		Stmt: dbmeta.Stmt{
			always(`SELECT p.SCHEMA_NAME AS "schema"`),
			always(`, p.OBJECT_NAME AS "name"`),
			always(`, LOWER(p.OBJECT_TYPE) AS "type"`),
			always(`, STRING_AGG(CASE WHEN p.COLUMN_NAME IS NULL` +
				` THEN p.GRANTEE || '=' || p.PRIVILEGE END, ', '` +
				` ORDER BY p.GRANTEE, p.PRIVILEGE) AS "access"`),
			always(`, COALESCE(STRING_AGG(CASE WHEN p.COLUMN_NAME IS NOT NULL` +
				` THEN p.COLUMN_NAME || ':' || p.GRANTEE || '=' || p.PRIVILEGE END, ', '` +
				` ORDER BY p.COLUMN_NAME, p.GRANTEE), '') AS "column_access"`),
			always(`, '' AS "policies"`),
			always(`FROM SYS.GRANTED_PRIVILEGES p`),
			// A system privilege is granted on nothing, so it has no object
			// and no schema. Those rows are a principal's rights rather
			// than an object's grants, which is what this kind reports.
			always(`WHERE p.OBJECT_NAME IS NOT NULL`),
			always(`AND ` + notSystem(`p.SCHEMA_NAME`)),
			always(`AND ` + like(`p.SCHEMA_NAME`, `@schema`)),
			always(`AND ` + like(`p.OBJECT_NAME`, `@name`)),
			always(`GROUP BY p.SCHEMA_NAME, p.OBJECT_NAME, p.OBJECT_TYPE`),
			always(`ORDER BY p.SCHEMA_NAME, p.OBJECT_NAME`),
		},
		Fields: []dbmeta.Field{
			{Name: "schema"}, {Name: "name"},
			{Name: "type", Desc: "the kind of object, lower cased. HANA grants on more kinds than a table, so this can say sequence, library or remote source"},
			{Name: "access", Desc: "grantee=privilege, comma separated. Absent where every grant on the object names a column"},
			{Name: "column_access", Desc: "column:grantee=privilege, comma separated, and empty where there are none"},
			{Name: "policies", Desc: "always empty: HANA restricts rows with an analytic privilege, which is an object of its own rather than a policy on the table"},
		},
		Params: schemaAndName("object"),
		Scan: func(rows *sql.Rows) (dbmeta.Privilege, error) {
			var v dbmeta.Privilege
			err := rows.Scan(&v.Schema, &v.Name, &v.Type, &v.Access, &v.ColumnAccess, &v.Policies)
			return v, err
		},
	})
}
