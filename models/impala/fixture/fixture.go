// Package fixture builds the objects the Impala queries read.
//
// It creates one of every object those queries report, so a test has rows
// worth checking. Hard rule 9 requires it, and D53 requires the core objects
// to match every other fixture.
//
// Impala keeps a primary key and a foreign key as information and enforces
// neither, and refuses NOT NULL on a Parquet table, so every column here is
// nullable. A function of a user's own needs a library file, which the image
// has none of, so the functions the queries read are the built in ones.
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

// Everything is a database holding one of every object the Impala queries
// read.
var Everything = Fixture{
	Name:   "everything",
	Schema: "dbmeta_fixture",
	Setup: []Step{
		at("schema", `CREATE DATABASE dbmeta_fixture COMMENT 'the dbmeta fixture'`),
		at("author", `CREATE TABLE dbmeta_fixture.author (
	author_id INT COMMENT 'surrogate key',
	name STRING,
	rating INT,
	shade STRING,
	PRIMARY KEY (author_id)
) COMMENT 'people who write' STORED AS PARQUET`),
		at("book", `CREATE TABLE dbmeta_fixture.book (
	book_id INT,
	author_id INT,
	title STRING,
	published DATE,
	PRIMARY KEY (book_id),
	FOREIGN KEY (author_id) REFERENCES dbmeta_fixture.author (author_id)
) PARTITIONED BY (year INT) STORED AS PARQUET`),
		at("region", `CREATE TABLE dbmeta_fixture.region (
	country STRING,
	area STRING,
	PRIMARY KEY (country, area)
) STORED AS PARQUET`),
		at("shipment", `CREATE TABLE dbmeta_fixture.shipment (
	shipment_id INT,
	country STRING,
	area STRING,
	amount DECIMAL(12, 2),
	PRIMARY KEY (shipment_id)
) STORED AS PARQUET`),
		at("view", `CREATE VIEW dbmeta_fixture.recent AS
	SELECT book_id, title FROM dbmeta_fixture.book WHERE published IS NOT NULL`),
		// Rows, and statistics over them, so the column statistics query
		// has values to report.
		at("author rows", `INSERT INTO dbmeta_fixture.author VALUES`+
			` (1, 'Ursula', 5, 'red'), (2, 'Octavia', 4, 'red'), (3, 'Iain', NULL, 'blue')`),
		at("statistics", `COMPUTE STATS dbmeta_fixture.author`),
	},
	Teardown: []Step{
		at("schema", `DROP DATABASE IF EXISTS dbmeta_fixture CASCADE`),
	},
}
