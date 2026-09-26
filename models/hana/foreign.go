package hana

import (
	"database/sql"

	"github.com/xo/dbmeta"
)

// Smart data access is why SAP HANA answers the foreign data kinds from a
// catalog rather than by analogy. Every one of the four PostgreSQL concepts
// has a view of its own:
//
//	SYS.ADAPTERS         the wrapper
//	SYS.REMOTE_SOURCES   the server
//	SYS.REMOTE_USERS     the user mapping
//	SYS.VIRTUAL_TABLES   the foreign table
//
// No other model here answers all four.
func registerForeign() {
	// \dew.
	dbmeta.ForeignDataWrappers.Register(dbmeta.HANA, &dbmeta.Binding[dbmeta.ForeignDataWrapper]{
		Stmt: dbmeta.Stmt{
			always(`SELECT a.ADAPTER_NAME AS "name"`),
			always(`, '' AS "owner"`),
			always(`, '' AS "handler"`),
			always(`, '' AS "validator"`),
			always(`, a.CONFIGURATION AS "options"`),
			always(`, CAST(NULL AS NVARCHAR(1)) AS "access"`),
			always(`, CAST(NULL AS NVARCHAR(1)) AS "comment"`),
			always(`FROM SYS.ADAPTERS a`),
			always(`WHERE ` + like(`a.ADAPTER_NAME`, `@name`)),
			always(`ORDER BY a.ADAPTER_NAME`),
		},
		Fields: []dbmeta.Field{
			{Name: "name", Desc: "the adapter name, such as hanaodbc or odbc"},
			{Name: "owner", Desc: "always empty: an adapter belongs to the server rather than to a user"},
			{Name: "handler", Desc: "always empty: HANA names no handler function"},
			{Name: "validator", Desc: "always empty, for the same reason"},
			{Name: "options", Desc: "from CONFIGURATION, which is the adapter's own settings as text"},
			{Name: "access", Desc: "always absent: Privileges reads the grants"},
			{Name: "comment", Desc: "always absent: COMMENT ON has no adapter form"},
		},
		Params: []dbmeta.Param{
			{Name: "name", Desc: "adapter name pattern, empty for every one", Default: ""},
		},
		Scan: func(rows *sql.Rows) (dbmeta.ForeignDataWrapper, error) {
			var v dbmeta.ForeignDataWrapper
			err := rows.Scan(&v.Name, &v.Owner, &v.Handler, &v.Validator, &v.Options,
				&v.Access, &v.Comment)
			return v, err
		},
	})

	// \des.
	dbmeta.ForeignServers.Register(dbmeta.HANA, &dbmeta.Binding[dbmeta.ForeignServer]{
		Stmt: dbmeta.Stmt{
			always(`SELECT r.REMOTE_SOURCE_NAME AS "name"`),
			always(`, r.ADAPTER_NAME AS "wrapper"`),
			always(`, '' AS "owner"`),
			always(`, '' AS "type"`),
			always(`, '' AS "version"`),
			always(`, r.CONNECTION_INFO AS "options"`),
			always(`, CAST(NULL AS NVARCHAR(1)) AS "access"`),
			always(`, CAST(NULL AS NVARCHAR(1)) AS "comment"`),
			always(`FROM SYS.REMOTE_SOURCES r`),
			always(`WHERE ` + like(`r.REMOTE_SOURCE_NAME`, `@name`)),
			always(`ORDER BY r.REMOTE_SOURCE_NAME`),
		},
		Fields: []dbmeta.Field{
			{Name: "name"},
			{Name: "wrapper", Desc: "the adapter this source reaches through"},
			{Name: "owner", Desc: "always empty: REMOTE_SOURCES records no owner"},
			{Name: "type", Desc: "always empty: the adapter carries the kind and this row names the adapter"},
			{Name: "version", Desc: "always empty: HANA records no version for a remote source"},
			{Name: "options", Desc: "from CONNECTION_INFO, the connection settings as text"},
			{Name: "access", Desc: "always absent: Privileges reads the grants"},
			{Name: "comment", Desc: "always absent: COMMENT ON has no remote source form"},
		},
		Params: []dbmeta.Param{
			{Name: "name", Desc: "remote source name pattern, empty for every one", Default: ""},
		},
		Scan: func(rows *sql.Rows) (dbmeta.ForeignServer, error) {
			var v dbmeta.ForeignServer
			err := rows.Scan(&v.Name, &v.Wrapper, &v.Owner, &v.Type, &v.Version,
				&v.Options, &v.Access, &v.Comment)
			return v, err
		},
	})

	// \deu.
	dbmeta.UserMappings.Register(dbmeta.HANA, &dbmeta.Binding[dbmeta.UserMapping]{
		Stmt: dbmeta.Stmt{
			always(`SELECT u.REMOTE_DATABASE_NAME AS "server"`),
			always(`, u.USER_NAME AS "name"`),
			always(`, 'remote user ' || u.REMOTE_USER_NAME AS "options"`),
			always(`FROM SYS.REMOTE_USERS u`),
			always(`WHERE ` + like(`u.USER_NAME`, `@name`)),
			always(`ORDER BY u.REMOTE_DATABASE_NAME, u.USER_NAME`),
		},
		Fields: []dbmeta.Field{
			{Name: "server", Desc: "the remote database the mapping reaches"},
			{Name: "name", Desc: "the local user the mapping is for"},
			{Name: "options", Desc: "the remote user name. HANA keeps the password and never reports it"},
		},
		Params: []dbmeta.Param{
			{Name: "name", Desc: "local user name pattern, empty for every mapping", Default: ""},
		},
		Scan: func(rows *sql.Rows) (dbmeta.UserMapping, error) {
			var v dbmeta.UserMapping
			err := rows.Scan(&v.Server, &v.Name, &v.Options)
			return v, err
		},
	})

	// \det.
	dbmeta.ForeignTables.Register(dbmeta.HANA, &dbmeta.Binding[dbmeta.ForeignTable]{
		Stmt: dbmeta.Stmt{
			always(`SELECT t.SCHEMA_NAME AS "schema"`),
			always(`, t.TABLE_NAME AS "name"`),
			always(`, t.REMOTE_SOURCE_NAME AS "server"`),
			always(`, 'remote ' || COALESCE(t.REMOTE_OWNER_NAME, '') || '.' ||` +
				` COALESCE(t.REMOTE_OBJECT_NAME, '') AS "options"`),
			always(`, CAST(NULL AS NVARCHAR(1)) AS "comment"`),
			always(`FROM SYS.VIRTUAL_TABLES t`),
			always(`WHERE ` + notSystem(`t.SCHEMA_NAME`)),
			always(`AND ` + like(`t.SCHEMA_NAME`, `@schema`)),
			always(`AND ` + like(`t.TABLE_NAME`, `@name`)),
			always(`ORDER BY t.SCHEMA_NAME, t.TABLE_NAME`),
		},
		Fields: []dbmeta.Field{
			{Name: "schema"}, {Name: "name"},
			{Name: "server", Desc: "the remote source this virtual table reads through"},
			{Name: "options", Desc: "the object it stands for on the remote side"},
			{Name: "comment", Desc: "always absent: Tables carries the comment, and a virtual table appears there too"},
		},
		Params: schemaAndName("virtual table"),
		Scan: func(rows *sql.Rows) (dbmeta.ForeignTable, error) {
			var v dbmeta.ForeignTable
			err := rows.Scan(&v.Schema, &v.Name, &v.Server, &v.Options, &v.Comment)
			return v, err
		},
	})

	// \dRs. HANA replicates by subscribing to a remote source, which is the
	// subscriber side and the half PostgreSQL calls a subscription. There is
	// no publisher side in the catalog, so Publications has no answer.
	dbmeta.Subscriptions.Register(dbmeta.HANA, &dbmeta.Binding[dbmeta.Subscription]{
		Stmt: dbmeta.Stmt{
			always(`SELECT s.SUBSCRIPTION_NAME AS "name"`),
			always(`, s.OWNER_NAME AS "owner"`),
			always(`, ` + yes(`s.IS_VALID`) + ` AS "enabled"`),
			always(`, s.REMOTE_SOURCE_NAME AS "publications"`),
			always(`, s.CHANGE_MODE AS "synchronous"`),
			always(`, CAST(NULL AS NVARCHAR(1)) AS "slot"`),
			always(`, CAST(NULL AS NVARCHAR(1)) AS "comment"`),
			always(`FROM SYS.REMOTE_SUBSCRIPTIONS s`),
			always(`WHERE ` + like(`s.SUBSCRIPTION_NAME`, `@name`)),
			always(`ORDER BY s.SUBSCRIPTION_NAME`),
		},
		Fields: []dbmeta.Field{
			{Name: "name"},
			{Name: "owner"},
			{Name: "enabled", Desc: "from IS_VALID"},
			{Name: "publications", Desc: "the remote source subscribed to. HANA subscribes to a source rather than to a named publication, so this is the source"},
			{Name: "synchronous", Desc: "from CHANGE_MODE, which is how changes are applied"},
			{Name: "slot", Desc: "always absent: HANA has no replication slot"},
			{Name: "comment", Desc: "always absent: COMMENT ON has no subscription form"},
		},
		Params: []dbmeta.Param{
			{Name: "name", Desc: "subscription name pattern, empty for every one", Default: ""},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Subscription, error) {
			var v dbmeta.Subscription
			err := rows.Scan(&v.Name, &v.Owner, &v.Enabled, &v.Publications,
				&v.Synchronous, &v.Slot, &v.Comment)
			return v, err
		},
	})

	// \dF. A HANA text configuration is what a full text index points at,
	// which is the job a PostgreSQL text search configuration does.
	dbmeta.TextSearchConfigs.Register(dbmeta.HANA, &dbmeta.Binding[dbmeta.TextSearchConfig]{
		Stmt: dbmeta.Stmt{
			always(`SELECT c.SCHEMA_NAME AS "schema"`),
			always(`, c.NAME AS "name"`),
			always(`, c.TYPE AS "parser"`),
			always(`, CAST(NULL AS NVARCHAR(1)) AS "comment"`),
			always(`FROM SYS.TEXT_CONFIGURATIONS c`),
			always(`WHERE ` + notSystem(`c.SCHEMA_NAME`)),
			always(`AND ` + like(`c.SCHEMA_NAME`, `@schema`)),
			always(`AND ` + like(`c.NAME`, `@name`)),
			always(`ORDER BY c.SCHEMA_NAME, c.NAME`),
		},
		Fields: []dbmeta.Field{
			{Name: "schema"}, {Name: "name"},
			{Name: "parser", Desc: "from TYPE, the kind of configuration. HANA has no separate parser object, so the kind is what stands for one"},
			{Name: "comment", Desc: "always absent: COMMENT ON has no text configuration form"},
		},
		Params: schemaAndName("configuration"),
		Scan: func(rows *sql.Rows) (dbmeta.TextSearchConfig, error) {
			var v dbmeta.TextSearchConfig
			err := rows.Scan(&v.Schema, &v.Name, &v.Parser, &v.Comment)
			return v, err
		},
	})
}
