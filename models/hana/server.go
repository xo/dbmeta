package hana

import (
	"database/sql"

	"github.com/xo/dbmeta"
)

func registerServer() {
	registerDatabases()
	registerSettings()
	registerCollations()
	registerComments()
	registerStats()
	registerCurrent()
}

func registerDatabases() {
	// \l. A HANA system holds a system database and its tenants, and
	// M_DATABASES lists both. A connection reaches one of them.
	dbmeta.Databases.Register(dbmeta.HANA, &dbmeta.Binding[dbmeta.Database]{
		Stmt: dbmeta.Stmt{
			always(`SELECT d.DATABASE_NAME AS "name"`),
			always(`, d.OS_USER AS "owner"`),
			always(`, 'UTF8' AS "encoding"`),
			always(`, '' AS "collate"`),
			always(`, '' AS "ctype"`),
			always(`, CAST(NULL AS NVARCHAR(1)) AS "access"`),
			always(`, CAST(NULL AS NVARCHAR(1)) AS "tablespace"`),
			always(`, '' AS "size"`),
			always(`, d.DESCRIPTION AS "comment"`),
			always(`FROM SYS.M_DATABASES d`),
			always(`WHERE ` + like(`d.DATABASE_NAME`, `@name`)),
			always(`ORDER BY d.DATABASE_NAME`),
		},
		Fields: []dbmeta.Field{
			{Name: "name", Desc: "the tenant name. A HANA system has a system database and one or more tenants, and this lists both"},
			{Name: "owner", Desc: "the operating system user the database runs as, which is the nearest thing HANA records to an owner"},
			{Name: "encoding", Desc: "always UTF8: HANA stores every string as Unicode and has no per database encoding"},
			{Name: "collate", Desc: "always empty: a collation is per column in HANA rather than per database"},
			{Name: "ctype", Desc: "always empty, for the same reason"},
			{Name: "access", Desc: "always absent: HANA grants nothing on a database"},
			{Name: "tablespace", Desc: "always absent: HANA has no tablespaces"},
			{Name: "size", Desc: "always empty: the size is in the monitoring views per host and service rather than per database, and reading it is not one bounded statement"},
			{Name: "comment", Desc: "from DESCRIPTION"},
		},
		Params: []dbmeta.Param{
			{Name: "name", Desc: "database name pattern, empty for every one", Default: ""},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Database, error) {
			var v dbmeta.Database
			err := rows.Scan(&v.Name, &v.Owner, &v.Encoding, &v.Collate, &v.CType,
				&v.Access, &v.Tablespace, &v.Size, &v.Comment)
			return v, err
		},
	})

	// \dA. A HANA table is stored by row or by column and the choice is the
	// nearest thing the product has to an access method, the way a MySQL
	// storage engine and a Trino connector are.
	dbmeta.AccessMethods.Register(dbmeta.HANA, &dbmeta.Binding[dbmeta.AccessMethod]{
		Stmt: dbmeta.Stmt{
			always(`SELECT LOWER(t.TABLE_TYPE) || ' store' AS "name"`),
			always(`, 'table' AS "type"`),
			always(`, '' AS "handler"`),
			always(`, 'used by ' || CAST(COUNT(*) AS NVARCHAR(20)) || ' tables' AS "comment"`),
			always(`FROM SYS.TABLES t`),
			always(`WHERE ` + notSystem(`t.SCHEMA_NAME`)),
			always(`AND ` + like(`LOWER(t.TABLE_TYPE) || ' store'`, `@name`)),
			always(`GROUP BY t.TABLE_TYPE`),
			always(`ORDER BY 1`),
		},
		Fields: []dbmeta.Field{
			{Name: "name", Desc: "row store or column store, which is how a HANA table is held"},
			{Name: "type", Desc: "always table: the choice applies to a table"},
			{Name: "handler", Desc: "always empty: HANA names no handler function"},
			{Name: "comment", Desc: "how many tables use it. HANA has no catalog of storage kinds, so this counts the tables that name each one, and a kind nothing uses does not appear"},
		},
		Params: []dbmeta.Param{
			{Name: "name", Desc: "access method name pattern, empty for every one", Default: ""},
			{Name: "with_system", Desc: "include the schemas SAP HANA keeps for itself", Default: false},
		},
		Scan: func(rows *sql.Rows) (dbmeta.AccessMethod, error) {
			var v dbmeta.AccessMethod
			err := rows.Scan(&v.Name, &v.Type, &v.Handler, &v.Comment)
			return v, err
		},
	})
}

func registerSettings() {
	// \dconfig. M_INIFILE_CONTENTS is the server configuration as the
	// attached database sees it, layered: a key appears once per layer that
	// sets it, and the last layer wins.
	dbmeta.Settings.Register(dbmeta.HANA, &dbmeta.Binding[dbmeta.Setting]{
		Stmt: dbmeta.Stmt{
			always(`SELECT c.FILE_NAME || '/' || c.SECTION || '/' || c.KEY AS "name"`),
			always(`, c.VALUE AS "value"`),
			always(`, CAST(NULL AS NVARCHAR(1)) AS "type"`),
			always(`, c.LAYER_NAME AS "context"`),
			always(`, CAST(NULL AS NVARCHAR(1)) AS "access"`),
			always(`FROM SYS.M_INIFILE_CONTENTS c`),
			always(`WHERE ` + like(`c.KEY`, `@name`)),
			always(`ORDER BY c.FILE_NAME, c.SECTION, c.KEY, c.LAYER_NAME`),
		},
		Fields: []dbmeta.Field{
			{Name: "name", Desc: "file/section/key, because a HANA setting is identified by all three and the key alone is not unique"},
			{Name: "value"},
			{Name: "type", Desc: "always absent: M_INIFILE_CONTENTS records no data type"},
			{Name: "context", Desc: "the layer the value comes from, such as DEFAULT, SYSTEM or DATABASE. A key appears once per layer that sets it and the last one wins"},
			{Name: "access", Desc: "always absent: a setting carries no grant"},
		},
		Params: []dbmeta.Param{
			{Name: "name", Desc: "setting key pattern, empty for every setting. It matches the key alone and not the file or section", Default: ""},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Setting, error) {
			var v dbmeta.Setting
			err := rows.Scan(&v.Name, &v.Value, &v.Type, &v.Context, &v.Access)
			return v, err
		},
	})
}

func registerCollations() {
	// \dO.
	dbmeta.Collations.Register(dbmeta.HANA, &dbmeta.Binding[dbmeta.Collation]{
		Stmt: dbmeta.Stmt{
			always(`SELECT '' AS "schema"`),
			always(`, c.COLLATION_NAME AS "name"`),
			always(`, CAST(NULL AS NVARCHAR(1)) AS "provider"`),
			always(`, c.LOCALE AS "collate"`),
			always(`, c.LOCALE AS "ctype"`),
			always(`, c.LOCALE AS "locale"`),
			always(`, TRUE AS "deterministic"`),
			always(`, c.DESCRIPTION AS "comment"`),
			always(`FROM SYS.COLLATIONS c`),
			always(`WHERE ` + like(`c.COLLATION_NAME`, `@name`)),
			always(`ORDER BY c.COLLATION_NAME`),
		},
		Fields: []dbmeta.Field{
			{Name: "schema", Desc: "always empty: a HANA collation belongs to the engine rather than to a schema"},
			{Name: "name"},
			{Name: "provider", Desc: "always absent: HANA records no provider"},
			{Name: "collate", Desc: "the locale, which is the only thing HANA records beyond the name"},
			{Name: "ctype", Desc: "the same as collate: HANA has no separate character classification"},
			{Name: "locale"},
			{Name: "deterministic", Desc: "always true: HANA has no non deterministic collation"},
			{Name: "comment", Desc: "from DESCRIPTION, which is the collation's own text and not a COMMENT ON"},
		},
		Params: []dbmeta.Param{
			{Name: "name", Desc: "collation name pattern, empty for every one", Default: ""},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Collation, error) {
			var v dbmeta.Collation
			err := rows.Scan(&v.Schema, &v.Name, &v.Provider, &v.Collate, &v.CType,
				&v.Locale, &v.Deterministic, &v.Comment)
			return v, err
		},
	})
}

func registerComments() {
	// Every comment in the database. HANA puts COMMENTS on the object's own
	// catalog view rather than in one place, so this is a union over the
	// views that have the column.
	dbmeta.Comments.Register(dbmeta.HANA, &dbmeta.Binding[dbmeta.Comment]{
		Stmt: dbmeta.Stmt{
			always(`SELECT t.SCHEMA_NAME AS "schema", t.TABLE_NAME AS "name"`),
			always(`, 'table' AS "type", t.COMMENTS AS "comment"`),
			always(`FROM SYS.TABLES t WHERE t.COMMENTS IS NOT NULL`),
			always(`AND ` + notSystem(`t.SCHEMA_NAME`) + ` AND ` + like(`t.TABLE_NAME`, `@name`)),
			always(`UNION ALL`),
			always(`SELECT v.SCHEMA_NAME, v.VIEW_NAME, 'view', v.COMMENTS`),
			always(`FROM SYS.VIEWS v WHERE v.COMMENTS IS NOT NULL`),
			always(`AND ` + notSystem(`v.SCHEMA_NAME`) + ` AND ` + like(`v.VIEW_NAME`, `@name`)),
			always(`UNION ALL`),
			always(`SELECT '', u.USER_NAME, 'user', u.COMMENTS`),
			always(`FROM SYS.USERS u WHERE u.COMMENTS IS NOT NULL`),
			always(`AND ` + like(`u.USER_NAME`, `@name`)),
			always(`UNION ALL`),
			always(`SELECT '', r.ROLE_NAME, 'role', r.COMMENTS`),
			always(`FROM SYS.ROLES r WHERE r.COMMENTS IS NOT NULL`),
			always(`AND ` + like(`r.ROLE_NAME`, `@name`)),
			always(`ORDER BY 3, 1, 2`),
		},
		Fields: []dbmeta.Field{
			{Name: "schema", Desc: "empty for a user and a role, which belong to the database rather than to a schema"},
			{Name: "name"},
			{Name: "type", Desc: "table, view, user or role. A column comment is left out, because it needs two names to identify it and this kind has one, and Columns carries it"},
			{Name: "comment"},
		},
		Params: []dbmeta.Param{
			{Name: "name", Desc: "object name pattern, empty for every commented object", Default: ""},
			{Name: "with_system", Desc: "include the schemas SAP HANA keeps for itself", Default: false},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Comment, error) {
			var v dbmeta.Comment
			err := rows.Scan(&v.Schema, &v.Name, &v.Type, &v.Comment)
			return v, err
		},
	})
}

func registerStats() {
	// \ss. M_CS_ALL_COLUMNS is the column store's own accounting and it
	// carries a row count and a distinct count per column, which is two of
	// the things a planner statistic holds.
	dbmeta.ColumnStats.Register(dbmeta.HANA, &dbmeta.Binding[dbmeta.ColumnStat]{
		Stmt: dbmeta.Stmt{
			always(`SELECT '' AS "catalog"`),
			always(`, m.SCHEMA_NAME AS "schema"`),
			always(`, m.TABLE_NAME AS "table"`),
			always(`, m.COLUMN_NAME AS "name"`),
			always(`, CAST(NULL AS BIGINT) AS "avg_width"`),
			always(`, CAST(NULL AS DOUBLE) AS "null_frac"`),
			always(`, CAST(m.DISTINCT_COUNT AS DOUBLE) AS "distinct"`),
			always(`, CAST(NULL AS NVARCHAR(1)) AS "min"`),
			always(`, CAST(NULL AS NVARCHAR(1)) AS "max"`),
			always(`, CAST(NULL AS NVARCHAR(1)) AS "mean"`),
			always(`, CAST(NULL AS NVARCHAR(1)) AS "top_n"`),
			always(`, CAST(NULL AS NVARCHAR(1)) AS "top_n_freqs"`),
			always(`FROM SYS.M_CS_ALL_COLUMNS m`),
			always(`WHERE ` + notSystem(`m.SCHEMA_NAME`)),
			always(`AND ` + like(`m.SCHEMA_NAME`, `@schema`)),
			always(`AND ` + like(`m.TABLE_NAME`, `@parent`)),
			always(`AND ` + like(`m.COLUMN_NAME`, `@name`)),
			always(`ORDER BY m.SCHEMA_NAME, m.TABLE_NAME, m.COLUMN_NAME`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "always empty: a connection reaches one tenant database"},
			{Name: "schema"}, {Name: "table"}, {Name: "name"},
			{Name: "avg_width", Desc: "always absent: the column store reports memory per column rather than an average value width, and the two are not the same number"},
			{Name: "null_frac", Desc: "always absent: M_CS_ALL_COLUMNS counts rows and distinct values and does not count nulls"},
			{Name: "distinct", Desc: "from DISTINCT_COUNT, a count rather than a fraction. It is the column store's own accounting and it is exact rather than sampled"},
			{Name: "min", Desc: "always absent: SYS.DATA_STATISTICS holds a minimum and only for a column somebody created a statistics object on"},
			{Name: "max", Desc: "always absent, for the same reason"},
			{Name: "mean", Desc: "always absent: HANA computes no mean"},
			{Name: "top_n", Desc: "always absent: HANA keeps no most common value list"},
			{Name: "top_n_freqs", Desc: "always absent, for the same reason"},
		},
		Params: []dbmeta.Param{
			{Name: "schema", Desc: "schema name pattern, empty for every schema", Default: ""},
			{Name: "parent", Desc: "table name pattern, empty for every table", Default: ""},
			{Name: "name", Desc: "column name pattern. Only a column table appears: a row store table has no entry here", Default: ""},
			{Name: "with_system", Desc: "include the schemas SAP HANA keeps for itself", Default: false},
		},
		Scan: func(rows *sql.Rows) (dbmeta.ColumnStat, error) {
			var v dbmeta.ColumnStat
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Table, &v.Name, &v.AvgWidth,
				&v.NullFrac, &v.Distinct, &v.Min, &v.Max, &v.Mean, &v.TopN, &v.TopNFreqs)
			return v, err
		},
	})

	// \dX. A HANA data statistics object is created over one or more columns
	// of one table, which is what an extended statistic is.
	dbmeta.ExtendedStats.Register(dbmeta.HANA, &dbmeta.Binding[dbmeta.ExtendedStat]{
		Stmt: dbmeta.Stmt{
			always(`SELECT s.DATA_STATISTICS_SCHEMA_NAME AS "schema"`),
			always(`, s.DATA_STATISTICS_NAME AS "name"`),
			always(`, '' AS "owner"`),
			always(`, s.DATA_SOURCE_OBJECT_NAME AS "table"`),
			// The columns are part of the kind rather than a field of
			// their own, because ExtendedStat has nowhere else to put
			// them and they are what makes the statistic what it is.
			always(`, LOWER(s.DATA_STATISTICS_TYPE) || ' on ' ||` +
				` s.DATA_SOURCE_COLUMN_NAMES AS "kinds"`),
			always(`, CAST(NULL AS NVARCHAR(1)) AS "comment"`),
			always(`FROM SYS.DATA_STATISTICS s`),
			always(`WHERE ` + notSystem(`s.DATA_STATISTICS_SCHEMA_NAME`)),
			always(`AND ` + like(`s.DATA_STATISTICS_SCHEMA_NAME`, `@schema`)),
			always(`AND ` + like(`s.DATA_STATISTICS_NAME`, `@name`)),
			always(`ORDER BY 1, 2`),
		},
		Fields: []dbmeta.Field{
			{Name: "schema"}, {Name: "name"},
			{Name: "owner", Desc: "always empty: DATA_STATISTICS records no owner"},
			{Name: "table", Desc: "the object the statistic is over, which HANA allows to be a view as well as a table"},
			{Name: "kinds", Desc: "the statistic kind and the columns it covers, such as histogram on A,B. HANA records the two separately and this kind has one field for both"},
			{Name: "comment", Desc: "always absent: COMMENT ON has no statistics form"},
		},
		Params: schemaAndName("statistic"),
		Scan: func(rows *sql.Rows) (dbmeta.ExtendedStat, error) {
			var v dbmeta.ExtendedStat
			err := rows.Scan(&v.Schema, &v.Name, &v.Owner, &v.Table, &v.Kinds, &v.Comment)
			return v, err
		},
	})
}

func registerCurrent() {
	dbmeta.CurrentSchema.Register(dbmeta.HANA, &dbmeta.Binding[dbmeta.Schema]{
		Stmt: dbmeta.Stmt{
			always(`SELECT '' AS "catalog"`),
			always(`, CURRENT_SCHEMA AS "name"`),
			always(`, '' AS "owner"`),
			always(`, CAST(NULL AS NVARCHAR(1)) AS "comment"`),
			always(`FROM SYS.DUMMY`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "always empty: a connection reaches one tenant database"},
			{Name: "name", Desc: "from CURRENT_SCHEMA, which is the user's default schema unless the session set one"},
			{Name: "owner", Desc: "always empty: Schemas carries the owner"},
			{Name: "comment", Desc: "always absent: COMMENT ON has no schema form"},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Schema, error) {
			var v dbmeta.Schema
			err := rows.Scan(&v.Catalog, &v.Name, &v.Owner, &v.Comment)
			return v, err
		},
	})

	dbmeta.CurrentUser.Register(dbmeta.HANA, &dbmeta.Binding[dbmeta.User]{
		Stmt: dbmeta.Stmt{
			always(`SELECT CURRENT_USER AS "name"`),
			always(`, SESSION_USER AS "session"`),
			always(`FROM SYS.DUMMY`),
		},
		Fields: []dbmeta.Field{
			{Name: "name", Desc: "from CURRENT_USER, which is the definer inside a routine that runs as one"},
			{Name: "session", Desc: "from SESSION_USER, the principal that connected. It differs from name inside a definer rights routine"},
		},
		Scan: func(rows *sql.Rows) (dbmeta.User, error) {
			var v dbmeta.User
			err := rows.Scan(&v.Name, &v.Session)
			return v, err
		},
	})
}
