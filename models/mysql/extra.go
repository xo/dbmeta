package mysql

import (
	"database/sql"

	"github.com/xo/dbmeta"
)

// The kinds psql has no command for. D47 allows them, and information_schema
// answers most of them for both products, because the standard defines the
// column level views that psql renders as text.

func registerExtra() {
	registerConstraintColumns()
	registerRoutineParameters()
	registerViews()
	registerColumnStats()
	registerCurrentSchema()
}

// registerConstraintColumns is Constraints in parts.
//
// KEY_COLUMN_USAGE is the standard view and both products have it. It carries
// the referenced side of a foreign key, so this needs no join.
//
// It covers a primary key, a unique constraint and a foreign key. It does not
// cover a check constraint, which has no columns in the standard and whose
// expression Constraints already reports.
func registerConstraintColumns() {
	dbmeta.ConstraintColumns.Register(dbmeta.MySQL, &dbmeta.Binding[dbmeta.ConstraintColumn]{
		Stmt: dbmeta.Stmt{
			{{SQL: `SELECT k.constraint_catalog AS "catalog"`}},
			{{SQL: `, k.table_schema AS "schema"`}},
			{{SQL: `, k.table_name AS "table"`}},
			{{SQL: `, k.constraint_name AS "constraint"`}},
			{{SQL: `, k.column_name AS "name"`}},
			{{SQL: `, k.ordinal_position AS "ordinal"`}},
			{{SQL: `, CASE WHEN k.referenced_table_name IS NULL THEN NULL` +
				` ELSE k.constraint_catalog END AS "foreign_catalog"`}},
			{{SQL: `, k.referenced_table_schema AS "foreign_schema"`}},
			{{SQL: `, k.referenced_table_name AS "foreign_table"`}},
			{{SQL: `, k.referenced_column_name AS "foreign_name"`}},
			{{SQL: `FROM information_schema.KEY_COLUMN_USAGE k`}},
			{{SQL: `WHERE (@with_system OR k.table_schema NOT IN (` + systemSchemas + `))`}},
			{{SQL: `AND (@schema = '' OR k.table_schema LIKE @schema)`}},
			{{SQL: `AND (@parent = '' OR k.table_name LIKE @parent)`}},
			{{SQL: `AND (@name = '' OR k.constraint_name LIKE @name)`}},
			{{SQL: `ORDER BY 2, 3, 4, 6`}},
		},
		Fields: []dbmeta.Field{
			{Name: "catalog"}, {Name: "schema"}, {Name: "table"},
			{Name: "constraint"},
			{Name: "name", Desc: "the column the constraint is on"},
			{Name: "ordinal", Desc: "one based position within the constraint"},
			{Name: "foreign_catalog", Desc: "set for a foreign key, absent otherwise"},
			{Name: "foreign_schema"}, {Name: "foreign_table"},
			{Name: "foreign_name", Desc: "the column this one points at"},
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
// PARAMETERS is the standard view. A function records its return value as
// ordinal zero with no name, which is why Mode reports return for that row
// rather than leaving it empty.
func registerRoutineParameters() {
	dbmeta.RoutineParameters.Register(dbmeta.MySQL, &dbmeta.Binding[dbmeta.RoutineParameter]{
		Stmt: dbmeta.Stmt{
			{{SQL: `SELECT p.specific_catalog AS "catalog"`}},
			{{SQL: `, p.specific_schema AS "schema"`}},
			{{SQL: `, p.specific_name AS "routine"`}},
			{{SQL: `, p.specific_name AS "routine_id"`}},
			{{SQL: `, p.parameter_name AS "name"`}},
			{{SQL: `, p.ordinal_position AS "ordinal"`}},
			{{SQL: `, CASE WHEN p.ordinal_position = 0 THEN 'return'` +
				` ELSE LOWER(COALESCE(p.parameter_mode, 'in')) END AS "mode"`}},
			{{SQL: `, p.dtd_identifier AS "data_type"`}},
			// Padded rather than read. MySQL has no PARAMETER_DEFAULT column
			// at all, MariaDB added one only recently, and neither fills it
			// for an ordinary stored routine, because neither lets a
			// parameter have a default. Reading it would need a version gate
			// for a column that is always empty.
			{{SQL: `, NULL AS "default"`}},
			{{SQL: `FROM information_schema.PARAMETERS p`}},
			{{SQL: `WHERE (@with_system OR p.specific_schema NOT IN (` + systemSchemas + `))`}},
			{{SQL: `AND (@schema = '' OR p.specific_schema LIKE @schema)`}},
			{{SQL: `AND (@parent = '' OR p.specific_name LIKE @parent)`}},
			{{SQL: `AND (@name = '' OR COALESCE(p.parameter_name, '') LIKE @name)`}},
			{{SQL: `ORDER BY 2, 3, 6`}},
		},
		Fields: []dbmeta.Field{
			{Name: "catalog"}, {Name: "schema"},
			{Name: "routine"},
			{
				Name: "routine_id",
				Desc: "the same as the name: neither product overloads a routine, so the name identifies it",
			},
			{Name: "name", Desc: "absent for the row that describes a function's return value"},
			{Name: "ordinal", Desc: "one based, and zero for a function's return value"},
			{Name: "mode", Desc: "in, out, inout, or return for a function's return value"},
			{Name: "data_type", Desc: "the type as declared, with its length"},
			{
				Name: "default",
				Desc: "always absent: neither product lets a stored routine parameter have a default",
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

// registerViews answers what a view selects.
func registerViews() {
	dbmeta.Views.Register(dbmeta.MySQL, &dbmeta.Binding[dbmeta.View]{
		Stmt: dbmeta.Stmt{
			{{SQL: `SELECT v.table_catalog AS "catalog"`}},
			{{SQL: `, v.table_schema AS "schema"`}},
			{{SQL: `, v.table_name AS "name"`}},
			{{SQL: `, v.view_definition AS "definition"`}},
			{{SQL: `, LOWER(v.check_option) AS "check_option"`}},
			{{SQL: `, v.is_updatable = 'YES' AS "updatable"`}},
			// Neither product publishes whether a view accepts an INSERT.
			// It is not the same as accepting an UPDATE.
			{{SQL: `, NULL AS "insertable"`}},
			{{SQL: `, NULLIF(t.table_comment, '') AS "comment"`}},
			{{SQL: `FROM information_schema.VIEWS v`}},
			{{SQL: `LEFT JOIN information_schema.TABLES t` +
				` ON t.table_schema = v.table_schema AND t.table_name = v.table_name`}},
			{{SQL: `WHERE (@with_system OR v.table_schema NOT IN (` + systemSchemas + `))`}},
			{{SQL: `AND (@schema = '' OR v.table_schema LIKE @schema)`}},
			{{SQL: `AND (@name = '' OR v.table_name LIKE @name)`}},
			{{SQL: `ORDER BY 2, 3`}},
		},
		Fields: []dbmeta.Field{
			{Name: "catalog"}, {Name: "schema"}, {Name: "name"},
			{Name: "definition", Desc: "the statement the view selects, rewritten by the server"},
			{Name: "check_option", Desc: "none, local or cascaded"},
			{Name: "updatable"},
			{Name: "insertable", Desc: "always absent: neither product publishes it"},
			{Name: "comment", Desc: "MariaDB and MySQL write VIEW here for every view"},
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

// registerColumnStats backs usql's \ss, for MariaDB only.
//
// mysql.column_stats holds the engine independent statistics that ANALYZE
// TABLE ... PERSISTENT writes. It is the same kind of thing as PostgreSQL's
// pg_statistic: one row per column, describing the values.
//
// MySQL has information_schema.COLUMN_STATISTICS and it is not the same thing.
// It holds one JSON histogram per column and nothing else, so there is no
// width, no null fraction and no distinct count to report, and a histogram
// exists only where somebody ran ANALYZE TABLE ... UPDATE HISTOGRAM. Reporting
// a row with everything absent would be worse than reporting none, so MySQL
// answers ErrNotSupported. See docs/COVERAGE.md.
//
// Reading mysql.column_stats needs SELECT on the mysql schema, like the other
// queries in this model that read it.
func registerColumnStats() {
	dbmeta.ColumnStats.Register(dbmeta.MySQL, &dbmeta.Binding[dbmeta.ColumnStat]{
		Stmt: dbmeta.Stmt{
			{frag(onMaria, `SELECT '' AS "catalog"`)},
			{frag(onMaria, `, c.db_name AS "schema"`)},
			{frag(onMaria, `, c.table_name AS "table"`)},
			{frag(onMaria, `, c.column_name AS "name"`)},
			{frag(onMaria, `, CAST(c.avg_length AS SIGNED) AS "avg_width"`)},
			{frag(onMaria, `, c.nulls_ratio AS "null_frac"`)},
			// MariaDB records the average number of rows per distinct value
			// rather than a count, so the count is the row count over it. A
			// table never analyzed has no row count and reports absent.
			{frag(onMaria, `, CASE WHEN c.avg_frequency > 0 AND t.cardinality IS NOT NULL`+
				` THEN t.cardinality / c.avg_frequency ELSE NULL END AS "distinct"`)},
			{frag(onMaria, `, CONVERT(c.min_value USING utf8mb4) AS "min"`)},
			{frag(onMaria, `, CONVERT(c.max_value USING utf8mb4) AS "max"`)},
			{frag(onMaria, `, NULL AS "mean"`)},
			{frag(onMaria, `, NULL AS "top_n"`)},
			{frag(onMaria, `, NULL AS "top_n_freqs"`)},
			{frag(onMaria, `FROM mysql.column_stats c`)},
			{frag(onMaria, `LEFT JOIN mysql.table_stats t`+
				` ON t.db_name = c.db_name AND t.table_name = c.table_name`)},
			{frag(onMaria, `WHERE (@with_system OR c.db_name NOT IN (`+systemSchemas+`))`)},
			{frag(onMaria, `AND (@schema = '' OR c.db_name LIKE @schema)`)},
			{frag(onMaria, `AND (@parent = '' OR c.table_name LIKE @parent)`)},
			{frag(onMaria, `AND (@name = '' OR c.column_name LIKE @name)`)},
			{frag(onMaria, `ORDER BY 2, 3, 4`)},
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "always empty: MariaDB records no catalog here"},
			{Name: "schema"}, {Name: "table"}, {Name: "name"},
			{Name: "avg_width", Desc: "average size of a value in bytes"},
			{Name: "null_frac"},
			{
				Name: "distinct",
				Desc: "the row count over the average frequency, because MariaDB records the frequency and not the count",
			},
			{Name: "min"}, {Name: "max"},
			{Name: "mean", Desc: "always absent: MariaDB computes no mean"},
			{
				Name: "top_n",
				Desc: "always absent: MariaDB keeps a histogram rather than a list of common values",
			},
			{Name: "top_n_freqs", Desc: "always absent, for the same reason"},
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
// A connection that named no database reports nothing, which is a fact: an
// unqualified name resolves nowhere until a USE statement runs.
func registerCurrentSchema() {
	// CURRENT_USER() is the account the grant tables matched, which is what
	// privileges are decided by. USER() is what the client supplied. They
	// differ when a connection matches a wildcard host or the anonymous
	// account, and both are reported as user@host.
	dbmeta.CurrentUser.Register(dbmeta.MySQL, &dbmeta.Binding[dbmeta.User]{
		Stmt: dbmeta.Stmt{
			{{SQL: "SELECT CURRENT_USER() AS `name`"}},
			{{SQL: ", USER() AS `session`"}},
		},
		Fields: []dbmeta.Field{
			{Name: "name", Desc: "the account the grant tables matched, as user@host"},
			{Name: "session", Desc: "the account the client supplied, as user@host"},
		},
		Scan: func(rows *sql.Rows) (dbmeta.User, error) {
			var v dbmeta.User
			err := rows.Scan(&v.Name, &v.Session)
			return v, err
		},
	})

	dbmeta.CurrentSchema.Register(dbmeta.MySQL, &dbmeta.Binding[dbmeta.Schema]{
		Stmt: dbmeta.Stmt{
			{{SQL: `SELECT s.catalog_name AS "catalog"`}},
			{{SQL: `, s.schema_name AS "name"`}},
			{{SQL: `, '' AS "owner"`}},
			{{SQL: `, NULL AS "comment"`}},
			{{SQL: `FROM information_schema.SCHEMATA s`}},
			{{SQL: `WHERE s.schema_name = DATABASE()`}},
		},
		Fields: []dbmeta.Field{
			{Name: "catalog"},
			{Name: "name", Desc: "the database in use, and no row when the connection named none"},
			{Name: "owner", Desc: "always empty: a schema has no owner here"},
			{Name: "comment", Desc: "always absent"},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Schema, error) {
			var v dbmeta.Schema
			err := rows.Scan(&v.Catalog, &v.Name, &v.Owner, &v.Comment)
			return v, err
		},
	})
}
