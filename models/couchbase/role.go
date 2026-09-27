package couchbase

import (
	"database/sql"

	"github.com/xo/dbmeta"
)

// anyRole is true when a user holds any of the named roles.
func anyRole(roles string) string {
	return "ANY r IN u.`roles` SATISFIES r.`role` IN [" + roles + "] END"
}

// The parts of a grant's bucket_name, which is a bucket, bucket:scope or
// bucket:scope:collection, and is missing for a grant on the whole cluster.
const (
	grantParts = "SPLIT(IFMISSING(a.bucket_name, ''), ':')"
	grantLevel = "ARRAY_LENGTH(" + grantParts + ")"
	grantName  = "CASE WHEN " + grantLevel + " = 3 THEN " + grantParts + "[1] || '.' || " + grantParts + "[2]" +
		" WHEN " + grantLevel + " = 2 THEN " + grantParts + "[1] ELSE '' END"
)

func registerRoles() {
	// \du. A Couchbase user is the role. A Couchbase role, such as select
	// on a bucket, is a privilege, and the privileges query returns those.
	// A group holds roles for its users and cannot log in, and SQL++ has no
	// catalog of groups, so a group appears only as what a user is a member
	// of.
	dbmeta.Roles.Register(dbmeta.Couchbase, &dbmeta.Binding[dbmeta.Role]{
		Stmt: dbmeta.Stmt{
			from76("SELECT u.id AS `name`"),
			from76(", " + anyRole("'admin'") + " AS `superuser`"),
			from76(", " + anyRole("'admin', 'security_admin', 'user_admin_local', 'user_admin_external'") +
				" AS `create_role`"),
			from76(", " + anyRole("'admin', 'cluster_admin'") + " AS `create_db`"),
			from76(", true AS `can_login`"),
			from76(", false AS `replication`"),
			from76(", false AS `bypass_rls`"),
			from76(", true AS `inherit`"),
			from76(", -1 AS `conn_limit`"),
			from76(", NULL AS `valid_until`"),
			from76(", CONCAT2(', ', IFMISSING(u.`groups`, [])) AS `member_of`"),
			from76(", IFMISSING(u.name, NULL) AS `comment`"),
			from76("FROM system:user_info u"),
			from76("WHERE " + like("u.id", "@name")),
			from76("ORDER BY u.id"),
		},
		Fields: []dbmeta.Field{
			{Name: "name", Desc: "the user"},
			{Name: "superuser", Desc: "whether the user holds the role admin"},
			{Name: "create_role", Desc: "whether the user holds a role that administers users"},
			{Name: "create_db", Desc: "whether the user holds a role that creates buckets"},
			{Name: "can_login", Desc: "always true: every user logs in, and a group does not appear here"},
			{Name: "replication", Desc: "always false: replication is an XDCR role rather than a user flag"},
			{Name: "bypass_rls", Desc: "always false: Couchbase has no row level security"},
			{Name: "inherit", Desc: "always true: a user always has the roles of its groups"},
			{Name: "conn_limit", Desc: "always -1: a user has no connection limit"},
			{Name: "valid_until", Desc: "always absent: a password does not expire here"},
			{Name: "member_of", Desc: "the groups the user is in, as one text"},
			{Name: "comment", Desc: "the user's full name, which is the one note Couchbase keeps on a user"},
		},
		Params: []dbmeta.Param{{Name: "name", Desc: "user name pattern, empty for every user", Default: ""}},
		Scan: func(rows *sql.Rows) (dbmeta.Role, error) {
			var v dbmeta.Role
			err := rows.Scan(&v.Name, &v.Superuser, &v.CreateRole, &v.CreateDB,
				&v.CanLogin, &v.Replication, &v.BypassRLS, &v.Inherit, &v.ConnLimit,
				&v.ValidUntil, &v.MemberOf, &v.Comment)
			return v, err
		},
	})

	// One row for each group a user is in.
	dbmeta.RoleGrants.Register(dbmeta.Couchbase, &dbmeta.Binding[dbmeta.RoleGrant]{
		Stmt: dbmeta.Stmt{
			from76("SELECT u.id AS `role`"),
			from76(", g AS `member_of`"),
			from76(", NULL AS `grantor`"),
			from76(", false AS `admin`"),
			from76(", true AS `inherit`"),
			from76(", true AS `set`"),
			from76("FROM system:user_info u"),
			from76("UNNEST IFMISSING(u.`groups`, []) AS g"),
			from76("WHERE " + like("u.id", "@name")),
			from76("ORDER BY u.id, g"),
		},
		Fields: []dbmeta.Field{
			{Name: "role", Desc: "the user"},
			{Name: "member_of", Desc: "the group"},
			{Name: "grantor", Desc: "always absent: Couchbase records no grantor"},
			{Name: "admin", Desc: "always false: a group has no admin option"},
			{Name: "inherit", Desc: "always true: a user always has the roles of its groups"},
			{Name: "set", Desc: "always true: there is no SET ROLE to withhold"},
		},
		Params: []dbmeta.Param{{Name: "name", Desc: "user name pattern, empty for every user", Default: ""}},
		Scan: func(rows *sql.Rows) (dbmeta.RoleGrant, error) {
			var v dbmeta.RoleGrant
			err := rows.Scan(&v.Role, &v.MemberOf, &v.Grantor, &v.Admin, &v.Inherit, &v.Set)
			return v, err
		},
	})

	// \dp. system:applicable_roles holds one row for each role a user holds,
	// with the object it is on as bucket_name: a bucket, bucket:scope, or
	// bucket:scope:collection, and nothing for a role on the whole cluster.
	// A row here is one object with every grant on it.
	dbmeta.Privileges.Register(dbmeta.Couchbase, &dbmeta.Binding[dbmeta.Privilege]{
		Stmt: dbmeta.Stmt{
			from76("SELECT " + grantParts + "[0] AS `schema`"),
			from76(", " + grantName + " AS `name`"),
			from76(", CASE WHEN a.bucket_name IS MISSING THEN 'cluster'" +
				" WHEN " + grantLevel + " = 3 THEN 'collection'" +
				" WHEN " + grantLevel + " = 2 THEN 'scope' ELSE 'bucket' END AS `type`"),
			from76(", CONCAT2(', ', ARRAY_SORT(ARRAY_AGG(a.grantee || '=' || a.`role`))) AS `access`"),
			from76(", '' AS `column_access`"),
			from76(", '' AS `policies`"),
			from76("FROM system:applicable_roles a"),
			from76("GROUP BY a.bucket_name"),
			from76("HAVING " + like(grantParts+"[0]", "@schema")),
			from76("AND " + like(grantName, "@name")),
			from76("ORDER BY 1, 2"),
		},
		Fields: []dbmeta.Field{
			{Name: "schema", Desc: "the bucket, and empty for a role on the whole cluster"},
			{Name: "name", Desc: "the scope, or the scope and the collection as scope.collection, and empty for a bucket"},
			{Name: "type", Desc: "cluster, bucket, scope or collection"},
			{Name: "access", Desc: "user=role for each role held on the object, as one text"},
			{Name: "column_access", Desc: "always empty: a role is on a collection and not on a field"},
			{Name: "policies", Desc: "always empty: Couchbase has no row level security"},
		},
		Params: []dbmeta.Param{
			{Name: "schema", Desc: "bucket name pattern, empty for every bucket", Default: ""},
			{Name: "name", Desc: "scope or scope.collection pattern, empty for every object", Default: ""},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Privilege, error) {
			var v dbmeta.Privilege
			err := rows.Scan(&v.Schema, &v.Name, &v.Type, &v.Access, &v.ColumnAccess, &v.Policies)
			return v, err
		},
	})

	// Who the connection is authenticated as. CURRENT_USERS returns each
	// user of the request as domain:name, and one connection has one.
	dbmeta.CurrentUser.Register(dbmeta.Couchbase, &dbmeta.Binding[dbmeta.User]{
		Stmt: dbmeta.Stmt{
			from76("SELECT REGEXP_REPLACE(CURRENT_USERS()[0], '^[^:]*:', '') AS `name`"),
			from76(", NULL AS `session`"),
		},
		Fields: []dbmeta.Field{
			{Name: "name", Desc: "the user, without the domain CURRENT_USERS writes before it"},
			{Name: "session", Desc: "always absent: a SQL++ request carries no session user apart from the user"},
		},
		Scan: func(rows *sql.Rows) (dbmeta.User, error) {
			var v dbmeta.User
			err := rows.Scan(&v.Name, &v.Session)
			return v, err
		},
	})
}
