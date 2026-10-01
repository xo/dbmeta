package surrealdb

import (
	"database/sql"

	"github.com/xo/dbmeta"
)

// users is every system user that can reach the database the connection is
// in: those defined on the root, on its namespace and on the database, each
// with its level. INFO writes a role in lower case on the root and in upper
// case below it, so each role is in upper case here. Only the root user reads
// INFO FOR ROOT, and a user defined on a database reads neither of the first
// two.
const users = "array::concat(" +
	"array::map((INFO FOR ROOT STRUCTURE).users, |$u| {name: $u.name, level: 'root', comment: $u.comment," +
	" roles: array::map($u.roles, |$r| string::uppercase($r))})" +
	", array::map((INFO FOR NS STRUCTURE).users, |$u| {name: $u.name, level: 'namespace', comment: $u.comment," +
	" roles: array::map($u.roles, |$r| string::uppercase($r))})" +
	", array::map((INFO FOR DB STRUCTURE).users, |$u| {name: $u.name, level: 'database', comment: $u.comment," +
	" roles: array::map($u.roles, |$r| string::uppercase($r))}))"

func registerRoles() {
	// \du. A system user is the role. SurrealDB has three roles, OWNER,
	// EDITOR and VIEWER, which a user holds at the level it is defined at,
	// and RoleGrants returns them. A record user signs in through a DEFINE
	// ACCESS, is a record of a table and not a role, and is not here.
	//
	// 3.1 refuses an ORDER BY on a field that the SELECT does not return, so
	// the users are put in order by name and level before the SELECT reads
	// them.
	dbmeta.Roles.Register(dbmeta.SurrealDB, &dbmeta.Binding[dbmeta.Role]{
		Stmt: from3("SELECT true AS bypass_rls, true AS can_login, comment, -1 AS conn_limit" +
			", level IN ['root', 'namespace'] AND (roles CONTAINS 'OWNER' OR roles CONTAINS 'EDITOR') AS create_db" +
			", roles CONTAINS 'OWNER' AS create_role, true AS inherit" +
			", array::join(roles, ', ') AS member_of, name, false AS replication" +
			", level = 'root' AND roles CONTAINS 'OWNER' AS superuser, NULL AS valid_until" +
			" FROM (SELECT name, level, comment, roles FROM " + users + " ORDER BY name, level)" +
			" WHERE " + like("name", "@name")),
		Fields: []dbmeta.Field{
			{Name: "bypass_rls", Desc: "always true: the PERMISSIONS of a table hold a record user and never a system user"},
			{Name: "can_login", Desc: "always true: every system user signs in"},
			{Name: "comment"},
			{Name: "conn_limit", Desc: "always -1: a user has no connection limit"},
			{Name: "create_db", Desc: "whether the user is an OWNER or an EDITOR on the root or a namespace, which can define a namespace or a database"},
			{Name: "create_role", Desc: "whether the user is an OWNER, which can define a user at its level"},
			{Name: "inherit", Desc: "always true: a user holds what its roles allow"},
			{Name: "member_of", Desc: "the roles the user holds, as one text"},
			{
				Name: "name",
				Desc: "the user. A user of one name can be defined on the root, on the" +
					" namespace and on the database, and is then three rows",
			},
			{Name: "replication", Desc: "always false: SurrealDB has no replication role"},
			{Name: "superuser", Desc: "whether the user is an OWNER on the root, which can do everything on the server"},
			{Name: "valid_until", Desc: "always absent: a password does not expire, and a token and a session have a duration"},
		},
		Params: []dbmeta.Param{{Name: "name", Desc: "user name pattern, empty for every user", Default: ""}},
		Scan: func(rows *sql.Rows) (dbmeta.Role, error) {
			var v dbmeta.Role
			err := rows.Scan(&v.BypassRLS, &v.CanLogin, &v.Comment, &v.ConnLimit, &v.CreateDB,
				&v.CreateRole, &v.Inherit, &v.MemberOf, &v.Name, &v.Replication, &v.Superuser,
				&v.ValidUntil)
			return v, err
		},
	})

	// One row for each role a user holds.
	dbmeta.RoleGrants.Register(dbmeta.SurrealDB, &dbmeta.Binding[dbmeta.RoleGrant]{
		Stmt: from3("SELECT false AS admin, NULL AS grantor, true AS inherit, member_of, role, true AS set" +
			" FROM array::flatten(array::map(" + users + ", |$x| array::map($x.roles, |$r|" +
			" {role: $x.name, member_of: $r, level: $x.level})))" +
			" WHERE " + like("role", "@name") +
			" ORDER BY role, member_of"),
		Fields: []dbmeta.Field{
			{Name: "admin", Desc: "always false: a role has no admin option"},
			{Name: "grantor", Desc: "always absent: SurrealDB records no grantor"},
			{Name: "inherit", Desc: "always true: a user holds what its roles allow"},
			{Name: "member_of", Desc: "the role: OWNER, EDITOR or VIEWER"},
			{Name: "role", Desc: "the user"},
			{Name: "set", Desc: "always true: there is no SET ROLE to withhold"},
		},
		Params: []dbmeta.Param{{Name: "name", Desc: "user name pattern, empty for every user", Default: ""}},
		Scan: func(rows *sql.Rows) (dbmeta.RoleGrant, error) {
			var v dbmeta.RoleGrant
			err := rows.Scan(&v.Admin, &v.Grantor, &v.Inherit, &v.MemberOf, &v.Role, &v.Set)
			return v, err
		},
	})

	// \dp. The PERMISSIONS of a table say what a record user can do to its
	// records, and those of a field what it can do to the field. A system
	// user is held to its role and not to these. Each is FULL, NONE, or
	// WHERE and an expression that a record must meet.
	dbmeta.Privileges.Register(dbmeta.SurrealDB, &dbmeta.Binding[dbmeta.Privilege]{
		Stmt: from3("SELECT access, column_access, name, NULL AS policies, " + currentDB + " AS schema, type" +
			" FROM array::map(" + tables + ", |$x| {name: $x.name, type: " + typeOf + "," +
			" access: " + tableAccess + "," +
			" column_access: array::join(array::map((INFO FOR TABLE $x.name STRUCTURE).fields, |$f| " +
			fieldAccess + "), '\\n')})" +
			" WHERE " + like(currentDB, "@schema") +
			" AND " + like("name", "@name") +
			" ORDER BY name"),
		Fields: []dbmeta.Field{
			{Name: "access", Desc: "the PERMISSIONS of the table, as select=, create=, update= and delete= joined by commas"},
			{
				Name: "column_access",
				Desc: "the PERMISSIONS of each field, as field: select=, create= and update=," +
					" one line for each field, and empty for a table with no field",
			},
			{Name: "name", Desc: "the table"},
			{Name: "policies", Desc: "always absent: a WHERE in access is the rule a record must meet"},
			{Name: "schema", Desc: "the database"},
			{Name: "type", Desc: "table, view or relation, as Tables returns it"},
		},
		Params: []dbmeta.Param{
			{Name: "schema", Desc: "database name pattern, empty for the database of the connection", Default: ""},
			{Name: "name", Desc: "table name pattern, empty for every table", Default: ""},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Privilege, error) {
			var v dbmeta.Privilege
			err := rows.Scan(&v.Access, &v.ColumnAccess, &v.Name, &v.Policies, &v.Schema, &v.Type)
			return v, err
		},
	})
}

// tableAccess is the PERMISSIONS of the table in $x, as one text.
var tableAccess = "'select=' + " + permission("$x.permissions.select") +
	" + ', create=' + " + permission("$x.permissions.create") +
	" + ', update=' + " + permission("$x.permissions.update") +
	" + ', delete=' + " + permission("$x.permissions.delete")

// fieldAccess is the PERMISSIONS of the field in $f, as one text.
var fieldAccess = "$f.name + ': select=' + " + permission("$f.permissions.select") +
	" + ', create=' + " + permission("$f.permissions.create") +
	" + ', update=' + " + permission("$f.permissions.update")
