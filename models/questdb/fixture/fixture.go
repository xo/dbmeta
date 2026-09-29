// Package fixture builds the tables the QuestDB queries read.
//
// It creates one of every object those queries report, so a test has rows
// worth checking rather than an empty catalog. Hard rule 9 requires it, and
// D53 requires the core objects to match every other fixture, so that the
// cross family comparison reads the same schema everywhere.
//
// # One schema
//
// QuestDB has no schemas. Its information_schema and pg_catalog put every
// table in public, so that is the fixture's schema, and the tables are named
// as every other fixture names them.
//
// # What QuestDB cannot be asked for
//
// No primary key, no foreign key, no unique constraint, no check, no NOT NULL
// and no default. So every column is nullable and book carries an author_id
// that nothing enforces. No sequence, no trigger, no role in the open source
// edition, no comment and no function of a user's own.
//
// # What only QuestDB has
//
// book has a designated timestamp and is partitioned by year, which is what
// the partitioned tables query reads. It is a WAL table, because a
// materialized view needs one as its base. shade is a SYMBOL with an index,
// which no query can list, because the index flag is only in table_columns,
// which reads one table at a time. docs/COVERAGE.md says so.
//
// # Rows are visible later
//
// A WAL table applies an insert after it returns, so a test that reads rows
// waits for them. No catalog query reads rows, so the fixture does not wait.
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

// Fixture is a database and the statements that build and remove it.
type Fixture struct {
	Name     string
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

func at(name, query string) Step {
	return Step{Name: name, Stmt: dbmeta.Always(query)}
}

// Everything is the tables, the view and the materialized view that hold
// one of every object the QuestDB queries read.
var Everything = Fixture{
	Name:   "everything",
	Schema: "public",
	Setup: []Step{
		// The core objects D53 asks every fixture for.
		at("author", `CREATE TABLE author (
	author_id INT,
	name VARCHAR,
	rating INT,
	shade SYMBOL INDEX
)`),
		at("book", `CREATE TABLE book (
	book_id INT,
	author_id INT,
	title VARCHAR,
	published TIMESTAMP
) TIMESTAMP(published) PARTITION BY YEAR WAL`),
		at("region", `CREATE TABLE region (
	country SYMBOL,
	area SYMBOL
)`),
		at("shipment", `CREATE TABLE shipment (
	shipment_id INT,
	country SYMBOL,
	area SYMBOL,
	amount DECIMAL(12, 2)
)`),
		at("view", `CREATE VIEW recent AS (SELECT book_id, title FROM book)`),
		// A materialized view, which the views query reports beside the
		// view, and which is partitioned by year as its base table is.
		at("materialized view", `CREATE MATERIALIZED VIEW book_count AS (
	SELECT published, count() AS books FROM book SAMPLE BY 1y
)`),
		// 1978 rather than 1968, the year the book came out, because
		// QuestDB refuses a designated timestamp before 1970.
		at("book rows", `INSERT INTO book (book_id, author_id, title, published)`+
			` VALUES (1, 1, 'A Wizard of Earthsea', '1978-01-01')`),
	},
	Teardown: []Step{
		at("materialized view", `DROP MATERIALIZED VIEW IF EXISTS book_count`),
		at("view", `DROP VIEW IF EXISTS recent`),
		at("shipment", `DROP TABLE IF EXISTS shipment`),
		at("region", `DROP TABLE IF EXISTS region`),
		at("book", `DROP TABLE IF EXISTS book`),
		at("author", `DROP TABLE IF EXISTS author`),
	},
}
