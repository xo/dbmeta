// Package cosmos holds the metadata queries for Azure Cosmos DB, with the API
// for NoSQL.
//
// Import it for its effect:
//
//	import _ "github.com/xo/dbmeta/models/cosmos"
//
// # What it reads
//
// It reads the hosted service. The SQL of Cosmos DB reads the documents of one
// container and nothing else, so the catalog is not in it. A database, a
// container, its policies, its scripts, its users and its throughput are
// resources of the REST API. The driver of dbimp, which is the one dburl names
// for the cosmos scheme, answers a SELECT against nine reserved names with a
// GET request on the matching resource, and every statement here is one of
// them. D223 first left Cosmos DB without a model because the driver sent no
// such request, and D228 builds the model now that it does.
//
// A statement names no database and no container. The driver takes both from
// the path of the URL, as cosmos://x:key@account.documents.azure.com/database
// or .../database/container. So a connection reads one database, and the
// stored procedures, the triggers and the functions of the one container that
// the URL names. A URL with no container makes the driver refuse those
// statements before it sends a request. Nothing in the statement can name
// another database. The grammar of the driver can name one in a WHERE, but a
// parameter of dbmeta is a pattern and the key takes an exact name, and reading
// every database is a walk, which D223 does not allow for Cosmos DB.
//
// # A statement cannot filter
//
// The grammar of a catalog statement has no LIKE and no list of columns. Each
// statement is SELECT * FROM a reserved name, and the Keep function of each
// binding narrows the rows in Go with dbmeta.Like, as the Cassandra model does
// (D200). The columns are the ones that the driver documents, in its order, so
// every binding names them as its fields, and Scan reads them by name.
//
// # What it answers
//
// 10 of the 65. Databases, schemas, tables, indexes, partitioned tables,
// functions, triggers, roles, privileges and settings. Every one is an
// analogue, and docs/COVERAGE.md says why. A database is a database and also a
// schema, because it holds containers the way a schema holds tables and
// nothing lies between it and the account. A container is a table. Its
// indexing policy is its one index, its partition key makes it a table
// partitioned by hash, and its user defined functions and triggers are
// functions and triggers. A user is a role, a permission is a privilege, and a
// throughput offer is a setting.
//
// The rest is absent or out of reach of one statement. A document has no
// declared attribute, so there is no column, and sampling documents is a read
// of the data, which D47 does not allow. A container row holds many index
// paths, many unique keys and many computed properties, and a statement can
// return one object for a row only, so index columns, constraints and
// constraint columns are not answered. Stored procedures have no kind of their
// own, and the one statement of Functions reads user defined functions.
//
// # The version
//
// Cosmos DB is a service that has no release, and no statement reports one. The
// model declares no version query, so the version is unknown. usql declares no
// Version for its driver either. See D228.
//
// # Throughput and cost
//
// A request unit is the cost that Cosmos DB charges for a request. Reading a
// feed costs 2 request units for a page, measured on the hosted account. A
// statement reads one feed for a database, a container or a user, so the cost
// grows with the number of containers or users and not with the data. The
// permissions of a database take one request for the users and one for each
// user.
package cosmos

import (
	"database/sql"
	"encoding/json"
	"strconv"
	"strings"

	"github.com/xo/dbmeta"
)

func init() {
	dbmeta.RegisterDialect(dbmeta.Cosmos, &dbmeta.Info{
		// The SQL of Cosmos DB has comments that start with two hyphens and
		// names in double quotes, and it keeps the case of a name. The server
		// refuses a semicolon at the end of a statement, so a client removes it.
		Terminator: dbmeta.TerminatorStripped,
		// A catalog statement has no parameter, so this is never called. The
		// server binds a named parameter written @name.
		Placeholder: func(n int) string { return "@p" + strconv.Itoa(n) },
	})
	registerRelations()
	registerRoutines()
	registerRoles()
}

// always is a fragment every release takes.
func always(query string) dbmeta.Choice { return dbmeta.Choice{{Query: query}} }

// catalog returns the statement that reads one reserved name.
func catalog(name string) dbmeta.Stmt {
	return dbmeta.Stmt{always(`SELECT * FROM "` + name + `"`)}
}

// field declares a column of a catalog statement.
func field(name, desc string) dbmeta.Field { return dbmeta.Field{Name: name, Desc: desc} }

// row is one row of a catalog statement, by column name.
type row map[string]any

// scanRow reads the columns of the current row. The driver gives a list as a
// []any, a policy as a map[string]any, a number as an int64 and a missing value
// as nil, and Scan into an any keeps each as it is.
func scanRow(rows *sql.Rows) (row, error) {
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	vals := make([]any, len(cols))
	ptrs := make([]any, len(cols))
	for i := range vals {
		ptrs[i] = &vals[i]
	}
	if err := rows.Scan(ptrs...); err != nil {
		return nil, err
	}
	r := make(row, len(cols))
	for i, c := range cols {
		r[c] = vals[i]
	}
	return r, nil
}

// text returns the string in column name, and the empty string for NULL.
func (r row) text(name string) string {
	s, _ := r[name].(string)
	return s
}

// nullText returns the string in column name, absent for NULL.
func (r row) nullText(name string) sql.Null[string] {
	s, ok := r[name].(string)
	return sql.Null[string]{V: s, Valid: ok}
}

// number returns the integer in column name, absent for NULL.
func (r row) number(name string) sql.Null[int64] {
	n, ok := r[name].(int64)
	return sql.Null[int64]{V: n, Valid: ok}
}

// list returns the strings in the list in column name.
func (r row) list(name string) []string {
	items, _ := r[name].([]any)
	out := make([]string, 0, len(items))
	for _, item := range items {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

// options collects name=value pairs, and joins them with a comma and a space
// as the options of every kind are written. A pair with an empty value is left
// out. It returns an absent value when no pair is left.
type options []string

// add appends name=value when value is not empty.
func (o *options) add(name, value string) {
	if value != "" {
		*o = append(*o, name+"="+value)
	}
}

// addNumber appends name=value when the number is present.
func (o *options) addNumber(name string, n sql.Null[int64]) {
	if n.Valid {
		*o = append(*o, name+"="+strconv.FormatInt(n.V, 10))
	}
}

// value returns the pairs as one text, absent when there are none.
func (o options) value() sql.Null[string] {
	if len(o) == 0 {
		return sql.Null[string]{}
	}
	return sql.Null[string]{V: strings.Join(o, ", "), Valid: true}
}

// jsonText writes v as JSON text, with the keys of a map sorted.
func jsonText(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(b)
}

// arg returns the pattern that the caller gave for name.
func arg(args map[string]any, name string) string {
	s, _ := args[name].(string)
	return s
}

// keep returns the Keep function of a kind. Each of schema, parent and name
// reads the value that the filter of that name matches, and a nil one means the
// kind has none, so only a pattern that matches the empty string keeps the row.
func keep[T any](schema, parent, name func(T) string) func(T, map[string]any) bool {
	read := func(f func(T) string, v T) string {
		if f == nil {
			return ""
		}
		return f(v)
	}
	return func(v T, args map[string]any) bool {
		return dbmeta.Like(arg(args, "schema"), read(schema, v)) &&
			dbmeta.Like(arg(args, "parent"), read(parent, v)) &&
			dbmeta.Like(arg(args, "name"), read(name, v))
	}
}

// Parameters. A statement takes none, so each one narrows the rows in Go.

func nameParam(kind string) dbmeta.Param {
	return dbmeta.Param{Name: "name", Desc: kind + " name pattern, empty for every " + kind, Default: ""}
}

func schemaParam() dbmeta.Param {
	return dbmeta.Param{
		Name:    "schema",
		Desc:    "database name pattern, empty for every database. A statement reads the database that the URL names, so a pattern can only narrow that one",
		Default: "",
	}
}

func parentParam(kind string) dbmeta.Param {
	return dbmeta.Param{Name: "parent", Desc: kind + " name pattern, empty for every " + kind, Default: ""}
}

// accountFilters are the parameters of a kind that is not in a database.
func accountFilters(kind string) []dbmeta.Param { return []dbmeta.Param{nameParam(kind)} }

// databaseFilters are the parameters of a kind that is in a database.
func databaseFilters(kind string) []dbmeta.Param {
	return []dbmeta.Param{schemaParam(), nameParam(kind)}
}

// containerFilters are the parameters of a kind that is in a container.
func containerFilters(kind string) []dbmeta.Param {
	return []dbmeta.Param{schemaParam(), parentParam("container"), nameParam(kind)}
}
