package hive

import (
	"database/sql"

	"github.com/xo/dbmeta"
)

func registerExtra() {
	registerFunctions()
	registerRoles()
	registerComments()
	registerCurrent()
}

func registerFunctions() {
	// \df. A Hive function is a Java class registered under a name. There
	// is no body, no parameter list and no return type in the metastore:
	// all of that is in the class.
	dbmeta.Functions.Register(dbmeta.Hive, &dbmeta.Binding[dbmeta.Function]{
		Stmt: dbmeta.Stmt{
			always(`SELECT '' AS "catalog"`),
			always(`, d.NAME AS "schema"`),
			always(`, f.FUNC_NAME AS "name"`),
			always(`, CAST(f.FUNC_ID AS string) AS "id"`),
			always(`, 'function' AS "kind"`),
			always(`, '' AS "result_type"`),
			always(`, '' AS "arg_types"`),
			always(`, '' AS "volatility"`),
			always(`, '' AS "parallel"`),
			always(`, f.OWNER_NAME AS "owner"`),
			always(`, '' AS "security"`),
			always(`, CAST(NULL AS string) AS "access"`),
			always(`, CASE f.FUNC_TYPE WHEN 1 THEN 'java' ELSE CAST(f.FUNC_TYPE AS string) END AS "language"`),
			always(`, f.CLASS_NAME AS "source"`),
			always(`, CAST(NULL AS string) AS "comment"`),
			always(`FROM sys.FUNCS f JOIN sys.DBS d ON d.DB_ID = f.DB_ID`),
			always(`WHERE ` + notSystem),
			always(`AND ` + like(`d.NAME`, `@schema`)),
			always(`AND ` + like(`f.FUNC_NAME`, `@name`)),
			always(`ORDER BY d.NAME, f.FUNC_NAME`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "always empty: the metastore has no level above a database"},
			{Name: "schema"}, {Name: "name"},
			{Name: "id", Desc: "the metastore's own function id"},
			{Name: "kind", Desc: "always function: Hive has no procedure"},
			{Name: "result_type", Desc: "always empty: the return type is in the Java class and not in the metastore"},
			{Name: "arg_types", Desc: "always empty, for the same reason. This is why RoutineParameters has no answer for Hive at all"},
			{Name: "volatility", Desc: "always empty: Hive marks no routine deterministic in the metastore"},
			{Name: "parallel", Desc: "always empty: Hive has no parallel safety marking"},
			{Name: "owner", Desc: "from FUNCS.OWNER_NAME"},
			{Name: "security", Desc: "always empty: Hive has no SQL SECURITY clause"},
			{Name: "access", Desc: "always absent: Privileges reads the grants"},
			{Name: "language", Desc: "java for a registered class, which is every function Hive records. A built in function is not in the metastore and does not appear here"},
			{Name: "source", Desc: "the Java class name, which is the nearest thing to a body Hive has"},
			{Name: "comment", Desc: "always absent: Hive records no comment on a function"},
		},
		Params: schemaAndName("function"),
		Scan: func(rows *sql.Rows) (dbmeta.Function, error) {
			var v dbmeta.Function
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Name, &v.ID, &v.Kind,
				&v.ResultType, &v.ArgTypes, &v.Volatility, &v.Parallel, &v.Owner,
				&v.Security, &v.Access, &v.Language, &v.Source, &v.Comment)
			return v, err
		},
	})

	// \dA. A SerDe is how Hive reads and writes a table's rows, which is
	// the question an access method answers. models/mysql maps a storage
	// engine the same way and models/hana maps the row and column stores.
	dbmeta.AccessMethods.Register(dbmeta.Hive, &dbmeta.Binding[dbmeta.AccessMethod]{
		Stmt: dbmeta.Stmt{
			always(`SELECT e.SLIB AS "name"`),
			always(`, 'table' AS "type"`),
			always(`, '' AS "handler"`),
			always(`, CONCAT('used by ', CAST(COUNT(*) AS string), ' tables') AS "comment"`),
			always(`FROM sys.SERDES e JOIN sys.SDS s ON s.SERDE_ID = e.SERDE_ID`),
			always(`WHERE e.SLIB IS NOT NULL`),
			always(`AND ` + like(`e.SLIB`, `@name`)),
			always(`GROUP BY e.SLIB`),
			always(`ORDER BY e.SLIB`),
		},
		Fields: []dbmeta.Field{
			{Name: "name", Desc: "the SerDe class, such as org.apache.hadoop.hive.serde2.lazy.LazySimpleSerDe. It is how Hive reads and writes the rows of a table"},
			{Name: "type", Desc: "always table: a SerDe applies to a table"},
			{Name: "handler", Desc: "always empty: the SerDe class is the handler and it is in name"},
			{Name: "comment", Desc: "how many storage descriptors use it. Hive keeps no catalog of the SerDes it has, so this counts the ones in use and a SerDe nothing uses does not appear"},
		},
		Params: []dbmeta.Param{
			{Name: "name", Desc: "SerDe class pattern, empty for every one", Default: ""},
		},
		Scan: func(rows *sql.Rows) (dbmeta.AccessMethod, error) {
			var v dbmeta.AccessMethod
			err := rows.Scan(&v.Name, &v.Type, &v.Handler, &v.Comment)
			return v, err
		},
	})
}

func registerRoles() {
	// \du and \dg. Hive has roles and no users: a principal is whoever
	// the client says it is, and a user exists only as a name in a grant.
	dbmeta.Roles.Register(dbmeta.Hive, &dbmeta.Binding[dbmeta.Role]{
		Stmt: dbmeta.Stmt{
			always(`SELECT r.ROLE_NAME AS "name"`),
			always(`, r.ROLE_NAME = 'admin' AS "superuser"`),
			always(`, FALSE AS "create_role"`),
			always(`, FALSE AS "create_db"`),
			always(`, FALSE AS "can_login"`),
			always(`, FALSE AS "replication"`),
			always(`, FALSE AS "bypass_rls"`),
			always(`, TRUE AS "inherit"`),
			always(`, CAST(0 AS bigint) AS "conn_limit"`),
			always(`, CAST(NULL AS string) AS "valid_until"`),
			always(`, '' AS "member_of"`),
			always(`, CAST(NULL AS string) AS "comment"`),
			always(`FROM sys.ROLES r`),
			always(`WHERE ` + like(`r.ROLE_NAME`, `@name`)),
			always(`ORDER BY r.ROLE_NAME`),
		},
		Fields: []dbmeta.Field{
			{Name: "name"},
			{Name: "superuser", Desc: "true for admin, which is the role Hive creates for that purpose. Hive records no superuser flag"},
			{Name: "create_role", Desc: "always false: the right to create a role is not recorded on the role"},
			{Name: "create_db", Desc: "always false, for the same reason"},
			{Name: "can_login", Desc: "always false: every row here is a role. Hive has no users in the metastore, only names that appear in grants"},
			{Name: "replication", Desc: "always false: Hive configures replication outside the metastore"},
			{Name: "bypass_rls", Desc: "always false: Hive has no row level security"},
			{Name: "inherit", Desc: "always true: a granted role applies to its members"},
			{Name: "conn_limit", Desc: "always zero: Hive sets no per principal connection limit"},
			{Name: "valid_until", Desc: "always absent: a Hive role does not expire"},
			{Name: "member_of", Desc: "always empty: RoleGrants reads membership"},
			{Name: "comment", Desc: "always absent: Hive records no comment on a role"},
		},
		Params: []dbmeta.Param{
			{Name: "name", Desc: "role name pattern, empty for every role", Default: ""},
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
	dbmeta.RoleGrants.Register(dbmeta.Hive, &dbmeta.Binding[dbmeta.RoleGrant]{
		Stmt: dbmeta.Stmt{
			always(`SELECT m.PRINCIPAL_NAME AS "role"`),
			always(`, r.ROLE_NAME AS "member_of"`),
			always(`, m.GRANTOR AS "grantor"`),
			always(`, m.GRANT_OPTION > 0 AS "admin"`),
			always(`, TRUE AS "inherit"`),
			always(`, TRUE AS "set"`),
			always(`FROM sys.ROLE_MAP m JOIN sys.ROLES r ON r.ROLE_ID = m.ROLE_ID`),
			always(`WHERE ` + like(`m.PRINCIPAL_NAME`, `@name`)),
			always(`ORDER BY m.PRINCIPAL_NAME, r.ROLE_NAME`),
		},
		Fields: []dbmeta.Field{
			{Name: "role", Desc: "the grantee, which is a user name or another role. PRINCIPAL_TYPE says which and this kind has nowhere to put it"},
			{Name: "member_of", Desc: "the role granted"},
			{Name: "grantor"},
			{Name: "admin", Desc: "from GRANT_OPTION"},
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

	// \dp. One row per table, with the grants folded together the way psql
	// prints them. A database grant is in DB_PRIVS and a column grant in
	// TBL_COL_PRIVS, and both arrive as their own rows here.
	dbmeta.Privileges.Register(dbmeta.Hive, &dbmeta.Binding[dbmeta.Privilege]{
		Stmt: dbmeta.Stmt{
			always(`SELECT d.NAME AS "schema"`),
			always(`, t.TBL_NAME AS "name"`),
			always(`, 'table' AS "type"`),
			always(`, CONCAT_WS(', ', COLLECT_LIST(` +
				`CONCAT(p.PRINCIPAL_NAME, '=', p.TBL_PRIV))) AS "access"`),
			always(`, '' AS "column_access"`),
			always(`, '' AS "policies"`),
			always(`FROM sys.TBL_PRIVS p`),
			always(`JOIN sys.TBLS t ON t.TBL_ID = p.TBL_ID`),
			always(`JOIN sys.DBS d ON d.DB_ID = t.DB_ID`),
			always(`WHERE ` + notSystem),
			always(`AND ` + like(`d.NAME`, `@schema`)),
			always(`AND ` + like(`t.TBL_NAME`, `@name`)),
			always(`GROUP BY d.NAME, t.TBL_NAME`),
			always(`ORDER BY d.NAME, t.TBL_NAME`),
		},
		Fields: []dbmeta.Field{
			{Name: "schema"}, {Name: "name"},
			{Name: "type", Desc: "always table: this reads the table grants. Hive also grants on a database and on a column, and neither is an object this kind names"},
			{Name: "access", Desc: "grantee=privilege, comma separated, the way psql prints it. COLLECT_LIST does not promise an order, so two runs can differ in order and not in content"},
			{Name: "column_access", Desc: "always empty: Hive keeps a column grant in TBL_COL_PRIVS and folding it in would need a second aggregate over a second table"},
			{Name: "policies", Desc: "always empty: Hive has no row level policy in the metastore"},
		},
		Params: schemaAndName("table"),
		Scan: func(rows *sql.Rows) (dbmeta.Privilege, error) {
			var v dbmeta.Privilege
			err := rows.Scan(&v.Schema, &v.Name, &v.Type, &v.Access, &v.ColumnAccess, &v.Policies)
			return v, err
		},
	})
}

func registerComments() {
	// Every comment in the metastore. Hive keeps a table's and a
	// database's as a property and a column's on the column, so this is a
	// union over the three places.
	dbmeta.Comments.Register(dbmeta.Hive, &dbmeta.Binding[dbmeta.Comment]{
		Stmt: dbmeta.Stmt{
			always(`SELECT d.NAME AS "schema", t.TBL_NAME AS "name"`),
			always(`, 'table' AS "type", p.PARAM_VALUE AS "comment"`),
			always(`FROM sys.TABLE_PARAMS p`),
			always(`JOIN sys.TBLS t ON t.TBL_ID = p.TBL_ID`),
			always(`JOIN sys.DBS d ON d.DB_ID = t.DB_ID`),
			always(`WHERE p.PARAM_KEY = 'comment' AND p.PARAM_VALUE IS NOT NULL`),
			always(`AND ` + notSystem + ` AND ` + like(`t.TBL_NAME`, `@name`)),
			always(`UNION ALL`),
			always(`SELECT d.NAME, d.NAME, 'database', p.PARAM_VALUE`),
			always(`FROM sys.DATABASE_PARAMS p JOIN sys.DBS d ON d.DB_ID = p.DB_ID`),
			always(`WHERE p.PARAM_KEY = 'comment' AND p.PARAM_VALUE IS NOT NULL`),
			always(`AND ` + notSystem + ` AND ` + like(`d.NAME`, `@name`)),
			always(`ORDER BY 3, 1, 2`),
		},
		Fields: []dbmeta.Field{
			{Name: "schema", Desc: "the database. For a database's own comment it repeats the name"},
			{Name: "name"},
			{Name: "type", Desc: "table or database. A column comment is left out, because it needs two names to identify it and this kind has one, and Columns carries it"},
			{Name: "comment"},
		},
		Params: []dbmeta.Param{
			{Name: "name", Desc: "object name pattern, empty for every commented object", Default: ""},
			{Name: "with_system", Desc: "include sys and information_schema", Default: false},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Comment, error) {
			var v dbmeta.Comment
			err := rows.Scan(&v.Schema, &v.Name, &v.Type, &v.Comment)
			return v, err
		},
	})
}

func registerCurrent() {
	dbmeta.CurrentSchema.Register(dbmeta.Hive, &dbmeta.Binding[dbmeta.Schema]{
		Stmt: dbmeta.Stmt{
			always(`SELECT '' AS "catalog"`),
			always(`, current_database() AS "name"`),
			always(`, '' AS "owner"`),
			always(`, CAST(NULL AS string) AS "comment"`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "always empty: the metastore has no level above a database"},
			{Name: "name", Desc: "from current_database(), which is the database the session is using"},
			{Name: "owner", Desc: "always empty: Schemas carries the owner"},
			{Name: "comment", Desc: "always absent: Schemas carries the comment"},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Schema, error) {
			var v dbmeta.Schema
			err := rows.Scan(&v.Catalog, &v.Name, &v.Owner, &v.Comment)
			return v, err
		},
	})

	dbmeta.CurrentUser.Register(dbmeta.Hive, &dbmeta.Binding[dbmeta.User]{
		Stmt: dbmeta.Stmt{
			always(`SELECT current_user() AS "name"`),
			always(`, current_user() AS "session"`),
		},
		Fields: []dbmeta.Field{
			{Name: "name", Desc: "from current_user()"},
			{
				Name: "session",
				Desc: "the same as name: Hive does not separate the authenticated" +
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
