package influxdb

import (
	"database/sql"

	"github.com/xo/dbmeta"
)

// iox is the schema every measurement is in, and so every trigger's table.
const iox = "iox"

func registerTriggers() {
	// A processing engine trigger runs a Python plugin when something
	// happens: a write to one table, a write to any table, a schedule or a
	// request. system.processing_engine_triggers lists them for the database
	// of the connection, with the specification as JSON text, such as
	// {"single_table_wal_write":{"table_name":"author"}}. The table is read
	// out of that text where the specification is a write to one table, and
	// it is empty for the others. See D170.
	dbmeta.Triggers.Register(dbmeta.InfluxDB, &dbmeta.Binding[dbmeta.Trigger]{
		Stmt: dbmeta.Stmt{
			always(`SELECT '` + iox + `' AS "schema"`),
			always(`, x.tbl AS "table"`),
			always(`, x.name AS "name"`),
			always(`, x.state AS "enabled"`),
			always(`, x.spec AS "definition"`),
			always(`, NULL AS "comment"`),
			always(`FROM (SELECT CASE WHEN p.trigger_specification LIKE '%single_table_wal_write%'`),
			always(`THEN regexp_replace(p.trigger_specification, '^.*"table_name":"([^"]*)".*$', '\1') ELSE '' END AS tbl`),
			always(`, p.trigger_name AS name`),
			always(`, CASE WHEN p.disabled THEN 'disabled' ELSE 'enabled' END AS state`),
			always(`, p.trigger_specification AS spec`),
			always(`FROM system.processing_engine_triggers p) x`),
			always(`WHERE (@schema = '' OR '` + iox + `' LIKE @schema)`),
			always(`AND (@parent = '' OR x.tbl LIKE @parent)`),
			always(`AND (@name = '' OR x.name LIKE @name)`),
			always(`ORDER BY x.tbl, x.name`),
		},
		Fields: []dbmeta.Field{
			{Name: "schema", Desc: "always iox, the schema every measurement is in"},
			{
				Name: "table",
				Desc: "the measurement a write to one table runs the trigger for, and empty" +
					" for a trigger on every write, on a schedule or on a request",
			},
			{Name: "name"},
			{Name: "enabled", Desc: "enabled, or disabled where the trigger was created disabled"},
			{
				Name: "definition",
				Desc: "the specification of the trigger as the server keeps it, which is JSON text." +
					" The plugin file, its arguments and its error behavior are not carried",
			},
			{Name: "comment", Desc: "always absent: a trigger carries no comment"},
		},
		Params: []dbmeta.Param{
			{Name: "schema", Desc: "schema name pattern, empty for every schema", Default: ""},
			{Name: "parent", Desc: "table name pattern, empty for every table", Default: ""},
			{Name: "name", Desc: "trigger name pattern, empty for every trigger", Default: ""},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Trigger, error) {
			var v dbmeta.Trigger
			err := rows.Scan(&v.Schema, &v.Table, &v.Name, &v.Enabled, &v.Definition, &v.Comment)
			return v, err
		},
	})
}
