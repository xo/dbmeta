package avatica

import (
	"database/sql"

	"github.com/xo/dbmeta"
)

func registerServer() {
	registerDatabases()
	registerSettings()
	registerCollations()
	registerComments()
	registerCurrent()
}

func registerDatabases() {
	// One server holds one database, and its one catalog is PUBLIC.
	dbmeta.Databases.Register(dbmeta.Avatica, &dbmeta.Binding[dbmeta.Database]{
		Stmt: dbmeta.Stmt{
			always(`SELECT d.CATALOG_NAME AS "name"`),
			always(`, ` + text + ` AS "owner"`),
			always(`, s.FORM_OF_USE AS "encoding"`),
			always(`, s.DEFAULT_COLLATE_NAME AS "collate"`),
			always(`, '' AS "ctype"`),
			always(`, ` + text + ` AS "access"`),
			always(`, ` + text + ` AS "tablespace"`),
			always(`, ` + text + ` AS "size"`),
			always(`, ` + text + ` AS "comment"`),
			always(`FROM INFORMATION_SCHEMA.INFORMATION_SCHEMA_CATALOG_NAME d`),
			always(`CROSS JOIN INFORMATION_SCHEMA.CHARACTER_SETS s`),
			always(`WHERE s.CHARACTER_SET_NAME = 'SQL_TEXT'`),
			always(`AND ` + like("d.CATALOG_NAME", "@name")),
			always(`ORDER BY 1`),
		},
		Fields: []dbmeta.Field{
			{Name: "name", Desc: "the catalog name, which is PUBLIC. The name of the database in the URL, such as mem:dbmeta, is in SYSTEM_SESSIONINFO and is not the catalog"},
			{Name: "owner", Desc: "always absent: HSQLDB records no owner of the database"},
			{Name: "encoding", Desc: "from FORM_OF_USE of the character set SQL_TEXT, which is UTF16: HSQLDB holds every string as Unicode"},
			{Name: "collate", Desc: "the default collation of the character set SQL_TEXT, which is what a character column gets"},
			{Name: "ctype", Desc: "always empty: HSQLDB has no separate character classification"},
			{Name: "access", Desc: "always absent: HSQLDB grants nothing on a database"},
			{Name: "tablespace", Desc: "always absent: HSQLDB has no tablespaces"},
			{Name: "size", Desc: "always absent: the catalog has no size of the database"},
			{Name: "comment", Desc: "always absent: COMMENT ON takes no database in HSQLDB"},
		},
		Params: []dbmeta.Param{
			{Name: "name", Desc: "database name pattern, empty for every one", Default: ""},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Database, error) {
			var v dbmeta.Database
			err := rows.Scan(&v.Name, dbmeta.NullAsEmpty(&v.Owner), &v.Encoding, &v.Collate, &v.CType,
				&v.Access, &v.Tablespace, &v.Size, &v.Comment)
			return v, err
		},
	})
}

func registerSettings() {
	// SYSTEM_PROPERTIES holds the properties of the database, and every user
	// can read it. SYSTEM_CONNECTION_PROPERTIES holds the same names with the
	// default of each and no value, so this is the one that has the value.
	dbmeta.Settings.Register(dbmeta.Avatica, &dbmeta.Binding[dbmeta.Setting]{
		Stmt: dbmeta.Stmt{
			always(`SELECT p.PROPERTY_NAME AS "name"`),
			always(`, p.PROPERTY_VALUE AS "value"`),
			always(`, p.PROPERTY_CLASS AS "type"`),
			always(`, p.PROPERTY_SCOPE || ' ' || p.PROPERTY_NAMESPACE AS "context"`),
			always(`, ` + text + ` AS "access"`),
			always(`, ` + text + ` AS "display"`),
			always(`FROM INFORMATION_SCHEMA.SYSTEM_PROPERTIES p`),
			always(`WHERE ` + like("p.PROPERTY_NAME", "@name")),
			always(`ORDER BY 1`),
		},
		Fields: []dbmeta.Field{
			{Name: "name", Desc: "the property, such as sql.enforce_size or hsqldb.tx"},
			{Name: "value"},
			{Name: "type", Desc: "the Java class of the value, such as Integer, Boolean or String"},
			{Name: "context", Desc: "the scope and the namespace, such as SESSION database.properties"},
			{Name: "access", Desc: "always absent: a setting carries no grant"},
			{Name: "display", Desc: "always absent: HSQLDB shows a value in one form, which is value"},
		},
		Params: []dbmeta.Param{
			{Name: "name", Desc: "setting name pattern, empty for every setting", Default: ""},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Setting, error) {
			var v dbmeta.Setting
			err := rows.Scan(&v.Name, &v.Value, &v.Type, &v.Context, &v.Access, &v.Display)
			return v, err
		},
	})
}

func registerCollations() {
	// COLLATIONS lists the collations of the server, which belong to no
	// schema although the view puts them in INFORMATION_SCHEMA.
	dbmeta.Collations.Register(dbmeta.Avatica, &dbmeta.Binding[dbmeta.Collation]{
		Stmt: dbmeta.Stmt{
			always(`SELECT c.COLLATION_SCHEMA AS "schema"`),
			always(`, c.COLLATION_NAME AS "name"`),
			always(`, ` + text + ` AS "provider"`),
			always(`, ` + text + ` AS "collate"`),
			always(`, ` + text + ` AS "ctype"`),
			always(`, ` + text + ` AS "locale"`),
			always(`, TRUE AS "deterministic"`),
			always(`, c.PAD_ATTRIBUTE AS "comment"`),
			always(`, ` + text + ` AS "rules"`),
			always(`FROM INFORMATION_SCHEMA.COLLATIONS c`),
			always(`WHERE ` + like("c.COLLATION_NAME", "@name")),
			always(`ORDER BY 2`),
		},
		Fields: []dbmeta.Field{
			{Name: "schema", Desc: "always INFORMATION_SCHEMA: HSQLDB lists every collation there, although none belongs to a schema"},
			{Name: "name", Desc: "SQL_TEXT, SQL_TEXT_UCC, SQL_IDENTIFIER, or the name of a language, such as Afrikaans"},
			{Name: "provider", Desc: "always absent: HSQLDB records no provider, and the language collations come from the Java runtime"},
			{Name: "collate", Desc: "always absent: the catalog records the name alone"},
			{Name: "ctype", Desc: "always absent, for the same reason"},
			{Name: "locale", Desc: "always absent, for the same reason"},
			{Name: "deterministic", Desc: "always true: HSQLDB has no non deterministic collation"},
			{Name: "comment", Desc: "PAD_ATTRIBUTE, which is PAD SPACE or NO PAD. It is the one fact the catalog keeps about a collation beyond its name, and HSQLDB writes no comment on a collation"},
			{Name: "rules", Desc: "always absent: the catalog has no tailoring rules"},
		},
		Params: []dbmeta.Param{
			{Name: "name", Desc: "collation name pattern, empty for every one", Default: ""},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Collation, error) {
			var v dbmeta.Collation
			err := rows.Scan(&v.Schema, &v.Name, &v.Provider, &v.Collate, &v.CType,
				&v.Locale, &v.Deterministic, &v.Comment, &v.Rules)
			return v, err
		},
	})
}

func registerComments() {
	// SYSTEM_COMMENTS holds the comment of a table, a view and a column, and
	// the server writes one on each of its own views, so with_system adds those.
	dbmeta.Comments.Register(dbmeta.Avatica, &dbmeta.Binding[dbmeta.Comment]{
		Stmt: dbmeta.Stmt{
			always(`SELECT c.OBJECT_SCHEMA AS "schema"`),
			always(`, CASE WHEN c.COLUMN_NAME IS NULL THEN c.OBJECT_NAME ELSE c.OBJECT_NAME || '.' || c.COLUMN_NAME END AS "name"`),
			always(`, LOWER(c.OBJECT_TYPE) AS "type"`),
			always(`, c.COMMENT AS "comment"`),
			always(`FROM INFORMATION_SCHEMA.SYSTEM_COMMENTS c`),
			always(`WHERE c.COMMENT IS NOT NULL AND c.COMMENT <> ''`),
			always(`AND ` + notSystem("c.OBJECT_SCHEMA")),
			always(`AND ` + like("c.OBJECT_NAME", "@name")),
			always(`ORDER BY 3, 1, 2`),
		},
		Fields: []dbmeta.Field{
			{Name: "schema"},
			{Name: "name", Desc: "the table or view, and for a column the table, a dot and the column"},
			{Name: "type", Desc: "table, view or column"},
			{Name: "comment"},
		},
		Params: []dbmeta.Param{
			{Name: "name", Desc: "object name pattern, empty for every commented object. It matches the table and not the column", Default: ""},
			system,
		},
		Scan: func(rows *sql.Rows) (dbmeta.Comment, error) {
			var v dbmeta.Comment
			err := rows.Scan(&v.Schema, &v.Name, &v.Type, &v.Comment)
			return v, err
		},
	})
}

func registerCurrent() {
	dbmeta.CurrentUser.Register(dbmeta.Avatica, &dbmeta.Binding[dbmeta.User]{
		Stmt: dbmeta.Stmt{
			always(`SELECT CURRENT_USER AS "name"`),
			always(`, SESSION_USER AS "session"`),
			always(`FROM (VALUES (0)) AS d (x)`),
		},
		Fields: []dbmeta.Field{
			{Name: "name", Desc: "from CURRENT_USER, which is the owner of a routine that runs with definer rights"},
			{Name: "session", Desc: "from SESSION_USER, the user that connected"},
		},
		Scan: func(rows *sql.Rows) (dbmeta.User, error) {
			var v dbmeta.User
			err := rows.Scan(&v.Name, &v.Session)
			return v, err
		},
	})
}
