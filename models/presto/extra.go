package presto

import (
	"database/sql"

	"github.com/xo/dbmeta"
)

func registerExtra() {
	// \dT. system.jdbc.types is the engine's type list rather than a per
	// catalog one, because a Presto type belongs to the engine.
	dbmeta.Types.Register(dbmeta.Presto, &dbmeta.Binding[dbmeta.Type]{
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
			{Name: "internal", Desc: "the same as name: Presto has no separate internal spelling"},
			{Name: "kind", Desc: "always base: every Presto type is built in"},
			{Name: "elements", Desc: "always empty: Presto has no enumerated type"},
			{Name: "owner", Desc: "always empty: a built in type has no owner"},
			{Name: "access", Desc: "always absent: a type carries no grant"},
			{Name: "comment", Desc: "always absent: Presto stores no type comment"},
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

	// \dA. A connector is how Presto reaches and searches a table, which is
	// the question an access method answers. models/mysql maps a storage
	// engine the same way.
	dbmeta.AccessMethods.Register(dbmeta.Presto, &dbmeta.Binding[dbmeta.AccessMethod]{
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
			{Name: "handler", Desc: "always empty: Presto names no handler function"},
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

// Presto registers no Roles and no RoleGrants, and Trino registers both.
//
// The difference is not the query, which is the standard view in both, but
// what the connector does with it. Trino's memory connector answers nothing
// and Presto's raises:
//
//	NOT_SUPPORTED: This connector does not support roles
//
// D34 says a query dbmeta offers must run. One that fails outright on the only
// connector the image configures is not one this model can offer, so it is
// left out rather than shipped to fail. Privileges is registered, because
// information_schema.table_privileges answers on the same connector.
func registerRoles() {
	// \dp. One row per table, with the grants folded into a string the way
	// psql prints them.
	dbmeta.Privileges.Register(dbmeta.Presto, &dbmeta.Binding[dbmeta.Privilege]{
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
			{Name: "type", Desc: "always table: Presto grants on a table"},
			{Name: "access", Desc: "grantee=privilege, comma separated, the way psql prints it"},
			{Name: "column_access", Desc: "always empty: Presto grants no column privilege"},
			{Name: "policies", Desc: "always empty: Presto has no row level policy of its own"},
		},
		Params: []dbmeta.Param{
			{Name: "schema", Desc: "schema name pattern, empty for every schema", Default: ""},
			{Name: "name", Desc: "table name pattern, empty for every table", Default: ""},
			{Name: "with_system", Desc: "include the catalogs and schemas Presto keeps for itself", Default: false},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Privilege, error) {
			var v dbmeta.Privilege
			err := rows.Scan(&v.Schema, &v.Name, &v.Type, &v.Access, &v.ColumnAccess, &v.Policies)
			return v, err
		},
	})
}

// Presto registers no CurrentSchema. Neither current_catalog nor
// current_schema resolves, and nothing in system.runtime carries the session,
// so there is no statement to register. models/trino answers it because Presto
// has both expressions.
func registerCurrent() {
	dbmeta.CurrentUser.Register(dbmeta.Presto, &dbmeta.Binding[dbmeta.User]{
		Stmt: dbmeta.Stmt{
			always(`SELECT current_user AS "name"`),
			always(`, current_user AS "session"`),
		},
		Fields: []dbmeta.Field{
			{Name: "name"},
			{
				Name: "session",
				Desc: "the same as name: Presto does not separate the authenticated" +
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
