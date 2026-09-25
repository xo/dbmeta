package oracle

import (
	"database/sql"

	"github.com/xo/dbmeta"
)

func registerTables() {
	// The comments on a table and on a column, in one list.
	//
	// Oracle keeps them in two views with the same shape, and a comment is
	// one kind rather than two, so the two halves are unioned. A column is
	// named table.column, which is how a person writes it and how COMMENT ON
	// COLUMN takes it.
	dbmeta.Comments.Register(dbmeta.Oracle, &dbmeta.Binding[dbmeta.Comment]{
		Stmt: dbmeta.Stmt{
			always(`SELECT t.owner AS "schema"`),
			always(`, t.table_name AS "name"`),
			always(`, LOWER(t.table_type) AS "type"`),
			always(`, t.comments AS "comment"`),
			always(`FROM all_tab_comments t`),
			always(`WHERE t.comments IS NOT NULL`),
			notSystem("AND", "t.owner"),
			always(`AND (@schema IS NULL OR t.owner LIKE @schema)`),
			always(`AND (@name IS NULL OR t.table_name LIKE @name)`),
			always(`UNION ALL`),
			always(`SELECT c.owner`),
			always(`, c.table_name || '.' || c.column_name`),
			always(`, 'column'`),
			always(`, c.comments`),
			always(`FROM all_col_comments c`),
			always(`WHERE c.comments IS NOT NULL`),
			notSystem("AND", "c.owner"),
			always(`AND (@schema IS NULL OR c.owner LIKE @schema)`),
			always(`AND (@name IS NULL OR c.table_name LIKE @name)`),
			always(`ORDER BY 1, 3, 2`),
		},
		Fields: []dbmeta.Field{
			{Name: "schema"},
			{Name: "name", Desc: "the object name, or table.column for a column comment"},
			{Name: "type"}, {Name: "comment"},
		},
		Params: schemaNameSystem("table"),
		Scan: func(rows *sql.Rows) (dbmeta.Comment, error) {
			var v dbmeta.Comment
			err := rows.Scan(&v.Schema, &v.Name, &v.Type, &v.Comment)
			return v, err
		},
	})

	// The triggers on a table or a view.
	dbmeta.Triggers.Register(dbmeta.Oracle, &dbmeta.Binding[dbmeta.Trigger]{
		Stmt: dbmeta.Stmt{
			always(`SELECT t.table_owner AS "schema"`),
			always(`, t.table_name AS "table"`),
			always(`, t.trigger_name AS "name"`),
			always(`, t.status AS "enabled"`),
			// description holds the header Oracle rebuilds from the
			// dictionary: the name, the timing, the event and the WHEN
			// clause. The body is a LONG in the same view, and a LONG cannot
			// be concatenated onto it. psql prints the same header, because
			// pg_get_triggerdef stops at the function call too.
			always(`, NVL(t.description, '') AS "definition"`),
			always(`, NULL AS "comment"`),
			always(`FROM all_triggers t`),
			always(`WHERE t.base_object_type IN ('TABLE', 'VIEW')`),
			notSystem("AND", "t.table_owner"),
			always(`AND (@schema IS NULL OR t.table_owner LIKE @schema)`),
			always(`AND (@name IS NULL OR t.table_name LIKE @name)`),
			always(`ORDER BY t.table_owner, t.table_name, t.trigger_name`),
		},
		Fields: []dbmeta.Field{
			{Name: "schema"}, {Name: "table"}, {Name: "name"}, {Name: "enabled"},
			{
				Name: "definition",
				Desc: "the CREATE TRIGGER header without the body, which Oracle" +
					" keeps as a LONG in the same row",
			},
			{Name: "comment", Desc: "always absent: Oracle records no comment on a trigger"},
		},
		Params: schemaNameSystem("table"),
		Scan: func(rows *sql.Rows) (dbmeta.Trigger, error) {
			var v dbmeta.Trigger
			err := rows.Scan(&v.Schema, &v.Table, &v.Name, &v.Enabled,
				&v.Definition, &v.Comment)
			return v, err
		},
	})

	// \dy. The triggers that fire on a schema or on the database rather than
	// on a table. Oracle keeps them in the same view as a table trigger and
	// base_object_type is what separates them.
	dbmeta.EventTriggers.Register(dbmeta.Oracle, &dbmeta.Binding[dbmeta.EventTrigger]{
		Stmt: dbmeta.Stmt{
			always(`SELECT t.trigger_name AS "name"`),
			always(`, LOWER(t.base_object_type) AS "event"`),
			always(`, t.owner AS "owner"`),
			always(`, t.status AS "enabled"`),
			always(`, '' AS "function"`),
			always(`, t.triggering_event AS "tags"`),
			always(`, NULL AS "comment"`),
			always(`FROM all_triggers t`),
			always(`WHERE t.base_object_type IN ('SCHEMA', 'DATABASE')`),
			notSystem("AND", "t.owner"),
			always(`AND (@name IS NULL OR t.trigger_name LIKE @name)`),
			always(`ORDER BY t.owner, t.trigger_name`),
		},
		Fields: []dbmeta.Field{
			{Name: "name"},
			{
				Name: "event",
				Desc: "where the trigger fires, schema or database: Oracle names" +
					" the scope where PostgreSQL names a point in the command",
			},
			{Name: "owner"}, {Name: "enabled"},
			{
				Name: "function",
				Desc: "always empty: an Oracle trigger holds its body inline" +
					" rather than calling a function",
			},
			{
				Name: "tags",
				Desc: "the triggering event, such as CREATE OR ALTER, which is" +
					" the list of commands the trigger fires on",
			},
			{Name: "comment", Desc: "always absent: Oracle records no comment on a trigger"},
		},
		Params: []dbmeta.Param{
			{Name: "name", Desc: "trigger name pattern, empty for every one", Default: ""},
			{Name: "with_system", Desc: "include the schemas Oracle keeps for itself", Default: false},
		},
		Scan: func(rows *sql.Rows) (dbmeta.EventTrigger, error) {
			var v dbmeta.EventTrigger
			err := rows.Scan(&v.Name, &v.Event, &v.Owner, &v.Enabled, &v.Function,
				&v.Tags, &v.Comment)
			return v, err
		},
	})

	// \dP. The partitioned tables, with the key the partitioning is by.
	//
	// Express and Free have no partitioning option, so the view is there and
	// answers no rows. That is the right answer rather than a failure, and it
	// is why the fixture creates no partitioned table.
	dbmeta.PartitionedTables.Register(dbmeta.Oracle,
		&dbmeta.Binding[dbmeta.PartitionedTable]{
			Stmt: dbmeta.Stmt{
				always(`SELECT p.owner AS "schema"`),
				always(`, p.table_name AS "name"`),
				always(`, p.owner AS "owner"`),
				always(`, 'table' AS "type"`),
				always(`, '' AS "parent"`),
				always(`, p.partitioning_type AS "strategy"`),
				always(`, NVL((SELECT `),
				listagg("k.column_name", "k.column_position"),
				always(`  FROM all_part_key_columns k`),
				always(`  WHERE k.owner = p.owner AND k.name = p.table_name`),
				always(`  AND k.object_type = 'TABLE'), '') AS "expression"`),
				always(`, m.comments AS "comment"`),
				always(`FROM all_part_tables p`),
				always(`LEFT JOIN all_tab_comments m ON m.owner = p.owner` +
					` AND m.table_name = p.table_name`),
				notSystem("WHERE", "p.owner"),
				always(`AND (@schema IS NULL OR p.owner LIKE @schema)`),
				always(`AND (@name IS NULL OR p.table_name LIKE @name)`),
				always(`ORDER BY p.owner, p.table_name`),
			},
			Fields: []dbmeta.Field{
				{Name: "schema"}, {Name: "name"}, {Name: "owner"},
				{
					Name: "type",
					Desc: "always table: Oracle partitions an index separately" +
						" rather than as a partitioned relation of its own",
				},
				{
					Name: "parent",
					Desc: "always empty: an Oracle partition is not a table," +
						" so a partitioned table has no parent to name",
				},
				{Name: "strategy"},
				{Name: "expression", Desc: "the partitioning key columns, in order"},
				{Name: "comment"},
			},
			Params: schemaNameSystem("table"),
			Scan: func(rows *sql.Rows) (dbmeta.PartitionedTable, error) {
				var v dbmeta.PartitionedTable
				err := rows.Scan(&v.Schema, &v.Name, &v.Owner, &v.Type, &v.Parent,
					&v.Strategy, &v.Expression, &v.Comment)
				return v, err
			},
		})

	// The column statistics the optimizer reads.
	dbmeta.ColumnStats.Register(dbmeta.Oracle, &dbmeta.Binding[dbmeta.ColumnStat]{
		Stmt: dbmeta.Stmt{
			always(`SELECT SYS_CONTEXT('USERENV', 'DB_NAME') AS "catalog"`),
			always(`, s.owner AS "schema"`),
			always(`, s.table_name AS "table"`),
			always(`, s.column_name AS "name"`),
			always(`, s.avg_col_len AS "avg_width"`),
			// Oracle counts the NULLs and PostgreSQL keeps the fraction, so
			// the count is divided by the row count of the table. A table
			// that has never been analyzed has no row count and the answer
			// is unknown rather than zero.
			always(`, CASE WHEN t.num_rows > 0 THEN s.num_nulls / t.num_rows END AS "null_frac"`),
			always(`, s.num_distinct AS "distinct"`),
			// low_value and high_value are RAW: the value in the internal
			// format of its own type, which only DBMS_STATS can decode and
			// only one type at a time. The hex is the whole fact Oracle
			// gives in one statement.
			always(`, RAWTOHEX(s.low_value) AS "min"`),
			always(`, RAWTOHEX(s.high_value) AS "max"`),
			always(`, NULL AS "mean"`),
			always(`, NULL AS "top_n"`),
			always(`, NULL AS "top_n_freqs"`),
			always(`FROM all_tab_col_statistics s`),
			always(`LEFT JOIN all_tables t ON t.owner = s.owner` +
				` AND t.table_name = s.table_name`),
			notSystem("WHERE", "s.owner"),
			always(`AND (@schema IS NULL OR s.owner LIKE @schema)`),
			always(`AND (@name IS NULL OR s.table_name LIKE @name)`),
			always(`ORDER BY s.owner, s.table_name, s.column_name`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog"}, {Name: "schema"}, {Name: "table"}, {Name: "name"},
			{Name: "avg_width"},
			{Name: "null_frac", Desc: "the NULL count over the row count of the table"},
			{Name: "distinct", Desc: "the distinct count, which Oracle keeps as a count"},
			{
				Name: "min",
				Desc: "the lowest value as hex, in the internal format of its" +
					" own type: Oracle keeps it as RAW",
			},
			{
				Name: "max",
				Desc: "the highest value as hex, in the internal format of its" +
					" own type: Oracle keeps it as RAW",
			},
			{Name: "mean", Desc: "always absent: Oracle records no mean"},
			{
				Name: "top_n",
				Desc: "always absent: Oracle keeps the histogram as one row per" +
					" bucket, which no single column can hold",
			},
			{Name: "top_n_freqs", Desc: "always absent, for the same reason as top_n"},
		},
		Params: schemaNameSystem("table"),
		Scan: func(rows *sql.Rows) (dbmeta.ColumnStat, error) {
			var v dbmeta.ColumnStat
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Table, &v.Name, &v.AvgWidth,
				&v.NullFrac, &v.Distinct, &v.Min, &v.Max, &v.Mean, &v.TopN, &v.TopNFreqs)
			return v, err
		},
	})
}
