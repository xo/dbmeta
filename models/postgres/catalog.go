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
			{{Query: `SELECT d.datname AS "name"`}},
			{{Query: `, pg_catalog.pg_get_userbyid(d.datdba) AS "owner"`}},
			{{Query: `, pg_catalog.pg_encoding_to_char(d.encoding) AS "encoding"`}},
			{{Query: `, d.datcollate AS "collate"`}},
			{{Query: `, d.datctype AS "ctype"`}},
			{{Query: `, pg_catalog.array_to_string(d.datacl, E'\n') AS "access"`}},
			{{Query: `, t.spcname AS "tablespace"`}},
			{{Query: `, CASE WHEN pg_catalog.has_database_privilege(d.datname, 'CONNECT')` +
				` THEN pg_catalog.pg_size_pretty(pg_catalog.pg_database_size(d.datname))` +
				` ELSE 'no access' END AS "size"`}},
			{{Query: `, pg_catalog.shobj_description(d.oid, 'pg_database') AS "comment"`}},
			{{Query: `FROM pg_catalog.pg_database d`}},
			{{Query: `LEFT JOIN pg_catalog.pg_tablespace t ON t.oid = d.dattablespace`}},
			{{Query: `WHERE (@name = '' OR d.datname LIKE @name)`}},
			{{Query: `ORDER BY 1`}},
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
			{{Query: `SELECT spcname AS "name"`}},
			{{Query: `, pg_catalog.pg_get_userbyid(spcowner) AS "owner"`}},
			{{Query: `, pg_catalog.pg_tablespace_location(oid) AS "location"`}},
			{{Query: `, pg_catalog.array_to_string(spcoptions, ', ') AS "options"`}},
			{{Query: `, pg_catalog.pg_size_pretty(pg_catalog.pg_tablespace_size(oid)) AS "size"`}},
			{{Query: `, pg_catalog.array_to_string(spcacl, E'\n') AS "access"`}},
			{{Query: `, pg_catalog.shobj_description(oid, 'pg_tablespace') AS "comment"`}},
			{{Query: `FROM pg_catalog.pg_tablespace`}},
			{{Query: `WHERE (@name = '' OR spcname LIKE @name)`}},
			{{Query: `ORDER BY 1`}},
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
			{{Query: `SELECT amname AS "name"`}},
			{{Query: `, CASE amtype WHEN 'i' THEN 'index' WHEN 't' THEN 'table' ELSE amtype::text END AS "type"`}},
			{{Query: `, amhandler::text AS "handler"`}},
			{{Query: `, pg_catalog.obj_description(oid, 'pg_am') AS "comment"`}},
			{{Query: `FROM pg_catalog.pg_am`}},
			{{Query: `WHERE (@name = '' OR amname LIKE @name)`}},
			{{Query: `ORDER BY 1`}},
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
			{{Query: `SELECT l.lanname AS "name"`}},
			{{Query: `, pg_catalog.pg_get_userbyid(l.lanowner) AS "owner"`}},
			{{Query: `, l.lanpltrusted AS "trusted"`}},
			{{Query: `, NOT l.lanispl AS "internal"`}},
			{{Query: `, l.lanplcallfoid::pg_catalog.regprocedure::text AS "handler"`}},
			{{Query: `, l.lanvalidator::pg_catalog.regprocedure::text AS "validator"`}},
			{{Query: `, l.laninline::pg_catalog.regprocedure::text AS "inline"`}},
			{{Query: `, pg_catalog.array_to_string(l.lanacl, E'\n') AS "access"`}},
			{{Query: `, d.description AS "comment"`}},
			{{Query: `FROM pg_catalog.pg_language l`}},
			{{Query: `LEFT JOIN pg_catalog.pg_description d ON d.objoid = l.oid AND d.classoid = 'pg_language'::regclass`}},
			{{Query: `WHERE l.lanplcallfoid != 0`}},
			{{Query: `AND (@name = '' OR l.lanname LIKE @name)`}},
			{{Query: `ORDER BY 1`}},
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
			{{Query: `SELECT n.nspname AS "schema"`}},
			{{Query: `, c.conname AS "name"`}},
			{{Query: `, pg_catalog.pg_encoding_to_char(c.conforencoding) AS "source"`}},
			{{Query: `, pg_catalog.pg_encoding_to_char(c.contoencoding) AS "target"`}},
			{{Query: `, c.condefault AS "default"`}},
			{{Query: `, d.description AS "comment"`}},
			{{Query: `FROM pg_catalog.pg_conversion c`}},
			{{Query: `JOIN pg_catalog.pg_namespace n ON n.oid = c.connamespace`}},
			{{Query: `LEFT JOIN pg_catalog.pg_description d ON d.objoid = c.oid AND d.classoid = 'pg_conversion'::regclass`}},
			{{Query: `WHERE (@with_system OR (n.nspname <> 'pg_catalog' AND n.nspname <> 'information_schema'))`}},
			{{Query: `AND (@schema = '' OR n.nspname LIKE @schema)`}},
			{{Query: `AND (@name = '' OR c.conname LIKE @name)`}},
			{{Query: `ORDER BY 1, 2`}},
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
			{{Query: `SELECT pg_catalog.format_type(c.castsource, NULL) AS "source"`}},
			{{Query: `, pg_catalog.format_type(c.casttarget, NULL) AS "target"`}},
			{{Query: `, CASE WHEN c.castmethod = 'b' THEN '(binary coercible)'` +
				` WHEN c.castmethod = 'i' THEN '(with inout)'` +
				` ELSE p.proname::text END AS "function"`}},
			{{Query: `, CASE WHEN c.castcontext = 'e' THEN 'no'` +
				` WHEN c.castcontext = 'a' THEN 'in assignment'` +
				` ELSE 'yes' END AS "implicit"`}},
			{{Query: `, p.proleakproof AS "leakproof"`}},
			{{Query: `, d.description AS "comment"`}},
			{{Query: `FROM pg_catalog.pg_cast c`}},
			{{Query: `LEFT JOIN pg_catalog.pg_proc p ON c.castfunc = p.oid`}},
			{{Query: `LEFT JOIN pg_catalog.pg_description d ON d.objoid = c.oid AND d.classoid = 'pg_cast'::regclass`}},
			{{Query: `WHERE (@name = '' OR pg_catalog.format_type(c.castsource, NULL) LIKE @name` +
				` OR pg_catalog.format_type(c.casttarget, NULL) LIKE @name)`}},
			{{Query: `ORDER BY 1, 2`}},
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
			{{Query: `SELECT n.nspname AS "schema"`}},
			{{Query: `, c.collname AS "name"`}},
			{
				{Query: `, NULL AS "provider"`},
				{Min: v10, Query: `, CASE c.collprovider WHEN 'd' THEN 'default' WHEN 'c' THEN 'libc' WHEN 'i' THEN 'icu' WHEN 'b' THEN 'builtin' ELSE '' END AS "provider"`},
			},
			{{Query: `, c.collcollate AS "collate"`}},
			{{Query: `, c.collctype AS "ctype"`}},
			// the locale column was renamed twice. psql gates it the same
			// way at describe.c:5093, and an older server falls back to the
			// collate string rather than reporting nothing.
			{
				{Query: `, c.collcollate AS "locale"`},
				{Min: v15, Query: `, c.colliculocale AS "locale"`},
				{Min: v17, Query: `, c.colllocale AS "locale"`},
			},
			{
				{Query: `, true AS "deterministic"`},
				{Min: v12, Query: `, c.collisdeterministic AS "deterministic"`},
			},
			{{Query: `, pg_catalog.obj_description(c.oid, 'pg_collation') AS "comment"`}},
			{{Query: `FROM pg_catalog.pg_collation c`}},
			{{Query: `JOIN pg_catalog.pg_namespace n ON n.oid = c.collnamespace`}},
			{{Query: `WHERE (@with_system OR (n.nspname <> 'pg_catalog' AND n.nspname <> 'information_schema'))`}},
			{{Query: `AND (@schema = '' OR n.nspname LIKE @schema)`}},
			{{Query: `AND (@name = '' OR c.collname LIKE @name)`}},
			{{Query: `ORDER BY 1, 2`}},
		},
		Fields: []dbmeta.Field{
			{Name: "schema"},
			{Name: "name"},
			{Name: "provider", Desc: "libc, icu or builtin", Min: v10},
			{Name: "collate"},
			{Name: "ctype"},
			{Name: "locale", Desc: "the locale, from colllocale at 17, colliculocale at 15, and collcollate below that"},
			{Name: "deterministic", Desc: "whether equal strings are always identical. Always true below release 12, which had no other behaviour"},
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
			{{Query: `SELECT o.oid AS "oid"`}},
			{{Query: `, pg_catalog.pg_get_userbyid(o.lomowner) AS "owner"`}},
			{{Query: `, pg_catalog.array_to_string(o.lomacl, E'\n') AS "access"`}},
			{{Query: `, pg_catalog.obj_description(o.oid, 'pg_largeobject') AS "comment"`}},
			{{Query: `FROM pg_catalog.pg_largeobject_metadata o`}},
			{{Query: `ORDER BY 1`}},
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
			{{Query: `SELECT e.evtname AS "name"`}},
			{{Query: `, e.evtevent AS "event"`}},
			{{Query: `, pg_catalog.pg_get_userbyid(e.evtowner) AS "owner"`}},
			{{Query: `, CASE e.evtenabled WHEN 'O' THEN 'enabled' WHEN 'R' THEN 'replica'` +
				` WHEN 'A' THEN 'always' WHEN 'D' THEN 'disabled' ELSE '' END AS "enabled"`}},
			{{Query: `, e.evtfoid::pg_catalog.regproc::text AS "function"`}},
			{{Query: `, pg_catalog.array_to_string(e.evttags, ', ') AS "tags"`}},
			{{Query: `, pg_catalog.obj_description(e.oid, 'pg_event_trigger') AS "comment"`}},
			{{Query: `FROM pg_catalog.pg_event_trigger e`}},
			{{Query: `WHERE (@name = '' OR e.evtname LIKE @name)`}},
			{{Query: `ORDER BY 1`}},
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
			{{Query: `SELECT s.name AS "name"`}},
			{{Query: `, s.setting AS "value"`}},
			{{Query: `, s.vartype AS "type"`}},
			{{Query: `, s.context AS "context"`}},
			{
				{Query: `, NULL AS "access"`},
				{Min: v15, Query: `, pg_catalog.array_to_string(p.paracl, E'\n') AS "access"`},
			},
			{{Query: `FROM pg_catalog.pg_settings s`}},
			{
				{Query: ``},
				{Min: v15, Query: `LEFT JOIN pg_catalog.pg_parameter_acl p ON p.parname = s.name`},
			},
			{{Query: `WHERE (@name = '' OR s.name LIKE @name)`}},
			{{Query: `ORDER BY 1`}},
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
