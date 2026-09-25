package postgres

import (
	"database/sql"

	"github.com/xo/dbmeta"
)

// The kinds psql has no command for. D47 allows them: psql sets the object
// model and does not set the column set, and a consumer needs the parts where
// psql prints prose.

// notSystemSchema excludes the schemas PostgreSQL keeps for itself unless the
// caller asks for them.
const notSystemSchema = `(@with_system OR (n.nspname !~ '^pg_' AND n.nspname <> 'information_schema'))`

func registerExtra() {
	registerConstraintColumns()
	registerRoutineParameters()
	registerEnumValues()
	registerViews()
	registerColumnStats()
	registerCurrentSchema()
}

// registerConstraintColumns is Constraints in parts.
//
// conkey holds the columns in constraint order and confkey the columns they
// point at, in the same order. Unnesting one with ordinality and indexing the
// other by it keeps a composite key together, which is the whole reason this
// kind exists.
func registerConstraintColumns() {
	dbmeta.ConstraintColumns.Register(dbmeta.PostgreSQL, &dbmeta.Binding[dbmeta.ConstraintColumn]{
		Stmt: dbmeta.Stmt{
			{{Query: `SELECT current_database() AS "catalog"`}},
			{{Query: `, n.nspname AS "schema"`}},
			{{Query: `, t.relname AS "table"`}},
			{{Query: `, r.conname AS "constraint"`}},
			{{Query: `, a.attname AS "name"`}},
			{{Query: `, k.ordinality AS "ordinal"`}},
			{{Query: `, CASE WHEN r.confrelid <> 0 THEN current_database() ELSE NULL END AS "foreign_catalog"`}},
			{{Query: `, fn.nspname AS "foreign_schema"`}},
			{{Query: `, ft.relname AS "foreign_table"`}},
			{{Query: `, fa.attname AS "foreign_name"`}},
			{{Query: `FROM pg_catalog.pg_constraint r`}},
			{{Query: `JOIN pg_catalog.pg_class t ON t.oid = r.conrelid`}},
			{{Query: `JOIN pg_catalog.pg_namespace n ON n.oid = t.relnamespace`}},
			{{Query: `CROSS JOIN LATERAL pg_catalog.unnest(r.conkey) WITH ORDINALITY AS k(attnum, ordinality)`}},
			{{Query: `JOIN pg_catalog.pg_attribute a ON a.attrelid = r.conrelid AND a.attnum = k.attnum`}},
			{{Query: `LEFT JOIN pg_catalog.pg_class ft ON ft.oid = r.confrelid`}},
			{{Query: `LEFT JOIN pg_catalog.pg_namespace fn ON fn.oid = ft.relnamespace`}},
			{{Query: `LEFT JOIN pg_catalog.pg_attribute fa ON fa.attrelid = r.confrelid` +
				` AND fa.attnum = r.confkey[k.ordinality]`}},
			{{Query: `WHERE r.conrelid <> 0`}},
			// Left out for the reason Constraints leaves it out: release 18
			// records a NOT NULL here and no earlier release does, so
			// reporting it would make the same schema answer differently on
			// two servers. See D49.
			{{Query: `AND r.contype <> 'n'`}},
			{{Query: `AND ` + notSystemSchema}},
			{{Query: `AND (@schema = '' OR n.nspname LIKE @schema)`}},
			{{Query: `AND (@parent = '' OR t.relname LIKE @parent)`}},
			{{Query: `AND (@name = '' OR r.conname LIKE @name)`}},
			{{Query: `ORDER BY 2, 3, 4, 6`}},
		},
		Fields: []dbmeta.Field{
			{Name: "catalog"}, {Name: "schema"}, {Name: "table"},
			{Name: "constraint", Desc: "the constraint this column belongs to"},
			{Name: "name", Desc: "the column the constraint is on"},
			{Name: "ordinal", Desc: "one based position within the constraint"},
			{Name: "foreign_catalog", Desc: "set for a foreign key, absent otherwise"},
			{Name: "foreign_schema", Desc: "set for a foreign key, absent otherwise"},
			{Name: "foreign_table", Desc: "set for a foreign key, absent otherwise"},
			{
				Name: "foreign_name",
				Desc: "the column this one points at, in the same position of the key",
			},
		},
		Params: schemaParentName("constraint"),
		Scan: func(rows *sql.Rows) (dbmeta.ConstraintColumn, error) {
			var v dbmeta.ConstraintColumn
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Table, &v.Constraint, &v.Name,
				&v.Ordinal, &v.ForeignCatalog, &v.ForeignSchema, &v.ForeignTable, &v.ForeignName)
			return v, err
		},
	})
}

// registerRoutineParameters is the signature in parts.
//
// proargtypes holds the input parameters, and proallargtypes holds every
// parameter including the output ones and is NULL when there are none, which
// is why the two are coalesced. proargnames and proargmodes line up with
// whichever of the two applies.
func registerRoutineParameters() {
	dbmeta.RoutineParameters.Register(dbmeta.PostgreSQL, &dbmeta.Binding[dbmeta.RoutineParameter]{
		Stmt: dbmeta.Stmt{
			{{Query: `SELECT current_database() AS "catalog"`}},
			{{Query: `, n.nspname AS "schema"`}},
			{{Query: `, p.proname AS "routine"`}},
			{{Query: `, p.oid::text AS "routine_id"`}},
			{{Query: `, NULLIF(p.proargnames[k.ordinality], '') AS "name"`}},
			{{Query: `, k.ordinality AS "ordinal"`}},
			{{Query: `, CASE COALESCE(p.proargmodes[k.ordinality], 'i')` +
				` WHEN 'i' THEN 'in' WHEN 'o' THEN 'out' WHEN 'b' THEN 'inout'` +
				` WHEN 'v' THEN 'variadic' WHEN 't' THEN 'table'` +
				` ELSE p.proargmodes[k.ordinality]::text END AS "mode"`}},
			{{Query: `, pg_catalog.format_type(k.typ, NULL) AS "data_type"`}},
			// A default applies to the last parameters, so the position a
			// default starts at is the count minus pronargdefaults.
			{{Query: `, CASE WHEN p.pronargdefaults > 0` +
				` AND k.ordinality > pg_catalog.array_length(p.proargtypes, 1) - p.pronargdefaults` +
				` AND COALESCE(p.proargmodes[k.ordinality], 'i') IN ('i', 'b', 'v')` +
				` THEN 'yes' ELSE NULL END AS "default"`}},
			{{Query: `FROM pg_catalog.pg_proc p`}},
			{{Query: `JOIN pg_catalog.pg_namespace n ON n.oid = p.pronamespace`}},
			{{Query: `CROSS JOIN LATERAL pg_catalog.unnest(` +
				`COALESCE(p.proallargtypes, p.proargtypes::oid[])) WITH ORDINALITY AS k(typ, ordinality)`}},
			{{Query: `WHERE ` + notSystemSchema}},
			{{Query: `AND (@schema = '' OR n.nspname LIKE @schema)`}},
			{{Query: `AND (@parent = '' OR p.proname LIKE @parent)`}},
			{{Query: `AND (@name = '' OR COALESCE(p.proargnames[k.ordinality], '') LIKE @name)`}},
			{{Query: `ORDER BY 2, 3, 4, 6`}},
		},
		Fields: []dbmeta.Field{
			{Name: "catalog"}, {Name: "schema"},
			{Name: "routine", Desc: "the function or procedure this parameter belongs to"},
			{
				Name: "routine_id",
				Desc: "the oid, because PostgreSQL overloads a name and the name alone does not identify a routine",
			},
			{Name: "name", Desc: "absent for a parameter declared without one"},
			{Name: "ordinal", Desc: "one based position in the declaration"},
			{Name: "mode", Desc: "in, out, inout, variadic, or table for a column of a returned table"},
			{Name: "data_type"},
			{
				Name: "default",
				Desc: "yes when the parameter has a default. PostgreSQL stores the expressions as one list and does not index it per parameter",
			},
		},
		Params: schemaParentName("parameter"),
		Scan: func(rows *sql.Rows) (dbmeta.RoutineParameter, error) {
			var v dbmeta.RoutineParameter
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Routine, &v.RoutineID, &v.Name,
				&v.Ordinal, &v.Mode, &v.DataType, &v.Default)
			return v, err
		},
	})
}

// registerEnumValues is Type.Elements in rows.
func registerEnumValues() {
	dbmeta.EnumValues.Register(dbmeta.PostgreSQL, &dbmeta.Binding[dbmeta.EnumValue]{
		Stmt: dbmeta.Stmt{
			{{Query: `SELECT current_database() AS "catalog"`}},
			{{Query: `, n.nspname AS "schema"`}},
			{{Query: `, t.typname AS "enum"`}},
			{{Query: `, e.enumlabel AS "label"`}},
			{{Query: `, ROW_NUMBER() OVER (PARTITION BY t.oid ORDER BY e.enumsortorder) AS "ordinal"`}},
			{{Query: `FROM pg_catalog.pg_type t`}},
			{{Query: `JOIN pg_catalog.pg_namespace n ON n.oid = t.typnamespace`}},
			{{Query: `JOIN pg_catalog.pg_enum e ON e.enumtypid = t.oid`}},
			{{Query: `WHERE ` + notSystemSchema}},
			{{Query: `AND (@schema = '' OR n.nspname LIKE @schema)`}},
			{{Query: `AND (@name = '' OR t.typname LIKE @name)`}},
			{{Query: `ORDER BY 2, 3, 5`}},
		},
		Fields: []dbmeta.Field{
			{Name: "catalog"}, {Name: "schema"},
			{Name: "enum", Desc: "the enumerated type this label belongs to"},
			{Name: "label"},
			{
				Name: "ordinal",
				Desc: "one based sort order. PostgreSQL stores a float so that a label can be inserted between two others, and this counts instead",
			},
		},
		Params: schemaNameSystem("enum"),
		Scan: func(rows *sql.Rows) (dbmeta.EnumValue, error) {
			var v dbmeta.EnumValue
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Enum, &v.Label, &v.Ordinal)
			return v, err
		},
	})
}

// registerViews answers what a view selects.
//
// pg_relation_is_updatable returns a bit set. Bit 3, worth 8, is whether the
// view accepts an UPDATE, and bit 1, worth 2, whether it accepts an INSERT.
func registerViews() {
	dbmeta.Views.Register(dbmeta.PostgreSQL, &dbmeta.Binding[dbmeta.View]{
		Stmt: dbmeta.Stmt{
			{{Query: `SELECT current_database() AS "catalog"`}},
			{{Query: `, n.nspname AS "schema"`}},
			{{Query: `, c.relname AS "name"`}},
			{{Query: `, pg_catalog.pg_get_viewdef(c.oid, true) AS "definition"`}},
			{{Query: `, CASE WHEN 'check_option=cascaded' = ANY(c.reloptions) THEN 'cascaded'` +
				` WHEN 'check_option=local' = ANY(c.reloptions) THEN 'local'` +
				` ELSE 'none' END AS "check_option"`}},
			{{Query: `, pg_catalog.pg_relation_is_updatable(c.oid, false) & 8 = 8 AS "updatable"`}},
			{{Query: `, pg_catalog.pg_relation_is_updatable(c.oid, false) & 2 = 2 AS "insertable"`}},
			{{Query: `, pg_catalog.obj_description(c.oid, 'pg_class') AS "comment"`}},
			{{Query: `FROM pg_catalog.pg_class c`}},
			{{Query: `JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace`}},
			// v is a view and m a materialized one, which psql lists together
			{{Query: `WHERE c.relkind IN ('v', 'm')`}},
			{{Query: `AND ` + notSystemSchema}},
			{{Query: `AND (@schema = '' OR n.nspname LIKE @schema)`}},
			{{Query: `AND (@name = '' OR c.relname LIKE @name)`}},
			{{Query: `ORDER BY 2, 3`}},
		},
		Fields: []dbmeta.Field{
			{Name: "catalog"}, {Name: "schema"}, {Name: "name"},
			{Name: "definition", Desc: "the statement the view selects, as the catalog stores it"},
			{Name: "check_option", Desc: "none, local or cascaded"},
			{Name: "updatable"}, {Name: "insertable"},
			{Name: "comment"},
		},
		Params: schemaNameSystem("view"),
		Scan: func(rows *sql.Rows) (dbmeta.View, error) {
			var v dbmeta.View
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Name, &v.Definition,
				&v.CheckOption, &v.Updatable, &v.Insertable, &v.Comment)
			return v, err
		},
	})
}

// registerColumnStats backs usql's \ss.
//
// pg_stats is a view over pg_statistic that applies row level security, so it
// shows only the columns the caller may read. A column never analyzed has no
// row, which is a fact rather than a gap.
func registerColumnStats() {
	dbmeta.ColumnStats.Register(dbmeta.PostgreSQL, &dbmeta.Binding[dbmeta.ColumnStat]{
		Stmt: dbmeta.Stmt{
			{{Query: `SELECT current_database() AS "catalog"`}},
			{{Query: `, s.schemaname AS "schema"`}},
			{{Query: `, s.tablename AS "table"`}},
			{{Query: `, s.attname AS "name"`}},
			{{Query: `, s.avg_width AS "avg_width"`}},
			{{Query: `, s.null_frac AS "null_frac"`}},
			{{Query: `, s.n_distinct AS "distinct"`}},
			// The bounds come from the histogram, which is absent for a column
			// with few enough distinct values to fit in the common list.
			{{Query: `, (s.histogram_bounds::text::text[])[1] AS "min"`}},
			{{Query: `, (s.histogram_bounds::text::text[])` +
				`[pg_catalog.array_length(s.histogram_bounds::text::text[], 1)] AS "max"`}},
			// PostgreSQL keeps no mean. It is not padded with a literal,
			// because absent and zero are different. See docs/NULLS.md.
			{{Query: `, NULL AS "mean"`}},
			{{Query: `, pg_catalog.array_to_string(s.most_common_vals::text::text[], E'\n') AS "top_n"`}},
			{{Query: `, pg_catalog.array_to_string(s.most_common_freqs, E'\n') AS "top_n_freqs"`}},
			{{Query: `FROM pg_catalog.pg_stats s`}},
			{{Query: `WHERE (@with_system OR (s.schemaname !~ '^pg_' AND s.schemaname <> 'information_schema'))`}},
			{{Query: `AND (@schema = '' OR s.schemaname LIKE @schema)`}},
			{{Query: `AND (@parent = '' OR s.tablename LIKE @parent)`}},
			{{Query: `AND (@name = '' OR s.attname LIKE @name)`}},
			{{Query: `ORDER BY 2, 3, 4`}},
		},
		Fields: []dbmeta.Field{
			{Name: "catalog"}, {Name: "schema"}, {Name: "table"}, {Name: "name"},
			{Name: "avg_width", Desc: "average size of a value in bytes"},
			{Name: "null_frac", Desc: "fraction of values that are null"},
			{
				Name: "distinct",
				Desc: "distinct values, or a negative number meaning that fraction of the row count",
			},
			{Name: "min", Desc: "lowest histogram bound, absent when there is no histogram"},
			{Name: "max", Desc: "highest histogram bound, absent when there is no histogram"},
			{Name: "mean", Desc: "always absent: PostgreSQL does not compute a mean"},
			{Name: "top_n", Desc: "the most common values, one per line"},
			{Name: "top_n_freqs", Desc: "their frequencies, one per line, in the same order"},
		},
		Params: schemaParentName("column"),
		Scan: func(rows *sql.Rows) (dbmeta.ColumnStat, error) {
			var v dbmeta.ColumnStat
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Table, &v.Name, &v.AvgWidth,
				&v.NullFrac, &v.Distinct, &v.Min, &v.Max, &v.Mean, &v.TopN, &v.TopNFreqs)
			return v, err
		},
	})
}

// registerCurrentSchema answers where an unqualified name resolves.
//
// It is session dependent, which is why it is its own kind and not a field on
// anything. current_schema is the first entry of search_path that exists.
func registerCurrentSchema() {
	// \conninfo. current_user is the effective user and changes with SET
	// ROLE. session_user is who the connection authenticated as and does not.
	dbmeta.CurrentUser.Register(dbmeta.PostgreSQL, &dbmeta.Binding[dbmeta.User]{
		Stmt: dbmeta.Stmt{
			{{Query: `SELECT current_user AS "name"`}},
			{{Query: `, session_user AS "session"`}},
		},
		Fields: []dbmeta.Field{
			{Name: "name", Desc: "the effective user, which SET ROLE changes"},
			{Name: "session", Desc: "the user the connection authenticated as"},
		},
		Scan: func(rows *sql.Rows) (dbmeta.User, error) {
			var v dbmeta.User
			err := rows.Scan(&v.Name, &v.Session)
			return v, err
		},
	})

	dbmeta.CurrentSchema.Register(dbmeta.PostgreSQL, &dbmeta.Binding[dbmeta.Schema]{
		Stmt: dbmeta.Stmt{
			{{Query: `SELECT current_database() AS "catalog"`}},
			{{Query: `, n.nspname AS "name"`}},
			{{Query: `, pg_catalog.pg_get_userbyid(n.nspowner) AS "owner"`}},
			{{Query: `, pg_catalog.obj_description(n.oid, 'pg_namespace') AS "comment"`}},
			{{Query: `FROM pg_catalog.pg_namespace n`}},
			{{Query: `WHERE n.nspname = pg_catalog.current_schema()`}},
		},
		Fields: []dbmeta.Field{
			{Name: "catalog"},
			{Name: "name", Desc: "the first schema of search_path that exists"},
			{Name: "owner"}, {Name: "comment"},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Schema, error) {
			var v dbmeta.Schema
			err := rows.Scan(&v.Catalog, &v.Name, &v.Owner, &v.Comment)
			return v, err
		},
	})
}
