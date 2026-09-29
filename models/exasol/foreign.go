package exasol

import (
	"database/sql"

	"github.com/xo/dbmeta"
)

// A virtual schema is how Exasol reaches data it does not hold, and it
// lines up with PostgreSQL's foreign data in three of four places:
//
//	adapter script      the wrapper, the code that answers for a source
//	virtual schema      the server, one source with its properties
//	virtual table       the foreign table, which belongs to a virtual schema
//
// A virtual table belongs to its virtual schema the way a foreign table
// belongs to its server, which is why the schema is the server rather than
// the connection object its properties name.
func registerForeign() {
	// \dew.
	dbmeta.ForeignDataWrappers.Register(dbmeta.Exasol, &dbmeta.Binding[dbmeta.ForeignDataWrapper]{
		Stmt: dbmeta.Stmt{
			always(`SELECT s.SCRIPT_SCHEMA || '.' || s.SCRIPT_NAME AS "name"`),
			always(`, s.SCRIPT_OWNER AS "owner"`),
			always(`, s.SCRIPT_LANGUAGE AS "handler"`),
			always(`, CAST(NULL AS VARCHAR(1)) AS "validator"`),
			always(`, CAST(NULL AS VARCHAR(1)) AS "access"`),
			always(`, CAST(NULL AS VARCHAR(1)) AS "options"`),
			always(`, s.SCRIPT_COMMENT AS "comment"`),
			always(`FROM EXA_ALL_SCRIPTS s`),
			always(`WHERE s.SCRIPT_TYPE = 'ADAPTER'`),
			always(`AND ` + like(`s.SCRIPT_SCHEMA || '.' || s.SCRIPT_NAME`, `@name`)),
			always(`ORDER BY 1`),
		},
		Fields: []dbmeta.Field{
			{Name: "name", Desc: "the adapter script, schema qualified, because a script belongs to a schema and a wrapper is named without one"},
			{Name: "owner"},
			{Name: "handler", Desc: "the language the adapter is written in, such as LUA or JAVA. The script is the handler, and Functions carries its text"},
			{Name: "validator", Desc: "always absent: Exasol names no validator"},
			{Name: "access", Desc: "always absent: Privileges reads the grants"},
			{Name: "options", Desc: "always absent: an adapter takes its options from each virtual schema, which ForeignServers reports"},
			{Name: "comment", Desc: "from COMMENT ON SCRIPT"},
		},
		Params: nameOnly("adapter script"),
		Scan: func(rows *sql.Rows) (dbmeta.ForeignDataWrapper, error) {
			var v dbmeta.ForeignDataWrapper
			err := rows.Scan(dbmeta.NullAsEmpty(&v.Name), dbmeta.NullAsEmpty(&v.Owner), &v.Handler, &v.Validator, &v.Access,
				&v.Options, &v.Comment)
			return v, err
		},
	})

	// \des. A virtual schema's properties are its options, one row each in
	// their own view, folded here into one string.
	dbmeta.ForeignServers.Register(dbmeta.Exasol, &dbmeta.Binding[dbmeta.ForeignServer]{
		Stmt: dbmeta.Stmt{
			always(`SELECT v.SCHEMA_NAME AS "name"`),
			always(`, v.SCHEMA_OWNER AS "owner"`),
			always(`, v.ADAPTER_SCRIPT_SCHEMA || '.' || v.ADAPTER_SCRIPT_NAME AS "wrapper"`),
			always(`, CAST(NULL AS VARCHAR(1)) AS "type"`),
			always(`, CAST(NULL AS VARCHAR(1)) AS "version"`),
			always(`, CAST(NULL AS VARCHAR(1)) AS "access"`),
			// One bounded read of the schema's own properties.
			always(`, (SELECT GROUP_CONCAT(p.PROPERTY_NAME || '=' || p.PROPERTY_VALUE` +
				` ORDER BY p.PROPERTY_NAME SEPARATOR ', ')` +
				` FROM EXA_ALL_VIRTUAL_SCHEMA_PROPERTIES p` +
				` WHERE p.SCHEMA_OBJECT_ID = v.SCHEMA_OBJECT_ID) AS "options"`),
			always(`, s.SCHEMA_COMMENT AS "comment"`),
			always(`FROM EXA_ALL_VIRTUAL_SCHEMAS v`),
			always(`JOIN EXA_ALL_SCHEMAS s ON s.SCHEMA_OBJECT_ID = v.SCHEMA_OBJECT_ID`),
			always(`WHERE ` + like(`v.SCHEMA_NAME`, `@name`)),
			always(`ORDER BY v.SCHEMA_NAME`),
		},
		Fields: []dbmeta.Field{
			{Name: "name", Desc: "the virtual schema"},
			{Name: "owner"},
			{Name: "wrapper", Desc: "the adapter script behind it, schema qualified, which is a name ForeignDataWrappers reports"},
			{Name: "type", Desc: "always absent: Exasol records no server type. A property such as SQL_DIALECT says what the source is where the adapter takes one"},
			{Name: "version", Desc: "always absent: Exasol records no version for a source"},
			{Name: "access", Desc: "always absent: Privileges reads the grants"},
			{Name: "options", Desc: "the properties, name=value, comma separated and in name order, and absent where there are none"},
			{Name: "comment", Desc: "from COMMENT ON SCHEMA"},
		},
		Params: nameOnly("virtual schema"),
		Scan: func(rows *sql.Rows) (dbmeta.ForeignServer, error) {
			var v dbmeta.ForeignServer
			err := rows.Scan(dbmeta.NullAsEmpty(&v.Name), dbmeta.NullAsEmpty(&v.Owner), dbmeta.NullAsEmpty(&v.Wrapper), &v.Type, &v.Version,
				&v.Access, &v.Options, &v.Comment)
			return v, err
		},
	})

	// \deu. A connection holds an address and the credentials to use it,
	// and it is granted to a user or a role, which is what a user mapping
	// records: who can reach a source, and as whom. Both views are the
	// administrator's, because the connection view carries the remote user
	// and every grant, and a lesser principal is refused them.
	dbmeta.UserMappings.Register(dbmeta.Exasol, &dbmeta.Binding[dbmeta.UserMapping]{
		Stmt: dbmeta.Stmt{
			always(`SELECT c.CONNECTION_NAME AS "server"`),
			always(`, g.GRANTEE AS "name"`),
			always(`, 'user=' || c.USER_NAME || ', address=' || c.CONNECTION_STRING AS "options"`),
			always(`FROM EXA_DBA_CONNECTION_PRIVS g`),
			always(`JOIN EXA_DBA_CONNECTIONS c ON c.CONNECTION_NAME = g.GRANTED_CONNECTION`),
			always(`WHERE ` + like(`g.GRANTEE`, `@name`)),
			always(`ORDER BY c.CONNECTION_NAME, g.GRANTEE`),
		},
		Fields: []dbmeta.Field{
			{Name: "server", Desc: "the connection. It is not a virtual schema, which is what ForeignServers reports, and a virtual schema names the connection it uses in its CONNECTION_NAME property"},
			{Name: "name", Desc: "the user or role the connection is granted to"},
			{Name: "options", Desc: "the remote user and the address, and absent where the connection has no user. Exasol keeps the password and never reports it"},
		},
		Params: nameOnly("grantee"),
		Scan: func(rows *sql.Rows) (dbmeta.UserMapping, error) {
			var v dbmeta.UserMapping
			err := rows.Scan(dbmeta.NullAsEmpty(&v.Server), &v.Name, &v.Options)
			return v, err
		},
	})

	// \det.
	dbmeta.ForeignTables.Register(dbmeta.Exasol, &dbmeta.Binding[dbmeta.ForeignTable]{
		Stmt: dbmeta.Stmt{
			always(`SELECT t.TABLE_SCHEMA AS "schema"`),
			always(`, t.TABLE_NAME AS "name"`),
			always(`, t.TABLE_SCHEMA AS "server"`),
			always(`, CAST(NULL AS VARCHAR(1)) AS "options"`),
			always(`, a.TABLE_COMMENT AS "comment"`),
			always(`FROM EXA_ALL_VIRTUAL_TABLES t`),
			always(`JOIN EXA_ALL_TABLES a ON a.TABLE_OBJECT_ID = t.TABLE_OBJECT_ID`),
			always(`WHERE ` + like(`t.TABLE_SCHEMA`, `@schema`)),
			always(`AND ` + like(`t.TABLE_NAME`, `@name`)),
			always(`ORDER BY t.TABLE_SCHEMA, t.TABLE_NAME`),
		},
		Fields: []dbmeta.Field{
			{Name: "schema"}, {Name: "name"},
			{Name: "server", Desc: "the virtual schema the table belongs to, which is the same name as schema. A virtual table cannot live anywhere else"},
			{Name: "options", Desc: "always absent: a virtual table takes no options of its own. Its schema's properties apply"},
			{Name: "comment", Desc: "the comment on the table"},
		},
		Params: []dbmeta.Param{
			{Name: "schema", Desc: "virtual schema name pattern, empty for every one", Default: ""},
			{Name: "name", Desc: "virtual table name pattern, empty for every one", Default: ""},
		},
		Scan: func(rows *sql.Rows) (dbmeta.ForeignTable, error) {
			var v dbmeta.ForeignTable
			err := rows.Scan(dbmeta.NullAsEmpty(&v.Schema), dbmeta.NullAsEmpty(&v.Name), dbmeta.NullAsEmpty(&v.Server), &v.Options, &v.Comment)
			return v, err
		},
	})
}
