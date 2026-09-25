// Package fixture holds a known good DuckDB schema.
//
// Like every fixture here it is exported API and additive: a later release may
// add an object and will not rename or remove one. See the PostgreSQL fixture
// for the rules, which are the same.
//
// It is not versioned. DuckDB is a library, so the release is whichever one
// the caller linked and there is nothing for a step to be too new for. The
// Resolve methods keep the shape the other fixtures have so that a caller can
// treat them alike.
package fixture

import "github.com/xo/dbmeta"

// Step is one statement of a fixture.
type Step struct {
	Name string
	Stmt dbmeta.Stmt
}

// Result is what a step resolved to.
type Result struct {
	Name    string
	Query   string
	Skipped bool
	Reason  string
}

// Fixture is a schema, with the statements that build it and drop it.
type Fixture struct {
	Name     string
	Schema   string
	Setup    []Step
	Teardown []Step
}

// ResolveSetup returns the statements that build the fixture.
func (f Fixture) ResolveSetup(versions dbmeta.VersionSet) ([]Result, error) {
	return resolve(f.Setup, versions)
}

// ResolveTeardown returns the statements that drop it.
func (f Fixture) ResolveTeardown(versions dbmeta.VersionSet) ([]Result, error) {
	return resolve(f.Teardown, versions)
}

func resolve(steps []Step, versions dbmeta.VersionSet) ([]Result, error) {
	out := make([]Result, 0, len(steps))
	for _, step := range steps {
		query, err := step.Stmt.Build(versions)
		if err != nil {
			return nil, err
		}
		out = append(out, Result{Name: step.Name, Query: query})
	}
	return out, nil
}

func at(name, query string) Step {
	return Step{Name: name, Stmt: dbmeta.Always(query)}
}

// Everything is a schema holding one of every object the DuckDB queries read.
//
// The names match the other fixtures, so a test that reads author and book on
// PostgreSQL reads the same two here. That is deliberate and it is what makes
// a cross database comparison possible at all.
//
// Do not name the database file dbmeta_fixture. DuckDB names the catalog after
// the file, and a catalog and a schema of the same name are ambiguous in a
// qualified reference.
var Everything = Fixture{
	Name:   "everything",
	Schema: "dbmeta_fixture",
	Setup: []Step{
		at("schema", `CREATE SCHEMA dbmeta_fixture`),
		// No comment on the schema. DuckDB 1.5 refuses one, with "Adding
		// comments to schemas is not implemented", although duckdb_schemas
		// carries the column. The other fixtures comment their schema.

		at("enum type", `CREATE TYPE dbmeta_fixture.colour AS ENUM ('red', 'green', 'blue')`),
		at("sequence", `CREATE SEQUENCE dbmeta_fixture.counter START 10 INCREMENT 2`),

		at("author table", `CREATE TABLE dbmeta_fixture.author (
	author_id INTEGER PRIMARY KEY,
	name VARCHAR NOT NULL,
	rating INTEGER,
	shade dbmeta_fixture.colour DEFAULT 'red'
)`),
		at("author comment", `COMMENT ON TABLE dbmeta_fixture.author IS 'people who write'`),
		at("author column comment",
			`COMMENT ON COLUMN dbmeta_fixture.author.author_id IS 'surrogate key'`),

		at("book table", `CREATE TABLE dbmeta_fixture.book (
	book_id INTEGER PRIMARY KEY,
	author_id INTEGER NOT NULL REFERENCES dbmeta_fixture.author(author_id),
	title VARCHAR NOT NULL UNIQUE,
	published DATE,
	CONSTRAINT title_not_empty CHECK (title <> '')
)`),
		at("book index", `CREATE INDEX book_published ON dbmeta_fixture.book (published)`),

		at("view", `CREATE VIEW dbmeta_fixture.recent AS
	SELECT book_id, title FROM dbmeta_fixture.book WHERE published IS NOT NULL`),

		// A composite primary key and a composite foreign key, so that
		// ConstraintColumns has more than one column per constraint to order.
		at("region table", `CREATE TABLE dbmeta_fixture.region (
	country VARCHAR NOT NULL,
	area VARCHAR NOT NULL,
	PRIMARY KEY (country, area)
)`),
		at("shipment table", `CREATE TABLE dbmeta_fixture.shipment (
	shipment_id INTEGER PRIMARY KEY,
	country VARCHAR NOT NULL,
	area VARCHAR NOT NULL,
	amount INTEGER NOT NULL,
	CONSTRAINT shipment_region_fk FOREIGN KEY (country, area)
		REFERENCES dbmeta_fixture.region(country, area)
)`),

		// A macro is the only routine a caller can create, and it is what
		// gives RoutineParameters a row that is not built in.
		at("macro", `CREATE MACRO dbmeta_fixture.addup(a, b) AS a + b`),
	},
	Teardown: []Step{
		at("drop schema", `DROP SCHEMA IF EXISTS dbmeta_fixture CASCADE`),
	},
}
