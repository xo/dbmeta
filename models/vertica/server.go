package vertica

import (
	"database/sql"

	"github.com/xo/dbmeta"
)

func registerServer() {
	// \l. A Vertica cluster runs one database.
	dbmeta.Databases.Register(dbmeta.Vertica, &dbmeta.Binding[dbmeta.Database]{
		Stmt: dbmeta.Stmt{
			always(`SELECT d.database_name AS "name"`),
			always(`, d.owner_name AS "owner"`),
			always(`, 'UTF8' AS "encoding"`),
			always(`, '' AS "collate"`),
			always(`, '' AS "ctype"`),
			always(`, CAST(NULL AS VARCHAR) AS "access"`),
			always(`, CAST(NULL AS VARCHAR) AS "tablespace"`),
			always(`, CAST(NULL AS VARCHAR) AS "size"`),
			always(`, CAST(NULL AS VARCHAR) AS "comment"`),
			always(`FROM v_catalog.databases d`),
			always(`WHERE ` + like(`d.database_name`, `@name`)),
		},
		Fields: []dbmeta.Field{
			{Name: "name", Desc: "the database. A Vertica cluster runs one"},
			{Name: "owner"},
			{Name: "encoding", Desc: "always UTF8: Vertica stores every string as UTF-8"},
			{Name: "collate", Desc: "always empty: a collation is a session's locale in Vertica rather than the database's"},
			{Name: "ctype", Desc: "always empty, for the same reason"},
			{Name: "access", Desc: "always absent: Privileges reads the grants"},
			{Name: "tablespace", Desc: "always absent: Tablespaces reads the storage locations, which belong to the nodes rather than to the database"},
			{Name: "size", Desc: "always absent: the size is in the monitoring tables per projection, and reading it is not one bounded statement"},
			{Name: "comment", Desc: "always absent: COMMENT ON has no database form"},
		},
		Params: nameOnly("database"),
		Scan: func(rows *sql.Rows) (dbmeta.Database, error) {
			var v dbmeta.Database
			err := rows.Scan(&v.Name, &v.Owner, &v.Encoding, &v.Collate, &v.CType,
				&v.Access, &v.Tablespace, &v.Size, &v.Comment)
			return v, err
		},
	})

	// \db. A storage location is a directory on a node that holds data or
	// temporary files, which is what a tablespace is, and a label lets a
	// storage policy send a table's data to it.
	dbmeta.Tablespaces.Register(dbmeta.Vertica, &dbmeta.Binding[dbmeta.Tablespace]{
		Stmt: dbmeta.Stmt{
			always(`SELECT COALESCE(NULLIF(l.location_label, ''), l.location_path) AS "name"`),
			always(`, CAST(NULL AS VARCHAR) AS "owner"`),
			always(`, l.location_path AS "location"`),
			always(`, 'node=' || l.node_name || ', usage=' || l.location_usage AS "options"`),
			always(`, CAST(NULL AS VARCHAR) AS "size"`),
			always(`, CAST(NULL AS VARCHAR) AS "access"`),
			always(`, CAST(NULL AS VARCHAR) AS "comment"`),
			always(`FROM v_catalog.storage_locations l`),
			always(`WHERE NOT l.is_retired`),
			always(`AND ` + like(`COALESCE(NULLIF(l.location_label, ''), l.location_path)`, `@name`)),
			always(`ORDER BY l.node_name, l.location_path`),
		},
		Fields: []dbmeta.Field{
			{Name: "name", Desc: "the location's label, which a storage policy names, and its path where it has no label"},
			{Name: "owner", Desc: "always absent: a storage location belongs to the database"},
			{Name: "location", Desc: "the directory on the node"},
			{Name: "options", Desc: "the node and what the location holds, such as DATA,TEMP"},
			{Name: "size", Desc: "always absent: the disk use is in the monitoring tables"},
			{Name: "access", Desc: "always absent: Privileges reads the grants"},
			{Name: "comment", Desc: "always absent: COMMENT ON has no storage location form"},
		},
		Params: nameOnly("storage location"),
		Scan: func(rows *sql.Rows) (dbmeta.Tablespace, error) {
			var v dbmeta.Tablespace
			err := rows.Scan(&v.Name, &v.Owner, &v.Location, &v.Options, &v.Size, &v.Access, &v.Comment)
			return v, err
		},
	})

	// \dconfig. A parameter has a value at each level it can be set at, and
	// configuration_parameters reports the one in force and where it came
	// from.
	dbmeta.Settings.Register(dbmeta.Vertica, &dbmeta.Binding[dbmeta.Setting]{
		Stmt: dbmeta.Stmt{
			always(`SELECT c.parameter_name AS "name"`),
			always(`, c.current_value AS "value"`),
			always(`, CAST(NULL AS VARCHAR) AS "type"`),
			always(`, LOWER(c.current_level) AS "context"`),
			always(`, CAST(NULL AS VARCHAR) AS "access"`),
			always(`FROM v_monitor.configuration_parameters c`),
			always(`WHERE ` + like(`c.parameter_name`, `@name`)),
			always(`ORDER BY c.parameter_name, c.node_name`),
		},
		Fields: []dbmeta.Field{
			{Name: "name", Desc: "the parameter. On a cluster of more than one node it appears once per node"},
			{Name: "value", Desc: "the value in force"},
			{Name: "type", Desc: "always absent: Vertica records no data type for a parameter"},
			{Name: "context", Desc: "the level the value comes from: default, database, node or session"},
			{Name: "access", Desc: "always absent: a parameter carries no grant"},
		},
		Params: nameOnly("parameter"),
		Scan: func(rows *sql.Rows) (dbmeta.Setting, error) {
			var v dbmeta.Setting
			err := rows.Scan(&v.Name, &v.Value, &v.Type, &v.Context, &v.Access)
			return v, err
		},
	})

	// \des. An HCatalog schema reaches the tables of a Hive metastore through
	// the HCatalog connector, which is what a foreign server is: one source,
	// and the connection settings to reach it.
	dbmeta.ForeignServers.Register(dbmeta.Vertica, &dbmeta.Binding[dbmeta.ForeignServer]{
		Stmt: dbmeta.Stmt{
			always(`SELECT h.schema_name AS "name"`),
			always(`, h.schema_owner AS "owner"`),
			always(`, 'hcatalog' AS "wrapper"`),
			always(`, CAST(NULL AS VARCHAR) AS "type"`),
			always(`, CAST(NULL AS VARCHAR) AS "version"`),
			always(`, CAST(NULL AS VARCHAR) AS "access"`),
			always(`, 'hostname=' || h.hostname || ', port=' || CAST(h.port AS VARCHAR)` +
				` || ', hcatalog_schema=' || h.hcatalog_schema_name AS "options"`),
			always(`, ` + comment("SCHEMA", "''", "h.schema_name") + ` AS "comment"`),
			always(`FROM v_catalog.hcatalog_schemata h`),
			always(`WHERE ` + like(`h.schema_name`, `@name`)),
			always(`ORDER BY h.schema_name`),
		},
		Fields: []dbmeta.Field{
			{Name: "name", Desc: "the HCatalog schema, which is a schema in Vertica as well"},
			{Name: "owner"},
			{Name: "wrapper", Desc: "always hcatalog: the HCatalog connector is the only kind of source that is a schema"},
			{Name: "type", Desc: "always absent: the source is always a Hive metastore"},
			{Name: "version", Desc: "always absent: Vertica records no version for the metastore"},
			{Name: "access", Desc: "always absent: Privileges reads the grants"},
			{Name: "options", Desc: "the metastore host and port and the Hive schema the Vertica schema maps to"},
			{Name: "comment", Desc: "from COMMENT ON SCHEMA"},
		},
		Params: nameOnly("HCatalog schema"),
		Scan: func(rows *sql.Rows) (dbmeta.ForeignServer, error) {
			var v dbmeta.ForeignServer
			err := rows.Scan(&v.Name, &v.Owner, &v.Wrapper, &v.Type, &v.Version,
				&v.Access, &v.Options, &v.Comment)
			return v, err
		},
	})

	// \det. An external table reads its rows from files through the COPY
	// statement it was created with, which is what a foreign table does.
	// There is no server object, so the statement is the options.
	dbmeta.ForeignTables.Register(dbmeta.Vertica, &dbmeta.Binding[dbmeta.ForeignTable]{
		Stmt: dbmeta.Stmt{
			always(`SELECT t.table_schema AS "schema"`),
			always(`, t.table_name AS "name"`),
			always(`, '' AS "server"`),
			always(`, t.table_definition AS "options"`),
			always(`, ` + comment("TABLE", "t.table_schema", "t.table_name") + ` AS "comment"`),
			always(`FROM v_catalog.tables t`),
			always(`WHERE t.table_definition <> ''`),
			always(`AND ` + notSystem(`t.table_schema`)),
			always(`AND ` + like(`t.table_schema`, `@schema`)),
			always(`AND ` + like(`t.table_name`, `@name`)),
			always(`ORDER BY t.table_schema, t.table_name`),
		},
		Fields: []dbmeta.Field{
			{Name: "schema"}, {Name: "name"},
			{Name: "server", Desc: "always empty: an external table names its files directly and there is no server object"},
			{Name: "options", Desc: "the COPY statement the table reads through, such as COPY FROM '/data/*.csv' DELIMITER ','"},
			{Name: "comment", Desc: "the table's comment"},
		},
		Params: schemaAndName("external table"),
		Scan: func(rows *sql.Rows) (dbmeta.ForeignTable, error) {
			var v dbmeta.ForeignTable
			err := rows.Scan(&v.Schema, &v.Name, &v.Server, &v.Options, &v.Comment)
			return v, err
		},
	})

	// The first schema on the search path that exists. CURRENT_SCHEMA() is
	// a meta-function that Vertica allows only in the outermost select list,
	// not in a WHERE, a join or a derived table, so the schema's row cannot
	// be joined to it and the owner and comment are left to Schemas.
	dbmeta.CurrentSchema.Register(dbmeta.Vertica, &dbmeta.Binding[dbmeta.Schema]{
		Stmt: dbmeta.Stmt{
			always(`SELECT '' AS "catalog"`),
			always(`, CURRENT_SCHEMA() AS "name"`),
			always(`, '' AS "owner"`),
			always(`, CAST(NULL AS VARCHAR) AS "comment"`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "always empty: a Vertica connection reaches one database"},
			{Name: "name", Desc: "from CURRENT_SCHEMA(), the first schema on the search path that exists"},
			{Name: "owner", Desc: "always empty: Vertica refuses CURRENT_SCHEMA() anywhere but the select list, so the schema's row cannot be joined to it. Schemas carries the owner"},
			{Name: "comment", Desc: "always absent, for the same reason. Schemas carries the comment"},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Schema, error) {
			var v dbmeta.Schema
			err := rows.Scan(&v.Catalog, &v.Name, &v.Owner, &v.Comment)
			return v, err
		},
	})

	dbmeta.CurrentUser.Register(dbmeta.Vertica, &dbmeta.Binding[dbmeta.User]{
		Stmt: dbmeta.Stmt{
			always(`SELECT CURRENT_USER() AS "name"`),
			always(`, SESSION_USER() AS "session"`),
		},
		Fields: []dbmeta.Field{
			{Name: "name", Desc: "from CURRENT_USER(), which a stored procedure running as its definer changes"},
			{Name: "session", Desc: "from SESSION_USER(), the user that connected"},
		},
		Scan: func(rows *sql.Rows) (dbmeta.User, error) {
			var v dbmeta.User
			err := rows.Scan(&v.Name, &v.Session)
			return v, err
		},
	})
}
