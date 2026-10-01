package cockroachdb

import (
	"database/sql"

	"github.com/xo/dbmeta"
)

// Releases of CockroachDB that a fragment gates on, each with the key
// [Release]. A gate sits at the oldest release measured to have what it
// needs: 24.3.36, 26.2.7 and 26.3.2 were measured on 2026-09-29, so a
// function that 24.3 lacks and 26.2 has gates at 26.2.
var (
	v262 = dbmeta.V(26, 2)
	v263 = dbmeta.V(26, 3)
)

// systemSchemas is the filter the postgres model writes for the schemas a
// server keeps for itself. The statements here repeat it, so that with_system
// means the same as in the statements shared from that model.
const systemSchemas = `(@with_system OR (n.nspname !~ '^pg_' AND n.nspname <> 'information_schema'))`

// registerOwn registers the statements CockroachDB answers differently from
// PostgreSQL. Each one is the postgres model's statement with the function or
// the type that CockroachDB lacks taken out, measured on 24.3, 26.2 and 26.3.
func registerOwn() {
	// pg_size_pretty and pg_database_size arrive in 26.3.
	dbmeta.Databases.Register(dbmeta.CockroachDB, &dbmeta.Binding[dbmeta.Database]{
		Stmt: dbmeta.Stmt{
			{{Query: `SELECT d.datname AS "name"`}},
			{{Query: `, pg_catalog.pg_get_userbyid(d.datdba) AS "owner"`}},
			{{Query: `, pg_catalog.pg_encoding_to_char(d.encoding) AS "encoding"`}},
			{{Query: `, d.datcollate AS "collate"`}},
			{{Query: `, d.datctype AS "ctype"`}},
			{{Query: `, pg_catalog.array_to_string(d.datacl, E'\n') AS "access"`}},
			{{Query: `, t.spcname AS "tablespace"`}},
			{
				{Key: Release, Min: v263, Query: `, CASE WHEN pg_catalog.has_database_privilege(d.datname, 'CONNECT')` +
					` THEN pg_catalog.pg_size_pretty(pg_catalog.pg_database_size(d.datname))` +
					` ELSE 'no access' END AS "size"`},
				{Query: `, NULL AS "size"`},
			},
			{{Query: `, pg_catalog.shobj_description(d.oid, 'pg_database') AS "comment"`}},
			{{Query: `FROM pg_catalog.pg_database d`}},
			{{Query: `LEFT JOIN pg_catalog.pg_tablespace t ON t.oid = d.dattablespace`}},
			{{Query: `WHERE (@name = '' OR d.datname LIKE @name)`}},
			{{Query: `ORDER BY 1`}},
		},
		Fields: []dbmeta.Field{
			{Name: "name"}, {Name: "owner"}, {Name: "encoding"}, {Name: "collate"}, {Name: "ctype"},
			{Name: "access"}, {Name: "tablespace"},
			{Name: "size", Key: Release, Min: v263},
			{Name: "comment"},
		},
		Params: []dbmeta.Param{{Name: "name", Desc: "database name pattern, empty for every database", Default: ""}},
		Scan: func(rows *sql.Rows) (dbmeta.Database, error) {
			var v dbmeta.Database
			err := rows.Scan(&v.Name, &v.Owner, &v.Encoding, &v.Collate, &v.CType,
				&v.Access, &v.Tablespace, &v.Size, &v.Comment)
			return v, err
		},
	})

	// CockroachDB keeps no tablespace on a path, and has neither
	// pg_tablespace_location nor pg_tablespace_size on any release measured.
	dbmeta.Tablespaces.Register(dbmeta.CockroachDB, &dbmeta.Binding[dbmeta.Tablespace]{
		Stmt: dbmeta.Stmt{
			{{Query: `SELECT spcname AS "name"`}},
			{{Query: `, pg_catalog.pg_get_userbyid(spcowner) AS "owner"`}},
			{{Query: `, NULL AS "location"`}},
			{{Query: `, pg_catalog.array_to_string(spcoptions, ', ') AS "options"`}},
			{{Query: `, NULL AS "size"`}},
			{{Query: `, pg_catalog.array_to_string(spcacl, E'\n') AS "access"`}},
			{{Query: `, pg_catalog.shobj_description(oid, 'pg_tablespace') AS "comment"`}},
			{{Query: `FROM pg_catalog.pg_tablespace`}},
			{{Query: `WHERE (@name = '' OR spcname LIKE @name)`}},
			{{Query: `ORDER BY 1`}},
		},
		Fields: []dbmeta.Field{
			{Name: "name"}, {Name: "owner"},
			{Name: "location", Desc: "always absent: CockroachDB keeps no tablespace on a path"},
			{Name: "options"},
			{Name: "size", Desc: "always absent: CockroachDB has no pg_tablespace_size"},
			{Name: "access"}, {Name: "comment"},
		},
		Params: []dbmeta.Param{{Name: "name", Desc: "tablespace name pattern, empty for every tablespace", Default: ""}},
		Scan: func(rows *sql.Rows) (dbmeta.Tablespace, error) {
			var v dbmeta.Tablespace
			err := rows.Scan(&v.Name, &v.Owner, &v.Location, &v.Options, &v.Size, &v.Access, &v.Comment)
			return v, err
		},
	})

	// pg_get_triggerdef arrives by 26.2. 24.3 has triggers and no function
	// that writes one out.
	dbmeta.Triggers.Register(dbmeta.CockroachDB, &dbmeta.Binding[dbmeta.Trigger]{
		Stmt: dbmeta.Stmt{
			{{Query: `SELECT n.nspname AS "schema"`}},
			{{Query: `, c.relname AS "table"`}},
			{{Query: `, t.tgname AS "name"`}},
			{{Query: `, CASE t.tgenabled WHEN 'O' THEN 'enabled' WHEN 'D' THEN 'disabled'` +
				` WHEN 'R' THEN 'replica' WHEN 'A' THEN 'always' ELSE '' END AS "enabled"`}},
			{
				{Key: Release, Min: v262, Query: `, pg_catalog.pg_get_triggerdef(t.oid, true) AS "definition"`},
				{Query: `, NULL AS "definition"`},
			},
			{{Query: `, pg_catalog.obj_description(t.oid, 'pg_trigger') AS "comment"`}},
			{{Query: `FROM pg_catalog.pg_trigger t`}},
			{{Query: `JOIN pg_catalog.pg_class c ON c.oid = t.tgrelid`}},
			{{Query: `JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace`}},
			{{Query: `WHERE NOT t.tgisinternal`}},
			{{Query: `AND ` + systemSchemas}},
			{{Query: `AND (@schema = '' OR n.nspname LIKE @schema)`}},
			{{Query: `AND (@parent = '' OR c.relname LIKE @parent)`}},
			{{Query: `AND (@name = '' OR t.tgname LIKE @name)`}},
			{{Query: `ORDER BY 1, 2, 3`}},
		},
		Fields: []dbmeta.Field{
			{Name: "schema"}, {Name: "table"}, {Name: "name"}, {Name: "enabled"},
			{Name: "definition", Key: Release, Min: v262},
			{Name: "comment"},
		},
		Params: schemaParentName("trigger"),
		Scan: func(rows *sql.Rows) (dbmeta.Trigger, error) {
			var v dbmeta.Trigger
			err := rows.Scan(&v.Schema, &v.Table, &v.Name, &v.Enabled, &v.Definition, &v.Comment)
			return v, err
		},
	})

	// CockroachDB has no pg_index_column_has_property. Bit 1 of indoption is
	// the descending flag, as in PostgreSQL, and CockroachDB fills it.
	dbmeta.IndexColumns.Register(dbmeta.CockroachDB, &dbmeta.Binding[dbmeta.IndexColumn]{
		Stmt: dbmeta.Stmt{
			{{Query: `SELECT n.nspname AS "schema"`}},
			{{Query: `, t.relname AS "table"`}},
			{{Query: `, c.relname AS "index"`}},
			{{Query: `, a.attname AS "name"`}},
			{{Query: `, k.ordinality AS "ordinal"`}},
			{{Query: `, pg_catalog.pg_get_indexdef(i.indexrelid, k.ordinality::int, true) AS "expression"`}},
			{{Query: `, (i.indoption[k.ordinality - 1] & 1) = 1 AS "descending"`}},
			{{Query: `FROM pg_catalog.pg_index i`}},
			{{Query: `JOIN pg_catalog.pg_class c ON c.oid = i.indexrelid`}},
			{{Query: `JOIN pg_catalog.pg_class t ON t.oid = i.indrelid`}},
			{{Query: `JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace`}},
			{{Query: `CROSS JOIN LATERAL pg_catalog.unnest(i.indkey) WITH ORDINALITY AS k(attnum, ordinality)`}},
			{{Query: `LEFT JOIN pg_catalog.pg_attribute a ON a.attrelid = i.indrelid AND a.attnum = k.attnum`}},
			{{Query: `WHERE ` + systemSchemas}},
			{{Query: `AND (@schema = '' OR n.nspname LIKE @schema)`}},
			{{Query: `AND (@parent = '' OR t.relname LIKE @parent)`}},
			{{Query: `AND (@name = '' OR c.relname LIKE @name)`}},
			{{Query: `ORDER BY 1, 2, 3, 5`}},
		},
		Fields: dbmeta.Fields("schema", "table", "index", "name", "ordinal", "expression", "descending"),
		Params: schemaParentName("index"),
		Scan: func(rows *sql.Rows) (dbmeta.IndexColumn, error) {
			var v dbmeta.IndexColumn
			err := rows.Scan(&v.Schema, &v.Table, &v.Index, &v.Name, &v.Ordinal,
				&v.Expression, &v.Descending)
			return v, err
		},
	})

	// CockroachDB has no pg_describe_object. Its pg_extension and its
	// dependencies of kind e were empty on every release measured, so the
	// statement answers no rows, and the description has no source
	// even with a row.
	dbmeta.ExtensionObjects.Register(dbmeta.CockroachDB, &dbmeta.Binding[dbmeta.ExtensionObject]{
		Stmt: dbmeta.Stmt{
			{{Query: `SELECT e.extname AS "extension"`}},
			{{Query: `, NULL AS "description"`}},
			{{Query: `FROM pg_catalog.pg_depend d`}},
			{{Query: `JOIN pg_catalog.pg_extension e ON e.oid = d.refobjid`}},
			{{Query: `WHERE d.refclassid = 'pg_catalog.pg_extension'::pg_catalog.regclass`}},
			{{Query: `AND d.deptype = 'e'`}},
			{{Query: `AND (@name = '' OR e.extname LIKE @name)`}},
			{{Query: `ORDER BY 1`}},
		},
		Fields: []dbmeta.Field{
			{Name: "extension"},
			{Name: "description", Desc: "always absent: CockroachDB has no pg_describe_object"},
		},
		Params: []dbmeta.Param{{Name: "name", Desc: "extension name pattern, empty for every one", Default: ""}},
		Scan: func(rows *sql.Rows) (dbmeta.ExtensionObject, error) {
			var v dbmeta.ExtensionObject
			err := rows.Scan(&v.Extension, &v.Description)
			return v, err
		},
	})

	// CockroachDB has no regoperator type, so the operator is written out
	// from pg_operator, as regoperator writes it: the name and the two
	// argument types. Its pg_amop was empty on every release measured.
	dbmeta.OperatorFamilyOperators.Register(dbmeta.CockroachDB, &dbmeta.Binding[dbmeta.OperatorFamilyOperator]{
		Stmt: dbmeta.Stmt{
			{{Query: `SELECT am.amname AS "access_method"`}},
			{{Query: `, f.opfname AS "family"`}},
			{{Query: `, op.oprname || '(' || pg_catalog.format_type(op.oprleft, NULL) || ','` +
				` || pg_catalog.format_type(op.oprright, NULL) || ')' AS "operator"`}},
			{{Query: `, o.amopstrategy AS "strategy"`}},
			{{Query: `, CASE o.amoppurpose WHEN 'o' THEN 'ordering' WHEN 's' THEN 'search'` +
				` ELSE o.amoppurpose::text END AS "purpose"`}},
			{{Query: `FROM pg_catalog.pg_amop o`}},
			{{Query: `JOIN pg_catalog.pg_operator op ON op.oid = o.amopopr`}},
			{{Query: `JOIN pg_catalog.pg_opfamily f ON f.oid = o.amopfamily`}},
			{{Query: `JOIN pg_catalog.pg_am am ON am.oid = f.opfmethod`}},
			{{Query: `WHERE (@access_method = '' OR am.amname LIKE @access_method)`}},
			{{Query: `AND (@name = '' OR f.opfname LIKE @name)`}},
			{{Query: `ORDER BY 1, 2, 4`}},
		},
		Fields: dbmeta.Fields("access_method", "family", "operator", "strategy", "purpose"),
		Params: []dbmeta.Param{
			{Name: "access_method", Desc: "access method name pattern, empty for every one", Default: ""},
			{Name: "name", Desc: "operator family name pattern, empty for every operator family", Default: ""},
		},
		Scan: func(rows *sql.Rows) (dbmeta.OperatorFamilyOperator, error) {
			var v dbmeta.OperatorFamilyOperator
			err := rows.Scan(&v.AccessMethod, &v.Family, &v.Operator, &v.Strategy, &v.Purpose)
			return v, err
		},
	})

	// pg_get_statisticsobjdef_columns is absent, so the definition is built
	// from the columns, the way the postgres model builds it below release
	// 14. CockroachDB has no statistics on an expression, so the columns
	// are the whole definition.
	dbmeta.ExtendedStats.Register(dbmeta.CockroachDB, &dbmeta.Binding[dbmeta.ExtendedStat]{
		Stmt: dbmeta.Stmt{
			{{Query: `SELECT n.nspname AS "schema"`}},
			{{Query: `, s.stxname AS "name"`}},
			{{Query: `, pg_catalog.pg_get_userbyid(s.stxowner) AS "owner"`}},
			{{Query: `, c.relname AS "table"`}},
			{{Query: `, pg_catalog.array_to_string(s.stxkind, ', ') AS "kinds"`}},
			{{Query: `, pg_catalog.obj_description(s.oid, 'pg_statistic_ext') AS "comment"`}},
			{{Query: `, pg_catalog.format('%s FROM %s', (SELECT pg_catalog.string_agg(pg_catalog.quote_ident(a.attname), ', ')` +
				` FROM pg_catalog.unnest(s.stxkeys) k(attnum)` +
				` JOIN pg_catalog.pg_attribute a ON a.attrelid = s.stxrelid AND a.attnum = k.attnum AND NOT a.attisdropped),` +
				` s.stxrelid::pg_catalog.regclass) AS "definition"`}},
			{{Query: `, 'd' = ANY(s.stxkind) AS "ndistinct"`}},
			{{Query: `, 'f' = ANY(s.stxkind) AS "dependencies"`}},
			{{Query: `, 'm' = ANY(s.stxkind) AS "mcv"`}},
			{{Query: `FROM pg_catalog.pg_statistic_ext s`}},
			{{Query: `JOIN pg_catalog.pg_class c ON c.oid = s.stxrelid`}},
			{{Query: `JOIN pg_catalog.pg_namespace n ON n.oid = s.stxnamespace`}},
			{{Query: `WHERE (@schema = '' OR n.nspname LIKE @schema)`}},
			{{Query: `AND (@name = '' OR s.stxname LIKE @name)`}},
			{{Query: `ORDER BY 1, 2`}},
		},
		Fields: []dbmeta.Field{
			{Name: "schema"}, {Name: "name"}, {Name: "owner"}, {Name: "table"}, {Name: "kinds"},
			{Name: "comment"},
			{Name: "definition", Desc: "the columns the object covers, and their table"},
			{Name: "ndistinct"}, {Name: "dependencies"}, {Name: "mcv"},
		},
		Params: []dbmeta.Param{
			{Name: "schema", Desc: "schema name pattern, empty for every schema", Default: ""},
			{Name: "name", Desc: "statistics object name pattern, empty for every statistics object", Default: ""},
			{Name: "with_system", Desc: "include the objects CockroachDB keeps for itself", Default: false},
		},
		Scan: func(rows *sql.Rows) (dbmeta.ExtendedStat, error) {
			var v dbmeta.ExtendedStat
			err := rows.Scan(&v.Schema, &v.Name, &v.Owner, &v.Table, &v.Kinds, &v.Comment,
				&v.Definition, &v.Ndistinct, &v.Dependencies, &v.MCV)
			return v, err
		},
	})
}

// schemaParentName is the parameter set of an object that belongs to a
// table, as the postgres model declares it.
func schemaParentName(kind string) []dbmeta.Param {
	return []dbmeta.Param{
		{Name: "schema", Desc: "schema name pattern, empty for every schema", Default: ""},
		{Name: "parent", Desc: "table name pattern, empty for every table", Default: ""},
		{Name: "name", Desc: kind + " name pattern, empty for every " + kind, Default: ""},
		{Name: "with_system", Desc: "include the objects CockroachDB keeps for itself", Default: false},
	}
}
