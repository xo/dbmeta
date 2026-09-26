// Package firebird holds the metadata queries for Firebird.
//
// Import it for its effect:
//
//	import _ "github.com/xo/dbmeta/models/firebird"
//
// # One flat namespace, and no schema at all
//
// Firebird 3.0 through 5.0 has no schemas. Every object in a database lives
// in one namespace and object names are unique across the whole database, so
// there is nothing between the database and the table. Firebird 6.0 adds SQL
// schemas and is not in range.
//
// So Schemas and CurrentSchema report [dbmeta.ErrNotSupported] rather than
// one invented row, and every other query returns an empty schema. An empty
// string is the honest answer where the product has no such level, and a
// fabricated one would be indistinguishable from a real schema to a caller
// that cannot see the server.
//
// A Firebird database is also a file rather than a name. The server serves
// whatever file a connection asks for and keeps no catalog of the files it
// has served, so Databases returns exactly one row, the database attached.
//
// # RDB$, and the two catalogs beside it
//
// There is no information_schema. The catalog is the RDB$ tables, which are
// ordinary tables inside the database and which a statement reads the way it
// reads any other. Two other prefixes matter. MON$ is the monitoring set,
// which is where the attached database describes itself. SEC$ is a view on
// the server's security database, which is where users live, because a user
// is a property of the server and a role is a property of the database.
//
// Everything in RDB$ is CHAR(63) and blank padded, so every name this model
// returns is trimmed. A comment is a BLOB SUB_TYPE TEXT, which the driver
// hands back as a string and as NULL when absent, so no cast is needed and
// none is written.
//
// # What it answers
//
// 24 of the 55. Tables, columns, views, indexes, index columns, constraints,
// constraint columns, triggers, event triggers, sequences, domains,
// functions, routine parameters, types, collations, roles, role grants,
// privileges, comments, databases, settings, publications, publication
// tables and the current user.
//
// Three of those need Firebird 4.0 and report [dbmeta.ErrTooOld] on 3.0:
// settings reads RDB$CONFIG, and publications and publication tables read
// RDB$PUBLICATIONS and RDB$PUBLICATION_TABLES. None of the three exists
// before 4.0.
//
// What is missing is missing because Firebird has no such thing. There are no
// schemas, tablespaces, casts, operators, aggregates, enumerated types,
// extensions, foreign tables or text search objects at any release in range,
// and no catalog of large objects, because a BLOB is addressed from the row
// that holds it. docs/COVERAGE.md holds the rest, including what a second
// opinion got wrong.
package firebird

import "github.com/xo/dbmeta"

// versionQuery reads the engine version, which Firebird reports as three
// numbers such as 5.0.4. RDB$DATABASE is the one row table every Firebird
// database has, and it is what a query selects from when it wants no table.
//
// This is the statement usql runs for Firebird as well. See docs/USQL.md.
const versionQuery = `SELECT rdb$get_context('SYSTEM', 'ENGINE_VERSION') FROM rdb$database`

// parseVersion reads the one column versionQuery returns.
func parseVersion(cols []string) (dbmeta.VersionSet, error) {
	var s dbmeta.VersionSet
	if len(cols) != 1 {
		return s, dbmeta.ErrInvalidVersion
	}
	s.Set("", dbmeta.ParseVersion(cols[0]))
	s.Display = "Firebird " + cols[0]
	return s, nil
}

func init() {
	dbmeta.RegisterDialect(dbmeta.Firebird, &dbmeta.Info{
		// Firebird binds by position and writes a question mark. A parameter
		// repeated in the statement is sent twice, which is what every filter
		// here does, and the server infers the type from the comparison
		// rather than needing a cast. That was measured on 3.0 and on 5.0,
		// because a bare parameter is refused in some positions and these
		// are not among them.
		Placeholder:    func(int) string { return "?" },
		VersionQuery:   versionQuery,
		VersionColumns: 1,
		ParseVersion:   parseVersion,
	})
	registerRelations()
	registerRoutines()
	registerRoles()
	registerServer()
}

// always is a fragment every release takes.
func always(query string) dbmeta.Choice { return dbmeta.Choice{{Query: query}} }

// v4 is the one release that added anything this model reads. 5.0 added a
// partial index predicate in RDB$INDICES.RDB$CONDITION_SOURCE and there is no
// field on [dbmeta.Index] to carry it, so nothing here gates on 5.0.
var v4 = dbmeta.V(4, 0)

// like builds a filter that matches everything when the parameter is empty.
//
// Every parameter is cast, and that is not decoration. Firebird takes the
// type of a bare parameter from what it is compared with, so writing
// `? = ”` types the parameter VARCHAR(0) and the server then refuses any
// value at all:
//
//	arithmetic exception, numeric overflow, or string truncation
//	string right truncation
//	expected length 0, actual 6
//
// An empty filter still passed, which is what makes this worth a comment: a
// first run against a real server with no arguments reports nothing wrong.
//
// The width is 255 rather than 63, the identifier limit, because the value is
// a LIKE pattern rather than a name.
// The column is trimmed as well. A catalog name is CHAR(63) and blank
// padded, and LIKE compares the padding: `RDB$RELATION_NAME LIKE 'AUTHOR'`
// matches nothing at all, because the stored value is AUTHOR and 57 spaces.
// Equality pads and LIKE does not, which is why a filter can look right and
// return an empty result rather than an error.
//
// Only the trailing blanks are padding. A quoted identifier can begin with a
// space, and a plain TRIM would take that too and report a name the database
// does not have.
func like(col, param string) string {
	p := `CAST(` + param + ` AS VARCHAR(255))`
	return `(` + p + ` = '' OR TRIM(TRAILING FROM ` + col + `) LIKE ` + p + `)`
}

// userObject is the filter that leaves out the catalog's own rows.
//
// RDB$SYSTEM_FLAG is NULL rather than zero on a row some releases wrote, so
// every test of it goes through COALESCE. That is a filter and not a returned
// value, which is the distinction docs/NULLS.md draws: collapsing a NULL on
// the way out loses a fact, and collapsing one in a WHERE clause is how the
// question is asked.
func userObject(col string) string {
	return `COALESCE(` + col + `, 0) = 0`
}

// fieldType builds the SQL type name for a row of RDB$FIELDS, which records a
// type as a code, a sub type, a length, a precision and a scale rather than
// as a name.
//
// The codes for DECFLOAT, INT128 and the two time zone types arrived in 4.0.
// They need no version fragment, because a release that does not have a type
// never stores its code, so the arm simply never matches.
func fieldType(p string) string {
	return `TRIM(TRAILING FROM CASE ` + p + `.RDB$FIELD_TYPE` +
		` WHEN 7 THEN ` + exact(p, `'SMALLINT'`) +
		` WHEN 8 THEN ` + exact(p, `'INTEGER'`) +
		` WHEN 9 THEN 'QUAD'` +
		` WHEN 10 THEN 'FLOAT'` +
		` WHEN 12 THEN 'DATE'` +
		` WHEN 13 THEN 'TIME'` +
		` WHEN 14 THEN 'CHAR(' || ` + p + `.RDB$CHARACTER_LENGTH || ')'` +
		` WHEN 16 THEN ` + exact(p, `'BIGINT'`) +
		` WHEN 23 THEN 'BOOLEAN'` +
		` WHEN 24 THEN 'DECFLOAT(16)'` +
		` WHEN 25 THEN 'DECFLOAT(34)'` +
		` WHEN 26 THEN ` + exact(p, `'INT128'`) +
		` WHEN 27 THEN 'DOUBLE PRECISION'` +
		` WHEN 28 THEN 'TIME WITH TIME ZONE'` +
		` WHEN 29 THEN 'TIMESTAMP WITH TIME ZONE'` +
		` WHEN 35 THEN 'TIMESTAMP'` +
		` WHEN 37 THEN 'VARCHAR(' || ` + p + `.RDB$CHARACTER_LENGTH || ')'` +
		` WHEN 261 THEN 'BLOB SUB_TYPE ' || ` + p + `.RDB$FIELD_SUB_TYPE` +
		` ELSE 'UNKNOWN' END)`
}

// exact writes the NUMERIC or DECIMAL spelling of an integer backed exact
// type, and the plain integer name where the sub type says it is not one.
// Firebird stores DECIMAL(10,2) as a LONG with sub type 2 and a scale of -2.
func exact(p, plain string) string {
	return `CASE ` + p + `.RDB$FIELD_SUB_TYPE` +
		` WHEN 1 THEN 'NUMERIC(' || ` + p + `.RDB$FIELD_PRECISION || ',' || (-` + p + `.RDB$FIELD_SCALE) || ')'` +
		` WHEN 2 THEN 'DECIMAL(' || ` + p + `.RDB$FIELD_PRECISION || ',' || (-` + p + `.RDB$FIELD_SCALE) || ')'` +
		` ELSE ` + plain + ` END`
}
