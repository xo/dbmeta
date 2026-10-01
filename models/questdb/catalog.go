package questdb

import (
	"database/sql"

	"github.com/xo/dbmeta"
)

// public is the one schema QuestDB reports. It has no schemas of its own, and
// its information_schema and pg_catalog put every table in public, which is
// what a PostgreSQL client expects to see.
const public = `'public'`

// inPublic is the schema filter for a relation, which is always in public.
const inPublic = `(@schema = '' OR ` + public + ` LIKE @schema)`

func always(query string) dbmeta.Choice { return dbmeta.Choice{{Query: query}} }

func fields(names ...string) []dbmeta.Field { return dbmeta.Fields(names...) }

// relationParams is the filter for a relation. QuestDB lists no relation of
// its own in tables(), views() or materialized_views(), so with_system is
// declared, so that a caller can pass it to every model, and changes nothing.
func relationParams(kind string) []dbmeta.Param {
	return []dbmeta.Param{
		{Name: "schema", Desc: "schema name pattern, empty for every schema", Default: ""},
		{Name: "name", Desc: kind + " name pattern, empty for every " + kind, Default: ""},
		{Name: "with_system", Desc: "changes nothing: QuestDB lists no relation of its own", Default: false},
	}
}

func schemaNameSystem(kind string) []dbmeta.Param {
	return []dbmeta.Param{
		{Name: "schema", Desc: "schema name pattern, empty for every schema", Default: ""},
		{Name: "name", Desc: kind + " name pattern, empty for every " + kind, Default: ""},
		{Name: "with_system", Desc: "include the objects QuestDB keeps for itself", Default: false},
	}
}

// register registers every statement the model answers.
func register() {
	registerRelations()
	registerServer()
	registerRoutines()
}

// tableType is the word for a table's kind, from the letter tables() gives.
const tableType = `CASE t.table_type WHEN 'T' THEN 'table' WHEN 'V' THEN 'view'` +
	` WHEN 'M' THEN 'materialized view' ELSE lower(t.table_type) END`

func registerRelations() {
	// pg_namespace holds public and pg_catalog, which QuestDB imitates for a
	// PostgreSQL client. Neither has an owner QuestDB keeps.
	dbmeta.Schemas.Register(dbmeta.QuestDB, &dbmeta.Binding[dbmeta.Schema]{
		Stmt: dbmeta.Stmt{
			always(`SELECT current_database() AS "catalog"`),
			always(`, n.nspname AS "name"`),
			always(`, '' AS "owner"`),
			always(`, NULL AS "comment"`),
			always(`FROM pg_catalog.pg_namespace n`),
			always(`WHERE (@with_system OR n.nspname <> 'pg_catalog')`),
			always(`AND (@name = '' OR n.nspname LIKE @name)`),
			always(`ORDER BY 2`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog"}, {Name: "name"},
			{Name: "owner", Desc: "always empty: QuestDB keeps no owner for a schema"},
			{Name: "comment", Desc: "always absent: QuestDB has no COMMENT statement"},
		},
		Params: []dbmeta.Param{
			{Name: "name", Desc: "schema name pattern, empty for every schema", Default: ""},
			{Name: "with_system", Desc: "include pg_catalog, which QuestDB imitates for a PostgreSQL client", Default: false},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Schema, error) {
			var v dbmeta.Schema
			err := rows.Scan(&v.Catalog, &v.Name, &v.Owner, &v.Comment)
			return v, err
		},
	})

	dbmeta.CurrentSchema.Register(dbmeta.QuestDB, &dbmeta.Binding[dbmeta.Schema]{
		Stmt: dbmeta.Stmt{
			always(`SELECT current_database() AS "catalog"`),
			always(`, current_schema() AS "name"`),
			always(`, '' AS "owner"`),
			always(`, NULL AS "comment"`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog"}, {Name: "name"},
			{Name: "owner", Desc: "always empty: QuestDB keeps no owner for a schema"},
			{Name: "comment", Desc: "always absent: QuestDB has no COMMENT statement"},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Schema, error) {
			var v dbmeta.Schema
			err := rows.Scan(&v.Catalog, &v.Name, &v.Owner, &v.Comment)
			return v, err
		},
	})

	// tables() lists every table, view and materialized view, each with its
	// kind as one letter. QuestDB keeps no system table in it.
	dbmeta.Tables.Register(dbmeta.QuestDB, &dbmeta.Binding[dbmeta.Table]{
		Stmt: dbmeta.Stmt{
			always(`SELECT current_database() AS "catalog"`),
			always(`, ` + public + ` AS "schema"`),
			always(`, t.table_name AS "name"`),
			always(`, ` + tableType + ` AS "type"`),
			always(`, NULL AS "comment"`),
			always(`FROM tables() t`),
			always(`WHERE ` + inPublic),
			always(`AND (@name = '' OR t.table_name LIKE @name)`),
			// QuestDB refuses a LIKE whose pattern is not a constant, so the
			// list is searched with strpos.
			always(`AND (@types = '' OR strpos(',' || CAST(@types AS varchar) || ',', ',' || ` + tableType + ` || ',') > 0)`),
			always(`ORDER BY 3`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog"},
			{Name: "schema", Desc: "always public, the one schema QuestDB reports"},
			{Name: "name"}, {Name: "type"},
			{Name: "comment", Desc: "always absent: QuestDB has no COMMENT statement"},
		},
		Params: append(relationParams("relation"), dbmeta.TypesParam()),
		Scan: func(rows *sql.Rows) (dbmeta.Table, error) {
			var v dbmeta.Table
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Name, &v.Type, &v.Comment)
			return v, err
		},
	})

	// information_schema.columns lists the columns of every relation in one
	// statement, which table_columns() cannot, because it takes one table
	// name as a constant. The type is the PostgreSQL type the wire protocol
	// sends, so a SYMBOL column reads character varying and a DECIMAL reads
	// numeric. The position counts from 0, and the model adds 1, so that the
	// first column is 1 as it is on every other database. QuestDB has no NOT
	// NULL, no default and no key, so every column is nullable and none is a
	// key.
	dbmeta.Columns.Register(dbmeta.QuestDB, &dbmeta.Binding[dbmeta.Column]{
		Stmt: dbmeta.Stmt{
			always(`SELECT c.table_catalog AS "catalog"`),
			always(`, c.table_schema AS "schema"`),
			always(`, c.table_name AS "table"`),
			always(`, c.column_name AS "name"`),
			always(`, c.ordinal_position + 1 AS "ordinal"`),
			always(`, c.data_type AS "data_type"`),
			always(`, c.is_nullable = 'yes' AS "nullable"`),
			always(`, c.column_default AS "default"`),
			always(`, false AS "primary_key"`),
			always(`, '' AS "identity"`),
			always(`, '' AS "generated"`),
			always(`, NULL AS "comment"`),
			always(`, NULL AS "collation"`),
			always(`FROM information_schema.columns c`),
			always(`WHERE (@schema = '' OR c.table_schema LIKE @schema)`),
			always(`AND (@parent = '' OR c.table_name LIKE @parent)`),
			always(`AND (@name = '' OR c.column_name LIKE @name)`),
			always(`ORDER BY 3, 5`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog"}, {Name: "schema"}, {Name: "table"}, {Name: "name"},
			{Name: "ordinal", Desc: "the position from 1. QuestDB reports it from 0, and the model adds 1"},
			{Name: "data_type", Desc: "the PostgreSQL type QuestDB sends, so SYMBOL reads character varying. The QuestDB type is only in table_columns, which reads one table at a time"},
			{Name: "nullable", Desc: "always true: QuestDB has no NOT NULL"},
			{Name: "default", Desc: "always absent in practice: QuestDB has no column default"},
			{Name: "primary_key", Desc: "always false: QuestDB has no primary key"},
			{Name: "identity", Desc: "always empty: QuestDB has no identity column"},
			{Name: "generated", Desc: "always empty: QuestDB has no generated column"},
			{Name: "comment", Desc: "always absent: QuestDB has no COMMENT statement"},
			{Name: "collation", Desc: "always absent: QuestDB has no collation"},
		},
		Params: []dbmeta.Param{
			{Name: "schema", Desc: "schema name pattern, empty for every schema", Default: ""},
			{Name: "parent", Desc: "relation name pattern, empty for every relation", Default: ""},
			{Name: "name", Desc: "column name pattern, empty for every column", Default: ""},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Column, error) {
			var v dbmeta.Column
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Table, &v.Name, &v.Ordinal, &v.DataType,
				&v.Nullable, &v.Default, &v.PrimaryKey, &v.Identity, &v.Generated, &v.Comment, &v.Collation)
			return v, err
		},
	})

	// A view and a materialized view, as the postgres model reports both. A
	// QuestDB view refuses an insert, an update and a delete.
	dbmeta.Views.Register(dbmeta.QuestDB, &dbmeta.Binding[dbmeta.View]{
		Stmt: dbmeta.Stmt{
			always(`SELECT r."catalog", r."schema", r."name", r."definition", r."check_option"` +
				`, r."updatable", r."insertable", r."comment" FROM (`),
			always(`SELECT current_database() AS "catalog", ` + public + ` AS "schema"` +
				`, v.view_name AS "name", v.view_sql AS "definition"` +
				`, NULL AS "check_option", false AS "updatable", false AS "insertable", NULL AS "comment"` +
				` FROM views() v`),
			always(`UNION ALL`),
			always(`SELECT current_database() AS "catalog", ` + public + ` AS "schema"` +
				`, m.view_name AS "name", m.view_sql AS "definition"` +
				`, NULL AS "check_option", false AS "updatable", false AS "insertable", NULL AS "comment"` +
				` FROM materialized_views() m`),
			always(`) r`),
			always(`WHERE ` + inPublic),
			always(`AND (@name = '' OR r."name" LIKE @name)`),
			always(`ORDER BY 3`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog"},
			{Name: "schema", Desc: "always public, the one schema QuestDB reports"},
			{Name: "name"},
			{Name: "definition", Desc: "the SELECT the view was created with"},
			{Name: "check_option", Desc: "always absent: QuestDB has no WITH CHECK OPTION"},
			{Name: "updatable", Desc: "always false: QuestDB refuses an update through a view"},
			{Name: "insertable", Desc: "always false: QuestDB refuses an insert through a view"},
			{Name: "comment", Desc: "always absent: QuestDB has no COMMENT statement"},
		},
		Params: relationParams("view"),
		Scan: func(rows *sql.Rows) (dbmeta.View, error) {
			var v dbmeta.View
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Name, &v.Definition,
				&v.CheckOption, &v.Updatable, &v.Insertable, &v.Comment)
			return v, err
		},
	})

	// A partitioned table is one whose designated timestamp divides it into a
	// partition for each interval, which is PostgreSQL's RANGE. The
	// expression names the interval and the column, as YEAR (published).
	dbmeta.PartitionedTables.Register(dbmeta.QuestDB, &dbmeta.Binding[dbmeta.PartitionedTable]{
		Stmt: dbmeta.Stmt{
			always(`SELECT ` + public + ` AS "schema"`),
			always(`, t.table_name AS "name"`),
			always(`, '' AS "owner"`),
			always(`, CASE t.table_type WHEN 'M' THEN 'materialized view' ELSE 'table' END AS "type"`),
			always(`, '' AS "parent"`),
			always(`, 'RANGE' AS "strategy"`),
			always(`, t.partitionBy || ' (' || t.designatedTimestamp || ')' AS "expression"`),
			always(`, NULL AS "comment"`),
			always(`FROM tables() t`),
			always(`WHERE t.partitionBy NOT IN ('NONE', 'N/A')`),
			always(`AND ` + inPublic),
			always(`AND (@name = '' OR t.table_name LIKE @name)`),
			always(`ORDER BY 2`),
		},
		Fields: []dbmeta.Field{
			{Name: "schema", Desc: "always public, the one schema QuestDB reports"},
			{Name: "name"},
			{Name: "owner", Desc: "always empty: QuestDB keeps no owner for a table"},
			{Name: "type", Desc: "table, or materialized view, which QuestDB partitions the same way"},
			{Name: "parent", Desc: "always empty: a QuestDB partition is not a table of its own"},
			{Name: "strategy", Desc: "always RANGE: QuestDB partitions by an interval of the designated timestamp"},
			{Name: "expression", Desc: "the interval and the designated timestamp, as YEAR (published)"},
			{Name: "comment", Desc: "always absent: QuestDB has no COMMENT statement"},
		},
		Params: relationParams("partitioned table"),
		Scan: func(rows *sql.Rows) (dbmeta.PartitionedTable, error) {
			var v dbmeta.PartitionedTable
			err := rows.Scan(&v.Schema, &v.Name, &v.Owner, &v.Type, &v.Parent, &v.Strategy, &v.Expression, &v.Comment)
			return v, err
		},
	})
}

func registerServer() {
	// pg_database holds qdb, the one database a QuestDB server has.
	dbmeta.Databases.Register(dbmeta.QuestDB, &dbmeta.Binding[dbmeta.Database]{
		Stmt: dbmeta.Stmt{
			always(`SELECT d.datname AS "name"`),
			always(`, '' AS "owner"`),
			always(`, 'UTF8' AS "encoding"`),
			always(`, d.datcollate AS "collate"`),
			always(`, d.datctype AS "ctype"`),
			always(`, NULL AS "access"`),
			always(`, NULL AS "tablespace"`),
			always(`, NULL AS "size"`),
			always(`, NULL AS "comment"`),
			always(`FROM pg_catalog.pg_database d`),
			always(`WHERE (@name = '' OR d.datname LIKE @name)`),
			always(`ORDER BY 1`),
		},
		Fields: []dbmeta.Field{
			{Name: "name"},
			{Name: "owner", Desc: "always empty: QuestDB keeps no owner for a database"},
			{Name: "encoding", Desc: "always UTF8, which is the only encoding QuestDB stores text in"},
			{Name: "collate"}, {Name: "ctype"},
			{Name: "access", Desc: "always absent: the open source edition has no privileges"},
			{Name: "tablespace", Desc: "always absent: QuestDB has no tablespaces"},
			{Name: "size", Desc: "always absent: table_storage() has the size of each table, and pg_database has none"},
			{Name: "comment", Desc: "always absent: QuestDB has no COMMENT statement"},
		},
		Params: []dbmeta.Param{{Name: "name", Desc: "database name pattern, empty for every database", Default: ""}},
		Scan: func(rows *sql.Rows) (dbmeta.Database, error) {
			var v dbmeta.Database
			err := rows.Scan(&v.Name, &v.Owner, &v.Encoding, &v.Collate, &v.CType,
				&v.Access, &v.Tablespace, &v.Size, &v.Comment)
			return v, err
		},
	})

	// SHOW PARAMETERS is a relation QuestDB lets a query select from. It has
	// no type for a value. Whether a setting takes effect without a restart
	// is what context says, in QuestDB's words rather than PostgreSQL's.
	dbmeta.Settings.Register(dbmeta.QuestDB, &dbmeta.Binding[dbmeta.Setting]{
		Stmt: dbmeta.Stmt{
			always(`SELECT p.property_path AS "name"`),
			always(`, p.value AS "value"`),
			always(`, NULL AS "type"`),
			always(`, CASE WHEN p.reloadable THEN 'reloadable' ELSE 'restart' END AS "context"`),
			always(`, NULL AS "access"`),
			always(`, NULL AS "display"`),
			always(`FROM (SHOW PARAMETERS) p`),
			always(`WHERE (@name = '' OR p.property_path LIKE @name)`),
			always(`ORDER BY 1`),
		},
		Fields: []dbmeta.Field{
			{Name: "name", Desc: "the property, as server.conf names it"},
			{Name: "value", Desc: "the value, which QuestDB masks where the setting is sensitive"},
			{Name: "type", Desc: "always absent: QuestDB records no type for a setting"},
			{Name: "context", Desc: "reloadable where the setting takes effect without a restart, and restart otherwise"},
			{Name: "access", Desc: "always absent: the open source edition has no privileges"},
			{Name: "display", Desc: "always absent: QuestDB shows a value in one form, which is value"},
		},
		Params: []dbmeta.Param{{Name: "name", Desc: "parameter name pattern, empty for every parameter", Default: ""}},
		Scan: func(rows *sql.Rows) (dbmeta.Setting, error) {
			var v dbmeta.Setting
			err := rows.Scan(&v.Name, &v.Value, &v.Type, &v.Context, &v.Access, &v.Display)
			return v, err
		},
	})

	dbmeta.CurrentUser.Register(dbmeta.QuestDB, &dbmeta.Binding[dbmeta.User]{
		Stmt: dbmeta.Stmt{
			always(`SELECT current_user AS "name"`),
			always(`, session_user AS "session"`),
		},
		Fields: fields("name", "session"),
		Scan: func(rows *sql.Rows) (dbmeta.User, error) {
			var v dbmeta.User
			err := rows.Scan(&v.Name, &v.Session)
			return v, err
		},
	})
}

func registerRoutines() {
	dbmeta.Functions.Register(dbmeta.QuestDB, functions(`f.type <> 'GROUP_BY'`))
	dbmeta.Aggregates.Register(dbmeta.QuestDB, functions(`f.type = 'GROUP_BY'`))
}

// functions reads functions(), which lists every function QuestDB has, one
// row per signature. Every one is built in, because QuestDB has no CREATE
// FUNCTION, so a caller sees them only with with_system. A signature can be
// both an aggregate and a window function, so the id carries the kind.
func functions(kind string) *dbmeta.Binding[dbmeta.Function] {
	return &dbmeta.Binding[dbmeta.Function]{
		Stmt: dbmeta.Stmt{
			always(`SELECT current_database() AS "catalog"`),
			always(`, '' AS "schema"`),
			always(`, f.name AS "name"`),
			always(`, f.type || ' ' || f.signature AS "id"`),
			always(`, CASE f.type WHEN 'GROUP_BY' THEN 'agg' WHEN 'WINDOW' THEN 'window' ELSE 'func' END AS "kind"`),
			always(`, NULL AS "result_type"`),
			always(`, regexp_replace(f.signature_translated, '^[^(]*\((.*)\)$', '$1') AS "arg_types"`),
			always(`, '' AS "volatility"`),
			always(`, '' AS "parallel"`),
			always(`, NULL AS "owner"`),
			always(`, '' AS "security"`),
			always(`, NULL AS "access"`),
			always(`, 'internal' AS "language"`),
			always(`, NULL AS "source"`),
			always(`, NULL AS "comment"`),
			always(`, NULL AS "definition"`),
			always(`FROM functions() f`),
			always(`WHERE @with_system`),
			always(`AND ` + kind),
			always(`AND (@schema = '' OR '' LIKE @schema)`),
			always(`AND (@name = '' OR f.name LIKE @name)`),
			always(`ORDER BY 3, 4`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog"},
			{Name: "schema", Desc: "always empty: a QuestDB function belongs to no schema"},
			{Name: "name"},
			{Name: "id", Desc: "the kind and the signature, such as GROUP_BY sum(D), because a signature can be both an aggregate and a window function"},
			{Name: "kind", Desc: "agg for an aggregate, window for a window function, and func for the rest"},
			{Name: "result_type", Desc: "always absent: functions() records no result type"},
			{Name: "arg_types", Desc: "the argument types, as signature_translated writes them"},
			{Name: "volatility", Desc: "always empty: QuestDB marks no function as immutable or volatile"},
			{Name: "parallel", Desc: "always empty: QuestDB marks no parallel safety"},
			{Name: "owner", Desc: "always absent: a built in function has no owner"},
			{Name: "security", Desc: "always empty: a built in function has no security mode"},
			{Name: "access", Desc: "always absent: the open source edition has no privileges"},
			{Name: "language", Desc: "always internal: every function is built into the server"},
			{Name: "source", Desc: "always absent: a built in function has no source text"},
			{Name: "comment", Desc: "always absent: QuestDB has no COMMENT statement"},
			{Name: "definition", Desc: "always absent, for the same reason as source"},
		},
		Params: schemaNameSystem("function"),
		Scan: func(rows *sql.Rows) (dbmeta.Function, error) {
			var v dbmeta.Function
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Name, &v.ID, &v.Kind, &v.ResultType, &v.ArgTypes,
				&v.Volatility, &v.Parallel, &v.Owner, &v.Security, &v.Access, &v.Language, &v.Source, &v.Comment,
				&v.Definition)
			return v, err
		},
	}
}
