// Package surrealdb holds the metadata queries for SurrealDB, in SurrealQL.
//
// Import it for its effect:
//
//	import _ "github.com/xo/dbmeta/models/surrealdb"
//
// # The catalog
//
// SurrealDB has no relational catalog. It records a schema all the same: the
// INFO statement returns the namespaces of the server, the databases of a
// namespace, the tables, functions, sequences and users of a database, and
// the fields, indexes and events of a table. INFO ... STRUCTURE returns each
// of them as an object, and from 3.0 an INFO statement is a value that a
// SELECT reads. So every query here is one statement. A kind below a table
// reads INFO FOR TABLE once for each table inside that statement, through
// array::map, and no kind is a walk (D146). D164 holds the mapping and the
// reasons for it.
//
// A namespace is the catalog and a database is the schema. A table is the
// table, a table defined AS SELECT is a view, and a defined field is a
// column. A field that a schemaless table holds and no DEFINE FIELD names is
// not a column, because only the records say that it exists, and D47 forbids
// a source that reads them.
//
// Every statement below a schema reads the database the connection is in,
// which the path of the URL names. So a kind such as Tables reports that one
// database as its schema, and a pattern that names another database matches
// nothing.
//
// # The columns arrive in the order of their names
//
// The server sorts the keys of every object it returns, and the driver takes
// the columns of a row from the keys. So every query here declares its
// fields in the order of their names, which is the order the columns arrive
// in, and Scan reads them in that order.
//
// # Parameters
//
// SurrealQL has named parameters only, so the dialect sets Info.Named and a
// parameter is written $p1, $p2 and so on (dbimp D50). SurrealQL has no LIKE.
// A pattern is turned into a regular expression inside the statement, with %
// as any run of characters, _ as one character, and a backslash before a
// character that is that character, as dbmeta.Like reads it.
//
// # The version
//
// No SurrealQL statement returns the version of the server. The driver reads
// it through the RPC method version, and usql prints it that way. The
// version query here is a probe that tells 2.x from 3.x and nothing finer,
// and parseVersion also reads the RPC answer, such as surrealdb-3.3.0. Ken
// chose this on 2026-10-01. See D164.
//
// # What it answers
//
// 18 of the 56 on 3.x, and 1 on 2.7. Databases, schemas, the current
// schema, tables, views, columns, indexes, index columns, constraints,
// constraint columns, triggers, functions, routine parameters, sequences,
// roles, role grants, privileges and comments on 3.x. The current schema on
// 2.7, because a 2.x statement cannot hold INFO as a value: RETURN (INFO FOR
// DB) is a parse error there. So every other kind is too old on 2.7.
//
// # What is missing
//
// There is no current user: no function names the system user a session
// signed in as, and $auth is NONE for one. There is no type, domain,
// collation, cast, operator, extension or setting a statement can read.
// docs/COVERAGE.md holds the rest.
package surrealdb

import (
	"strconv"
	"strings"

	"github.com/xo/dbmeta"
)

// versionQuery tells a 3.x server from a 2.x server, because no statement
// returns the release. 3.x sorts the members of a set and 2.x keeps their
// order, which dbimp recorded on 2.7.0, 3.1.6, 3.2.4 and 3.3.0. It parses on
// both lines, where a probe that reads INFO as a value fails to parse on 2.x.
//
// usql reads the release through the RPC method version, which a statement
// cannot reach. See docs/USQL.md.
const versionQuery = `RETURN IF (<set>[2, 1])[0] = 1 THEN '3' ELSE '2' END`

// rpcPrefix starts the answer of the RPC method version, such as
// surrealdb-3.3.0 or surrealdb-3.1.6+20260813.cfbaec4.
const rpcPrefix = "surrealdb-"

// parseVersion reads the one column versionQuery returns, which is the major
// release alone, or the answer of the RPC method version, which a caller
// reads through the driver. The display line is the one usql prints, such
// as SurrealDB 3.3.0, or SurrealDB 3 from the probe.
func parseVersion(cols []string) (dbmeta.VersionSet, error) {
	var s dbmeta.VersionSet
	if len(cols) != 1 {
		return s, dbmeta.ErrInvalidVersion
	}
	text := strings.TrimPrefix(strings.TrimSpace(cols[0]), rpcPrefix)
	ver := dbmeta.ParseVersion(text)
	if text == "" || ver.Unknown {
		return s, dbmeta.ErrInvalidVersion
	}
	s.Set("", ver)
	s.Display = "SurrealDB " + text
	return s, nil
}

func init() {
	dbmeta.RegisterDialect(dbmeta.SurrealDB, &dbmeta.Info{
		// SurrealQL has comments from --, from # and from // to the end of
		// the line, block comments, and names between backticks. A name
		// keeps its case, which scanEveryQuery measures (D143).
		Syntax: dbmeta.Syntax{BlockComments: true, SlashComments: true, HashComments: true, Backticks: true},
		// A parameter is bound by its name, because SurrealQL has no
		// positional parameter. Placeholder writes what bind writes.
		Named:          true,
		Placeholder:    func(n int) string { return "$p" + strconv.Itoa(n) },
		VersionQuery:   versionQuery,
		VersionColumns: 1,
		ParseVersion:   parseVersion,
	})
	registerRelations()
	registerRoutines()
	registerRoles()
}

// v3 is the release from which an INFO statement is a value that a SELECT
// reads. Every kind but the current schema needs it, and 2.7 reports that it
// is too old.
var v3 = dbmeta.V(3)

// from3 is a statement that needs INFO as a value, which 3.0 is the first
// release to allow.
func from3(query string) dbmeta.Stmt { return dbmeta.Stmt{{{Min: v3, Query: query}}} }

// like is the condition that col matches the pattern in param, and that
// everything matches an empty pattern.
func like(col, param string) string {
	return "(" + param + " = '' OR string::matches(" + col + ", " + regex(param) + "))"
}

// regex turns the LIKE pattern in param into a regular expression, one
// character at a time, with array::fold. The fold keeps the expression so far
// as s, and as e whether the last character was a backslash that makes this
// one literal. string::split with the empty string returns every character
// with an empty string on each side, and the one at the end is what lets the
// fold see a backslash at the end, which is then itself, as dbmeta.Like reads
// it. (?s) lets a dot match a newline, as % and _ do.
//
// The fold is written with a lookup in an object rather than a chain of ELSE
// IF, because the server refuses a statement whose expressions nest too deep.
func regex(param string) string {
	return "('^(?s)' + array::fold(string::split(" + param + ", ''), {s: '', e: false}, |$r, $c|" +
		" {s: $r.s + (IF $r.e THEN (IF $c = '' THEN '\\\\' ELSE " + quoteChar("$c") + " END)" +
		" ELSE ({'%': '.*', '_': '.', '\\\\': ''}[$c] ?? " + quoteChar("$c") + ") END)," +
		" e: !$r.e AND $c = '\\\\'}).s + '$')"
}

// quoteChar is the character in c as a regular expression that matches it
// alone. The server's regular expressions take a backslash before any ASCII
// character that is not a letter, a digit, < or >, and refuse one before
// those, so a letter, a digit, < and > and any character outside ASCII are
// left as they are.
func quoteChar(c string) string {
	return "(IF " + c + " = '' OR string::is_alphanum(" + c + ") OR !string::is_ascii(" + c + ")" +
		" OR " + c + " IN ['<', '>'] THEN " + c + " ELSE '\\\\' + " + c + " END)"
}

// The catalog and the schema of every object below a schema: the namespace
// and the database the connection is in.
const (
	currentNS = "session::ns()"
	currentDB = "session::db()"
)

// tables is every table of the database the connection is in.
const tables = "(INFO FOR DB STRUCTURE).tables"

// parentTables is the tables of the database whose names match @parent. A
// kind below a table reads INFO FOR TABLE only for these, so a pattern that
// names one table reads one table.
var parentTables = "array::filter(" + tables + ", |$t| " + like("$t.name", "@parent") + ")"

// eachTable is one row for each item that INFO FOR TABLE lists under key,
// for every table in src, in one statement. row is an object that can name
// the table as $t, the item as $x and its position in the list, from 0, as
// $i. The list is in the order of the names.
func eachTable(src, key, row string) string {
	return "array::flatten(array::map(" + src + ", |$t| array::map((INFO FOR TABLE $t.name STRUCTURE)." +
		key + ", |$x, $i| " + row + ")))"
}

// permission is a permission as DEFINE writes it: FULL for true, NONE for
// false, and WHERE before an expression.
func permission(v string) string {
	return "(IF " + v + " = true THEN 'FULL' ELSE IF " + v + " = false THEN 'NONE' ELSE 'WHERE ' + " + v + " END)"
}
