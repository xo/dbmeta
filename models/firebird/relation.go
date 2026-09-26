package firebird

import (
	"database/sql"

	"github.com/xo/dbmeta"
)

// noSchema is what every query returns for the schema, because Firebird has
// no such level before 6.0. See the package comment.
const noSchema = `'' AS "schema"`

// schemaDesc says the same thing to a reader of go doc.
const schemaDesc = "always empty: Firebird has one flat namespace per database and gives it no name"

// relationType turns RDB$RELATION_TYPE into the word this package uses.
// Firebird records a view as a relation with a type of its own, so one
// expression covers both and Tables returns them together the way psql does.
const relationType = `CASE r.RDB$RELATION_TYPE` +
	` WHEN 0 THEN 'table'` +
	` WHEN 1 THEN 'view'` +
	` WHEN 2 THEN 'external table'` +
	` WHEN 3 THEN 'virtual table'` +
	` WHEN 4 THEN 'global temporary table'` +
	` WHEN 5 THEN 'global temporary table'` +
	` ELSE 'table' END AS "type"`

func registerRelations() {
	registerTables()
	registerColumns()
	registerIndexes()
	registerConstraints()
	registerTriggers()
}

func registerTables() {
	// \dt and \dv. Firebird records a view in RDB$RELATIONS beside a table,
	// so one statement answers both and the type column separates them.
	dbmeta.Tables.Register(dbmeta.Firebird, &dbmeta.Binding[dbmeta.Table]{
		Stmt: dbmeta.Stmt{
			always(`SELECT '' AS "catalog"`),
			always(`, ` + noSchema),
			always(`, TRIM(TRAILING FROM r.RDB$RELATION_NAME) AS "name"`),
			always(`, ` + relationType),
			always(`, r.RDB$DESCRIPTION AS "comment"`),
			always(`FROM RDB$RELATIONS r`),
			always(`WHERE ` + userObject(`r.RDB$SYSTEM_FLAG`)),
			always(`AND ` + like(`''`, `@schema`) + ``),
			always(`AND ` + like(`r.RDB$RELATION_NAME`, `@name`) + ``),
			always(`ORDER BY r.RDB$RELATION_NAME`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "always empty: Firebird has no level above the database"},
			{Name: "schema", Desc: schemaDesc},
			{Name: "name"},
			{Name: "type", Desc: "table, view, external table, virtual table or global temporary table"},
			{Name: "comment", Desc: "from RDB$DESCRIPTION, which COMMENT ON writes"},
		},
		Params: schemaAndName("table"),
		Scan: func(rows *sql.Rows) (dbmeta.Table, error) {
			var v dbmeta.Table
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Name, &v.Type, &v.Comment)
			return v, err
		},
	})

	// \dv in full. RDB$VIEW_SOURCE holds the text of the SELECT and nothing
	// else, so a caller that wants CREATE VIEW writes the header itself.
	dbmeta.Views.Register(dbmeta.Firebird, &dbmeta.Binding[dbmeta.View]{
		Stmt: dbmeta.Stmt{
			always(`SELECT '' AS "catalog"`),
			always(`, ` + noSchema),
			always(`, TRIM(TRAILING FROM r.RDB$RELATION_NAME) AS "name"`),
			always(`, r.RDB$VIEW_SOURCE AS "definition"`),
			always(`, CAST(NULL AS VARCHAR(8)) AS "check_option"`),
			always(`, CAST(NULL AS BOOLEAN) AS "updatable"`),
			always(`, CAST(NULL AS BOOLEAN) AS "insertable"`),
			always(`, r.RDB$DESCRIPTION AS "comment"`),
			always(`FROM RDB$RELATIONS r`),
			always(`WHERE ` + userObject(`r.RDB$SYSTEM_FLAG`) + ` AND r.RDB$VIEW_BLR IS NOT NULL`),
			always(`AND ` + like(`''`, `@schema`) + ``),
			always(`AND ` + like(`r.RDB$RELATION_NAME`, `@name`) + ``),
			always(`ORDER BY r.RDB$RELATION_NAME`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "always empty: Firebird has no level above the database"},
			{Name: "schema", Desc: schemaDesc},
			{Name: "name"},
			{Name: "definition", Desc: "the SELECT alone, from RDB$VIEW_SOURCE, without a CREATE VIEW header"},
			{Name: "check_option", Desc: "always absent: Firebird records no WITH CHECK OPTION in the catalog"},
			{Name: "updatable", Desc: "always absent: whether a view is updatable is decided when it is used"},
			{Name: "insertable", Desc: "always absent, for the same reason"},
			{Name: "comment"},
		},
		Params: schemaAndName("view"),
		Scan: func(rows *sql.Rows) (dbmeta.View, error) {
			var v dbmeta.View
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Name, &v.Definition,
				&v.CheckOption, &v.Updatable, &v.Insertable, &v.Comment)
			return v, err
		},
	})

	// \ds. A Firebird sequence is a generator and the two words mean the same
	// thing, which is why both spellings work in DDL.
	dbmeta.Sequences.Register(dbmeta.Firebird, &dbmeta.Binding[dbmeta.Sequence]{
		Stmt: dbmeta.Stmt{
			always(`SELECT ` + noSchema),
			always(`, TRIM(TRAILING FROM g.RDB$GENERATOR_NAME) AS "name"`),
			always(`, 'BIGINT' AS "data_type"`),
			always(`, g.RDB$INITIAL_VALUE AS "start"`),
			always(`, CAST(NULL AS BIGINT) AS "minimum"`),
			always(`, CAST(NULL AS BIGINT) AS "maximum"`),
			always(`, CAST(g.RDB$GENERATOR_INCREMENT AS BIGINT) AS "increment"`),
			always(`, CAST(NULL AS BOOLEAN) AS "cycles"`),
			always(`, '' AS "owned_by"`),
			always(`, g.RDB$DESCRIPTION AS "comment"`),
			always(`FROM RDB$GENERATORS g`),
			always(`WHERE ` + userObject(`g.RDB$SYSTEM_FLAG`)),
			always(`AND ` + like(`''`, `@schema`) + ``),
			always(`AND ` + like(`g.RDB$GENERATOR_NAME`, `@name`) + ``),
			always(`ORDER BY g.RDB$GENERATOR_NAME`),
		},
		Fields: []dbmeta.Field{
			{Name: "schema", Desc: schemaDesc},
			{Name: "name"},
			{Name: "data_type", Desc: "always BIGINT: every Firebird generator is 64 bit"},
			{Name: "start", Desc: "from RDB$INITIAL_VALUE, the value the sequence was created with"},
			{Name: "minimum", Desc: "always absent: a generator has no bound of its own and runs to the range of a BIGINT"},
			{Name: "maximum", Desc: "always absent, for the same reason"},
			{Name: "increment", Desc: "from RDB$GENERATOR_INCREMENT"},
			{Name: "cycles", Desc: "always absent: a Firebird generator does not cycle"},
			{Name: "owned_by", Desc: "always empty: an identity column names its generator and the generator does not name the column"},
			{Name: "comment"},
		},
		Params: schemaAndName("sequence"),
		Scan: func(rows *sql.Rows) (dbmeta.Sequence, error) {
			var v dbmeta.Sequence
			err := rows.Scan(&v.Schema, &v.Name, &v.DataType, &v.Start, &v.Minimum,
				&v.Maximum, &v.Increment, &v.Cycles, &v.OwnedBy, &v.Comment)
			return v, err
		},
	})
}

// schemaAndName is the filter set most queries here take. The schema filter
// is kept even though Firebird has no schemas, so that a caller written
// against another dialect is not refused for naming it.
func schemaAndName(kind string) []dbmeta.Param {
	return []dbmeta.Param{
		{
			Name: "schema",
			Desc: "schema name pattern. Firebird has no schemas, so only an" +
				" empty value or a pattern matching the empty string returns rows",
			Default: "",
		},
		{Name: "name", Desc: kind + " name pattern, empty for every one", Default: ""},
	}
}

// inPrimaryKey is a correlated test of whether a column is part of the
// primary key. It is one bounded subquery per row, which D47 allows: the
// index of a primary key names a handful of columns whatever the size of the
// catalog.
const inPrimaryKey = `EXISTS (SELECT 1 FROM RDB$RELATION_CONSTRAINTS rc` +
	` JOIN RDB$INDEX_SEGMENTS s ON s.RDB$INDEX_NAME = rc.RDB$INDEX_NAME` +
	` WHERE rc.RDB$RELATION_NAME = rf.RDB$RELATION_NAME` +
	` AND rc.RDB$CONSTRAINT_TYPE = 'PRIMARY KEY'` +
	` AND s.RDB$FIELD_NAME = rf.RDB$FIELD_NAME) AS "primary_key"`

func registerColumns() {
	// The columns of a table or a view. RDB$RELATION_FIELDS holds what the
	// relation says and RDB$FIELDS holds the type, which is a row of its own
	// because a domain and a column share the same table. RDB$FIELD_SOURCE is
	// the join, and it names the domain where the column was declared with
	// one and an internal RDB$ name where it was not.
	dbmeta.Columns.Register(dbmeta.Firebird, &dbmeta.Binding[dbmeta.Column]{
		Stmt: dbmeta.Stmt{
			always(`SELECT '' AS "catalog"`),
			always(`, ` + noSchema),
			always(`, TRIM(TRAILING FROM rf.RDB$RELATION_NAME) AS "table"`),
			always(`, TRIM(TRAILING FROM rf.RDB$FIELD_NAME) AS "name"`),
			always(`, rf.RDB$FIELD_POSITION + 1 AS "ordinal"`),
			always(`, ` + fieldType("f") + ` AS "data_type"`),
			// NOT NULL can be recorded on the column or on the domain behind
			// it, and either one makes the column not nullable.
			always(`, COALESCE(rf.RDB$NULL_FLAG, f.RDB$NULL_FLAG, 0) = 0 AS "nullable"`),
			// The column's own default wins over the domain's, which is what
			// Firebird does when it writes a row.
			always(`, COALESCE(rf.RDB$DEFAULT_SOURCE, f.RDB$DEFAULT_SOURCE) AS "default"`),
			always(`, ` + inPrimaryKey),
			always(`, CASE rf.RDB$IDENTITY_TYPE WHEN 0 THEN 'always'` +
				` WHEN 1 THEN 'by default' ELSE '' END AS "identity"`),
			always(`, CASE WHEN f.RDB$COMPUTED_SOURCE IS NOT NULL` +
				` THEN 'virtual' ELSE '' END AS "generated"`),
			always(`, rf.RDB$DESCRIPTION AS "comment"`),
			always(`FROM RDB$RELATION_FIELDS rf`),
			always(`JOIN RDB$RELATIONS r ON r.RDB$RELATION_NAME = rf.RDB$RELATION_NAME`),
			always(`JOIN RDB$FIELDS f ON f.RDB$FIELD_NAME = rf.RDB$FIELD_SOURCE`),
			always(`WHERE ` + userObject(`r.RDB$SYSTEM_FLAG`)),
			always(`AND ` + like(`''`, `@schema`) + ``),
			always(`AND ` + like(`rf.RDB$RELATION_NAME`, `@parent`) + ``),
			always(`AND ` + like(`rf.RDB$FIELD_NAME`, `@name`) + ``),
			always(`ORDER BY rf.RDB$RELATION_NAME, rf.RDB$FIELD_POSITION`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "always empty: Firebird has no level above the database"},
			{Name: "schema", Desc: schemaDesc},
			{Name: "table"}, {Name: "name"},
			{Name: "ordinal", Desc: "from RDB$FIELD_POSITION, which counts from zero, plus one"},
			{Name: "data_type", Desc: "assembled from the type code, sub type, length, precision and scale, which is how Firebird stores a type"},
			{Name: "nullable", Desc: "false when either the column or the domain behind it records NOT NULL"},
			{Name: "default", Desc: "the source text, which begins with the word DEFAULT. The column's own default, or the domain's where the column has none"},
			{Name: "primary_key"},
			{Name: "identity", Desc: "always or by default, and empty when the column is not an identity"},
			{Name: "generated", Desc: "virtual for a COMPUTED BY column, and empty otherwise. Firebird computes such a column on read and never stores it"},
			{Name: "comment"},
		},
		Params: []dbmeta.Param{
			{Name: "schema", Desc: "schema name pattern. Firebird has no schemas", Default: ""},
			{Name: "parent", Desc: "table name pattern, empty for every table", Default: ""},
			{Name: "name", Desc: "column name pattern, empty for every column", Default: ""},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Column, error) {
			var v dbmeta.Column
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Table, &v.Name, &v.Ordinal,
				&v.DataType, &v.Nullable, &v.Default, &v.PrimaryKey, &v.Identity,
				&v.Generated, &v.Comment)
			return v, err
		},
	})
}

func registerIndexes() {
	// \di. RDB$INDICES holds an index whether a constraint made it or not,
	// and the primary flag comes from the constraint that owns it.
	dbmeta.Indexes.Register(dbmeta.Firebird, &dbmeta.Binding[dbmeta.Index]{
		Stmt: dbmeta.Stmt{
			always(`SELECT '' AS "catalog"`),
			always(`, ` + noSchema),
			always(`, TRIM(TRAILING FROM i.RDB$RELATION_NAME) AS "table"`),
			always(`, TRIM(TRAILING FROM i.RDB$INDEX_NAME) AS "name"`),
			always(`, 'btree' AS "type"`),
			always(`, COALESCE(i.RDB$UNIQUE_FLAG, 0) = 1 AS "unique"`),
			always(`, EXISTS (SELECT 1 FROM RDB$RELATION_CONSTRAINTS rc` +
				` WHERE rc.RDB$INDEX_NAME = i.RDB$INDEX_NAME` +
				` AND rc.RDB$CONSTRAINT_TYPE = 'PRIMARY KEY') AS "primary"`),
			always(`, i.RDB$DESCRIPTION AS "comment"`),
			always(`FROM RDB$INDICES i`),
			always(`WHERE ` + userObject(`i.RDB$SYSTEM_FLAG`)),
			always(`AND ` + like(`''`, `@schema`) + ``),
			always(`AND ` + like(`i.RDB$RELATION_NAME`, `@parent`) + ``),
			always(`AND ` + like(`i.RDB$INDEX_NAME`, `@name`) + ``),
			always(`ORDER BY i.RDB$RELATION_NAME, i.RDB$INDEX_NAME`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "always empty: Firebird has no level above the database"},
			{Name: "schema", Desc: schemaDesc},
			{Name: "table"}, {Name: "name"},
			{Name: "type", Desc: "always btree: Firebird has one index kind and no pluggable access method"},
			{Name: "unique"},
			{Name: "primary", Desc: "true when a PRIMARY KEY constraint owns this index"},
			{Name: "comment"},
		},
		Params: parentAndName("index"),
		Scan: func(rows *sql.Rows) (dbmeta.Index, error) {
			var v dbmeta.Index
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Table, &v.Name, &v.Type,
				&v.Unique, &v.Primary, &v.Comment)
			return v, err
		},
	})

	// The columns of each index. An expression index records no segment at
	// all, so its expression arrives with a NULL name, which is why the name
	// here is the nullable one and the ordinal is not.
	dbmeta.IndexColumns.Register(dbmeta.Firebird, &dbmeta.Binding[dbmeta.IndexColumn]{
		Stmt: dbmeta.Stmt{
			always(`SELECT ` + noSchema),
			always(`, TRIM(TRAILING FROM i.RDB$RELATION_NAME) AS "table"`),
			always(`, TRIM(TRAILING FROM i.RDB$INDEX_NAME) AS "index"`),
			always(`, TRIM(TRAILING FROM COALESCE(s.RDB$FIELD_NAME, '')) AS "name"`),
			always(`, CAST(COALESCE(s.RDB$FIELD_POSITION, 0) + 1 AS BIGINT) AS "ordinal"`),
			always(`, i.RDB$EXPRESSION_SOURCE AS "expression"`),
			always(`, COALESCE(i.RDB$INDEX_TYPE, 0) = 1 AS "descending"`),
			always(`FROM RDB$INDICES i`),
			always(`LEFT JOIN RDB$INDEX_SEGMENTS s ON s.RDB$INDEX_NAME = i.RDB$INDEX_NAME`),
			always(`WHERE ` + userObject(`i.RDB$SYSTEM_FLAG`)),
			always(`AND ` + like(`''`, `@schema`) + ``),
			always(`AND ` + like(`i.RDB$RELATION_NAME`, `@parent`) + ``),
			always(`AND ` + like(`i.RDB$INDEX_NAME`, `@name`) + ``),
			always(`ORDER BY i.RDB$RELATION_NAME, i.RDB$INDEX_NAME, s.RDB$FIELD_POSITION`),
		},
		Fields: []dbmeta.Field{
			{Name: "schema", Desc: schemaDesc},
			{Name: "table"}, {Name: "index"},
			{Name: "name", Desc: "empty for an expression index, which records an expression and no segment"},
			{Name: "ordinal", Desc: "from RDB$FIELD_POSITION, which counts from zero, plus one"},
			{Name: "expression", Desc: "the source text of an expression index, absent for an ordinary one"},
			{Name: "descending", Desc: "from RDB$INDEX_TYPE, which is the whole index rather than the column: Firebird orders every segment the same way"},
		},
		Params: parentAndName("index"),
		Scan: func(rows *sql.Rows) (dbmeta.IndexColumn, error) {
			var v dbmeta.IndexColumn
			err := rows.Scan(&v.Schema, &v.Table, &v.Index, &v.Name, &v.Ordinal,
				&v.Expression, &v.Descending)
			return v, err
		},
	})
}

// parentAndName is the filter set for a child of a table.
func parentAndName(kind string) []dbmeta.Param {
	return []dbmeta.Param{
		{Name: "schema", Desc: "schema name pattern. Firebird has no schemas", Default: ""},
		{Name: "parent", Desc: "table name pattern, empty for every table", Default: ""},
		{Name: "name", Desc: kind + " name pattern, empty for every one", Default: ""},
	}
}

// checkSource reaches the text of a CHECK constraint.
//
// Firebird implements a check with a pair of system triggers and records the
// expression on them rather than on the constraint, so RDB$CHECK_CONSTRAINTS
// is a map from the constraint to its triggers and the source is one join
// further on. There is more than one trigger per check, so this takes the
// first by name to get one row.
const checkSource = `(SELECT FIRST 1 t.RDB$TRIGGER_SOURCE FROM RDB$CHECK_CONSTRAINTS cc` +
	` JOIN RDB$TRIGGERS t ON t.RDB$TRIGGER_NAME = cc.RDB$TRIGGER_NAME` +
	` WHERE cc.RDB$CONSTRAINT_NAME = rc.RDB$CONSTRAINT_NAME` +
	` ORDER BY t.RDB$TRIGGER_NAME) AS "definition"`

func registerConstraints() {
	// The constraints on a table. Firebird records NOT NULL as a constraint
	// of its own, which PostgreSQL does not, so it appears here as a row with
	// type not null. That is the product's answer rather than a shaped one.
	dbmeta.Constraints.Register(dbmeta.Firebird, &dbmeta.Binding[dbmeta.Constraint]{
		Stmt: dbmeta.Stmt{
			always(`SELECT ` + noSchema),
			always(`, TRIM(TRAILING FROM rc.RDB$RELATION_NAME) AS "table"`),
			always(`, TRIM(TRAILING FROM rc.RDB$CONSTRAINT_NAME) AS "name"`),
			always(`, LOWER(TRIM(TRAILING FROM rc.RDB$CONSTRAINT_TYPE)) AS "type"`),
			always(`, ` + checkSource),
			always(`, rc.RDB$DEFERRABLE = 'YES' AS "deferrable"`),
			always(`, rc.RDB$INITIALLY_DEFERRED = 'YES' AS "deferred"`),
			always(`, CAST(NULL AS VARCHAR(1)) AS "comment"`),
			always(`FROM RDB$RELATION_CONSTRAINTS rc`),
			always(`JOIN RDB$RELATIONS r ON r.RDB$RELATION_NAME = rc.RDB$RELATION_NAME`),
			always(`WHERE ` + userObject(`r.RDB$SYSTEM_FLAG`)),
			always(`AND ` + like(`''`, `@schema`) + ``),
			always(`AND ` + like(`rc.RDB$RELATION_NAME`, `@parent`) + ``),
			always(`AND ` + like(`rc.RDB$CONSTRAINT_NAME`, `@name`) + ``),
			always(`ORDER BY rc.RDB$RELATION_NAME, rc.RDB$CONSTRAINT_NAME`),
		},
		Fields: []dbmeta.Field{
			{Name: "schema", Desc: schemaDesc},
			{Name: "table"}, {Name: "name"},
			{Name: "type", Desc: "primary key, foreign key, unique, check or not null. Firebird records NOT NULL as a constraint and PostgreSQL does not"},
			{Name: "definition", Desc: "the CHECK text, reached through the system triggers that implement it. Absent for every other kind, which records no expression"},
			{Name: "deferrable", Desc: "always false: Firebird accepts the word and defers nothing"},
			{Name: "deferred", Desc: "always false, for the same reason"},
			{Name: "comment", Desc: "always absent: COMMENT ON has no constraint form"},
		},
		Params: parentAndName("constraint"),
		Scan: func(rows *sql.Rows) (dbmeta.Constraint, error) {
			var v dbmeta.Constraint
			err := rows.Scan(&v.Schema, &v.Table, &v.Name, &v.Type, &v.Definition,
				&v.Deferrable, &v.Deferred, &v.Comment)
			return v, err
		},
	})

	// The columns of each constraint, and the column each foreign key points
	// at. A foreign key names the unique constraint it references rather than
	// the table, so the target takes three more joins: to RDB$REF_CONSTRAINTS
	// for the referenced constraint, to its row in RDB$RELATION_CONSTRAINTS
	// for the table, and to that constraint's index segments for the column
	// in the matching position.
	dbmeta.ConstraintColumns.Register(dbmeta.Firebird, &dbmeta.Binding[dbmeta.ConstraintColumn]{
		Stmt: dbmeta.Stmt{
			always(`SELECT '' AS "catalog"`),
			always(`, ` + noSchema),
			always(`, TRIM(TRAILING FROM rc.RDB$RELATION_NAME) AS "table"`),
			always(`, TRIM(TRAILING FROM rc.RDB$CONSTRAINT_NAME) AS "constraint"`),
			always(`, TRIM(TRAILING FROM s.RDB$FIELD_NAME) AS "name"`),
			always(`, CAST(s.RDB$FIELD_POSITION + 1 AS BIGINT) AS "ordinal"`),
			always(`, CASE WHEN urc.RDB$RELATION_NAME IS NULL THEN NULL` +
				` ELSE '' END AS "foreign_catalog"`),
			always(`, CASE WHEN urc.RDB$RELATION_NAME IS NULL THEN NULL` +
				` ELSE '' END AS "foreign_schema"`),
			always(`, TRIM(TRAILING FROM urc.RDB$RELATION_NAME) AS "foreign_table"`),
			always(`, TRIM(TRAILING FROM us.RDB$FIELD_NAME) AS "foreign_name"`),
			always(`FROM RDB$RELATION_CONSTRAINTS rc`),
			always(`JOIN RDB$RELATIONS r ON r.RDB$RELATION_NAME = rc.RDB$RELATION_NAME`),
			always(`JOIN RDB$INDEX_SEGMENTS s ON s.RDB$INDEX_NAME = rc.RDB$INDEX_NAME`),
			always(`LEFT JOIN RDB$REF_CONSTRAINTS ref ON ref.RDB$CONSTRAINT_NAME = rc.RDB$CONSTRAINT_NAME`),
			always(`LEFT JOIN RDB$RELATION_CONSTRAINTS urc ON urc.RDB$CONSTRAINT_NAME = ref.RDB$CONST_NAME_UQ`),
			always(`LEFT JOIN RDB$INDEX_SEGMENTS us ON us.RDB$INDEX_NAME = urc.RDB$INDEX_NAME` +
				` AND us.RDB$FIELD_POSITION = s.RDB$FIELD_POSITION`),
			always(`WHERE ` + userObject(`r.RDB$SYSTEM_FLAG`)),
			always(`AND ` + like(`''`, `@schema`) + ``),
			always(`AND ` + like(`rc.RDB$RELATION_NAME`, `@parent`) + ``),
			always(`AND ` + like(`rc.RDB$CONSTRAINT_NAME`, `@name`) + ``),
			// A check has no index, so it has no segments and the arm
			// above never reaches it. Its columns are recorded instead as
			// the dependencies of the pair of system triggers that
			// implement it, which is one join further on and is the only
			// place Firebird keeps them.
			always(`UNION ALL`),
			always(`SELECT '', ''`),
			always(`, TRIM(TRAILING FROM x.t)`),
			always(`, TRIM(TRAILING FROM x.c)`),
			always(`, TRIM(TRAILING FROM x.f)`),
			// A dependency carries no position, so the columns of a check
			// are numbered by name. It is an order rather than the order
			// they were written in, which Firebird does not record.
			always(`, CAST(ROW_NUMBER() OVER (PARTITION BY x.c ORDER BY x.f) AS BIGINT)`),
			always(`, CAST(NULL AS VARCHAR(1)), CAST(NULL AS VARCHAR(1))`),
			always(`, CAST(NULL AS VARCHAR(1)), CAST(NULL AS VARCHAR(1))`),
			// DISTINCT, because a check is implemented by one trigger for
			// INSERT and one for UPDATE and both depend on the same column.
			always(`FROM (SELECT DISTINCT rc.RDB$RELATION_NAME AS t` +
				`, rc.RDB$CONSTRAINT_NAME AS c, d.RDB$FIELD_NAME AS f` +
				` FROM RDB$RELATION_CONSTRAINTS rc` +
				` JOIN RDB$RELATIONS r ON r.RDB$RELATION_NAME = rc.RDB$RELATION_NAME` +
				` JOIN RDB$CHECK_CONSTRAINTS cc ON cc.RDB$CONSTRAINT_NAME = rc.RDB$CONSTRAINT_NAME` +
				` JOIN RDB$DEPENDENCIES d ON d.RDB$DEPENDENT_NAME = cc.RDB$TRIGGER_NAME` +
				` AND d.RDB$DEPENDED_ON_NAME = rc.RDB$RELATION_NAME` +
				` WHERE rc.RDB$CONSTRAINT_TYPE = 'CHECK'` +
				` AND d.RDB$FIELD_NAME IS NOT NULL` +
				` AND ` + userObject(`r.RDB$SYSTEM_FLAG`) +
				` AND ` + like("rc.RDB$RELATION_NAME", "@parent") +
				` AND ` + like("rc.RDB$CONSTRAINT_NAME", "@name") + `) x`),
			always(`WHERE ` + like("''", "@schema")),
			always(`ORDER BY 3, 4, 6`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "always empty: Firebird has no level above the database"},
			{Name: "schema", Desc: schemaDesc},
			{Name: "table"}, {Name: "constraint"}, {Name: "name"},
			{Name: "ordinal", Desc: "from RDB$FIELD_POSITION, which counts from zero, plus one. The columns of a check are numbered by name instead, because Firebird records no position for one"},
			{Name: "foreign_catalog", Desc: "empty for a foreign key and absent for every other kind"},
			{Name: "foreign_schema", Desc: "empty for a foreign key and absent for every other kind"},
			{Name: "foreign_table", Desc: "absent unless this is a foreign key"},
			{Name: "foreign_name", Desc: "absent unless this is a foreign key"},
		},
		Params: parentAndName("constraint"),
		Scan: func(rows *sql.Rows) (dbmeta.ConstraintColumn, error) {
			var v dbmeta.ConstraintColumn
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Table, &v.Constraint, &v.Name,
				&v.Ordinal, &v.ForeignCatalog, &v.ForeignSchema, &v.ForeignTable, &v.ForeignName)
			return v, err
		},
	})
}

func registerTriggers() {
	// The triggers on a table. A Firebird trigger with no relation is a
	// database trigger, which EventTriggers reads instead, so this takes only
	// the ones that name a table.
	dbmeta.Triggers.Register(dbmeta.Firebird, &dbmeta.Binding[dbmeta.Trigger]{
		Stmt: dbmeta.Stmt{
			always(`SELECT ` + noSchema),
			always(`, TRIM(TRAILING FROM t.RDB$RELATION_NAME) AS "table"`),
			always(`, TRIM(TRAILING FROM t.RDB$TRIGGER_NAME) AS "name"`),
			always(`, CASE WHEN COALESCE(t.RDB$TRIGGER_INACTIVE, 0) = 1` +
				` THEN 'disabled' ELSE 'enabled' END AS "enabled"`),
			always(`, COALESCE(t.RDB$TRIGGER_SOURCE, '') AS "definition"`),
			always(`, t.RDB$DESCRIPTION AS "comment"`),
			always(`FROM RDB$TRIGGERS t`),
			always(`WHERE ` + userObject(`t.RDB$SYSTEM_FLAG`) + ` AND t.RDB$RELATION_NAME IS NOT NULL`),
			always(`AND ` + like(`''`, `@schema`) + ``),
			always(`AND ` + like(`t.RDB$RELATION_NAME`, `@parent`) + ``),
			always(`AND ` + like(`t.RDB$TRIGGER_NAME`, `@name`) + ``),
			always(`ORDER BY t.RDB$RELATION_NAME, t.RDB$TRIGGER_NAME`),
		},
		Fields: []dbmeta.Field{
			{Name: "schema", Desc: schemaDesc},
			{Name: "table"}, {Name: "name"},
			{Name: "enabled", Desc: "from RDB$TRIGGER_INACTIVE"},
			{Name: "definition", Desc: "the body alone, from RDB$TRIGGER_SOURCE, without a CREATE TRIGGER header"},
			{Name: "comment"},
		},
		Params: parentAndName("trigger"),
		Scan: func(rows *sql.Rows) (dbmeta.Trigger, error) {
			var v dbmeta.Trigger
			err := rows.Scan(&v.Schema, &v.Table, &v.Name, &v.Enabled, &v.Definition, &v.Comment)
			return v, err
		},
	})
}
