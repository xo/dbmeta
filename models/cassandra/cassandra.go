// Package cassandra holds the metadata queries for Apache Cassandra.
//
// Import it for its effect:
//
//	import _ "github.com/xo/dbmeta/models/cassandra"
//
// # CQL is not SQL, and three things follow
//
// First, a statement selects columns and nothing else. There is no CASE, no
// arithmetic on text, no function that turns one value into another. A field
// that another model computes in the statement is computed in Scan here, from
// a raw catalog column the statement returns. See D62.
//
// Second, a filter cannot be optional. CQL has no OR and no IS NULL, and a
// partition key takes only = or IN, so the form every other model writes,
// (@schema IS NULL OR col LIKE @schema), cannot be expressed. Every query here
// returns every row and each parameter says so. A consumer narrows the result
// itself, which usql already does to match psql. That includes the system
// keyspaces, because NOT IN is not available either.
//
// Third, there is no order across partitions. CQL orders rows only within one
// partition and only by a clustering column, so a result arrives in token
// order and the same query can return the same rows in another order on
// another cluster. No query here writes ORDER BY, because writing one would
// not make the answer ordered.
//
// # Padding
//
// A column Cassandra does not have is selected as (text)NULL, which is the CQL
// spelling of NULL AS "name". A bare NULL is refused with "cannot infer type
// for term NULL in selection clause", and the type hint is what the server
// asks for. See docs/NULLS.md, which this obeys: a fact that is absent is
// NULL and never a literal.
//
// # What it answers
//
// 17 of the 55. Keyspaces as schemas, tables, columns, materialized views as
// views, user defined types, indexes, index columns, the primary key as a
// constraint and its columns, triggers, comments, functions, aggregates,
// roles, role grants, privileges and settings.
//
// Settings needs 4.0, where the system_views keyspace arrived. Everything else
// answers on every release from 3.11 up.
package cassandra

import (
	"fmt"
	"sort"
	"strings"

	"github.com/xo/dbmeta"
)

// versionSQL reads the three versions Cassandra reports.
//
// They move independently, which is why [dbmeta.VersionSet] holds more than
// one. The release is the main version, because that is what a fragment gates
// on. CQL and the native protocol are recorded under their own keys so a
// caller can read them, and so a future fragment can gate on either.
const versionSQL = `SELECT release_version, cql_version, native_protocol_version` +
	` FROM system.local WHERE key = 'local'`

// parseVersion reads the three columns versionSQL returns.
func parseVersion(cols []string) (dbmeta.VersionSet, error) {
	var s dbmeta.VersionSet
	if len(cols) != 3 {
		return s, dbmeta.ErrInvalidVersion
	}
	release, cql, protocol := cols[0], cols[1], cols[2]
	s.Set("", dbmeta.ParseVersion(release))
	s.Set("cql", dbmeta.ParseVersion(cql))
	s.Set("protocol", dbmeta.ParseVersion(protocol))
	// The same line usql prints, so that a person reading either sees one
	// thing. The protocol is a bare number and carries a v, which is how
	// Cassandra's own documentation writes it.
	s.Display = "Cassandra " + release + ", CQL " + cql + ", Protocol v" + protocol
	return s, nil
}

// changePassword builds the statement that sets a role's password.
//
// CQL has one form and it is ALTER ROLE. There is no ALTER USER worth using:
// CREATE USER and ALTER USER are the pre-2.2 spelling, kept for compatibility
// and defined in terms of roles.
//
// Nothing is detected first. A CQL string literal escapes a quote by doubling
// it and has no backslash escape at all, so there is no server setting that
// changes the answer and no reason to ask. That is why this returns no
// ErrQuotingUnknown where the PostgreSQL one does.
func changePassword(c dbmeta.PasswordChange, _ dbmeta.Quoting) (string, error) {
	return "ALTER ROLE " + dbmeta.QuoteIdentifier(c.User, `"`, `"`) +
		" WITH PASSWORD = " + dbmeta.QuoteLiteral(c.Password, dbmeta.Quoting{}), nil
}

func init() {
	dbmeta.RegisterDialect(dbmeta.Cassandra, &dbmeta.Info{
		// CQL binds by position and writes a question mark, the same as
		// MySQL. The number is not used.
		Placeholder:    func(int) string { return "?" },
		VersionSQL:     versionSQL,
		VersionColumns: 3,
		ParseVersion:   parseVersion,
		ChangePassword: changePassword,
	})
	registerSchema()
	registerExtra()
	registerRoles()
}

// v40 is where the system_views keyspace arrived.
var v40 = dbmeta.V(4)

// always is a fragment every release takes.
func always(sqlstr string) dbmeta.Choice { return dbmeta.Choice{{SQL: sqlstr}} }

// filters declares the filters a caller may pass.
//
// None of them narrows anything. CQL cannot express an optional filter, so
// every query returns every row and the caller narrows the result. They are
// declared rather than left out so that a caller passing one gets the rows
// rather than ErrUnknownParam, and every description says plainly that it does
// nothing. See D62.
func filters(kind string) []dbmeta.Param {
	const why = ", which Cassandra ignores: CQL cannot express an optional" +
		" filter, so every row is returned and the caller narrows it"
	return []dbmeta.Param{
		{Name: "schema", Desc: "keyspace name" + why, Default: ""},
		{Name: "name", Desc: kind + " name" + why, Default: ""},
		{
			Name:    "with_system",
			Desc:    "include the keyspaces Cassandra keeps for itself" + why,
			Default: false,
		},
	}
}

// pad is the scan target for a column the statement selects as (text)NULL.
//
// The driver cannot report a CQL null. gocql decodes one as the zero value of
// its type and go-cql-driver hands that to database/sql, so scanning a null
// text into sql.Null[string] gives a valid empty string rather than an absent
// one. Verified: SELECT (text)NULL comes back Valid with "".
//
// A padded column is known to be absent, so it is discarded here and the field
// keeps its zero value, which is the invalid Null docs/NULLS.md asks for. The
// column is still selected, because the statement should say what it returns
// and because a query returns as many columns as it declares fields.
//
// This does not rescue a real catalog column that is null. Nothing can, with
// this driver, and docs/COVERAGE.md says so.
type pad struct{}

// Scan discards the value and satisfies sql.Scanner.
func (pad) Scan(any) error { return nil }

// isKey reports whether a column kind is part of the primary key.
//
// system_schema.columns.kind is one of partition_key, clustering, regular or
// static. The first two make up the primary key, which is also the only thing
// in Cassandra that cannot be null.
func isKey(kind string) bool {
	return kind == "partition_key" || kind == "clustering"
}

// indexTarget reads the column an index is on.
//
// system_schema.indexes.options is a map, and the target entry holds the
// column name. For an index on a collection it holds a call such as
// keys(m) or values(m), and the name inside the parentheses is the column.
func indexTarget(options map[string]string) (col string, expr string) {
	target := options["target"]
	if target == "" {
		return "", ""
	}
	open := strings.IndexByte(target, '(')
	if open < 0 || !strings.HasSuffix(target, ")") {
		return target, ""
	}
	return target[open+1 : len(target)-1], target
}

// textList renders a CQL list or set of text as one comma separated string.
//
// A collection has no place in a flat row, and D47 says a child of an object
// is its own kind rather than a slice on the parent. These are not children:
// the field names of a user defined type and the argument types of a function
// are part of the thing itself, and every other model returns them as one
// text. The driver hands them over as whatever gocql decoded, so the type
// switch covers what it can produce.
func textList(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case []string:
		return strings.Join(t, ", ")
	case []any:
		parts := make([]string, len(t))
		for i, e := range t {
			parts[i] = toText(e)
		}
		return strings.Join(parts, ", ")
	case map[string]string:
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		parts := make([]string, len(keys))
		for i, k := range keys {
			parts[i] = k + "=" + t[k]
		}
		return strings.Join(parts, ", ")
	}
	return toText(v)
}

// toText renders one decoded CQL value.
func toText(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case []byte:
		return string(t)
	}
	return fmt.Sprint(v)
}

// textMap returns a CQL map<text, text> as a Go map, whatever the driver
// decoded it into. An absent map is an empty one rather than nil, so a caller
// reading a key never has to test for it.
func textMap(v any) map[string]string {
	switch t := v.(type) {
	case map[string]string:
		return t
	case map[string]any:
		out := make(map[string]string, len(t))
		for k, e := range t {
			out[k] = toText(e)
		}
		return out
	}
	return map[string]string{}
}
