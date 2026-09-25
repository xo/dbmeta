package clickhouse

import (
	"database/sql"

	"github.com/xo/dbmeta"
)

func registerRoles() {
	// \du. ClickHouse keeps a user and a role in two tables, and PostgreSQL
	// keeps both in pg_roles, so the two are unioned. The difference between
	// them is whether it can log in, which is what can_login records here.
	dbmeta.Roles.Register(dbmeta.ClickHouse, &dbmeta.Binding[dbmeta.Role]{
		Stmt: dbmeta.Stmt{
			always(`SELECT u.name AS "name"`),
			always(`, 0 AS "superuser"`),
			always(`, 0 AS "create_role"`),
			always(`, 0 AS "create_db"`),
			always(`, 1 AS "can_login"`),
			always(`, 0 AS "replication"`),
			always(`, 0 AS "bypass_rls"`),
			always(`, 1 AS "inherit"`),
			always(`, -1 AS "conn_limit"`),
			always(`, NULL AS "valid_until"`),
			always(`, arrayStringConcat(u.default_roles_list, ', ') AS "member_of"`),
			always(`, NULL AS "comment"`),
			always(`FROM system.users u`),
			always(`WHERE (@name = '' OR u.name LIKE @name)`),
			always(`UNION ALL`),
			always(`SELECT r.name`),
			always(`, 0`),
			always(`, 0`),
			always(`, 0`),
			always(`, 0`),
			always(`, 0`),
			always(`, 0`),
			always(`, 1`),
			always(`, -1`),
			always(`, NULL`),
			always(`, ''`),
			always(`, NULL`),
			always(`FROM system.roles r`),
			always(`WHERE (@name = '' OR r.name LIKE @name)`),
			always(`ORDER BY 1`),
		},
		Fields: []dbmeta.Field{
			{Name: "name"},
			{
				Name: "superuser",
				Desc: "always false: ClickHouse has no superuser flag and grants" +
					" ACCESS MANAGEMENT instead, which the privileges query returns",
			},
			{Name: "create_role", Desc: "always false: it is a grant rather than a flag"},
			{Name: "create_db", Desc: "always false, for the same reason"},
			{Name: "can_login", Desc: "true for a user and false for a role, which is the difference between them"},
			{Name: "replication", Desc: "always false: replication is a table engine here"},
			{Name: "bypass_rls", Desc: "always false: a row policy is bypassed by a grant, not a flag"},
			{Name: "inherit", Desc: "always true: a granted role is always inherited"},
			{Name: "conn_limit", Desc: "always -1: a limit is set by a quota rather than on the user"},
			{
				Name: "valid_until",
				Desc: "always absent: system.users records an expiry and it is for the" +
					" authentication method rather than the account",
			},
			{Name: "member_of", Desc: "the default roles of a user, and empty for a role"},
			{Name: "comment", Desc: "always absent: a user carries no comment"},
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

	// One row per role granted to a user or to another role.
	dbmeta.RoleGrants.Register(dbmeta.ClickHouse, &dbmeta.Binding[dbmeta.RoleGrant]{
		Stmt: dbmeta.Stmt{
			always(`SELECT ifNull(g.user_name, ifNull(g.role_name, '')) AS "role"`),
			always(`, g.granted_role_name AS "member_of"`),
			always(`, NULL AS "grantor"`),
			always(`, g.with_admin_option AS "admin"`),
			always(`, 1 AS "inherit"`),
			always(`, g.granted_role_is_default AS "set"`),
			always(`FROM system.role_grants g`),
			always(`WHERE (@name = '' OR g.granted_role_name LIKE @name)`),
			always(`ORDER BY 1, 2`),
		},
		Fields: []dbmeta.Field{
			{
				Name: "role",
				Desc: "who was granted it, which is a user or another role:" +
					" system.role_grants keeps the two in separate columns",
			},
			{Name: "member_of", Desc: "the role that was granted"},
			{Name: "grantor", Desc: "always absent: ClickHouse records no grantor"},
			{Name: "admin", Desc: "whether it was granted WITH ADMIN OPTION"},
			{Name: "inherit", Desc: "always true: a granted role is always inherited"},
			{
				Name: "set",
				Desc: "whether the role is a default one, which is the nearest" +
					" thing to being set at login",
			},
		},
		Params: []dbmeta.Param{
			{Name: "name", Desc: "granted role name pattern, empty for every one", Default: ""},
		},
		Scan: func(rows *sql.Rows) (dbmeta.RoleGrant, error) {
			var v dbmeta.RoleGrant
			err := rows.Scan(&v.Role, &v.MemberOf, &v.Grantor, &v.Admin,
				&v.Inherit, &v.Set)
			return v, err
		},
	})

	// \dp. One row per object, with the grants on it folded into one list.
	dbmeta.Privileges.Register(dbmeta.ClickHouse, &dbmeta.Binding[dbmeta.Privilege]{
		Stmt: dbmeta.Stmt{
			// Every identity column here is Nullable and access_type is an
			// Enum16, so each is made a plain string before it is joined.
			// system.grants keeps the grantee in two columns, one for a user
			// and one for a role, and exactly one of them is set.
			always(`SELECT ifNull(g.database, '') AS "schema"`),
			always(`, ifNull(g.table, '') AS "name"`),
			always(`, 'table' AS "type"`),
			always(`, arrayStringConcat(groupArray(concat(` +
				`ifNull(g.user_name, ifNull(g.role_name, '')), '=', toString(g.access_type))` +
				`), ', ') AS "access"`),
			always(`, arrayStringConcat(groupArray(ifNull(g.column, '')), ', ')` +
				` AS "column_access"`),
			always(`, '' AS "policies"`),
			always(`FROM system.grants g`),
			always(`WHERE g.database IS NOT NULL`),
			always(`AND ` + notSystem("g.database")),
			always(`AND (@schema = '' OR g.database LIKE @schema)`),
			always(`AND (@name = '' OR ifNull(g.table, '') LIKE @name)`),
			always(`GROUP BY g.database, g.table`),
			always(`ORDER BY g.database, g.table`),
		},
		Fields: []dbmeta.Field{
			{Name: "schema"},
			{Name: "name", Desc: "the table, and empty for a grant on the whole database"},
			{Name: "type", Desc: "always table: ClickHouse grants on a database, a table or a column"},
			{Name: "access", Desc: "the grants as grantee=privilege, one list per object"},
			{Name: "column_access", Desc: "the columns named by a column level grant, where there is one"},
			{
				Name: "policies",
				Desc: "always empty: a ClickHouse row policy is in system.row_policies" +
					" and reaching it needs a second statement",
			},
		},
		Params: schemaNameSystem("table"),
		Scan: func(rows *sql.Rows) (dbmeta.Privilege, error) {
			var v dbmeta.Privilege
			err := rows.Scan(&v.Schema, &v.Name, &v.Type, &v.Access,
				&v.ColumnAccess, &v.Policies)
			return v, err
		},
	})

	// The session settings. system.server_settings holds the ones a restart
	// changes and this holds the ones a query can.
	dbmeta.Settings.Register(dbmeta.ClickHouse, &dbmeta.Binding[dbmeta.Setting]{
		Stmt: dbmeta.Stmt{
			always(`SELECT s.name AS "name"`),
			always(`, s.value AS "value"`),
			always(`, s.type AS "type"`),
			always(`, if(s.readonly = 0, 'user', 'internal') AS "context"`),
			always(`, if(s.readonly = 0, 'rw', 'r') AS "access"`),
			always(`FROM system.settings s`),
			always(`WHERE (@name = '' OR s.name LIKE @name)`),
			always(`ORDER BY s.name`),
		},
		Fields: []dbmeta.Field{
			{Name: "name"}, {Name: "value"},
			{Name: "type", Desc: "the declared type of the setting"},
			{Name: "context", Desc: "user for a setting a session can change, internal otherwise"},
			{Name: "access", Desc: "rw for a setting a session can change, r otherwise"},
		},
		Params: []dbmeta.Param{
			{Name: "name", Desc: "setting name pattern, empty for every one", Default: ""},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Setting, error) {
			var v dbmeta.Setting
			err := rows.Scan(&v.Name, &v.Value, &v.Type, &v.Context, &v.Access)
			return v, err
		},
	})

	// \df and \da. One table holds both and is_aggregate separates them.
	dbmeta.Functions.Register(dbmeta.ClickHouse, &dbmeta.Binding[dbmeta.Function]{
		Stmt:   functionStmt("0"),
		Fields: functionFields("function"),
		Params: []dbmeta.Param{
			{Name: "name", Desc: "function name pattern, empty for every one", Default: ""},
		},
		Scan: scanFunction,
	})
	dbmeta.Aggregates.Register(dbmeta.ClickHouse, &dbmeta.Binding[dbmeta.Function]{
		Stmt:   functionStmt("1"),
		Fields: functionFields("aggregate"),
		Params: []dbmeta.Param{
			{Name: "name", Desc: "aggregate name pattern, empty for every one", Default: ""},
		},
		Scan: scanFunction,
	})

	// \db. A storage policy names the volumes and disks a table's parts live
	// on, which is what a tablespace is for.
	dbmeta.Tablespaces.Register(dbmeta.ClickHouse, &dbmeta.Binding[dbmeta.Tablespace]{
		Stmt: dbmeta.Stmt{
			always(`SELECT p.policy_name AS "name"`),
			always(`, '' AS "owner"`),
			always(`, arrayStringConcat(p.disks, ', ') AS "location"`),
			always(`, concat('volume=', p.volume_name, ', type=', p.volume_type) AS "options"`),
			always(`, '' AS "size"`),
			always(`, NULL AS "access"`),
			always(`, NULL AS "comment"`),
			always(`FROM system.storage_policies p`),
			always(`WHERE (@name = '' OR p.policy_name LIKE @name)`),
			always(`ORDER BY p.policy_name, p.volume_priority`),
		},
		Fields: []dbmeta.Field{
			{Name: "name", Desc: "the storage policy, once per volume in it"},
			{Name: "owner", Desc: "always empty: a policy records no owner"},
			{Name: "location", Desc: "the disks the volume is made of"},
			{Name: "options", Desc: "the volume name and its type"},
			{
				Name: "size",
				Desc: "always empty: a size would have to sum system.disks, which is" +
					" a second statement",
			},
			{Name: "access", Desc: "always absent: a policy is not grantable"},
			{Name: "comment", Desc: "always absent: a policy carries no comment"},
		},
		Params: []dbmeta.Param{
			{Name: "name", Desc: "policy name pattern, empty for every one", Default: ""},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Tablespace, error) {
			var v dbmeta.Tablespace
			err := rows.Scan(&v.Name, &v.Owner, &v.Location, &v.Options, &v.Size,
				&v.Access, &v.Comment)
			return v, err
		},
	})

	// \det. A table whose engine reads somebody else's data is a foreign
	// table: the statement reads it like any other and the rows are held
	// elsewhere.
	dbmeta.ForeignTables.Register(dbmeta.ClickHouse, &dbmeta.Binding[dbmeta.ForeignTable]{
		Stmt: dbmeta.Stmt{
			always(`SELECT t.database AS "schema"`),
			always(`, t.name AS "name"`),
			always(`, t.engine AS "server"`),
			always(`, nullIf(t.engine_full, '') AS "options"`),
			always(`, nullIf(t.comment, '') AS "comment"`),
			always(`FROM system.tables t`),
			always(`WHERE t.engine IN (` + foreignEngines + `)`),
			always(`AND ` + notSystem("t.database")),
			always(`AND (@schema = '' OR t.database LIKE @schema)`),
			always(`AND (@name = '' OR t.name LIKE @name)`),
			always(`ORDER BY t.database, t.name`),
		},
		Fields: []dbmeta.Field{
			{Name: "schema"}, {Name: "name"},
			{
				Name: "server",
				Desc: "the table engine, which names the kind of remote rather than" +
					" a server object: ClickHouse has no CREATE SERVER",
			},
			{Name: "options", Desc: "the engine clause as written, which holds the connection"},
			{Name: "comment"},
		},
		Params: schemaNameSystem("table"),
		Scan: func(rows *sql.Rows) (dbmeta.ForeignTable, error) {
			var v dbmeta.ForeignTable
			err := rows.Scan(&v.Schema, &v.Name, &v.Server, &v.Options, &v.Comment)
			return v, err
		},
	})

	// \des. A named collection is a stored set of connection details that a
	// table engine or a table function refers to by name, which is what a
	// foreign server is.
	dbmeta.ForeignServers.Register(dbmeta.ClickHouse, &dbmeta.Binding[dbmeta.ForeignServer]{
		Stmt: dbmeta.Stmt{
			always(`SELECT n.name AS "name"`),
			always(`, '' AS "owner"`),
			always(`, 'named collection' AS "wrapper"`),
			always(`, NULL AS "type"`),
			always(`, NULL AS "version"`),
			always(`, NULL AS "access"`),
			always(`, nullIf(n.create_query, '') AS "options"`),
			always(`, NULL AS "comment"`),
			always(`FROM system.named_collections n`),
			always(`WHERE (@name = '' OR n.name LIKE @name)`),
			always(`ORDER BY n.name`),
		},
		Fields: []dbmeta.Field{
			{Name: "name"},
			{Name: "owner", Desc: "always empty: a named collection records no owner"},
			{
				Name: "wrapper",
				Desc: "always named collection: ClickHouse has one mechanism where" +
					" PostgreSQL has a wrapper per kind of remote",
			},
			{Name: "type", Desc: "always absent: a named collection has no type"},
			{Name: "version", Desc: "always absent: it records no version"},
			{Name: "access", Desc: "always absent: a grant on it is in the privileges query"},
			{
				Name: "options",
				Desc: "the CREATE NAMED COLLECTION as written. A key marked NOT" +
					" OVERRIDABLE is shown and a secret is masked by the server",
			},
			{Name: "comment", Desc: "always absent: a named collection carries no comment"},
		},
		Params: []dbmeta.Param{
			{Name: "name", Desc: "collection name pattern, empty for every one", Default: ""},
		},
		Scan: func(rows *sql.Rows) (dbmeta.ForeignServer, error) {
			var v dbmeta.ForeignServer
			err := rows.Scan(&v.Name, &v.Owner, &v.Wrapper, &v.Type, &v.Version,
				&v.Access, &v.Options, &v.Comment)
			return v, err
		},
	})

	// The database a statement resolves an unqualified name in.
	dbmeta.CurrentSchema.Register(dbmeta.ClickHouse, &dbmeta.Binding[dbmeta.Schema]{
		Stmt: dbmeta.Stmt{
			always(`SELECT '' AS "catalog"`),
			always(`, currentDatabase() AS "name"`),
			always(`, '' AS "owner"`),
			always(`, NULL AS "comment"`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "always empty: ClickHouse has nothing above a database"},
			{Name: "name"},
			{Name: "owner", Desc: "always empty: a database records no owner"},
			{Name: "comment", Desc: "always absent: reading it would need a second statement"},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Schema, error) {
			var v dbmeta.Schema
			err := rows.Scan(&v.Catalog, &v.Name, &v.Owner, &v.Comment)
			return v, err
		},
	})

	// Who the connection is.
	dbmeta.CurrentUser.Register(dbmeta.ClickHouse, &dbmeta.Binding[dbmeta.User]{
		Stmt: dbmeta.Stmt{
			always(`SELECT currentUser() AS "name"`),
			always(`, currentUser() AS "session"`),
		},
		Fields: []dbmeta.Field{
			{Name: "name"},
			{
				Name: "session",
				Desc: "the same as the name: ClickHouse has no SET ROLE that changes" +
					" who the statement runs as",
			},
		},
		Scan: func(rows *sql.Rows) (dbmeta.User, error) {
			var v dbmeta.User
			err := rows.Scan(&v.Name, &v.Session)
			return v, err
		},
	})
}

// functionStmt builds the statement behind both \df and \da.
func functionStmt(aggregate string) dbmeta.Stmt {
	return dbmeta.Stmt{
		always(`SELECT '' AS "catalog"`),
		always(`, '' AS "schema"`),
		always(`, f.name AS "name"`),
		always(`, NULL AS "id"`),
		always(`, if(f.is_aggregate, 'aggregate', 'function') AS "kind"`),
		always(`, '' AS "result_type"`),
		always(`, '' AS "arg_types"`),
		// system.functions.deterministic arrived with system.constraints and
		// is absent on 25.8, so the older side pads. Both sides return one
		// text column, which is what rule 3 asks.
		dbmeta.Choice{
			{Query: `, '' AS "volatility"`},
			{Min: v268, Query: `, if(f.deterministic, 'immutable', 'volatile') AS "volatility"`},
		},
		always(`, '' AS "parallel"`),
		always(`, '' AS "owner"`),
		always(`, '' AS "security"`),
		always(`, NULL AS "access"`),
		always(`, f.origin AS "language"`),
		always(`, nullIf(f.create_query, '') AS "source"`),
		always(`, nullIf(f.description, '') AS "comment"`),
		always(`FROM system.functions f`),
		always(`WHERE f.is_aggregate = ` + aggregate),
		always(`AND (@name = '' OR f.name LIKE @name)`),
		always(`ORDER BY f.name`),
	}
}

// functionFields describes what both routine queries return.
func functionFields(kind string) []dbmeta.Field {
	return []dbmeta.Field{
		{Name: "catalog", Desc: "always empty: a " + kind + " belongs to the server"},
		{Name: "schema", Desc: "always empty, for the same reason"},
		{Name: "name"},
		{Name: "id", Desc: "always absent: a " + kind + " has no id in the catalog"},
		{Name: "kind"},
		{
			Name: "result_type",
			Desc: "always empty: a ClickHouse " + kind + " is overloaded across" +
				" types and the catalog records no one return type",
		},
		{Name: "arg_types", Desc: "always empty, for the same reason as result_type"},
		{
			Name: "volatility", Min: v268,
			Desc: "immutable for a deterministic function and volatile otherwise," +
				" and empty before 26.8 where system.functions records no" +
				" determinism",
		},
		{Name: "parallel", Desc: "always empty: ClickHouse has no parallel safety mark"},
		{Name: "owner", Desc: "always empty: a built in function has no owner"},
		{Name: "security", Desc: "always empty: ClickHouse has no SECURITY DEFINER"},
		{Name: "access", Desc: "always absent: a grant on a function is in the privileges query"},
		{
			Name: "language",
			Desc: "where it came from: System for a built in one and one of the" +
				" user defined origins otherwise",
		},
		{Name: "source", Desc: "the CREATE FUNCTION for a user defined one, absent for a built in"},
		{Name: "comment", Desc: "the server's own description of the " + kind},
	}
}

// scanFunction reads one row of either routine query.
func scanFunction(rows *sql.Rows) (dbmeta.Function, error) {
	var v dbmeta.Function
	err := rows.Scan(&v.Catalog, &v.Schema, &v.Name, &v.ID, &v.Kind, &v.ResultType,
		&v.ArgTypes, &v.Volatility, &v.Parallel, &v.Owner, &v.Security, &v.Access,
		&v.Language, &v.Source, &v.Comment)
	return v, err
}
