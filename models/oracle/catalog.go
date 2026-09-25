package oracle

import (
	"database/sql"

	"github.com/xo/dbmeta"
)

func registerCatalog() {
	// \dp. Oracle records one row per grant and PostgreSQL keeps one list per
	// object, so the rows are folded back into a list.
	dbmeta.Privileges.Register(dbmeta.Oracle, &dbmeta.Binding[dbmeta.Privilege]{
		Stmt: dbmeta.Stmt{
			always(`SELECT p.table_schema AS "schema"`),
			always(`, p.table_name AS "name"`),
			// An index lives in a namespace of its own, so a table and an
			// index can share a name in one schema. Joining all_objects
			// would then double every grant, and the subquery cannot.
			always(`, NVL((SELECT MAX(o.object_type) FROM all_objects o`),
			always(`  WHERE o.owner = p.table_schema AND o.object_name = p.table_name`),
			always(`  AND o.object_type NOT IN ('INDEX', 'LOB', 'TABLE PARTITION',`),
			always(`  'INDEX PARTITION', 'TABLE SUBPARTITION')), '') AS "type"`),
			always(`, `),
			listagg("p.grantee || '=' || p.privilege", "p.grantee, p.privilege"),
			always(`  AS "access"`),
			always(`, '' AS "column_access"`),
			always(`, '' AS "policies"`),
			always(`FROM all_tab_privs p`),
			always(`WHERE ` + notSystem("p.table_schema")),
			always(`AND (@schema IS NULL OR p.table_schema LIKE @schema)`),
			always(`AND (@name IS NULL OR p.table_name LIKE @name)`),
			always(`GROUP BY p.table_schema, p.table_name`),
			always(`ORDER BY p.table_schema, p.table_name`),
		},
		Fields: []dbmeta.Field{
			{Name: "schema"}, {Name: "name"},
			{Name: "type", Desc: "the object type, read from all_objects"},
			{Name: "access", Desc: "the grants as grantee=privilege, one list per object"},
			{
				Name: "column_access",
				Desc: "always empty: Oracle keeps a column grant in" +
					" all_col_privs, which is a second statement",
			},
			{
				Name: "policies",
				Desc: "always empty: an Oracle row level policy is a" +
					" DBMS_RLS object rather than a grant",
			},
		},
		Params: schemaNameSystem("object"),
		Scan: func(rows *sql.Rows) (dbmeta.Privilege, error) {
			var v dbmeta.Privilege
			err := rows.Scan(&v.Schema, &v.Name, &v.Type, &v.Access,
				&v.ColumnAccess, &v.Policies)
			return v, err
		},
	})

	// \des. A database link is what Oracle has where PostgreSQL has a foreign
	// server: a named, stored way to reach another database.
	dbmeta.ForeignServers.Register(dbmeta.Oracle, &dbmeta.Binding[dbmeta.ForeignServer]{
		Stmt: dbmeta.Stmt{
			always(`SELECT l.db_link AS "name"`),
			always(`, l.owner AS "owner"`),
			always(`, 'database link' AS "wrapper"`),
			always(`, NULL AS "type"`),
			always(`, NULL AS "version"`),
			always(`, NULL AS "access"`),
			always(`, 'user=' || NVL(l.username, '') || ', host=' ||` +
				` NVL(l.host, '') AS "options"`),
			always(`, NULL AS "comment"`),
			always(`FROM all_db_links l`),
			always(`WHERE ` + notSystem("l.owner")),
			always(`AND (@name IS NULL OR l.db_link LIKE @name)`),
			always(`ORDER BY l.owner, l.db_link`),
		},
		Fields: []dbmeta.Field{
			{Name: "name"}, {Name: "owner"},
			{
				Name: "wrapper",
				Desc: "always database link: Oracle has one mechanism where" +
					" PostgreSQL has a wrapper per kind of remote",
			},
			{Name: "type", Desc: "always absent: a database link has no type"},
			{Name: "version", Desc: "always absent: a database link records no version"},
			{
				Name: "access",
				Desc: "always absent: Oracle records a grant as a row, which the" +
					" privileges query returns",
			},
			{Name: "options", Desc: "the user and the host the link connects as"},
			{Name: "comment", Desc: "always absent: Oracle records no comment on a link"},
		},
		Params: []dbmeta.Param{
			{Name: "name", Desc: "link name pattern, empty for every one", Default: ""},
			{Name: "with_system", Desc: "include the schemas Oracle keeps for itself", Default: false},
		},
		Scan: func(rows *sql.Rows) (dbmeta.ForeignServer, error) {
			var v dbmeta.ForeignServer
			err := rows.Scan(&v.Name, &v.Owner, &v.Wrapper, &v.Type, &v.Version,
				&v.Access, &v.Options, &v.Comment)
			return v, err
		},
	})

	// \det. An external table reads a file through a directory object, which
	// is the Oracle way to select from something the database does not hold.
	dbmeta.ForeignTables.Register(dbmeta.Oracle, &dbmeta.Binding[dbmeta.ForeignTable]{
		Stmt: dbmeta.Stmt{
			always(`SELECT e.owner AS "schema"`),
			always(`, e.table_name AS "name"`),
			always(`, NVL(e.default_directory_name, '') AS "server"`),
			// access_parameters is a CLOB. TO_CHAR on a CLOB past 4000 bytes
			// raises ORA-22835, so the read is bounded rather than left to
			// fail on a long one.
			always(`, DBMS_LOB.SUBSTR(e.access_parameters, 4000, 1) AS "options"`),
			always(`, m.comments AS "comment"`),
			always(`FROM all_external_tables e`),
			always(`LEFT JOIN all_tab_comments m ON m.owner = e.owner` +
				` AND m.table_name = e.table_name`),
			always(`WHERE ` + notSystem("e.owner")),
			always(`AND (@schema IS NULL OR e.owner LIKE @schema)`),
			always(`AND (@name IS NULL OR e.table_name LIKE @name)`),
			always(`ORDER BY e.owner, e.table_name`),
		},
		Fields: []dbmeta.Field{
			{Name: "schema"}, {Name: "name"},
			{
				Name: "server",
				Desc: "the directory object the table reads through, which is" +
					" the nearest thing Oracle has to a foreign server",
			},
			{Name: "options", Desc: "the access parameters, cut at 4000 bytes"},
			{Name: "comment"},
		},
		Params: schemaNameSystem("table"),
		Scan: func(rows *sql.Rows) (dbmeta.ForeignTable, error) {
			var v dbmeta.ForeignTable
			err := rows.Scan(&v.Schema, &v.Name, &v.Server, &v.Options, &v.Comment)
			return v, err
		},
	})
}
