// Package mysql holds the metadata queries for MariaDB and MySQL.
//
// Import it for its effect:
//
//	import _ "github.com/xo/dbmeta/models/mysql"
//
// # One driver, two products
//
// D14 makes a driver a family rather than a product. MariaDB is the reference
// these queries are written and tested against, and MySQL must also work. The
// two share information_schema for most of what they report and diverge at the
// edges, so a fragment gated on a version is gated on the MariaDB version
// unless it says otherwise.
//
// # What MariaDB answers with something of its own
//
// This is a native model rather than a shared one, because MariaDB records
// several things information_schema has no view for, and answers several of
// psql's object kinds with an analogue:
//
//   - a storage engine is the closest thing to an access method, in ENGINES
//   - a plugin is the closest thing to an extension, in ALL_PLUGINS
//   - a comment lives on the object, in TABLES.TABLE_COMMENT and
//     COLUMNS.COLUMN_COMMENT, rather than in a catalog of comments
//   - indexes live in STATISTICS, which the standard does not define at all
//   - roles and their grants live in mysql.user and mysql.roles_mapping
//   - an aggregate is a row in mysql.proc or mysql.func, not a kind of routine
//   - a foreign server is a row in mysql.servers, and it carries the one
//     credential every local user reaches it with
//   - a foreign table is a table on an engine that reads remote data
//
// This model answers 30 of the 65 questions on MariaDB and 27 on MySQL. Six
// of them on MariaDB, and four on MySQL, read the mysql schema and need
// SELECT on it, because neither product publishes those tables through
// information_schema.
//
// # Two products, one dialect
//
// A fragment that belongs to one product gates on that product's version key,
// [MariaDB] or [MySQL], and never on the number alone. MariaDB is at 11.8 and
// MySQL at 9, and neither number says anything about the other, so a gate at
// 10.2 quietly means "MariaDB only" and answers wrongly for MySQL. See D44.
//
// A schema and a database are the same thing here, which is the one place the
// object model does not fit. See docs/COVERAGE.md for what it cannot answer.
package mysql

import (
	"database/sql"
	"strings"

	"github.com/xo/dbmeta"
)

// Reference is the MariaDB release these queries were written against.
const Reference = "11.8.9-MariaDB"

// Version keys, one per product.
//
// MariaDB and MySQL share this dialect and this model, and their release
// numbers have no relation to each other: MariaDB is at 11.8 while MySQL is at
// 9.x, and a feature in one says nothing about the other. So a fragment that
// belongs to one product gates on that product's key rather than on the number
// alone. [parseVersion] records the version under the key of the product it
// found, and under no other, so a gate on the key the server did not report
// never applies. See D44.
const (
	// MariaDB is the version key a MariaDB server reports under.
	MariaDB = "mariadb"
	// MySQL is the version key a MySQL server reports under.
	MySQL = "mysql"
)

// Releases a fragment gates on, per product.
var (
	// MariaDB recorded a check constraint from 10.2 and grew the view that
	// lists sequences in 11.5.
	mariaCheck = dbmeta.Gate{Key: MariaDB, Min: dbmeta.V(10, 2)}
	mariaSeq   = dbmeta.Gate{Key: MariaDB, Min: dbmeta.V(11, 5)}
	mariaAgg   = dbmeta.Gate{Key: MariaDB, Min: dbmeta.V(10, 3)}
	// MySQL recorded a check constraint from 8.0.16. It has no sequences at
	// any release and no aggregate of its own.
	mysqlCheck = dbmeta.Gate{Key: MySQL, Min: dbmeta.V(8, 0, 16)}
	// performance_schema.variables_metadata arrived in MySQL 9. Below it the
	// server publishes a variable's value and neither its type nor its scope.
	mysqlVarMeta = dbmeta.Gate{Key: MySQL, Min: dbmeta.V(9)}

	// onMaria and onMySQL say "this product, any release". Use them where the
	// two products spell the same thing differently and both have always
	// spelled it their own way.
	onMaria = dbmeta.Gate{Key: MariaDB}
	onMySQL = dbmeta.Gate{Key: MySQL}
)

// frag returns a fragment that applies when the server meets g.
func frag(g dbmeta.Gate, query string) dbmeta.Fragment {
	return dbmeta.Fragment{Min: g.Min, Key: g.Key, Query: query}
}

func init() {
	dbmeta.RegisterDialect(dbmeta.MySQL, &dbmeta.Info{
		// The syntax is usql's lexer flags for this product, and the fold
		// is measured by scanEveryQuery (D143).
		Syntax:      dbmeta.Syntax{BlockComments: true, HashComments: true, Backticks: true},
		Placeholder: func(int) string { return "?" },
		// usql scans each column into the type the driver reports, and tidb,
		// vitess and singlestore copy this Info (D230).
		ScanTypes:      true,
		VersionQuery:   `SELECT VERSION()`,
		VersionColumns: 1,
		ParseVersion:   parseVersion,

		QuotingQuery:   `SELECT @@sql_mode`,
		QuotingColumns: 1,
		ParseQuoting:   parseQuoting,
		ChangePassword: changePassword,
	})
	registerRelations()
	registerRoutines()
	registerServer()
	registerForeign()
	registerExtra()
}

// parseVersion reads what SELECT VERSION() returns, such as "11.8.9-MariaDB"
// or "8.0.36".
//
// The suffix is where the flavor shows. MariaDB appends its own name and MySQL
// does not. D14 records a flavor rather than ranking it, so the suffix is kept
// and never compared.
func parseVersion(cols []string) (dbmeta.VersionSet, error) {
	if len(cols) == 0 {
		return dbmeta.VersionSet{}, dbmeta.ErrInvalidVersion
	}
	raw := strings.TrimSpace(cols[0])
	ver := dbmeta.ParseVersion(raw)
	ver.Raw = raw
	product := "MySQL"
	if strings.Contains(strings.ToLower(ver.Suffix), "mariadb") {
		product = "MariaDB"
	}
	var set dbmeta.VersionSet
	set.Set("", ver)
	// The product gets its own key, and only the product that was found. A
	// fragment naming the other key then cannot apply, whatever the numbers
	// say. Never set a key for a product this did not detect.
	set.Set(strings.ToLower(product), ver)
	set.Display = product + " " + raw
	return set, nil
}

// IsMariaDB reports whether a version set came from MariaDB rather than MySQL.
// A caller narrowing behavior by product reads this. A fragment does not: it
// gates on the [MariaDB] key instead.
func IsMariaDB(versions dbmeta.VersionSet) bool {
	return versions.Has(MariaDB)
}

// systemSchemas are the schemas MariaDB keeps for itself.
const systemSchemas = `'mysql', 'information_schema', 'performance_schema', 'sys'`

// TiDB is the version key the TiDB model sets. The TiDB model shares these
// statements, and the one thing in them that differs on TiDB is the list of
// schemas the server keeps for itself, so the filter carries an alternative
// under this key. No server this model reads reports it. See D133.
const TiDB = "tidb"

// tidbSystemSchemas are the schemas TiDB keeps for itself. TiDB spells three
// of them in capitals and compares a schema name with the case it has, so
// the list is written as TiDB spells it, and METRICS_SCHEMA is TiDB's own.
const tidbSystemSchemas = `'mysql', 'INFORMATION_SCHEMA', 'PERFORMANCE_SCHEMA', 'METRICS_SCHEMA', 'sys'`

// Vitess is the version key the Vitess model sets, for the same reason as
// [TiDB]: the Vitess model shares these statements, and Vitess keeps one more
// schema for itself, _vt. See D135.
const Vitess = "vitess"

// vitessSystemSchemas are the schemas Vitess keeps for itself: MySQL's, and
// _vt, where Vitess keeps its own state.
const vitessSystemSchemas = systemSchemas + `, '_vt'`

// MemSQL is the version key the SingleStore model sets, for the same reason
// as [TiDB]: the SingleStore model shares these statements, and SingleStore
// keeps its own schemas. See D141.
const MemSQL = "memsql"

// memsqlSystemSchemas are the schemas SingleStore keeps for itself.
const memsqlSystemSchemas = `'information_schema', 'memsql', 'cluster'`

// notSystem is the filter that hides the schemas the server keeps for
// itself unless the caller asks for them, for the column col, after the
// keyword given.
func notSystem(keyword, col string) dbmeta.Choice {
	return dbmeta.Choice{
		{Query: keyword + ` (@with_system OR ` + col + ` NOT IN (` + systemSchemas + `))`},
		{Key: TiDB, Query: keyword + ` (@with_system OR ` + col + ` NOT IN (` + tidbSystemSchemas + `))`},
		{Key: Vitess, Query: keyword + ` (@with_system OR ` + col + ` NOT IN (` + vitessSystemSchemas + `))`},
		{Key: MemSQL, Query: keyword + ` (@with_system OR ` + col + ` NOT IN (` + memsqlSystemSchemas + `))`},
	}
}

// Keyspace is the expression that names the keyspace whose shard stores the
// database col. Vitess stores a keyspace in one MySQL database for each
// shard, named vt_, the keyspace, an underscore and the shard, such as
// vt_dbmeta_0 or vt_dbmeta_-80, and information_schema names that database.
// vtgate refuses that name in a query and accepts the keyspace, so the model
// reports the keyspace. A database that does not have that form is its own
// name, such as mysql or _vt. The Vitess model uses it in the statements of
// its own. See D135.
func Keyspace(col string) string {
	return `CASE WHEN LEFT(` + col + `, 3) = 'vt_' AND LOCATE('_', ` + col + `, 4) > 0` +
		` THEN SUBSTRING(` + col + `, 4, CHAR_LENGTH(` + col + `) - 4` +
		` - CHAR_LENGTH(SUBSTRING_INDEX(` + col + `, '_', -1)))` +
		` ELSE ` + col + ` END`
}

// schemaAs selects the schema in col as the column alias, after the prefix
// given, which is SELECT or a comma. On Vitess it selects the keyspace.
func schemaAs(prefix, col, alias string) dbmeta.Choice {
	return dbmeta.Choice{
		{Query: prefix + col + ` AS "` + alias + `"`},
		{Key: Vitess, Query: prefix + Keyspace(col) + ` AS "` + alias + `"`},
	}
}

// schemaLike is the filter that matches the schema in col to the pattern in
// the parameter param. On Vitess it matches the keyspace.
func schemaLike(param, col string) dbmeta.Choice {
	return dbmeta.Choice{
		{Query: `AND (@` + param + ` = '' OR ` + col + ` LIKE @` + param + `)`},
		{Key: Vitess, Query: `AND (@` + param + ` = '' OR ` + Keyspace(col) + ` LIKE @` + param + `)`},
	}
}

// tableComment is the comment of the table t. MySQL and MariaDB write an
// empty string for no comment, which is turned back into NULL, and the word
// VIEW for every view, which cannot have a comment in either product.
const tableComment = `CASE WHEN t.table_type = 'VIEW' THEN NULL ELSE NULLIF(t.table_comment, '') END`

// tableType is the word for the kind of the table t.
const tableType = `CASE t.table_type WHEN 'BASE TABLE' THEN 'table'` +
	` WHEN 'VIEW' THEN 'view'` +
	` WHEN 'SEQUENCE' THEN 'sequence'` +
	` WHEN 'SYSTEM VIEW' THEN 'view'` +
	` ELSE LOWER(t.table_type) END`

func fields(names ...string) []dbmeta.Field { return dbmeta.Fields(names...) }

func schemaNameSystem(kind string) []dbmeta.Param {
	return []dbmeta.Param{
		{Name: "schema", Desc: "schema name pattern, empty for every schema", Default: ""},
		{Name: "name", Desc: kind + " name pattern, empty for every " + kind, Default: ""},
		{Name: "with_system", Desc: "include the schemas MariaDB keeps for itself", Default: false},
	}
}

// schemaParentName are the parameters of a kind whose objects belong to a
// table, such as a column.
func schemaParentName(kind string) []dbmeta.Param {
	return schemaParentOf("table", kind)
}

// schemaParentOf are the parameters of a kind whose objects belong to the
// object parent names, such as a parameter to its routine.
func schemaParentOf(parent, kind string) []dbmeta.Param {
	return append([]dbmeta.Param{
		{Name: "parent", Desc: parent + " name pattern, empty for every " + parent, Default: ""},
	}, schemaNameSystem(kind)...)
}

func nameSystem(kind string) []dbmeta.Param {
	return []dbmeta.Param{
		{Name: "name", Desc: kind + " name pattern, empty for every " + kind, Default: ""},
		{Name: "with_system", Desc: "include what MariaDB keeps for itself", Default: false},
	}
}

// parseQuoting reads sql_mode.
//
// A backslash escapes unless NO_BACKSLASH_ESCAPES is set, and it is not set by
// default on either product. This is the setting that makes quote doubling
// alone wrong here: with the password x\\ and only the quote doubled, the
// backslash escapes the closing quote and the literal runs on into whatever
// follows the statement.
func parseQuoting(cols []string) (dbmeta.Quoting, error) {
	if len(cols) == 0 {
		return dbmeta.Quoting{}, dbmeta.ErrQuotingUnknown
	}
	plain := !strings.Contains(strings.ToUpper(cols[0]), "NO_BACKSLASH_ESCAPES")
	return dbmeta.Quoting{
		BackslashEscapes: sql.Null[bool]{V: plain, Valid: true},
	}, nil
}

// changePassword builds ALTER USER ... IDENTIFIED BY.
//
// An account here is a user and a host rather than a name, and both halves are
// string literals rather than identifiers. The name is split on the last @,
// because a user name can contain one and a host cannot. A name with no @ is
// written without a host, which the server reads as any host, and that is the
// server's rule rather than a guess made here.
//
// usql cannot change a password on this product at all: its mysql driver
// declares no ChangePassword. This is the one statement here that is an
// addition rather than a move. See D56.
func changePassword(c dbmeta.PasswordChange, q dbmeta.Quoting) (string, error) {
	if !q.BackslashEscapes.Valid {
		return "", dbmeta.ErrQuotingUnknown
	}
	account := dbmeta.QuoteLiteral(c.User, q)
	if at := strings.LastIndex(c.User, "@"); at >= 0 {
		account = dbmeta.QuoteLiteral(c.User[:at], q) + "@" +
			dbmeta.QuoteLiteral(c.User[at+1:], q)
	}
	return "ALTER USER " + account +
		" IDENTIFIED BY " + dbmeta.QuoteLiteral(c.Password, q), nil
}
