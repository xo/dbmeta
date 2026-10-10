package avatica

import (
	"database/sql"

	"github.com/xo/dbmeta"
)

func registerRelations() {
	registerSchemas()
	registerTables()
	registerColumns()
	registerIndexes()
	registerConstraints()
	registerTriggers()
}

// tableType is the word for a table's kind. HSQLDB keeps a table in memory,
// in a cache file or in a text file, and records which.
//
// HSQLDB pads a CASE of string literals to the longest one, as if it were a
// CHAR, so every CASE of literals here is trimmed.
const tableType = `RTRIM(CASE t.TABLE_TYPE WHEN 'TABLE' THEN LOWER(t.HSQLDB_TYPE) || ' table'` +
	` WHEN 'VIEW' THEN 'view' WHEN 'SYSTEM TABLE' THEN 'system table'` +
	` WHEN 'GLOBAL TEMPORARY' THEN 'global temporary table' ELSE LOWER(t.TABLE_TYPE) END)`

func registerSchemas() {
	// SYSTEM_SCHEMAS lists every schema the user can see, and SCHEMATA lists
	// the schemas the user owns, so the owner is absent for a schema that an
	// ordinary user does not own.
	dbmeta.Schemas.Register(dbmeta.Avatica, &dbmeta.Binding[dbmeta.Schema]{
		Stmt: dbmeta.Stmt{
			always(`SELECT s.TABLE_CATALOG AS "catalog"`),
			always(`, s.TABLE_SCHEM AS "name"`),
			always(`, o.SCHEMA_OWNER AS "owner"`),
			always(`, ` + text + ` AS "comment"`),
			always(`FROM INFORMATION_SCHEMA.SYSTEM_SCHEMAS s`),
			always(`LEFT JOIN INFORMATION_SCHEMA.SCHEMATA o ON o.SCHEMA_NAME = s.TABLE_SCHEM`),
			always(`WHERE ` + notSystem("s.TABLE_SCHEM")),
			always(`AND ` + like("s.TABLE_SCHEM", "@name")),
			always(`ORDER BY 2`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: catalogDesc},
			{Name: "name"},
			{Name: "owner", Desc: "from SCHEMATA, which lists only the schemas the user owns. It is empty for a schema that an ordinary user does not own, and the administrator sees every owner"},
			{Name: "comment", Desc: "always absent: HSQLDB has no COMMENT ON a schema"},
		},
		Params: []dbmeta.Param{
			{Name: "name", Desc: "schema name pattern, empty for every schema", Default: ""},
			system,
		},
		Scan: func(rows *sql.Rows) (dbmeta.Schema, error) {
			var v dbmeta.Schema
			err := rows.Scan(&v.Catalog, &v.Name, dbmeta.NullAsEmpty(&v.Owner), &v.Comment)
			return v, err
		},
	})

	dbmeta.CurrentSchema.Register(dbmeta.Avatica, &dbmeta.Binding[dbmeta.Schema]{
		Stmt: dbmeta.Stmt{
			always(`SELECT CURRENT_CATALOG AS "catalog"`),
			always(`, CURRENT_SCHEMA AS "name"`),
			always(`, (SELECT o.SCHEMA_OWNER FROM INFORMATION_SCHEMA.SCHEMATA o WHERE o.SCHEMA_NAME = CURRENT_SCHEMA) AS "owner"`),
			always(`, ` + text + ` AS "comment"`),
			always(`FROM (VALUES (0)) AS d (x)`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: catalogDesc},
			{Name: "name", Desc: "from CURRENT_SCHEMA, which is PUBLIC until the session sets another"},
			{Name: "owner", Desc: "empty for a schema that an ordinary user does not own, as in Schemas"},
			{Name: "comment", Desc: "always absent: HSQLDB has no COMMENT ON a schema"},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Schema, error) {
			var v dbmeta.Schema
			err := rows.Scan(&v.Catalog, &v.Name, dbmeta.NullAsEmpty(&v.Owner), &v.Comment)
			return v, err
		},
	})
}

func registerTables() {
	// SYSTEM_TABLES holds every table and view the user can see, and its
	// REMARKS is what COMMENT ON wrote. The tables of INFORMATION_SCHEMA are
	// system tables.
	dbmeta.Tables.Register(dbmeta.Avatica, &dbmeta.Binding[dbmeta.Table]{
		Stmt: dbmeta.Stmt{
			always(`SELECT t.TABLE_CAT AS "catalog"`),
			always(`, t.TABLE_SCHEM AS "schema"`),
			always(`, t.TABLE_NAME AS "name"`),
			always(`, ` + tableType + ` AS "type"`),
			always(`, t.REMARKS AS "comment"`),
			always(`, o.SCHEMA_OWNER AS "owner"`),
			always(`, RTRIM(CASE t.TABLE_TYPE WHEN 'TABLE' THEN 'permanent' WHEN 'GLOBAL TEMPORARY' THEN 'temporary' ELSE NULL END) AS "persistence"`),
			always(`FROM INFORMATION_SCHEMA.SYSTEM_TABLES t`),
			always(`LEFT JOIN INFORMATION_SCHEMA.SCHEMATA o ON o.SCHEMA_NAME = t.TABLE_SCHEM`),
			always(`WHERE ` + notSystem("t.TABLE_SCHEM")),
			always(`AND ` + like("t.TABLE_SCHEM", "@schema")),
			always(`AND ` + like("t.TABLE_NAME", "@name")),
			always(`AND (CAST(@types AS VARCHAR(256)) = '' OR ` + inList("@types", tableType) + `)`),
			always(`ORDER BY 2, 3`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: catalogDesc},
			{Name: "schema"},
			{Name: "name"},
			{Name: "type", Desc: "memory table, cached table or text table, which is where HSQLDB keeps the rows, then global temporary table, view or system table"},
			{Name: "comment", Desc: "from REMARKS, which COMMENT ON writes"},
			{Name: "owner", Desc: "the owner of the schema, which owns every object in it. SCHEMATA lists only the schemas the user owns, so it is absent for another user's schema"},
			{Name: "persistence", Desc: "permanent, or temporary for a global temporary table, and absent for a view"},
		},
		Params: append(schemaAndName("table"), dbmeta.TypesParam()),
		Scan: func(rows *sql.Rows) (dbmeta.Table, error) {
			var v dbmeta.Table
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Name, &v.Type, &v.Comment, &v.Owner, &v.Persistence)
			return v, err
		},
	})

	dbmeta.Views.Register(dbmeta.Avatica, &dbmeta.Binding[dbmeta.View]{
		Stmt: dbmeta.Stmt{
			always(`SELECT v.TABLE_CATALOG AS "catalog"`),
			always(`, v.TABLE_SCHEMA AS "schema"`),
			always(`, v.TABLE_NAME AS "name"`),
			always(`, v.VIEW_DEFINITION AS "definition"`),
			always(`, LOWER(v.CHECK_OPTION) AS "check_option"`),
			always(`, v.IS_UPDATABLE = 'YES' AS "updatable"`),
			always(`, v.INSERTABLE_INTO = 'YES' AS "insertable"`),
			always(`, t.REMARKS AS "comment"`),
			always(`FROM INFORMATION_SCHEMA.VIEWS v`),
			always(`LEFT JOIN INFORMATION_SCHEMA.SYSTEM_TABLES t ON t.TABLE_SCHEM = v.TABLE_SCHEMA AND t.TABLE_NAME = v.TABLE_NAME`),
			always(`WHERE ` + notSystem("v.TABLE_SCHEMA")),
			always(`AND ` + like("v.TABLE_SCHEMA", "@schema")),
			always(`AND ` + like("v.TABLE_NAME", "@name")),
			always(`ORDER BY 2, 3`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: catalogDesc},
			{Name: "schema"}, {Name: "name"},
			{Name: "definition", Desc: "the query of the view, which HSQLDB rewrites with every name in upper case and qualified. It is absent when the user cannot read it"},
			{Name: "check_option", Desc: "cascaded or none, from CHECK_OPTION"},
			{Name: "updatable", Desc: "from IS_UPDATABLE"},
			{Name: "insertable", Desc: "from INSERTABLE_INTO"},
			{Name: "comment", Desc: "from REMARKS, which COMMENT ON writes"},
		},
		Params: schemaAndName("view"),
		Scan: func(rows *sql.Rows) (dbmeta.View, error) {
			var v dbmeta.View
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Name, &v.Definition,
				&v.CheckOption, &v.Updatable, &v.Insertable, &v.Comment)
			return v, err
		},
	})

	dbmeta.Sequences.Register(dbmeta.Avatica, &dbmeta.Binding[dbmeta.Sequence]{
		Stmt: dbmeta.Stmt{
			always(`SELECT q.SEQUENCE_SCHEMA AS "schema"`),
			always(`, q.SEQUENCE_NAME AS "name"`),
			always(`, q.DATA_TYPE AS "data_type"`),
			always(`, CAST(q.START_WITH AS VARCHAR(40)) AS "start"`),
			always(`, CAST(q.MINIMUM_VALUE AS VARCHAR(40)) AS "minimum"`),
			always(`, CAST(q.MAXIMUM_VALUE AS VARCHAR(40)) AS "maximum"`),
			always(`, CAST(q.INCREMENT AS VARCHAR(40)) AS "increment"`),
			always(`, q.CYCLE_OPTION = 'YES' AS "cycles"`),
			always(`, '' AS "owned_by"`),
			always(`, ` + text + ` AS "comment"`),
			always(`FROM INFORMATION_SCHEMA.SEQUENCES q`),
			always(`WHERE ` + notSystem("q.SEQUENCE_SCHEMA")),
			always(`AND ` + like("q.SEQUENCE_SCHEMA", "@schema")),
			always(`AND ` + like("q.SEQUENCE_NAME", "@name")),
			always(`ORDER BY 1, 2`),
		},
		Fields: []dbmeta.Field{
			{Name: "schema"}, {Name: "name"},
			{Name: "data_type", Desc: "from DATA_TYPE, which is BIGINT unless the sequence says AS INTEGER"},
			{Name: "start", Desc: "from START_WITH"},
			{Name: "minimum"}, {Name: "maximum"}, {Name: "increment"},
			{Name: "cycles", Desc: "from CYCLE_OPTION"},
			{Name: "owned_by", Desc: "always empty: an identity column keeps its counter inside the column, and no sequence names a column"},
			{Name: "comment", Desc: "always absent: COMMENT ON takes no sequence in HSQLDB"},
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

func registerColumns() {
	// COLUMNS holds the columns of a table and of a view, and SYSTEM_COMMENTS
	// holds the comment. The key is a column of SYSTEM_PRIMARYKEYS. The comments
	// are joined as a derived table, which HSQLDB indexes. Joined as the view,
	// they took 1.5 seconds for 13,000 columns, against 0.03 (D186).
	dbmeta.Columns.Register(dbmeta.Avatica, &dbmeta.Binding[dbmeta.Column]{
		Stmt: dbmeta.Stmt{
			always(`SELECT c.TABLE_CATALOG AS "catalog"`),
			always(`, c.TABLE_SCHEMA AS "schema"`),
			always(`, c.TABLE_NAME AS "table"`),
			always(`, c.COLUMN_NAME AS "name"`),
			always(`, c.ORDINAL_POSITION AS "ordinal"`),
			always(`, CASE WHEN c.DOMAIN_NAME IS NOT NULL THEN c.DOMAIN_NAME ELSE ` + sqlType("c", "UDT_NAME") + ` END AS "data_type"`),
			always(`, c.IS_NULLABLE = 'YES' AS "nullable"`),
			always(`, c.COLUMN_DEFAULT AS "default"`),
			always(`, k.COLUMN_NAME IS NOT NULL AS "primary_key"`),
			always(`, RTRIM(CASE WHEN c.IS_IDENTITY = 'YES' THEN LOWER(c.IDENTITY_GENERATION) ELSE '' END) AS "identity"`),
			always(`, RTRIM(CASE WHEN c.IS_GENERATED = 'ALWAYS' THEN 'stored' ELSE '' END) AS "generated"`),
			always(`, m.COMMENT AS "comment"`),
			always(`, c.COLLATION_NAME AS "collation"`),
			always(`FROM INFORMATION_SCHEMA.COLUMNS c`),
			always(`LEFT JOIN INFORMATION_SCHEMA.SYSTEM_PRIMARYKEYS k ON k.TABLE_SCHEM = c.TABLE_SCHEMA` +
				` AND k.TABLE_NAME = c.TABLE_NAME AND k.COLUMN_NAME = c.COLUMN_NAME`),
			always(`LEFT JOIN (SELECT OBJECT_SCHEMA, OBJECT_NAME, COLUMN_NAME, COMMENT FROM INFORMATION_SCHEMA.SYSTEM_COMMENTS` +
				` WHERE OBJECT_TYPE = 'COLUMN') m ON m.OBJECT_SCHEMA = c.TABLE_SCHEMA` +
				` AND m.OBJECT_NAME = c.TABLE_NAME AND m.COLUMN_NAME = c.COLUMN_NAME`),
			always(`WHERE ` + notSystem("c.TABLE_SCHEMA")),
			always(`AND ` + like("c.TABLE_SCHEMA", "@schema")),
			always(`AND ` + like("c.TABLE_NAME", "@parent")),
			always(`AND ` + like("c.COLUMN_NAME", "@name")),
			always(`ORDER BY 2, 3, 5`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: catalogDesc},
			{Name: "schema"}, {Name: "table"}, {Name: "name"},
			{Name: "ordinal", Desc: "from ORDINAL_POSITION, which counts from one"},
			{Name: "data_type", Desc: "the type with its length, or its precision and scale, assembled from the columns that COLUMNS keeps them in. The name of the domain where the column has one, and of the user defined type where it has one"},
			{Name: "nullable", Desc: "from IS_NULLABLE. HSQLDB writes a NOT NULL as a check constraint named SYS_CT and a number, which Constraints lists"},
			{Name: "default", Desc: "the expression, which keeps its quotes, as in 'plain'"},
			{Name: "primary_key", Desc: "true for a column of the primary key, which SYSTEM_PRIMARYKEYS lists"},
			{Name: "identity", Desc: "always or by default for an identity column, and empty for any other"},
			{Name: "generated", Desc: "stored for a GENERATED ALWAYS AS column, and empty for any other. HSQLDB computes the value when it writes the row"},
			{Name: "comment", Desc: "from SYSTEM_COMMENTS, which COMMENT ON ... COLUMN writes"},
			{Name: "collation", Desc: "SQL_TEXT for a character column, which is the default collation of the database, and absent for any other"},
		},
		Params: parentAndName("column"),
		Scan: func(rows *sql.Rows) (dbmeta.Column, error) {
			var v dbmeta.Column
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Table, &v.Name, &v.Ordinal, &v.DataType,
				&v.Nullable, &v.Default, &v.PrimaryKey, &v.Identity, &v.Generated, &v.Comment, &v.Collation)
			return v, err
		},
	})
}

// primaryIndex is true when the index is the one a primary key constraint
// owns. SYSTEM_KEY_INDEX_USAGE names the index each constraint made, and
// SYSTEM_PRIMARYKEYS names the constraint of each primary key. A join with
// TABLE_CONSTRAINTS in place of SYSTEM_PRIMARYKEYS took 1.2 seconds over 5,100
// indexes, against 0.025 (D186).
const primaryIndex = `EXISTS (SELECT 1 FROM INFORMATION_SCHEMA.SYSTEM_KEY_INDEX_USAGE u` +
	` JOIN INFORMATION_SCHEMA.SYSTEM_PRIMARYKEYS k ON k.TABLE_SCHEM = i.TABLE_SCHEM` +
	` AND k.TABLE_NAME = i.TABLE_NAME AND k.PK_NAME = u.CONSTRAINT_NAME AND k.KEY_SEQ = 1` +
	` WHERE u.INDEX_SCHEMA = i.TABLE_SCHEM AND u.INDEX_NAME = i.INDEX_NAME)`

func registerIndexes() {
	// SYSTEM_INDEXINFO has one row for each column of an index, so the index
	// is the row with ORDINAL_POSITION 1.
	dbmeta.Indexes.Register(dbmeta.Avatica, &dbmeta.Binding[dbmeta.Index]{
		Stmt: dbmeta.Stmt{
			always(`SELECT i.TABLE_CAT AS "catalog"`),
			always(`, i.TABLE_SCHEM AS "schema"`),
			always(`, i.TABLE_NAME AS "table"`),
			always(`, i.INDEX_NAME AS "name"`),
			always(`, 'tree' AS "type"`),
			always(`, NOT i.NON_UNIQUE AS "unique"`),
			always(`, ` + primaryIndex + ` AS "primary"`),
			always(`, ` + text + ` AS "comment"`),
			always(`FROM INFORMATION_SCHEMA.SYSTEM_INDEXINFO i`),
			always(`WHERE i.ORDINAL_POSITION = 1`),
			always(`AND ` + notSystem("i.TABLE_SCHEM")),
			always(`AND ` + like("i.TABLE_SCHEM", "@schema")),
			always(`AND ` + like("i.TABLE_NAME", "@parent")),
			always(`AND ` + like("i.INDEX_NAME", "@name")),
			always(`ORDER BY 2, 3, 4`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: catalogDesc},
			{Name: "schema"}, {Name: "table"},
			{Name: "name", Desc: "a constraint makes an index named SYS_IDX_, the constraint name and a number, and the number changes between runs"},
			{Name: "type", Desc: "always tree: HSQLDB has one kind of index, and SYSTEM_INDEXINFO reports it as the JDBC kind tableIndexOther"},
			{Name: "unique", Desc: "from NON_UNIQUE, inverted"},
			{Name: "primary", Desc: "true when a PRIMARY KEY constraint made this index"},
			{Name: "comment", Desc: "always absent: COMMENT ON takes no index in HSQLDB"},
		},
		Params: parentAndName("index"),
		Scan: func(rows *sql.Rows) (dbmeta.Index, error) {
			var v dbmeta.Index
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Table, &v.Name, &v.Type, &v.Unique, &v.Primary, &v.Comment)
			return v, err
		},
	})

	dbmeta.IndexColumns.Register(dbmeta.Avatica, &dbmeta.Binding[dbmeta.IndexColumn]{
		Stmt: dbmeta.Stmt{
			always(`SELECT i.TABLE_SCHEM AS "schema"`),
			always(`, i.TABLE_NAME AS "table"`),
			always(`, i.INDEX_NAME AS "index"`),
			always(`, i.COLUMN_NAME AS "name"`),
			always(`, CAST(i.ORDINAL_POSITION AS BIGINT) AS "ordinal"`),
			always(`, ` + text + ` AS "expression"`),
			always(`, i.ASC_OR_DESC = 'D' AS "descending"`),
			always(`FROM INFORMATION_SCHEMA.SYSTEM_INDEXINFO i`),
			always(`WHERE ` + notSystem("i.TABLE_SCHEM")),
			always(`AND ` + like("i.TABLE_SCHEM", "@schema")),
			always(`AND ` + like("i.TABLE_NAME", "@parent")),
			always(`AND ` + like("i.INDEX_NAME", "@name")),
			always(`ORDER BY 1, 2, 3, 5`),
		},
		Fields: []dbmeta.Field{
			{Name: "schema"}, {Name: "table"}, {Name: "index"},
			{Name: "name", Desc: "the column. It is never absent, because HSQLDB has no expression index"},
			{Name: "ordinal", Desc: "from ORDINAL_POSITION, which counts from one"},
			{Name: "expression", Desc: "always absent: HSQLDB indexes columns and has no expression index"},
			{Name: "descending", Desc: "true when ASC_OR_DESC is D"},
		},
		Params: parentAndName("index"),
		Scan: func(rows *sql.Rows) (dbmeta.IndexColumn, error) {
			var v dbmeta.IndexColumn
			err := rows.Scan(&v.Schema, &v.Table, &v.Index, &v.Name, &v.Ordinal, &v.Expression, &v.Descending)
			return v, err
		},
	})
}

func registerConstraints() {
	// TABLE_CONSTRAINTS names every constraint of a table, and CHECK_CONSTRAINTS
	// holds the text of a check. A NOT NULL is a check named SYS_CT and a number.
	dbmeta.Constraints.Register(dbmeta.Avatica, &dbmeta.Binding[dbmeta.Constraint]{
		Stmt: dbmeta.Stmt{
			always(`SELECT k.CONSTRAINT_SCHEMA AS "schema"`),
			always(`, k.TABLE_NAME AS "table"`),
			always(`, k.CONSTRAINT_NAME AS "name"`),
			always(`, LOWER(k.CONSTRAINT_TYPE) AS "type"`),
			always(`, c.CHECK_CLAUSE AS "definition"`),
			always(`, k.IS_DEFERRABLE = 'YES' AS "deferrable"`),
			always(`, k.INITIALLY_DEFERRED = 'YES' AS "deferred"`),
			always(`, ` + text + ` AS "comment"`),
			always(`FROM INFORMATION_SCHEMA.TABLE_CONSTRAINTS k`),
			always(`LEFT JOIN INFORMATION_SCHEMA.CHECK_CONSTRAINTS c ON c.CONSTRAINT_SCHEMA = k.CONSTRAINT_SCHEMA` +
				` AND c.CONSTRAINT_NAME = k.CONSTRAINT_NAME AND k.CONSTRAINT_TYPE = 'CHECK'`),
			always(`WHERE ` + notSystem("k.CONSTRAINT_SCHEMA")),
			always(`AND ` + like("k.CONSTRAINT_SCHEMA", "@schema")),
			always(`AND ` + like("k.TABLE_NAME", "@parent")),
			always(`AND ` + like("k.CONSTRAINT_NAME", "@name")),
			always(`ORDER BY 1, 2, 3`),
		},
		Fields: []dbmeta.Field{
			{Name: "schema"}, {Name: "table"},
			{Name: "name", Desc: "a constraint with no name gets SYS_CT and a number, and so does every NOT NULL"},
			{Name: "type", Desc: "primary key, foreign key, unique or check. A NOT NULL is a check, and HSQLDB has no other kind"},
			{Name: "definition", Desc: "the CHECK text for a check, and absent for every other kind. ConstraintColumns lists the columns of a key"},
			{Name: "deferrable", Desc: "from IS_DEFERRABLE, which is always NO: HSQLDB defers no constraint"},
			{Name: "deferred", Desc: "from INITIALLY_DEFERRED, which is always NO"},
			{Name: "comment", Desc: "always absent: COMMENT ON takes no constraint in HSQLDB"},
		},
		Params: parentAndName("constraint"),
		Scan: func(rows *sql.Rows) (dbmeta.Constraint, error) {
			var v dbmeta.Constraint
			err := rows.Scan(&v.Schema, &v.Table, &v.Name, &v.Type, &v.Definition,
				&v.Deferrable, &v.Deferred, &v.Comment)
			return v, err
		},
	})

	// KEY_COLUMN_USAGE lists the columns of a key. A foreign key reaches the
	// columns it points at through REFERENTIAL_CONSTRAINTS, which names the key
	// at the other end, and POSITION_IN_UNIQUE_CONSTRAINT, which names the
	// column of it.
	dbmeta.ConstraintColumns.Register(dbmeta.Avatica, &dbmeta.Binding[dbmeta.ConstraintColumn]{
		Stmt: dbmeta.Stmt{
			always(`SELECT k.CONSTRAINT_CATALOG AS "catalog"`),
			always(`, k.CONSTRAINT_SCHEMA AS "schema"`),
			always(`, k.TABLE_NAME AS "table"`),
			always(`, k.CONSTRAINT_NAME AS "constraint"`),
			always(`, k.COLUMN_NAME AS "name"`),
			always(`, k.ORDINAL_POSITION AS "ordinal"`),
			always(`, f.CONSTRAINT_CATALOG AS "foreign_catalog"`),
			always(`, f.TABLE_SCHEMA AS "foreign_schema"`),
			always(`, f.TABLE_NAME AS "foreign_table"`),
			always(`, f.COLUMN_NAME AS "foreign_name"`),
			always(`FROM INFORMATION_SCHEMA.KEY_COLUMN_USAGE k`),
			always(`LEFT JOIN INFORMATION_SCHEMA.REFERENTIAL_CONSTRAINTS r ON r.CONSTRAINT_SCHEMA = k.CONSTRAINT_SCHEMA` +
				` AND r.CONSTRAINT_NAME = k.CONSTRAINT_NAME`),
			always(`LEFT JOIN INFORMATION_SCHEMA.KEY_COLUMN_USAGE f ON f.CONSTRAINT_SCHEMA = r.UNIQUE_CONSTRAINT_SCHEMA` +
				` AND f.CONSTRAINT_NAME = r.UNIQUE_CONSTRAINT_NAME AND f.ORDINAL_POSITION = k.POSITION_IN_UNIQUE_CONSTRAINT`),
			always(`WHERE ` + notSystem("k.CONSTRAINT_SCHEMA")),
			always(`AND ` + like("k.CONSTRAINT_SCHEMA", "@schema")),
			always(`AND ` + like("k.TABLE_NAME", "@parent")),
			always(`AND ` + like("k.CONSTRAINT_NAME", "@name")),
			always(`ORDER BY 2, 3, 4, 6`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: catalogDesc},
			{Name: "schema"}, {Name: "table"}, {Name: "constraint"}, {Name: "name"},
			{Name: "ordinal", Desc: "from ORDINAL_POSITION, which counts from one. A check has no row here, because KEY_COLUMN_USAGE lists keys only"},
			{Name: "foreign_catalog", Desc: "PUBLIC for a foreign key and absent for a primary key and a unique constraint"},
			{Name: "foreign_schema", Desc: "absent unless this is a foreign key"},
			{Name: "foreign_table", Desc: "absent unless this is a foreign key"},
			{Name: "foreign_name", Desc: "absent unless this is a foreign key"},
		},
		Params: parentAndName("constraint"),
		Scan: func(rows *sql.Rows) (dbmeta.ConstraintColumn, error) {
			var v dbmeta.ConstraintColumn
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Table, &v.Constraint, &v.Name, &v.Ordinal,
				&v.ForeignCatalog, &v.ForeignSchema, &v.ForeignTable, &v.ForeignName)
			return v, err
		},
	})
}

func registerTriggers() {
	// TRIGGERS has one row for each event a trigger fires on. HSQLDB keeps the
	// body and not the whole statement, so the definition is put together from
	// the parts.
	dbmeta.Triggers.Register(dbmeta.Avatica, &dbmeta.Binding[dbmeta.Trigger]{
		Stmt: dbmeta.Stmt{
			always(`SELECT t.EVENT_OBJECT_SCHEMA AS "schema"`),
			always(`, t.EVENT_OBJECT_TABLE AS "table"`),
			always(`, t.TRIGGER_NAME AS "name"`),
			always(`, 'enabled' AS "enabled"`),
			always(`, 'CREATE TRIGGER ' || t.TRIGGER_NAME || ' ' || t.ACTION_TIMING || ' ' || t.EVENT_MANIPULATION` +
				` || ' ON ' || t.EVENT_OBJECT_SCHEMA || '.' || t.EVENT_OBJECT_TABLE` +
				` || ' FOR EACH ' || t.ACTION_ORIENTATION || ' ' || t.ACTION_STATEMENT AS "definition"`),
			always(`, ` + text + ` AS "comment"`),
			always(`FROM INFORMATION_SCHEMA.TRIGGERS t`),
			always(`WHERE ` + notSystem("t.EVENT_OBJECT_SCHEMA")),
			always(`AND ` + like("t.EVENT_OBJECT_SCHEMA", "@schema")),
			always(`AND ` + like("t.EVENT_OBJECT_TABLE", "@parent")),
			always(`AND ` + like("t.TRIGGER_NAME", "@name")),
			always(`ORDER BY 1, 2, 3`),
		},
		Fields: []dbmeta.Field{
			{Name: "schema", Desc: "the schema of the table the trigger is on. A trigger has a schema of its own, and the table's is what a caller groups by"},
			{Name: "table"}, {Name: "name"},
			{Name: "enabled", Desc: "always enabled: HSQLDB has no statement that disables a trigger"},
			{Name: "definition", Desc: "CREATE TRIGGER, the name, the timing, the event, the table and the orientation, then the body from ACTION_STATEMENT. It is put together here, because the catalog keeps the parts. A trigger on two events has one row for each"},
			{Name: "comment", Desc: "always absent: COMMENT ON takes no trigger in HSQLDB"},
		},
		Params: parentAndName("trigger"),
		Scan: func(rows *sql.Rows) (dbmeta.Trigger, error) {
			var v dbmeta.Trigger
			err := rows.Scan(&v.Schema, &v.Table, &v.Name, &v.Enabled, &v.Definition, &v.Comment)
			return v, err
		},
	})
}
