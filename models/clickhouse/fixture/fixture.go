// Package fixture builds the database the ClickHouse queries read.
//
// It creates one of every object those queries report, so a test has rows
// worth checking rather than an empty catalog. Hard rule 9 requires it, and
// D53 requires the core objects to match every other fixture, so that the
// cross family comparison reads the same schema everywhere.
//
// # A database is a schema
//
// ClickHouse has one level of namespace, so the fixture's database is what
// every other fixture calls its schema.
//
// # What ClickHouse cannot be asked for
//
// No foreign key and no unique constraint. A primary key orders the data and
// does not make it unique, so book carries an author_id that nothing enforces.
// No sequence, no trigger, no user defined type.
//
// No named collection either, and that one is a server setting rather than a
// missing feature. Creating one needs
// access_control_improvements.named_collection_control, which the official
// image exposes no variable for, so the foreign servers query is verified to
// run and returns no rows. See docs/COVERAGE.md.
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

// Everything is a database holding one of every object the ClickHouse queries
// read.
var Everything = Fixture{
	Name:   "everything",
	Schema: "dbmeta_fixture",
	Setup: []Step{
		at("database", `CREATE DATABASE dbmeta_fixture COMMENT 'the fixture'`),

		// The core objects D53 asks every fixture for. MergeTree is the
		// engine everything real uses, and it needs an ORDER BY, which is
		// what ClickHouse calls the primary key.
		at("author", `CREATE TABLE dbmeta_fixture.author (
	author_id Int32,
	name String COMMENT 'the author name',
	rating Nullable(Int32),
	shade String DEFAULT 'red',
	upper_name String MATERIALIZED upper(name)
) ENGINE = MergeTree ORDER BY author_id COMMENT 'the authors'`),

		// A skipping index and a constraint, so those two queries have a row.
		// Nothing enforces author_id: ClickHouse has no foreign key.
		at("book", `CREATE TABLE dbmeta_fixture.book (
	book_id Int32,
	author_id Int32,
	title String,
	published Nullable(Date),
	INDEX book_title_idx title TYPE set(100) GRANULARITY 1,
	CONSTRAINT book_title_ck CHECK length(title) > 0
) ENGINE = MergeTree ORDER BY (book_id, title)
	PARTITION BY book_id COMMENT 'the books'`),

		at("region", `CREATE TABLE dbmeta_fixture.region (
	country String,
	area String
) ENGINE = MergeTree ORDER BY (country, area) COMMENT 'the regions'`),

		at("shipment", `CREATE TABLE dbmeta_fixture.shipment (
	shipment_id Int32,
	country String,
	area String,
	amount Decimal(12, 2)
) ENGINE = MergeTree ORDER BY shipment_id COMMENT 'the shipments'`),

		// A plain view, which is a stored query rather than a table.
		at("view", `CREATE VIEW dbmeta_fixture.recent AS
	SELECT book_id, title FROM dbmeta_fixture.book`),

		// A table whose rows live somewhere else. Creating it opens no
		// connection, which is why a URL that answers nothing is fine.
		at("foreign table", `CREATE TABLE dbmeta_fixture.remote (
	id Int32,
	name String
) ENGINE = URL('http://127.0.0.1:1/none.csv', 'CSV')`),

		// A user, a role and a grant, so those three queries have a row.
		at("role", `CREATE ROLE dbmeta_reader`),
		at("user", `CREATE USER dbmeta_grantee IDENTIFIED BY 'P4ssw0rd'`),
		at("role grant", `GRANT dbmeta_reader TO dbmeta_grantee`),
		at("permission", `GRANT SELECT ON dbmeta_fixture.* TO dbmeta_reader`),

		// Rows, so a query reading data has something.
		at("author rows", `INSERT INTO dbmeta_fixture.author (author_id, name, rating)`+
			` VALUES (1, 'Ursula', 5)`),
		at("book rows", `INSERT INTO dbmeta_fixture.book`+
			` (book_id, author_id, title, published)`+
			` VALUES (1, 1, 'A Wizard of Earthsea', '1968-01-01')`),
		at("region rows", `INSERT INTO dbmeta_fixture.region (country, area)`+
			` VALUES ('US', 'west')`),
		at("shipment rows", `INSERT INTO dbmeta_fixture.shipment`+
			` (shipment_id, country, area, amount) VALUES (1, 'US', 'west', 12.50)`),
	},
	Teardown: []Step{
		// The database takes its tables, view and index with it. The user and
		// the role are outside it.
		at("database", `DROP DATABASE IF EXISTS dbmeta_fixture SYNC`),
		at("user", `DROP USER IF EXISTS dbmeta_grantee`),
		at("role", `DROP ROLE IF EXISTS dbmeta_reader`),
	},
}
