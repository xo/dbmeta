package postgres

import (
	"database/sql"

	"github.com/xo/dbmeta"
)

// Replication, text search, operator families and extensions.

func init() {
	registerPublications()
	registerPublicationTables()
	registerSubscriptions()
	registerTextSearch()
	registerOperatorFamilies()
	registerExtensions()
	registerComments()
}

// registerPublications backs \dRp, from listPublications.
//
// pg_publication arrived in release 10, which is above the 9.6 floor, so a 9.6
// server has no such catalog. The whole query is gated rather than padded: a
// caller on 9.6 gets ErrVersionTooOld, which is the honest answer, because the
// object does not exist rather than being empty.
//
// pubtruncate arrived in release 11 and pubviaroot in release 13.
func registerPublications() {
	dbmeta.Publications.Register(dbmeta.PostgreSQL, &dbmeta.Binding[dbmeta.Publication]{
		Stmt: dbmeta.Stmt{
			{{Min: v10, SQL: `SELECT p.pubname AS "name"`}},
			{{Min: v10, SQL: `, pg_catalog.pg_get_userbyid(p.pubowner) AS "owner"`}},
			{{Min: v10, SQL: `, p.puballtables AS "all_tables"`}},
			{{Min: v10, SQL: `, p.pubinsert AS "insert"`}},
			{{Min: v10, SQL: `, p.pubupdate AS "update"`}},
			{{Min: v10, SQL: `, p.pubdelete AS "delete"`}},
			{
				{Min: v10, SQL: `, false AS "truncate"`},
				{Min: v11, SQL: `, p.pubtruncate AS "truncate"`},
			},
			{
				{Min: v10, SQL: `, false AS "via_root"`},
				{Min: v13, SQL: `, p.pubviaroot AS "via_root"`},
			},
			{{Min: v10, SQL: `, pg_catalog.obj_description(p.oid, 'pg_publication') AS "comment"`}},
			{{Min: v10, SQL: `FROM pg_catalog.pg_publication p`}},
			{{Min: v10, SQL: `WHERE (@name = '' OR p.pubname LIKE @name)`}},
			{{Min: v10, SQL: `ORDER BY 1`}},
		},
		Fields: []dbmeta.Field{
			{Name: "name", Min: v10}, {Name: "owner", Min: v10}, {Name: "all_tables", Min: v10},
			{Name: "insert", Min: v10}, {Name: "update", Min: v10}, {Name: "delete", Min: v10},
			{Name: "truncate", Min: v11}, {Name: "via_root", Min: v13}, {Name: "comment", Min: v10},
		},
		Params: []dbmeta.Param{{Name: "name", Desc: "publication name pattern, empty for every one", Default: ""}},
		Scan: func(rows *sql.Rows) (dbmeta.Publication, error) {
			var v dbmeta.Publication
			err := rows.Scan(&v.Name, &v.Owner, &v.AllTables, &v.Insert, &v.Update,
				&v.Delete, &v.Truncate, &v.ViaRoot, &v.Comment)
			return v, err
		},
	})
}

// registerPublicationTables backs \dRp+, from describePublications.
//
// A column list and a row filter both arrived in release 15.
func registerPublicationTables() {
	dbmeta.PublicationTables.Register(dbmeta.PostgreSQL, &dbmeta.Binding[dbmeta.PublicationTable]{
		Stmt: dbmeta.Stmt{
			{{Min: v10, SQL: `SELECT p.pubname AS "publication"`}},
			{{Min: v10, SQL: `, n.nspname AS "schema"`}},
			{{Min: v10, SQL: `, c.relname AS "name"`}},
			{
				{Min: v10, SQL: `, '' AS "columns"`},
				{Min: v15, SQL: `, COALESCE(pg_catalog.array_to_string(ARRAY(` +
					`SELECT a.attname FROM pg_catalog.pg_attribute a` +
					` WHERE a.attrelid = c.oid AND a.attnum = ANY(pr.prattrs)` +
					` ORDER BY a.attnum), ', '), '') AS "columns"`},
			},
			{
				{Min: v10, SQL: `, '' AS "where"`},
				{Min: v15, SQL: `, pg_catalog.pg_get_expr(pr.prqual, c.oid) AS "where"`},
			},
			{{Min: v10, SQL: `FROM pg_catalog.pg_publication p`}},
			{{Min: v10, SQL: `JOIN pg_catalog.pg_publication_rel pr ON pr.prpubid = p.oid`}},
			{{Min: v10, SQL: `JOIN pg_catalog.pg_class c ON c.oid = pr.prrelid`}},
			{{Min: v10, SQL: `JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace`}},
			{{Min: v10, SQL: `WHERE (@name = '' OR p.pubname LIKE @name)`}},
			{{Min: v10, SQL: `ORDER BY 1, 2, 3`}},
		},
		Fields: []dbmeta.Field{
			{Name: "publication", Min: v10}, {Name: "schema", Min: v10}, {Name: "name", Min: v10},
			{Name: "columns", Desc: "published columns, empty for every column", Min: v15},
			{Name: "where", Desc: "row filter, empty for every row", Min: v15},
		},
		Params: []dbmeta.Param{{Name: "name", Desc: "publication name pattern, empty for every one", Default: ""}},
		Scan: func(rows *sql.Rows) (dbmeta.PublicationTable, error) {
			var v dbmeta.PublicationTable
			err := rows.Scan(&v.Publication, &v.Schema, &v.Name, &v.Columns, &v.Where)
			return v, err
		},
	})
}

// registerSubscriptions backs \dRs, from describeSubscriptions.
func registerSubscriptions() {
	dbmeta.Subscriptions.Register(dbmeta.PostgreSQL, &dbmeta.Binding[dbmeta.Subscription]{
		Stmt: dbmeta.Stmt{
			{{Min: v10, SQL: `SELECT s.subname AS "name"`}},
			{{Min: v10, SQL: `, pg_catalog.pg_get_userbyid(s.subowner) AS "owner"`}},
			{{Min: v10, SQL: `, s.subenabled AS "enabled"`}},
			{{Min: v10, SQL: `, pg_catalog.array_to_string(s.subpublications, ', ') AS "publications"`}},
			{{Min: v10, SQL: `, s.subsynccommit AS "synchronous"`}},
			{{Min: v10, SQL: `, s.subslotname AS "slot"`}},
			{{Min: v10, SQL: `, pg_catalog.shobj_description(s.oid, 'pg_subscription') AS "comment"`}},
			{{Min: v10, SQL: `FROM pg_catalog.pg_subscription s`}},
			{{Min: v10, SQL: `WHERE (@name = '' OR s.subname LIKE @name)`}},
			{{Min: v10, SQL: `ORDER BY 1`}},
		},
		Fields: []dbmeta.Field{
			{Name: "name", Min: v10}, {Name: "owner", Min: v10}, {Name: "enabled", Min: v10},
			{Name: "publications", Min: v10}, {Name: "synchronous", Min: v10},
			{Name: "slot", Min: v10}, {Name: "comment", Min: v10},
		},
		Params: []dbmeta.Param{{Name: "name", Desc: "subscription name pattern, empty for every one", Default: ""}},
		Scan: func(rows *sql.Rows) (dbmeta.Subscription, error) {
			var v dbmeta.Subscription
			err := rows.Scan(&v.Name, &v.Owner, &v.Enabled, &v.Publications,
				&v.Synchronous, &v.Slot, &v.Comment)
			return v, err
		},
	})
}

// registerTextSearch backs \dFp, \dFd, \dFt and \dF, from the listTS family.
func registerTextSearch() {
	dbmeta.TextSearchParsers.Register(dbmeta.PostgreSQL, &dbmeta.Binding[dbmeta.TextSearchParser]{
		Stmt: dbmeta.Stmt{
			{{SQL: `SELECT n.nspname AS "schema"`}},
			{{SQL: `, p.prsname AS "name"`}},
			{{SQL: `, pg_catalog.obj_description(p.oid, 'pg_ts_parser') AS "comment"`}},
			{{SQL: `FROM pg_catalog.pg_ts_parser p`}},
			{{SQL: `LEFT JOIN pg_catalog.pg_namespace n ON n.oid = p.prsnamespace`}},
			{{SQL: `WHERE (@schema = '' OR n.nspname LIKE @schema)`}},
			{{SQL: `AND (@name = '' OR p.prsname LIKE @name)`}},
			{{SQL: `ORDER BY 1, 2`}},
		},
		Fields: fields("schema", "name", "comment"),
		Params: schemaNameSystem("parser"),
		Scan: func(rows *sql.Rows) (dbmeta.TextSearchParser, error) {
			var v dbmeta.TextSearchParser
			err := rows.Scan(&v.Schema, &v.Name, &v.Comment)
			return v, err
		},
	})

	dbmeta.TextSearchDictionaries.Register(dbmeta.PostgreSQL, &dbmeta.Binding[dbmeta.TextSearchDictionary]{
		Stmt: dbmeta.Stmt{
			{{SQL: `SELECT n.nspname AS "schema"`}},
			{{SQL: `, d.dictname AS "name"`}},
			{{SQL: `, t.tmplname AS "template"`}},
			{{SQL: `, d.dictinitoption AS "options"`}},
			{{SQL: `, pg_catalog.obj_description(d.oid, 'pg_ts_dict') AS "comment"`}},
			{{SQL: `FROM pg_catalog.pg_ts_dict d`}},
			{{SQL: `LEFT JOIN pg_catalog.pg_namespace n ON n.oid = d.dictnamespace`}},
			{{SQL: `LEFT JOIN pg_catalog.pg_ts_template t ON t.oid = d.dicttemplate`}},
			{{SQL: `WHERE (@schema = '' OR n.nspname LIKE @schema)`}},
			{{SQL: `AND (@name = '' OR d.dictname LIKE @name)`}},
			{{SQL: `ORDER BY 1, 2`}},
		},
		Fields: fields("schema", "name", "template", "options", "comment"),
		Params: schemaNameSystem("dictionary"),
		Scan: func(rows *sql.Rows) (dbmeta.TextSearchDictionary, error) {
			var v dbmeta.TextSearchDictionary
			err := rows.Scan(&v.Schema, &v.Name, &v.Template, &v.Options, &v.Comment)
			return v, err
		},
	})

	dbmeta.TextSearchTemplates.Register(dbmeta.PostgreSQL, &dbmeta.Binding[dbmeta.TextSearchTemplate]{
		Stmt: dbmeta.Stmt{
			{{SQL: `SELECT n.nspname AS "schema"`}},
			{{SQL: `, t.tmplname AS "name"`}},
			{{SQL: `, t.tmplinit::pg_catalog.regproc::text AS "init"`}},
			{{SQL: `, t.tmpllexize::pg_catalog.regproc::text AS "lexize"`}},
			{{SQL: `, pg_catalog.obj_description(t.oid, 'pg_ts_template') AS "comment"`}},
			{{SQL: `FROM pg_catalog.pg_ts_template t`}},
			{{SQL: `LEFT JOIN pg_catalog.pg_namespace n ON n.oid = t.tmplnamespace`}},
			{{SQL: `WHERE (@schema = '' OR n.nspname LIKE @schema)`}},
			{{SQL: `AND (@name = '' OR t.tmplname LIKE @name)`}},
			{{SQL: `ORDER BY 1, 2`}},
		},
		Fields: fields("schema", "name", "init", "lexize", "comment"),
		Params: schemaNameSystem("template"),
		Scan: func(rows *sql.Rows) (dbmeta.TextSearchTemplate, error) {
			var v dbmeta.TextSearchTemplate
			err := rows.Scan(&v.Schema, &v.Name, &v.Init, &v.Lexize, &v.Comment)
			return v, err
		},
	})

	dbmeta.TextSearchConfigs.Register(dbmeta.PostgreSQL, &dbmeta.Binding[dbmeta.TextSearchConfig]{
		Stmt: dbmeta.Stmt{
			{{SQL: `SELECT n.nspname AS "schema"`}},
			{{SQL: `, c.cfgname AS "name"`}},
			{{SQL: `, p.prsname AS "parser"`}},
			{{SQL: `, pg_catalog.obj_description(c.oid, 'pg_ts_config') AS "comment"`}},
			{{SQL: `FROM pg_catalog.pg_ts_config c`}},
			{{SQL: `LEFT JOIN pg_catalog.pg_namespace n ON n.oid = c.cfgnamespace`}},
			{{SQL: `LEFT JOIN pg_catalog.pg_ts_parser p ON p.oid = c.cfgparser`}},
			{{SQL: `WHERE (@schema = '' OR n.nspname LIKE @schema)`}},
			{{SQL: `AND (@name = '' OR c.cfgname LIKE @name)`}},
			{{SQL: `ORDER BY 1, 2`}},
		},
		Fields: fields("schema", "name", "parser", "comment"),
		Params: schemaNameSystem("configuration"),
		Scan: func(rows *sql.Rows) (dbmeta.TextSearchConfig, error) {
			var v dbmeta.TextSearchConfig
			err := rows.Scan(&v.Schema, &v.Name, &v.Parser, &v.Comment)
			return v, err
		},
	})
}

// registerOperatorFamilies backs \dAc, \dAf, \dAo and \dAp.
func registerOperatorFamilies() {
	dbmeta.OperatorClasses.Register(dbmeta.PostgreSQL, &dbmeta.Binding[dbmeta.OperatorClass]{
		Stmt: dbmeta.Stmt{
			{{SQL: `SELECT am.amname AS "access_method"`}},
			{{SQL: `, n.nspname AS "schema"`}},
			{{SQL: `, c.opcname AS "name"`}},
			{{SQL: `, pg_catalog.format_type(c.opcintype, NULL) AS "input_type"`}},
			{{SQL: `, c.opcdefault AS "default"`}},
			{{SQL: `, f.opfname AS "family"`}},
			{{SQL: `, pg_catalog.pg_get_userbyid(c.opcowner) AS "owner"`}},
			{{SQL: `FROM pg_catalog.pg_opclass c`}},
			{{SQL: `JOIN pg_catalog.pg_am am ON am.oid = c.opcmethod`}},
			{{SQL: `JOIN pg_catalog.pg_namespace n ON n.oid = c.opcnamespace`}},
			{{SQL: `JOIN pg_catalog.pg_opfamily f ON f.oid = c.opcfamily`}},
			{{SQL: `WHERE (@access_method = '' OR am.amname LIKE @access_method)`}},
			{{SQL: `AND (@name = '' OR c.opcname LIKE @name)`}},
			{{SQL: `ORDER BY 1, 2, 3`}},
		},
		Fields: fields("access_method", "schema", "name", "input_type", "default", "family", "owner"),
		Params: accessMethodName("operator class"),
		Scan: func(rows *sql.Rows) (dbmeta.OperatorClass, error) {
			var v dbmeta.OperatorClass
			err := rows.Scan(&v.AccessMethod, &v.Schema, &v.Name, &v.InputType,
				&v.Default, &v.Family, &v.Owner)
			return v, err
		},
	})

	dbmeta.OperatorFamilies.Register(dbmeta.PostgreSQL, &dbmeta.Binding[dbmeta.OperatorFamily]{
		Stmt: dbmeta.Stmt{
			{{SQL: `SELECT am.amname AS "access_method"`}},
			{{SQL: `, n.nspname AS "schema"`}},
			{{SQL: `, f.opfname AS "name"`}},
			{{SQL: `, pg_catalog.pg_get_userbyid(f.opfowner) AS "owner"`}},
			{{SQL: `FROM pg_catalog.pg_opfamily f`}},
			{{SQL: `JOIN pg_catalog.pg_am am ON am.oid = f.opfmethod`}},
			{{SQL: `JOIN pg_catalog.pg_namespace n ON n.oid = f.opfnamespace`}},
			{{SQL: `WHERE (@access_method = '' OR am.amname LIKE @access_method)`}},
			{{SQL: `AND (@name = '' OR f.opfname LIKE @name)`}},
			{{SQL: `ORDER BY 1, 2, 3`}},
		},
		Fields: fields("access_method", "schema", "name", "owner"),
		Params: accessMethodName("operator family"),
		Scan: func(rows *sql.Rows) (dbmeta.OperatorFamily, error) {
			var v dbmeta.OperatorFamily
			err := rows.Scan(&v.AccessMethod, &v.Schema, &v.Name, &v.Owner)
			return v, err
		},
	})

	dbmeta.OperatorFamilyOperators.Register(dbmeta.PostgreSQL, &dbmeta.Binding[dbmeta.OperatorFamilyOperator]{
		Stmt: dbmeta.Stmt{
			{{SQL: `SELECT am.amname AS "access_method"`}},
			{{SQL: `, f.opfname AS "family"`}},
			{{SQL: `, o.amopopr::pg_catalog.regoperator::text AS "operator"`}},
			{{SQL: `, o.amopstrategy AS "strategy"`}},
			{{SQL: `, CASE o.amoppurpose WHEN 'o' THEN 'ordering' WHEN 's' THEN 'search'` +
				` ELSE o.amoppurpose::text END AS "purpose"`}},
			{{SQL: `FROM pg_catalog.pg_amop o`}},
			{{SQL: `JOIN pg_catalog.pg_opfamily f ON f.oid = o.amopfamily`}},
			{{SQL: `JOIN pg_catalog.pg_am am ON am.oid = f.opfmethod`}},
			{{SQL: `WHERE (@access_method = '' OR am.amname LIKE @access_method)`}},
			{{SQL: `AND (@name = '' OR f.opfname LIKE @name)`}},
			{{SQL: `ORDER BY 1, 2, 4`}},
		},
		Fields: fields("access_method", "family", "operator", "strategy", "purpose"),
		Params: accessMethodName("operator family"),
		Scan: func(rows *sql.Rows) (dbmeta.OperatorFamilyOperator, error) {
			var v dbmeta.OperatorFamilyOperator
			err := rows.Scan(&v.AccessMethod, &v.Family, &v.Operator, &v.Strategy, &v.Purpose)
			return v, err
		},
	})

	dbmeta.OperatorFamilyFunctions.Register(dbmeta.PostgreSQL, &dbmeta.Binding[dbmeta.OperatorFamilyFunction]{
		Stmt: dbmeta.Stmt{
			{{SQL: `SELECT am.amname AS "access_method"`}},
			{{SQL: `, f.opfname AS "family"`}},
			{{SQL: `, pg_catalog.format_type(p.amproclefttype, NULL) AS "left_type"`}},
			{{SQL: `, pg_catalog.format_type(p.amprocrighttype, NULL) AS "right_type"`}},
			{{SQL: `, p.amprocnum AS "number"`}},
			{{SQL: `, p.amproc::pg_catalog.regprocedure::text AS "function"`}},
			{{SQL: `FROM pg_catalog.pg_amproc p`}},
			{{SQL: `JOIN pg_catalog.pg_opfamily f ON f.oid = p.amprocfamily`}},
			{{SQL: `JOIN pg_catalog.pg_am am ON am.oid = f.opfmethod`}},
			{{SQL: `WHERE (@access_method = '' OR am.amname LIKE @access_method)`}},
			{{SQL: `AND (@name = '' OR f.opfname LIKE @name)`}},
			{{SQL: `ORDER BY 1, 2, 5`}},
		},
		Fields: fields("access_method", "family", "left_type", "right_type", "number", "function"),
		Params: accessMethodName("operator family"),
		Scan: func(rows *sql.Rows) (dbmeta.OperatorFamilyFunction, error) {
			var v dbmeta.OperatorFamilyFunction
			err := rows.Scan(&v.AccessMethod, &v.Family, &v.LeftType, &v.RightType,
				&v.Number, &v.Function)
			return v, err
		},
	})
}

// registerExtensions backs \dx, \dx+ and \dX.
func registerExtensions() {
	dbmeta.Extensions.Register(dbmeta.PostgreSQL, &dbmeta.Binding[dbmeta.Extension]{
		Stmt: dbmeta.Stmt{
			{{SQL: `SELECT e.extname AS "name"`}},
			{{SQL: `, e.extversion AS "version"`}},
			{{SQL: `, n.nspname AS "schema"`}},
			{{SQL: `, pg_catalog.obj_description(e.oid, 'pg_extension') AS "comment"`}},
			{{SQL: `FROM pg_catalog.pg_extension e`}},
			{{SQL: `LEFT JOIN pg_catalog.pg_namespace n ON n.oid = e.extnamespace`}},
			{{SQL: `WHERE (@name = '' OR e.extname LIKE @name)`}},
			{{SQL: `ORDER BY 1`}},
		},
		Fields: fields("name", "version", "schema", "comment"),
		Params: []dbmeta.Param{{Name: "name", Desc: "extension name pattern, empty for every one", Default: ""}},
		Scan: func(rows *sql.Rows) (dbmeta.Extension, error) {
			var v dbmeta.Extension
			err := rows.Scan(&v.Name, &v.Version, &v.Schema, &v.Comment)
			return v, err
		},
	})

	dbmeta.ExtensionObjects.Register(dbmeta.PostgreSQL, &dbmeta.Binding[dbmeta.ExtensionObject]{
		Stmt: dbmeta.Stmt{
			{{SQL: `SELECT e.extname AS "extension"`}},
			{{SQL: `, pg_catalog.pg_describe_object(d.classid, d.objid, 0) AS "description"`}},
			{{SQL: `FROM pg_catalog.pg_depend d`}},
			{{SQL: `JOIN pg_catalog.pg_extension e ON e.oid = d.refobjid`}},
			{{SQL: `WHERE d.refclassid = 'pg_catalog.pg_extension'::pg_catalog.regclass`}},
			{{SQL: `AND d.deptype = 'e'`}},
			{{SQL: `AND (@name = '' OR e.extname LIKE @name)`}},
			{{SQL: `ORDER BY 1, 2`}},
		},
		Fields: fields("extension", "description"),
		Params: []dbmeta.Param{{Name: "name", Desc: "extension name pattern, empty for every one", Default: ""}},
		Scan: func(rows *sql.Rows) (dbmeta.ExtensionObject, error) {
			var v dbmeta.ExtensionObject
			err := rows.Scan(&v.Extension, &v.Description)
			return v, err
		},
	})

	// pg_statistic_ext arrived in release 10.
	dbmeta.ExtendedStats.Register(dbmeta.PostgreSQL, &dbmeta.Binding[dbmeta.ExtendedStat]{
		Stmt: dbmeta.Stmt{
			{{Min: v10, SQL: `SELECT n.nspname AS "schema"`}},
			{{Min: v10, SQL: `, s.stxname AS "name"`}},
			{{Min: v10, SQL: `, pg_catalog.pg_get_userbyid(s.stxowner) AS "owner"`}},
			{{Min: v10, SQL: `, c.relname AS "table"`}},
			{{Min: v10, SQL: `, pg_catalog.array_to_string(s.stxkind, ', ') AS "kinds"`}},
			{{Min: v10, SQL: `, pg_catalog.obj_description(s.oid, 'pg_statistic_ext') AS "comment"`}},
			{{Min: v10, SQL: `FROM pg_catalog.pg_statistic_ext s`}},
			{{Min: v10, SQL: `JOIN pg_catalog.pg_class c ON c.oid = s.stxrelid`}},
			{{Min: v10, SQL: `JOIN pg_catalog.pg_namespace n ON n.oid = s.stxnamespace`}},
			{{Min: v10, SQL: `WHERE (@schema = '' OR n.nspname LIKE @schema)`}},
			{{Min: v10, SQL: `AND (@name = '' OR s.stxname LIKE @name)`}},
			{{Min: v10, SQL: `ORDER BY 1, 2`}},
		},
		Fields: []dbmeta.Field{
			{Name: "schema", Min: v10}, {Name: "name", Min: v10}, {Name: "owner", Min: v10},
			{Name: "table", Min: v10}, {Name: "kinds", Min: v10}, {Name: "comment", Min: v10},
		},
		Params: schemaNameSystem("statistics object"),
		Scan: func(rows *sql.Rows) (dbmeta.ExtendedStat, error) {
			var v dbmeta.ExtendedStat
			err := rows.Scan(&v.Schema, &v.Name, &v.Owner, &v.Table, &v.Kinds, &v.Comment)
			return v, err
		},
	})
}

// registerComments backs \dd, from objectDescription.
//
// psql unions many catalogs here. This covers the relations, which is what a
// caller asks for in practice, and the other object kinds carry their own
// comment column already.
func registerComments() {
	dbmeta.Comments.Register(dbmeta.PostgreSQL, &dbmeta.Binding[dbmeta.Comment]{
		Stmt: dbmeta.Stmt{
			{{SQL: `SELECT n.nspname AS "schema"`}},
			{{SQL: `, c.relname AS "name"`}},
			{{SQL: `, CASE c.relkind WHEN 'r' THEN 'table' WHEN 'p' THEN 'table'` +
				` WHEN 'v' THEN 'view' WHEN 'm' THEN 'materialized view'` +
				` WHEN 'S' THEN 'sequence' WHEN 'i' THEN 'index' WHEN 'f' THEN 'foreign table'` +
				` ELSE c.relkind::text END AS "type"`}},
			{{SQL: `, d.description AS "comment"`}},
			{{SQL: `FROM pg_catalog.pg_description d`}},
			{{SQL: `JOIN pg_catalog.pg_class c ON c.oid = d.objoid AND d.classoid = 'pg_class'::regclass`}},
			{{SQL: `JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace`}},
			{{SQL: `WHERE d.objsubid = 0`}},
			{{SQL: `AND (@with_system OR (n.nspname !~ '^pg_' AND n.nspname <> 'information_schema'))`}},
			{{SQL: `AND (@schema = '' OR n.nspname LIKE @schema)`}},
			{{SQL: `AND (@name = '' OR c.relname LIKE @name)`}},
			{{SQL: `ORDER BY 1, 2`}},
		},
		Fields: fields("schema", "name", "type", "comment"),
		Params: schemaNameSystem("object"),
		Scan: func(rows *sql.Rows) (dbmeta.Comment, error) {
			var v dbmeta.Comment
			err := rows.Scan(&v.Schema, &v.Name, &v.Type, &v.Comment)
			return v, err
		},
	})
}
