// Package exasol holds the metadata queries for Exasol.
//
// Import it for its effect:
//
//	import _ "github.com/xo/dbmeta/models/exasol"
//
// # The EXA_ALL views
//
// Exasol has no information_schema worth the name. Its catalog is a set of
// system tables in SYS, in three families: EXA_USER_ for what the current
// user owns, EXA_ALL_ for what it can see, and EXA_DBA_ for everything, which
// needs SELECT ANY DICTIONARY. This model reads EXA_ALL_, for the reason the
// Oracle model reads ALL_: it answers every principal with what that
// principal can see and asks no special right.
//
// Two queries read EXA_DBA_ because nothing else holds the answer. RoleGrants
// reads EXA_DBA_ROLE_PRIVS and UserMappings reads the connection views, and
// both are refused to a principal without the right. test/parity_test.go
// records that.
//
// The system tables themselves are not in the EXA_ALL views, which list what
// users made. with_system reaches them through EXA_SYSCAT and
// EXA_SYS_COLUMNS.
//
// # An empty string is NULL
//
// Exasol reads an empty string literal as NULL, the way Oracle does. It
// changes two things here.
//
// A filter tests for NULL rather than for the empty string. An empty filter
// arrives as an empty string and is NULL by the time it is compared, so a
// comparison with the empty string is never true, and the first version of
// this model matched nothing at all.
//
// A field that the object model says is a plain string is scanned through
// [dbmeta.NullAsEmpty], which reads NULL as the empty string. The statement
// selects an empty string for a field that is always empty and gets NULL
// back, and a NULL cannot be scanned into a string. A field whose type is
// sql.Null keeps its NULL, and
// three fields where the empty string means something are restored from the
// row: a column that is not an identity has identity "", and so for
// generated, and a foreign key's catalog is "". See [present].
//
// # What it answers
//
// 25 of the 65. Tables, schemas, columns, views, indexes, index columns,
// constraints, constraint columns, partitioned tables, comments, functions,
// aggregates, types, languages, roles, role grants, privileges, settings,
// the database, foreign data wrappers, foreign servers, user mappings,
// foreign tables, the current schema and the current user.
//
// Three answer with an analogue. A virtual schema is a foreign server, the
// adapter script behind it is the wrapper, and its virtual tables are the
// foreign tables. A connection granted to a principal is a user mapping. A
// set UDF that returns one value is an aggregate.
//
// Every one was run against 2025.2.1 on the Community Edition and 2026.2.0
// on the nano image, which answer identically, so this model has no version
// fragment.
//
// # What is missing
//
// 31 kinds. Exasol has no sequence, trigger, domain, enumerated type,
// collation, tablespace, extension, publication, text search object or
// operator of any kind, no unique or check constraint, and no CREATE INDEX:
// it builds its own indices. The parameters of a routine are kept only in
// its text, and there are no planner statistics a statement can read.
// docs/COVERAGE.md holds the rest, including the leads a second opinion
// invented.
package exasol

import (
	"database/sql"

	"github.com/xo/dbmeta"
)

// versionQuery reads the server version.
//
// It is the statement usql runs for Exasol, word for word, so the two agree.
// EXA_METADATA holds the driver facing properties of the database and every
// user can read it. See docs/USQL.md.
const versionQuery = `SELECT PARAM_VALUE FROM EXA_METADATA WHERE PARAM_NAME = 'databaseProductVersion'`

// parseVersion reads the one column versionQuery returns, which is the
// release alone, such as 2025.2.1.
func parseVersion(cols []string) (dbmeta.VersionSet, error) {
	var s dbmeta.VersionSet
	if len(cols) != 1 || cols[0] == "" {
		return s, dbmeta.ErrInvalidVersion
	}
	s.Set("", dbmeta.ParseVersion(cols[0]))
	s.Display = "Exasol " + cols[0]
	return s, nil
}

func init() {
	dbmeta.RegisterDialect(dbmeta.Exasol, &dbmeta.Info{
		// The syntax is usql's lexer flags for this product, and the fold
		// is measured by scanEveryQuery (D143).
		Syntax: dbmeta.Syntax{BlockComments: true},
		Fold:   dbmeta.FoldUpper,
		// The Exasol driver binds by position and writes a question mark.
		Placeholder:    func(int) string { return "?" },
		VersionQuery:   versionQuery,
		VersionColumns: 1,
		ParseVersion:   parseVersion,
		ChangePassword: changePassword,
	})
	registerRelations()
	registerRoutines()
	registerRoles()
	registerServer()
	registerForeign()
}

// always is a fragment every release takes.
func always(query string) dbmeta.Choice { return dbmeta.Choice{{Query: query}} }

// like builds a filter that matches everything when the parameter is empty.
//
// It tests for NULL rather than for the empty string, because Exasol reads
// an empty string as NULL. An empty filter arrives as an empty string and is
// NULL by the time it is compared, so comparing it with the empty string is
// never true and matched nothing at all.
func like(col, param string) string {
	return `(` + param + ` IS NULL OR ` + col + ` LIKE ` + param + `)`
}

// present scans a nullable text column whose value is the empty string where
// the statement selected an empty string.
//
// It is for a field that has three states, where the empty string is one of
// them and means something: identity is empty for a column that is not an
// identity, and absent where the release has no source. On Exasol the
// statement cannot say the empty string, so a field that is empty whenever
// the row has it reads NULL as a valid empty value here.
func present(p *sql.Null[string]) sql.Scanner { return presentString{p} }

type presentString struct{ p *sql.Null[string] }

// Scan satisfies sql.Scanner.
func (e presentString) Scan(v any) error {
	var s string
	if err := dbmeta.NullAsEmpty(&s).Scan(v); err != nil {
		return err
	}
	*e.p = sql.Null[string]{V: s, Valid: true}
	return nil
}

// system is true when the caller asked for the objects Exasol keeps for
// itself. Those are not in the EXA_ALL views at all, which list what users
// made, so every query that can reach them does it with a second arm over
// the system catalog behind this test.
const system = `@with_system = TRUE`

// schemaAndName is the filter set most queries here take.
func schemaAndName(kind string) []dbmeta.Param {
	return []dbmeta.Param{
		{Name: "schema", Desc: "schema name pattern, empty for every schema", Default: ""},
		{Name: "name", Desc: kind + " name pattern, empty for every one", Default: ""},
		{Name: "with_system", Desc: "include the objects Exasol keeps for itself in SYS and EXA_STATISTICS", Default: false},
	}
}

// parentAndName is the filter set for a child of a table.
func parentAndName(kind string) []dbmeta.Param {
	return []dbmeta.Param{
		{Name: "schema", Desc: "schema name pattern, empty for every schema", Default: ""},
		{Name: "parent", Desc: "table name pattern, empty for every table", Default: ""},
		{Name: "name", Desc: kind + " name pattern, empty for every one", Default: ""},
	}
}

// nameOnly is the filter set for an object that belongs to the database.
func nameOnly(kind string) []dbmeta.Param {
	return []dbmeta.Param{
		{Name: "name", Desc: kind + " name pattern, empty for every one", Default: ""},
	}
}
