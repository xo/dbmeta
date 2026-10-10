package snowflake

import (
	"database/sql"
	"strings"

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

// keyParams are childParams and the catalog pattern, which a key statement
// reads to scope its SHOW. See keyScope.
func keyParams(parent, kind string) []dbmeta.Param {
	return append([]dbmeta.Param{
		{Name: "catalog", Desc: "database name pattern, empty for the current database", Default: ""},
	}, childParams(parent, kind)...)
}

func childParams(parent, kind string) []dbmeta.Param {
	return append([]dbmeta.Param{
		{Name: "parent", Desc: parent + " name pattern, empty for every " + parent, Default: ""},
	}, schemaNameSystem(kind)...)
}

// tableType is the word for the kind of the table t. A dynamic, an Iceberg and
// a hybrid table are base tables to table_type and have a flag of their own.
const tableType = `CASE t.table_type WHEN 'BASE TABLE' THEN` +
	` CASE WHEN t.is_dynamic = 'YES' THEN 'dynamic table'` +
	` WHEN t.is_iceberg = 'YES' THEN 'iceberg table'` +
	` WHEN t.is_hybrid = 'YES' THEN 'hybrid table'` +
	` WHEN t.is_transient = 'YES' THEN 'transient table' ELSE 'table' END` +
	` ELSE LOWER(t.table_type) END`

// persistence is Table.Persistence. A view has none, and a transient table
// has no fail safe period, which is the nearest word to PostgreSQL's unlogged.
const persistence = `CASE WHEN t.table_type IN ('VIEW', 'MATERIALIZED VIEW') THEN NULL` +
	` WHEN t.is_temporary = 'YES' THEN 'temporary'` +
	` WHEN t.is_transient = 'YES' THEN 'transient' ELSE 'permanent' END`

// tableOptions is Table.Options. A view has no retention time and no key.
const tableOptions = `CASE WHEN t.retention_time IS NULL THEN NULL` +
	` ELSE 'retention_time=' || t.retention_time::VARCHAR` +
	` || CASE WHEN t.clustering_key IS NULL THEN '' ELSE ', cluster_by=' || t.clustering_key END END`

// piped is a chain of SHOW statements and the select that reads them, as one
// statement. SHOW is not a table, but Snowflake chains a statement onto it
// with the pipe operator, and the select after it reads the result of the
// latest stage as $1, the stage before as $2 and so on.
//
// A bind parameter is refused after the pipe, so the dialect writes its values
// into the statement (D203). A SHOW with no scope reads the current schema, which a session can
// change, so every stage names a scope. The scope is the derived value
// @scope, which keyScope computes. A session with no database reads every
// database that the role can see, so a statement that does not read
// information_schema also keeps the rows of CURRENT_DATABASE().
func piped(shows ...string) string {
	return strings.Join(shows, ` ->> `) + ` ->> `
}

// keyFilter is the filter of a statement that reads a SHOW result, on the
// columns that hold the database, the schema, the table and the constraint.
func keyFilter(catalog, schema, table, constraint string) string {
	return like(catalog, "@catalog") + ` AND ` + notSystem(schema) + ` AND ` + like(schema, "@schema") +
		` AND ` + like(table, "@parent") + ` AND ` + like(constraint, "@name")
}

// scope is a piece of SQL that a binding derived and the dialect writes into a
// statement as it is. Only keyScope makes one, and it quotes every name.
type scope string

// keyScope is the scope of the SHOW statements that read keys. It is the
// narrowest one that cannot lose a row: IN TABLE when the schema and the table
// are both exact names, IN SCHEMA when the database and the schema are, and IN
// DATABASE otherwise, which with no name is the current database.
//
// A pattern is an exact name when it holds no wildcard, so it can match only
// one name. The server stores the name as the caller wrote it, because every
// other statement here matches with LIKE, which is case sensitive, so the name
// is quoted as it is and never folded. The statement filters the rows with the
// same patterns, so the scope only decides how many rows the server reads.
//
// SHOW refuses a schema without its database, "Must specify the full search
// path starting from database", and the statement cannot name the current
// database. So a schema scope needs the catalog pattern to be exact as well.
// A table scope does not, because a name of two parts starts at the current
// database. A table that does not exist, or that the role cannot see, is an
// error from the server in that scope, where the wide scope answers no rows.
func keyScope(args map[string]any) any {
	catalog, hasCatalog := exactName(arg(args, "catalog"))
	prefix := ""
	if hasCatalog {
		prefix = quote(catalog) + "."
	}
	schema, ok := exactName(arg(args, "schema"))
	if !ok {
		if hasCatalog {
			return scope("DATABASE " + quote(catalog))
		}
		return scope("DATABASE")
	}
	if table, ok := exactName(arg(args, "parent")); ok {
		return scope("TABLE " + prefix + quote(schema) + "." + quote(table))
	}
	if hasCatalog {
		return scope("SCHEMA " + prefix + quote(schema))
	}
	return scope("DATABASE")
}

// quote writes a name as a quoted Snowflake identifier. A double quote in the
// name is doubled, which is what keeps a hostile name inside the quotes.
func quote(name string) string { return dbmeta.QuoteIdentifier(name, `"`, `"`) }

// exactName reads a LIKE pattern as one name. It returns false for the empty
// pattern, which means every name, and for a pattern with %, _ or a backslash.
// A backslash can escape a wildcard, and whether the server reads it so
// depends on the statement, so a pattern with one is not an exact name and the
// scope stays wide. A wider scope reads more rows and loses none.
func exactName(pattern string) (string, bool) {
	if pattern == "" || strings.ContainsAny(pattern, `%_\`) {
		return "", false
	}
	return pattern, true
}

// arg returns a string argument, and the empty string when it is absent.
func arg(args map[string]any, name string) string {
	s, _ := args[name].(string)
	return s
}

func register() {
	registerRelations()
	registerRoutines()
	registerSettings()
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
			always(`, t.table_owner AS "owner"`),
			always(`, ` + persistence + ` AS "persistence"`),
			always(`, t.bytes::BIGINT AS "size"`),
			always(`, t.row_count::BIGINT AS "rows"`),
			always(`, ` + tableOptions + ` AS "options"`),
			always(`FROM information_schema.tables t`),
			always(`WHERE ` + notSystem("t.table_schema")),
			always(`AND ` + like("t.table_schema", "@schema")),
			always(`AND ` + like("t.table_name", "@name")),
			always(`AND (@types = '' OR ` + dbmeta.InList(`@types`, tableType) + `)`),
			always(`ORDER BY 2, 3`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog"}, {Name: "schema"}, {Name: "name"},
			{Name: "type", Desc: "table, transient table, dynamic table, iceberg table, hybrid table, view, materialized view, external table or event table, from table_type and the flags beside it"},
			{Name: "comment"},
			{Name: "owner", Desc: "TABLE_OWNER, the role that owns the table. Absent for a view the role does not own"},
			{Name: "persistence", Desc: "permanent, transient or temporary. Absent for a view. Snowflake has no unlogged table"},
			{Name: "size", Desc: "BYTES, the bytes of active data. Absent for a view, and a bound, because time travel and fail safe data are not counted"},
			{Name: "rows", Desc: "ROW_COUNT, which Snowflake keeps and does not estimate. Absent for a view"},
			{Name: "options", Desc: "retention_time in days and cluster_by when a clustering key is set, as the CREATE TABLE clauses name them. Absent for a view"},
		},
		Params: append(schemaNameSystem("table"), dbmeta.TypesParam()),
		Scan: func(rows *sql.Rows) (dbmeta.Table, error) {
			var v dbmeta.Table
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Name, &v.Type, &v.Comment,
				&v.Owner, &v.Persistence, &v.Size, &v.Rows, &v.Options)
			return v, err
		},
	})

	// information_schema has no KEY_COLUMN_USAGE, so the columns of a key are
	// only in SHOW PRIMARY KEYS. The pipe operator chains one select onto it,
	// and that select joins the columns view to the keys. The statement is
	// scoped to the table or the schema when the patterns name one, and the
	// filters of the other statements apply to the rows. See D203.
	//
	// The SQL API sends a boolean of a piped statement as the text 0 or 1, and
	// the driver of dbimp reads only true and false. So the two booleans are
	// numbers here, and database/sql converts them for the bool fields. The
	// cast can go when the driver reads 0 and 1. See D213.
	dbmeta.Columns.Register(dbmeta.Snowflake, &dbmeta.Binding[dbmeta.Column]{
		Stmt: dbmeta.Stmt{
			always(piped(`SHOW PRIMARY KEYS IN @scope`) + `SELECT c.table_catalog AS "catalog"`),
			always(`, c.table_schema AS "schema"`),
			always(`, c.table_name AS "table"`),
			always(`, c.column_name AS "name"`),
			always(`, c.ordinal_position AS "ordinal"`),
			always(`, c.data_type AS "data_type"`),
			always(`, IFF(c.is_nullable = 'YES', 1, 0) AS "nullable"`),
			always(`, c.column_default AS "default"`),
			always(`, IFF(k."key_sequence" IS NOT NULL, 1, 0) AS "primary_key"`),
			always(`, CASE WHEN c.is_identity = 'YES' THEN 'by default' END AS "identity"`),
			always(`, NULL AS "generated"`),
			always(`, c.comment AS "comment"`),
			always(`, c.collation_name AS "collation"`),
			always(`FROM information_schema.columns c`),
			always(`LEFT JOIN $1 k ON k."database_name" = c.table_catalog`),
			always(`AND k."schema_name" = c.table_schema AND k."table_name" = c.table_name`),
			always(`AND k."column_name" = c.column_name`),
			always(`WHERE ` + like("c.table_catalog", "@catalog")),
			always(`AND ` + notSystem("c.table_schema")),
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
			{Name: "primary_key", Desc: "true when SHOW PRIMARY KEYS lists the column"},
			{Name: "identity", Desc: "by default for an IDENTITY or AUTOINCREMENT column, which accepts a value of its own"},
			{Name: "generated", Desc: "always absent: Snowflake has no generated column"},
			{Name: "comment"},
			{Name: "collation", Desc: "the COLLATE of a text column, and absent where it has none"},
		},
		Params:  keyParams("table", "column"),
		Derived: []dbmeta.Derived{{Name: "scope", From: keyScope}},
		Scan: func(rows *sql.Rows) (dbmeta.Column, error) {
			var v dbmeta.Column
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Table, &v.Name, &v.Ordinal,
				&v.DataType, &v.Nullable, &v.Default, &v.PrimaryKey, &v.Identity,
				&v.Generated, &v.Comment, &v.Collation)
			return v, err
		},
	})

	// A key is declared and not enforced, except NOT NULL. ENFORCED says so.
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
			always(`, k.enforced = 'YES' AS "enforced"`),
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
			{Name: "deferrable", Desc: "IS_DEFERRABLE as the server says it, which is NO for every key"},
			{Name: "deferred", Desc: "INITIALLY_DEFERRED as the server says it, which is YES for every key. Read enforced first"},
			{Name: "comment"},
			{Name: "enforced", Desc: "ENFORCED, which is NO for every key. Only NOT NULL is checked"},
		},
		Params: childParams("table", "constraint"),
		Scan: func(rows *sql.Rows) (dbmeta.Constraint, error) {
			var v dbmeta.Constraint
			err := rows.Scan(&v.Schema, &v.Table, &v.Name, &v.Type, &v.Definition,
				&v.Deferrable, &v.Deferred, &v.Comment, &v.Enforced)
			return v, err
		},
	})

	// The columns of a primary key, a unique key and a foreign key, and the
	// column a foreign key points at. Each SHOW is one stage of the pipe, and
	// the select reads the result of the latest stage as $1, the one before as
	// $2 and the first as $3. See D203.
	dbmeta.ConstraintColumns.Register(dbmeta.Snowflake, &dbmeta.Binding[dbmeta.ConstraintColumn]{
		Stmt: dbmeta.Stmt{
			always(piped(`SHOW PRIMARY KEYS IN @scope`, `SHOW UNIQUE KEYS IN @scope`, `SHOW IMPORTED KEYS IN @scope`) +
				`SELECT "database_name" AS "catalog", "schema_name" AS "schema"`),
			always(`, "table_name" AS "table", "constraint_name" AS "constraint"`),
			always(`, "column_name" AS "name", "key_sequence" AS "ordinal"`),
			always(`, NULL AS "foreign_catalog", NULL AS "foreign_schema"`),
			always(`, NULL AS "foreign_table", NULL AS "foreign_name"`),
			always(`FROM $3`),
			always(`WHERE "database_name" = CURRENT_DATABASE()`),
			always(`AND ` + keyFilter(`"database_name"`, `"schema_name"`, `"table_name"`, `"constraint_name"`)),
			always(`UNION ALL SELECT "database_name", "schema_name", "table_name", "constraint_name",`),
			always(` "column_name", "key_sequence", NULL, NULL, NULL, NULL FROM $2`),
			always(`WHERE "database_name" = CURRENT_DATABASE()`),
			always(`AND ` + keyFilter(`"database_name"`, `"schema_name"`, `"table_name"`, `"constraint_name"`)),
			always(`UNION ALL SELECT "fk_database_name", "fk_schema_name", "fk_table_name", "fk_name",`),
			always(` "fk_column_name", "key_sequence", "pk_database_name", "pk_schema_name",`),
			always(` "pk_table_name", "pk_column_name" FROM $1`),
			always(`WHERE "fk_database_name" = CURRENT_DATABASE()`),
			always(`AND ` + keyFilter(`"fk_database_name"`, `"fk_schema_name"`, `"fk_table_name"`, `"fk_name"`)),
			always(`ORDER BY 2, 3, 4, 6`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog"}, {Name: "schema"}, {Name: "table"}, {Name: "constraint"},
			{Name: "name"}, {Name: "ordinal"},
			{Name: "foreign_catalog", Desc: "set for a foreign key only"},
			{Name: "foreign_schema", Desc: "set for a foreign key only"},
			{Name: "foreign_table", Desc: "set for a foreign key only"},
			{Name: "foreign_name", Desc: "the column the key points at, set for a foreign key only"},
		},
		Params:  keyParams("table", "constraint"),
		Derived: []dbmeta.Derived{{Name: "scope", From: keyScope}},
		Scan: func(rows *sql.Rows) (dbmeta.ConstraintColumn, error) {
			var v dbmeta.ConstraintColumn
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Table, &v.Constraint, &v.Name,
				&v.Ordinal, &v.ForeignCatalog, &v.ForeignSchema, &v.ForeignTable, &v.ForeignName)
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
			always(`, CAST(s."INCREMENT" AS VARCHAR) AS "increment"`),
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
			always(`, ` + tableType + ` AS "type"`),
			always(`, p.grantee || '=' || p.privilege_type || '/' || p.grantor AS "access"`),
			always(`, NULL AS "column_access"`),
			always(`, NULL AS "policies"`),
			always(`FROM information_schema.table_privileges p`),
			always(`JOIN information_schema.tables t ON t.table_catalog = p.table_catalog`),
			always(`AND t.table_schema = p.table_schema AND t.table_name = p.table_name`),
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
			{Name: "admin", Desc: "whether the grantee can grant the role on"},
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
			{Name: "session", Desc: "the role the session acts as, which decides what it can do"},
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
		always(`, LOWER(f.volatility) AS "volatility"`),
		always(`, '' AS "parallel"`),
		always(`, f.function_owner AS "owner"`),
		always(`, CASE WHEN f.is_secure = 'YES' THEN 'secure' ELSE '' END AS "security"`),
		always(`, NULL AS "access"`),
		always(`, LOWER(f.function_language) AS "language"`),
		always(`, f.function_definition AS "source"`),
		always(`, f.comment AS "comment"`),
		always(`, NULL AS "definition"`),
		always(`, f.function_definition AS "prosrc"`),
		always(`FROM information_schema.functions f`),
		always(`WHERE ` + notSystem("f.function_schema")),
		always(`AND ` + like("f.function_schema", "@schema")),
		always(`AND ` + like("f.function_name", "@name")),
		always(`UNION ALL`),
		always(`SELECT p.procedure_catalog, p.procedure_schema, p.procedure_name,`),
		always(` p.procedure_name || p.argument_signature, 'proc', p.data_type, p.argument_signature,`),
		always(` '', '', p.procedure_owner, '', NULL, LOWER(p.procedure_language), p.procedure_definition, p.comment, NULL,`),
		always(` p.procedure_definition`),
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
			{Name: "volatility", Desc: "volatile or immutable for a function, and empty for a procedure"},
			{Name: "parallel", Desc: "always empty: Snowflake does not record it"},
			{Name: "owner"},
			{Name: "security", Desc: "secure for a secure function, and empty otherwise"},
			{Name: "access", Desc: "always absent: a grant on a function is not in information_schema"},
			{Name: "language", Desc: "sql, javascript, python, java or scala"},
			{Name: "source"}, {Name: "comment"},
			{Name: "definition", Desc: "always absent: information_schema keeps the body, which is source, and GET_DDL is not measured yet"},
			{Name: "prosrc", Desc: "the body of the function or the procedure, which is the same text as source. Snowflake has no other"},
		},
		Params: schemaNameSystem("function"),
		Scan: func(rows *sql.Rows) (dbmeta.Function, error) {
			var v dbmeta.Function
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Name, &v.ID, &v.Kind, &v.ResultType,
				&v.ArgTypes, dbmeta.NullAsEmpty(&v.Volatility), &v.Parallel, &v.Owner, &v.Security, &v.Access,
				&v.Language, &v.Source, &v.Comment, &v.Definition, &v.Prosrc)
			return v, err
		},
	})
}

// registerSettings reads the parameters of the session. SHOW PARAMETERS is the
// only source, and the pipe operator turns it into one select. See D203.
func registerSettings() {
	dbmeta.Settings.Register(dbmeta.Snowflake, &dbmeta.Binding[dbmeta.Setting]{
		Stmt: dbmeta.Stmt{
			always(piped(`SHOW PARAMETERS`) + `SELECT "key" AS "name"`),
			always(`, "value" AS "value"`),
			always(`, "type" AS "type"`),
			always(`, "level" AS "context"`),
			always(`, NULL AS "access"`),
			always(`, NULL AS "display"`),
			always(`FROM $1`),
			always(`WHERE ` + like(`"key"`, "@name")),
			always(`ORDER BY 1`),
		},
		Fields: []dbmeta.Field{
			{Name: "name"}, {Name: "value"}, {Name: "type"},
			{Name: "context", Desc: "the level the value is set at, which is empty for the default, or ACCOUNT, SESSION or an object"},
			{Name: "access", Desc: "always absent: SHOW PARAMETERS records no access"},
			{Name: "display", Desc: "always absent: Snowflake shows a value in one form, which is value"},
		},
		Params: []dbmeta.Param{
			{Name: "name", Desc: "parameter name pattern, empty for every one", Default: ""},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Setting, error) {
			var v dbmeta.Setting
			err := rows.Scan(&v.Name, &v.Value, &v.Type, &v.Context, &v.Access, &v.Display)
			return v, err
		},
	})
}
