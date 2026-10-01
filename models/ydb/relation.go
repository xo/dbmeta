package ydb

import (
	"database/sql"

	"github.com/xo/dbmeta"
)

func registerRelations() {
	// \l. A connection names one database and no view lists another, so
	// this is one row: the database the connection is in.
	dbmeta.Databases.Register(dbmeta.YDB, &dbmeta.Binding[dbmeta.Database]{
		Stmt: dbmeta.Stmt{
			always("SELECT d.root AS `name`"),
			always(", o.Sid AS `owner`"),
			always(", '' AS `encoding`"),
			always(", '' AS `collate`"),
			always(", '' AS `ctype`"),
			always(", a.access AS `access`"),
			always(", CAST(NULL AS Utf8) AS `tablespace`"),
			always(", CAST(NULL AS Utf8) AS `size`"),
			always(", CAST(NULL AS Utf8) AS `comment`"),
			always("FROM " + root),
			always("JOIN `.sys/auth_owners` AS o ON o.Path = d.root"),
			always("LEFT JOIN " + acl + " AS a ON a.path = d.root"),
			always("WHERE (@name = '' OR d.root LIKE @name)"),
		},
		Fields: []dbmeta.Field{
			{Name: "name", Desc: "the path of the database, such as /local"},
			{Name: "owner"},
			{
				Name: "encoding",
				Desc: "always empty: YDB stores no encoding. A Utf8 value is UTF-8" +
					" and a String value is bytes",
			},
			{Name: "collate", Desc: "always empty: YDB has no collation"},
			{Name: "ctype", Desc: "always empty: YDB has no character type"},
			{
				Name: "access",
				Desc: "the explicit grants on the database as sid=permission, and absent" +
					" when it has none",
			},
			{
				Name: "tablespace",
				Desc: "always absent: a database has several storage pools, and" +
					" Tablespaces lists them",
			},
			{
				Name: "size",
				Desc: "always absent: the size is a sum over every partition in" +
					" partition_stats, which grows with the catalog",
			},
			{Name: "comment", Desc: "always absent: YDB has no comment"},
		},
		Params: []dbmeta.Param{
			{Name: "name", Desc: "database path pattern, empty for every database", Default: ""},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Database, error) {
			var v dbmeta.Database
			err := rows.Scan(&v.Name, &v.Owner, &v.Encoding, &v.Collate, &v.CType,
				&v.Access, &v.Tablespace, &v.Size, &v.Comment)
			return v, err
		},
	})

	// \dn. A schema is a directory, named by its path relative to the
	// database. A directory that holds nothing is not here, because nothing
	// tells it from a view or a topic. See D161.
	dbmeta.Schemas.Register(dbmeta.YDB, &dbmeta.Binding[dbmeta.Schema]{
		Stmt: dbmeta.Stmt{
			always("SELECT s.catalog AS `catalog`"),
			always(", s.name AS `name`"),
			always(", s.owner AS `owner`"),
			always(", CAST(NULL AS Utf8) AS `comment`"),
			always("FROM (SELECT d.root AS catalog"),
			always(", String::JoinFromList(ListSkip(String::SplitToList(o.Path, '/'), d.depth), '/') AS name"),
			always(", " + topOf("String::SplitToList(o.Path, '/')") + " AS top"),
			always(", o.Sid AS owner"),
			always("FROM " + directories + " AS dir"),
			always("JOIN `.sys/auth_owners` AS o ON o.Path = dir.path"),
			always("CROSS JOIN " + root + ") AS s"),
			always("WHERE " + notSystem("s.top")),
			always("AND (@schema = '' OR s.name LIKE @schema)"),
			always("ORDER BY `catalog`, `name`"),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "the path of the database"},
			{Name: "name", Desc: "the path of the directory relative to the database"},
			{Name: "owner"},
			{Name: "comment", Desc: "always absent: YDB has no comment"},
		},
		Params: []dbmeta.Param{
			{Name: "schema", Desc: "directory path pattern, empty for every directory", Default: ""},
			{Name: "with_system", Desc: "include the directories YDB keeps for itself", Default: false},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Schema, error) {
			var v dbmeta.Schema
			err := rows.Scan(&v.Catalog, &v.Name, &v.Owner, &v.Comment)
			return v, err
		},
	})

	// \dt. A table is a path with partitions. A view is not here, because
	// no view says which paths are views. See D161.
	dbmeta.Tables.Register(dbmeta.YDB, &dbmeta.Binding[dbmeta.Table]{
		Stmt: dbmeta.Stmt{
			always("SELECT t.catalog AS `catalog`"),
			always(", t.schema AS `schema`"),
			always(", t.name AS `name`"),
			always(", t.type AS `type`"),
			always(", CAST(NULL AS Utf8) AS `comment`"),
			always("FROM (SELECT d.root AS catalog"),
			always(", " + schemaOf("p.parts") + " AS schema"),
			always(", ListLast(p.parts) AS name"),
			always(", " + topOf("p.parts") + " AS top"),
			always(", 'table' AS type"),
			always("FROM (SELECT path, String::SplitToList(path, '/') AS parts" +
				" FROM " + tablePaths + ") AS p"),
			always("CROSS JOIN " + root + ") AS t"),
			always("WHERE " + notSystem("t.top")),
			always("AND (@schema = '' OR t.schema LIKE @schema)"),
			always("AND (@name = '' OR t.name LIKE @name)"),
			// Every table is a table, so the types test names no column of
			// its own, and a test that names none is decided before the read.
			// YDB fails a read of a .sys view that such a test makes empty, so
			// t.name, which is never absent, keeps the test on the rows. See
			// D161.
			always("AND (@types = '' OR t.name IS NULL OR " + dbmeta.InList("@types", "t.type") + ")"),
			always("ORDER BY `catalog`, `schema`, `name`"),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "the path of the database"},
			{Name: "schema", Desc: "the directory relative to the database, and empty at its root"},
			{Name: "name"},
			{
				Name: "type",
				Desc: "always table, for a row table and a column table alike, because" +
					" a new column table reports no tablet for about a minute. A view" +
					" is not listed, because no view names one",
			},
			{Name: "comment", Desc: "always absent: YDB has no comment"},
		},
		Params: []dbmeta.Param{
			{Name: "schema", Desc: "directory path pattern, empty for every directory", Default: ""},
			{Name: "name", Desc: "table name pattern, empty for every table", Default: ""},
			dbmeta.TypesParam(),
			{Name: "with_system", Desc: "include the directories YDB keeps for itself", Default: false},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Table, error) {
			var v dbmeta.Table
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Name, &v.Type, &v.Comment)
			return v, err
		},
	})

	// \db. A storage pool is where a database keeps its data, and a column
	// family of a table names the kind of pool its data goes to, such as
	// ssd or hdd.
	dbmeta.Tablespaces.Register(dbmeta.YDB, &dbmeta.Binding[dbmeta.Tablespace]{
		Stmt: dbmeta.Stmt{
			always("SELECT p.Name AS `name`"),
			always(", CAST(NULL AS Utf8) AS `owner`"),
			always(", CAST(NULL AS Utf8) AS `location`"),
			always(", 'kind=' || p.Kind || ', erasure=' || p.ErasureSpecies AS `options`"),
			always(", CAST(NULL AS Utf8) AS `size`"),
			always(", CAST(NULL AS Utf8) AS `access`"),
			always(", CAST(NULL AS Utf8) AS `comment`"),
			always("FROM `.sys/ds_storage_pools` AS p"),
			always("WHERE (@name = '' OR p.Name LIKE @name)"),
			always("ORDER BY `name`"),
		},
		Fields: []dbmeta.Field{
			{Name: "name"},
			{Name: "owner", Desc: "always absent: a storage pool has no owner"},
			{
				Name: "location",
				Desc: "always absent: a pool is a set of storage groups on many disks" +
					" rather than one place",
			},
			{
				Name: "options",
				Desc: "the kind, which a column family names, and the erasure, which" +
					" says how the pool keeps copies of the data",
			},
			{Name: "size", Desc: "always absent: ds_storage_pools records no size"},
			{Name: "access", Desc: "always absent: a storage pool has no grants"},
			{Name: "comment", Desc: "always absent: YDB has no comment"},
		},
		Params: []dbmeta.Param{
			{Name: "name", Desc: "storage pool name pattern, empty for every pool", Default: ""},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Tablespace, error) {
			var v dbmeta.Tablespace
			err := rows.Scan(&v.Name, &v.Owner, &v.Location, &v.Options, &v.Size,
				&v.Access, &v.Comment)
			return v, err
		},
	})
}
