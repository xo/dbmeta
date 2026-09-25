package oracle

import (
	"database/sql"

	"github.com/xo/dbmeta"
)

func registerExtra() {
	// \d NAME, the constraint section.
	//
	// Oracle names the kinds with one letter and the letters are its own, so
	// they are translated to the words psql uses. R is a foreign key because
	// Oracle calls it a referential constraint, and C covers both a check and
	// a NOT NULL, which D49 says is not a constraint row, so the NOT NULL
	// ones are filtered out by their generated search condition.
	dbmeta.Constraints.Register(dbmeta.Oracle, &dbmeta.Binding[dbmeta.Constraint]{
		Stmt: dbmeta.Stmt{
			always(`SELECT c.owner AS "schema"`),
			always(`, c.table_name AS "table"`),
			always(`, c.constraint_name AS "name"`),
			always(`, CASE c.constraint_type`),
			always(`    WHEN 'P' THEN 'primary key' WHEN 'U' THEN 'unique'`),
			always(`    WHEN 'R' THEN 'foreign key' WHEN 'C' THEN 'check'`),
			always(`    WHEN 'V' THEN 'check option' WHEN 'O' THEN 'read only'`),
			always(`    ELSE LOWER(c.constraint_type) END AS "type"`),
			// search_condition is a LONG. search_condition_vc is the same
			// thing as a VARCHAR2 and arrived in 12c, so an older release
			// reads the LONG and a newer one reads the string.
			dbmeta.Choice{
				{SQL: `, c.search_condition AS "definition"`},
				{Min: v12, SQL: `, c.search_condition_vc AS "definition"`},
			},
			always(`, CASE c.deferrable WHEN 'DEFERRABLE' THEN 1 ELSE 0 END AS "deferrable"`),
			always(`, CASE c.deferred WHEN 'DEFERRED' THEN 1 ELSE 0 END AS "deferred"`),
			always(`, NULL AS "comment"`),
			always(`FROM all_constraints c`),
			always(`WHERE ` + notSystem("c.owner")),
			// A NOT NULL is a check constraint in this dictionary and is not
			// a constraint row here. See D49.
			always(`AND NOT (c.constraint_type = 'C' AND c.generated = 'GENERATED NAME'`),
			always(`  AND c.constraint_name LIKE 'SYS_C%')`),
			always(`AND (@schema IS NULL OR c.owner LIKE @schema)`),
			always(`AND (@name IS NULL OR c.table_name LIKE @name)`),
			always(`ORDER BY c.owner, c.table_name, c.constraint_name`),
		},
		Fields: []dbmeta.Field{
			{Name: "schema"}, {Name: "table"}, {Name: "name"},
			{Name: "type", Desc: "the psql word for Oracle's one letter kind"},
			{Name: "definition", Desc: "the check condition, absent for a key"},
			{Name: "deferrable"}, {Name: "deferred"},
			{Name: "comment", Desc: "always absent: Oracle records no comment on a constraint"},
		},
		Params: schemaNameSystem("table"),
		Scan: func(rows *sql.Rows) (dbmeta.Constraint, error) {
			var v dbmeta.Constraint
			err := rows.Scan(&v.Schema, &v.Table, &v.Name, &v.Type, &v.Definition,
				&v.Deferrable, &v.Deferred, &v.Comment)
			return v, err
		},
	})

	// The columns of each constraint, in order, with what a foreign key
	// points at. D46 added this kind because both consumers need it.
	dbmeta.ConstraintColumns.Register(dbmeta.Oracle, &dbmeta.Binding[dbmeta.ConstraintColumn]{
		Stmt: dbmeta.Stmt{
			always(`SELECT SYS_CONTEXT('USERENV', 'DB_NAME') AS "catalog"`),
			always(`, cc.owner AS "schema"`),
			always(`, cc.table_name AS "table"`),
			always(`, cc.constraint_name AS "constraint"`),
			always(`, cc.column_name AS "name"`),
			always(`, cc.position AS "ordinal"`),
			always(`, CASE WHEN r.owner IS NULL THEN NULL`),
			always(`    ELSE SYS_CONTEXT('USERENV', 'DB_NAME') END AS "foreign_catalog"`),
			always(`, r.owner AS "foreign_schema"`),
			always(`, r.table_name AS "foreign_table"`),
			always(`, rc.column_name AS "foreign_name"`),
			always(`FROM all_cons_columns cc`),
			always(`JOIN all_constraints c`),
			always(`  ON c.owner = cc.owner AND c.constraint_name = cc.constraint_name`),
			// The constraint a foreign key points at, and the column of it in
			// the same position.
			always(`LEFT JOIN all_constraints r`),
			always(`  ON r.owner = c.r_owner AND r.constraint_name = c.r_constraint_name`),
			always(`LEFT JOIN all_cons_columns rc`),
			always(`  ON rc.owner = r.owner AND rc.constraint_name = r.constraint_name`),
			always(`  AND rc.position = cc.position`),
			always(`WHERE ` + notSystem("cc.owner")),
			always(`AND c.constraint_type IN ('P', 'U', 'R')`),
			always(`AND (@schema IS NULL OR cc.owner LIKE @schema)`),
			always(`AND (@name IS NULL OR cc.table_name LIKE @name)`),
			always(`ORDER BY cc.owner, cc.table_name, cc.constraint_name, cc.position`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog"}, {Name: "schema"}, {Name: "table"},
			{Name: "constraint"}, {Name: "name"}, {Name: "ordinal"},
			{Name: "foreign_catalog"}, {Name: "foreign_schema"},
			{Name: "foreign_table"}, {Name: "foreign_name"},
		},
		Params: schemaNameSystem("table"),
		Scan: func(rows *sql.Rows) (dbmeta.ConstraintColumn, error) {
			var v dbmeta.ConstraintColumn
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Table, &v.Constraint,
				&v.Name, &v.Ordinal, &v.ForeignCatalog, &v.ForeignSchema,
				&v.ForeignTable, &v.ForeignName)
			return v, err
		},
	})

	// \di.
	dbmeta.Indexes.Register(dbmeta.Oracle, &dbmeta.Binding[dbmeta.Index]{
		Stmt: dbmeta.Stmt{
			always(`SELECT SYS_CONTEXT('USERENV', 'DB_NAME') AS "catalog"`),
			always(`, i.owner AS "schema"`),
			always(`, i.table_name AS "table"`),
			always(`, i.index_name AS "name"`),
			always(`, LOWER(i.index_type) AS "type"`),
			always(`, CASE i.uniqueness WHEN 'UNIQUE' THEN 1 ELSE 0 END AS "unique"`),
			// Oracle has no flag for this. An index is the primary key's when
			// a primary key constraint names it.
			always(`, CASE WHEN c.constraint_name IS NULL THEN 0 ELSE 1 END AS "primary"`),
			always(`, NULL AS "comment"`),
			always(`FROM all_indexes i`),
			always(`LEFT JOIN all_constraints c`),
			always(`  ON c.owner = i.owner AND c.index_name = i.index_name`),
			always(`  AND c.constraint_type = 'P'`),
			always(`WHERE ` + notSystem("i.owner")),
			always(`AND (@schema IS NULL OR i.owner LIKE @schema)`),
			always(`AND (@name IS NULL OR i.table_name LIKE @name)`),
			always(`ORDER BY i.owner, i.table_name, i.index_name`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog"}, {Name: "schema"}, {Name: "table"}, {Name: "name"},
			{Name: "type", Desc: "normal, bitmap, iot - top or a domain index"},
			{Name: "unique"},
			{Name: "primary", Desc: "true when a primary key constraint uses this index"},
			{Name: "comment", Desc: "always absent: Oracle records no comment on an index"},
		},
		Params: schemaNameSystem("table"),
		Scan: func(rows *sql.Rows) (dbmeta.Index, error) {
			var v dbmeta.Index
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Table, &v.Name, &v.Type,
				&v.Unique, &v.Primary, &v.Comment)
			return v, err
		},
	})

	// The columns of each index, in order.
	dbmeta.IndexColumns.Register(dbmeta.Oracle, &dbmeta.Binding[dbmeta.IndexColumn]{
		Stmt: dbmeta.Stmt{
			always(`SELECT ic.index_owner AS "schema"`),
			always(`, ic.table_name AS "table"`),
			always(`, ic.index_name AS "index"`),
			always(`, ic.column_name AS "name"`),
			always(`, ic.column_position AS "ordinal"`),
			// A function based index stores its expression separately and
			// puts a generated name in column_name.
			always(`, e.column_expression AS "expression"`),
			always(`, CASE ic.descend WHEN 'DESC' THEN 1 ELSE 0 END AS "descending"`),
			always(`FROM all_ind_columns ic`),
			always(`LEFT JOIN all_ind_expressions e`),
			always(`  ON e.index_owner = ic.index_owner AND e.index_name = ic.index_name`),
			always(`  AND e.column_position = ic.column_position`),
			always(`WHERE ` + notSystem("ic.index_owner")),
			always(`AND (@schema IS NULL OR ic.index_owner LIKE @schema)`),
			always(`AND (@name IS NULL OR ic.table_name LIKE @name)`),
			always(`ORDER BY ic.index_owner, ic.index_name, ic.column_position`),
		},
		Fields: []dbmeta.Field{
			{Name: "schema"}, {Name: "table"}, {Name: "index"}, {Name: "name"},
			{Name: "ordinal"},
			{Name: "expression", Desc: "set for a function based index"},
			{Name: "descending"},
		},
		Params: schemaNameSystem("table"),
		Scan: func(rows *sql.Rows) (dbmeta.IndexColumn, error) {
			var v dbmeta.IndexColumn
			err := rows.Scan(&v.Schema, &v.Table, &v.Index, &v.Name, &v.Ordinal,
				&v.Expression, &v.Descending)
			return v, err
		},
	})

	// \ds.
	dbmeta.Sequences.Register(dbmeta.Oracle, &dbmeta.Binding[dbmeta.Sequence]{
		Stmt: dbmeta.Stmt{
			always(`SELECT s.sequence_owner AS "schema"`),
			always(`, s.sequence_name AS "name"`),
			always(`, 'NUMBER' AS "data_type"`),
			always(`, s.min_value AS "start"`),
			always(`, s.min_value AS "minimum"`),
			always(`, s.max_value AS "maximum"`),
			always(`, s.increment_by AS "increment"`),
			always(`, CASE s.cycle_flag WHEN 'Y' THEN 1 ELSE 0 END AS "cycles"`),
			always(`, '' AS "owned_by"`),
			always(`, NULL AS "comment"`),
			always(`FROM all_sequences s`),
			always(`WHERE ` + notSystem("s.sequence_owner")),
			always(`AND (@schema IS NULL OR s.sequence_owner LIKE @schema)`),
			always(`AND (@name IS NULL OR s.sequence_name LIKE @name)`),
			always(`ORDER BY s.sequence_owner, s.sequence_name`),
		},
		Fields: []dbmeta.Field{
			{Name: "schema"}, {Name: "name"},
			{Name: "data_type", Desc: "always NUMBER: an Oracle sequence has no declared type"},
			{
				Name: "start",
				Desc: "the minimum, which is what Oracle keeps: it does not record the START WITH",
			},
			{Name: "minimum"}, {Name: "maximum"}, {Name: "increment"}, {Name: "cycles"},
			{Name: "owned_by", Desc: "always empty: an Oracle sequence is independent of any column"},
			{Name: "comment", Desc: "always absent: Oracle records no comment on a sequence"},
		},
		Params: schemaNameSystem("sequence"),
		Scan: func(rows *sql.Rows) (dbmeta.Sequence, error) {
			var v dbmeta.Sequence
			err := rows.Scan(&v.Schema, &v.Name, &v.DataType, &v.Start, &v.Minimum,
				&v.Maximum, &v.Increment, &v.Cycles, &v.OwnedBy, &v.Comment)
			return v, err
		},
	})

	// The statement a view selects. D46 added this kind.
	dbmeta.Views.Register(dbmeta.Oracle, &dbmeta.Binding[dbmeta.View]{
		Stmt: dbmeta.Stmt{
			always(`SELECT SYS_CONTEXT('USERENV', 'DB_NAME') AS "catalog"`),
			always(`, v.owner AS "schema"`),
			always(`, v.view_name AS "name"`),
			// text is a LONG. text_vc is the same as a VARCHAR2 and arrived
			// in 18c, so an older release reads the LONG.
			dbmeta.Choice{
				{SQL: `, v.text AS "definition"`},
				{Min: v18, SQL: `, v.text_vc AS "definition"`},
			},
			always(`, NULL AS "check_option"`),
			always(`, NULL AS "updatable"`),
			always(`, NULL AS "insertable"`),
			always(`, c.comments AS "comment"`),
			always(`FROM all_views v`),
			always(`LEFT JOIN all_tab_comments c`),
			always(`  ON c.owner = v.owner AND c.table_name = v.view_name`),
			always(`WHERE ` + notSystem("v.owner")),
			always(`AND (@schema IS NULL OR v.owner LIKE @schema)`),
			always(`AND (@name IS NULL OR v.view_name LIKE @name)`),
			always(`ORDER BY v.owner, v.view_name`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog"}, {Name: "schema"}, {Name: "name"},
			{Name: "definition", Desc: "the SELECT, stored as a LONG before 18c"},
			{
				Name: "check_option",
				Desc: "always absent: Oracle records WITH CHECK OPTION as a constraint rather than here",
			},
			{Name: "updatable", Desc: "always absent: Oracle decides this per statement"},
			{Name: "insertable", Desc: "always absent, for the same reason"},
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

	// The schema an unqualified name resolves in.
	dbmeta.CurrentSchema.Register(dbmeta.Oracle, &dbmeta.Binding[dbmeta.Schema]{
		Stmt: dbmeta.Stmt{
			always(`SELECT SYS_CONTEXT('USERENV', 'DB_NAME') AS "catalog"`),
			always(`, SYS_CONTEXT('USERENV', 'CURRENT_SCHEMA') AS "name"`),
			always(`, SYS_CONTEXT('USERENV', 'CURRENT_SCHEMA') AS "owner"`),
			always(`, NULL AS "comment"`),
			always(`FROM dual`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog"},
			{Name: "name", Desc: "what ALTER SESSION SET CURRENT_SCHEMA last set, or the user"},
			{Name: "owner", Desc: "the same as the name: an Oracle schema is a user"},
			{Name: "comment", Desc: "always absent"},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Schema, error) {
			var v dbmeta.Schema
			err := rows.Scan(&v.Catalog, &v.Name, &v.Owner, &v.Comment)
			return v, err
		},
	})

	// Who the connection authenticated as. D55 added this kind.
	dbmeta.CurrentUser.Register(dbmeta.Oracle, &dbmeta.Binding[dbmeta.User]{
		Stmt: dbmeta.Stmt{
			// CURRENT_SCHEMA is the effective one and SESSION_USER is who
			// logged in. ALTER SESSION SET CURRENT_SCHEMA moves the first and
			// leaves the second, which is the distinction D55 reports.
			always(`SELECT SYS_CONTEXT('USERENV', 'CURRENT_USER') AS "name"`),
			always(`, SYS_CONTEXT('USERENV', 'SESSION_USER') AS "session"`),
			always(`FROM dual`),
		},
		Fields: []dbmeta.Field{
			{Name: "name", Desc: "the effective user"},
			{Name: "session", Desc: "the user the connection authenticated as"},
		},
		Scan: func(rows *sql.Rows) (dbmeta.User, error) {
			var v dbmeta.User
			err := rows.Scan(&v.Name, &v.Session)
			return v, err
		},
	})
}
