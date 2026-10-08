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
	v14 = dbmeta.V(14)
	v15 = dbmeta.V(15)
	v16 = dbmeta.V(16)
	v17 = dbmeta.V(17)
)

func init() {
	dbmeta.RegisterDialect(dbmeta.PostgreSQL, &dbmeta.Info{
		// The syntax is usql's lexer flags for this product, and the fold
		// is measured by scanEveryQuery (D143).
		Syntax:         dbmeta.Syntax{DollarQuotes: true, BlockComments: true},
		Fold:           dbmeta.FoldLower,
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
			{{Query: `, pg_catalog.array_to_string(n.nspacl, E'\n') AS "access"`}},
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
			{Name: "access", Desc: "access privileges, one per line, absent for the default"},
		},
		Params: []dbmeta.Param{
			{Name: "name", Desc: "schema name pattern, empty for every schema", Default: ""},
			{Name: "with_system", Desc: "include the schemas PostgreSQL keeps for itself", Default: false},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Schema, error) {
			var s dbmeta.Schema
			err := rows.Scan(&s.Catalog, &s.Name, &s.Owner, &s.Comment, &s.Access)
			return s, err
		},
	})
}

// relationType is the word for a relation's kind, from pg_class.relkind.
const relationType = `CASE c.relkind` +
	` WHEN 'r' THEN 'table'` +
	` WHEN 'p' THEN 'partitioned table'` +
	` WHEN 'v' THEN 'view'` +
	` WHEN 'm' THEN 'materialized view'` +
	` WHEN 'S' THEN 'sequence'` +
	` WHEN 'f' THEN 'foreign table'` +
	` WHEN 'c' THEN 'composite type'` +
	` ELSE c.relkind::text END`

// persistence is the word for a relation's persistence, from
// pg_class.relpersistence, as psql spells it.
const persistence = `CASE c.relpersistence WHEN 'p' THEN 'permanent'` +
	` WHEN 'u' THEN 'unlogged' WHEN 't' THEN 'temporary'` +
	` ELSE c.relpersistence::text END`

// registerTables backs \dt, \dv, \dm and \ds.
//
// Translated from listTables. The relation kinds follow pg_class.relkind: r is
// an ordinary table, p a partitioned table, v a view, m a materialized view, S
// a sequence, f a foreign table.
//
// Size is pg_table_size, which is what \dt+ prints, and the access method
// arrived in release 12. See D198.
func registerTables() {
	dbmeta.Tables.Register(dbmeta.PostgreSQL, &dbmeta.Binding[dbmeta.Table]{
		Stmt: dbmeta.Stmt{
			{{Query: `SELECT current_database() AS "catalog"`}},
			{{Query: `, n.nspname AS "schema"`}},
			{{Query: `, c.relname AS "name"`}},
			{{Query: `, ` + relationType + ` AS "type"`}},
			{{Query: `, pg_catalog.obj_description(c.oid, 'pg_class') AS "comment"`}},
			{{Query: `, pg_catalog.pg_get_userbyid(c.relowner) AS "owner"`}},
			{{Query: `, ` + persistence + ` AS "persistence"`}},
			// relam holds the table access method from release 12
			{
				{Query: `, NULL AS "access_method"`},
				{Min: v12, Query: `, am.amname AS "access_method"`},
			},
			sizeOf("c.oid"),
			{{Query: `, c.reltuples::bigint AS "rows"`}},
			// The storage parameters of the TOAST table come after the
			// table's own, with a prefix, as psql prints them. An empty list
			// is none.
			{{Query: `, NULLIF(pg_catalog.array_to_string(c.reloptions ||` +
				` ARRAY(SELECT 'toast.' || x FROM pg_catalog.unnest(tc.reloptions) x), ', '), '') AS "options"`}},
			{{Query: `, c.relrowsecurity AS "row_security"`}},
			{{Query: `, c.relforcerowsecurity AS "row_security_forced"`}},
			{{Query: `FROM pg_catalog.pg_class c`}},
			{{Query: `JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace`}},
			{{Query: `LEFT JOIN pg_catalog.pg_class tc ON tc.oid = c.reltoastrelid`}},
			{
				{Query: ``},
				{Min: v12, Query: `LEFT JOIN pg_catalog.pg_am am ON am.oid = c.relam`},
			},
			// A composite type is not listed unless the caller names it in
			// types, because psql lists none and describes one by name.
			{{Query: `WHERE (c.relkind IN ('r', 'p', 'v', 'm', 'S', 'f') OR (c.relkind = 'c' AND @types <> ''))`}},
			{{Query: `AND (@with_system OR (n.nspname !~ '^pg_' AND n.nspname <> 'information_schema'))`}},
			{{Query: `AND (@schema = '' OR n.nspname LIKE @schema)`}},
			{{Query: `AND (@name = '' OR c.relname LIKE @name)`}},
			{{Query: `AND (@types = '' OR ` + dbmeta.InList(`CAST(@types AS text)`, relationType) + `)`}},
			{{Query: `ORDER BY 2, 3`}},
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "database the relation belongs to"},
			{Name: "schema", Desc: "schema the relation belongs to"},
			{Name: "name", Desc: "relation name"},
			{Name: "type", Desc: "table, partitioned table, view, materialized view, sequence or foreign table"},
			{Name: "comment", Desc: "comment on the relation"},
			{Name: "owner", Desc: "role that owns the relation"},
			{Name: "persistence", Desc: "permanent, unlogged or temporary"},
			{Name: "access_method", Desc: "table access method, absent for a view and below release 12", Min: v12},
			{Name: "size", Desc: "bytes on disk, as pg_table_size counts them"},
			{Name: "rows", Desc: "the planner's estimate of the rows, which is -1 before the first analyze from release 14 and 0 before it"},
			{Name: "options", Desc: "storage parameters, such as fillfactor=70, absent when none is set"},
			{Name: "row_security", Desc: "whether row level security is on"},
			{Name: "row_security_forced", Desc: "whether row level security applies to the owner too"},
		},
		Params: []dbmeta.Param{
			{Name: "schema", Desc: "schema name pattern, empty for every schema", Default: ""},
			{Name: "name", Desc: "relation name pattern, empty for every relation", Default: ""},
			{Name: "with_system", Desc: "include the relations PostgreSQL keeps for itself", Default: false},
			dbmeta.TypesParam(),
		},
		Scan: func(rows *sql.Rows) (dbmeta.Table, error) {
			var t dbmeta.Table
			err := rows.Scan(&t.Catalog, &t.Schema, &t.Name, &t.Type, &t.Comment,
				&t.Owner, &t.Persistence, &t.AccessMethod, &t.Size, &t.Rows,
				&t.Options, &t.RowSecurity, &t.RowSecurityForced)
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
			// attcollation is zero for a type that cannot be collated.
			{{Query: `, co.collname AS "collation"`}},
			{{Query: `, CASE a.attstorage WHEN 'p' THEN 'plain' WHEN 'm' THEN 'main'` +
				` WHEN 'e' THEN 'external' WHEN 'x' THEN 'extended'` +
				` ELSE a.attstorage::text END AS "storage"`}},
			// attcompression arrived in release 14 and is empty for the default
			{
				{Query: `, NULL AS "compression"`},
				{Min: v14, Query: `, CASE a.attcompression WHEN 'p' THEN 'pglz' WHEN 'l' THEN 'lz4' END AS "compression"`},
			},
			// attstattarget is -1 for the default until release 17, and NULL from it
			{{Query: `, NULLIF(a.attstattarget, -1)::integer AS "stats_target"`}},
			{{Query: `FROM pg_catalog.pg_attribute a`}},
			{{Query: `JOIN pg_catalog.pg_class c ON c.oid = a.attrelid`}},
			{{Query: `JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace`}},
			{{Query: `LEFT JOIN pg_catalog.pg_attrdef d ON d.adrelid = a.attrelid AND d.adnum = a.attnum`}},
			{{Query: `LEFT JOIN pg_catalog.pg_index pk ON pk.indrelid = a.attrelid AND pk.indisprimary`}},
			{{Query: `LEFT JOIN pg_catalog.pg_collation co ON co.oid = a.attcollation`}},
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
			{Name: "collation", Desc: "the collation of the column, which is default where the column chose none, and absent for a type that cannot be collated"},
			{Name: "storage", Desc: "plain, main, external or extended"},
			{Name: "compression", Desc: "compression method set on the column, absent for the default", Min: v14},
			{Name: "stats_target", Desc: "statistics target set on the column, absent for the default"},
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
				&c.Identity, &c.Generated, &c.Comment, &c.Collation,
				&c.Storage, &c.Compression, &c.StatsTarget,
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

// sizeOf is the bytes on disk of a relation, as pg_table_size counts them. The
// function is in PostgreSQL from 9.0 and in CockroachDB from 26.3, which
// shares this statement, so a CockroachDB before 26.3 reads NULL. The key is
// the one that models/cockroachdb sets as its Release.
func sizeOf(oid string) dbmeta.Choice {
	size := `, pg_catalog.pg_table_size(` + oid + `) AS "size"`
	return dbmeta.Choice{
		{Query: size},
		{Key: "cockroachdb", Query: `, NULL AS "size"`},
		{Key: "cockroachdb", Min: dbmeta.V(26, 3), Query: size},
	}
}
