package firebird

import (
	"database/sql"

	"github.com/xo/dbmeta"
)

// systemName filters out the catalog's own objects by name. A grant row
// names an object of any kind and there is no one table to join against to
// learn whether it is the server's, so the prefix is the test.
func systemName(col string) string {
	return `(` + col + ` NOT STARTING WITH 'RDB$'` +
		` AND ` + col + ` NOT STARTING WITH 'MON$'` +
		` AND ` + col + ` NOT STARTING WITH 'SEC$')`
}

// privilegeObject turns RDB$OBJECT_TYPE into the word for the kind of thing
// the grant is on.
const privilegeObject = `CASE p.RDB$OBJECT_TYPE` +
	` WHEN 0 THEN 'table' WHEN 1 THEN 'view' WHEN 2 THEN 'trigger'` +
	` WHEN 5 THEN 'procedure' WHEN 7 THEN 'exception' WHEN 8 THEN 'user'` +
	` WHEN 9 THEN 'domain' WHEN 11 THEN 'character set' WHEN 13 THEN 'role'` +
	` WHEN 14 THEN 'sequence' WHEN 15 THEN 'function' WHEN 16 THEN 'collation'` +
	` WHEN 17 THEN 'package' ELSE 'other' END`

func registerRoles() {
	registerRoleList()
	registerRoleGrants()
	registerPrivileges()
}

func registerRoleList() {
	// \du and \dg. Firebird splits the two halves of a PostgreSQL role
	// between two places, because a user belongs to the server and a role
	// belongs to the database. SEC$USERS is a view on the security database
	// and RDB$ROLES is an ordinary catalog table, so this is one statement
	// over both and can_login is what tells them apart.
	//
	// SEC$USERS answers for the caller alone unless the caller administers
	// the server. That is a real difference and test/parity_test.go records
	// it rather than the query hiding it.
	dbmeta.Roles.Register(dbmeta.Firebird, &dbmeta.Binding[dbmeta.Role]{
		Stmt: dbmeta.Stmt{
			always(`SELECT TRIM(TRAILING FROM u.SEC$USER_NAME) AS "name"`),
			always(`, COALESCE(u.SEC$ADMIN, FALSE) OR TRIM(TRAILING FROM u.SEC$USER_NAME) = 'SYSDBA' AS "superuser"`),
			always(`, COALESCE(u.SEC$ADMIN, FALSE) AS "create_role"`),
			always(`, FALSE AS "create_db"`),
			always(`, COALESCE(u.SEC$ACTIVE, TRUE) AS "can_login"`),
			always(`, FALSE AS "replication"`),
			always(`, FALSE AS "bypass_rls"`),
			always(`, TRUE AS "inherit"`),
			always(`, CAST(0 AS BIGINT) AS "conn_limit"`),
			always(`, CAST(NULL AS VARCHAR(1)) AS "valid_until"`),
			always(`, '' AS "member_of"`),
			always(`, u.SEC$DESCRIPTION AS "comment"`),
			always(`FROM SEC$USERS u`),
			always(`WHERE ` + like(`u.SEC$USER_NAME`, `@name`) + ``),
			always(`UNION ALL`),
			always(`SELECT TRIM(TRAILING FROM r.RDB$ROLE_NAME)`),
			always(`, FALSE, FALSE, FALSE, FALSE, FALSE, FALSE, TRUE`),
			always(`, CAST(0 AS BIGINT)`),
			always(`, CAST(NULL AS VARCHAR(1))`),
			always(`, ''`),
			always(`, r.RDB$DESCRIPTION`),
			always(`FROM RDB$ROLES r`),
			always(`WHERE ` + like(`r.RDB$ROLE_NAME`, `@name`) + ``),
			always(`ORDER BY 1`),
		},
		Fields: []dbmeta.Field{
			{Name: "name"},
			{Name: "superuser", Desc: "true for SYSDBA and for a user that administers the security database. A role is never a superuser"},
			{Name: "create_role", Desc: "from SEC$ADMIN, which is the right to manage users. Always false for a role"},
			{Name: "create_db", Desc: "always false: the right to create a database is a system privilege from 4.0 and is not recorded on the role in a form one statement can read"},
			{Name: "can_login", Desc: "true for a user and false for a role, which is what separates the two halves of this result. An inactive user reports false"},
			{Name: "replication", Desc: "always false: Firebird configures replication in a file rather than on a principal"},
			{Name: "bypass_rls", Desc: "always false: Firebird has no row level security"},
			{Name: "inherit", Desc: "always true: a granted role applies to its members"},
			{Name: "conn_limit", Desc: "always zero: Firebird sets no per principal connection limit"},
			{Name: "valid_until", Desc: "always absent: a Firebird principal does not expire"},
			{Name: "member_of", Desc: "always empty: RoleGrants reads membership"},
			{Name: "comment"},
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
}

func registerRoleGrants() {
	// \drg. Firebird records membership of a role in the same table as every
	// other grant, under the privilege letter M, and the role granted is in
	// the column that otherwise holds the object.
	dbmeta.RoleGrants.Register(dbmeta.Firebird, &dbmeta.Binding[dbmeta.RoleGrant]{
		Stmt: dbmeta.Stmt{
			always(`SELECT TRIM(TRAILING FROM p.RDB$USER) AS "role"`),
			always(`, TRIM(TRAILING FROM p.RDB$RELATION_NAME) AS "member_of"`),
			always(`, TRIM(TRAILING FROM p.RDB$GRANTOR) AS "grantor"`),
			always(`, COALESCE(p.RDB$GRANT_OPTION, 0) > 0 AS "admin"`),
			always(`, TRUE AS "inherit"`),
			always(`, TRUE AS "set"`),
			always(`FROM RDB$USER_PRIVILEGES p`),
			always(`WHERE p.RDB$PRIVILEGE = 'M'`),
			always(`AND ` + like(`p.RDB$USER`, `@name`) + ``),
			always(`ORDER BY p.RDB$USER, p.RDB$RELATION_NAME`),
		},
		Fields: []dbmeta.Field{
			{Name: "role", Desc: "the grantee, which is a user or another role"},
			{Name: "member_of", Desc: "the role granted, from RDB$RELATION_NAME, which holds the object of every grant and for a membership holds the role"},
			{Name: "grantor"},
			{Name: "admin", Desc: "from RDB$GRANT_OPTION, which is WITH ADMIN OPTION for a role"},
			{Name: "inherit", Desc: "always true: a granted role applies to its members"},
			{Name: "set", Desc: "always true: a member can SET ROLE to it"},
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
}

func registerPrivileges() {
	// \dp. One row per object, with the grants folded into a string the way
	// psql prints them. A membership is left out, because RoleGrants reads
	// those and an object is not a role.
	//
	// Firebird records a column level grant in the same table, with the
	// column named, so those are folded separately into column_access.
	dbmeta.Privileges.Register(dbmeta.Firebird, &dbmeta.Binding[dbmeta.Privilege]{
		Stmt: dbmeta.Stmt{
			always(`SELECT ` + noSchema),
			always(`, TRIM(TRAILING FROM p.RDB$RELATION_NAME) AS "name"`),
			always(`, ` + privilegeObject + ` AS "type"`),
			always(`, LIST(CASE WHEN p.RDB$FIELD_NAME IS NULL` +
				` THEN TRIM(TRAILING FROM p.RDB$USER) || '=' || TRIM(TRAILING FROM p.RDB$PRIVILEGE) END, ', ') AS "access"`),
			always(`, COALESCE(LIST(CASE WHEN p.RDB$FIELD_NAME IS NOT NULL` +
				` THEN TRIM(TRAILING FROM p.RDB$FIELD_NAME) || ':' || TRIM(TRAILING FROM p.RDB$USER) || '='` +
				` || TRIM(TRAILING FROM p.RDB$PRIVILEGE) END, ', '), '') AS "column_access"`),
			always(`, '' AS "policies"`),
			always(`FROM RDB$USER_PRIVILEGES p`),
			always(`WHERE p.RDB$PRIVILEGE <> 'M'`),
			always(`AND ` + systemName(`p.RDB$RELATION_NAME`)),
			always(`AND ` + like(`''`, `@schema`) + ``),
			always(`AND ` + like(`p.RDB$RELATION_NAME`, `@name`) + ``),
			always(`GROUP BY p.RDB$RELATION_NAME, p.RDB$OBJECT_TYPE`),
			always(`ORDER BY p.RDB$RELATION_NAME`),
		},
		Fields: []dbmeta.Field{
			{Name: "schema", Desc: schemaDesc},
			{Name: "name"},
			{Name: "type", Desc: "from RDB$OBJECT_TYPE. Firebird grants on more kinds than a table, so this can say sequence, package or exception"},
			{Name: "access", Desc: "grantee=privilege, comma separated. The privilege is Firebird's letter: S select, I insert, U update, D delete, R references, X execute. Absent where every grant on the object is a column grant"},
			{Name: "column_access", Desc: "column:grantee=privilege, comma separated, and empty where there are none"},
			{Name: "policies", Desc: "always empty: Firebird has no row level policy"},
		},
		Params: schemaAndName("object"),
		Scan: func(rows *sql.Rows) (dbmeta.Privilege, error) {
			var v dbmeta.Privilege
			err := rows.Scan(&v.Schema, &v.Name, &v.Type, &v.Access, &v.ColumnAccess, &v.Policies)
			return v, err
		},
	})
}
