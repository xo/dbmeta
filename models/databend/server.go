package databend

import (
	"database/sql"

	"github.com/xo/dbmeta"
)

func registerServer() {
	// \dconfig. level is where a value was set: DEFAULT, SESSION or GLOBAL.
	dbmeta.Settings.Register(dbmeta.Databend, &dbmeta.Binding[dbmeta.Setting]{
		Stmt: dbmeta.Stmt{
			always(`SELECT s.name AS "name"`),
			always(`, s.value AS "value"`),
			always(`, lower(s.type) AS "type"`),
			always(`, lower(s.level) AS "context"`),
			always(`, NULL AS "access"`),
			always(`, NULL AS "display"`),
			always(`FROM system.settings s`),
			always(`WHERE ` + like("s.name", "@name")),
			always(`ORDER BY 1`),
		},
		Fields: []dbmeta.Field{
			{Name: "name"}, {Name: "value"},
			{Name: "type", Desc: "the type of the value, such as uint64 or string"},
			{Name: "context", Desc: "where the value was set: default, session or global"},
			{Name: "access", Desc: "always absent: a setting has no grant of its own"},
			{Name: "display", Desc: "always absent: Databend shows a value in one form, which is value"},
		},
		Params: []dbmeta.Param{
			{Name: "name", Desc: "setting name pattern, empty for every setting", Default: ""},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Setting, error) {
			var v dbmeta.Setting
			err := rows.Scan(&v.Name, &v.Value, &v.Type, &v.Context, &v.Access, &v.Display)
			return v, err
		},
	})

	// \dA. A table engine is how a table is stored, which is what an access
	// method is, as the mysql model reads ENGINES.
	dbmeta.AccessMethods.Register(dbmeta.Databend, &dbmeta.Binding[dbmeta.AccessMethod]{
		Stmt: dbmeta.Stmt{
			always(`SELECT e."Engine" AS "name"`),
			always(`, 'table' AS "type"`),
			always(`, NULL AS "handler"`),
			always(`, e."Comment" AS "comment"`),
			always(`FROM system.engines e`),
			always(`WHERE ` + like(`e."Engine"`, "@name")),
			always(`ORDER BY 1`),
		},
		Fields: []dbmeta.Field{
			{Name: "name", Desc: "the engine, such as FUSE"},
			{Name: "type", Desc: "always table: an engine stores a table"},
			{Name: "handler", Desc: "always absent: an engine is built into the server"},
			{Name: "comment"},
		},
		Params: []dbmeta.Param{
			{Name: "name", Desc: "engine name pattern, empty for every engine", Default: ""},
		},
		Scan: func(rows *sql.Rows) (dbmeta.AccessMethod, error) {
			var v dbmeta.AccessMethod
			err := rows.Scan(&v.Name, &v.Type, &v.Handler, &v.Comment)
			return v, err
		},
	})

	// \du. A user can log in and a role cannot. account_admin is the role
	// that may do anything, so a user that holds it is a superuser.
	dbmeta.Roles.Register(dbmeta.Databend, &dbmeta.Binding[dbmeta.Role]{
		Stmt: dbmeta.Stmt{
			always(`SELECT u.name AS "name"`),
			always(`, ` + holdsAdmin("u.roles") + ` AS "superuser"`),
			always(`, false AS "create_role"`),
			always(`, false AS "create_db"`),
			always(`, NOT u.disabled AS "can_login"`),
			always(`, false AS "replication"`),
			always(`, false AS "bypass_rls"`),
			always(`, true AS "inherit"`),
			always(`, -1 AS "conn_limit"`),
			always(`, NULL AS "valid_until"`),
			always(`, u.roles AS "member_of"`),
			always(`, NULL AS "comment"`),
			always(`FROM system.users u`),
			always(`WHERE ` + like("u.name", "@name")),
			always(`UNION ALL`),
			always(`SELECT r.name, r.name = 'account_admin' OR ` + holdsAdmin("r.inherited_roles_name")),
			always(`, false, false, false, false, false, true, -1, NULL, r.inherited_roles_name, NULL`),
			always(`FROM system.roles r`),
			always(`WHERE ` + like("r.name", "@name")),
			// Databend orders a union by a column name and not by a position.
			always(`ORDER BY "name"`),
		},
		Fields: []dbmeta.Field{
			{Name: "name"},
			{Name: "superuser", Desc: "true for account_admin and for whoever holds it, which may do anything"},
			{Name: "create_role", Desc: "always false: it is a grant rather than a flag"},
			{Name: "create_db", Desc: "always false: it is a grant rather than a flag"},
			{Name: "can_login", Desc: "true for a user that is not disabled, and false for a role, which cannot log in"},
			{Name: "replication", Desc: "always false: Databend has no replication role"},
			{Name: "bypass_rls", Desc: "always false: a row policy is bypassed by a grant, not a flag"},
			{Name: "inherit", Desc: "always true: a granted role is always inherited"},
			{Name: "conn_limit", Desc: "always -1: Databend has no limit per user"},
			{Name: "valid_until", Desc: "always absent: a user has no expiry"},
			{Name: "member_of", Desc: "the roles granted to a user, or the roles a role inherits, joined by commas"},
			{Name: "comment", Desc: "always absent: system.users records no comment"},
		},
		Params: []dbmeta.Param{
			{Name: "name", Desc: "user or role name pattern, empty for every one", Default: ""},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Role, error) {
			var v dbmeta.Role
			err := rows.Scan(&v.Name, &v.Superuser, &v.CreateRole, &v.CreateDB,
				&v.CanLogin, &v.Replication, &v.BypassRLS, &v.Inherit, &v.ConnLimit,
				&v.ValidUntil, &v.MemberOf, &v.Comment)
			return v, err
		},
	})

	// A role granted to a user, or to another role. Each is a list joined
	// by commas on its row, so the list is split.
	dbmeta.RoleGrants.Register(dbmeta.Databend, &dbmeta.Binding[dbmeta.RoleGrant]{
		Stmt: dbmeta.Stmt{
			always(`SELECT g.role AS "role"`),
			always(`, g.member_of AS "member_of"`),
			always(`, NULL AS "grantor"`),
			always(`, false AS "admin"`),
			always(`, true AS "inherit"`),
			always(`, true AS "set"`),
			always(`FROM (`),
			always(`  SELECT u.name AS role, trim(unnest(split(u.roles, ','))) AS member_of FROM system.users u`),
			always(`  UNION ALL`),
			always(`  SELECT r.name, trim(unnest(split(r.inherited_roles_name, ','))) FROM system.roles r`),
			always(`) g`),
			always(`WHERE g.member_of <> ''`),
			always(`AND ` + like("g.member_of", "@name")),
			always(`ORDER BY 1, 2`),
		},
		Fields: []dbmeta.Field{
			{Name: "role", Desc: "who was granted it, a user or another role"},
			{Name: "member_of", Desc: "the role that was granted"},
			{Name: "grantor", Desc: "always absent: Databend records no grantor"},
			{Name: "admin", Desc: "always false: Databend has no WITH ADMIN OPTION"},
			{Name: "inherit", Desc: "always true: a granted role is always inherited"},
			{Name: "set", Desc: "always true: a session can take any role it was granted"},
		},
		Params: []dbmeta.Param{
			{Name: "name", Desc: "granted role name pattern, empty for every one", Default: ""},
		},
		Scan: func(rows *sql.Rows) (dbmeta.RoleGrant, error) {
			var v dbmeta.RoleGrant
			err := rows.Scan(&v.Role, &v.MemberOf, &v.Grantor, &v.Admin, &v.Inherit, &v.Set)
			return v, err
		},
	})

	// current_user() is the user and its host, as 'root'@'%'.
	dbmeta.CurrentUser.Register(dbmeta.Databend, &dbmeta.Binding[dbmeta.User]{
		Stmt: dbmeta.Stmt{
			always(`SELECT current_user() AS "name"`),
			always(`, NULL AS "session"`),
		},
		Fields: []dbmeta.Field{
			{Name: "name", Desc: "the user and the host it matched, as Databend writes them: 'root'@'%'"},
			{Name: "session", Desc: "always absent: Databend has one user per session"},
		},
		Scan: func(rows *sql.Rows) (dbmeta.User, error) {
			var v dbmeta.User
			err := rows.Scan(&v.Name, &v.Session)
			return v, err
		},
	})
}

// holdsAdmin is the condition that the list of roles in col holds
// account_admin.
func holdsAdmin(col string) string {
	return dbmeta.InList(`replace(`+col+`, ' ', '')`, `'account_admin'`)
}
