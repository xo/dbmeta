// Package neo4j holds the metadata queries for Neo4j, the graph database, in
// Cypher.
//
// Import it for its effect:
//
//	import _ "github.com/xo/dbmeta/models/neo4j"
//
// # The catalog
//
// Neo4j has no relational catalog. A server holds databases, and a database
// holds nodes and relationships. A node carries labels, a relationship has one
// type, and both carry properties. The catalog is a set of SHOW commands, such
// as SHOW DATABASES, SHOW INDEXES and SHOW CONSTRAINTS, and a set of
// procedures, such as db.labels(). A SHOW command takes YIELD, WHERE and
// RETURN, so it filters and projects like a SELECT, and every query here is
// one statement. D162 holds the mapping and the reasons for it.
//
// A database is the database, and it is the schema too, because a database
// has no namespace inside it. Every statement reads the database the
// connection is in, which is the path of the URL, so Schemas and
// CurrentSchema return that one database and every object reports it as its
// schema. Databases lists every database on the server.
//
// A node label and a relationship type are the two kinds of table. An index
// and a constraint belong to the label or the type they are on.
//
// A user is the role, and a Neo4j role is what a user is a member of. SHOW
// USERS and SHOW ROLES cannot be joined in one statement on any release, and
// a Neo4j role is granted privileges and cannot log in.
//
// # Patterns
//
// Cypher has no LIKE. A pattern parameter is turned into a Java regular
// expression inside the statement, with % as any run of characters, _ as one
// character, and a backslash before a character that is that character, as
// dbmeta.Like reads it.
//
// # Two releases, two grammars
//
// On 5.26, a SHOW command cannot be joined with anything. It cannot be in a
// subquery or a UNION, and no UNWIND can follow it. From 2026.05, in Cypher 25,
// SHOW INDEXES, SHOW CONSTRAINTS, SHOW FUNCTIONS and SHOW PROCEDURES can be
// joined with other clauses. SHOW USERS, SHOW ROLES and SHOW PRIVILEGES still
// cannot. A kind that needs one row for each item of a list, or the rows of
// two SHOW commands, needs 2026.05, and 5.26 reports that it is too old. Each
// such statement starts with CYPHER 25, so that it runs as Cypher 25 in a
// database whose default language is Cypher 5.
//
// # What it answers
//
// 17 of the 56 on 2026.09, and 13 on 5.26. Databases, schemas, the current
// schema, tables, indexes, constraints, aggregates, roles, role grants, role
// settings, privileges, the current user and settings on both. Index columns,
// constraint columns, functions and routine parameters from 2026.05.
//
// # What is missing
//
// Columns is the largest gap. Neo4j keeps no catalog of the properties of a
// label or a type. The only source is db.schema.nodeTypeProperties() and
// db.schema.relTypeProperties(), which read every node and every relationship
// to find them. On 2026.09.0 it took 0.37 seconds for 1 million nodes and 0.95
// seconds for 4 million, so its cost grows with the data. D47 forbids that,
// and Ken chose on 2026-10-01 to leave Columns unsupported for that reason.
//
// There is no view, no sequence, no trigger, no user defined type, no comment
// and no foreign key. A relationship joins two nodes and is not a constraint
// on a property. docs/COVERAGE.md holds the rest.
package neo4j

import (
	"strconv"
	"strings"

	"github.com/xo/dbmeta"
)

// versionQuery reads the release, such as 2026.09.0, and the edition, such as
// enterprise.
//
// It is the statement usql runs for Neo4j, with a name for each column. See
// docs/USQL.md.
const versionQuery = `CALL dbms.components() YIELD name, versions, edition` +
	` WHERE name = 'Neo4j Kernel' RETURN versions[0] AS version, edition`

// parseVersion reads the two columns versionQuery returns. The display line is
// the one usql prints, such as Neo4j 2026.09.0 enterprise.
func parseVersion(cols []string) (dbmeta.VersionSet, error) {
	var s dbmeta.VersionSet
	if len(cols) != 2 || cols[0] == "" {
		return s, dbmeta.ErrInvalidVersion
	}
	ver := dbmeta.ParseVersion(strings.TrimSpace(cols[0]))
	if ver.Unknown {
		return s, dbmeta.ErrInvalidVersion
	}
	s.Set("", ver)
	s.Display = strings.TrimSpace("Neo4j " + cols[0] + " " + cols[1])
	return s, nil
}

func init() {
	dbmeta.RegisterDialect(dbmeta.Neo4j, &dbmeta.Info{
		// Cypher has block comments, comments from // to the end of the
		// line, and names between backticks. A name keeps its case, and
		// scanEveryQuery measures that (D143).
		Syntax: dbmeta.Syntax{BlockComments: true, SlashComments: true, Backticks: true},
		// The driver fills $1 with the first positional argument, $2 with the
		// second, and so on (dbimp D64).
		Placeholder:    func(n int) string { return "$" + strconv.Itoa(n) },
		VersionQuery:   versionQuery,
		VersionColumns: 2,
		ParseVersion:   parseVersion,
	})
	registerRelations()
	registerRoutines()
	registerRoles()
}

// The releases the fragments gate on. v526 is the floor of the model, the LTS
// release. v2605 is the release from which a SHOW command can be joined with
// other clauses in Cypher 25.
var (
	v526  = dbmeta.V(5, 26)
	v2605 = dbmeta.V(2026, 5)
)

// both is a statement that every release from the floor takes.
func both(query string) dbmeta.Stmt { return dbmeta.Stmt{{{Min: v526, Query: query}}} }

// composed is a statement that needs a SHOW command joined with other clauses,
// which 2026.05 is the first release to allow. 5.26 reports that it is too
// old.
func composed(query string) dbmeta.Stmt {
	return dbmeta.Stmt{{{Min: v2605, Query: "CYPHER 25 " + query}}}
}

// like is the condition that col matches the pattern in param, and that
// everything matches an empty pattern.
func like(col, param string) string {
	return "(" + param + " = '' OR " + col + " =~ " + regex(param) + ")"
}

// regex turns the LIKE pattern in param into a Java regular expression, one
// character at a time. Each literal character is quoted between \Q and \E. A
// backslash makes the next character literal, and a backslash at the end is
// itself, which is what dbmeta.Like does. The empty string after the last
// character is what lets the reduction see a backslash at the end. (?s) lets
// a dot match a newline, as % and _ do.
func regex(param string) string {
	return "('(?s)' + reduce(r = {s: '', e: false}, c IN split(" + param + ", '') + [''] | CASE" +
		" WHEN r.e THEN {s: r.s + '\\\\Q' + CASE WHEN c = '' THEN '\\\\' ELSE c END + '\\\\E', e: false}" +
		" WHEN c = '\\\\' THEN {s: r.s, e: true}" +
		" WHEN c = '%' THEN {s: r.s + '.*', e: false}" +
		" WHEN c = '_' THEN {s: r.s + '.', e: false}" +
		" ELSE {s: r.s + '\\\\Q' + c + '\\\\E', e: false} END).s)"
}

// join is the items of the list expression list joined by sep, and the empty
// string for an empty list.
func join(list, sep string) string {
	return "substring(reduce(s = '', x IN " + list + " | s + '" + sep + "' + x), " +
		strconv.Itoa(len(sep)) + ")"
}

// currentDB is the name of the database the connection is in, as an
// expression that a SHOW command can hold on every release.
const currentDB = "COLLECT { CALL db.info() YIELD name AS d RETURN d }[0]"
