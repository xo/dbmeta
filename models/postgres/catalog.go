package postgres

import (
	"database/sql"

	"github.com/xo/dbmeta"
)

// Server wide and simple catalog objects. Each query names the psql command it
// backs and the function in describe.c it was translated from.

func init() {
	registerDatabases()
	registerTablespaces()
	registerAccessMethods()
	registerLanguages()
	registerConversions()
	registerCasts()
	registerCollations()
	registerLargeObjects()
	registerEventTriggers()
	registerSettings()
}

// registerDatabases backs \l, from listAllDbs.
func registerDatabases() {
	dbmeta.Databases.Register(dbmeta.PostgreSQL, &dbmeta.Binding[dbmeta.Database]{
		Stmt: dbmeta.Stmt{
			{{SQL: `SELECT d.datname AS "name"`}},
			{{SQL: `, pg_catalog.pg_get_userbyid(d.datdba) AS "owner"`}},
			{{SQL: `, pg_catalog.pg_encoding_to_char(d.encoding) AS "encoding"`}},
			{{SQL: `, d.datcollate AS "collate"`}},
			{{SQL: `, d.datctype AS "ctype"`}},
			{{SQL: `, pg_catalog.array_to_string(d.datacl, E'\n') AS "access"`}},
			{{SQL: `, t.spcname AS "tablespace"`}},
			{{SQL: `, CASE WHEN pg_catalog.has_database_privilege(d.datname, 'CONNECT')` +
				` THEN pg_catalog.pg_size_pretty(pg_catalog.pg_database_size(d.datname))` +
				` ELSE 'no access' END AS "size"`}},
			{{SQL: `, pg_catalog.shobj_description(d.oid, 'pg_database') AS "comment"`}},
			{{SQL: `FROM pg_catalog.pg_database d`}},
			{{SQL: `LEFT JOIN pg_catalog.pg_tablespace t ON t.oid = d.dattablespace`}},
			{{SQL: `WHERE (@name = '' OR d.datname LIKE @name)`}},
			{{SQL: `ORDER BY 1`}},
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
}

// registerTablespaces backs \db, from describeTablespaces.
func registerTablespaces() {
	dbmeta.Tablespaces.Register(dbmeta.PostgreSQL, &dbmeta.Binding[dbmeta.Tablespace]{
		Stmt: dbmeta.Stmt{
			{{SQL: `SELECT spcname AS "name"`}},
			{{SQL: `, pg_catalog.pg_get_userbyid(spcowner) AS "owner"`}},
			{{SQL: `, pg_catalog.pg_tablespace_location(oid) AS "location"`}},
			{{SQL: `, pg_catalog.array_to_string(spcoptions, ', ') AS "options"`}},
			{{SQL: `, pg_catalog.pg_size_pretty(pg_catalog.pg_tablespace_size(oid)) AS "size"`}},
			{{SQL: `, pg_catalog.array_to_string(spcacl, E'\n') AS "access"`}},
			{{SQL: `, pg_catalog.shobj_description(oid, 'pg_tablespace') AS "comment"`}},
			{{SQL: `FROM pg_catalog.pg_tablespace`}},
			{{SQL: `WHERE (@name = '' OR spcname LIKE @name)`}},
			{{SQL: `ORDER BY 1`}},
		},
		Fields: fields("name", "owner", "location", "options", "size", "access", "comment"),
		Params: []dbmeta.Param{{Name: "name", Desc: "tablespace name pattern, empty for every tablespace", Default: ""}},
		Scan: func(rows *sql.Rows) (dbmeta.Tablespace, error) {
			var v dbmeta.Tablespace
			err := rows.Scan(&v.Name, &v.Owner, &v.Location, &v.Options, &v.Size, &v.Access, &v.Comment)
			return v, err
		},
	})
}

// registerAccessMethods backs \dA, from describeAccessMethods.
//
// pg_am gained amtype and amhandler in release 9.6, which is the floor, so no
// fragment is gated.
func registerAccessMethods() {
	dbmeta.AccessMethods.Register(dbmeta.PostgreSQL, &dbmeta.Binding[dbmeta.AccessMethod]{
		Stmt: dbmeta.Stmt{
			{{SQL: `SELECT amname AS "name"`}},
			{{SQL: `, CASE amtype WHEN 'i' THEN 'index' WHEN 't' THEN 'table' ELSE amtype::text END AS "type"`}},
			{{SQL: `, amhandler::text AS "handler"`}},
			{{SQL: `, pg_catalog.obj_description(oid, 'pg_am') AS "comment"`}},
			{{SQL: `FROM pg_catalog.pg_am`}},
			{{SQL: `WHERE (@name = '' OR amname LIKE @name)`}},
			{{SQL: `ORDER BY 1`}},
		},
		Fields: fields("name", "type", "handler", "comment"),
		Params: []dbmeta.Param{{Name: "name", Desc: "access method name pattern, empty for every one", Default: ""}},
		Scan: func(rows *sql.Rows) (dbmeta.AccessMethod, error) {
			var v dbmeta.AccessMethod
			err := rows.Scan(&v.Name, &v.Type, &v.Handler, &v.Comment)
			return v, err
		},
	})
}

// registerLanguages backs \dL, from listLanguages.
func registerLanguages() {
	dbmeta.Languages.Register(dbmeta.PostgreSQL, &dbmeta.Binding[dbmeta.Language]{
		Stmt: dbmeta.Stmt{
			{{SQL: `SELECT l.lanname AS "name"`}},
			{{SQL: `, pg_catalog.pg_get_userbyid(l.lanowner) AS "owner"`}},
			{{SQL: `, l.lanpltrusted AS "trusted"`}},
			{{SQL: `, NOT l.lanispl AS "internal"`}},
			{{SQL: `, l.lanplcallfoid::pg_catalog.regprocedure::text AS "handler"`}},
			{{SQL: `, l.lanvalidator::pg_catalog.regprocedure::text AS "validator"`}},
			{{SQL: `, l.laninline::pg_catalog.regprocedure::text AS "inline"`}},
			{{SQL: `, pg_catalog.array_to_string(l.lanacl, E'\n') AS "access"`}},
			{{SQL: `, d.description AS "comment"`}},
			{{SQL: `FROM pg_catalog.pg_language l`}},
			{{SQL: `LEFT JOIN pg_catalog.pg_description d ON d.objoid = l.oid AND d.classoid = 'pg_language'::regclass`}},
			{{SQL: `WHERE l.lanplcallfoid != 0`}},
			{{SQL: `AND (@name = '' OR l.lanname LIKE @name)`}},
			{{SQL: `ORDER BY 1`}},
		},
		Fields: fields("name", "owner", "trusted", "internal", "handler", "validator", "inline", "access", "comment"),
		Params: []dbmeta.Param{{Name: "name", Desc: "language name pattern, empty for every language", Default: ""}},
		Scan: func(rows *sql.Rows) (dbmeta.Language, error) {
			var v dbmeta.Language
			err := rows.Scan(&v.Name, &v.Owner, &v.Trusted, &v.Internal,
				&v.Handler, &v.Validator, &v.Inline, &v.Access, &v.Comment)
			return v, err
		},
	})
}

// registerConversions backs \dc, from listConversions.
func registerConversions() {
	dbmeta.Conversions.Register(dbmeta.PostgreSQL, &dbmeta.Binding[dbmeta.Conversion]{
		Stmt: dbmeta.Stmt{
			{{SQL: `SELECT n.nspname AS "schema"`}},
			{{SQL: `, c.conname AS "name"`}},
			{{SQL: `, pg_catalog.pg_encoding_to_char(c.conforencoding) AS "source"`}},
			{{SQL: `, pg_catalog.pg_encoding_to_char(c.contoencoding) AS "target"`}},
			{{SQL: `, c.condefault AS "default"`}},
			{{SQL: `, d.description AS "comment"`}},
			{{SQL: `FROM pg_catalog.pg_conversion c`}},
			{{SQL: `JOIN pg_catalog.pg_namespace n ON n.oid = c.connamespace`}},
			{{SQL: `LEFT JOIN pg_catalog.pg_description d ON d.objoid = c.oid AND d.classoid = 'pg_conversion'::regclass`}},
			{{SQL: `WHERE (@with_system OR (n.nspname <> 'pg_catalog' AND n.nspname <> 'information_schema'))`}},
			{{SQL: `AND (@schema = '' OR n.nspname LIKE @schema)`}},
			{{SQL: `AND (@name = '' OR c.conname LIKE @name)`}},
			{{SQL: `ORDER BY 1, 2`}},
		},
		Fields: fields("schema", "name", "source", "target", "default", "comment"),
		Params: schemaNameSystem("conversion"),
		Scan: func(rows *sql.Rows) (dbmeta.Conversion, error) {
			var v dbmeta.Conversion
			err := rows.Scan(&v.Schema, &v.Name, &v.Source, &v.Target, &v.Default, &v.Comment)
			return v, err
		},
	})
}

// registerCasts backs \dC, from listCasts.
func registerCasts() {
	dbmeta.Casts.Register(dbmeta.PostgreSQL, &dbmeta.Binding[dbmeta.Cast]{
		Stmt: dbmeta.Stmt{
			{{SQL: `SELECT pg_catalog.format_type(c.castsource, NULL) AS "source"`}},
			{{SQL: `, pg_catalog.format_type(c.casttarget, NULL) AS "target"`}},
			{{SQL: `, CASE WHEN c.castmethod = 'b' THEN '(binary coercible)'` +
				` WHEN c.castmethod = 'i' THEN '(with inout)'` +
				` ELSE p.proname::text END AS "function"`}},
			{{SQL: `, CASE WHEN c.castcontext = 'e' THEN 'no'` +
				` WHEN c.castcontext = 'a' THEN 'in assignment'` +
				` ELSE 'yes' END AS "implicit"`}},
			{{SQL: `, p.proleakproof AS "leakproof"`}},
			{{SQL: `, d.description AS "comment"`}},
			{{SQL: `FROM pg_catalog.pg_cast c`}},
			{{SQL: `LEFT JOIN pg_catalog.pg_proc p ON c.castfunc = p.oid`}},
			{{SQL: `LEFT JOIN pg_catalog.pg_description d ON d.objoid = c.oid AND d.classoid = 'pg_cast'::regclass`}},
			{{SQL: `WHERE (@name = '' OR pg_catalog.format_type(c.castsource, NULL) LIKE @name` +
				` OR pg_catalog.format_type(c.casttarget, NULL) LIKE @name)`}},
			{{SQL: `ORDER BY 1, 2`}},
		},
		Fields: fields("source", "target", "function", "implicit", "leakproof", "comment"),
		Params: []dbmeta.Param{{Name: "name", Desc: "type name pattern on either side, empty for every cast", Default: ""}},
		Scan: func(rows *sql.Rows) (dbmeta.Cast, error) {
			var v dbmeta.Cast
			err := rows.Scan(&v.Source, &v.Target, &v.Function, &v.Implicit, &v.LeakProof, &v.Comment)
			return v, err
		},
	})
}

// registerCollations backs \dO, from listCollations.
//
// Three fragments are gated. collprovider arrived in release 10, and the ICU
// locale and the deterministic flag arrived in release 12.
func registerCollations() {
	dbmeta.Collations.Register(dbmeta.PostgreSQL, &dbmeta.Binding[dbmeta.Collation]{
		Stmt: dbmeta.Stmt{
			{{SQL: `SELECT n.nspname AS "schema"`}},
			{{SQL: `, c.collname AS "name"`}},
			{
				{SQL: `, '' AS "provider"`},
				{Min: v10, SQL: `, CASE c.collprovider WHEN 'd' THEN 'default' WHEN 'c' THEN 'libc' WHEN 'i' THEN 'icu' WHEN 'b' THEN 'builtin' ELSE '' END AS "provider"`},
			},
			{{SQL: `, c.collcollate AS "collate"`}},
			{{SQL: `, c.collctype AS "ctype"`}},
			{
				{SQL: `, '' AS "locale"`},
				{Min: v12, SQL: `, c.colliculocale AS "locale"`},
				{Min: v17, SQL: `, c.colllocale AS "locale"`},
			},
			{
				{SQL: `, true AS "deterministic"`},
				{Min: v12, SQL: `, c.collisdeterministic AS "deterministic"`},
			},
			{{SQL: `, pg_catalog.obj_description(c.oid, 'pg_collation') AS "comment"`}},
			{{SQL: `FROM pg_catalog.pg_collation c`}},
			{{SQL: `JOIN pg_catalog.pg_namespace n ON n.oid = c.collnamespace`}},
			{{SQL: `WHERE (@with_system OR (n.nspname <> 'pg_catalog' AND n.nspname <> 'information_schema'))`}},
			{{SQL: `AND (@schema = '' OR n.nspname LIKE @schema)`}},
			{{SQL: `AND (@name = '' OR c.collname LIKE @name)`}},
			{{SQL: `ORDER BY 1, 2`}},
		},
		Fields: []dbmeta.Field{
			{Name: "schema"},
			{Name: "name"},
			{Name: "provider", Desc: "libc, icu or builtin", Min: v10},
			{Name: "collate"},
			{Name: "ctype"},
			{Name: "locale", Desc: "ICU locale, empty when the provider is not ICU", Min: v12},
			{Name: "deterministic", Desc: "whether equal strings are always identical", Min: v12},
			{Name: "comment"},
		},
		Params: schemaNameSystem("collation"),
		Scan: func(rows *sql.Rows) (dbmeta.Collation, error) {
			var v dbmeta.Collation
			err := rows.Scan(&v.Schema, &v.Name, &v.Provider, &v.Collate, &v.CType,
				&v.Locale, &v.Deterministic, &v.Comment)
			return v, err
		},
	})
}

// registerLargeObjects backs \dl, from listLargeObjects.
func registerLargeObjects() {
	dbmeta.LargeObjects.Register(dbmeta.PostgreSQL, &dbmeta.Binding[dbmeta.LargeObject]{
		Stmt: dbmeta.Stmt{
			{{SQL: `SELECT o.oid AS "oid"`}},
			{{SQL: `, pg_catalog.pg_get_userbyid(o.lomowner) AS "owner"`}},
			{{SQL: `, pg_catalog.array_to_string(o.lomacl, E'\n') AS "access"`}},
			{{SQL: `, pg_catalog.obj_description(o.oid, 'pg_largeobject') AS "comment"`}},
			{{SQL: `FROM pg_catalog.pg_largeobject_metadata o`}},
			{{SQL: `ORDER BY 1`}},
		},
		Fields: fields("oid", "owner", "access", "comment"),
		Scan: func(rows *sql.Rows) (dbmeta.LargeObject, error) {
			var v dbmeta.LargeObject
			err := rows.Scan(&v.OID, &v.Owner, &v.Access, &v.Comment)
			return v, err
		},
	})
}

// registerEventTriggers backs \dy, from listEventTriggers.
func registerEventTriggers() {
	dbmeta.EventTriggers.Register(dbmeta.PostgreSQL, &dbmeta.Binding[dbmeta.EventTrigger]{
		Stmt: dbmeta.Stmt{
			{{SQL: `SELECT e.evtname AS "name"`}},
			{{SQL: `, e.evtevent AS "event"`}},
			{{SQL: `, pg_catalog.pg_get_userbyid(e.evtowner) AS "owner"`}},
			{{SQL: `, CASE e.evtenabled WHEN 'O' THEN 'enabled' WHEN 'R' THEN 'replica'` +
				` WHEN 'A' THEN 'always' WHEN 'D' THEN 'disabled' ELSE '' END AS "enabled"`}},
			{{SQL: `, e.evtfoid::pg_catalog.regproc::text AS "function"`}},
			{{SQL: `, pg_catalog.array_to_string(e.evttags, ', ') AS "tags"`}},
			{{SQL: `, pg_catalog.obj_description(e.oid, 'pg_event_trigger') AS "comment"`}},
			{{SQL: `FROM pg_catalog.pg_event_trigger e`}},
			{{SQL: `WHERE (@name = '' OR e.evtname LIKE @name)`}},
			{{SQL: `ORDER BY 1`}},
		},
		Fields: fields("name", "event", "owner", "enabled", "function", "tags", "comment"),
		Params: []dbmeta.Param{{Name: "name", Desc: "event trigger name pattern, empty for every one", Default: ""}},
		Scan: func(rows *sql.Rows) (dbmeta.EventTrigger, error) {
			var v dbmeta.EventTrigger
			err := rows.Scan(&v.Name, &v.Event, &v.Owner, &v.Enabled, &v.Function, &v.Tags, &v.Comment)
			return v, err
		},
	})
}

// registerSettings backs \dconfig, from describeConfigurationParameters.
//
// pg_parameter_acl arrived in release 15, so the access column pads below it.
func registerSettings() {
	dbmeta.Settings.Register(dbmeta.PostgreSQL, &dbmeta.Binding[dbmeta.Setting]{
		Stmt: dbmeta.Stmt{
			{{SQL: `SELECT s.name AS "name"`}},
			{{SQL: `, s.setting AS "value"`}},
			{{SQL: `, s.vartype AS "type"`}},
			{{SQL: `, s.context AS "context"`}},
			{
				{SQL: `, '' AS "access"`},
				{Min: v15, SQL: `, pg_catalog.array_to_string(p.paracl, E'\n') AS "access"`},
			},
			{{SQL: `FROM pg_catalog.pg_settings s`}},
			{
				{SQL: ``},
				{Min: v15, SQL: `LEFT JOIN pg_catalog.pg_parameter_acl p ON p.parname = s.name`},
			},
			{{SQL: `WHERE (@name = '' OR s.name LIKE @name)`}},
			{{SQL: `ORDER BY 1`}},
		},
		Fields: []dbmeta.Field{
			{Name: "name"}, {Name: "value"}, {Name: "type"}, {Name: "context"},
			{Name: "access", Desc: "grants on the parameter", Min: v15},
		},
		Params: []dbmeta.Param{{Name: "name", Desc: "parameter name pattern, empty for every parameter", Default: ""}},
		Scan: func(rows *sql.Rows) (dbmeta.Setting, error) {
			var v dbmeta.Setting
			err := rows.Scan(&v.Name, &v.Value, &v.Type, &v.Context, &v.Access)
			return v, err
		},
	})
}
