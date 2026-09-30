package cratedb

import (
	"database/sql"

	"github.com/xo/dbmeta"
)

// v64 is CrateDB 6.4, the first release measured with the view
// information_schema.collations. 6.3 has none.
var v64 = dbmeta.V(6, 4)

// systemSchemas are the schemas CrateDB keeps for itself: the two catalogs
// it shares with PostgreSQL, its own sys, and blob, which holds blob tables.
const systemSchemas = `'information_schema', 'pg_catalog', 'sys', 'blob'`

// notSystem is the filter that hides those schemas unless the caller asks for
// them, for a column that holds a schema's name.
func notSystem(col string) string {
	return `(@with_system OR ` + col + ` NOT IN (` + systemSchemas + `))`
}

// tableType turns information_schema.tables.table_type into the words the
// postgres model uses.
const tableType = `CASE t.table_type WHEN 'BASE TABLE' THEN 'table' WHEN 'VIEW' THEN 'view'` +
	` WHEN 'FOREIGN' THEN 'foreign table' ELSE lower(t.table_type) END`

// options aggregates the options of one object from one of the option views
// as key=value pairs, the way PostgreSQL writes an options array.
func options(view, where string) string {
	return `(SELECT array_to_string(array_agg(o.option_name || '=' || o.option_value), ', ')` +
		` FROM information_schema.` + view + ` o WHERE ` + where + `)`
}

func always(query string) dbmeta.Choice { return dbmeta.Choice{{Query: query}} }

// registerOwn registers the statements that read CrateDB's own catalog.
func registerOwn() {
	registerRelations()
	registerRoutines()
	registerRoles()
	registerServer()
}

func registerRelations() {
	dbmeta.Schemas.Register(dbmeta.CrateDB, &dbmeta.Binding[dbmeta.Schema]{
		Stmt: dbmeta.Stmt{
			always(`SELECT current_database() AS "catalog"`),
			always(`, s.schema_name AS "name"`),
			// CrateDB keeps no owner for a schema.
			always(`, '' AS "owner"`),
			always(`, NULL AS "comment"`),
			always(`FROM information_schema.schemata s`),
			always(`WHERE ` + notSystem("s.schema_name")),
			always(`AND (@name = '' OR s.schema_name LIKE @name)`),
			always(`ORDER BY 2`),
		},
		Fields: fields("catalog", "name", "owner", "comment"),
		Params: []dbmeta.Param{
			{Name: "name", Desc: "schema name pattern, empty for every schema", Default: ""},
			{Name: "with_system", Desc: "include the schemas CrateDB keeps for itself", Default: false},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Schema, error) {
			var v dbmeta.Schema
			err := rows.Scan(&v.Catalog, &v.Name, &v.Owner, &v.Comment)
			return v, err
		},
	})

	dbmeta.CurrentSchema.Register(dbmeta.CrateDB, &dbmeta.Binding[dbmeta.Schema]{
		Stmt: dbmeta.Stmt{
			always(`SELECT current_database() AS "catalog"`),
			always(`, current_schema() AS "name"`),
			always(`, '' AS "owner"`),
			always(`, NULL AS "comment"`),
		},
		Fields: fields("catalog", "name", "owner", "comment"),
		Scan: func(rows *sql.Rows) (dbmeta.Schema, error) {
			var v dbmeta.Schema
			err := rows.Scan(&v.Catalog, &v.Name, &v.Owner, &v.Comment)
			return v, err
		},
	})

	dbmeta.Tables.Register(dbmeta.CrateDB, &dbmeta.Binding[dbmeta.Table]{
		Stmt: dbmeta.Stmt{
			always(`SELECT current_database() AS "catalog"`),
			always(`, t.table_schema AS "schema"`),
			always(`, t.table_name AS "name"`),
			always(`, ` + tableType + ` AS "type"`),
			// CrateDB has no COMMENT statement.
			always(`, NULL AS "comment"`),
			always(`FROM information_schema.tables t`),
			always(`WHERE ` + notSystem("t.table_schema")),
			always(`AND (@schema = '' OR t.table_schema LIKE @schema)`),
			always(`AND (@name = '' OR t.table_name LIKE @name)`),
			always(`AND (@types = '' OR ` + dbmeta.InList(`CAST(@types AS text)`, tableType) + `)`),
			always(`ORDER BY 2, 3`),
		},
		Fields: fields("catalog", "schema", "name", "type", "comment"),
		Params: append(schemaNameSystem("relation"), dbmeta.TypesParam()),
		Scan: func(rows *sql.Rows) (dbmeta.Table, error) {
			var v dbmeta.Table
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Name, &v.Type, &v.Comment)
			return v, err
		},
	})

	dbmeta.Columns.Register(dbmeta.CrateDB, &dbmeta.Binding[dbmeta.Column]{
		Stmt: dbmeta.Stmt{
			always(`SELECT current_database() AS "catalog"`),
			always(`, c.table_schema AS "schema"`),
			always(`, c.table_name AS "table"`),
			always(`, c.column_name AS "name"`),
			always(`, c.ordinal_position AS "ordinal"`),
			always(`, c.data_type AS "data_type"`),
			always(`, c.is_nullable = 'YES' AS "nullable"`),
			always(`, c.column_default AS "default"`),
			always(`, EXISTS (SELECT 1 FROM information_schema.key_column_usage k` +
				` JOIN information_schema.table_constraints tc` +
				` ON tc.constraint_schema = k.constraint_schema AND tc.constraint_name = k.constraint_name` +
				` WHERE tc.constraint_type = 'PRIMARY KEY'` +
				` AND k.table_schema = c.table_schema AND k.table_name = c.table_name` +
				` AND k.column_name = c.column_name) AS "primary_key"`),
			// CrateDB has no identity column, and says so for each column.
			always(`, CASE WHEN c.is_identity THEN c.identity_generation ELSE '' END AS "identity"`),
			// A generated column's value is computed when a row is written
			// and kept, which is what PostgreSQL calls stored, and what the
			// postgres model reports as s.
			always(`, CASE c.is_generated WHEN 'ALWAYS' THEN 's' ELSE '' END AS "generated"`),
			always(`, NULL AS "comment"`),
			always(`, NULL AS "collation"`),
			always(`FROM information_schema.columns c`),
			always(`WHERE c.table_schema NOT IN (` + systemSchemas + `)`),
			always(`AND (@schema = '' OR c.table_schema LIKE @schema)`),
			always(`AND (@parent = '' OR c.table_name LIKE @parent)`),
			always(`AND (@name = '' OR c.column_name LIKE @name)`),
			always(`ORDER BY 2, 3, 5`),
		},
		Fields: fields("catalog", "schema", "table", "name", "ordinal", "data_type", "nullable", "default",
			"primary_key", "identity", "generated", "comment", "collation"),
		Params: []dbmeta.Param{
			{Name: "schema", Desc: "schema name pattern, empty for every schema", Default: ""},
			{Name: "parent", Desc: "table name pattern, empty for every table", Default: ""},
			{Name: "name", Desc: "column name pattern, empty for every column", Default: ""},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Column, error) {
			var v dbmeta.Column
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Table, &v.Name, &v.Ordinal, &v.DataType,
				&v.Nullable, &v.Default, &v.PrimaryKey, &v.Identity, &v.Generated, &v.Comment, &v.Collation)
			return v, err
		},
	})

	dbmeta.Views.Register(dbmeta.CrateDB, &dbmeta.Binding[dbmeta.View]{
		Stmt: dbmeta.Stmt{
			always(`SELECT current_database() AS "catalog"`),
			always(`, v.table_schema AS "schema"`),
			always(`, v.table_name AS "name"`),
			always(`, v.view_definition AS "definition"`),
			always(`, lower(v.check_option) AS "check_option"`),
			always(`, v.is_updatable AS "updatable"`),
			// CrateDB refuses an INSERT into a view, and says so in no column.
			always(`, NULL AS "insertable"`),
			always(`, NULL AS "comment"`),
			always(`FROM information_schema.views v`),
			always(`WHERE ` + notSystem("v.table_schema")),
			always(`AND (@schema = '' OR v.table_schema LIKE @schema)`),
			always(`AND (@name = '' OR v.table_name LIKE @name)`),
			always(`ORDER BY 2, 3`),
		},
		Fields: fields("catalog", "schema", "name", "definition", "check_option", "updatable", "insertable", "comment"),
		Params: schemaNameSystem("view"),
		Scan: func(rows *sql.Rows) (dbmeta.View, error) {
			var v dbmeta.View
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Name, &v.Definition, &v.CheckOption,
				&v.Updatable, &v.Insertable, &v.Comment)
			return v, err
		},
	})

	// A partitioned table has one partition for each distinct value of its
	// partition columns, which is PostgreSQL's LIST, and the expression is
	// written the way pg_get_partkeydef writes one.
	dbmeta.PartitionedTables.Register(dbmeta.CrateDB, &dbmeta.Binding[dbmeta.PartitionedTable]{
		Stmt: dbmeta.Stmt{
			always(`SELECT t.table_schema AS "schema"`),
			always(`, t.table_name AS "name"`),
			always(`, '' AS "owner"`),
			always(`, 'table' AS "type"`),
			always(`, '' AS "parent"`),
			always(`, 'LIST' AS "strategy"`),
			always(`, 'LIST (' || array_to_string(t.partitioned_by, ', ') || ')' AS "expression"`),
			always(`, NULL AS "comment"`),
			always(`FROM information_schema.tables t`),
			always(`WHERE t.partitioned_by IS NOT NULL`),
			always(`AND ` + notSystem("t.table_schema")),
			always(`AND (@schema = '' OR t.table_schema LIKE @schema)`),
			always(`AND (@name = '' OR t.table_name LIKE @name)`),
			always(`ORDER BY 1, 2`),
		},
		Fields: fields("schema", "name", "owner", "type", "parent", "strategy", "expression", "comment"),
		Params: schemaNameSystem("partitioned table"),
		Scan: func(rows *sql.Rows) (dbmeta.PartitionedTable, error) {
			var v dbmeta.PartitionedTable
			err := rows.Scan(&v.Schema, &v.Name, &v.Owner, &v.Type, &v.Parent, &v.Strategy, &v.Expression, &v.Comment)
			return v, err
		},
	})

	// pg_index holds one index for each primary key and nothing else. A
	// full text index is in no catalog, only in SHOW CREATE TABLE. pg_am is
	// empty, so an index has no access method to name, and CrateDB reports
	// a primary key as not unique, which this returns as it stands.
	dbmeta.Indexes.Register(dbmeta.CrateDB, &dbmeta.Binding[dbmeta.Index]{
		Stmt: dbmeta.Stmt{
			always(`SELECT current_database() AS "catalog"`),
			always(`, n.nspname AS "schema"`),
			always(`, t.relname AS "table"`),
			always(`, c.relname AS "name"`),
			always(`, '' AS "type"`),
			always(`, i.indisunique AS "unique"`),
			always(`, i.indisprimary AS "primary"`),
			always(`, NULL AS "comment"`),
			always(`FROM pg_catalog.pg_index i`),
			always(`JOIN pg_catalog.pg_class c ON c.oid = i.indexrelid`),
			always(`JOIN pg_catalog.pg_class t ON t.oid = i.indrelid`),
			always(`JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace`),
			always(`WHERE ` + notSystem("n.nspname")),
			always(`AND (@schema = '' OR n.nspname LIKE @schema)`),
			always(`AND (@parent = '' OR t.relname LIKE @parent)`),
			always(`AND (@name = '' OR c.relname LIKE @name)`),
			always(`ORDER BY 2, 3, 4`),
		},
		Fields: fields("catalog", "schema", "table", "name", "type", "unique", "primary", "comment"),
		Params: schemaParentName("index"),
		Scan: func(rows *sql.Rows) (dbmeta.Index, error) {
			var v dbmeta.Index
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Table, &v.Name, &v.Type,
				&v.Unique, &v.Primary, &v.Comment)
			return v, err
		},
	})

	// The only index is a primary key's, and it has the name of its
	// constraint, so its columns are the key's columns. CrateDB has no
	// LATERAL to unnest pg_index.indkey with. The expression of a column is
	// its name, which is what pg_get_indexdef gives on PostgreSQL.
	dbmeta.IndexColumns.Register(dbmeta.CrateDB, &dbmeta.Binding[dbmeta.IndexColumn]{
		Stmt: dbmeta.Stmt{
			always(`SELECT k.table_schema AS "schema"`),
			always(`, k.table_name AS "table"`),
			always(`, k.constraint_name AS "index"`),
			always(`, k.column_name AS "name"`),
			always(`, k.ordinal_position AS "ordinal"`),
			always(`, k.column_name AS "expression"`),
			always(`, false AS "descending"`),
			always(`FROM information_schema.key_column_usage k`),
			always(`JOIN information_schema.table_constraints tc` +
				` ON tc.constraint_schema = k.constraint_schema AND tc.constraint_name = k.constraint_name`),
			always(`WHERE tc.constraint_type = 'PRIMARY KEY'`),
			always(`AND ` + notSystem("k.table_schema")),
			always(`AND (@schema = '' OR k.table_schema LIKE @schema)`),
			always(`AND (@parent = '' OR k.table_name LIKE @parent)`),
			always(`AND (@name = '' OR k.constraint_name LIKE @name)`),
			always(`ORDER BY 1, 2, 3, 5`),
		},
		Fields: fields("schema", "table", "index", "name", "ordinal", "expression", "descending"),
		Params: schemaParentName("index"),
		Scan: func(rows *sql.Rows) (dbmeta.IndexColumn, error) {
			var v dbmeta.IndexColumn
			err := rows.Scan(&v.Schema, &v.Table, &v.Index, &v.Name, &v.Ordinal,
				&v.Expression, &v.Descending)
			return v, err
		},
	})

	// pg_constraint holds the primary keys and the check constraints, and
	// leaves out the NOT NULL checks that information_schema lists, which is
	// where PostgreSQL keeps them too. A definition needs
	// pg_get_constraintdef, which arrived in 6.4. Before that, the check
	// expression is in no catalog at all, only in SHOW CREATE TABLE.
	dbmeta.Constraints.Register(dbmeta.CrateDB, &dbmeta.Binding[dbmeta.Constraint]{
		Stmt: dbmeta.Stmt{
			always(`SELECT n.nspname AS "schema"`),
			always(`, t.relname AS "table"`),
			always(`, r.conname AS "name"`),
			always(`, CASE r.contype WHEN 'c' THEN 'check' WHEN 'f' THEN 'foreign key'` +
				` WHEN 'p' THEN 'primary key' WHEN 'u' THEN 'unique' ELSE r.contype END AS "type"`),
			{
				{Key: Release, Min: v64, Query: `, pg_catalog.pg_get_constraintdef(r.oid, true) AS "definition"`},
				{Query: `, NULL AS "definition"`},
			},
			always(`, r.condeferrable AS "deferrable"`),
			always(`, r.condeferred AS "deferred"`),
			always(`, NULL AS "comment"`),
			always(`FROM pg_catalog.pg_constraint r`),
			always(`JOIN pg_catalog.pg_class t ON t.oid = r.conrelid`),
			always(`JOIN pg_catalog.pg_namespace n ON n.oid = t.relnamespace`),
			always(`WHERE ` + notSystem("n.nspname")),
			always(`AND (@schema = '' OR n.nspname LIKE @schema)`),
			always(`AND (@parent = '' OR t.relname LIKE @parent)`),
			always(`AND (@name = '' OR r.conname LIKE @name)`),
			always(`ORDER BY 1, 2, 3`),
		},
		Fields: []dbmeta.Field{
			{Name: "schema"}, {Name: "table"}, {Name: "name"}, {Name: "type"},
			{Name: "definition", Key: Release, Min: v64},
			{Name: "deferrable"}, {Name: "deferred"}, {Name: "comment"},
		},
		Params: schemaParentName("constraint"),
		Scan: func(rows *sql.Rows) (dbmeta.Constraint, error) {
			var v dbmeta.Constraint
			err := rows.Scan(&v.Schema, &v.Table, &v.Name, &v.Type, &v.Definition,
				&v.Deferrable, &v.Deferred, &v.Comment)
			return v, err
		},
	})

	// CrateDB has no foreign key, so a key's columns point at nothing.
	dbmeta.ConstraintColumns.Register(dbmeta.CrateDB, &dbmeta.Binding[dbmeta.ConstraintColumn]{
		Stmt: dbmeta.Stmt{
			always(`SELECT current_database() AS "catalog"`),
			always(`, k.table_schema AS "schema"`),
			always(`, k.table_name AS "table"`),
			always(`, k.constraint_name AS "constraint"`),
			always(`, k.column_name AS "name"`),
			always(`, k.ordinal_position AS "ordinal"`),
			always(`, NULL AS "foreign_catalog"`),
			always(`, NULL AS "foreign_schema"`),
			always(`, NULL AS "foreign_table"`),
			always(`, NULL AS "foreign_name"`),
			always(`FROM information_schema.key_column_usage k`),
			always(`WHERE ` + notSystem("k.table_schema")),
			always(`AND (@schema = '' OR k.table_schema LIKE @schema)`),
			always(`AND (@parent = '' OR k.table_name LIKE @parent)`),
			always(`AND (@name = '' OR k.constraint_name LIKE @name)`),
			always(`ORDER BY 2, 3, 4, 6`),
		},
		Fields: fields("catalog", "schema", "table", "constraint", "name", "ordinal",
			"foreign_catalog", "foreign_schema", "foreign_table", "foreign_name"),
		Params: schemaParentName("constraint"),
		Scan: func(rows *sql.Rows) (dbmeta.ConstraintColumn, error) {
			var v dbmeta.ConstraintColumn
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Table, &v.Constraint, &v.Name, &v.Ordinal,
				&v.ForeignCatalog, &v.ForeignSchema, &v.ForeignTable, &v.ForeignName)
			return v, err
		},
	})

	dbmeta.ColumnStats.Register(dbmeta.CrateDB, &dbmeta.Binding[dbmeta.ColumnStat]{
		Stmt: dbmeta.Stmt{
			always(`SELECT current_database() AS "catalog"`),
			always(`, s.schemaname AS "schema"`),
			always(`, s.tablename AS "table"`),
			always(`, s.attname AS "name"`),
			always(`, s.avg_width AS "avg_width"`),
			always(`, s.null_frac AS "null_frac"`),
			always(`, s.n_distinct AS "distinct"`),
			always(`, s.histogram_bounds[1] AS "min"`),
			always(`, s.histogram_bounds[array_length(s.histogram_bounds, 1)] AS "max"`),
			always(`, NULL AS "mean"`),
			always(`, array_to_string(s.most_common_vals, E'\n') AS "top_n"`),
			always(`, array_to_string(s.most_common_freqs, E'\n') AS "top_n_freqs"`),
			always(`FROM pg_catalog.pg_stats s`),
			always(`WHERE ` + notSystem("s.schemaname")),
			always(`AND (@schema = '' OR s.schemaname LIKE @schema)`),
			always(`AND (@parent = '' OR s.tablename LIKE @parent)`),
			always(`AND (@name = '' OR s.attname LIKE @name)`),
			always(`ORDER BY 2, 3, 4`),
		},
		Fields: fields("catalog", "schema", "table", "name", "avg_width", "null_frac", "distinct",
			"min", "max", "mean", "top_n", "top_n_freqs"),
		Params: schemaParentName("column"),
		Scan: func(rows *sql.Rows) (dbmeta.ColumnStat, error) {
			var v dbmeta.ColumnStat
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Table, &v.Name, &v.AvgWidth, &v.NullFrac, &v.Distinct,
				&v.Min, &v.Max, &v.Mean, &v.TopN, &v.TopNFreqs)
			return v, err
		},
	})
}

func registerRoutines() {
	// Every type is built in and lives in pg_catalog, so a caller sees them
	// only with with_system. CrateDB has no CREATE TYPE, no enum and no
	// owner or privilege on a type.
	dbmeta.Types.Register(dbmeta.CrateDB, &dbmeta.Binding[dbmeta.Type]{
		Stmt: dbmeta.Stmt{
			always(`SELECT current_database() AS "catalog"`),
			always(`, n.nspname AS "schema"`),
			always(`, pg_catalog.format_type(t.oid, NULL) AS "name"`),
			always(`, t.typname AS "internal"`),
			always(`, CASE t.typtype WHEN 'b' THEN 'base' WHEN 'c' THEN 'composite'` +
				` WHEN 'd' THEN 'domain' WHEN 'e' THEN 'enum' WHEN 'p' THEN 'pseudo'` +
				` WHEN 'r' THEN 'range' WHEN 'm' THEN 'multirange' ELSE t.typtype END AS "kind"`),
			always(`, '' AS "elements"`),
			always(`, NULL AS "owner"`),
			always(`, NULL AS "access"`),
			always(`, NULL AS "comment"`),
			always(`, CASE WHEN t.typrelid <> 0 THEN 'tuple' WHEN t.typlen < 0 THEN 'var'` +
				` ELSE CAST(t.typlen AS text) END AS "size"`),
			always(`FROM pg_catalog.pg_type t`),
			always(`JOIN pg_catalog.pg_namespace n ON n.oid = t.typnamespace`),
			// leave out the array type that every scalar type creates
			always(`WHERE NOT EXISTS (SELECT 1 FROM pg_catalog.pg_type el WHERE el.oid = t.typelem AND el.typarray = t.oid)`),
			always(`AND ` + notSystem("n.nspname")),
			always(`AND (@schema = '' OR n.nspname LIKE @schema)`),
			always(`AND (@name = '' OR t.typname LIKE @name OR pg_catalog.format_type(t.oid, NULL) LIKE @name)`),
			always(`ORDER BY 2, 3`),
		},
		Fields: fields("catalog", "schema", "name", "internal", "kind", "elements", "owner", "access", "comment", "size"),
		Params: schemaNameSystem("type"),
		Scan: func(rows *sql.Rows) (dbmeta.Type, error) {
			var v dbmeta.Type
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Name, &v.Internal, &v.Kind,
				&v.Elements, &v.Owner, &v.Access, &v.Comment, &v.Size)
			return v, err
		},
	})

	// A function is a user defined function in JavaScript. CrateDB overloads
	// a name, and specific_name holds the name with its argument types, so it
	// is the id.
	dbmeta.Functions.Register(dbmeta.CrateDB, &dbmeta.Binding[dbmeta.Function]{
		Stmt: dbmeta.Stmt{
			always(`SELECT current_database() AS "catalog"`),
			always(`, r.routine_schema AS "schema"`),
			always(`, r.routine_name AS "name"`),
			always(`, r.specific_name AS "id"`),
			always(`, 'func' AS "kind"`),
			always(`, r.data_type AS "result_type"`),
			always(`, regexp_replace(r.specific_name, '^[^(]*\((.*)\)$', '$1') AS "arg_types"`),
			always(`, CASE WHEN r.is_deterministic THEN 'immutable' ELSE 'volatile' END AS "volatility"`),
			always(`, '' AS "parallel"`),
			always(`, NULL AS "owner"`),
			always(`, '' AS "security"`),
			always(`, NULL AS "access"`),
			always(`, r.routine_body AS "language"`),
			always(`, r.routine_definition AS "source"`),
			always(`, NULL AS "comment"`),
			always(`, NULL AS "definition"`),
			always(`FROM information_schema.routines r`),
			always(`WHERE r.routine_type = 'FUNCTION'`),
			always(`AND ` + notSystem("r.routine_schema")),
			always(`AND (@schema = '' OR r.routine_schema LIKE @schema)`),
			always(`AND (@name = '' OR r.routine_name LIKE @name)`),
			always(`ORDER BY 2, 3, 4`),
		},
		Fields: append(fields("catalog", "schema", "name", "id", "kind", "result_type", "arg_types", "volatility",
			"parallel", "owner", "security", "access", "language", "source", "comment"), dbmeta.Field{
			Name: "definition",
			Desc: "always absent: information_schema keeps the body, which is source, and no CREATE statement",
		}),
		Params: schemaNameSystem("function"),
		Scan: func(rows *sql.Rows) (dbmeta.Function, error) {
			var v dbmeta.Function
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Name, &v.ID, &v.Kind, &v.ResultType, &v.ArgTypes,
				&v.Volatility, &v.Parallel, &v.Owner, &v.Security, &v.Access, &v.Language, &v.Source, &v.Comment,
				&v.Definition)
			return v, err
		},
	})
}

func registerRoles() {
	// pg_roles holds the users, which can log in, and the roles, which
	// cannot. It is read rather than sys.users and sys.roles, because a user
	// who is not a superuser is refused the sys schema and may read
	// pg_roles. CrateDB has no database to create and no row security, so
	// those two are false. The parity test found the refusal. See D61.
	dbmeta.Roles.Register(dbmeta.CrateDB, &dbmeta.Binding[dbmeta.Role]{
		Stmt: dbmeta.Stmt{
			always(`SELECT r.rolname AS "name"`),
			always(`, r.rolsuper AS "superuser"`),
			always(`, r.rolcreaterole AS "create_role"`),
			always(`, false AS "create_db"`),
			always(`, r.rolcanlogin AS "can_login"`),
			always(`, r.rolreplication AS "replication"`),
			always(`, false AS "bypass_rls"`),
			always(`, r.rolinherit AS "inherit"`),
			always(`, r.rolconnlimit AS "conn_limit"`),
			always(`, CAST(r.rolvaliduntil AS text) AS "valid_until"`),
			// The same list the postgres model makes, empty for a role that
			// is a member of none.
			always(`, COALESCE((SELECT array_to_string(array_agg(g.rolname), ', ')` +
				` FROM pg_catalog.pg_auth_members m JOIN pg_catalog.pg_roles g ON g.oid = m.roleid` +
				` WHERE m.member = r.oid), '') AS "member_of"`),
			always(`, NULL AS "comment"`),
			always(`FROM pg_catalog.pg_roles r`),
			always(`WHERE (@name = '' OR r.rolname LIKE @name)`),
			always(`ORDER BY 1`),
		},
		Fields: fields("name", "superuser", "create_role", "create_db", "can_login", "replication",
			"bypass_rls", "inherit", "conn_limit", "valid_until", "member_of", "comment"),
		Params: []dbmeta.Param{
			{Name: "name", Desc: "role name pattern, empty for every role", Default: ""},
			{Name: "with_system", Desc: "CrateDB keeps no role for itself, so this changes nothing", Default: false},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Role, error) {
			var v dbmeta.Role
			err := rows.Scan(&v.Name, &v.Superuser, &v.CreateRole, &v.CreateDB, &v.CanLogin, &v.Replication,
				&v.BypassRLS, &v.Inherit, &v.ConnLimit, &v.ValidUntil, &v.MemberOf, &v.Comment)
			return v, err
		},
	})

	// A privilege on a table, as grantee=type/grantor in the order sys.privileges
	// holds them, with a denied privilege marked. A relation with no grant of
	// its own reads NULL, as PostgreSQL's does.
	dbmeta.Privileges.Register(dbmeta.CrateDB, &dbmeta.Binding[dbmeta.Privilege]{
		Stmt: dbmeta.Stmt{
			always(`SELECT t.table_schema AS "schema"`),
			always(`, t.table_name AS "name"`),
			always(`, ` + tableType + ` AS "type"`),
			always(`, (SELECT CASE WHEN count(*) > 0 THEN array_to_string(array_agg(p.grantee || '=' || p.type || '/' || p.grantor` +
				` || CASE p.state WHEN 'DENY' THEN ' denied' ELSE '' END), E'\n') END` +
				` FROM sys.privileges p WHERE p.class = 'TABLE'` +
				` AND p.ident = t.table_schema || '.' || t.table_name) AS "access"`),
			always(`, NULL AS "column_access"`),
			always(`, NULL AS "policies"`),
			always(`FROM information_schema.tables t`),
			always(`WHERE ` + notSystem("t.table_schema")),
			always(`AND (@schema = '' OR t.table_schema LIKE @schema)`),
			always(`AND (@name = '' OR t.table_name LIKE @name)`),
			always(`ORDER BY 1, 2`),
		},
		Fields: []dbmeta.Field{
			{Name: "schema"}, {Name: "name"}, {Name: "type"},
			{Name: "access", Desc: "grantee=type/grantor for each privilege granted on the table, one a line, and denied after a denied one"},
			{Name: "column_access", Desc: "always absent: CrateDB grants no privilege on a column"},
			{Name: "policies", Desc: "always absent: CrateDB has no row security"},
		},
		Params: schemaNameSystem("relation"),
		Scan: func(rows *sql.Rows) (dbmeta.Privilege, error) {
			var v dbmeta.Privilege
			err := rows.Scan(&v.Schema, &v.Name, &v.Type, &v.Access, &v.ColumnAccess, &v.Policies)
			return v, err
		},
	})
}

func registerServer() {
	// CrateDB has one database, crate, and keeps no owner and no size for it.
	dbmeta.Databases.Register(dbmeta.CrateDB, &dbmeta.Binding[dbmeta.Database]{
		Stmt: dbmeta.Stmt{
			always(`SELECT d.datname AS "name"`),
			always(`, '' AS "owner"`),
			always(`, pg_catalog.pg_encoding_to_char(d.encoding) AS "encoding"`),
			always(`, d.datcollate AS "collate"`),
			always(`, d.datctype AS "ctype"`),
			always(`, array_to_string(d.datacl, E'\n') AS "access"`),
			always(`, NULL AS "tablespace"`),
			always(`, NULL AS "size"`),
			always(`, NULL AS "comment"`),
			always(`FROM pg_catalog.pg_database d`),
			always(`WHERE (@name = '' OR d.datname LIKE @name)`),
			always(`ORDER BY 1`),
		},
		Fields: fields("name", "owner", "encoding", "collate", "ctype", "access", "tablespace", "size", "comment"),
		Params: []dbmeta.Param{{Name: "name", Desc: "database name pattern, empty for every database", Default: ""}},
		Scan: func(rows *sql.Rows) (dbmeta.Database, error) {
			var v dbmeta.Database
			err := rows.Scan(&v.Name, &v.Owner, &v.Encoding, &v.Collate, &v.CType,
				&v.Access, &v.Tablespace, &v.Size, &v.Comment)
			return v, err
		},
	})

	// The collation view arrived in 6.4. It holds CrateDB's one collation,
	// in pg_catalog, so a caller sees it only with with_system.
	dbmeta.Collations.Register(dbmeta.CrateDB, &dbmeta.Binding[dbmeta.Collation]{
		Stmt: dbmeta.Stmt{
			{{Key: Release, Min: v64, Query: `SELECT c.collation_schema AS "schema"`}},
			{{Key: Release, Min: v64, Query: `, c.collation_name AS "name"`}},
			{{Key: Release, Min: v64, Query: `, NULL AS "provider"`}},
			{{Key: Release, Min: v64, Query: `, c.collation_name AS "collate"`}},
			{{Key: Release, Min: v64, Query: `, c.collation_name AS "ctype"`}},
			{{Key: Release, Min: v64, Query: `, NULL AS "locale"`}},
			{{Key: Release, Min: v64, Query: `, true AS "deterministic"`}},
			{{Key: Release, Min: v64, Query: `, NULL AS "comment"`}},
			{{Key: Release, Min: v64, Query: `, NULL AS "rules"`}},
			{{Key: Release, Min: v64, Query: `FROM information_schema.collations c`}},
			{{Key: Release, Min: v64, Query: `WHERE ` + notSystem("c.collation_schema")}},
			{{Key: Release, Min: v64, Query: `AND (@schema = '' OR c.collation_schema LIKE @schema)`}},
			{{Key: Release, Min: v64, Query: `AND (@name = '' OR c.collation_name LIKE @name)`}},
			{{Key: Release, Min: v64, Query: `ORDER BY 1, 2`}},
		},
		Fields: append(fields("schema", "name", "provider", "collate", "ctype", "locale", "deterministic", "comment"),
			dbmeta.Field{Name: "rules", Desc: "always absent: CrateDB has no tailoring rules"}),
		Params: schemaNameSystem("collation"),
		Scan: func(rows *sql.Rows) (dbmeta.Collation, error) {
			var v dbmeta.Collation
			err := rows.Scan(&v.Schema, &v.Name, &v.Provider, &v.Collate, &v.CType, &v.Locale, &v.Deterministic, &v.Comment,
				&v.Rules)
			return v, err
		},
	})

	dbmeta.ForeignServers.Register(dbmeta.CrateDB, &dbmeta.Binding[dbmeta.ForeignServer]{
		Stmt: dbmeta.Stmt{
			always(`SELECT s.foreign_server_name AS "name"`),
			always(`, s.authorization_identifier AS "owner"`),
			always(`, s.foreign_data_wrapper_name AS "wrapper"`),
			always(`, s.foreign_server_type AS "type"`),
			always(`, s.foreign_server_version AS "version"`),
			always(`, NULL AS "access"`),
			always(`, ` + options("foreign_server_options", `o.foreign_server_name = s.foreign_server_name`) + ` AS "options"`),
			always(`, NULL AS "comment"`),
			always(`FROM information_schema.foreign_servers s`),
			always(`WHERE (@name = '' OR s.foreign_server_name LIKE @name)`),
			always(`ORDER BY 1`),
		},
		Fields: fields("name", "owner", "wrapper", "type", "version", "access", "options", "comment"),
		Params: []dbmeta.Param{{Name: "name", Desc: "foreign server name pattern, empty for every one", Default: ""}},
		Scan: func(rows *sql.Rows) (dbmeta.ForeignServer, error) {
			var v dbmeta.ForeignServer
			err := rows.Scan(&v.Name, &v.Owner, &v.Wrapper, &v.Type, &v.Version, &v.Access, &v.Options, &v.Comment)
			return v, err
		},
	})

	dbmeta.ForeignTables.Register(dbmeta.CrateDB, &dbmeta.Binding[dbmeta.ForeignTable]{
		Stmt: dbmeta.Stmt{
			always(`SELECT f.foreign_table_schema AS "schema"`),
			always(`, f.foreign_table_name AS "name"`),
			always(`, f.foreign_server_name AS "server"`),
			always(`, ` + options("foreign_table_options", `o.foreign_table_schema = f.foreign_table_schema`+
				` AND o.foreign_table_name = f.foreign_table_name`) + ` AS "options"`),
			always(`, NULL AS "comment"`),
			always(`FROM information_schema.foreign_tables f`),
			always(`WHERE (@schema = '' OR f.foreign_table_schema LIKE @schema)`),
			always(`AND (@name = '' OR f.foreign_table_name LIKE @name)`),
			always(`ORDER BY 1, 2`),
		},
		Fields: fields("schema", "name", "server", "options", "comment"),
		Params: []dbmeta.Param{
			{Name: "schema", Desc: "schema name pattern, empty for every schema", Default: ""},
			{Name: "name", Desc: "foreign table name pattern, empty for every one", Default: ""},
		},
		Scan: func(rows *sql.Rows) (dbmeta.ForeignTable, error) {
			var v dbmeta.ForeignTable
			err := rows.Scan(&v.Schema, &v.Name, &v.Server, &v.Options, &v.Comment)
			return v, err
		},
	})

	dbmeta.UserMappings.Register(dbmeta.CrateDB, &dbmeta.Binding[dbmeta.UserMapping]{
		Stmt: dbmeta.Stmt{
			always(`SELECT m.foreign_server_name AS "server"`),
			always(`, m.authorization_identifier AS "name"`),
			always(`, ` + options("user_mapping_options", `o.foreign_server_name = m.foreign_server_name`+
				` AND o.authorization_identifier = m.authorization_identifier`) + ` AS "options"`),
			always(`FROM information_schema.user_mappings m`),
			always(`WHERE (@name = '' OR m.authorization_identifier LIKE @name)`),
			always(`AND (@server = '' OR m.foreign_server_name LIKE @server)`),
			always(`ORDER BY 1, 2`),
		},
		Fields: fields("server", "name", "options"),
		Params: []dbmeta.Param{
			{Name: "name", Desc: "user name pattern, empty for every one", Default: ""},
			{Name: "server", Desc: "foreign server name pattern, empty for every one", Default: ""},
		},
		Scan: func(rows *sql.Rows) (dbmeta.UserMapping, error) {
			var v dbmeta.UserMapping
			err := rows.Scan(&v.Server, &v.Name, &v.Options)
			return v, err
		},
	})

	// A publication replicates inserts, updates and deletes and no TRUNCATE,
	// and has no partition root to publish through.
	dbmeta.Publications.Register(dbmeta.CrateDB, &dbmeta.Binding[dbmeta.Publication]{
		Stmt: dbmeta.Stmt{
			always(`SELECT p.pubname AS "name"`),
			always(`, r.rolname AS "owner"`),
			always(`, p.puballtables AS "all_tables"`),
			always(`, p.pubinsert AS "insert"`),
			always(`, p.pubupdate AS "update"`),
			always(`, p.pubdelete AS "delete"`),
			always(`, false AS "truncate"`),
			always(`, false AS "via_root"`),
			always(`, NULL AS "comment"`),
			always(`FROM pg_catalog.pg_publication p`),
			always(`JOIN pg_catalog.pg_roles r ON r.oid = p.pubowner`),
			always(`WHERE (@name = '' OR p.pubname LIKE @name)`),
			always(`ORDER BY 1`),
		},
		Fields: fields("name", "owner", "all_tables", "insert", "update", "delete", "truncate", "via_root", "comment"),
		Params: []dbmeta.Param{{Name: "name", Desc: "publication name pattern, empty for every one", Default: ""}},
		Scan: func(rows *sql.Rows) (dbmeta.Publication, error) {
			var v dbmeta.Publication
			err := rows.Scan(&v.Name, &v.Owner, &v.AllTables, &v.Insert, &v.Update, &v.Delete,
				&v.Truncate, &v.ViaRoot, &v.Comment)
			return v, err
		},
	})

	// A publication publishes whole tables, every column and every row.
	dbmeta.PublicationTables.Register(dbmeta.CrateDB, &dbmeta.Binding[dbmeta.PublicationTable]{
		Stmt: dbmeta.Stmt{
			always(`SELECT t.pubname AS "publication"`),
			always(`, t.schemaname AS "schema"`),
			always(`, t.tablename AS "name"`),
			always(`, '' AS "columns"`),
			always(`, NULL AS "where"`),
			always(`FROM pg_catalog.pg_publication_tables t`),
			always(`WHERE (@name = '' OR t.pubname LIKE @name)`),
			always(`ORDER BY 1, 2, 3`),
		},
		Fields: fields("publication", "schema", "name", "columns", "where"),
		Params: []dbmeta.Param{{Name: "name", Desc: "publication name pattern, empty for every one", Default: ""}},
		Scan: func(rows *sql.Rows) (dbmeta.PublicationTable, error) {
			var v dbmeta.PublicationTable
			err := rows.Scan(&v.Publication, &v.Schema, &v.Name, &v.Columns, &v.Where)
			return v, err
		},
	})

	dbmeta.Subscriptions.Register(dbmeta.CrateDB, &dbmeta.Binding[dbmeta.Subscription]{
		Stmt: dbmeta.Stmt{
			always(`SELECT s.subname AS "name"`),
			always(`, r.rolname AS "owner"`),
			always(`, s.subenabled AS "enabled"`),
			always(`, array_to_string(s.subpublications, ', ') AS "publications"`),
			always(`, s.subsynccommit AS "synchronous"`),
			always(`, s.subslotname AS "slot"`),
			always(`, NULL AS "comment"`),
			always(`FROM pg_catalog.pg_subscription s`),
			always(`JOIN pg_catalog.pg_roles r ON r.oid = s.subowner`),
			always(`WHERE (@name = '' OR s.subname LIKE @name)`),
			always(`ORDER BY 1`),
		},
		Fields: fields("name", "owner", "enabled", "publications", "synchronous", "slot", "comment"),
		Params: []dbmeta.Param{{Name: "name", Desc: "subscription name pattern, empty for every one", Default: ""}},
		Scan: func(rows *sql.Rows) (dbmeta.Subscription, error) {
			var v dbmeta.Subscription
			err := rows.Scan(&v.Name, &v.Owner, &v.Enabled, &v.Publications, &v.Synchronous, &v.Slot, &v.Comment)
			return v, err
		},
	})
}

func fields(names ...string) []dbmeta.Field { return dbmeta.Fields(names...) }

func schemaNameSystem(kind string) []dbmeta.Param {
	return []dbmeta.Param{
		{Name: "schema", Desc: "schema name pattern, empty for every schema", Default: ""},
		{Name: "name", Desc: kind + " name pattern, empty for every " + kind, Default: ""},
		{Name: "with_system", Desc: "include the objects CrateDB keeps for itself", Default: false},
	}
}

func schemaParentName(kind string) []dbmeta.Param {
	return []dbmeta.Param{
		{Name: "schema", Desc: "schema name pattern, empty for every schema", Default: ""},
		{Name: "parent", Desc: "table name pattern, empty for every table", Default: ""},
		{Name: "name", Desc: kind + " name pattern, empty for every " + kind, Default: ""},
		{Name: "with_system", Desc: "include the objects CrateDB keeps for itself", Default: false},
	}
}
