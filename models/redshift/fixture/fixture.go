// Package fixture builds the objects the Redshift queries read.
//
// It creates one of every object those queries report, so a test has rows
// worth checking. Hard rule 9 requires it, and D53 requires the core objects
// to match every other fixture. A key is declared and not enforced.
//
// It was written from Redshift's documentation and has not run, because no
// cluster is provisioned yet. See D144.
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

// Everything is a schema holding one of every object the Redshift queries
// read.
var Everything = Fixture{
	Name:   "everything",
	Schema: "dbmeta_fixture",
	Setup: []Step{
		at("schema", `CREATE SCHEMA dbmeta_fixture`),
		at("author", `CREATE TABLE dbmeta_fixture.author (
	author_id INTEGER IDENTITY(1, 1) PRIMARY KEY,
	name VARCHAR(255) NOT NULL,
	rating INTEGER,
	shade VARCHAR(16) DEFAULT 'red'
)`),
		at("author comment", `COMMENT ON TABLE dbmeta_fixture.author IS 'people who write'`),
		at("author column comment", `COMMENT ON COLUMN dbmeta_fixture.author.author_id IS 'surrogate key'`),
		at("book", `CREATE TABLE dbmeta_fixture.book (
	book_id INTEGER PRIMARY KEY,
	author_id INTEGER NOT NULL REFERENCES dbmeta_fixture.author (author_id),
	title VARCHAR(255) NOT NULL UNIQUE,
	published DATE
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
	amount DECIMAL(12, 2) NOT NULL,
	FOREIGN KEY (country, area) REFERENCES dbmeta_fixture.region (country, area)
)`),
		at("view", `CREATE VIEW dbmeta_fixture.recent AS
	SELECT book_id, title FROM dbmeta_fixture.book WHERE published IS NOT NULL`),
		at("function", `CREATE FUNCTION dbmeta_fixture.f_shout(VARCHAR) RETURNS VARCHAR`+
			` STABLE AS $$ SELECT UPPER($1) $$ LANGUAGE sql`),
	},
	Teardown: []Step{
		at("schema", `DROP SCHEMA IF EXISTS dbmeta_fixture CASCADE`),
	},
}
