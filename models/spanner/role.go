package spanner

import (
	"database/sql"
	"strings"

	"github.com/xo/dbmeta"
)

// Database roles and the grants on objects.

// grants is a relation of one row for each object that has a grant, which
// holds the object as the columns of keys and the grants on it as access. A
// grantee is one line, grantee=privileges, with the privileges joined by a
// comma, and the lines are joined by a newline. view is the INFORMATION_SCHEMA
// view of the grants, and keys are its columns that name the object.
//
// It is a relation that a statement joins once, so the grants are read in one
// pass over the view and not once for each object.
func grants(view string, keys ...string) string {
	cols := strings.Join(keys, ", ")
	return `(SELECT ` + cols + `, STRING_AGG(l.line, '\n' ORDER BY l.line) AS access FROM (SELECT ` + cols +
		`, grantee || '=' || STRING_AGG(privilege_type, ',' ORDER BY privilege_type) AS line` +
		` FROM information_schema.` + view + ` GROUP BY ` + cols + `, grantee) l GROUP BY ` + cols + `)`
}

// columnGrants is a relation of one row for each table that has a grant on a
// column, which holds the table as table_schema and table_name and the grants
// as access, one on each line as column:grantee=privileges. COLUMN_PRIVILEGES
// also holds a row for every column of a table that was granted as a whole, and
// those rows are left out, because the grant is not on the column.
const columnGrants = `(SELECT c.table_schema, c.table_name, STRING_AGG(c.line, '\n' ORDER BY c.line) AS access FROM (` +
	`SELECT p.table_schema, p.table_name, p.column_name || ':' || p.grantee || '=' ||` +
	` STRING_AGG(p.privilege_type, ',' ORDER BY p.privilege_type) AS line` +
	` FROM information_schema.column_privileges p` +
	` WHERE NOT EXISTS (SELECT 1 FROM information_schema.table_privileges w` +
	` WHERE w.table_schema = p.table_schema AND w.table_name = p.table_name` +
	` AND w.grantee = p.grantee AND w.privilege_type = p.privilege_type)` +
	` GROUP BY p.table_schema, p.table_name, p.column_name, p.grantee) c GROUP BY c.table_schema, c.table_name)`

func registerRoles() {
	// \du. A database role is a name that grants attach to. It has no password
	// and no flag, and it cannot log in. The system roles are public,
	// spanner_info_reader and spanner_sys_reader, and they are hidden unless the
	// caller asks.
	dbmeta.Roles.Register(dbmeta.Spanner, &dbmeta.Binding[dbmeta.Role]{
		Stmt: dbmeta.Stmt{
			always("SELECT r.role_name AS `name`"),
			always(", FALSE AS `superuser`"),
			always(", FALSE AS `create_role`"),
			always(", FALSE AS `create_db`"),
			always(", FALSE AS `can_login`"),
			always(", FALSE AS `replication`"),
			always(", FALSE AS `bypass_rls`"),
			always(", TRUE AS `inherit`"),
			always(", -1 AS `conn_limit`"),
			always(", CAST(NULL AS STRING) AS `valid_until`"),
			always(", COALESCE(g.member_of, '') AS `member_of`"),
			always(", CAST(NULL AS STRING) AS `comment`"),
			always("FROM information_schema.roles r"),
			always("LEFT JOIN (SELECT grantee, STRING_AGG(role_name, ', ' ORDER BY role_name) AS member_of"),
			always("FROM information_schema.role_grantees GROUP BY grantee) g ON g.grantee = r.role_name"),
			always("WHERE (@with_system OR NOT r.is_system)"),
			always("AND " + like("r.role_name", "@name")),
			always("ORDER BY 1"),
		},
		Fields: []dbmeta.Field{
			{Name: "name"},
			{Name: "superuser", Desc: "always false: a database role has no flags. Who can administer the database is decided by IAM and is not in INFORMATION_SCHEMA"},
			{Name: "create_role", Desc: "always false, for the same reason"},
			{Name: "create_db", Desc: "always false, for the same reason"},
			{Name: "can_login", Desc: "always false: a database role is a name for grants, and a session names the role it uses"},
			{Name: "replication", Desc: "always false: Spanner has no replication flag"},
			{Name: "bypass_rls", Desc: "always false: Spanner has no row level security"},
			{Name: "inherit", Desc: "always true: a member of a role has the privileges of the role"},
			{Name: "conn_limit", Desc: "always -1: Spanner sets no limit on the connections of a role"},
			{Name: "valid_until", Desc: "always absent: a role does not expire"},
			{Name: "member_of", Desc: "the roles this role belongs to directly, joined by a comma and a space, and empty for none"},
			{Name: "comment", Desc: "always absent: Spanner records no comment on anything"},
		},
		Params: []dbmeta.Param{
			{Name: "name", Desc: "role name pattern, empty for every role", Default: ""},
			{Name: "with_system", Desc: "include public, spanner_info_reader and spanner_sys_reader", Default: false},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Role, error) {
			var v dbmeta.Role
			err := rows.Scan(&v.Name, &v.Superuser, &v.CreateRole, &v.CreateDB,
				&v.CanLogin, &v.Replication, &v.BypassRLS, &v.Inherit, &v.ConnLimit,
				&v.ValidUntil, &v.MemberOf, &v.Comment)
			return v, err
		},
	})

	// \drg. ROLE_GRANTEES lists a role and the role that is a member of it, so
	// GRANT ROLE reader TO ROLE staff is the row reader, staff.
	dbmeta.RoleGrants.Register(dbmeta.Spanner, &dbmeta.Binding[dbmeta.RoleGrant]{
		Stmt: dbmeta.Stmt{
			always("SELECT g.grantee AS `role`"),
			always(", g.role_name AS `member_of`"),
			always(", CAST(NULL AS STRING) AS `grantor`"),
			always(", FALSE AS `admin`"),
			always(", TRUE AS `inherit`"),
			always(", FALSE AS `set`"),
			always("FROM information_schema.role_grantees g"),
			always("WHERE " + like("g.role_name", "@name")),
			always("ORDER BY 1, 2"),
		},
		Fields: []dbmeta.Field{
			{Name: "role", Desc: "the role that is a member"},
			{Name: "member_of", Desc: "the role it belongs to"},
			{Name: "grantor", Desc: "always absent: ROLE_GRANTEES records no grantor"},
			{Name: "admin", Desc: "always false: Spanner has no admin option on a role"},
			{Name: "inherit", Desc: "always true: a member has the privileges of its role"},
			{Name: "set", Desc: "always false: Spanner has no SET ROLE, and a session names its role when it connects"},
		},
		Params: nameOnly("granted role"),
		Scan: func(rows *sql.Rows) (dbmeta.RoleGrant, error) {
			var v dbmeta.RoleGrant
			err := rows.Scan(&v.Role, &v.MemberOf, &v.Grantor, &v.Admin, &v.Inherit, &v.Set)
			return v, err
		},
	})

	// \dp. One row for each table, view, change stream and function, with the
	// grants on it folded into one text, one grantee on each line. An object
	// that nothing was granted on has an absent access, as psql has for a default.
	// The column grants are the ones that name a column, and a column that
	// holds a grant only because its table does is left to the table.
	dbmeta.Privileges.Register(dbmeta.Spanner, &dbmeta.Binding[dbmeta.Privilege]{
		Stmt: dbmeta.Stmt{
			always("SELECT x.schema AS `schema`"),
			always(", x.name AS `name`"),
			always(", x.type AS `type`"),
			always(", x.access AS `access`"),
			always(", x.column_access AS `column_access`"),
			always(", CAST(NULL AS STRING) AS `policies`"),
			always("FROM ("),
			always("SELECT t.table_schema AS schema, t.table_name AS name"),
			always(", IF(t.table_type = 'BASE TABLE', 'table', LOWER(t.table_type)) AS type"),
			always(", a.access AS access, c.access AS column_access"),
			always("FROM information_schema.tables t"),
			always("LEFT JOIN " + grants("table_privileges", "table_schema", "table_name") + " a"),
			always("ON a.table_schema = t.table_schema AND a.table_name = t.table_name"),
			always("LEFT JOIN " + columnGrants + " c"),
			always("ON c.table_schema = t.table_schema AND c.table_name = t.table_name"),
			always("UNION ALL"),
			always("SELECT s.change_stream_schema, s.change_stream_name, 'change stream', a.access, CAST(NULL AS STRING)"),
			always("FROM information_schema.change_streams s"),
			always("LEFT JOIN " + grants("change_stream_privileges", "change_stream_schema", "change_stream_name") + " a"),
			always("ON a.change_stream_schema = s.change_stream_schema AND a.change_stream_name = s.change_stream_name"),
			always("UNION ALL"),
			always("SELECT r.specific_schema, r.specific_name, LOWER(r.routine_type), a.access, CAST(NULL AS STRING)"),
			always("FROM information_schema.routines r"),
			always("LEFT JOIN " + grants("routine_privileges", "specific_schema", "specific_name") + " a"),
			always("ON a.specific_schema = r.specific_schema AND a.specific_name = r.specific_name"),
			always(") x"),
			always("WHERE " + notSystem("x.schema")),
			always("AND " + like("x.schema", "@schema")),
			always("AND " + like("x.name", "@name")),
			always("ORDER BY 1, 2"),
		},
		Fields: []dbmeta.Field{
			{Name: "schema", Desc: "empty for the default schema"},
			{Name: "name"},
			{Name: "type", Desc: "table, view, synonym, change stream, function or table function"},
			{Name: "access", Desc: "the grants on the object, one grantee on each line as grantee=privileges, with the privileges joined by a comma, such as SELECT,UPDATE. Absent when nothing was granted. Spanner records no grantor"},
			{Name: "column_access", Desc: "the grants that name a column, one on each line as column:grantee=privileges. A grant on the whole table is in access and not here"},
			{Name: "policies", Desc: "always absent: Spanner has no row level security"},
		},
		Params: schemaNameSystem("object"),
		Scan: func(rows *sql.Rows) (dbmeta.Privilege, error) {
			var v dbmeta.Privilege
			err := rows.Scan(&v.Schema, &v.Name, &v.Type, &v.Access, &v.ColumnAccess, &v.Policies)
			return v, err
		},
	})

	// The grants that name a column, one row for each column and grantee. The
	// view COLUMN_PRIVILEGES also holds a row for every column of a table that was
	// granted as a whole, and those rows are left out, because the grant is not
	// on the column. Spanner has no window function, so the ordinal of a grantee
	// is the count of the grantees of its column that sort at or before it.
	dbmeta.ColumnPrivileges.Register(dbmeta.Spanner, &dbmeta.Binding[dbmeta.ColumnPrivilege]{
		Stmt: dbmeta.Stmt{
			always("WITH g AS (SELECT c.table_schema, c.table_name, c.column_name, c.grantee"),
			always(", STRING_AGG(c.privilege_type, ',' ORDER BY c.privilege_type) AS privileges"),
			always("FROM information_schema.column_privileges c"),
			always("WHERE NOT EXISTS (SELECT 1 FROM information_schema.table_privileges w"),
			always("WHERE w.table_schema = c.table_schema AND w.table_name = c.table_name"),
			always("AND w.grantee = c.grantee AND w.privilege_type = c.privilege_type)"),
			always("AND " + notSystem("c.table_schema")),
			always("AND " + like("c.table_schema", "@schema")),
			always("AND " + like("c.table_name", "@parent")),
			always("AND " + like("c.column_name", "@name")),
			always("GROUP BY c.table_schema, c.table_name, c.column_name, c.grantee)"),
			always("SELECT p.table_schema AS `schema`"),
			always(", p.table_name AS `table`"),
			always(", p.column_name AS `column`"),
			always(", COUNT(q.grantee) AS `ordinal`"),
			always(", p.grantee || '=' || p.privileges AS `access`"),
			always(", p.grantee AS `grantee`"),
			always(", CAST(NULL AS STRING) AS `grantor`"),
			always(", p.privileges AS `privileges`"),
			always("FROM g p JOIN g q ON q.table_schema = p.table_schema AND q.table_name = p.table_name"),
			always("AND q.column_name = p.column_name AND q.grantee <= p.grantee"),
			always("GROUP BY p.table_schema, p.table_name, p.column_name, p.grantee, p.privileges"),
			always("ORDER BY 1, 2, 3, 4"),
		},
		Fields: []dbmeta.Field{
			{Name: "schema"}, {Name: "table"}, {Name: "column"},
			{Name: "ordinal", Desc: "the position of the grantee among the grantees of the column, in the order of their names"},
			{Name: "access", Desc: "grantee=privileges, with the privileges joined by a comma"},
			{Name: "grantee"},
			{Name: "grantor", Desc: "always absent: Spanner records no grantor"},
			{Name: "privileges", Desc: "the privileges, joined by a comma, such as SELECT,UPDATE"},
		},
		Params: schemaParentName("table", "column"),
		Scan: func(rows *sql.Rows) (dbmeta.ColumnPrivilege, error) {
			var v dbmeta.ColumnPrivilege
			err := rows.Scan(&v.Schema, &v.Table, &v.Column, &v.Ordinal, &v.Access,
				&v.Grantee, &v.Grantor, &v.Privileges)
			return v, err
		},
	})
}
