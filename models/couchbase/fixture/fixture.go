// Package fixture builds the scope the Couchbase queries read.
//
// It creates one of every object those queries report, so a test has rows
// worth checking rather than an empty catalog. Hard rule 9 requires it, and
// D53 asks for the core objects of every fixture.
//
// It builds in the bucket dbmeta, which the dbrun setup makes, because SQL++
// can create a scope, a collection, an index, a function and a sequence and
// cannot create a bucket.
//
// # A collection is not there at once
//
// A collection that CREATE COLLECTION made is not visible to the next
// statement for a moment, and CREATE INDEX on it fails with "Keyspace not
// found". The test that runs this fixture tries such a step again until it
// succeeds, rather than the fixture waiting a fixed time.
//
// # What Couchbase cannot be asked for
//
// No view: SQL++ has none, so the core object recent does not exist here. No
// constraint of any kind, no comment, no trigger and no type.
//
// No group. A group holds roles for its users, and SQL++ on 7.6 and 8.0 has
// no statement that creates one, so RoleGrants is verified to run and returns
// no fixture row.
//
// No JavaScript function. Its body lives in a library, which the REST
// interface manages and SQL++ cannot create.
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

// Fixture is a scope and the statements that build and remove it.
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

// v76 is the floor of the model, which the fixture shares.
var v76 = dbmeta.V(7, 6)

func at(name, query string) Step {
	return Step{Name: name, Stmt: dbmeta.Stmt{{{Min: v76, Query: query}}}}
}

// Everything is a scope holding one of every object the Couchbase queries
// read.
var Everything = Fixture{
	Name:    "everything",
	Catalog: "dbmeta",
	Schema:  "dbmeta_fixture",
	Setup: []Step{
		// A scope is what every other fixture calls a schema, and it sits in
		// the bucket the dbrun setup made.
		at("scope", "CREATE SCOPE `dbmeta`.`dbmeta_fixture` IF NOT EXISTS"),

		// The core tables of D53, as collections. A collection has no
		// columns, so what makes each one what it is lives in its documents.
		at("author", "CREATE COLLECTION `dbmeta`.`dbmeta_fixture`.`author` IF NOT EXISTS"),
		at("book", "CREATE COLLECTION `dbmeta`.`dbmeta_fixture`.`book` IF NOT EXISTS"),
		at("region", "CREATE COLLECTION `dbmeta`.`dbmeta_fixture`.`region` IF NOT EXISTS"),
		at("shipment", "CREATE COLLECTION `dbmeta`.`dbmeta_fixture`.`shipment` IF NOT EXISTS"),

		// A primary index, so that the collection can be read with no other,
		// and the core index book_published, on one field.
		at("primary index", "CREATE PRIMARY INDEX IF NOT EXISTS ON `dbmeta`.`dbmeta_fixture`.`book`"),
		at("book_published", "CREATE INDEX `book_published` IF NOT EXISTS"+
			" ON `dbmeta`.`dbmeta_fixture`.`book`(`published`)"),

		// A key in each shape the index columns query reports: two fields,
		// one of them descending, and an expression over an array.
		at("book_author", "CREATE INDEX `book_author` IF NOT EXISTS"+
			" ON `dbmeta`.`dbmeta_fixture`.`book`(`author_id`, `title` DESC)"),
		// A partial index, so that the index has a condition, and a setting
		// in WITH, so that it has options.
		at("book_recent", "CREATE INDEX `book_recent` IF NOT EXISTS"+
			" ON `dbmeta`.`dbmeta_fixture`.`book`(`published`) WHERE `published` > '2000-01-01'"+
			" WITH {\"num_replica\": 0}"),
		at("book_tags", "CREATE INDEX `book_tags` IF NOT EXISTS"+
			" ON `dbmeta`.`dbmeta_fixture`.`book`(DISTINCT ARRAY t FOR t IN `tags` END)"),

		// An inline function with two parameters, and a variadic one, whose
		// one parameter is written ...
		at("function", "CREATE OR REPLACE FUNCTION `dbmeta`.`dbmeta_fixture`.`full_title`(title, subtitle)"+
			" { title || ': ' || subtitle }"),
		at("variadic function", "CREATE OR REPLACE FUNCTION `dbmeta`.`dbmeta_fixture`.`total`(...)"+
			" { ARRAY_SUM(args) }"),

		// A sequence, which arrived in 7.6.
		at("sequence", "CREATE SEQUENCE IF NOT EXISTS `dbmeta`.`dbmeta_fixture`.`book_seq`"+
			" START WITH 1 INCREMENT BY 1"),

		// A role on one collection for the ordinary user the dbrun setup
		// made, so that the privileges query has a row at that level. The
		// setup resets the user's roles on its next start.
		at("grant", "GRANT query_select ON `dbmeta`.`dbmeta_fixture`.`book` TO dbmeta_user"),

		// Documents, so that the collections hold something.
		at("author rows", "UPSERT INTO `dbmeta`.`dbmeta_fixture`.`author` (KEY, VALUE)"+
			" VALUES ('author::1', {'author_id': 1, 'name': 'Ursula', 'rating': 5})"),
		at("book rows", "UPSERT INTO `dbmeta`.`dbmeta_fixture`.`book` (KEY, VALUE)"+
			" VALUES ('book::1', {'book_id': 1, 'author_id': 1, 'title': 'A Wizard of Earthsea',"+
			" 'published': '1968-01-01', 'tags': ['fantasy']})"),
	},
	Teardown: []Step{
		// The scope takes its collections, indexes, functions and sequence
		// with it. The grant is on the user and goes separately.
		at("grant", "REVOKE query_select ON `dbmeta`.`dbmeta_fixture`.`book` FROM dbmeta_user"),
		at("scope", "DROP SCOPE `dbmeta`.`dbmeta_fixture` IF EXISTS"),
	},
}
