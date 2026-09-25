// Package postgres holds the metadata queries for PostgreSQL.
//
// Import it for its effect. It registers what PostgreSQL provides, and the
// root package answers for it afterwards:
//
//	import _ "github.com/xo/dbmeta/models/postgres"
//
// Every query here is translated from src/bin/psql/describe.c in the
// PostgreSQL source. Each one records which command it backs and which tree it
// was translated from, because a query for a release below 10 comes from an
// older checkout. See docs/QUERIES.md.
package postgres

import (
	"database/sql"
	"strconv"
	"strings"

	"github.com/xo/dbmeta"
)

// Tree is the PostgreSQL checkout every query below was translated from.
//
// Release 20 removed the psql code paths for servers below release 10, so a
// fragment for an older release cannot come from this tree. Any such fragment
// names its own source beside it.
const Tree = "REL_19_BETA1-1062-gd9de60c5e47"

// Release versions that a fragment gates on.
var (
	v10 = dbmeta.V(10)
	v11 = dbmeta.V(11)
	v12 = dbmeta.V(12)
	v13 = dbmeta.V(13)
	v15 = dbmeta.V(15)
	v16 = dbmeta.V(16)
	v17 = dbmeta.V(17)
)

func init() {
	dbmeta.RegisterDialect(dbmeta.PostgreSQL, &dbmeta.Info{
		Placeholder:    func(n int) string { return "$" + strconv.Itoa(n) },
		VersionQuery:   `SHOW server_version`,
		VersionColumns: 1,
		ParseVersion:   parseVersion,

		QuotingQuery:   `SHOW standard_conforming_strings`,
		QuotingColumns: 1,
		ParseQuoting:   parseQuoting,
		ChangePassword: changePassword,
	})
	registerSchemas()
	registerTables()
	registerColumns()
	registerExtra()
}

// parseVersion reads what SHOW server_version returns, such as "16.2" or
// "16.2 (Debian 16.2-1.pgdg120+2)".
func parseVersion(cols []string) (dbmeta.VersionSet, error) {
	if len(cols) == 0 {
		return dbmeta.VersionSet{}, dbmeta.ErrInvalidVersion
	}
	raw := strings.TrimSpace(cols[0])
	var set dbmeta.VersionSet
	// a packaged build appends its own detail in brackets, which is not part
	// of the version
	ver := dbmeta.ParseVersion(raw)
	ver.Raw = raw
	if i := strings.IndexByte(ver.Suffix, '('); i >= 0 {
		ver.Suffix = strings.TrimSpace(ver.Suffix[:i])
	}
	set.Set("", ver)
	set.Display = "PostgreSQL " + raw
	return set, nil
}

// registerSchemas backs \dn.
//
// Translated from listSchemas. It carries no version gate below release 15,
// and the gate at 15 covers publication membership, which this query does not
// return, so the statement is the same for every supported release.
func registerSchemas() {
	dbmeta.Schemas.Register(dbmeta.PostgreSQL, &dbmeta.Binding[dbmeta.Schema]{
		Stmt: dbmeta.Stmt{
			{{Query: `SELECT current_database() AS "catalog"`}},
			{{Query: `, n.nspname AS "name"`}},
			{{Query: `, pg_catalog.pg_get_userbyid(n.nspowner) AS "owner"`}},
			{{Query: `, pg_catalog.obj_description(n.oid, 'pg_namespace') AS "comment"`}},
			{{Query: `FROM pg_catalog.pg_namespace n`}},
			{{Query: `WHERE (@with_system OR (n.nspname !~ '^pg_' AND n.nspname <> 'information_schema'))`}},
			{{Query: `AND (@name = '' OR n.nspname LIKE @name)`}},
			{{Query: `ORDER BY 2`}},
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "database the schema belongs to"},
			{Name: "name", Desc: "schema name"},
			{Name: "owner", Desc: "role that owns the schema"},
			{Name: "comment", Desc: "comment on the schema"},
		},
		Params: []dbmeta.Param{
			{Name: "name", Desc: "schema name pattern, empty for every schema", Default: ""},
			{Name: "with_system", Desc: "include the schemas PostgreSQL keeps for itself", Default: false},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Schema, error) {
			var s dbmeta.Schema
			err := rows.Scan(&s.Catalog, &s.Name, &s.Owner, &s.Comment)
			return s, err
		},
	})
}

// registerTables backs \dt, \dv, \dm and \ds.
//
// Translated from listTables. The relation kinds follow pg_class.relkind: r is
// an ordinary table, p a partitioned table, v a view, m a materialized view, S
// a sequence, f a foreign table.
func registerTables() {
	dbmeta.Tables.Register(dbmeta.PostgreSQL, &dbmeta.Binding[dbmeta.Table]{
		Stmt: dbmeta.Stmt{
			{{Query: `SELECT current_database() AS "catalog"`}},
			{{Query: `, n.nspname AS "schema"`}},
			{{Query: `, c.relname AS "name"`}},
			{{Query: `, CASE c.relkind` +
				` WHEN 'r' THEN 'table'` +
				` WHEN 'p' THEN 'table'` +
				` WHEN 'v' THEN 'view'` +
				` WHEN 'm' THEN 'materialized view'` +
				` WHEN 'S' THEN 'sequence'` +
				` WHEN 'f' THEN 'foreign table'` +
				` ELSE c.relkind::text END AS "type"`}},
			{{Query: `, pg_catalog.obj_description(c.oid, 'pg_class') AS "comment"`}},
			{{Query: `FROM pg_catalog.pg_class c`}},
			{{Query: `JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace`}},
			{{Query: `WHERE c.relkind IN ('r', 'p', 'v', 'm', 'S', 'f')`}},
			{{Query: `AND (@with_system OR (n.nspname !~ '^pg_' AND n.nspname <> 'information_schema'))`}},
			{{Query: `AND (@schema = '' OR n.nspname LIKE @schema)`}},
			{{Query: `AND (@name = '' OR c.relname LIKE @name)`}},
			{{Query: `ORDER BY 2, 3`}},
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "database the relation belongs to"},
			{Name: "schema", Desc: "schema the relation belongs to"},
			{Name: "name", Desc: "relation name"},
			{Name: "type", Desc: "table, view, materialized view, sequence or foreign table"},
			{Name: "comment", Desc: "comment on the relation"},
		},
		Params: []dbmeta.Param{
			{Name: "schema", Desc: "schema name pattern, empty for every schema", Default: ""},
			{Name: "name", Desc: "relation name pattern, empty for every relation", Default: ""},
			{Name: "with_system", Desc: "include the relations PostgreSQL keeps for itself", Default: false},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Table, error) {
			var t dbmeta.Table
			err := rows.Scan(&t.Catalog, &t.Schema, &t.Name, &t.Type, &t.Comment)
			return t, err
		},
	})
}

// registerColumns backs the column list of \d name.
//
// Translated from describeOneTableDetails, which is the hardest entry point in
// describe.c and carries 21 of its 68 version gates. Only the column list is
// here. The indexes, constraints, triggers and partition detail that \d also
// prints are separate objects and follow later.
//
// Two fragments show the padding rule. The generated column expression arrived
// in release 12, and identity arrived in release 11, so an older server
// selects a literal under the same name and the column set never changes.
func registerColumns() {
	dbmeta.Columns.Register(dbmeta.PostgreSQL, &dbmeta.Binding[dbmeta.Column]{
		Stmt: dbmeta.Stmt{
			{{Query: `SELECT current_database() AS "catalog"`}},
			{{Query: `, n.nspname AS "schema"`}},
			{{Query: `, c.relname AS "table"`}},
			{{Query: `, a.attname AS "name"`}},
			{{Query: `, a.attnum AS "ordinal"`}},
			{{Query: `, pg_catalog.format_type(a.atttypid, a.atttypmod) AS "data_type"`}},
			{{Query: `, NOT a.attnotnull AS "nullable"`}},
			{{Query: `, pg_catalog.pg_get_expr(d.adbin, d.adrelid) AS "default"`}},
			// One more join, to the primary key index of the table. It is an
			// index lookup on the relation and it costs the same whether or
			// not the caller reads the column. See D47.
			{{Query: `, COALESCE(a.attnum = ANY(pk.indkey), FALSE) AS "primary_key"`}},
			// attidentity arrived in release 11
			{
				{Query: `, NULL AS "identity"`},
				{Min: v11, Query: `, a.attidentity AS "identity"`},
			},
			// attgenerated arrived in release 12
			{
				{Query: `, NULL AS "generated"`},
				{Min: v12, Query: `, a.attgenerated AS "generated"`},
			},
			{{Query: `, pg_catalog.col_description(c.oid, a.attnum) AS "comment"`}},
			{{Query: `FROM pg_catalog.pg_attribute a`}},
			{{Query: `JOIN pg_catalog.pg_class c ON c.oid = a.attrelid`}},
			{{Query: `JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace`}},
			{{Query: `LEFT JOIN pg_catalog.pg_attrdef d ON d.adrelid = a.attrelid AND d.adnum = a.attnum`}},
			{{Query: `LEFT JOIN pg_catalog.pg_index pk ON pk.indrelid = a.attrelid AND pk.indisprimary`}},
			{{Query: `WHERE a.attnum > 0 AND NOT a.attisdropped`}},
			{{Query: `AND (@schema = '' OR n.nspname LIKE @schema)`}},
			{{Query: `AND (@parent = '' OR c.relname LIKE @parent)`}},
			{{Query: `AND (@name = '' OR a.attname LIKE @name)`}},
			{{Query: `ORDER BY 2, 3, 5`}},
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "database the column belongs to"},
			{Name: "schema", Desc: "schema the table belongs to"},
			{Name: "table", Desc: "table the column belongs to"},
			{Name: "name", Desc: "column name"},
			{Name: "ordinal", Desc: "position of the column in the table"},
			{Name: "data_type", Desc: "type of the column, as PostgreSQL writes it"},
			{Name: "nullable", Desc: "whether the column accepts NULL"},
			{Name: "default", Desc: "default expression, empty when there is none"},
			{Name: "primary_key", Desc: "whether the column is part of the primary key"},
			{Name: "identity", Desc: "identity kind, empty when the column is not an identity", Min: v11},
			{Name: "generated", Desc: "generated kind, empty when the column is not generated", Min: v12},
			{Name: "comment", Desc: "comment on the column"},
		},
		Params: []dbmeta.Param{
			{Name: "schema", Desc: "schema name pattern, empty for every schema", Default: ""},
			{Name: "parent", Desc: "table name pattern, empty for every table", Default: ""},
			{Name: "name", Desc: "column name pattern, empty for every column", Default: ""},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Column, error) {
			var c dbmeta.Column
			err := rows.Scan(
				&c.Catalog, &c.Schema, &c.Table, &c.Name, &c.Ordinal,
				&c.DataType, &c.Nullable, &c.Default, &c.PrimaryKey,
				&c.Identity, &c.Generated, &c.Comment,
			)
			return c, err
		},
	})
}

// parseQuoting reads standard_conforming_strings.
//
// On means a backslash is an ordinary character, which is the default since
// release 9.1 and is what almost every server reports. Off means a backslash
// escapes, and then a password carrying one has to have it doubled.
func parseQuoting(cols []string) (dbmeta.Quoting, error) {
	if len(cols) == 0 {
		return dbmeta.Quoting{}, dbmeta.ErrQuotingUnknown
	}
	off := !strings.EqualFold(strings.TrimSpace(cols[0]), "on")
	return dbmeta.Quoting{
		BackslashEscapes: sql.Null[bool]{V: off, Valid: true},
	}, nil
}

// changePassword builds ALTER USER ... PASSWORD.
//
// PostgreSQL takes the password as a string literal and the role as an
// identifier, so the two are quoted by different rules. It has no old password
// clause and ignores PasswordChange.Old.
func changePassword(c dbmeta.PasswordChange, q dbmeta.Quoting) (string, error) {
	if !q.BackslashEscapes.Valid {
		return "", dbmeta.ErrQuotingUnknown
	}
	return "ALTER USER " + dbmeta.QuoteIdentifier(c.User, `"`, `"`) +
		" PASSWORD " + dbmeta.QuoteLiteral(c.Password, q), nil
}
