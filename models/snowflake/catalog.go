package snowflake

import (
	"database/sql"

	"github.com/xo/dbmeta"
)

func always(query string) dbmeta.Choice { return dbmeta.Choice{{Query: query}} }

// like is the filter that matches col to the pattern in param, and passes
// when the pattern is empty.
func like(col, param string) string {
	return `(` + param + ` = '' OR ` + col + ` LIKE ` + param + `)`
}

// notSystem hides INFORMATION_SCHEMA, the one schema Snowflake keeps in every
// database, unless the caller asks for it.
func notSystem(col string) string {
	return `(@with_system OR ` + col + ` <> 'INFORMATION_SCHEMA')`
}

func schemaNameSystem(kind string) []dbmeta.Param {
	return []dbmeta.Param{
		{Name: "schema", Desc: "schema name pattern, empty for every schema", Default: ""},
		{Name: "name", Desc: kind + " name pattern, empty for every " + kind, Default: ""},
		{Name: "with_system", Desc: "include INFORMATION_SCHEMA", Default: false},
	}
}

func childParams(parent, kind string) []dbmeta.Param {
	return append([]dbmeta.Param{
		{Name: "parent", Desc: parent + " name pattern, empty for every " + parent, Default: ""},
	}, schemaNameSystem(kind)...)
}

// tableType is the word for the kind of the table t.
const tableType = `CASE t.table_type WHEN 'BASE TABLE' THEN` +
	` CASE WHEN t.is_transient = 'YES' THEN 'transient table' ELSE 'table' END` +
	` ELSE LOWER(t.table_type) END`

func register() {
	registerRelations()
	registerRoutines()
}

func registerRelations() {
	dbmeta.Schemas.Register(dbmeta.Snowflake, &dbmeta.Binding[dbmeta.Schema]{
		Stmt: dbmeta.Stmt{
			always(`SELECT s.catalog_name AS "catalog"`),
			always(`, s.schema_name AS "name"`),
			always(`, s.schema_owner AS "owner"`),
			always(`, s.comment AS "comment"`),
			always(`FROM information_schema.schemata s`),
			always(`WHERE ` + notSystem("s.schema_name")),
			always(`AND ` + like("s.schema_name", "@name")),
			always(`ORDER BY 2`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog"}, {Name: "name"},
			{Name: "owner", Desc: "the role that owns the schema, and empty for INFORMATION_SCHEMA"},
			{Name: "comment"},
		},
		Params: []dbmeta.Param{
			{Name: "name", Desc: "schema name pattern, empty for every schema", Default: ""},
			{Name: "with_system", Desc: "include INFORMATION_SCHEMA", Default: false},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Schema, error) {
			var v dbmeta.Schema
			err := rows.Scan(&v.Catalog, &v.Name, dbmeta.NullAsEmpty(&v.Owner), &v.Comment)
			return v, err
		},
	})

	dbmeta.Databases.Register(dbmeta.Snowflake, &dbmeta.Binding[dbmeta.Database]{
		Stmt: dbmeta.Stmt{
			always(`SELECT d.database_name AS "name"`),
			always(`, d.database_owner AS "owner"`),
			always(`, '' AS "encoding"`),
			always(`, '' AS "collate"`),
			always(`, '' AS "ctype"`),
			always(`, NULL AS "access"`),
			always(`, NULL AS "tablespace"`),
			always(`, NULL AS "size"`),
			always(`, d.comment AS "comment"`),
			always(`FROM information_schema.databases d`),
			always(`WHERE ` + like("d.database_name", "@name")),
			always(`ORDER BY 1`),
		},
		Fields: []dbmeta.Field{
			{Name: "name"}, {Name: "owner"},
			{Name: "encoding", Desc: "always empty: every string is UTF-8"},
			{Name: "collate", Desc: "always empty: a database records no collation"},
			{Name: "ctype", Desc: "always empty: a database records no collation"},
			{Name: "access", Desc: "always absent: a grant on a database is not in information_schema"},
			{Name: "tablespace", Desc: "always absent: Snowflake has no tablespace"},
			{Name: "size", Desc: "always absent: information_schema records no size"},
			{Name: "comment"},
		},
		Params: []dbmeta.Param{
			{Name: "name", Desc: "database name pattern, empty for every database", Default: ""},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Database, error) {
			var v dbmeta.Database
			err := rows.Scan(&v.Name, dbmeta.NullAsEmpty(&v.Owner), &v.Encoding, &v.Collate, &v.CType,
				&v.Access, &v.Tablespace, &v.Size, &v.Comment)
			return v, err
		},
	})

	dbmeta.Tables.Register(dbmeta.Snowflake, &dbmeta.Binding[dbmeta.Table]{
		Stmt: dbmeta.Stmt{
			always(`SELECT t.table_catalog AS "catalog"`),
			always(`, t.table_schema AS "schema"`),
			always(`, t.table_name AS "name"`),
			always(`, ` + tableType + ` AS "type"`),
			always(`, t.comment AS "comment"`),
			always(`FROM information_schema.tables t`),
			always(`WHERE ` + notSystem("t.table_schema")),
			always(`AND ` + like("t.table_schema", "@schema")),
			always(`AND ` + like("t.table_name", "@name")),
			always(`AND (@types = '' OR ` + dbmeta.InList(`@types`, tableType) + `)`),
			always(`ORDER BY 2, 3`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog"}, {Name: "schema"}, {Name: "name"},
			{Name: "type", Desc: "table, transient table, view, materialized view, external table or event table, from table_type"},
			{Name: "comment"},
		},
		Params: append(schemaNameSystem("table"), dbmeta.TypesParam()),
		Scan: func(rows *sql.Rows) (dbmeta.Table, error) {
			var v dbmeta.Table
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Name, &v.Type, &v.Comment)
			return v, err
		},
	})

	// information_schema has no KEY_COLUMN_USAGE, so which columns a key
	// holds is only in SHOW PRIMARY KEYS, and primary_key is false.
	dbmeta.Columns.Register(dbmeta.Snowflake, &dbmeta.Binding[dbmeta.Column]{
		Stmt: dbmeta.Stmt{
			always(`SELECT c.table_catalog AS "catalog"`),
			always(`, c.table_schema AS "schema"`),
			always(`, c.table_name AS "table"`),
			always(`, c.column_name AS "name"`),
			always(`, c.ordinal_position AS "ordinal"`),
			always(`, c.data_type AS "data_type"`),
			always(`, c.is_nullable = 'YES' AS "nullable"`),
			always(`, c.column_default AS "default"`),
			always(`, FALSE AS "primary_key"`),
			always(`, CASE WHEN c.is_identity = 'YES' THEN 'by default' END AS "identity"`),
			always(`, NULL AS "generated"`),
			always(`, c.comment AS "comment"`),
			always(`, c.collation_name AS "collation"`),
			always(`FROM information_schema.columns c`),
			always(`WHERE ` + notSystem("c.table_schema")),
			always(`AND ` + like("c.table_schema", "@schema")),
			always(`AND ` + like("c.table_name", "@parent")),
			always(`AND ` + like("c.column_name", "@name")),
			always(`ORDER BY 2, 3, 5`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog"}, {Name: "schema"}, {Name: "table"}, {Name: "name"},
			{Name: "ordinal"},
			{Name: "data_type", Desc: "the type as Snowflake writes it, such as NUMBER or TEXT"},
			{Name: "nullable"}, {Name: "default"},
			{Name: "primary_key", Desc: "always false: information_schema has no KEY_COLUMN_USAGE, and SHOW PRIMARY KEYS is not a SELECT"},
			{Name: "identity", Desc: "by default for an IDENTITY or AUTOINCREMENT column, which accepts a value of its own"},
			{Name: "generated", Desc: "always absent: Snowflake has no generated column"},
			{Name: "comment"},
			{Name: "collation", Desc: "the COLLATE of a text column, and absent where it has none"},
		},
		Params: childParams("table", "column"),
		Scan: func(rows *sql.Rows) (dbmeta.Column, error) {
			var v dbmeta.Column
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Table, &v.Name, &v.Ordinal,
				&v.DataType, &v.Nullable, &v.Default, &v.PrimaryKey, &v.Identity,
				&v.Generated, &v.Comment, &v.Collation)
			return v, err
		},
	})

	// A key is declared and not enforced, except NOT NULL.
	dbmeta.Constraints.Register(dbmeta.Snowflake, &dbmeta.Binding[dbmeta.Constraint]{
		Stmt: dbmeta.Stmt{
			always(`SELECT k.table_schema AS "schema"`),
			always(`, k.table_name AS "table"`),
			always(`, k.constraint_name AS "name"`),
			always(`, LOWER(k.constraint_type) AS "type"`),
			always(`, NULL AS "definition"`),
			always(`, k.is_deferrable = 'YES' AS "deferrable"`),
			always(`, k.initially_deferred = 'YES' AS "deferred"`),
			always(`, k.comment AS "comment"`),
			always(`FROM information_schema.table_constraints k`),
			always(`WHERE ` + notSystem("k.table_schema")),
			always(`AND ` + like("k.table_schema", "@schema")),
			always(`AND ` + like("k.table_name", "@parent")),
			always(`AND ` + like("k.constraint_name", "@name")),
			always(`ORDER BY 1, 2, 3`),
		},
		Fields: []dbmeta.Field{
			{Name: "schema"}, {Name: "table"}, {Name: "name"},
			{Name: "type", Desc: "primary key, unique or foreign key. Snowflake declares them and enforces none"},
			{Name: "definition", Desc: "always absent: information_schema records no key's columns"},
			{Name: "deferrable"}, {Name: "deferred"}, {Name: "comment"},
		},
		Params: childParams("table", "constraint"),
		Scan: func(rows *sql.Rows) (dbmeta.Constraint, error) {
			var v dbmeta.Constraint
			err := rows.Scan(&v.Schema, &v.Table, &v.Name, &v.Type, &v.Definition,
				&v.Deferrable, &v.Deferred, &v.Comment)
			return v, err
		},
	})

	dbmeta.Views.Register(dbmeta.Snowflake, &dbmeta.Binding[dbmeta.View]{
		Stmt: dbmeta.Stmt{
			always(`SELECT v.table_catalog AS "catalog"`),
			always(`, v.table_schema AS "schema"`),
			always(`, v.table_name AS "name"`),
			always(`, v.view_definition AS "definition"`),
			always(`, NULL AS "check_option"`),
			always(`, v.is_updatable = 'YES' AS "updatable"`),
			always(`, v.insertable_into = 'YES' AS "insertable"`),
			always(`, v.comment AS "comment"`),
			always(`FROM information_schema.views v`),
			always(`WHERE ` + notSystem("v.table_schema")),
			always(`AND ` + like("v.table_schema", "@schema")),
			always(`AND ` + like("v.table_name", "@name")),
			always(`ORDER BY 2, 3`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog"}, {Name: "schema"}, {Name: "name"},
			{Name: "definition", Desc: "the CREATE VIEW statement, which is absent for a secure view the role does not own"},
			{Name: "check_option", Desc: "always absent: Snowflake has no WITH CHECK OPTION"},
			{Name: "updatable"}, {Name: "insertable"}, {Name: "comment"},
		},
		Params: schemaNameSystem("view"),
		Scan: func(rows *sql.Rows) (dbmeta.View, error) {
			var v dbmeta.View
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Name, &v.Definition, &v.CheckOption,
				&v.Updatable, &v.Insertable, &v.Comment)
			return v, err
		},
	})

	dbmeta.Comments.Register(dbmeta.Snowflake, &dbmeta.Binding[dbmeta.Comment]{
		Stmt: dbmeta.Stmt{
			always(`SELECT t.table_schema AS "schema"`),
			always(`, t.table_name AS "name"`),
			always(`, ` + tableType + ` AS "type"`),
			always(`, t.comment AS "comment"`),
			always(`FROM information_schema.tables t`),
			always(`WHERE t.comment IS NOT NULL`),
			always(`AND ` + notSystem("t.table_schema")),
			always(`AND ` + like("t.table_schema", "@schema")),
			always(`AND ` + like("t.table_name", "@name")),
			always(`ORDER BY 1, 2`),
		},
		Fields: dbmeta.Fields("schema", "name", "type", "comment"),
		Params: schemaNameSystem("object"),
		Scan: func(rows *sql.Rows) (dbmeta.Comment, error) {
			var v dbmeta.Comment
			err := rows.Scan(&v.Schema, &v.Name, &v.Type, &v.Comment)
			return v, err
		},
	})

	dbmeta.Sequences.Register(dbmeta.Snowflake, &dbmeta.Binding[dbmeta.Sequence]{
		Stmt: dbmeta.Stmt{
			always(`SELECT s.sequence_schema AS "schema"`),
			always(`, s.sequence_name AS "name"`),
			always(`, s.data_type AS "data_type"`),
			always(`, CAST(s.start_value AS VARCHAR) AS "start"`),
			always(`, CAST(s.minimum_value AS VARCHAR) AS "minimum"`),
			always(`, CAST(s.maximum_value AS VARCHAR) AS "maximum"`),
			always(`, CAST(s.increment AS VARCHAR) AS "increment"`),
			always(`, s.cycle_option = 'YES' AS "cycles"`),
			always(`, '' AS "owned_by"`),
			always(`, s.comment AS "comment"`),
			always(`FROM information_schema.sequences s`),
			always(`WHERE ` + notSystem("s.sequence_schema")),
			always(`AND ` + like("s.sequence_schema", "@schema")),
			always(`AND ` + like("s.sequence_name", "@name")),
			always(`ORDER BY 1, 2`),
		},
		Fields: []dbmeta.Field{
			{Name: "schema"}, {Name: "name"}, {Name: "data_type"},
			{Name: "start"}, {Name: "minimum"}, {Name: "maximum"}, {Name: "increment"},
			{Name: "cycles"},
			{Name: "owned_by", Desc: "always empty: a sequence belongs to no column"},
			{Name: "comment"},
		},
		Params: schemaNameSystem("sequence"),
		Scan: func(rows *sql.Rows) (dbmeta.Sequence, error) {
			var v dbmeta.Sequence
			err := rows.Scan(&v.Schema, &v.Name, &v.DataType, &v.Start, &v.Minimum,
				&v.Maximum, &v.Increment, &v.Cycles, &v.OwnedBy, &v.Comment)
			return v, err
		},
	})

	dbmeta.Privileges.Register(dbmeta.Snowflake, &dbmeta.Binding[dbmeta.Privilege]{
		Stmt: dbmeta.Stmt{
			always(`SELECT p.table_schema AS "schema"`),
			always(`, p.table_name AS "name"`),
			always(`, 'table' AS "type"`),
			always(`, p.grantee || '=' || p.privilege_type || '/' || p.grantor AS "access"`),
			always(`, NULL AS "column_access"`),
			always(`, NULL AS "policies"`),
			always(`FROM information_schema.table_privileges p`),
			always(`WHERE ` + notSystem("p.table_schema")),
			always(`AND ` + like("p.table_schema", "@schema")),
			always(`AND ` + like("p.table_name", "@name")),
			always(`ORDER BY 1, 2, 4`),
		},
		Fields: []dbmeta.Field{
			{Name: "schema"}, {Name: "name"}, {Name: "type"},
			{Name: "access", Desc: "one grant per row, as grantee=privilege/grantor"},
			{Name: "column_access", Desc: "always absent: Snowflake has no grant on a column"},
			{Name: "policies", Desc: "always absent: a row access policy is not in information_schema"},
		},
		Params: schemaNameSystem("table"),
		Scan: func(rows *sql.Rows) (dbmeta.Privilege, error) {
			var v dbmeta.Privilege
			err := rows.Scan(&v.Schema, &v.Name, &v.Type, &v.Access, &v.ColumnAccess, &v.Policies)
			return v, err
		},
	})

	// The roles granted to the current role and to the roles it holds,
	// which is all information_schema shows.
	dbmeta.RoleGrants.Register(dbmeta.Snowflake, &dbmeta.Binding[dbmeta.RoleGrant]{
		Stmt: dbmeta.Stmt{
			always(`SELECT r.grantee AS "role"`),
			always(`, r.role_name AS "member_of"`),
			always(`, NULL AS "grantor"`),
			always(`, r.is_grantable = 'YES' AS "admin"`),
			always(`, TRUE AS "inherit"`),
			always(`, TRUE AS "set"`),
			always(`FROM information_schema.applicable_roles r`),
			always(`WHERE ` + like("r.role_name", "@name")),
			always(`ORDER BY 1, 2`),
		},
		Fields: []dbmeta.Field{
			{Name: "role", Desc: "the user or role the role was granted to"},
			{Name: "member_of"},
			{Name: "grantor", Desc: "always absent: APPLICABLE_ROLES records no grantor"},
			{Name: "admin", Desc: "whether the grantee may grant the role on"},
			{Name: "inherit", Desc: "always true: a granted role is always inherited"},
			{Name: "set", Desc: "always true: a session can use any role it was granted"},
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

	dbmeta.CurrentSchema.Register(dbmeta.Snowflake, &dbmeta.Binding[dbmeta.Schema]{
		Stmt: dbmeta.Stmt{
			always(`SELECT s.catalog_name AS "catalog"`),
			always(`, s.schema_name AS "name"`),
			always(`, s.schema_owner AS "owner"`),
			always(`, s.comment AS "comment"`),
			always(`FROM information_schema.schemata s`),
			always(`WHERE s.schema_name = CURRENT_SCHEMA()`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog"}, {Name: "name"},
			{Name: "owner", Desc: "the role that owns the schema"},
			{Name: "comment"},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Schema, error) {
			var v dbmeta.Schema
			err := rows.Scan(&v.Catalog, &v.Name, dbmeta.NullAsEmpty(&v.Owner), &v.Comment)
			return v, err
		},
	})

	// The user, and the role the session acts as.
	dbmeta.CurrentUser.Register(dbmeta.Snowflake, &dbmeta.Binding[dbmeta.User]{
		Stmt: dbmeta.Stmt{
			always(`SELECT CURRENT_USER() AS "name"`),
			always(`, CURRENT_ROLE() AS "session"`),
		},
		Fields: []dbmeta.Field{
			{Name: "name"},
			{Name: "session", Desc: "the role the session acts as, which decides what it may do"},
		},
		Scan: func(rows *sql.Rows) (dbmeta.User, error) {
			var v dbmeta.User
			err := rows.Scan(&v.Name, &v.Session)
			return v, err
		},
	})
}

// routineStmt lists the functions, and the procedures beside them. A name is
// overloaded, so the argument signature tells two apart.
func routineStmt() dbmeta.Stmt {
	return dbmeta.Stmt{
		always(`SELECT f.function_catalog AS "catalog"`),
		always(`, f.function_schema AS "schema"`),
		always(`, f.function_name AS "name"`),
		always(`, f.function_name || f.argument_signature AS "id"`),
		always(`, 'func' AS "kind"`),
		always(`, f.data_type AS "result_type"`),
		always(`, f.argument_signature AS "arg_types"`),
		always(`, '' AS "volatility"`),
		always(`, '' AS "parallel"`),
		always(`, f.function_owner AS "owner"`),
		always(`, CASE WHEN f.is_secure = 'YES' THEN 'secure' ELSE '' END AS "security"`),
		always(`, NULL AS "access"`),
		always(`, LOWER(f.function_language) AS "language"`),
		always(`, f.function_definition AS "source"`),
		always(`, f.comment AS "comment"`),
		always(`, NULL AS "definition"`),
		always(`FROM information_schema.functions f`),
		always(`WHERE ` + notSystem("f.function_schema")),
		always(`AND ` + like("f.function_schema", "@schema")),
		always(`AND ` + like("f.function_name", "@name")),
		always(`UNION ALL`),
		always(`SELECT p.procedure_catalog, p.procedure_schema, p.procedure_name,`),
		always(` p.procedure_name || p.argument_signature, 'proc', p.data_type, p.argument_signature,`),
		always(` '', '', p.procedure_owner, '', NULL, LOWER(p.procedure_language), p.procedure_definition, p.comment, NULL`),
		always(`FROM information_schema.procedures p`),
		always(`WHERE ` + notSystem("p.procedure_schema")),
		always(`AND ` + like("p.procedure_schema", "@schema")),
		always(`AND ` + like("p.procedure_name", "@name")),
		always(`ORDER BY 2, 3, 4`),
	}
}

func registerRoutines() {
	dbmeta.Functions.Register(dbmeta.Snowflake, &dbmeta.Binding[dbmeta.Function]{
		Stmt: routineStmt(),
		Fields: []dbmeta.Field{
			{Name: "catalog"}, {Name: "schema"}, {Name: "name"},
			{Name: "id", Desc: "the name and the argument signature, because a name is overloaded"},
			{Name: "kind", Desc: "func or proc"},
			{Name: "result_type"},
			{Name: "arg_types", Desc: "the argument signature, such as (S VARCHAR)"},
			{Name: "volatility", Desc: "always empty: information_schema does not record it"},
			{Name: "parallel", Desc: "always empty: Snowflake does not record it"},
			{Name: "owner"},
			{Name: "security", Desc: "secure for a secure function, and empty otherwise"},
			{Name: "access", Desc: "always absent: a grant on a function is not in information_schema"},
			{Name: "language", Desc: "sql, javascript, python, java or scala"},
			{Name: "source"}, {Name: "comment"},
			{Name: "definition", Desc: "always absent: information_schema keeps the body, which is source, and GET_DDL is a call for each row"},
		},
		Params: schemaNameSystem("function"),
		Scan: func(rows *sql.Rows) (dbmeta.Function, error) {
			var v dbmeta.Function
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Name, &v.ID, &v.Kind, &v.ResultType,
				&v.ArgTypes, &v.Volatility, &v.Parallel, &v.Owner, &v.Security, &v.Access,
				&v.Language, &v.Source, &v.Comment, &v.Definition)
			return v, err
		},
	})
}
