// Package hive holds the metadata queries for Apache Hive.
//
// Import it for its effect:
//
//	import _ "github.com/xo/dbmeta/models/hive"
//
// # It reads sys, which is the metastore as tables
//
// Hive keeps its metadata in a relational metastore that SQL cannot reach.
// Hive 3.0 added a sys database that exposes that metastore as external
// tables over the JDBC storage handler, and those answer ordinary SQL. That
// is what this model reads.
//
// sys is not there when a server starts. The script that creates it ships
// with Hive and needs a running HiveServer2 to run against, so it is
// installed after the server answers rather than baked into an image.
// container/hive.go carries that as Server.Init and dbrun runs it.
//
// This is what separates Hive from Impala. D67 struck Impala because it
// answers only through SHOW and DESCRIBE, which are statements rather than
// relations and cannot be filtered, joined or aliased. sys is relations.
//
// # Hive cannot bind a parameter, so this model writes literals
//
// HiveServer2 has no parameter channel. Its Thrift request carries a session
// handle, a statement, a configuration overlay, an asynchronous flag and a
// timeout, and nothing else, so no driver can bind and Hive's own JDBC
// PreparedStatement substitutes on the client.
//
// So this dialect sets [dbmeta.Info.Literal] and every filter is rendered
// into the statement. It is the only model here that does. See D78, and read
// [literal] before changing anything about it.
//
// # What it answers
//
// 16 of the 55. Tables, schemas, columns, views, constraints, constraint
// columns, partitioned tables, functions, roles, role grants, privileges,
// comments, column statistics, access methods, the current schema and the
// current user.
//
// What is missing is mostly missing because Hive has no such thing. Indexes
// were removed in Hive 3.0, and there are no sequences, triggers, domains,
// collations, types, operators or foreign data of any kind.
package hive

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/xo/dbmeta"
)

// versionQuery reads the server version, which Hive reports as the release
// and the commit it was built from.
//
// usql registers no Version function for Hive, so it falls through to the
// generic SELECT version();, which is this statement. The two agree. See
// docs/USQL.md.
const versionQuery = `SELECT version()`

// parseVersion reads the one column versionQuery returns, which is a release
// and a commit separated by a space: "4.2.1 r0f4761a83b40...".
func parseVersion(cols []string) (dbmeta.VersionSet, error) {
	var s dbmeta.VersionSet
	if len(cols) != 1 {
		return s, dbmeta.ErrInvalidVersion
	}
	release, _, _ := strings.Cut(cols[0], " ")
	s.Set("", dbmeta.ParseVersion(release))
	s.Display = "Apache Hive " + cols[0]
	return s, nil
}

func init() {
	dbmeta.RegisterDialect(dbmeta.Hive, &dbmeta.Info{
		// Hive has no placeholder, because it has no parameter. Literal
		// is what this dialect uses and Placeholder is never called,
		// but a nil one would be a trap for anything that reads Info
		// without checking Literal first, so it says what it means.
		Placeholder: func(int) string {
			panic("hive: Placeholder is never used, because Hive cannot bind. See Info.Literal")
		},
		Literal:        literal,
		VersionQuery:   versionQuery,
		VersionColumns: 1,
		ParseVersion:   parseVersion,
	})
	registerRelations()
	registerExtra()
}

// literal renders one filter value as a Hive literal.
//
// # Why a doubled quote is wrong here
//
// Hive does not accept the SQL standard's doubled quote. Measured on 4.2.1:
//
//	SELECT 'a''b'  ->  ab
//	SELECT 'a\'b'  ->  a'b
//
// The first is read as two literals written next to each other and joined,
// so a doubled quote loses the quote and returns a wrong answer rather than
// an error. [dbmeta.QuoteLiteral] doubles, which is right for every other
// product here and wrong for this one, and that is why the dialect supplies
// this function rather than setting a flag.
//
// Hive uses C style backslash escapes, so a backslash is escaped first and
// then the quote. Doing it the other way round would escape the backslash
// this function just wrote.
//
// # What it refuses
//
// A type it does not know, and a string holding a NUL. Everything this
// project declares as a parameter is a string or a boolean, so an unknown
// type means a caller passed something a query did not declare, and
// answering that with an error is better than rendering it.
//
// A NUL is refused for the same reason [dbmeta.Dialect.ChangePassword]
// refuses one: it can end a string early in a layer below this and there is
// no reason for a filter to hold one.
func literal(v any) (string, error) {
	switch t := v.(type) {
	case string:
		if strings.ContainsRune(t, 0) {
			return "", fmt.Errorf("hive: %w: a filter cannot hold a NUL", dbmeta.ErrInvalidParam)
		}
		t = strings.ReplaceAll(t, `\`, `\\`)
		t = strings.ReplaceAll(t, `'`, `\'`)
		return "'" + t + "'", nil
	case bool:
		if t {
			return "TRUE", nil
		}
		return "FALSE", nil
	case int:
		return strconv.Itoa(t), nil
	case int64:
		return strconv.FormatInt(t, 10), nil
	}
	return "", fmt.Errorf("hive: %w: cannot render %T as a literal", dbmeta.ErrInvalidParam, v)
}

// always is a fragment every release takes, with its aliases rewritten.
func always(query string) dbmeta.Choice {
	return dbmeta.Choice{{Query: backtick(query)}}
}

// backtick turns every double quote in a fragment into Hive's identifier
// quote.
//
// Hive quotes an identifier with a backtick, and a Go raw string cannot hold
// one, so the statements here are written with double quoted aliases the way
// every other model writes them and converted once on the way in.
//
// The conversion is a blanket replacement and it is safe because of one
// invariant: the SQL in this package uses a double quote for an alias and
// for nothing else. Every string literal in it is single quoted, and a value
// a caller supplies never passes through here, because [literal] renders
// those separately and after this.
//
// It is needed rather than cosmetic. Hive accepts a double quoted alias from
// beeline and refuses one from the driver, which was measured:
//
//	SELECT current_user() AS "name"    beeline: ok, driver: fails
//	SELECT current_user() AS `name`    both: ok
//
// A bare alias works too and is not used, because several of the names this
// project wants are reserved words in Hive, among them default, table and
// distinct.
func backtick(s string) string { return strings.ReplaceAll(s, `"`, "`") }

// like builds a filter that matches everything when the parameter is empty.
func like(col, param string) string {
	return `(` + param + ` = '' OR ` + col + ` LIKE ` + param + `)`
}

// notSystem leaves out the databases Hive keeps for itself. sys is the
// catalog this model reads and information_schema is the views over it.
//
// It names the column rather than taking it, because every statement here
// reaches the database list the same way and aliases it d. A model whose
// filter can apply to more than one column takes the name instead.
const notSystem = `(@with_system = TRUE OR d.NAME NOT IN ('sys', 'information_schema'))`
