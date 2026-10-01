package neo4j

import (
	"database/sql"

	"github.com/xo/dbmeta"
)

func registerRoles() {
	// \du. A Neo4j user is the role. A Neo4j role is granted privileges,
	// holds users and cannot log in, and SHOW ROLES cannot be joined with
	// SHOW USERS on any release, so a role appears here only as what a user
	// is a member of. The role admin is the one that administers the
	// server.
	dbmeta.Roles.Register(dbmeta.Neo4j, &dbmeta.Binding[dbmeta.Role]{
		Stmt: both("SHOW USERS YIELD user, roles, suspended" +
			" WHERE " + like("user", "@name") +
			" RETURN user AS `name`, 'admin' IN roles AS `superuser`, 'admin' IN roles AS `create_role`" +
			", 'admin' IN roles AS `create_db`, suspended IS NULL OR NOT suspended AS `can_login`" +
			", false AS `replication`, false AS `bypass_rls`, true AS `inherit`, -1 AS `conn_limit`" +
			", NULL AS `valid_until`, " + join("roles", ", ") + " AS `member_of`" +
			", NULL AS `comment`" +
			" ORDER BY `name`"),
		Fields: []dbmeta.Field{
			{Name: "name", Desc: "the user"},
			{Name: "superuser", Desc: "whether the user holds the role admin"},
			{Name: "create_role", Desc: "whether the user holds the role admin, which is the role that manages users and roles"},
			{Name: "create_db", Desc: "whether the user holds the role admin, which is the role that creates databases"},
			{Name: "can_login", Desc: "false for a suspended user, and true for any other"},
			{Name: "replication", Desc: "always false: Neo4j has no replication privilege for a user"},
			{Name: "bypass_rls", Desc: "always false: no user passes over a DENY"},
			{Name: "inherit", Desc: "always true: a user always has the privileges of its roles"},
			{Name: "conn_limit", Desc: "always -1: a user has no connection limit"},
			{Name: "valid_until", Desc: "always absent: a password does not expire"},
			{Name: "member_of", Desc: "the roles the user holds, joined by commas, which include PUBLIC"},
			{Name: "comment", Desc: "always absent: a user carries no comment"},
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

	// One row for each role a user holds. SHOW ROLES WITH USERS returns a
	// row for each member of each role, and a row with no member for a role
	// that has none.
	dbmeta.RoleGrants.Register(dbmeta.Neo4j, &dbmeta.Binding[dbmeta.RoleGrant]{
		Stmt: both("SHOW ROLES WITH USERS YIELD role, member" +
			" WHERE member IS NOT NULL AND " + like("member", "@name") +
			" RETURN member AS `role`, role AS `member_of`, NULL AS `grantor`, false AS `admin`" +
			", true AS `inherit`, true AS `set`" +
			" ORDER BY `role`, `member_of`"),
		Fields: []dbmeta.Field{
			{Name: "role", Desc: "the user"},
			{Name: "member_of", Desc: "the role, which is PUBLIC for every user"},
			{Name: "grantor", Desc: "always absent: Neo4j records no grantor"},
			{Name: "admin", Desc: "always false: a role has no admin option"},
			{Name: "inherit", Desc: "always true: a user always has the privileges of its roles"},
			{Name: "set", Desc: "always true: there is no SET ROLE to withhold"},
		},
		Params: []dbmeta.Param{{Name: "name", Desc: "user name pattern, empty for every user", Default: ""}},
		Scan: func(rows *sql.Rows) (dbmeta.RoleGrant, error) {
			var v dbmeta.RoleGrant
			err := rows.Scan(&v.Role, &v.MemberOf, &v.Grantor, &v.Admin, &v.Inherit, &v.Set)
			return v, err
		},
	})

	// \drds. A user's home database is the one setting Neo4j keeps for a
	// user, set with ALTER USER SET HOME DATABASE. A session that names no
	// database runs there, as a role's search_path decides where a name
	// resolves on PostgreSQL.
	dbmeta.RoleSettings.Register(dbmeta.Neo4j, &dbmeta.Binding[dbmeta.RoleSetting]{
		Stmt: both("SHOW USERS YIELD user, home" +
			" WHERE home IS NOT NULL AND " + like("user", "@name") +
			" RETURN user AS `role`, '' AS `database`, 'home=' + home AS `settings`" +
			" ORDER BY `role`"),
		Fields: []dbmeta.Field{
			{Name: "role", Desc: "the user"},
			{Name: "database", Desc: "always empty: the home database holds in every database"},
			{Name: "settings", Desc: "home=, then the home database of the user"},
		},
		Params: []dbmeta.Param{{Name: "name", Desc: "user name pattern, empty for every user", Default: ""}},
		Scan: func(rows *sql.Rows) (dbmeta.RoleSetting, error) {
			var v dbmeta.RoleSetting
			err := rows.Scan(&v.Role, &v.Database, &v.Settings)
			return v, err
		},
	})

	// \dp. SHOW PRIVILEGES holds one row for each privilege a role holds.
	// A row here is one object, which is a graph, a segment of it and a
	// resource, with every privilege on it.
	dbmeta.Privileges.Register(dbmeta.Neo4j, &dbmeta.Binding[dbmeta.Privilege]{
		Stmt: both("SHOW PRIVILEGES YIELD access, action, resource, graph, segment, role" +
			" ORDER BY role, action, access" +
			" WHERE " + like("graph", "@schema") + " AND " + like("segment", "@name") +
			" RETURN graph AS `schema`, segment AS `name`, resource AS `type`" +
			", " + join("collect(role + '=' + CASE WHEN access = 'DENIED' THEN 'DENIED ' ELSE '' END + action)", ", ") + " AS `access`" +
			", NULL AS `column_access`, NULL AS `policies`" +
			" ORDER BY `schema`, `name`, `type`"),
		Fields: []dbmeta.Field{
			{Name: "schema", Desc: "the graph, which is a database, * for every database, or HOME for the home database"},
			{Name: "name", Desc: "the segment, such as NODE(book), RELATIONSHIP(*), database or FUNCTION(*)"},
			{Name: "type", Desc: "the resource, such as graph, all_properties, property(isbn) or database"},
			{
				Name: "access",
				Desc: "role=action for each privilege on the object, joined by commas, and" +
					" role=DENIED action for a privilege that is denied",
			},
			{Name: "column_access", Desc: "always absent: a privilege on one property is a row of its own, with the property in type"},
			{Name: "policies", Desc: "always absent: Neo4j has no row level security policy"},
		},
		Params: []dbmeta.Param{
			{Name: "schema", Desc: "graph name pattern, empty for every graph", Default: ""},
			{Name: "name", Desc: "segment pattern, empty for every segment", Default: ""},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Privilege, error) {
			var v dbmeta.Privilege
			err := rows.Scan(&v.Schema, &v.Name, &v.Type, &v.Access, &v.ColumnAccess, &v.Policies)
			return v, err
		},
	})

	// Who the connection is authenticated as. A request of the Query API
	// carries its own credentials, so there is no session user apart from
	// the user.
	dbmeta.CurrentUser.Register(dbmeta.Neo4j, &dbmeta.Binding[dbmeta.User]{
		Stmt: both("SHOW CURRENT USER YIELD user RETURN user AS `name`, NULL AS `session`"),
		Fields: []dbmeta.Field{
			{Name: "name", Desc: "the user"},
			{Name: "session", Desc: "always absent: Neo4j does not separate a session user from the user"},
		},
		Scan: func(rows *sql.Rows) (dbmeta.User, error) {
			var v dbmeta.User
			err := rows.Scan(&v.Name, &v.Session)
			return v, err
		},
	})

	// \dconfig. SHOW SETTINGS holds every setting of the server, with the
	// values it accepts written as a sentence.
	dbmeta.Settings.Register(dbmeta.Neo4j, &dbmeta.Binding[dbmeta.Setting]{
		Stmt: both("SHOW SETTINGS YIELD name, value, isDynamic, validValues" +
			" WHERE " + like("name", "@name") +
			" RETURN name AS `name`, value AS `value`, validValues AS `type`" +
			", CASE WHEN isDynamic THEN 'dynamic' ELSE 'static' END AS `context`" +
			", NULL AS `access`, NULL AS `display`" +
			" ORDER BY `name`"),
		Fields: []dbmeta.Field{
			{Name: "name", Desc: "the setting, such as server.memory.heap.max_size"},
			{Name: "value", Desc: "the value, and absent where it is not set"},
			{Name: "type", Desc: "the values the setting accepts, as Neo4j writes them, such as A boolean."},
			{Name: "context", Desc: "dynamic for a setting that changes while the server runs, and static for one that needs a restart"},
			{Name: "access", Desc: "always absent: SHOW SETTINGS says nothing about who can change a setting"},
			{Name: "display", Desc: "always absent: SHOW SETTINGS shows a value in one form, which is value"},
		},
		Params: []dbmeta.Param{{Name: "name", Desc: "setting name pattern, empty for every setting", Default: ""}},
		Scan: func(rows *sql.Rows) (dbmeta.Setting, error) {
			var v dbmeta.Setting
			err := rows.Scan(&v.Name, &v.Value, &v.Type, &v.Context, &v.Access, &v.Display)
			return v, err
		},
	})
}
