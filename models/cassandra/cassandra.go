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
// another cluster. No query here writes ORDER BY, because writing one does
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
// ScyllaDB accepts no literal in a select list at all, so there it selects a
// real column in the padded one's place. Scan discards a padded or fixed
// column on both products and sets the value itself. [fixed] holds the rule.
//
// # ScyllaDB
//
// ScyllaDB is a second product that speaks CQL, and this model reads it.
// Cassandra is the reference product and ScyllaDB is the flavor, the way
// MariaDB and MySQL share the mysql model. It keeps system_schema as
// Cassandra 3.0 laid it out, so most queries need nothing. Three things
// differ, and each is a fragment on the [Scylla] key: the roles, grants and
// permissions are in the system keyspace rather than system_auth, the
// settings are in system.config, and no literal is allowed in a select list.
//
// # What it answers
//
// 17 of the 56 on Cassandra, and 18 on ScyllaDB. Keyspaces as schemas,
// tables, columns, materialized views as views, user defined types, indexes,
// index columns, the primary key as a constraint and its columns, triggers,
// comments, functions, aggregates, roles, role grants, privileges and
// settings. ScyllaDB also answers role settings, from the service level
// attached to a role.
//
// On Cassandra, Settings needs 4.0, where the system_views keyspace arrived.
// Everything else answers on every release from 3.11 up. ScyllaDB answers all
// 18 on every release from 2025.1 up.
package cassandra

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/xo/dbmeta"
)

// Scylla is the version key a ScyllaDB server reports under.
//
// ScyllaDB is a second product that speaks CQL, and it shares this model the
// way MySQL shares MariaDB's. Cassandra is the reference product and ScyllaDB
// is the flavor. A fragment that belongs to ScyllaDB names this key, and a
// Cassandra server never reports it. See D44 and D91.
const Scylla = "scylla"

// onScylla is a gate that ScyllaDB meets at any release. A fragment for one
// ScyllaDB release and newer writes dbmeta.Gate{Key: Scylla, Min: ...}, and
// the release comes from the follow-up statement. See D92.
var onScylla = dbmeta.Gate{Key: Scylla}

// scylla returns a fragment that applies on ScyllaDB.
func scylla(query string) dbmeta.Fragment {
	return dbmeta.Fragment{Min: onScylla.Min, Key: onScylla.Key, Query: query}
}

// versionQuery reads the row that describes the node, as one JSON text.
//
// JSON is what lets one statement work on both products. Cassandra and
// ScyllaDB each have columns in system.local that the other lacks, and CQL
// refuses a statement that names a column the table does not have. SELECT
// JSON * names none, so it runs on both, and the row says which product sent
// it.
//
// The three versions Cassandra reports move independently, which is why
// [dbmeta.VersionSet] holds more than one. The release is the main version,
// because that is what a fragment gates on. CQL and the native protocol are
// recorded under their own keys so a caller can read them, and so a future
// fragment can gate on either.
const versionQuery = `SELECT JSON * FROM system.local WHERE key = 'local'`

// parseVersion reads the one column versionQuery returns.
//
// ScyllaDB is found by the supported_features column, which ScyllaDB puts in
// system.local and Cassandra does not have. Its release_version is not its own
// release. It is the Cassandra release it keeps compatible with, 3.0.8 on
// every release measured here, and it stays the main version, because that
// is the catalog ScyllaDB offers: system_schema as Cassandra 3.0 laid it out.
//
// The ScyllaDB release itself is in system.versions, which Cassandra does not
// have, so no statement that runs on both can read it. The Scylla key is
// recorded here as an unknown version, and [followUpQuery] reads the release.
// See D91 and D92.
func parseVersion(cols []string) (dbmeta.VersionSet, error) {
	var s dbmeta.VersionSet
	if len(cols) != 1 {
		return s, dbmeta.ErrInvalidVersion
	}
	var row map[string]any
	if err := json.Unmarshal([]byte(cols[0]), &row); err != nil {
		return s, fmt.Errorf("reading system.local: %w", dbmeta.ErrInvalidVersion)
	}
	release, _ := row["release_version"].(string)
	cql, _ := row["cql_version"].(string)
	protocol, _ := row["native_protocol_version"].(string)
	if release == "" || cql == "" || protocol == "" {
		return s, dbmeta.ErrInvalidVersion
	}
	s.Set("", dbmeta.ParseVersion(release))
	s.Set("cql", dbmeta.ParseVersion(cql))
	s.Set("protocol", dbmeta.ParseVersion(protocol))
	// The same line usql prints, so that a person reading either sees one
	// thing. The protocol is a bare number and carries a v, which is how
	// Cassandra's own documentation writes it.
	s.Display = "Cassandra " + release + ", CQL " + cql + ", Protocol v" + protocol
	if _, ok := row["supported_features"]; ok {
		s.Set(Scylla, dbmeta.Version{Unknown: true})
		// usql prints this line with "Cassandra" in front, and on ScyllaDB
		// that names the wrong product. The number is the compatibility
		// release, so the line says so.
		s.Display = "ScyllaDB, compatible with Cassandra " + release +
			", CQL " + cql + ", Protocol v" + protocol
	}
	return s, nil
}

// followUpQuery reads the ScyllaDB release, and asks nothing of Cassandra.
//
// system.versions is refused to a role that was granted nothing on it, while
// system.local is not. [dbmeta.Dialect.Version] keeps the first statement's
// answer when this one is refused, so such a role still learns that it is
// talking to ScyllaDB, and the release stays unknown. See D92.
func followUpQuery(s dbmeta.VersionSet) (string, int) {
	if !s.Has(Scylla) {
		return "", 0
	}
	return `SELECT version FROM system.versions WHERE key = 'local'`, 1
}

// parseFollowUp records the ScyllaDB release, such as
// 2026.3.1-0.20260904.97cbf7898aae, under the Scylla key and puts it in the
// display line.
func parseFollowUp(s dbmeta.VersionSet, cols []string) (dbmeta.VersionSet, error) {
	if len(cols) != 1 || cols[0] == "" {
		return s, dbmeta.ErrInvalidVersion
	}
	ver := dbmeta.ParseVersion(cols[0])
	if ver.Unknown {
		return s, dbmeta.ErrInvalidVersion
	}
	s.Set(Scylla, ver)
	s.Display = strings.Replace(s.Display, "ScyllaDB,", "ScyllaDB "+cols[0]+",", 1)
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
		// The syntax is usql's lexer flags for this product, and the fold
		// is measured by scanEveryQuery (D143).
		Syntax:  dbmeta.Syntax{DollarQuotes: true, BlockComments: true, SlashComments: true},
		Batches: []dbmeta.Batch{{Begin: "BEGIN BATCH", End: "APPLY BATCH"}},
		Fold:    dbmeta.FoldLower,
		// CQL binds by position and writes a question mark, the same as
		// MySQL. The number is not used.
		Placeholder:    func(int) string { return "?" },
		VersionQuery:   versionQuery,
		VersionColumns: 1,
		ParseVersion:   parseVersion,
		FollowUpQuery:  followUpQuery,
		ParseFollowUp:  parseFollowUp,
		ChangePassword: changePassword,
	})
	registerSchema()
	registerExtra()
	registerRoles()
}

// v40 is where the system_views keyspace arrived.
var v40 = dbmeta.V(4)

// always is a fragment every release takes.
func always(query string) dbmeta.Choice { return dbmeta.Choice{{Query: query}} }

// fixed selects a column whose value is known before the query runs, such as
// a padded NULL or a flag that is always false.
//
// On Cassandra the statement selects the literal, so that it says what the
// field holds. ScyllaDB accepts no literal in a select list: 2025.1 refuses
// (text)NULL, (boolean)false and even CAST(false AS boolean) as a syntax
// error, and 2026.3 refuses every NULL. So on ScyllaDB the fragment selects
// standIn, a real column of the same table, under the same name. The query
// then returns as many columns as it declares fields on both products.
//
// Scan discards the column on both products and sets the value itself, so one
// Scan reads either product. prefix is "SELECT " for the first column and
// ", " for any other.
func fixed(prefix, literal, standIn, name string) dbmeta.Choice {
	as := ` AS "` + name + `"`
	return dbmeta.Choice{{Query: prefix + literal + as}, scylla(prefix + standIn + as)}
}

// filters declares the filters a caller can pass.
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

// tableFilters declares the filters of Tables, which narrow nothing either,
// for the same reason as [filters]. types is declared because every other
// model takes it.
func tableFilters() []dbmeta.Param {
	const why = ", which Cassandra ignores: CQL cannot express an optional" +
		" filter, so every row is returned and the caller narrows it"
	types := dbmeta.TypesParam()
	types.Desc += why
	return append(filters("table"), types)
}

// childFilters declares the filters of a kind whose objects belong to a
// table, such as a column. They narrow nothing either, for the same reason as
// [filters], and parent is declared because every other model takes it for
// the table a child belongs to.
func childFilters(kind string) []dbmeta.Param {
	const why = ", which Cassandra ignores: CQL cannot express an optional" +
		" filter, so every row is returned and the caller narrows it"
	return append([]dbmeta.Param{{Name: "parent", Desc: "table name" + why, Default: ""}},
		filters(kind)...)
}

// pad is the scan target for a column whose value Scan already knows: a
// padded NULL, or a flag that is always false.
//
// On ScyllaDB the column is a real column standing in for a literal, because
// ScyllaDB accepts no literal in a select list, so its value means nothing and
// is discarded here. [fixed] holds that rule. The field keeps its zero value,
// or Scan sets the known one, which on a padded field is the invalid Null
// docs/NULLS.md asks for. The column is still selected, because a query returns
// as many columns as it declares fields.
//
// It began as a workaround for a driver fault. go-cql-driver sent an empty
// string for a CQL null, so a padded column arrived valid and empty.
// github.com/xo/cassandra reports a null as one, and D93 records the change.
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
