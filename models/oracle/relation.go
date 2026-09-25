package oracle

import (
	"database/sql"

	"github.com/xo/dbmeta"
)

// The releases a fragment gates on.
var (
	// v12 is where the identity column and search_condition_vc arrived.
	v12 = dbmeta.V(12)
	// v122 is where LISTAGG took an ON OVERFLOW clause.
	v122 = dbmeta.V(12, 2)
	// v18 is where all_views.text_vc arrived.
	v18 = dbmeta.V(18)
	// v23 is where the SQL domain arrived, with it all_domains.
	v23 = dbmeta.V(23)
)

// listagg builds a LISTAGG over a correlated subquery.
//
// LISTAGG returns a VARCHAR2, and a VARCHAR2 holds 4000 bytes. Before 12.2
// going past that raises ORA-01489 and the whole query fails, so a schema
// nobody would call unusual can stop a metadata read. 12.2 added ON OVERFLOW
// TRUNCATE, which cuts the string and appends a count instead. The fragment
// takes it where the release has it. Both sides return one VARCHAR2 column,
// which is what rule 3 asks.
func listagg(expr, order string) dbmeta.Choice {
	return dbmeta.Choice{
		{SQL: `LISTAGG(` + expr + `, ', ') WITHIN GROUP (ORDER BY ` + order + `)`},
		{Min: v122, SQL: `LISTAGG(` + expr + `, ', ' ON OVERFLOW TRUNCATE)` +
			` WITHIN GROUP (ORDER BY ` + order + `)`},
	}
}

// The schemas Oracle keeps for itself.
//
// Oracle has no flag for this. There is no "is this a system user" column, so
// the list is written out, which is what every other tool that reads this
// dictionary does. It is long because an Oracle install creates a lot of
// users that nobody calls a schema.
const systemSchemas = `'SYS', 'SYSTEM', 'SYSAUX', 'OUTLN', 'DBSNMP', 'APPQOSSYS',
	'CTXSYS', 'MDSYS', 'OLAPSYS', 'ORDDATA', 'ORDPLUGINS', 'ORDSYS', 'SI_INFORMTN_SCHEMA',
	'WMSYS', 'XDB', 'ANONYMOUS', 'APEX_PUBLIC_USER', 'FLOWS_FILES', 'LBACSYS',
	'AUDSYS', 'GSMADMIN_INTERNAL', 'GSMCATUSER', 'GSMUSER', 'DIP', 'ORACLE_OCM',
	'REMOTE_SCHEDULER_AGENT', 'SYSBACKUP', 'SYSDG', 'SYSKM', 'SYSRAC', 'SYS$UMF',
	'DVSYS', 'DVF', 'GGSYS', 'PDBADMIN', 'XS$NULL', 'DBSFWUSER', 'MDDATA'`

// notSystem filters the system schemas out unless the caller asks for them.
// The column is named by the caller, because the dictionary spells the owner
// differently from view to view.
func notSystem(col string) string {
	return `(@with_system = 1 OR ` + col + ` NOT IN (` + systemSchemas + `))`
}

// always is a fragment that every release takes.
func always(sqlstr string) dbmeta.Choice { return dbmeta.Choice{{SQL: sqlstr}} }

// schemaNameSystem is the filter set most queries take.
func schemaNameSystem(kind string) []dbmeta.Param {
	return []dbmeta.Param{
		{Name: "schema", Desc: "schema name pattern, empty for every schema", Default: ""},
		{Name: "name", Desc: kind + " name pattern, empty for every one", Default: ""},
		{Name: "with_system", Desc: "include the schemas Oracle keeps for itself", Default: false},
	}
}

func registerRelations() {
	// \dn. A schema is a user in Oracle, so this reads the users.
	dbmeta.Schemas.Register(dbmeta.Oracle, &dbmeta.Binding[dbmeta.Schema]{
		Stmt: dbmeta.Stmt{
			always(`SELECT SYS_CONTEXT('USERENV', 'DB_NAME') AS "catalog"`),
			always(`, u.username AS "name"`),
			// A schema and its owner are the same user, which is what having
			// no schema object separate from the user means.
			always(`, u.username AS "owner"`),
			always(`, NULL AS "comment"`),
			always(`FROM all_users u`),
			always(`WHERE ` + notSystem("u.username")),
			always(`AND (@name IS NULL OR u.username LIKE @name)`),
			always(`ORDER BY u.username`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "the database name: Oracle has one per instance"},
			{Name: "name"},
			{Name: "owner", Desc: "the same as the name: an Oracle schema is a user"},
			{Name: "comment", Desc: "always absent: Oracle records no comment on a user"},
		},
		Params: []dbmeta.Param{
			{Name: "name", Desc: "schema name pattern, empty for every schema", Default: ""},
			{Name: "with_system", Desc: "include the schemas Oracle keeps for itself", Default: false},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Schema, error) {
			var v dbmeta.Schema
			err := rows.Scan(&v.Catalog, &v.Name, &v.Owner, &v.Comment)
			return v, err
		},
	})

	// \dt, \dv and bare \d. Tables and views in one answer, which is the
	// shape psql uses, joined to the comment the dictionary keeps separately.
	dbmeta.Tables.Register(dbmeta.Oracle, &dbmeta.Binding[dbmeta.Table]{
		Stmt: dbmeta.Stmt{
			always(`SELECT SYS_CONTEXT('USERENV', 'DB_NAME') AS "catalog"`),
			always(`, o.owner AS "schema"`),
			always(`, o.object_name AS "name"`),
			always(`, CASE o.object_type WHEN 'VIEW' THEN 'view' ELSE 'table' END AS "type"`),
			always(`, c.comments AS "comment"`),
			always(`FROM all_objects o`),
			always(`LEFT JOIN all_tab_comments c`),
			always(`  ON c.owner = o.owner AND c.table_name = o.object_name`),
			always(`WHERE o.object_type IN ('TABLE', 'VIEW')`),
			// A nested table or an overflow segment is a table to the
			// dictionary and not a table to a person.
			always(`AND o.secondary = 'N'`),
			always(`AND ` + notSystem("o.owner")),
			always(`AND (@schema IS NULL OR o.owner LIKE @schema)`),
			always(`AND (@name IS NULL OR o.object_name LIKE @name)`),
			always(`ORDER BY o.owner, o.object_name`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog"}, {Name: "schema"}, {Name: "name"},
			{Name: "type", Desc: "table or view"},
			{Name: "comment", Desc: "the COMMENT ON TABLE, which Oracle keeps in its own view"},
		},
		Params: schemaNameSystem("table"),
		Scan: func(rows *sql.Rows) (dbmeta.Table, error) {
			var v dbmeta.Table
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Name, &v.Type, &v.Comment)
			return v, err
		},
	})
}

func registerColumns() {
	// \d NAME. The column list, with the primary key flag D47 requires and
	// the comment Oracle keeps in a view of its own.
	dbmeta.Columns.Register(dbmeta.Oracle, &dbmeta.Binding[dbmeta.Column]{
		Stmt: dbmeta.Stmt{
			always(`SELECT SYS_CONTEXT('USERENV', 'DB_NAME') AS "catalog"`),
			always(`, c.owner AS "schema"`),
			always(`, c.table_name AS "table"`),
			always(`, c.column_name AS "name"`),
			always(`, c.column_id AS "ordinal"`),
			always(`, c.data_type AS "data_type"`),
			// Oracle records nullability as a Y or an N rather than a
			// boolean, because it had no boolean type until 23ai.
			always(`, CASE c.nullable WHEN 'Y' THEN 1 ELSE 0 END AS "nullable"`),
			// data_default is a LONG, and a LONG cannot be joined, compared,
			// or wrapped in most functions. It is selected bare and scanned
			// as text, which is the only thing a LONG allows.
			always(`, c.data_default AS "default"`),
			always(`, CASE WHEN k.column_name IS NULL THEN 0 ELSE 1 END AS "primary_key"`),
			// An identity column arrived in 12c. Older releases have no such
			// thing, so the column is padded with NULL rather than a literal
			// and Field.Min says the difference. See docs/NULLS.md.
			dbmeta.Choice{
				{SQL: `, NULL AS "identity"`},
				{Min: v12, SQL: `, NULLIF(c.identity_column, 'NO') AS "identity"`},
			},
			// A virtual column is Oracle's generated column, and it is
			// reported from 11g on.
			always(`, NULLIF(c.virtual_column, 'NO') AS "generated"`),
			always(`, m.comments AS "comment"`),
			always(`FROM all_tab_cols c`),
			always(`LEFT JOIN all_col_comments m`),
			always(`  ON m.owner = c.owner AND m.table_name = c.table_name` +
				` AND m.column_name = c.column_name`),
			// The primary key columns, reached once rather than per row.
			always(`LEFT JOIN (`),
			always(`  SELECT cc.owner, cc.table_name, cc.column_name`),
			always(`  FROM all_cons_columns cc`),
			always(`  JOIN all_constraints k2 ON k2.owner = cc.owner`),
			always(`    AND k2.constraint_name = cc.constraint_name AND k2.constraint_type = 'P'`),
			always(`) k ON k.owner = c.owner AND k.table_name = c.table_name` +
				` AND k.column_name = c.column_name`),
			// A hidden column is one the system made, such as the one behind
			// a function based index. Nobody asked for it and psql shows no
			// equivalent.
			always(`WHERE c.hidden_column = 'NO'`),
			always(`AND ` + notSystem("c.owner")),
			always(`AND (@schema IS NULL OR c.owner LIKE @schema)`),
			always(`AND (@name IS NULL OR c.table_name LIKE @name)`),
			always(`ORDER BY c.owner, c.table_name, c.column_id`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog"}, {Name: "schema"}, {Name: "table"}, {Name: "name"},
			{Name: "ordinal"}, {Name: "data_type"}, {Name: "nullable"},
			{Name: "default", Desc: "the DEFAULT text, which Oracle stores as a LONG"},
			{Name: "primary_key"},
			{
				Name: "identity", Min: v12,
				Desc: "the identity kind, absent before 12c where Oracle had none",
			},
			{Name: "generated", Desc: "set for a virtual column"},
			{Name: "comment", Desc: "the COMMENT ON COLUMN"},
		},
		Params: schemaNameSystem("table"),
		Scan: func(rows *sql.Rows) (dbmeta.Column, error) {
			var v dbmeta.Column
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Table, &v.Name, &v.Ordinal,
				&v.DataType, &v.Nullable, &v.Default, &v.PrimaryKey,
				&v.Identity, &v.Generated, &v.Comment)
			return v, err
		},
	})
}
