package redshift

import (
	"database/sql"

	"github.com/xo/dbmeta"
)

func always(query string) dbmeta.Choice { return dbmeta.Choice{{Query: query}} }

// like is the filter that matches col to the pattern in param, and passes
// when the pattern is empty.
func like(col, param string) string {
	return `(CAST(` + param + ` AS text) = '' OR ` + col + ` LIKE CAST(` + param + ` AS text))`
}

// notSystem hides the schemas Redshift keeps for itself.
func notSystem(col string) string {
	return `(@with_system OR (` + col + ` NOT IN ('pg_catalog', 'information_schema',` +
		` 'pg_internal', 'catalog_history', 'pg_automv', 'pg_toast',` +
		` 'pg_auto_copy', 'pg_mv', 'pg_s3')` +
		` AND ` + col + ` NOT LIKE 'pg_temp_%'))`
}

func schemaNameSystem(kind string) []dbmeta.Param {
	return []dbmeta.Param{
		{Name: "schema", Desc: "schema name pattern, empty for every schema", Default: ""},
		{Name: "name", Desc: kind + " name pattern, empty for every " + kind, Default: ""},
		{Name: "with_system", Desc: "include the schemas Redshift keeps for itself", Default: false},
	}
}

func childParams(kind string) []dbmeta.Param {
	return append([]dbmeta.Param{
		{Name: "parent", Desc: "table name pattern, empty for every table", Default: ""},
	}, schemaNameSystem(kind)...)
}

// relationType is the word for a relation's kind, from pg_class.relkind.
const relationType = `CASE c.relkind WHEN 'r' THEN 'table' WHEN 'v' THEN 'view'` +
	` ELSE CAST(c.relkind AS text) END`

func register() {
	registerAccess()

	dbmeta.Schemas.Register(dbmeta.Redshift, &dbmeta.Binding[dbmeta.Schema]{
		Stmt: dbmeta.Stmt{
			always(`SELECT current_database() AS "catalog"`),
			always(`, n.nspname AS "name"`),
			always(`, pg_get_userbyid(n.nspowner) AS "owner"`),
			always(`, obj_description(n.oid, 'pg_namespace') AS "comment"`),
			always(`FROM pg_namespace n`),
			always(`WHERE ` + notSystem("n.nspname")),
			always(`AND ` + like("n.nspname", "@name")),
			always(`ORDER BY 2`),
		},
		Fields: dbmeta.Fields("catalog", "name", "owner", "comment"),
		Params: []dbmeta.Param{
			{Name: "name", Desc: "schema name pattern, empty for every schema", Default: ""},
			{Name: "with_system", Desc: "include the schemas Redshift keeps for itself", Default: false},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Schema, error) {
			var v dbmeta.Schema
			err := rows.Scan(&v.Catalog, &v.Name, dbmeta.NullAsEmpty(&v.Owner), &v.Comment)
			return v, err
		},
	})

	dbmeta.Databases.Register(dbmeta.Redshift, &dbmeta.Binding[dbmeta.Database]{
		Stmt: dbmeta.Stmt{
			always(`SELECT d.datname AS "name"`),
			always(`, pg_get_userbyid(d.datdba) AS "owner"`),
			always(`, pg_encoding_to_char(d.encoding) AS "encoding"`),
			always(`, '' AS "collate"`),
			always(`, '' AS "ctype"`),
			always(`, NULL AS "access"`),
			always(`, NULL AS "tablespace"`),
			always(`, NULL AS "size"`),
			always(`, NULL AS "comment"`),
			always(`FROM pg_database d`),
			always(`WHERE ` + like("d.datname", "@name")),
			always(`ORDER BY 1`),
		},
		Fields: []dbmeta.Field{
			{Name: "name"}, {Name: "owner"}, {Name: "encoding"},
			{Name: "collate", Desc: "always empty: pg_database records no collation"},
			{Name: "ctype", Desc: "always empty: pg_database records no collation"},
			{Name: "access", Desc: "always absent: a grant is in SVV_DATABASE_PRIVILEGES, which is not read"},
			{Name: "tablespace", Desc: "always absent: Redshift has no tablespace"},
			{Name: "size", Desc: "always absent: pg_database records no size"},
			{Name: "comment", Desc: "always absent: a database comment is not read"},
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

	dbmeta.Tables.Register(dbmeta.Redshift, &dbmeta.Binding[dbmeta.Table]{
		Stmt: dbmeta.Stmt{
			always(`SELECT current_database() AS "catalog"`),
			always(`, n.nspname AS "schema"`),
			always(`, c.relname AS "name"`),
			always(`, ` + relationType + ` AS "type"`),
			always(`, obj_description(c.oid, 'pg_class') AS "comment"`),
			always(`FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace`),
			always(`WHERE c.relkind IN ('r', 'v')`),
			always(`AND ` + notSystem("n.nspname")),
			always(`AND ` + like("n.nspname", "@schema")),
			always(`AND ` + like("c.relname", "@name")),
			always(`AND (CAST(@types AS text) = '' OR ` + dbmeta.InList(`CAST(@types AS text)`, relationType) + `)`),
			always(`ORDER BY 2, 3`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog"}, {Name: "schema"}, {Name: "name"},
			{Name: "type", Desc: "table or view"},
			{Name: "comment"},
		},
		Params: append(schemaNameSystem("table"), dbmeta.TypesParam()),
		Scan: func(rows *sql.Rows) (dbmeta.Table, error) {
			var v dbmeta.Table
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Name, &v.Type, &v.Comment)
			return v, err
		},
	})

	dbmeta.Columns.Register(dbmeta.Redshift, &dbmeta.Binding[dbmeta.Column]{
		Stmt: dbmeta.Stmt{
			always(`SELECT current_database() AS "catalog"`),
			always(`, n.nspname AS "schema"`),
			always(`, c.relname AS "table"`),
			always(`, a.attname AS "name"`),
			always(`, a.attnum AS "ordinal"`),
			always(`, format_type(a.atttypid, a.atttypmod) AS "data_type"`),
			always(`, NOT a.attnotnull AS "nullable"`),
			always(`, pg_get_expr(d.adbin, d.adrelid) AS "default"`),
			always(`, EXISTS (SELECT 1 FROM pg_constraint k WHERE k.conrelid = a.attrelid` +
				` AND k.contype = 'p' AND a.attnum = ANY (k.conkey)) AS "primary_key"`),
			always(`, CASE WHEN pg_get_expr(d.adbin, d.adrelid) LIKE '"identity"(%' THEN 'a'` +
				` WHEN pg_get_expr(d.adbin, d.adrelid) LIKE 'default_identity(%' THEN 'd'` +
				` ELSE '' END AS "identity"`),
			always(`, NULL AS "generated"`),
			always(`, col_description(a.attrelid, a.attnum) AS "comment"`),
			always(`, s.collation_name AS "collation"`),
			always(`FROM pg_attribute a`),
			always(`JOIN pg_class c ON c.oid = a.attrelid`),
			always(`JOIN pg_namespace n ON n.oid = c.relnamespace`),
			always(`LEFT JOIN pg_attrdef d ON d.adrelid = a.attrelid AND d.adnum = a.attnum`),
			always(`LEFT JOIN svv_columns s ON s.table_schema = n.nspname` +
				` AND s.table_name = c.relname AND s.column_name = a.attname`),
			always(`WHERE a.attnum > 0 AND NOT a.attisdropped AND c.relkind IN ('r', 'v')`),
			always(`AND ` + notSystem("n.nspname")),
			always(`AND ` + like("n.nspname", "@schema")),
			always(`AND ` + like("c.relname", "@parent")),
			always(`AND ` + like("a.attname", "@name")),
			always(`ORDER BY 2, 3, 5`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog"}, {Name: "schema"}, {Name: "table"}, {Name: "name"},
			{Name: "ordinal"}, {Name: "data_type"}, {Name: "nullable"},
			{Name: "default", Desc: "the default expression, which for an IDENTITY column is Redshift's identity() or default_identity() call"},
			{Name: "primary_key", Desc: "whether a declared primary key holds the column. Redshift does not enforce it"},
			{Name: "identity", Desc: "a, which is always, for an identity() default, because an INSERT cannot give it a value, and d, which is by default, for a default_identity() default. Empty otherwise"},
			{Name: "generated", Desc: "always absent: Redshift has no generated column, and GENERATED ALWAYS AS (expression) is a syntax error"},
			{Name: "comment"},
			{Name: "collation", Desc: "case_sensitive or case_insensitive for a character column, from SVV_COLUMNS, and absent for any other type"},
		},
		Params: childParams("column"),
		Scan: func(rows *sql.Rows) (dbmeta.Column, error) {
			var v dbmeta.Column
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Table, &v.Name, &v.Ordinal,
				&v.DataType, &v.Nullable, &v.Default, &v.PrimaryKey, &v.Identity,
				&v.Generated, &v.Comment, &v.Collation)
			return v, err
		},
	})

	dbmeta.Views.Register(dbmeta.Redshift, &dbmeta.Binding[dbmeta.View]{
		Stmt: dbmeta.Stmt{
			always(`SELECT current_database() AS "catalog"`),
			always(`, v.schemaname AS "schema"`),
			always(`, v.viewname AS "name"`),
			always(`, v.definition AS "definition"`),
			always(`, NULL AS "check_option"`),
			always(`, NULL AS "updatable"`),
			always(`, NULL AS "insertable"`),
			always(`, NULL AS "comment"`),
			always(`FROM pg_views v`),
			always(`WHERE ` + notSystem("v.schemaname")),
			always(`AND ` + like("v.schemaname", "@schema")),
			always(`AND ` + like("v.viewname", "@name")),
			always(`ORDER BY 2, 3`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog"}, {Name: "schema"}, {Name: "name"}, {Name: "definition"},
			{Name: "check_option", Desc: "always absent: Redshift has no WITH CHECK OPTION"},
			{Name: "updatable", Desc: "always absent: a Redshift view is read only"},
			{Name: "insertable", Desc: "always absent: a Redshift view is read only"},
			{Name: "comment", Desc: "always absent: the comment of a view is read by Tables"},
		},
		Params: schemaNameSystem("view"),
		Scan: func(rows *sql.Rows) (dbmeta.View, error) {
			var v dbmeta.View
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Name, &v.Definition, &v.CheckOption,
				&v.Updatable, &v.Insertable, &v.Comment)
			return v, err
		},
	})

	// A key is declared and not enforced.
	dbmeta.Constraints.Register(dbmeta.Redshift, &dbmeta.Binding[dbmeta.Constraint]{
		Stmt: dbmeta.Stmt{
			always(`SELECT n.nspname AS "schema"`),
			always(`, c.relname AS "table"`),
			always(`, k.conname AS "name"`),
			always(`, CASE k.contype WHEN 'p' THEN 'primary key' WHEN 'u' THEN 'unique'` +
				` WHEN 'f' THEN 'foreign key' WHEN 'c' THEN 'check' ELSE CAST(k.contype AS text) END AS "type"`),
			always(`, pg_get_constraintdef(k.oid) AS "definition"`),
			always(`, k.condeferrable AS "deferrable"`),
			always(`, k.condeferred AS "deferred"`),
			always(`, obj_description(k.oid, 'pg_constraint') AS "comment"`),
			always(`FROM pg_constraint k`),
			always(`JOIN pg_class c ON c.oid = k.conrelid`),
			always(`JOIN pg_namespace n ON n.oid = c.relnamespace`),
			always(`WHERE ` + notSystem("n.nspname")),
			always(`AND ` + like("n.nspname", "@schema")),
			always(`AND ` + like("c.relname", "@parent")),
			always(`AND ` + like("k.conname", "@name")),
			always(`ORDER BY 1, 2, 3`),
		},
		Fields: []dbmeta.Field{
			{Name: "schema"}, {Name: "table"}, {Name: "name"},
			{Name: "type", Desc: "primary key, unique or foreign key, which Redshift declares and does not enforce"},
			{Name: "definition"}, {Name: "deferrable"}, {Name: "deferred"}, {Name: "comment"},
		},
		Params: childParams("constraint"),
		Scan: func(rows *sql.Rows) (dbmeta.Constraint, error) {
			var v dbmeta.Constraint
			err := rows.Scan(&v.Schema, &v.Table, &v.Name, &v.Type, &v.Definition,
				&v.Deferrable, &v.Deferred, &v.Comment)
			return v, err
		},
	})

	dbmeta.Functions.Register(dbmeta.Redshift, &dbmeta.Binding[dbmeta.Function]{
		Stmt: dbmeta.Stmt{
			always(`SELECT current_database() AS "catalog"`),
			always(`, n.nspname AS "schema"`),
			always(`, p.proname AS "name"`),
			always(`, CAST(p.oid AS text) AS "id"`),
			always(`, CASE WHEN p.proisagg THEN 'agg' ELSE 'func' END AS "kind"`),
			always(`, format_type(p.prorettype, NULL) AS "result_type"`),
			always(`, oidvectortypes(p.proargtypes) AS "arg_types"`),
			always(`, CASE p.provolatile WHEN 'i' THEN 'immutable' WHEN 's' THEN 'stable'` +
				` ELSE 'volatile' END AS "volatility"`),
			always(`, '' AS "parallel"`),
			always(`, pg_get_userbyid(p.proowner) AS "owner"`),
			always(`, CASE WHEN p.prosecdef THEN 'definer' ELSE 'invoker' END AS "security"`),
			always(`, NULL AS "access"`),
			always(`, l.lanname AS "language"`),
			always(`, p.prosrc AS "source"`),
			always(`, obj_description(p.oid, 'pg_proc') AS "comment"`),
			always(`, NULL AS "definition"`),
			always(`FROM pg_proc p`),
			always(`JOIN pg_namespace n ON n.oid = p.pronamespace`),
			always(`LEFT JOIN pg_language l ON l.oid = p.prolang`),
			always(`WHERE NOT p.proisagg`),
			always(`AND ` + notSystem("n.nspname")),
			always(`AND ` + like("n.nspname", "@schema")),
			always(`AND ` + like("p.proname", "@name")),
			always(`ORDER BY 2, 3, 4`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog"}, {Name: "schema"}, {Name: "name"},
			{Name: "id", Desc: "the oid, because a name is overloaded"},
			{Name: "kind", Desc: "func. An aggregate is left out"},
			{Name: "result_type"},
			{Name: "arg_types", Desc: "the argument types, from oidvectortypes"},
			{Name: "volatility"},
			{Name: "parallel", Desc: "always empty: PostgreSQL 8.0 records no parallel safety"},
			{Name: "owner"}, {Name: "security"},
			{Name: "access", Desc: "always absent: the grant on a function is not read"},
			{Name: "language"}, {Name: "source"}, {Name: "comment"},
			{Name: "definition", Desc: "always absent: Redshift has no pg_get_functiondef, and SHOW FUNCTION is a statement of its own"},
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

	// A user from pg_user, a group from pg_group and a role from SVV_ROLES.
	// A group and a role cannot log in. None of the three has a source for
	// the flags below that Redshift does not record, so they read as they
	// always did.
	dbmeta.Roles.Register(dbmeta.Redshift, &dbmeta.Binding[dbmeta.Role]{
		Stmt: dbmeta.Stmt{
			always(`SELECT r."name", r.superuser, r.create_role, r.create_db, r.can_login`),
			always(`, r.replication, r.bypass_rls, r."inherit", r.conn_limit, r.valid_until`),
			always(`, r.member_of, r."comment"`),
			always(`FROM (`),
			always(`SELECT u.usename AS "name", u.usesuper AS superuser, FALSE AS create_role` +
				`, u.usecreatedb AS create_db, TRUE AS can_login, FALSE AS replication` +
				`, FALSE AS bypass_rls, TRUE AS "inherit", -1 AS conn_limit` +
				`, CAST(CAST(u.valuntil AS timestamp) AS varchar) AS valid_until` +
				`, '' AS member_of, CAST(NULL AS varchar) AS "comment" FROM pg_user u`),
			always(`UNION ALL SELECT g.groname, FALSE, FALSE, FALSE, FALSE, FALSE, FALSE, TRUE, -1` +
				`, CAST(NULL AS varchar), '', CAST(NULL AS varchar) FROM pg_group g`),
			always(`UNION ALL SELECT o.role_name, FALSE, FALSE, FALSE, FALSE, FALSE, FALSE, TRUE, -1` +
				`, CAST(NULL AS varchar), '', CAST(NULL AS varchar) FROM svv_roles o`),
			always(`) r`),
			always(`WHERE ` + like(`r."name"`, "@name")),
			always(`ORDER BY 1`),
		},
		Fields: []dbmeta.Field{
			{Name: "name", Desc: "a user, a group or a role"},
			{Name: "superuser", Desc: "from pg_user. False for a group and a role"},
			{Name: "create_role", Desc: "always false: it is a grant rather than a flag"},
			{Name: "create_db"},
			{Name: "can_login", Desc: "true for a user, false for a group and a role"},
			{Name: "replication", Desc: "always false: Redshift has no replication role"},
			{Name: "bypass_rls", Desc: "always false: pg_user records no such flag"},
			{Name: "inherit", Desc: "always true: a granted role is always inherited"},
			{Name: "conn_limit", Desc: "always -1: the limit is in SVV_USER_INFO, which shows a user only its own row, so no user can read the others"},
			{Name: "valid_until"},
			{Name: "member_of", Desc: "always empty: RoleGrants answers it, because SVV_USER_GRANTS shows a user only its own grants"},
			{Name: "comment", Desc: "always absent: a user, a group and a role take no comment"},
		},
		Params: []dbmeta.Param{
			{Name: "name", Desc: "name pattern, empty for every user, group and role", Default: ""},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Role, error) {
			var v dbmeta.Role
			err := rows.Scan(&v.Name, &v.Superuser, &v.CreateRole, &v.CreateDB,
				&v.CanLogin, &v.Replication, &v.BypassRLS, &v.Inherit, &v.ConnLimit,
				&v.ValidUntil, &v.MemberOf, &v.Comment)
			return v, err
		},
	})

	dbmeta.Comments.Register(dbmeta.Redshift, &dbmeta.Binding[dbmeta.Comment]{
		Stmt: dbmeta.Stmt{
			always(`SELECT n.nspname AS "schema"`),
			always(`, c.relname AS "name"`),
			always(`, ` + relationType + ` AS "type"`),
			always(`, obj_description(c.oid, 'pg_class') AS "comment"`),
			always(`FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace`),
			always(`WHERE c.relkind IN ('r', 'v') AND obj_description(c.oid, 'pg_class') IS NOT NULL`),
			always(`AND ` + notSystem("n.nspname")),
			always(`AND ` + like("n.nspname", "@schema")),
			always(`AND ` + like("c.relname", "@name")),
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

	dbmeta.CurrentSchema.Register(dbmeta.Redshift, &dbmeta.Binding[dbmeta.Schema]{
		Stmt: dbmeta.Stmt{
			always(`SELECT current_database() AS "catalog"`),
			always(`, n.nspname AS "name"`),
			always(`, pg_get_userbyid(n.nspowner) AS "owner"`),
			always(`, obj_description(n.oid, 'pg_namespace') AS "comment"`),
			always(`FROM pg_namespace n`),
			always(`WHERE n.nspname = current_schema()`),
		},
		Fields: dbmeta.Fields("catalog", "name", "owner", "comment"),
		Scan: func(rows *sql.Rows) (dbmeta.Schema, error) {
			var v dbmeta.Schema
			err := rows.Scan(&v.Catalog, &v.Name, dbmeta.NullAsEmpty(&v.Owner), &v.Comment)
			return v, err
		},
	})

	dbmeta.CurrentUser.Register(dbmeta.Redshift, &dbmeta.Binding[dbmeta.User]{
		Stmt: dbmeta.Stmt{
			always(`SELECT current_user AS "name"`),
			always(`, session_user AS "session"`),
		},
		Fields: dbmeta.Fields("name", "session"),
		Scan: func(rows *sql.Rows) (dbmeta.User, error) {
			var v dbmeta.User
			err := rows.Scan(&v.Name, &v.Session)
			return v, err
		},
	})
}
