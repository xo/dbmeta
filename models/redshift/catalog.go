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

// userNames is the users with the two columns the statements read. Redshift
// refuses pg_user itself beside a UNION of Tables or Columns, with the error
// "Specified types or functions are not supported on Redshift tables", and
// reports pg_shadow.valuntil of type abstime as the cause. A derived table
// that names its columns does not read the two it cannot handle (D221).
const userNames = `(SELECT usesysid, usename FROM pg_user)`

// notSystem hides the schemas Redshift keeps for itself.
func notSystem(col string) string {
	return systemFilter(`@with_system`, col)
}

// notSystemBeside is notSystem for a statement that reads SVV_TABLE_INFO.
// Redshift refuses a bare boolean parameter beside it, which is the error
// "Specified types or functions are not supported on Redshift tables", and
// accepts the same parameter in a comparison (D212).
func notSystemBeside(col string) string {
	return systemFilter(`@with_system = TRUE`, col)
}

func systemFilter(with, col string) string {
	return `(` + with + ` OR (` + col + ` NOT IN ('pg_catalog', 'information_schema',` +
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

// nspname and relname are the names of a schema and a relation as text, with
// the padding removed that Redshift adds when SVV_TABLE_INFO is in the statement.
const (
	nspname = `TRIM(TRAILING FROM n.nspname)`
	relname = `TRIM(TRAILING FROM c.relname)`
)

// tableType is relationType for the Tables statement, which keeps only tables
// and views. A cast of relkind to text is refused beside SVV_TABLE_INFO, which
// reads the compute nodes, so there is no ELSE branch. The WHERE clause leaves
// no other kind to reach it.
const tableType = `CASE c.relkind WHEN 'r' THEN 'table' WHEN 'v' THEN 'view' END`

// relationType is the word for a relation's kind, from pg_class.relkind.
const relationType = `CASE c.relkind WHEN 'r' THEN 'table' WHEN 'v' THEN 'view'` +
	` ELSE CAST(c.relkind AS text) END`

// compression is Column.Compression. attencodingtype numbers the encodings,
// and pg_table_def names them, which was measured for each number below. A
// number that was not measured is returned as it is, never guessed. A view
// has no encoding.
const compression = `CASE WHEN c.relkind = 'v' THEN NULL ELSE CASE a.attencodingtype` +
	` WHEN 0 THEN 'raw' WHEN 1 THEN 'bytedict' WHEN 2 THEN 'delta' WHEN 3 THEN 'lzo'` +
	` WHEN 4 THEN 'runlength' WHEN 5 THEN 'delta32k' WHEN 7 THEN 'text255'` +
	` WHEN 15 THEN 'mostly8' WHEN 16 THEN 'mostly16' WHEN 17 THEN 'mostly32'` +
	` WHEN 18 THEN 'text32k' WHEN 19 THEN 'zstd' WHEN 20 THEN 'az64'` +
	` ELSE CAST(a.attencodingtype AS text) END END`

func register() {
	registerAccess()

	dbmeta.Schemas.Register(dbmeta.Redshift, &dbmeta.Binding[dbmeta.Schema]{
		Stmt: dbmeta.Stmt{
			always(`SELECT current_database() AS "catalog"`),
			always(`, n.nspname AS "name"`),
			always(`, pg_get_userbyid(n.nspowner) AS "owner"`),
			always(`, obj_description(n.oid, 'pg_namespace') AS "comment"`),
			always(`, array_to_string(n.nspacl, ` + newline + `) AS "access"`),
			always(`FROM pg_namespace n`),
			always(`WHERE ` + notSystem("n.nspname")),
			always(`AND ` + like("n.nspname", "@name")),
			always(`ORDER BY 2`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog"}, {Name: "name"}, {Name: "owner"}, {Name: "comment"},
			{Name: "access", Desc: "nspacl, one grant on each line as grantee=privileges/grantor. Absent when the privileges are the default"},
		},
		Params: []dbmeta.Param{
			{Name: "name", Desc: "schema name pattern, empty for every schema", Default: ""},
			{Name: "with_system", Desc: "include the schemas Redshift keeps for itself", Default: false},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Schema, error) {
			var v dbmeta.Schema
			err := rows.Scan(&v.Catalog, &v.Name, dbmeta.NullAsEmpty(&v.Owner), &v.Comment, &v.Access)
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

	// Tables reads SVV_TABLE_INFO for the size, the rows and the options, and
	// the view is the cost of that (D212). Redshift refuses it to every user
	// who is not a superuser until the administrator grants SELECT on it, and a
	// user without the grant gets an error from this whole kind and not a NULL.
	// SVV_TABLE_INFO runs on the compute nodes, which changes four things in
	// the statement, each measured on Redshift Serverless 1.0.477953:
	//
	//   - The leader node functions pg_get_userbyid and obj_description are
	//     refused beside it, so the owner is a join to pg_user and the comment
	//     is a join to pg_description. 1259 is the oid of pg_class.
	//   - A name read beside it comes back padded to its type, char(128), and a
	//     LIKE with no wildcard then matches nothing. TRIM removes the padding
	//     before the filter reads it and before the row is returned.
	//   - CAST(c.relkind AS text) is refused beside it, so the type has no ELSE
	//     branch. See tableType. A boolean parameter on its own is refused too,
	//     so the system filter compares it. See notSystemBeside.
	//   - The view lists a table only when it holds a row. An empty table has
	//     no row in it, so its size, rows and options are NULL: they are
	//     unknown and not zero.
	//
	// The join is by table id, on a derived table, so the filter on the name
	// still narrows the read of pg_class.
	dbmeta.Tables.Register(dbmeta.Redshift, &dbmeta.Binding[dbmeta.Table]{
		Stmt: dbmeta.Stmt{
			always(`SELECT current_database() AS "catalog"`),
			always(`, ` + nspname + ` AS "schema"`),
			always(`, ` + relname + ` AS "name"`),
			always(`, ` + tableType + ` AS "type"`),
			always(`, d.description AS "comment"`),
			always(`, TRIM(TRAILING FROM u.usename) AS "owner"`),
			always(`, CASE WHEN n.nspname LIKE 'pg_temp_%' THEN 'temporary' ELSE 'permanent' END AS "persistence"`),
			always(`, CAST(i."size" AS BIGINT) * 1048576 AS "size"`),
			always(`, CAST(i."tbl_rows" AS BIGINT) AS "rows"`),
			always(`, CASE WHEN i."diststyle" IS NULL THEN NULL ELSE 'diststyle=' || TRIM(i."diststyle")` +
				` || CASE WHEN i."sortkey1" IS NULL THEN '' ELSE ', sortkey=' || TRIM(i."sortkey1") END END AS "options"`),
			always(`FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace`),
			always(`LEFT JOIN ` + userNames + ` u ON u.usesysid = c.relowner`),
			always(`LEFT JOIN pg_description d ON d.objoid = c.oid AND d.classoid = 1259 AND d.objsubid = 0`),
			always(`LEFT JOIN (SELECT "table_id", "size", "tbl_rows", "diststyle", "sortkey1"` +
				` FROM svv_table_info) i ON i."table_id" = c.oid`),
			always(`WHERE c.relkind IN ('r', 'v')`),
			always(`AND ` + notSystemBeside("n.nspname")),
			always(`AND ` + like(nspname, "@schema")),
			always(`AND ` + like(relname, "@name")),
			always(`AND (CAST(@types AS text) = '' OR ` + dbmeta.InList(`CAST(@types AS text)`, tableType) + `)`),
			always(`UNION ALL`),
			always(`SELECT current_database(), TRIM(TRAILING FROM x.schemaname), TRIM(TRAILING FROM x.tablename)`),
			always(`, 'external table', CAST(NULL AS varchar(256)), TRIM(TRAILING FROM xu.usename)`),
			always(`, 'permanent', CAST(NULL AS BIGINT), CAST(NULL AS BIGINT)`),
			always(`, 'location=' || x.location || ', serde=' || x.serialization_lib`),
			always(`FROM svv_external_tables x`),
			always(`LEFT JOIN svv_external_schemas xs ON TRIM(TRAILING FROM xs.schemaname) = TRIM(TRAILING FROM x.schemaname)`),
			always(`LEFT JOIN ` + userNames + ` xu ON xu.usesysid = xs.esowner`),
			always(`WHERE x.redshift_database_name = CAST(current_database() AS text)`),
			always(`AND ` + like("TRIM(TRAILING FROM x.schemaname)", "@schema")),
			always(`AND ` + like("TRIM(TRAILING FROM x.tablename)", "@name")),
			always(`AND (CAST(@types AS text) = '' OR ` + dbmeta.InList(`CAST(@types AS text)`, `'external table'`) + `)`),
			always(`ORDER BY 2, 3`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog"}, {Name: "schema"}, {Name: "name"},
			{Name: "type", Desc: "table, view or external table. An external table is a Redshift Spectrum table, whose rows are files in S3 that a Glue data catalog describes"},
			{Name: "comment"},
			{Name: "owner", Desc: "the user that owns the relation, from pg_class. For an external table, the owner of its external schema"},
			{Name: "persistence", Desc: "temporary for a table in a pg_temp schema, and permanent for the rest. Redshift has no unlogged table"},
			{Name: "size", Desc: "a bound in bytes: SVV_TABLE_INFO counts blocks of 1 MB, so this is the blocks times 1048576. Absent for a view, an external table and an empty table, which the view does not list. The user must have SELECT on SVV_TABLE_INFO, or the whole kind fails (D212)"},
			{Name: "rows", Desc: "tbl_rows of SVV_TABLE_INFO, the rows including those marked for deletion and not yet vacuumed. Absent for a view, an external table and an empty table"},
			{Name: "options", Desc: "the distribution style and the first sort key, as diststyle=KEY(event_id), sortkey=happened. The sort key part is absent when the table has none. Absent for a view and for an empty table. For an external table, the location and the serde library, as location=s3://bucket/path/, serde=org.apache.hadoop.hive.serde2.lazy.LazySimpleSerDe"},
		},
		Params: append(schemaNameSystem("table"), dbmeta.TypesParam()),
		Scan: func(rows *sql.Rows) (dbmeta.Table, error) {
			var v dbmeta.Table
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Name, &v.Type, &v.Comment,
				&v.Owner, &v.Persistence, &v.Size, &v.Rows, &v.Options)
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
			always(`, (pk.conrelid IS NOT NULL AND a.attnum = ANY (pk.conkey)) AS "primary_key"`),
			// The CAST gives the column a width. Beside a UNION the type of the
			// CASE is the width of its ELSE, which is 0, and the server then
			// refuses the a and the d as too long (D221).
			always(`, CAST(CASE WHEN pg_get_expr(d.adbin, d.adrelid) LIKE '"identity"(%' THEN 'a'` +
				` WHEN pg_get_expr(d.adbin, d.adrelid) LIKE 'default_identity(%' THEN 'd'` +
				` ELSE '' END AS varchar(1)) AS "identity"`),
			always(`, CAST(NULL AS varchar(1)) AS "generated"`),
			always(`, col_description(a.attrelid, a.attnum) AS "comment"`),
			always(`, s.collation_name AS "collation"`),
			always(`, ` + compression + ` AS "compression"`),
			always(`FROM pg_attribute a`),
			always(`JOIN pg_class c ON c.oid = a.attrelid`),
			always(`JOIN pg_namespace n ON n.oid = c.relnamespace`),
			always(`LEFT JOIN pg_attrdef d ON d.adrelid = a.attrelid AND d.adnum = a.attnum`),
			always(`LEFT JOIN (SELECT conrelid, conkey FROM pg_constraint WHERE contype = 'p') pk` +
				` ON pk.conrelid = a.attrelid`),
			always(`LEFT JOIN svv_columns s ON s.table_schema = n.nspname` +
				` AND s.table_name = c.relname AND s.column_name = a.attname`),
			always(`WHERE a.attnum > 0 AND NOT a.attisdropped AND c.relkind IN ('r', 'v')`),
			always(`AND ` + notSystem("n.nspname")),
			always(`AND ` + like("n.nspname", "@schema")),
			always(`AND ` + like("c.relname", "@parent")),
			always(`AND ` + like("a.attname", "@name")),
			always(`UNION ALL`),
			always(`SELECT current_database(), x.schemaname, x.tablename, x.columnname, x.columnnum`),
			always(`, x.external_type, TRUE, CAST(NULL AS varchar(256)), FALSE, CAST('' AS varchar(1))`),
			always(`, CAST(NULL AS varchar(256)), CAST(NULL AS varchar(256)), CAST(NULL AS varchar(256)), CAST(NULL AS varchar(256))`),
			always(`FROM svv_external_columns x`),
			always(`WHERE x.redshift_database_name = CAST(current_database() AS text)`),
			always(`AND ` + like("x.schemaname", "@schema")),
			always(`AND ` + like("x.tablename", "@parent")),
			always(`AND ` + like("x.columnname", "@name")),
			always(`ORDER BY 2, 3, 5`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog"}, {Name: "schema"}, {Name: "table"}, {Name: "name"},
			{Name: "ordinal"},
			{Name: "data_type", Desc: "format_type for a table or a view. For a column of an external table, the type that the Glue data catalog holds, such as int or string, which is not always a Redshift type name"},
			{Name: "nullable", Desc: "always true for a column of an external table, because SVV_EXTERNAL_COLUMNS leaves is_nullable empty and a Glue column has no NOT NULL"},
			{Name: "default", Desc: "the default expression, which for an IDENTITY column is Redshift's identity() or default_identity() call"},
			{Name: "primary_key", Desc: "whether a declared primary key holds the column. Redshift does not enforce it"},
			{Name: "identity", Desc: "a, which is always, for an identity() default, because an INSERT cannot give it a value, and d, which is by default, for a default_identity() default. Empty otherwise"},
			{Name: "generated", Desc: "always absent: Redshift has no generated column, and GENERATED ALWAYS AS (expression) is a syntax error"},
			{Name: "comment"},
			{Name: "collation", Desc: "case_sensitive or case_insensitive for a character column, from SVV_COLUMNS, and absent for any other type"},
			{Name: "compression", Desc: "the column encoding, such as az64 or lzo, from attencodingtype. raw is no compression. A view column has none"},
		},
		Params: childParams("column"),
		Scan: func(rows *sql.Rows) (dbmeta.Column, error) {
			var v dbmeta.Column
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Table, &v.Name, &v.Ordinal,
				&v.DataType, &v.Nullable, &v.Default, &v.PrimaryKey, &v.Identity,
				&v.Generated, &v.Comment, &v.Collation, &v.Compression)
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
			always(`, p.prosrc AS "prosrc"`),
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
			{Name: "prosrc", Desc: "prosrc, the body of the function, which is the same text as source"},
		},
		Params: schemaNameSystem("function"),
		Scan: func(rows *sql.Rows) (dbmeta.Function, error) {
			var v dbmeta.Function
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Name, &v.ID, &v.Kind, &v.ResultType,
				&v.ArgTypes, &v.Volatility, &v.Parallel, &v.Owner, &v.Security, &v.Access,
				&v.Language, &v.Source, &v.Comment, &v.Definition, &v.Prosrc)
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
