package ydb

import (
	"database/sql"

	"github.com/xo/dbmeta"
)

// memberOf is a relation of one row per user or group that belongs to a
// group, which holds it as sid and its groups, joined by commas, as groups.
const memberOf = "(SELECT MemberSid AS sid" +
	", String::JoinFromList(ListSort(AGGREGATE_LIST(GroupSid)), ', ') AS groups" +
	" FROM `.sys/auth_group_members` GROUP BY MemberSid)"

// roleColumns is the select list of one side of the roles union. login is
// the expression for can_login, and from is the relation the side reads.
//
// The COALESCE is over an aggregate that matched no rows, because a user
// in no group and a user whose list of groups is empty are one answer.
func roleColumns(login, from string) []dbmeta.Choice {
	return []dbmeta.Choice{
		always("SELECT r.Sid AS name"),
		always(", false AS superuser"),
		always(", false AS create_role"),
		always(", false AS create_db"),
		always(", " + login + " AS can_login"),
		always(", false AS replication"),
		always(", false AS bypass_rls"),
		always(", true AS inherit"),
		always(", -1 AS conn_limit"),
		always(", CAST(NULL AS Utf8) AS valid_until"),
		always(", COALESCE(m.groups, '') AS member_of"),
		always(", CAST(NULL AS Utf8) AS comment"),
		always("FROM " + from + " AS r"),
		always("LEFT JOIN " + memberOf + " AS m ON m.sid = r.Sid"),
	}
}

func registerRoles() {
	// \du. YDB keeps a user and a group in two views, and PostgreSQL keeps
	// both in pg_roles, so the two are joined. A user can log in and a
	// group cannot.
	roles := dbmeta.Stmt{
		always("SELECT u.name AS `name`"),
		always(", u.superuser AS `superuser`"),
		always(", u.create_role AS `create_role`"),
		always(", u.create_db AS `create_db`"),
		always(", u.can_login AS `can_login`"),
		always(", u.replication AS `replication`"),
		always(", u.bypass_rls AS `bypass_rls`"),
		always(", u.inherit AS `inherit`"),
		always(", u.conn_limit AS `conn_limit`"),
		always(", u.valid_until AS `valid_until`"),
		always(", u.member_of AS `member_of`"),
		always(", u.comment AS `comment`"),
		always("FROM ("),
	}
	roles = append(roles, roleColumns("r.IsEnabled", "`.sys/auth_users`")...)
	roles = append(roles, always("UNION ALL"))
	roles = append(roles, roleColumns("false", "`.sys/auth_groups`")...)
	roles = append(roles,
		always(") AS u"),
		always("WHERE (@name = '' OR u.name LIKE @name)"),
		always("ORDER BY `name`"),
	)
	dbmeta.Roles.Register(dbmeta.YDB, &dbmeta.Binding[dbmeta.Role]{
		Stmt: roles,
		Fields: []dbmeta.Field{
			{Name: "name", Desc: "the sid of a user or a group"},
			{
				Name: "superuser",
				Desc: "always false: YDB has no superuser flag. An administrator is a" +
					" member of a group that holds ydb.generic.full or a sid the" +
					" server configuration names, and RoleGrants and Privileges read" +
					" the first",
			},
			{Name: "create_role", Desc: "always false: it is a permission rather than a flag"},
			{Name: "create_db", Desc: "always false, for the same reason"},
			{
				Name: "can_login",
				Desc: "for a user, whether it is enabled, which ALTER USER ... NOLOGIN" +
					" turns off. Always false for a group, which cannot log in",
			},
			{Name: "replication", Desc: "always false: YDB has no replication flag"},
			{Name: "bypass_rls", Desc: "always false: YDB has no row level security"},
			{Name: "inherit", Desc: "always true: a member always has the permissions of its group"},
			{Name: "conn_limit", Desc: "always -1: YDB sets no limit on the connections of a user"},
			{Name: "valid_until", Desc: "always absent: a YDB user does not expire"},
			{Name: "member_of", Desc: "the groups the user or group belongs to directly"},
			{Name: "comment", Desc: "always absent: YDB has no comment"},
		},
		Params: []dbmeta.Param{
			{Name: "name", Desc: "user or group name pattern, empty for every one", Default: ""},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Role, error) {
			var v dbmeta.Role
			err := rows.Scan(&v.Name, &v.Superuser, &v.CreateRole, &v.CreateDB,
				&v.CanLogin, &v.Replication, &v.BypassRLS, &v.Inherit, &v.ConnLimit,
				&v.ValidUntil, &v.MemberOf, &v.Comment)
			return v, err
		},
	})

	// One row per user or group that belongs to a group.
	dbmeta.RoleGrants.Register(dbmeta.YDB, &dbmeta.Binding[dbmeta.RoleGrant]{
		Stmt: dbmeta.Stmt{
			always("SELECT g.MemberSid AS `role`"),
			always(", g.GroupSid AS `member_of`"),
			always(", CAST(NULL AS Utf8) AS `grantor`"),
			always(", false AS `admin`"),
			always(", true AS `inherit`"),
			always(", false AS `set`"),
			always("FROM `.sys/auth_group_members` AS g"),
			always("WHERE (@name = '' OR g.GroupSid LIKE @name)"),
			always("ORDER BY `role`, `member_of`"),
		},
		Fields: []dbmeta.Field{
			{Name: "role", Desc: "the user or group that belongs to the group"},
			{Name: "member_of", Desc: "the group"},
			{Name: "grantor", Desc: "always absent: YDB records no grantor"},
			{Name: "admin", Desc: "always false: YDB has no admin option on a group"},
			{Name: "inherit", Desc: "always true: a member always has the permissions of its group"},
			{Name: "set", Desc: "always false: YDB has no SET ROLE"},
		},
		Params: []dbmeta.Param{
			{Name: "name", Desc: "group name pattern, empty for every group", Default: ""},
		},
		Scan: func(rows *sql.Rows) (dbmeta.RoleGrant, error) {
			var v dbmeta.RoleGrant
			err := rows.Scan(&v.Role, &v.MemberOf, &v.Grantor, &v.Admin,
				&v.Inherit, &v.Set)
			return v, err
		},
	})

	// \dp. One row per object, with its explicit grants folded into one
	// list. A grant on a directory reaches everything in it, so a
	// directory is listed as well as a table.
	dbmeta.Privileges.Register(dbmeta.YDB, &dbmeta.Binding[dbmeta.Privilege]{
		Stmt: dbmeta.Stmt{
			always("SELECT x.schema AS `schema`"),
			always(", x.name AS `name`"),
			always(", x.type AS `type`"),
			always(", a.access AS `access`"),
			always(", CAST(NULL AS Utf8) AS `column_access`"),
			always(", CAST(NULL AS Utf8) AS `policies`"),
			always("FROM (SELECT o.path AS path"),
			always(", " + schemaOf("o.parts") + " AS schema"),
			always(", ListLast(o.parts) AS name"),
			always(", " + topOf("o.parts") + " AS top"),
			always(", CASE WHEN p.path IS NOT NULL THEN 'table'" +
				" WHEN dir.path IS NOT NULL THEN 'directory' ELSE 'object' END AS type"),
			always("FROM (SELECT Path AS path, String::SplitToList(Path, '/') AS parts" +
				" FROM `.sys/auth_owners`) AS o"),
			always("CROSS JOIN " + root),
			always("LEFT JOIN " + tablePaths + " AS p ON p.path = o.path"),
			always("LEFT JOIN " + directories + " AS dir ON dir.path = o.path"),
			always("WHERE ListLength(o.parts) > d.depth) AS x"),
			always("LEFT JOIN " + acl + " AS a ON a.path = x.path"),
			always("WHERE " + notSystem("x.top")),
			always("AND (@schema = '' OR x.schema LIKE @schema)"),
			always("AND (@name = '' OR x.name LIKE @name)"),
			always("ORDER BY `schema`, `name`"),
		},
		Fields: []dbmeta.Field{
			{Name: "schema", Desc: "the directory relative to the database, and empty at its root"},
			{Name: "name"},
			{
				Name: "type",
				Desc: "table or directory, and object for a path no" +
					" view names the kind of, such as a view, a topic or an empty directory",
			},
			{
				Name: "access",
				Desc: "the explicit grants as sid=permission, and absent when the object" +
					" has none and takes only what its directories grant",
			},
			{Name: "column_access", Desc: "always absent: YDB grants on a path and never on a column"},
			{Name: "policies", Desc: "always absent: YDB has no row level security"},
		},
		Params: []dbmeta.Param{
			{Name: "schema", Desc: "directory path pattern, empty for every directory", Default: ""},
			{Name: "name", Desc: "object name pattern, empty for every object", Default: ""},
			{Name: "with_system", Desc: "include the directories YDB keeps for itself", Default: false},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Privilege, error) {
			var v dbmeta.Privilege
			err := rows.Scan(&v.Schema, &v.Name, &v.Type, &v.Access,
				&v.ColumnAccess, &v.Policies)
			return v, err
		},
	})
}
