package oracle

import (
	"database/sql"

	"github.com/xo/dbmeta"
)

func registerCatalog() {
	// \dp. Oracle records one row per grant and PostgreSQL keeps one list per
	// object, so the rows are folded back into a list.
	objectTypeOf := `SELECT MAX(o.object_type) FROM all_objects o` +
		` WHERE o.owner = p.table_schema AND o.object_name = p.table_name` +
		` AND o.object_type NOT IN ('INDEX', 'LOB', 'TABLE PARTITION',` +
		` 'INDEX PARTITION', 'TABLE SUBPARTITION')`
	dbmeta.Privileges.Register(dbmeta.Oracle, &dbmeta.Binding[dbmeta.Privilege]{
		Stmt: dbmeta.Stmt{
			always(`SELECT p.table_schema AS "schema"`),
			always(`, p.table_name AS "name"`),
			// An index lives in a namespace of its own, so a table and an
			// index can share a name in one schema. Joining all_objects
			// then doubles every grant, and the subquery cannot.
			//
			// all_objects checks a privilege for every row, and 11g takes
			// minutes over the whole of it. From 12c all_tab_privs names the
			// type itself, and says UNKNOWN for a domain, a consumer group, a
			// job class and an evaluation context, which only all_objects
			// names. So the subquery runs for those alone. See D150.
			dbmeta.Choice{
				{Query: `, NVL((` + objectTypeOf + `), '') AS "type"`},
				{Min: v18, Query: `, CASE WHEN MAX(p.type) <> 'UNKNOWN' THEN MAX(p.type)` +
					` ELSE NVL((` + objectTypeOf + `), '') END AS "type"`},
			},
			always(`, `),
			listagg("p.grantee || '=' || p.privilege", "p.grantee, p.privilege"),
			always(`  AS "access"`),
			always(`, NULL AS "column_access"`),
			always(`, NULL AS "policies"`),
			always(`FROM all_tab_privs p`),
			notSystem("WHERE", "p.table_schema"),
			always(`AND (@schema IS NULL OR p.table_schema LIKE @schema)`),
			always(`AND (@name IS NULL OR p.table_name LIKE @name)`),
			always(`GROUP BY p.table_schema, p.table_name`),
			always(`ORDER BY p.table_schema, p.table_name`),
		},
		Fields: []dbmeta.Field{
			{Name: "schema"}, {Name: "name"},
			{Name: "type", Desc: "the object type, read from all_tab_privs, and from all_objects where all_tab_privs has no name for it or on 11g"},
			{Name: "access", Desc: "the grants as grantee=privilege, one list per object"},
			{
				Name: "column_access",
				Desc: "always absent: Oracle keeps a column grant in" +
					" all_col_privs, which is a second statement",
			},
			{
				Name: "policies",
				Desc: "always absent: an Oracle row level policy is a" +
					" DBMS_RLS object rather than a grant",
			},
		},
		Params: schemaNameSystem("object"),
		Scan: func(rows *sql.Rows) (dbmeta.Privilege, error) {
			var v dbmeta.Privilege
			err := rows.Scan(&v.Schema, &v.Name, dbmeta.NullAsEmpty(&v.Type), &v.Access,
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
			notSystem("WHERE", "l.owner"),
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
			notSystem("WHERE", "e.owner"),
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
			err := rows.Scan(&v.Schema, &v.Name, dbmeta.NullAsEmpty(&v.Server), &v.Options, &v.Comment)
			return v, err
		},
	})
}

// registerGrants lists the privileges on objects and the privileges on columns, one for each
// row. The privileges of the owner of an object are not rows. A column privilege is in
// ALL_COL_PRIVS, which holds no object type, so the type is looked up in ALL_OBJECTS for those
// rows alone. See D298.
func registerGrants() {
	grantee := `, CASE WHEN p.grantee = 'PUBLIC' THEN '' ELSE p.grantee END AS "grantee"` +
		`, CASE WHEN p.grantee = 'PUBLIC' THEN NULL WHEN EXISTS (SELECT 1 FROM all_users u WHERE u.username = p.grantee)` +
		` THEN 'user' ELSE 'role' END AS "grantee_type"`
	typeOf := `NVL((SELECT LOWER(o.object_type) FROM all_objects o WHERE o.owner = p.table_schema` +
		` AND o.object_name = p.table_name AND o.object_type NOT IN ('INDEX', 'PACKAGE BODY', 'TYPE BODY') AND ROWNUM = 1), 'table')`
	stmt := dbmeta.Stmt{
		always(`SELECT p.table_schema AS "schema", p.table_name AS "object"`),
		dbmeta.Choice{
			{Query: `, ` + typeOf + ` AS "object_type"`},
			{Min: v18, Query: `, CASE WHEN p.type <> 'UNKNOWN' THEN LOWER(p.type) ELSE ` + typeOf + ` END AS "object_type"`},
		},
		always(`, CAST(NULL AS VARCHAR2(1)) AS "arguments", CAST(NULL AS VARCHAR2(128)) AS "column"`),
		always(grantee),
		always(`, p.privilege AS "privilege", CASE WHEN p.grantable = 'YES' THEN 1 ELSE 0 END AS "grantable", 0 AS "denied"`),
		always(`FROM all_tab_privs p`),
		notSystem("WHERE", "p.table_schema"),
		always(`AND p.grantee <> p.table_schema AND p.privilege <> 'INHERIT PRIVILEGES'`),
		always(`AND (@schema IS NULL OR p.table_schema LIKE @schema)`),
		always(`UNION ALL SELECT p.table_schema, p.table_name, ` + typeOf + `, CAST(NULL AS VARCHAR2(1)), p.column_name`),
		always(grantee),
		always(`, p.privilege, CASE WHEN p.grantable = 'YES' THEN 1 ELSE 0 END, 0`),
		always(`FROM all_col_privs p`),
		notSystem("WHERE", "p.table_schema"),
		always(`AND p.grantee <> p.table_schema`),
		always(`AND (@schema IS NULL OR p.table_schema LIKE @schema)`),
		always(`ORDER BY 1, 2, 3, 6, 8`),
	}
	dbmeta.Grants.Register(dbmeta.Oracle, &dbmeta.Binding[dbmeta.Grant]{
		Stmt: stmt,
		Fields: []dbmeta.Field{
			{Name: "schema"}, {Name: "object"}, {Name: "object_type"},
			{Name: "arguments", Desc: "always absent: a routine of a package is one object"},
			{Name: "column"}, {Name: "grantee", Desc: "the user or the role, and empty for PUBLIC"},
			{Name: "grantee_type", Desc: "user or role, and absent for PUBLIC"},
			{Name: "privilege"}, {Name: "grantable", Desc: "true for WITH GRANT OPTION"},
			{Name: "denied", Desc: "always false: Oracle has no entry that takes a privilege away"},
		},
		Params: schemaNameSystem("object"),
		Scan: func(rows *sql.Rows) (dbmeta.Grant, error) {
			var v dbmeta.Grant
			err := rows.Scan(&v.Schema, &v.Object, &v.ObjectType, &v.Arguments, &v.Column, dbmeta.NullAsEmpty(&v.Grantee),
				&v.GranteeType, &v.Privilege, &v.Grantable, &v.Denied)
			return v, err
		},
	})
}
