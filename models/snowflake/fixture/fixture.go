// Package fixture builds the objects the Snowflake queries read.
//
// It creates one of every object those queries report, so a test has rows
// worth checking. Hard rule 9 requires it, and D53 requires the core objects
// to match every other fixture. A key is declared and not enforced.
//
// It ran on a trial account on 2026-10-08, release 10.36.101, and built on the
// first try. See D144 and D190.
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

// Everything is a schema holding one of every object the Snowflake queries
// read.
var Everything = Fixture{
	Name:   "everything",
	Schema: "DBMETA_FIXTURE",
	Setup: []Step{
		at("schema", `CREATE SCHEMA dbmeta_fixture COMMENT = 'the dbmeta fixture'`),
		at("author", `CREATE TABLE dbmeta_fixture.author (
	author_id INTEGER AUTOINCREMENT PRIMARY KEY COMMENT 'surrogate key',
	name VARCHAR(255) NOT NULL,
	rating INTEGER,
	shade VARCHAR(16) DEFAULT 'red'
) COMMENT = 'people who write'`),
		at("book", `CREATE TABLE dbmeta_fixture.book (
	book_id INTEGER PRIMARY KEY,
	author_id INTEGER NOT NULL,
	title VARCHAR(255) COLLATE 'en-ci' NOT NULL,
	published DATE,
	CONSTRAINT book_author_fk FOREIGN KEY (author_id) REFERENCES dbmeta_fixture.author (author_id),
	CONSTRAINT book_title_unique UNIQUE (title)
)`),
		at("region", `CREATE TABLE dbmeta_fixture.region (
	country VARCHAR(64) NOT NULL,
	area VARCHAR(64) NOT NULL,
	PRIMARY KEY (country, area)
)`),
		at("shipment", `CREATE TABLE dbmeta_fixture.shipment (
	shipment_id INTEGER PRIMARY KEY,
	country VARCHAR(64) NOT NULL,
	area VARCHAR(64) NOT NULL,
	amount NUMBER(12, 2) NOT NULL,
	CONSTRAINT shipment_region_fk FOREIGN KEY (country, area) REFERENCES dbmeta_fixture.region (country, area)
)`),
		at("view", `CREATE VIEW dbmeta_fixture.recent AS
	SELECT book_id, title FROM dbmeta_fixture.book WHERE published IS NOT NULL`),
		// A transient table with a clustering key and no time travel, and
		// three rows, for the fields of Table (D207).
		at("transient table", `CREATE TRANSIENT TABLE dbmeta_fixture.events (
	event_id INTEGER,
	happened TIMESTAMP_NTZ
) CLUSTER BY (event_id) DATA_RETENTION_TIME_IN_DAYS = 0`),
		at("author rows", `INSERT INTO dbmeta_fixture.author (name, rating)`+
			` VALUES ('Ursula', 5), ('Octavia', 4), ('Iain', 4)`),
		at("sequence", `CREATE SEQUENCE dbmeta_fixture.counter START = 10 INCREMENT = 2`),
		at("function", `CREATE FUNCTION dbmeta_fixture.shout(s VARCHAR) RETURNS VARCHAR`+
			` AS 'UPPER(s)'`),
		at("procedure", `CREATE PROCEDURE dbmeta_fixture.addup(a INTEGER, b INTEGER) RETURNS INTEGER`+
			` LANGUAGE SQL AS 'BEGIN RETURN a + b; END'`),
	},
	Teardown: []Step{
		at("schema", `DROP SCHEMA IF EXISTS dbmeta_fixture CASCADE`),
	},
}
