package sqlserver

import (
	"database/sql"

	"github.com/xo/dbmeta"
)

// Routines, types, security, storage and the rest of the catalog.

func registerServer() {
	registerRoutines()
	registerTypes()
	registerSecurity()
	registerStorage()
}

// routineStmt builds the statement behind \df and \da. kindFilter narrows to
// one set of sys.objects types, or is empty for every routine.
func routineStmt(kindFilter string) dbmeta.Stmt {
	kinds := `'FN', 'IF', 'TF', 'FS', 'FT', 'AF', 'P', 'PC'`
	if kindFilter != "" {
		kinds = kindFilter
	}
	return dbmeta.Stmt{
		always(`SELECT DB_NAME() AS "catalog"`),
		always(`, s.name AS "schema"`),
		always(`, o.name AS "name"`),
		always(`, CAST(o.object_id AS varchar(20)) AS "id"`),
		always(`, CASE o.type WHEN 'AF' THEN 'agg' WHEN 'P' THEN 'proc' WHEN 'PC' THEN 'proc'` +
			` WHEN 'IF' THEN 'func' WHEN 'TF' THEN 'func' WHEN 'FT' THEN 'func'` +
			` ELSE 'func' END AS "kind"`),
		// The return of a function is a parameter with id 0, which is how
		// SQL Server records it.
		always(`, COALESCE((SELECT ty.name FROM sys.parameters pr` +
			` JOIN sys.types ty ON ty.user_type_id = pr.user_type_id` +
			` WHERE pr.object_id = o.object_id AND pr.parameter_id = 0), '') AS "result_type"`),
		always(`, COALESCE(STUFF((SELECT ', ' + ty.name FROM sys.parameters pr` +
			` JOIN sys.types ty ON ty.user_type_id = pr.user_type_id` +
			` WHERE pr.object_id = o.object_id AND pr.parameter_id > 0` +
			` ORDER BY pr.parameter_id FOR XML PATH('')), 1, 2, ''), '') AS "arg_types"`),
		always(`, '' AS "volatility"`),
		always(`, '' AS "parallel"`),
		always(`, COALESCE(p.name, '') AS "owner"`),
		always(`, CASE WHEN m.execute_as_principal_id IS NULL THEN 'invoker' ELSE 'definer' END AS "security"`),
		always(`, NULL AS "access"`),
		always(`, CASE WHEN o.type IN ('FS', 'FT', 'PC') THEN 'clr' ELSE 'sql' END AS "language"`),
		always(`, m.definition AS "source"`),
		always(`, ` + commentOn("o.object_id") + ` AS "comment"`),
		always(`FROM sys.objects o`),
		always(`JOIN sys.schemas s ON s.schema_id = o.schema_id`),
		always(`LEFT JOIN sys.sql_modules m ON m.object_id = o.object_id`),
		always(`LEFT JOIN sys.database_principals p ON p.principal_id = s.principal_id`),
		always(`WHERE o.type IN (` + kinds + `)`),
		always(`AND ` + notSystem),
		always(`AND (@schema = '' OR s.name LIKE @schema)`),
		always(`AND (@name = '' OR o.name LIKE @name)`),
		always(`ORDER BY 2, 3`),
	}
}

func routineFields() []dbmeta.Field {
	return []dbmeta.Field{
		{Name: "catalog"}, {Name: "schema"}, {Name: "name"},
		{Name: "id", Desc: "the object id, which RoutineParameters joins on"},
		{
			Name: "kind",
			Desc: "func for a function of any shape, proc for a procedure, agg for a CLR aggregate",
		},
		{Name: "result_type", Desc: "the return type, which SQL Server records as the parameter with id 0"},
		{Name: "arg_types", Desc: "the parameter types joined. RoutineParameters has them as rows"},
		{Name: "volatility", Desc: "always empty: SQL Server records no volatility"},
		{Name: "parallel", Desc: "always empty: SQL Server has no parallel safety marking"},
		{Name: "owner", Desc: "the principal that owns the schema, because a routine has no separate owner"},
		{Name: "security", Desc: "definer where the routine has an EXECUTE AS, and invoker otherwise"},
		{Name: "access", Desc: "always absent: read privileges instead"},
		{Name: "language", Desc: "sql, or clr for a routine backed by an assembly"},
		{
			Name: "source",
			Desc: "the CREATE statement, absent for a CLR routine and for one created WITH ENCRYPTION",
		},
		{Name: "comment"},
	}
}

func scanRoutine(rows *sql.Rows) (dbmeta.Function, error) {
	var v dbmeta.Function
	err := rows.Scan(&v.Catalog, &v.Schema, &v.Name, &v.ID, &v.Kind, &v.ResultType,
		&v.ArgTypes, &v.Volatility, &v.Parallel, &v.Owner, &v.Security, &v.Access,
		&v.Language, &v.Source, &v.Comment)
	return v, err
}

func registerRoutines() {
	dbmeta.Functions.Register(dbmeta.SQLServer, &dbmeta.Binding[dbmeta.Function]{
		Stmt:   routineStmt(""),
		Fields: routineFields(),
		Params: schemaNameSystem("routine"),
		Scan:   scanRoutine,
	})

	// \da. AF is a CLR aggregate, which is the only kind of aggregate a caller
	// can create here: the built in ones are not objects.
	dbmeta.Aggregates.Register(dbmeta.SQLServer, &dbmeta.Binding[dbmeta.Function]{
		Stmt:   routineStmt(`'AF'`),
		Fields: routineFields(),
		Params: schemaNameSystem("aggregate"),
		Scan:   scanRoutine,
	})

	dbmeta.RoutineParameters.Register(dbmeta.SQLServer, &dbmeta.Binding[dbmeta.RoutineParameter]{
		Stmt: dbmeta.Stmt{
			always(`SELECT DB_NAME() AS "catalog"`),
			always(`, s.name AS "schema"`),
			always(`, o.name AS "routine"`),
			always(`, CAST(o.object_id AS varchar(20)) AS "routine_id"`),
			always(`, NULLIF(pr.name, '') AS "name"`),
			always(`, CAST(pr.parameter_id AS bigint) AS "ordinal"`),
			always(`, CASE WHEN pr.parameter_id = 0 THEN 'return'` +
				` WHEN pr.is_output = 1 THEN 'out' ELSE 'in' END AS "mode"`),
			always(`, ty.name AS "data_type"`),
			always(`, CASE WHEN pr.has_default_value = 1` +
				` THEN CAST(pr.default_value AS nvarchar(max)) ELSE NULL END AS "default"`),
			always(`FROM sys.parameters pr`),
			always(`JOIN sys.objects o ON o.object_id = pr.object_id`),
			always(`JOIN sys.schemas s ON s.schema_id = o.schema_id`),
			always(`JOIN sys.types ty ON ty.user_type_id = pr.user_type_id`),
			always(`WHERE ` + notSystem),
			always(`AND (@schema = '' OR s.name LIKE @schema)`),
			always(`AND (@parent = '' OR o.name LIKE @parent)`),
			always(`AND (@name = '' OR COALESCE(pr.name, '') LIKE @name)`),
			always(`ORDER BY 2, 3, 6`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog"}, {Name: "schema"}, {Name: "routine"},
			{Name: "routine_id", Desc: "the object id, the same value Function.ID carries"},
			{Name: "name", Desc: "absent for the row that describes a function's return value"},
			{Name: "ordinal", Desc: "one based, and zero for a function's return value"},
			{Name: "mode", Desc: "in, out, or return for a function's return value"},
			{Name: "data_type"},
			{
				Name: "default",
				Desc: "the default, which SQL Server records only for a CLR routine and leaves absent for a T-SQL one",
			},
		},
		Params: schemaParentName("parameter"),
		Scan: func(rows *sql.Rows) (dbmeta.RoutineParameter, error) {
			var v dbmeta.RoutineParameter
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Routine, &v.RoutineID, &v.Name,
				&v.Ordinal, &v.Mode, &v.DataType, &v.Default)
			return v, err
		},
	})

	// \dy. A DDL trigger fires on a statement that changes the schema, which
	// is what an event trigger is. parent_class 0 is the database.
	dbmeta.EventTriggers.Register(dbmeta.SQLServer, &dbmeta.Binding[dbmeta.EventTrigger]{
		Stmt: dbmeta.Stmt{
			always(`SELECT tr.name AS "name"`),
			always(`, COALESCE(STUFF((SELECT ', ' + te.type_desc FROM sys.trigger_events te` +
				` WHERE te.object_id = tr.object_id ORDER BY te.type` +
				` FOR XML PATH('')), 1, 2, ''), '') AS "event"`),
			always(`, '' AS "owner"`),
			always(`, CASE WHEN tr.is_disabled = 1 THEN 'disabled' ELSE 'enabled' END AS "enabled"`),
			always(`, tr.name AS "function"`),
			always(`, '' AS "tags"`),
			always(`, ` + commentOn("tr.object_id") + ` AS "comment"`),
			always(`FROM sys.triggers tr`),
			always(`WHERE tr.parent_class = 0`),
			always(`AND (@with_system = 1 OR tr.is_ms_shipped = 0)`),
			always(`AND (@name = '' OR tr.name LIKE @name)`),
			always(`ORDER BY 1`),
		},
		Fields: []dbmeta.Field{
			{Name: "name"},
			{
				Name: "event",
				Desc: "the statements the trigger fires on, joined. SQL Server allows many where PostgreSQL allows one",
			},
			{Name: "owner", Desc: "always empty: a DDL trigger has no separate owner"},
			{Name: "enabled", Desc: "enabled or disabled"},
			{
				Name: "function",
				Desc: "the trigger's own name: a SQL Server trigger carries its body rather than calling a function",
			},
			{Name: "tags", Desc: "always empty: SQL Server has no tag filter on a DDL trigger"},
			{Name: "comment"},
		},
		Params: []dbmeta.Param{
			{Name: "name", Desc: "trigger name pattern, empty for every one", Default: ""},
			{Name: "with_system", Desc: "include the triggers SQL Server ships", Default: false},
		},
		Scan: func(rows *sql.Rows) (dbmeta.EventTrigger, error) {
			var v dbmeta.EventTrigger
			err := rows.Scan(&v.Name, &v.Event, &v.Owner, &v.Enabled, &v.Function,
				&v.Tags, &v.Comment)
			return v, err
		},
	})
}

func registerTypes() {
	// \dT. Every type, built in and user defined alike.
	dbmeta.Types.Register(dbmeta.SQLServer, &dbmeta.Binding[dbmeta.Type]{
		Stmt: dbmeta.Stmt{
			always(`SELECT DB_NAME() AS "catalog"`),
			always(`, COALESCE(s.name, 'sys') AS "schema"`),
			always(`, ty.name AS "name"`),
			always(`, ty.name AS "internal"`),
			always(`, CASE WHEN ty.is_table_type = 1 THEN 'table'` +
				` WHEN ty.is_assembly_type = 1 THEN 'assembly'` +
				` WHEN ty.is_user_defined = 1 THEN 'alias' ELSE 'base' END AS "kind"`),
			always(`, '' AS "elements"`),
			always(`, COALESCE(p.name, '') AS "owner"`),
			always(`, NULL AS "access"`),
			always(`, NULL AS "comment"`),
			always(`FROM sys.types ty`),
			always(`LEFT JOIN sys.schemas s ON s.schema_id = ty.schema_id`),
			always(`LEFT JOIN sys.database_principals p ON p.principal_id = ty.principal_id`),
			always(`WHERE (@with_system = 1 OR ty.is_user_defined = 1)`),
			always(`AND (@schema = '' OR COALESCE(s.name, 'sys') LIKE @schema)`),
			always(`AND (@name = '' OR ty.name LIKE @name)`),
			always(`ORDER BY 2, 3`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog"}, {Name: "schema"}, {Name: "name"},
			{Name: "internal", Desc: "the same as the name: SQL Server has no separate internal name"},
			{Name: "kind", Desc: "base, alias for a user defined type, table for a table type, assembly for a CLR type"},
			{
				Name: "elements",
				Desc: "always empty: SQL Server has no enumerated type, so there is nothing to list",
			},
			{Name: "owner"},
			{Name: "access", Desc: "always absent: read privileges instead"},
			{Name: "comment", Desc: "always absent: an extended property on a type is class 6 and is not read here"},
		},
		Params: schemaNameSystem("type"),
		Scan: func(rows *sql.Rows) (dbmeta.Type, error) {
			var v dbmeta.Type
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Name, &v.Internal, &v.Kind,
				&v.Elements, &v.Owner, &v.Access, &v.Comment)
			return v, err
		},
	})

	// \dD. An alias type is a base type with a length and a nullability, which
	// is what a domain is without the check.
	dbmeta.Domains.Register(dbmeta.SQLServer, &dbmeta.Binding[dbmeta.Domain]{
		Stmt: dbmeta.Stmt{
			always(`SELECT DB_NAME() AS "catalog"`),
			always(`, s.name AS "schema"`),
			always(`, ty.name AS "name"`),
			always(`, bt.name AS "data_type"`),
			always(`, COALESCE(ty.collation_name, '') AS "collation"`),
			always(`, ty.is_nullable AS "nullable"`),
			always(`, NULL AS "default"`),
			always(`, '' AS "constraints"`),
			always(`, NULL AS "access"`),
			always(`, NULL AS "comment"`),
			always(`FROM sys.types ty`),
			always(`JOIN sys.schemas s ON s.schema_id = ty.schema_id`),
			always(`LEFT JOIN sys.types bt ON bt.user_type_id = ty.system_type_id`),
			always(`WHERE ty.is_user_defined = 1 AND ty.is_table_type = 0 AND ty.is_assembly_type = 0`),
			always(`AND ` + notSystem),
			always(`AND (@schema = '' OR s.name LIKE @schema)`),
			always(`AND (@name = '' OR ty.name LIKE @name)`),
			always(`ORDER BY 2, 3`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog"}, {Name: "schema"}, {Name: "name"},
			{Name: "data_type", Desc: "the base type the alias is built on"},
			{Name: "collation", Desc: "empty where the type is not text"},
			{Name: "nullable"},
			{
				Name: "default",
				Desc: "always absent: a default was bound with sp_bindefault, which is removed from current releases",
			},
			{
				Name: "constraints",
				Desc: "always empty: SQL Server has no CHECK on an alias type, which is the difference from a domain",
			},
			{Name: "access", Desc: "always absent: read privileges instead"},
			{Name: "comment", Desc: "always absent"},
		},
		Params: schemaNameSystem("domain"),
		Scan: func(rows *sql.Rows) (dbmeta.Domain, error) {
			var v dbmeta.Domain
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Name, &v.DataType, &v.Collation,
				&v.Nullable, &v.Default, &v.Constraints, &v.Access, &v.Comment)
			return v, err
		},
	})

	// \dO. fn_helpcollations is a function rather than a view and it is read
	// in a FROM clause like one.
	dbmeta.Collations.Register(dbmeta.SQLServer, &dbmeta.Binding[dbmeta.Collation]{
		Stmt: dbmeta.Stmt{
			always(`SELECT '' AS "schema"`),
			always(`, c.name AS "name"`),
			always(`, NULL AS "provider"`),
			always(`, c.name AS "collate"`),
			always(`, c.name AS "ctype"`),
			always(`, NULL AS "locale"`),
			always(`, CAST(1 AS bit) AS "deterministic"`),
			always(`, c.description AS "comment"`),
			always(`FROM sys.fn_helpcollations() c`),
			always(`WHERE (@name = '' OR c.name LIKE @name)`),
			always(`ORDER BY 2`),
		},
		Fields: []dbmeta.Field{
			{Name: "schema", Desc: "always empty: a collation belongs to the server"},
			{Name: "name"},
			{Name: "provider", Desc: "always absent: SQL Server names no provider"},
			{Name: "collate"}, {Name: "ctype"},
			{Name: "locale", Desc: "always absent: the name carries the locale"},
			{Name: "deterministic", Desc: "always true"},
			{Name: "comment", Desc: "the description, which says what the collation does"},
		},
		Params: []dbmeta.Param{
			{Name: "name", Desc: "collation name pattern, empty for every one", Default: ""},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Collation, error) {
			var v dbmeta.Collation
			err := rows.Scan(&v.Schema, &v.Name, &v.Provider, &v.Collate, &v.CType,
				&v.Locale, &v.Deterministic, &v.Comment)
			return v, err
		},
	})

	// \dd. SQL Server keeps comments in a catalog rather than on the object,
	// which is the shape psql expects and which no other model here has.
	dbmeta.Comments.Register(dbmeta.SQLServer, &dbmeta.Binding[dbmeta.Comment]{
		Stmt: dbmeta.Stmt{
			always(`SELECT s.name AS "schema"`),
			always(`, CASE WHEN p.minor_id = 0 THEN o.name` +
				` ELSE o.name + '.' + COALESCE(c.name, '') END AS "name"`),
			always(`, CASE WHEN p.minor_id > 0 THEN 'column'` +
				` WHEN o.type = 'U' THEN 'table' WHEN o.type = 'V' THEN 'view'` +
				` WHEN o.type = 'SO' THEN 'sequence' WHEN o.type IN ('P', 'PC') THEN 'procedure'` +
				` WHEN o.type IN ('FN', 'IF', 'TF', 'FS', 'FT') THEN 'function'` +
				` WHEN o.type = 'TR' THEN 'trigger' ELSE LOWER(o.type_desc) END AS "type"`),
			always(`, CAST(p.value AS nvarchar(max)) AS "comment"`),
			always(`FROM sys.extended_properties p`),
			always(`JOIN sys.objects o ON o.object_id = p.major_id`),
			always(`JOIN sys.schemas s ON s.schema_id = o.schema_id`),
			always(`LEFT JOIN sys.columns c` +
				` ON c.object_id = p.major_id AND c.column_id = p.minor_id`),
			always(`WHERE p.class = 1 AND p.name = 'MS_Description'`),
			always(`AND ` + notSystem),
			always(`AND (@schema = '' OR s.name LIKE @schema)`),
			always(`AND (@name = '' OR o.name LIKE @name)`),
			always(`ORDER BY 1, 3, 2`),
		},
		Fields: []dbmeta.Field{
			{Name: "schema"},
			{Name: "name", Desc: "the object, and table.column for a column"},
			{Name: "type", Desc: "table, view, column, sequence, procedure, function or trigger"},
			{
				Name: "comment",
				Desc: "the MS_Description extended property. SQL Server has no COMMENT ON, so a database where nobody ran sp_addextendedproperty has none",
			},
		},
		Params: schemaNameSystem("object"),
		Scan: func(rows *sql.Rows) (dbmeta.Comment, error) {
			var v dbmeta.Comment
			err := rows.Scan(&v.Schema, &v.Name, &v.Type, &v.Comment)
			return v, err
		},
	})
}

func registerSecurity() {
	// \du and \dg. A SQL Server user and a database role are both principals,
	// which is the same shape MariaDB has.
	dbmeta.Roles.Register(dbmeta.SQLServer, &dbmeta.Binding[dbmeta.Role]{
		Stmt: dbmeta.Stmt{
			always(`SELECT p.name AS "name"`),
			always(`, CAST(CASE WHEN p.name = 'dbo' THEN 1 ELSE 0 END AS bit) AS "superuser"`),
			always(`, CAST(0 AS bit) AS "create_role"`),
			always(`, CAST(0 AS bit) AS "create_db"`),
			always(`, CAST(CASE WHEN p.type IN ('S', 'U', 'G', 'E', 'X')` +
				` THEN 1 ELSE 0 END AS bit) AS "can_login"`),
			always(`, CAST(0 AS bit) AS "replication"`),
			always(`, CAST(0 AS bit) AS "bypass_rls"`),
			always(`, CAST(1 AS bit) AS "inherit"`),
			always(`, CAST(-1 AS bigint) AS "conn_limit"`),
			always(`, NULL AS "valid_until"`),
			always(`, '' AS "member_of"`),
			always(`, NULL AS "comment"`),
			always(`FROM sys.database_principals p`),
			always(`WHERE (@with_system = 1 OR p.is_fixed_role = 0)`),
			always(`AND (@name = '' OR p.name LIKE @name)`),
			always(`ORDER BY 1`),
		},
		Fields: []dbmeta.Field{
			{Name: "name", Desc: "a database user or a database role: SQL Server calls both a principal"},
			{
				Name: "superuser",
				Desc: "true for dbo, which owns the database. A server level sysadmin is not visible from inside one database",
			},
			{Name: "create_role", Desc: "always false: read privileges for CREATE ROLE instead"},
			{Name: "create_db", Desc: "always false: creating a database is a server level permission"},
			{Name: "can_login", Desc: "true for a principal backed by a login, and false for a role"},
			{Name: "replication", Desc: "always false: replication is configured outside the database"},
			{Name: "bypass_rls", Desc: "always false: read sys.security_policies for row level security"},
			{Name: "inherit", Desc: "always true: SQL Server has no other behaviour"},
			{Name: "conn_limit", Desc: "always -1: SQL Server limits connections per server rather than per principal"},
			{Name: "valid_until", Desc: "always absent: expiry belongs to the login, outside the database"},
			{Name: "member_of", Desc: "always empty: read role_grants instead"},
			{Name: "comment", Desc: "always absent"},
		},
		Params: []dbmeta.Param{
			{Name: "name", Desc: "principal name pattern, empty for every one", Default: ""},
			{Name: "with_system", Desc: "include the fixed database roles", Default: false},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Role, error) {
			var v dbmeta.Role
			err := rows.Scan(&v.Name, &v.Superuser, &v.CreateRole, &v.CreateDB, &v.CanLogin,
				&v.Replication, &v.BypassRLS, &v.Inherit, &v.ConnLimit, &v.ValidUntil,
				&v.MemberOf, &v.Comment)
			return v, err
		},
	})

	dbmeta.RoleGrants.Register(dbmeta.SQLServer, &dbmeta.Binding[dbmeta.RoleGrant]{
		Stmt: dbmeta.Stmt{
			always(`SELECT m.name AS "role"`),
			always(`, r.name AS "member_of"`),
			always(`, NULL AS "grantor"`),
			always(`, CAST(0 AS bit) AS "admin"`),
			always(`, CAST(1 AS bit) AS "inherit"`),
			always(`, CAST(1 AS bit) AS "set"`),
			always(`FROM sys.database_role_members rm`),
			always(`JOIN sys.database_principals r ON r.principal_id = rm.role_principal_id`),
			always(`JOIN sys.database_principals m ON m.principal_id = rm.member_principal_id`),
			always(`WHERE (@name = '' OR m.name LIKE @name)`),
			always(`ORDER BY 1, 2`),
		},
		Fields: []dbmeta.Field{
			{Name: "role", Desc: "the member"},
			{Name: "member_of", Desc: "the role it belongs to"},
			{Name: "grantor", Desc: "always absent: SQL Server does not record who granted a membership"},
			{Name: "admin", Desc: "always false: SQL Server has no admin option on a membership"},
			{Name: "inherit", Desc: "always true"},
			{Name: "set", Desc: "always true"},
		},
		Params: []dbmeta.Param{
			{Name: "name", Desc: "member name pattern, empty for every one", Default: ""},
		},
		Scan: func(rows *sql.Rows) (dbmeta.RoleGrant, error) {
			var v dbmeta.RoleGrant
			err := rows.Scan(&v.Role, &v.MemberOf, &v.Grantor, &v.Admin, &v.Inherit, &v.Set)
			return v, err
		},
	})

	// \dp and \z. One row per grant, which is the shape MariaDB has too.
	dbmeta.Privileges.Register(dbmeta.SQLServer, &dbmeta.Binding[dbmeta.Privilege]{
		Stmt: dbmeta.Stmt{
			always(`SELECT s.name AS "schema"`),
			always(`, o.name AS "name"`),
			always(`, LOWER(o.type_desc) AS "type"`),
			// COLLATE DATABASE_DEFAULT on every piece. A catalog name carries
			// the server collation and a literal carries the database one, so
			// concatenating them raises "cannot resolve collation conflict"
			// on any server where the two differ, which is the default.
			always(`, CAST(g.name AS nvarchar(256)) COLLATE DATABASE_DEFAULT + '='` +
				` + CAST(dp.permission_name AS nvarchar(256)) COLLATE DATABASE_DEFAULT` +
				` + ' (' + CAST(dp.state_desc AS nvarchar(64)) COLLATE DATABASE_DEFAULT` +
				` + ')' AS "access"`),
			always(`, CASE WHEN dp.minor_id > 0 THEN c.name ELSE NULL END AS "column_access"`),
			always(`, NULL AS "policies"`),
			always(`FROM sys.database_permissions dp`),
			always(`JOIN sys.objects o ON o.object_id = dp.major_id`),
			always(`JOIN sys.schemas s ON s.schema_id = o.schema_id`),
			always(`JOIN sys.database_principals g ON g.principal_id = dp.grantee_principal_id`),
			always(`LEFT JOIN sys.columns c` +
				` ON c.object_id = dp.major_id AND c.column_id = dp.minor_id`),
			always(`WHERE dp.class = 1`),
			always(`AND ` + notSystem),
			always(`AND (@schema = '' OR s.name LIKE @schema)`),
			always(`AND (@name = '' OR o.name LIKE @name)`),
			always(`ORDER BY 1, 2`),
		},
		Fields: []dbmeta.Field{
			{Name: "schema"}, {Name: "name"},
			{Name: "type", Desc: "what the grant is on, such as user_table or view"},
			{
				Name: "access",
				Desc: "one grant per row as grantee=permission (state), where the state is GRANT, DENY or GRANT_WITH_GRANT_OPTION",
			},
			{Name: "column_access", Desc: "the column, where the grant is on one rather than the whole object"},
			{Name: "policies", Desc: "always absent: read sys.security_policies for row level security"},
		},
		Params: schemaNameSystem("object"),
		Scan: func(rows *sql.Rows) (dbmeta.Privilege, error) {
			var v dbmeta.Privilege
			err := rows.Scan(&v.Schema, &v.Name, &v.Type, &v.Access, &v.ColumnAccess, &v.Policies)
			return v, err
		},
	})
}

func registerStorage() {
	// \db. A filegroup is a named place a table is stored in, which is what a
	// tablespace is. SQL Server puts it inside a database rather than beside
	// one, and that is the only difference that matters here.
	dbmeta.Tablespaces.Register(dbmeta.SQLServer, &dbmeta.Binding[dbmeta.Tablespace]{
		Stmt: dbmeta.Stmt{
			always(`SELECT f.name AS "name"`),
			always(`, '' AS "owner"`),
			always(`, COALESCE((SELECT TOP 1 mf.physical_name FROM sys.database_files mf` +
				` WHERE mf.data_space_id = f.data_space_id ORDER BY mf.file_id), '') AS "location"`),
			always(`, CASE WHEN f.is_default = 1 THEN 'default' ELSE NULL END AS "options"`),
			always(`, '' AS "size"`),
			always(`, NULL AS "comment"`),
			always(`FROM sys.filegroups f`),
			always(`WHERE (@name = '' OR f.name LIKE @name)`),
			always(`ORDER BY 1`),
		},
		Fields: []dbmeta.Field{
			{Name: "name"},
			{Name: "owner", Desc: "always empty: a filegroup has no owner"},
			{
				Name: "location",
				Desc: "the path of the first file in the group. A filegroup may hold several",
			},
			{Name: "options", Desc: "default for the filegroup a table without a clause goes to"},
			{Name: "size", Desc: "always empty: a size needs the file sizes summed"},
			{Name: "comment", Desc: "always absent"},
		},
		Params: []dbmeta.Param{
			{Name: "name", Desc: "filegroup name pattern, empty for every one", Default: ""},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Tablespace, error) {
			var v dbmeta.Tablespace
			err := rows.Scan(&v.Name, &v.Owner, &v.Location, &v.Options, &v.Size, &v.Comment)
			return v, err
		},
	})

	// \dP. A partitioned table is one whose clustered index sits on a
	// partition scheme rather than on a filegroup.
	dbmeta.PartitionedTables.Register(dbmeta.SQLServer, &dbmeta.Binding[dbmeta.PartitionedTable]{
		Stmt: dbmeta.Stmt{
			always(`SELECT s.name AS "schema"`),
			always(`, t.name AS "name"`),
			always(`, 'table' AS "type"`),
			always(`, '' AS "parent"`),
			always(`, CASE WHEN pf.boundary_value_on_right = 1 THEN 'range right'` +
				` ELSE 'range left' END AS "strategy"`),
			always(`, ps.name AS "expression"`),
			always(`, ` + commentOn("t.object_id") + ` AS "comment"`),
			always(`FROM sys.tables t`),
			always(`JOIN sys.schemas s ON s.schema_id = t.schema_id`),
			always(`JOIN sys.indexes i ON i.object_id = t.object_id AND i.index_id IN (0, 1)`),
			always(`JOIN sys.partition_schemes ps ON ps.data_space_id = i.data_space_id`),
			always(`JOIN sys.partition_functions pf ON pf.function_id = ps.function_id`),
			always(`WHERE ` + notSystem),
			always(`AND (@schema = '' OR s.name LIKE @schema)`),
			always(`AND (@name = '' OR t.name LIKE @name)`),
			always(`ORDER BY 1, 2`),
		},
		Fields: []dbmeta.Field{
			{Name: "schema"}, {Name: "name"},
			{Name: "type", Desc: "always table: SQL Server partitions an index with its table"},
			{
				Name: "parent",
				Desc: "always empty: SQL Server has no partition hierarchy, so a partition is not a table of its own",
			},
			{Name: "strategy", Desc: "range left or range right, which is where a boundary value falls"},
			{Name: "expression", Desc: "the partition scheme, which names the function and the filegroups"},
			{Name: "comment"},
		},
		Params: schemaNameSystem("table"),
		Scan: func(rows *sql.Rows) (dbmeta.PartitionedTable, error) {
			var v dbmeta.PartitionedTable
			err := rows.Scan(&v.Schema, &v.Name, &v.Type, &v.Parent, &v.Strategy,
				&v.Expression, &v.Comment)
			return v, err
		},
	})

	// \dconfig. Server wide configuration. The database scoped settings are a
	// different view with a different shape and are not unioned in, because
	// the two disagree about what a value is.
	dbmeta.Settings.Register(dbmeta.SQLServer, &dbmeta.Binding[dbmeta.Setting]{
		Stmt: dbmeta.Stmt{
			always(`SELECT c.name AS "name"`),
			always(`, CAST(c.value_in_use AS nvarchar(max)) AS "value"`),
			always(`, CASE WHEN c.is_dynamic = 1 THEN 'dynamic' ELSE 'static' END AS "type"`),
			always(`, CASE WHEN c.is_advanced = 1 THEN 'advanced' ELSE 'basic' END AS "context"`),
			always(`, NULL AS "access"`),
			always(`FROM sys.configurations c`),
			always(`WHERE (@name = '' OR c.name LIKE @name)`),
			always(`ORDER BY 1`),
		},
		Fields: []dbmeta.Field{
			{Name: "name"},
			{Name: "value", Desc: "the value in use, which differs from the configured value until a reconfigure"},
			{Name: "type", Desc: "dynamic where a change takes effect at once, static where it needs a restart"},
			{Name: "context", Desc: "advanced where the setting is hidden unless show advanced options is on"},
			{Name: "access", Desc: "always absent"},
		},
		Params: []dbmeta.Param{
			{Name: "name", Desc: "setting name pattern, empty for every setting", Default: ""},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Setting, error) {
			var v dbmeta.Setting
			err := rows.Scan(&v.Name, &v.Value, &v.Type, &v.Context, &v.Access)
			return v, err
		},
	})

	registerForeign()
	registerStats()

	// The one product here where the two names always differ. A connection
	// authenticates as a server login and acts as a database user, so sa acts
	// as dbo. usql reads neither: it shows the login it connected with,
	// because ALTER LOGIN is what it changes.
	dbmeta.CurrentUser.Register(dbmeta.SQLServer, &dbmeta.Binding[dbmeta.User]{
		Stmt: dbmeta.Stmt{
			always(`SELECT CAST(CURRENT_USER AS nvarchar(128)) AS "name"`),
			always(`, CAST(SUSER_NAME() AS nvarchar(128)) AS "session"`),
		},
		Fields: []dbmeta.Field{
			{Name: "name", Desc: "the database user the session acts as"},
			{Name: "session", Desc: "the server login the connection authenticated as"},
		},
		Scan: func(rows *sql.Rows) (dbmeta.User, error) {
			var v dbmeta.User
			err := rows.Scan(&v.Name, &v.Session)
			return v, err
		},
	})

	dbmeta.CurrentSchema.Register(dbmeta.SQLServer, &dbmeta.Binding[dbmeta.Schema]{
		Stmt: dbmeta.Stmt{
			always(`SELECT DB_NAME() AS "catalog"`),
			always(`, s.name AS "name"`),
			always(`, COALESCE(p.name, '') AS "owner"`),
			always(`, NULL AS "comment"`),
			always(`FROM sys.schemas s`),
			always(`LEFT JOIN sys.database_principals p ON p.principal_id = s.principal_id`),
			always(`WHERE s.name = SCHEMA_NAME()`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "the database in use"},
			{
				Name: "name",
				Desc: "what SCHEMA_NAME() returns, which is the schema an unqualified name resolves in now. It is not always the principal's default_schema_name",
			},
			{Name: "owner"},
			{Name: "comment", Desc: "always absent"},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Schema, error) {
			var v dbmeta.Schema
			err := rows.Scan(&v.Catalog, &v.Name, &v.Owner, &v.Comment)
			return v, err
		},
	})
}

// registerForeign backs \des, \deu and \det.
//
// A linked server is what SQL Server has instead of a foreign server, and it
// is the closest thing any database here has to the SQL/MED shape: a named
// remote, a credential mapping per local login, and tables read through it.
//
// Both queries read a server level catalog. A caller without VIEW ANY
// DEFINITION sees no rows rather than an error, which is how SQL Server
// answers everywhere.
func registerForeign() {
	dbmeta.ForeignServers.Register(dbmeta.SQLServer, &dbmeta.Binding[dbmeta.ForeignServer]{
		Stmt: dbmeta.Stmt{
			always(`SELECT sv.name AS "name"`),
			always(`, '' AS "owner"`),
			always(`, sv.provider AS "wrapper"`),
			always(`, NULLIF(sv.product, '') AS "type"`),
			always(`, NULL AS "version"`),
			always(`, NULL AS "access"`),
			always(`, NULLIF(sv.data_source, '') AS "options"`),
			always(`, NULL AS "comment"`),
			always(`FROM sys.servers sv`),
			always(`WHERE sv.is_linked = 1`),
			always(`AND (@name = '' OR sv.name LIKE @name)`),
			always(`ORDER BY 1`),
		},
		Fields: []dbmeta.Field{
			{Name: "name"},
			{Name: "owner", Desc: "always empty: a linked server has no owner"},
			{Name: "wrapper", Desc: "the OLE DB provider, which is what SQL Server has instead of a wrapper"},
			{Name: "type", Desc: "the product name the linked server was declared with"},
			{Name: "version", Desc: "always absent: SQL Server records no remote version"},
			{Name: "access", Desc: "always absent"},
			{Name: "options", Desc: "the data source, which is the address of the remote"},
			{Name: "comment", Desc: "always absent"},
		},
		Params: []dbmeta.Param{
			{Name: "name", Desc: "server name pattern, empty for every one", Default: ""},
		},
		Scan: func(rows *sql.Rows) (dbmeta.ForeignServer, error) {
			var v dbmeta.ForeignServer
			err := rows.Scan(&v.Name, &v.Owner, &v.Wrapper, &v.Type, &v.Version,
				&v.Access, &v.Options, &v.Comment)
			return v, err
		},
	})

	// \deu. A linked login maps a local principal to a remote credential,
	// which is what a user mapping is. Row zero is the mapping that applies to
	// everybody, which PostgreSQL writes as PUBLIC.
	dbmeta.UserMappings.Register(dbmeta.SQLServer, &dbmeta.Binding[dbmeta.UserMapping]{
		Stmt: dbmeta.Stmt{
			always(`SELECT sv.name AS "server"`),
			always(`, NULLIF(ll.remote_name, '') AS "name"`),
			always(`, CASE WHEN ll.uses_self_credential = 1` +
				` THEN 'uses the local credential' ELSE NULL END AS "options"`),
			always(`FROM sys.linked_logins ll`),
			always(`JOIN sys.servers sv ON sv.server_id = ll.server_id`),
			always(`WHERE sv.is_linked = 1`),
			always(`AND (@name = '' OR sv.name LIKE @name)`),
			always(`ORDER BY 1, 2`),
		},
		Fields: []dbmeta.Field{
			{Name: "server"},
			{
				Name: "name",
				Desc: "the remote login. Absent where the mapping uses the local credential rather than naming one",
			},
			{
				Name: "options",
				Desc: "says so where the mapping passes the local credential through",
			},
		},
		Params: []dbmeta.Param{
			{Name: "name", Desc: "server name pattern, empty for every one", Default: ""},
		},
		Scan: func(rows *sql.Rows) (dbmeta.UserMapping, error) {
			var v dbmeta.UserMapping
			err := rows.Scan(&v.Server, &v.Name, &v.Options)
			return v, err
		},
	})

	// \det. An external table reads data SQL Server does not hold, through
	// PolyBase. The catalog view exists in every install and is empty until
	// PolyBase is configured, which is a fact rather than a gap.
	// sys.external_tables arrived in 2016 along with PolyBase, so the whole
	// statement is gated: an older server has no external table to report.
	dbmeta.ForeignTables.Register(dbmeta.SQLServer, &dbmeta.Binding[dbmeta.ForeignTable]{
		Stmt: dbmeta.Stmt{
			{{Min: v13, SQL: `SELECT s.name AS "schema"`}},
			{{Min: v13, SQL: `, t.name AS "name"`}},
			{{Min: v13, SQL: `, COALESCE(ds.name, '') AS "server"`}},
			{{Min: v13, SQL: `, NULLIF(t.location, '') AS "options"`}},
			{{Min: v13, SQL: `, ` + commentOn("t.object_id") + ` AS "comment"`}},
			{{Min: v13, SQL: `FROM sys.external_tables t`}},
			{{Min: v13, SQL: `JOIN sys.schemas s ON s.schema_id = t.schema_id`}},
			{{Min: v13, SQL: `LEFT JOIN sys.external_data_sources ds` +
				` ON ds.data_source_id = t.data_source_id`}},
			{{Min: v13, SQL: `WHERE ` + notSystem}},
			{{Min: v13, SQL: `AND (@schema = '' OR s.name LIKE @schema)`}},
			{{Min: v13, SQL: `AND (@name = '' OR t.name LIKE @name)`}},
			{{Min: v13, SQL: `ORDER BY 1, 2`}},
		},
		Fields: []dbmeta.Field{
			{Name: "schema", Min: v13}, {Name: "name", Min: v13},
			{
				Name: "server", Min: v13,
				Desc: "the external data source, which is what SQL Server has instead of a server",
			},
			{Name: "options", Min: v13, Desc: "the location within the data source"},
			{Name: "comment", Min: v13},
		},
		Params: schemaNameSystem("table"),
		Scan: func(rows *sql.Rows) (dbmeta.ForeignTable, error) {
			var v dbmeta.ForeignTable
			err := rows.Scan(&v.Schema, &v.Name, &v.Server, &v.Options, &v.Comment)
			return v, err
		},
	})
}

// registerStats backs \ss and \dX.
//
// sys.dm_db_stats_properties is a function rather than a view, and CROSS APPLY
// calls it once per statistics object inside the one statement. It needs VIEW
// STATISTICS on the object, and a caller without it sees no rows.
//
// It arrived in SQL Server 2012, which is release 11.
func registerStats() {
	dbmeta.ColumnStats.Register(dbmeta.SQLServer, &dbmeta.Binding[dbmeta.ColumnStat]{
		Stmt: dbmeta.Stmt{
			{{Min: v11, SQL: `SELECT DB_NAME() AS "catalog"`}},
			{{Min: v11, SQL: `, s.name AS "schema"`}},
			{{Min: v11, SQL: `, o.name AS "table"`}},
			{{Min: v11, SQL: `, c.name AS "name"`}},
			{{Min: v11, SQL: `, NULL AS "avg_width"`}},
			{{Min: v11, SQL: `, NULL AS "null_frac"`}},
			{{Min: v11, SQL: `, CAST(sp.rows AS float) AS "distinct"`}},
			{{Min: v11, SQL: `, NULL AS "min"`}},
			{{Min: v11, SQL: `, NULL AS "max"`}},
			{{Min: v11, SQL: `, NULL AS "mean"`}},
			{{Min: v11, SQL: `, NULL AS "top_n"`}},
			{{Min: v11, SQL: `, NULL AS "top_n_freqs"`}},
			{{Min: v11, SQL: `FROM sys.stats st`}},
			{{Min: v11, SQL: `CROSS APPLY sys.dm_db_stats_properties(st.object_id, st.stats_id) sp`}},
			{{Min: v11, SQL: `JOIN sys.stats_columns sc` +
				` ON sc.object_id = st.object_id AND sc.stats_id = st.stats_id`}},
			{{Min: v11, SQL: `JOIN sys.objects o ON o.object_id = st.object_id`}},
			{{Min: v11, SQL: `JOIN sys.schemas s ON s.schema_id = o.schema_id`}},
			{{Min: v11, SQL: `JOIN sys.columns c` +
				` ON c.object_id = sc.object_id AND c.column_id = sc.column_id`}},
			{{Min: v11, SQL: `WHERE ` + notSystem}},
			{{Min: v11, SQL: `AND (@schema = '' OR s.name LIKE @schema)`}},
			{{Min: v11, SQL: `AND (@parent = '' OR o.name LIKE @parent)`}},
			{{Min: v11, SQL: `AND (@name = '' OR c.name LIKE @name)`}},
			{{Min: v11, SQL: `ORDER BY 2, 3, 4`}},
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Min: v13}, {Name: "schema", Min: v13},
			{Name: "table", Min: v13}, {Name: "name", Min: v13},
			{
				Name: "avg_width", Min: v11,
				Desc: "always absent: the width is in the histogram, which needs DBCC SHOW_STATISTICS and a second statement",
			},
			{Name: "null_frac", Min: v11, Desc: "always absent, for the same reason"},
			{
				Name: "distinct", Min: v11,
				Desc: "the rows the statistics were built from, which is not a distinct count. SQL Server keeps the distinct count in the histogram",
			},
			{Name: "min", Min: v11, Desc: "always absent: the bounds are in the histogram"},
			{Name: "max", Min: v11, Desc: "always absent, for the same reason"},
			{Name: "mean", Min: v11, Desc: "always absent: SQL Server computes no mean"},
			{Name: "top_n", Min: v11, Desc: "always absent: the common values are in the histogram"},
			{Name: "top_n_freqs", Min: v11, Desc: "always absent, for the same reason"},
		},
		Params: schemaParentName("column"),
		Scan: func(rows *sql.Rows) (dbmeta.ColumnStat, error) {
			var v dbmeta.ColumnStat
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Table, &v.Name, &v.AvgWidth,
				&v.NullFrac, &v.Distinct, &v.Min, &v.Max, &v.Mean, &v.TopN, &v.TopNFreqs)
			return v, err
		},
	})

	// \dX. A statistics object over more than one column is what
	// CREATE STATISTICS makes, and it is the same idea PostgreSQL has.
	dbmeta.ExtendedStats.Register(dbmeta.SQLServer, &dbmeta.Binding[dbmeta.ExtendedStat]{
		Stmt: dbmeta.Stmt{
			always(`SELECT s.name AS "schema"`),
			always(`, st.name AS "name"`),
			always(`, '' AS "owner"`),
			always(`, o.name AS "table"`),
			// The columns the statistics cover, joined, because the kind has
			// no field for them and psql prints them inside the kinds line.
			always(`, 'ndistinct over ' + COALESCE(STUFF((SELECT ', ' + c.name` +
				` FROM sys.stats_columns sc2` +
				` JOIN sys.columns c ON c.object_id = sc2.object_id AND c.column_id = sc2.column_id` +
				` WHERE sc2.object_id = st.object_id AND sc2.stats_id = st.stats_id` +
				` ORDER BY sc2.stats_column_id FOR XML PATH('')), 1, 2, ''), '') AS "kinds"`),
			always(`, ` + commentOn("st.object_id") + ` AS "comment"`),
			always(`FROM sys.stats st`),
			always(`JOIN sys.objects o ON o.object_id = st.object_id`),
			always(`JOIN sys.schemas s ON s.schema_id = o.schema_id`),
			// more than one column is what makes it extended rather than a
			// single column statistic
			always(`WHERE (SELECT COUNT(*) FROM sys.stats_columns sc` +
				` WHERE sc.object_id = st.object_id AND sc.stats_id = st.stats_id) > 1`),
			always(`AND ` + notSystem),
			always(`AND (@schema = '' OR s.name LIKE @schema)`),
			always(`AND (@name = '' OR st.name LIKE @name)`),
			always(`ORDER BY 1, 2`),
		},
		Fields: []dbmeta.Field{
			{Name: "schema"}, {Name: "name"},
			{Name: "owner", Desc: "always empty: a statistics object has no owner"},
			{Name: "table"},
			{
				Name: "kinds",
				Desc: "ndistinct over the columns it covers, in order. SQL Server builds one kind of multi column statistic and does not name it, so the columns go here, which is where psql prints them",
			},
			{Name: "comment"},
		},
		Params: schemaNameSystem("statistics object"),
		Scan: func(rows *sql.Rows) (dbmeta.ExtendedStat, error) {
			var v dbmeta.ExtendedStat
			err := rows.Scan(&v.Schema, &v.Name, &v.Owner, &v.Table, &v.Kinds, &v.Comment)
			return v, err
		},
	})
}
