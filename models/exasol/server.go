package exasol

import (
	"database/sql"

	"github.com/xo/dbmeta"
)

func registerServer() {
	registerDatabases()
	registerSettings()
	registerCurrent()
}

func registerDatabases() {
	// \l. An Exasol cluster runs one database and a connection reaches it,
	// so this is one row. EXA_METADATA holds its name, the same table the
	// version comes from.
	dbmeta.Databases.Register(dbmeta.Exasol, &dbmeta.Binding[dbmeta.Database]{
		Stmt: dbmeta.Stmt{
			always(`SELECT m.PARAM_VALUE AS "name"`),
			always(`, 'SYS' AS "owner"`),
			always(`, 'UTF8' AS "encoding"`),
			always(`, '' AS "collate"`),
			always(`, '' AS "ctype"`),
			always(`, CAST(NULL AS VARCHAR(1)) AS "access"`),
			always(`, CAST(NULL AS VARCHAR(1)) AS "tablespace"`),
			always(`, '' AS "size"`),
			always(`, CAST(NULL AS VARCHAR(1)) AS "comment"`),
			always(`FROM EXA_METADATA m`),
			always(`WHERE m.PARAM_NAME = 'databaseName'`),
			always(`AND ` + like(`m.PARAM_VALUE`, `@name`)),
		},
		Fields: []dbmeta.Field{
			{Name: "name", Desc: "the database name, from EXA_METADATA. An Exasol cluster runs one database"},
			{Name: "owner", Desc: "always SYS, the user the database is created with"},
			{Name: "encoding", Desc: "always UTF8: Exasol sets the character set per column, UTF8 or ASCII, and UTF8 is what every string column is unless it says otherwise"},
			{Name: "collate", Desc: "always empty: Exasol has no collation"},
			{Name: "ctype", Desc: "always empty, for the same reason"},
			{Name: "access", Desc: "always absent: Exasol grants nothing on a database"},
			{Name: "tablespace", Desc: "always absent: Exasol has no tablespaces"},
			{Name: "size", Desc: "always empty: the size is in the statistics tables per interval, and reading it is not one bounded statement"},
			{Name: "comment", Desc: "always absent: COMMENT ON has no database form"},
		},
		Params: nameOnly("database"),
		Scan: func(rows *sql.Rows) (dbmeta.Database, error) {
			var v dbmeta.Database
			err := rows.Scan(empty(&v.Name), empty(&v.Owner), empty(&v.Encoding), empty(&v.Collate), empty(&v.CType),
				&v.Access, &v.Tablespace, empty(&v.Size), &v.Comment)
			return v, err
		},
	})
}

func registerSettings() {
	// \dconfig. Every Exasol parameter has a value for the session and one
	// for the system, set with ALTER SESSION and ALTER SYSTEM, and
	// EXA_PARAMETERS holds both on one row. This reports each as a row of
	// its own, the way HANA reports a key once per layer.
	dbmeta.Settings.Register(dbmeta.Exasol, &dbmeta.Binding[dbmeta.Setting]{
		Stmt: dbmeta.Stmt{
			always(`SELECT p.PARAMETER_NAME AS "name"`),
			always(`, p.SESSION_VALUE AS "value"`),
			always(`, CAST(NULL AS VARCHAR(1)) AS "type"`),
			always(`, 'session' AS "context"`),
			always(`, CAST(NULL AS VARCHAR(1)) AS "access"`),
			always(`FROM EXA_PARAMETERS p`),
			always(`WHERE ` + like(`p.PARAMETER_NAME`, `@name`)),
			always(`UNION ALL`),
			always(`SELECT p.PARAMETER_NAME, p.SYSTEM_VALUE, CAST(NULL AS VARCHAR(1))`),
			always(`, 'system', CAST(NULL AS VARCHAR(1))`),
			always(`FROM EXA_PARAMETERS p`),
			always(`WHERE ` + like(`p.PARAMETER_NAME`, `@name`)),
			always(`ORDER BY 1, 4`),
		},
		Fields: []dbmeta.Field{
			{Name: "name", Desc: "the parameter, such as NLS_DATE_FORMAT or QUERY_TIMEOUT. Each appears twice, once per context"},
			{Name: "value", Desc: "the value in that context, and absent where the parameter is unset, such as SQL_PREPROCESSOR_SCRIPT with no script"},
			{Name: "type", Desc: "always absent: EXA_PARAMETERS records no data type"},
			{Name: "context", Desc: "session for the value this session sees, which ALTER SESSION sets, and system for the default, which ALTER SYSTEM sets"},
			{Name: "access", Desc: "always absent: a parameter carries no grant"},
		},
		Params: nameOnly("parameter"),
		Scan: func(rows *sql.Rows) (dbmeta.Setting, error) {
			var v dbmeta.Setting
			err := rows.Scan(empty(&v.Name), &v.Value, &v.Type, &v.Context, &v.Access)
			return v, err
		},
	})
}

func registerCurrent() {
	// An Exasol session has no current schema until it opens one, and
	// CREATE SCHEMA opens the schema it creates. Reading the schema row by
	// that name returns nothing when there is none, which is the answer
	// PostgreSQL gives for a search path that names nothing.
	dbmeta.CurrentSchema.Register(dbmeta.Exasol, &dbmeta.Binding[dbmeta.Schema]{
		Stmt: dbmeta.Stmt{
			always(`SELECT '' AS "catalog"`),
			always(`, s.SCHEMA_NAME AS "name"`),
			always(`, s.SCHEMA_OWNER AS "owner"`),
			always(`, s.SCHEMA_COMMENT AS "comment"`),
			always(`FROM EXA_ALL_SCHEMAS s`),
			always(`WHERE s.SCHEMA_NAME = CURRENT_SCHEMA`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "always empty: an Exasol connection reaches one database"},
			{Name: "name", Desc: "from CURRENT_SCHEMA, which is the schema OPEN SCHEMA or CREATE SCHEMA last opened. There is no row until one is open"},
			{Name: "owner"},
			{Name: "comment"},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Schema, error) {
			var v dbmeta.Schema
			err := rows.Scan(empty(&v.Catalog), empty(&v.Name), empty(&v.Owner), &v.Comment)
			return v, err
		},
	})

	dbmeta.CurrentUser.Register(dbmeta.Exasol, &dbmeta.Binding[dbmeta.User]{
		Stmt: dbmeta.Stmt{
			always(`SELECT CURRENT_USER AS "name"`),
			always(`, CAST(NULL AS VARCHAR(1)) AS "session"`),
			always(`FROM DUAL`),
		},
		Fields: []dbmeta.Field{
			{Name: "name", Desc: "from CURRENT_USER"},
			{Name: "session", Desc: "always absent: Exasol refuses SESSION_USER as not supported, and a session acts as the user it authenticated as, except under IMPERSONATE, which CURRENT_USER follows"},
		},
		Scan: func(rows *sql.Rows) (dbmeta.User, error) {
			var v dbmeta.User
			err := rows.Scan(empty(&v.Name), &v.Session)
			return v, err
		},
	})
}
