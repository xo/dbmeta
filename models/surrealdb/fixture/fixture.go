// Package fixture builds the objects the SurrealDB queries read.
//
// It creates one of every object those queries report, so a test has rows
// worth checking rather than an empty catalog. Hard rule 9 requires it, and
// D53 asks for the core objects of every fixture.
//
// It builds in the database dbmeta of the namespace dbmeta, which the dbrun
// setup makes, because every query below a schema reads the database the
// connection is in.
//
// # The core objects
//
// A table is SCHEMAFULL where the core object has columns, so that each
// column is a DEFINE FIELD. The record id is the key of every table, so
// author and book have a key of one field. region has no composite key,
// because a record id is one value, and a UNIQUE index on country and area
// stands in for it. shipment has no foreign key, because SurrealDB has none:
// a field of the type record<region> holds a link and the server does not
// check that the record exists. recent is a table defined AS SELECT, which
// is the view.
//
// # What SurrealDB cannot be asked for
//
// No current user: no statement names the system user of a session. No type,
// domain, collation or cast. A sequence needs 3.0, and a COMPUTED field 3.0
// too, so those steps are skipped on 2.7, where only the current schema is
// answered.
package fixture

import (
	"errors"

	"github.com/xo/dbmeta"
)

// Step is one statement the fixture runs.
type Step struct {
	Name string
	Stmt dbmeta.Stmt
}

// Result is what a step resolved to for one server.
type Result struct {
	Name    string
	Query   string
	Skipped bool
	Reason  string
}

// Fixture is the objects of one database and the statements that build and
// remove them.
type Fixture struct {
	Name     string
	Catalog  string
	Schema   string
	Setup    []Step
	Teardown []Step
}

// ResolveSetup returns the setup for a server.
func (f Fixture) ResolveSetup(versions dbmeta.VersionSet) ([]Result, error) {
	return resolve(f.Setup, versions)
}

// ResolveTeardown returns the teardown for a server.
func (f Fixture) ResolveTeardown(versions dbmeta.VersionSet) ([]Result, error) {
	return resolve(f.Teardown, versions)
}

func resolve(steps []Step, versions dbmeta.VersionSet) ([]Result, error) {
	out := make([]Result, 0, len(steps))
	for _, step := range steps {
		query, err := step.Stmt.Build(versions)
		switch {
		case errors.Is(err, dbmeta.ErrVersionTooOld):
			out = append(out, Result{
				Name:    step.Name,
				Skipped: true,
				Reason:  "the server is older than this step needs",
			})
		case err != nil:
			return nil, err
		default:
			out = append(out, Result{Name: step.Name, Query: query})
		}
	}
	return out, nil
}

// v3 is the first release with DEFINE SEQUENCE and COMPUTED.
var v3 = dbmeta.V(3)

// always is a step every release runs.
func always(name, query string) Step {
	return Step{Name: name, Stmt: dbmeta.Always(query)}
}

// from3 is a step that needs 3.0.
func from3(name, query string) Step {
	return Step{Name: name, Stmt: dbmeta.Stmt{{{Min: v3, Query: query}}}}
}

// both is a step that each line writes in its own grammar.
func both(name, old, query string) Step {
	return Step{Name: name, Stmt: dbmeta.Stmt{{{Query: old}, {Min: v3, Query: query}}}}
}

// Everything is the objects the SurrealDB queries read, in the database
// dbmeta.
var Everything = Fixture{
	Name:    "everything",
	Catalog: "dbmeta",
	Schema:  "dbmeta",
	Setup: []Step{
		// author: a key of one field, which is the record id, a field that
		// is not null, one that is, and one with a default.
		always("author", "DEFINE TABLE author SCHEMAFULL COMMENT 'people who write books'"),
		always("author.author_id", "DEFINE FIELD author_id ON author TYPE int"),
		always("author.name", "DEFINE FIELD name ON author TYPE string"),
		always("author.bio", "DEFINE FIELD bio ON author TYPE option<string>"),
		always("author.rating", "DEFINE FIELD rating ON author TYPE int DEFAULT 3 COMMENT 'out of five'"),
		// A field the server computes on each write, and one it computes on
		// each read.
		always("author.changed", "DEFINE FIELD changed ON author VALUE time::now() READONLY"),
		from3("author.shout", "DEFINE FIELD shout ON author COMPUTED string::uppercase(name)"),

		// book: a link to author, a check, a unique index and the core
		// index book_published, and an index on two fields.
		always("book", "DEFINE TABLE book SCHEMAFULL"),
		always("book.book_id", "DEFINE FIELD book_id ON book TYPE int"),
		always("book.author_id", "DEFINE FIELD author_id ON book TYPE record<author>"),
		always("book.title", "DEFINE FIELD title ON book TYPE string ASSERT string::len($value) > 0"),
		always("book.published", "DEFINE FIELD published ON book TYPE option<datetime>"),
		always("book.tags", "DEFINE FIELD tags ON book TYPE array<string> DEFAULT []"),
		always("book_title", "DEFINE INDEX book_title ON book FIELDS title UNIQUE COMMENT 'one book for each title'"),
		always("book_published", "DEFINE INDEX book_published ON book FIELDS published"),
		always("book_author", "DEFINE INDEX book_author ON book FIELDS author_id, title"),
		always("book_created", "DEFINE EVENT book_created ON book WHEN $event = 'CREATE'"+
			" THEN (CREATE log SET at = time::now()) COMMENT 'logs a new book'"),

		// region: what stands in for a composite key, and permissions that
		// differ from the default for a record user, on the table and on a
		// field.
		always("region", "DEFINE TABLE region SCHEMAFULL"+
			" PERMISSIONS FOR select FULL, FOR create WHERE $auth.admin = true, FOR update, delete NONE"),
		always("region.country", "DEFINE FIELD country ON region TYPE string"),
		always("region.area", "DEFINE FIELD area ON region TYPE string"+
			" PERMISSIONS FOR select FULL, FOR create, update WHERE $auth.admin = true"),
		always("region_key", "DEFINE INDEX region_key ON region FIELDS country, area UNIQUE"),

		// shipment: a link to region, which the server does not check.
		always("shipment", "DEFINE TABLE shipment SCHEMAFULL"),
		always("shipment.region", "DEFINE FIELD region ON shipment TYPE record<region>"),
		always("shipment.sent", "DEFINE FIELD sent ON shipment TYPE datetime"),

		// recent: a table defined AS SELECT, which is the view.
		always("recent", "DEFINE TABLE recent AS SELECT title, published FROM book WHERE published != NONE"),

		// wrote: a table of edges from author to book.
		always("wrote", "DEFINE TABLE wrote TYPE RELATION IN author OUT book"),

		// A function with two parameters and a result type.
		always("function", "DEFINE FUNCTION fn::full_title($title: string, $subtitle: string) -> string"+
			" { RETURN $title + ': ' + $subtitle; } COMMENT 'joins two titles'"),

		// A sequence, which arrived in 3.0.
		from3("sequence", "DEFINE SEQUENCE book_seq START 100"),

		// A param and an analyzer, which carry a comment. A full text index
		// is SEARCH ANALYZER on 2.x and FULLTEXT ANALYZER on 3.x.
		always("param", "DEFINE PARAM $max_rating VALUE 5 COMMENT 'the best rating'"),
		always("analyzer", "DEFINE ANALYZER simple TOKENIZERS blank FILTERS lowercase"),
		both("author_name", "DEFINE INDEX author_name ON author FIELDS name SEARCH ANALYZER simple BM25",
			"DEFINE INDEX author_name ON author FIELDS name FULLTEXT ANALYZER simple BM25"),

		// Records, so that the tables hold something.
		always("author rows", "CREATE author:ursula SET author_id = 1, name = 'Ursula', rating = 5"),
	},
	Teardown: []Step{
		always("recent", "REMOVE TABLE IF EXISTS recent"),
		always("wrote", "REMOVE TABLE IF EXISTS wrote"),
		always("shipment", "REMOVE TABLE IF EXISTS shipment"),
		always("region", "REMOVE TABLE IF EXISTS region"),
		always("book", "REMOVE TABLE IF EXISTS book"),
		always("author", "REMOVE TABLE IF EXISTS author"),
		always("log", "REMOVE TABLE IF EXISTS log"),
		always("function", "REMOVE FUNCTION IF EXISTS fn::full_title"),
		from3("sequence", "REMOVE SEQUENCE IF EXISTS book_seq"),
		always("param", "REMOVE PARAM IF EXISTS $max_rating"),
		always("analyzer", "REMOVE ANALYZER IF EXISTS simple"),
	},
}
