package trino

import (
	"database/sql"

	"github.com/xo/dbmeta"
)

func registerExtra() {
	// \dT. system.jdbc.types is the engine's type list rather than a per
	// catalog one, because a Trino type belongs to the engine.
	dbmeta.Types.Register(dbmeta.Trino, &dbmeta.Binding[dbmeta.Type]{
		Stmt: dbmeta.Stmt{
			always(`SELECT '' AS "catalog"`),
			always(`, '' AS "schema"`),
			always(`, t.type_name AS "name"`),
			always(`, t.type_name AS "internal"`),
			always(`, 'base' AS "kind"`),
			always(`, '' AS "elements"`),
			always(`, '' AS "owner"`),
			always(`, CAST(NULL AS varchar) AS "access"`),
			always(`, CAST(NULL AS varchar) AS "comment"`),
			always(`FROM system.jdbc.types t`),
			always(`WHERE (@name = '' OR t.type_name LIKE @name)`),
			always(`ORDER BY t.type_name`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "always empty: a type belongs to the engine rather than a catalog"},
			{Name: "schema", Desc: "always empty: a type belongs to the engine rather than a schema"},
			{Name: "name"},
			{Name: "internal", Desc: "the same as name: Trino has no separate internal spelling"},
			{Name: "kind", Desc: "always base: every Trino type is built in"},
			{Name: "elements", Desc: "always empty: Trino has no enumerated type"},
			{Name: "owner", Desc: "always empty: a built in type has no owner"},
			{Name: "access", Desc: "always absent: a type carries no grant"},
			{Name: "comment", Desc: "always absent: Trino stores no type comment"},
		},
		Params: []dbmeta.Param{
			{Name: "name", Desc: "type name pattern, empty for every type", Default: ""},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Type, error) {
			var v dbmeta.Type
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Name, &v.Internal, &v.Kind,
				&v.Elements, &v.Owner, &v.Access, &v.Comment)
			return v, err
		},
	})

	// \dA. A connector is how Trino reaches and searches a table, which is
	// the question an access method answers. models/mysql maps a storage
	// engine the same way.
	dbmeta.AccessMethods.Register(dbmeta.Trino, &dbmeta.Binding[dbmeta.AccessMethod]{
		Stmt: dbmeta.Stmt{
			always(`SELECT c.connector_name AS "name"`),
			always(`, 'table' AS "type"`),
			always(`, '' AS "handler"`),
			// One connector can back several catalogs, so the catalogs it
			// backs are the useful thing to say about it.
			always(`, array_join(array_agg(c.catalog_name ORDER BY c.catalog_name), ', ') AS "comment"`),
			always(`FROM system.metadata.catalogs c`),
			always(`WHERE (@name = '' OR c.connector_name LIKE @name)`),
			always(`GROUP BY c.connector_name`),
			always(`ORDER BY c.connector_name`),
		},
		Fields: []dbmeta.Field{
			{Name: "name", Desc: "the connector name, such as memory or tpch"},
			{Name: "type", Desc: "always table: a connector serves tables"},
			{Name: "handler", Desc: "always empty: Trino names no handler function"},
			{Name: "comment", Desc: "the catalogs this connector backs"},
		},
		Params: []dbmeta.Param{
			{Name: "name", Desc: "connector name pattern, empty for every connector", Default: ""},
		},
		Scan: func(rows *sql.Rows) (dbmeta.AccessMethod, error) {
			var v dbmeta.AccessMethod
			err := rows.Scan(&v.Name, &v.Type, &v.Handler, &v.Comment)
			return v, err
		},
	})

	registerRoles()
	registerCurrent()
}

// A role in Trino belongs to a catalog rather than to the server, and only a
// connector that implements role management has any. The memory connector
// does not, so these three run and return nothing on the test image. That is
// the same shape as ClickHouse and its named collections: the query is right
// and the fixture cannot build the object. See docs/COVERAGE.md.
func registerRoles() {
	// \du. Trino has roles and no users: a principal is whoever the client
	// says it is, and authentication is the server's business rather than the
	// catalog's.
	dbmeta.Roles.Register(dbmeta.Trino, &dbmeta.Binding[dbmeta.Role]{
		Stmt: dbmeta.Stmt{
			always(`SELECT r.role_name AS "name"`),
			always(`, false AS "superuser"`),
			always(`, false AS "create_role"`),
			always(`, false AS "create_db"`),
			always(`, false AS "can_login"`),
			always(`, false AS "replication"`),
			always(`, false AS "bypass_rls"`),
			always(`, true AS "inherit"`),
			always(`, CAST(0 AS bigint) AS "conn_limit"`),
			always(`, CAST(NULL AS varchar) AS "valid_until"`),
			always(`, '' AS "member_of"`),
			always(`, CAST(NULL AS varchar) AS "comment"`),
			always(`FROM information_schema.roles r`),
			always(`WHERE (@name = '' OR r.role_name LIKE @name)`),
			always(`ORDER BY r.role_name`),
		},
		Fields: []dbmeta.Field{
			{Name: "name"},
			{Name: "superuser", Desc: "always false: Trino marks no role as a superuser"},
			{Name: "create_role", Desc: "always false: the right to create a role is not recorded on the role"},
			{Name: "create_db", Desc: "always false: a catalog is configured on the server, not created"},
			{Name: "can_login", Desc: "always false: a Trino role is not a login"},
			{Name: "replication", Desc: "always false: Trino has no replication"},
			{Name: "bypass_rls", Desc: "always false: Trino has no row level security of its own"},
			{Name: "inherit", Desc: "always true: a granted role applies to its members"},
			{Name: "conn_limit", Desc: "always zero: Trino sets no per role connection limit"},
			{Name: "valid_until", Desc: "always absent: a role does not expire"},
			{Name: "member_of", Desc: "always empty: RoleGrants reads membership"},
			{Name: "comment", Desc: "always absent: Trino stores no role comment"},
		},
		Params: []dbmeta.Param{
			{
				Name: "name",
				Desc: "role name pattern, empty for every role. This reads the" +
					" session catalog, because a Trino role belongs to a catalog",
				Default: "",
			},
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
	dbmeta.RoleGrants.Register(dbmeta.Trino, &dbmeta.Binding[dbmeta.RoleGrant]{
		Stmt: dbmeta.Stmt{
			always(`SELECT a.grantee AS "role"`),
			always(`, a.role_name AS "member_of"`),
			always(`, CAST(NULL AS varchar) AS "grantor"`),
			always(`, a.is_grantable = 'YES' AS "admin"`),
			always(`, true AS "inherit"`),
			always(`, true AS "set"`),
			always(`FROM information_schema.applicable_roles a`),
			always(`WHERE (@name = '' OR a.grantee LIKE @name)`),
			always(`ORDER BY a.grantee, a.role_name`),
		},
		Fields: []dbmeta.Field{
			{Name: "role", Desc: "the grantee, which is a role or the current principal"},
			{Name: "member_of"},
			{Name: "grantor", Desc: "always absent: applicable_roles records no grantor"},
			{Name: "admin", Desc: "from is_grantable"},
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

	// \dp. One row per table, with the grants folded into a string the way
	// psql prints them.
	dbmeta.Privileges.Register(dbmeta.Trino, &dbmeta.Binding[dbmeta.Privilege]{
		Stmt: dbmeta.Stmt{
			always(`SELECT p.table_schema AS "schema"`),
			always(`, p.table_name AS "name"`),
			always(`, 'table' AS "type"`),
			always(`, array_join(array_agg(p.grantee || '=' || p.privilege_type` +
				` ORDER BY p.grantee, p.privilege_type), ', ') AS "access"`),
			always(`, '' AS "column_access"`),
			always(`, '' AS "policies"`),
			always(`FROM information_schema.table_privileges p`),
			always(`WHERE ` + notSystem("p.table_catalog", "p.table_schema")),
			always(`AND (@schema = '' OR p.table_schema LIKE @schema)`),
			always(`AND (@name = '' OR p.table_name LIKE @name)`),
			always(`GROUP BY p.table_schema, p.table_name`),
			always(`ORDER BY p.table_schema, p.table_name`),
		},
		Fields: []dbmeta.Field{
			{Name: "schema"}, {Name: "name"},
			{Name: "type", Desc: "always table: Trino grants on a table"},
			{Name: "access", Desc: "grantee=privilege, comma separated, the way psql prints it"},
			{Name: "column_access", Desc: "always empty: Trino grants no column privilege"},
			{Name: "policies", Desc: "always empty: Trino has no row level policy of its own"},
		},
		Params: []dbmeta.Param{
			{Name: "schema", Desc: "schema name pattern, empty for every schema", Default: ""},
			{Name: "name", Desc: "table name pattern, empty for every table", Default: ""},
			{Name: "with_system", Desc: "include the catalogs and schemas Trino keeps for itself", Default: false},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Privilege, error) {
			var v dbmeta.Privilege
			err := rows.Scan(&v.Schema, &v.Name, &v.Type, &v.Access, &v.ColumnAccess, &v.Policies)
			return v, err
		},
	})
}

func registerCurrent() {
	dbmeta.CurrentSchema.Register(dbmeta.Trino, &dbmeta.Binding[dbmeta.Schema]{
		Stmt: dbmeta.Stmt{
			always(`SELECT current_catalog AS "catalog"`),
			always(`, current_schema AS "name"`),
			always(`, '' AS "owner"`),
			always(`, CAST(NULL AS varchar) AS "comment"`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "the session catalog, which Trino has and the others do not"},
			{Name: "name"},
			{Name: "owner", Desc: "always empty: a schema records no owner"},
			{Name: "comment", Desc: "always absent: Trino stores no schema comment"},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Schema, error) {
			var v dbmeta.Schema
			err := rows.Scan(&v.Catalog, &v.Name, &v.Owner, &v.Comment)
			return v, err
		},
	})

	// Trino has no users. The client names a principal on every request and
	// the server takes it, unless an authenticator is configured, so the
	// effective user and the session user are the same string.
	dbmeta.CurrentUser.Register(dbmeta.Trino, &dbmeta.Binding[dbmeta.User]{
		Stmt: dbmeta.Stmt{
			always(`SELECT current_user AS "name"`),
			always(`, current_user AS "session"`),
		},
		Fields: []dbmeta.Field{
			{Name: "name"},
			{
				Name: "session",
				Desc: "the same as name: Trino does not separate the authenticated" +
					" principal from the effective one",
			},
		},
		Scan: func(rows *sql.Rows) (dbmeta.User, error) {
			var v dbmeta.User
			err := rows.Scan(&v.Name, &v.Session)
			return v, err
		},
	})
}
