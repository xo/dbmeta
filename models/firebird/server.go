package firebird

import (
	"database/sql"

	"github.com/xo/dbmeta"
)

func registerServer() {
	registerDatabases()
	registerCollations()
	registerEventTriggers()
	registerComments()
	registerReplication()
	registerCurrent()
}

func registerDatabases() {
	// \l. A Firebird database is a file and the server keeps no list of the
	// files it has served, so this reports the one attached and nothing
	// else. MON$DATABASE is how a database describes itself and RDB$DATABASE
	// is the one row table that carries its default character set and its
	// comment.
	dbmeta.Databases.Register(dbmeta.Firebird, &dbmeta.Binding[dbmeta.Database]{
		Stmt: dbmeta.Stmt{
			always(`SELECT TRIM(TRAILING FROM m.MON$DATABASE_NAME) AS "name"`),
			always(`, TRIM(TRAILING FROM COALESCE(m.MON$OWNER, '')) AS "owner"`),
			always(`, TRIM(TRAILING FROM COALESCE(d.RDB$CHARACTER_SET_NAME, '')) AS "encoding"`),
			always(`, TRIM(TRAILING FROM COALESCE((SELECT c.RDB$COLLATION_NAME FROM RDB$COLLATIONS c` +
				` JOIN RDB$CHARACTER_SETS s ON s.RDB$CHARACTER_SET_ID = c.RDB$CHARACTER_SET_ID` +
				` AND s.RDB$DEFAULT_COLLATE_NAME = c.RDB$COLLATION_NAME` +
				` WHERE s.RDB$CHARACTER_SET_NAME = d.RDB$CHARACTER_SET_NAME), '')) AS "collate"`),
			always(`, '' AS "ctype"`),
			always(`, CAST(NULL AS VARCHAR(1)) AS "access"`),
			always(`, CAST(NULL AS VARCHAR(1)) AS "tablespace"`),
			always(`, CAST(m.MON$PAGES * m.MON$PAGE_SIZE AS VARCHAR(30)) AS "size"`),
			always(`, d.RDB$DESCRIPTION AS "comment"`),
			always(`FROM MON$DATABASE m CROSS JOIN RDB$DATABASE d`),
			always(`WHERE ` + like(`m.MON$DATABASE_NAME`, `@name`) + ``),
		},
		Fields: []dbmeta.Field{
			{Name: "name", Desc: "the file path the server opened. A Firebird database is a file and has no name apart from it"},
			{Name: "owner", Desc: "from MON$OWNER, the user that created the database"},
			{Name: "encoding", Desc: "the default character set, which a column overrides with a clause of its own"},
			{Name: "collate", Desc: "the default collation of that character set"},
			{Name: "ctype", Desc: "always empty: Firebird has one collation setting and no separate character classification"},
			{Name: "access", Desc: "always absent: Firebird grants nothing on a database"},
			{Name: "tablespace", Desc: "always absent: Firebird has no tablespaces before 6.0"},
			{Name: "size", Desc: "the size in bytes, as pages times page size"},
			{Name: "comment"},
		},
		Params: []dbmeta.Param{
			{Name: "name", Desc: "database name pattern. There is only ever one row", Default: ""},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Database, error) {
			var v dbmeta.Database
			err := rows.Scan(&v.Name, &v.Owner, &v.Encoding, &v.Collate, &v.CType,
				&v.Access, &v.Tablespace, &v.Size, &v.Comment)
			return v, err
		},
	})

	// \dconfig. RDB$CONFIG arrived in 4.0 and is the server's configuration
	// as the attached database sees it, which is not the same as the file on
	// disk: a setting can be overridden per database.
	dbmeta.Settings.Register(dbmeta.Firebird, &dbmeta.Binding[dbmeta.Setting]{
		Stmt: dbmeta.Stmt{
			from4(`SELECT TRIM(TRAILING FROM c.RDB$CONFIG_NAME) AS "name"`),
			from4(`, c.RDB$CONFIG_VALUE AS "value"`),
			from4(`, CAST(NULL AS VARCHAR(1)) AS "type"`),
			from4(`, c.RDB$CONFIG_SOURCE AS "context"`),
			from4(`, CAST(NULL AS VARCHAR(1)) AS "access"`),
			from4(`FROM RDB$CONFIG c`),
			from4(`WHERE ` + like(`c.RDB$CONFIG_NAME`, `@name`) + ``),
			from4(`ORDER BY c.RDB$CONFIG_NAME`),
		},
		Fields: []dbmeta.Field{
			{Name: "name"},
			{Name: "value", Desc: "the value in force, absent where the setting has none"},
			{Name: "type", Desc: "always absent: RDB$CONFIG records no data type for a setting"},
			{Name: "context", Desc: "from RDB$CONFIG_SOURCE, which says where the value came from"},
			{Name: "access", Desc: "always absent: a setting carries no grant"},
		},
		Params: []dbmeta.Param{
			{Name: "name", Desc: "setting name pattern, empty for every setting", Default: ""},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Setting, error) {
			var v dbmeta.Setting
			err := rows.Scan(&v.Name, &v.Value, &v.Type, &v.Context, &v.Access)
			return v, err
		},
	})
}

func registerCollations() {
	// \dO. A Firebird collation belongs to a character set, so the character
	// set is what collate reports and there is no separate classification.
	dbmeta.Collations.Register(dbmeta.Firebird, &dbmeta.Binding[dbmeta.Collation]{
		Stmt: dbmeta.Stmt{
			always(`SELECT ` + noSchema),
			always(`, TRIM(TRAILING FROM c.RDB$COLLATION_NAME) AS "name"`),
			always(`, CAST(NULL AS VARCHAR(1)) AS "provider"`),
			always(`, TRIM(TRAILING FROM s.RDB$CHARACTER_SET_NAME) AS "collate"`),
			always(`, TRIM(TRAILING FROM s.RDB$CHARACTER_SET_NAME) AS "ctype"`),
			always(`, c.RDB$SPECIFIC_ATTRIBUTES AS "locale"`),
			always(`, TRUE AS "deterministic"`),
			always(`, c.RDB$DESCRIPTION AS "comment"`),
			always(`FROM RDB$COLLATIONS c`),
			always(`JOIN RDB$CHARACTER_SETS s ON s.RDB$CHARACTER_SET_ID = c.RDB$CHARACTER_SET_ID`),
			always(`WHERE ` + like(`''`, `@schema`) + ``),
			always(`AND ` + like(`c.RDB$COLLATION_NAME`, `@name`) + ``),
			always(`ORDER BY s.RDB$CHARACTER_SET_NAME, c.RDB$COLLATION_NAME`),
		},
		Fields: []dbmeta.Field{
			{Name: "schema", Desc: schemaDesc},
			{Name: "name"},
			{Name: "provider", Desc: "always absent: Firebird records no provider, and whether a collation comes from ICU or from the engine is not a column"},
			{Name: "collate", Desc: "the character set the collation belongs to. A Firebird collation is defined for one character set and cannot be used with another"},
			{Name: "ctype", Desc: "the same as collate: Firebird has no separate character classification"},
			{Name: "locale", Desc: "from RDB$SPECIFIC_ATTRIBUTES, which carries the ICU locale and the case and accent settings where there are any"},
			{Name: "deterministic", Desc: "always true: Firebird has no non deterministic collation"},
			{Name: "comment"},
		},
		Params: schemaAndName("collation"),
		Scan: func(rows *sql.Rows) (dbmeta.Collation, error) {
			var v dbmeta.Collation
			err := rows.Scan(&v.Schema, &v.Name, &v.Provider, &v.Collate, &v.CType,
				&v.Locale, &v.Deterministic, &v.Comment)
			return v, err
		},
	})
}

func registerEventTriggers() {
	// \dy. A Firebird trigger that names no table fires on a connection, a
	// transaction or a DDL statement, which is what an event trigger is.
	dbmeta.EventTriggers.Register(dbmeta.Firebird, &dbmeta.Binding[dbmeta.EventTrigger]{
		Stmt: dbmeta.Stmt{
			always(`SELECT TRIM(TRAILING FROM t.RDB$TRIGGER_NAME) AS "name"`),
			always(`, CASE t.RDB$TRIGGER_TYPE` +
				` WHEN 8192 THEN 'connect' WHEN 8193 THEN 'disconnect'` +
				` WHEN 8194 THEN 'transaction start' WHEN 8195 THEN 'transaction commit'` +
				` WHEN 8196 THEN 'transaction rollback' ELSE 'ddl' END AS "event"`),
			always(`, '' AS "owner"`),
			always(`, CASE WHEN COALESCE(t.RDB$TRIGGER_INACTIVE, 0) = 1` +
				` THEN 'disabled' ELSE 'enabled' END AS "enabled"`),
			always(`, '' AS "function"`),
			always(`, CAST(NULL AS VARCHAR(1)) AS "tags"`),
			always(`, t.RDB$DESCRIPTION AS "comment"`),
			always(`FROM RDB$TRIGGERS t`),
			always(`WHERE ` + userObject(`t.RDB$SYSTEM_FLAG`) + ` AND t.RDB$RELATION_NAME IS NULL`),
			always(`AND ` + like(`t.RDB$TRIGGER_NAME`, `@name`) + ``),
			always(`ORDER BY t.RDB$TRIGGER_NAME`),
		},
		Fields: []dbmeta.Field{
			{Name: "name"},
			{Name: "event", Desc: "connect, disconnect, one of the three transaction events, or ddl. A DDL trigger records the statements it fires on as a bit mask rather than as a list, so they are not separated here"},
			{Name: "owner", Desc: "always empty: RDB$TRIGGERS records no owner"},
			{Name: "enabled", Desc: "from RDB$TRIGGER_INACTIVE"},
			{Name: "function", Desc: "always empty: a Firebird trigger carries its own body rather than calling a function, and Triggers returns the body"},
			{Name: "tags", Desc: "always absent: Firebird has no command tag filter"},
			{Name: "comment"},
		},
		Params: []dbmeta.Param{
			{Name: "name", Desc: "trigger name pattern, empty for every one", Default: ""},
		},
		Scan: func(rows *sql.Rows) (dbmeta.EventTrigger, error) {
			var v dbmeta.EventTrigger
			err := rows.Scan(&v.Name, &v.Event, &v.Owner, &v.Enabled, &v.Function,
				&v.Tags, &v.Comment)
			return v, err
		},
	})
}

// commentsOn builds one arm of the comments union.
func commentsOn(table, name, kind, extra string) string {
	q := `SELECT '' AS "schema", TRIM(TRAILING FROM x.` + name + `) AS "name", '` + kind +
		`' AS "type", x.RDB$DESCRIPTION AS "comment" FROM ` + table + ` x` +
		` WHERE x.RDB$DESCRIPTION IS NOT NULL`
	if extra != "" {
		q += ` AND ` + extra
	}
	return q + ` AND ` + like(`x.`+name+``, `@name`) + ``
}

func registerComments() {
	// Every comment in the database, whatever it is on. Firebird stores one
	// on almost every catalog table, in a column called RDB$DESCRIPTION, so
	// this is a union over the tables that have one.
	//
	// A column comment is left out. It needs two names to identify it and
	// this kind has one, which is the same reason models elsewhere read a
	// column comment through Columns instead.
	dbmeta.Comments.Register(dbmeta.Firebird, &dbmeta.Binding[dbmeta.Comment]{
		Stmt: dbmeta.Stmt{
			always(commentsOn(`RDB$RELATIONS`, `RDB$RELATION_NAME`, `table`,
				userObject(`x.RDB$SYSTEM_FLAG`)+` AND x.RDB$VIEW_BLR IS NULL`)),
			always(`UNION ALL ` + commentsOn(`RDB$RELATIONS`, `RDB$RELATION_NAME`, `view`,
				userObject(`x.RDB$SYSTEM_FLAG`)+` AND x.RDB$VIEW_BLR IS NOT NULL`)),
			always(`UNION ALL ` + commentsOn(`RDB$FIELDS`, `RDB$FIELD_NAME`, `domain`,
				userObject(`x.RDB$SYSTEM_FLAG`)+` AND x.RDB$FIELD_NAME NOT STARTING WITH 'RDB$'`)),
			always(`UNION ALL ` + commentsOn(`RDB$PROCEDURES`, `RDB$PROCEDURE_NAME`, `procedure`,
				userObject(`x.RDB$SYSTEM_FLAG`))),
			always(`UNION ALL ` + commentsOn(`RDB$FUNCTIONS`, `RDB$FUNCTION_NAME`, `function`,
				userObject(`x.RDB$SYSTEM_FLAG`))),
			always(`UNION ALL ` + commentsOn(`RDB$TRIGGERS`, `RDB$TRIGGER_NAME`, `trigger`,
				userObject(`x.RDB$SYSTEM_FLAG`))),
			always(`UNION ALL ` + commentsOn(`RDB$GENERATORS`, `RDB$GENERATOR_NAME`, `sequence`,
				userObject(`x.RDB$SYSTEM_FLAG`))),
			always(`UNION ALL ` + commentsOn(`RDB$INDICES`, `RDB$INDEX_NAME`, `index`,
				userObject(`x.RDB$SYSTEM_FLAG`))),
			always(`UNION ALL ` + commentsOn(`RDB$EXCEPTIONS`, `RDB$EXCEPTION_NAME`, `exception`,
				userObject(`x.RDB$SYSTEM_FLAG`))),
			always(`UNION ALL ` + commentsOn(`RDB$PACKAGES`, `RDB$PACKAGE_NAME`, `package`,
				userObject(`x.RDB$SYSTEM_FLAG`))),
			always(`UNION ALL ` + commentsOn(`RDB$ROLES`, `RDB$ROLE_NAME`, `role`, ``)),
			always(`ORDER BY 3, 2`),
		},
		Fields: []dbmeta.Field{
			{Name: "schema", Desc: schemaDesc},
			{Name: "name"},
			{Name: "type", Desc: "the kind of object the comment is on"},
			{Name: "comment"},
		},
		Params: []dbmeta.Param{
			{Name: "name", Desc: "object name pattern, empty for every commented object", Default: ""},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Comment, error) {
			var v dbmeta.Comment
			err := rows.Scan(&v.Schema, &v.Name, &v.Type, &v.Comment)
			return v, err
		},
	})
}

func registerReplication() {
	// \dRp. Firebird 4.0 added logical replication and publishes a set of
	// tables the way PostgreSQL does. The subscriber side is configured in a
	// file rather than in the database, so Subscriptions has no answer here.
	dbmeta.Publications.Register(dbmeta.Firebird, &dbmeta.Binding[dbmeta.Publication]{
		Stmt: dbmeta.Stmt{
			from4(`SELECT TRIM(TRAILING FROM p.RDB$PUBLICATION_NAME) AS "name"`),
			from4(`, TRIM(TRAILING FROM COALESCE(p.RDB$OWNER_NAME, '')) AS "owner"`),
			from4(`, COALESCE(p.RDB$AUTO_ENABLE, 0) = 1 AS "all_tables"`),
			from4(`, TRUE AS "insert"`),
			from4(`, TRUE AS "update"`),
			from4(`, TRUE AS "delete"`),
			from4(`, FALSE AS "truncate"`),
			from4(`, FALSE AS "via_root"`),
			from4(`, CAST(NULL AS VARCHAR(1)) AS "comment"`),
			from4(`FROM RDB$PUBLICATIONS p`),
			from4(`WHERE ` + like(`p.RDB$PUBLICATION_NAME`, `@name`) + ``),
			from4(`ORDER BY p.RDB$PUBLICATION_NAME`),
		},
		Fields: []dbmeta.Field{
			{Name: "name"},
			{Name: "owner"},
			{Name: "all_tables", Desc: "from RDB$AUTO_ENABLE, which makes a table published as soon as it is created. It is the nearest thing Firebird has to FOR ALL TABLES"},
			{Name: "insert", Desc: "always true: Firebird replicates every kind of change and offers no per operation switch"},
			{Name: "update", Desc: "always true, for the same reason"},
			{Name: "delete", Desc: "always true, for the same reason"},
			{Name: "truncate", Desc: "always false: Firebird has no TRUNCATE"},
			{Name: "via_root", Desc: "always false: Firebird has no partitioned table, so there is no root to publish through"},
			{Name: "comment", Desc: "always absent: COMMENT ON has no publication form"},
		},
		Params: []dbmeta.Param{
			{Name: "name", Desc: "publication name pattern, empty for every one", Default: ""},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Publication, error) {
			var v dbmeta.Publication
			err := rows.Scan(&v.Name, &v.Owner, &v.AllTables, &v.Insert, &v.Update,
				&v.Delete, &v.Truncate, &v.ViaRoot, &v.Comment)
			return v, err
		},
	})

	// \dRp+.
	dbmeta.PublicationTables.Register(dbmeta.Firebird, &dbmeta.Binding[dbmeta.PublicationTable]{
		Stmt: dbmeta.Stmt{
			from4(`SELECT TRIM(TRAILING FROM t.RDB$PUBLICATION_NAME) AS "publication"`),
			from4(`, ` + noSchema),
			from4(`, TRIM(TRAILING FROM t.RDB$TABLE_NAME) AS "name"`),
			from4(`, '' AS "columns"`),
			from4(`, CAST(NULL AS VARCHAR(1)) AS "where"`),
			from4(`FROM RDB$PUBLICATION_TABLES t`),
			from4(`WHERE ` + like(`t.RDB$TABLE_NAME`, `@name`) + ``),
			from4(`ORDER BY t.RDB$PUBLICATION_NAME, t.RDB$TABLE_NAME`),
		},
		Fields: []dbmeta.Field{
			{Name: "publication"},
			{Name: "schema", Desc: schemaDesc},
			{Name: "name"},
			{Name: "columns", Desc: "always empty: Firebird publishes a whole table and has no column list"},
			{Name: "where", Desc: "always absent: Firebird has no row filter on a publication"},
		},
		Params: []dbmeta.Param{
			{Name: "name", Desc: "table name pattern, empty for every published table", Default: ""},
		},
		Scan: func(rows *sql.Rows) (dbmeta.PublicationTable, error) {
			var v dbmeta.PublicationTable
			err := rows.Scan(&v.Publication, &v.Schema, &v.Name, &v.Columns, &v.Where)
			return v, err
		},
	})
}

func registerCurrent() {
	// Firebird separates the user from the role it is acting under, and
	// CURRENT_ROLE is NONE where the connection named none.
	dbmeta.CurrentUser.Register(dbmeta.Firebird, &dbmeta.Binding[dbmeta.User]{
		Stmt: dbmeta.Stmt{
			always(`SELECT TRIM(TRAILING FROM CURRENT_USER) AS "name"`),
			always(`, TRIM(TRAILING FROM CURRENT_ROLE) AS "session"`),
			always(`FROM RDB$DATABASE`),
		},
		Fields: []dbmeta.Field{
			{Name: "name", Desc: "from CURRENT_USER, the authenticated principal"},
			{Name: "session", Desc: "from CURRENT_ROLE, the role the connection is acting under. It is NONE where the connection named no role, which is a role name rather than an absence"},
		},
		Scan: func(rows *sql.Rows) (dbmeta.User, error) {
			var v dbmeta.User
			err := rows.Scan(&v.Name, &v.Session)
			return v, err
		},
	})
}

// from4 is a fragment that exists only from Firebird 4.0. A query whose every
// fragment carries it reports [dbmeta.ErrVersionTooOld] on 3.0, which is the
// honest answer where the catalog table does not exist rather than being
// empty. See D63.
func from4(query string) dbmeta.Choice { return dbmeta.Choice{{Min: v4, Query: query}} }
