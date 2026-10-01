// Package fixture builds the objects the Databend queries read.
//
// It creates one of every object those queries report, so a test has rows
// worth checking rather than an empty catalog. Hard rule 9 requires it, and
// D53 requires the core objects to match every other fixture, so that the
// cross family comparison reads the same schema everywhere.
//
// # What Databend cannot be asked for
//
// No primary key, no foreign key and no unique constraint, so the core
// tables carry the columns and not the keys. A CHECK is the one constraint.
// A computed column needs an Enterprise license on 1.2.881, and the fixture
// has none, so no column is computed.
//
// # What belongs to no database
//
// A function, a procedure, a sequence, a role and a user belong to the
// tenant. Each is named with dbmeta_fixture in front, and the teardown drops
// each one by name, because dropping the database leaves them.
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

// Everything is the database and the objects beside it that hold one of
// every object the Databend queries read.
var Everything = Fixture{
	Name:   "everything",
	Schema: "dbmeta_fixture",
	Setup: []Step{
		at("schema", `CREATE DATABASE dbmeta_fixture`),
		// The core objects D53 asks every fixture for.
		at("author", `CREATE TABLE dbmeta_fixture.author (
	author_id INT NOT NULL COMMENT 'surrogate key',
	name VARCHAR NOT NULL,
	rating INT,
	shade VARCHAR DEFAULT 'red'
) COMMENT = 'people who write'`),
		// A CHECK, the one constraint Databend has, and a cluster key.
		at("book", `CREATE TABLE dbmeta_fixture.book (
	book_id INT NOT NULL,
	author_id INT NOT NULL,
	title VARCHAR NOT NULL,
	body VARCHAR,
	published DATE,
	CONSTRAINT title_not_empty CHECK (title <> '')
) CLUSTER BY (published)`),
		at("region", `CREATE TABLE dbmeta_fixture.region (
	country VARCHAR NOT NULL,
	area VARCHAR NOT NULL
)`),
		at("shipment", `CREATE TABLE dbmeta_fixture.shipment (
	shipment_id INT NOT NULL,
	country VARCHAR NOT NULL,
	area VARCHAR NOT NULL,
	amount DECIMAL(12, 2) NOT NULL
)`),
		at("view", `CREATE VIEW dbmeta_fixture.recent AS
	SELECT book_id, title FROM dbmeta_fixture.book WHERE published IS NOT NULL`),

		// An inverted index and an ngram index, which is what the indexes
		// query lists.
		at("inverted index", `CREATE INVERTED INDEX book_body ON dbmeta_fixture.book(body)`),
		at("ngram index", `CREATE NGRAM INDEX book_title ON dbmeta_fixture.book(title)`),

		// A function and a procedure, which belong to no database.
		at("function", `CREATE FUNCTION dbmeta_fixture_shout AS (s) -> upper(s)`),
		at("procedure", `CREATE PROCEDURE dbmeta_fixture_addup(a INT, b INT) RETURNS INT`+
			` LANGUAGE SQL AS $$ BEGIN RETURN a + b; END; $$`),
		at("sequence", `CREATE SEQUENCE dbmeta_fixture_counter`),

		// A role that another role and a user are granted, so the role
		// grants query has both kinds of row.
		at("role", `CREATE ROLE dbmeta_fixture_reader`),
		at("privilege", `GRANT SELECT ON dbmeta_fixture.* TO ROLE dbmeta_fixture_reader`),
		at("second role", `CREATE ROLE dbmeta_fixture_writer`),
		at("role grant to a role", `GRANT ROLE dbmeta_fixture_reader TO ROLE dbmeta_fixture_writer`),
		at("user", `CREATE USER dbmeta_fixture_member IDENTIFIED BY 'P4ssw0rd'`+
			` WITH DEFAULT_ROLE = 'dbmeta_fixture_reader'`),
		at("role grant to a user", `GRANT ROLE dbmeta_fixture_reader TO dbmeta_fixture_member`),

		// Rows, and statistics over them, so the column statistics query
		// has values to report.
		at("author rows", `INSERT INTO dbmeta_fixture.author (author_id, name, rating)`+
			` VALUES (1, 'Ursula', 5), (2, 'Octavia', 4), (3, 'Iain', NULL)`),
		at("analyze", `ANALYZE TABLE dbmeta_fixture.author`),
	},
	Teardown: []Step{
		at("user", `DROP USER IF EXISTS dbmeta_fixture_member`),
		at("second role", `DROP ROLE IF EXISTS dbmeta_fixture_writer`),
		at("role", `DROP ROLE IF EXISTS dbmeta_fixture_reader`),
		at("sequence", `DROP SEQUENCE IF EXISTS dbmeta_fixture_counter`),
		at("procedure", `DROP PROCEDURE IF EXISTS dbmeta_fixture_addup(INT, INT)`),
		at("function", `DROP FUNCTION IF EXISTS dbmeta_fixture_shout`),
		at("schema", `DROP DATABASE IF EXISTS dbmeta_fixture`),
	},
}
