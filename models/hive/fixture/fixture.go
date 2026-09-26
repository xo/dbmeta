// Package fixture builds the schema the Apache Hive queries read.
//
// It creates one of every object those queries report, so a test has rows
// worth checking rather than an empty catalog. Hard rule 9 requires it.
//
// # It does not build every core object, and it cannot
//
// D53 asks every fixture for six core objects and one of them is an index
// somebody created. Hive removed indexes in 3.0 and there is no statement
// that makes one, so this fixture builds five of the six and Hive stays out
// of the cross family comparison in the root module, the way Trino and
// Cassandra do. test/conform_test.go still reads it, because that comparison
// is per database rather than across the family.
//
// # What Hive cannot be asked for
//
// No constraint that is enforced. Hive accepts PRIMARY KEY, FOREIGN KEY,
// UNIQUE, NOT NULL and DEFAULT and records them in the metastore, and every
// one has to be declared DISABLE NOVALIDATE because Hive enforces none of
// them. They are declarations for a planner, and the queries read them as
// the catalog records them.
//
// No sequence, no trigger and no index, because Hive has none.
//
// No function. CREATE FUNCTION registers a Java class by name, so a fixture
// that made one would depend on a class being on the server's path. Functions
// is verified to run and returns nothing.
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

// Everything holds one of every object the Hive queries read.
var Everything = Fixture{
	Name:   "everything",
	Schema: "dbmeta_fixture",
	Setup: []Step{
		at("schema", `CREATE DATABASE dbmeta_fixture COMMENT 'the fixture'`),

		// Five of D53's six core objects. The sixth is an index and Hive
		// has none. Every constraint carries DISABLE NOVALIDATE, which
		// Hive requires because it enforces none of them.
		at("author", `CREATE TABLE dbmeta_fixture.author (
	author_id int NOT NULL DISABLE NOVALIDATE COMMENT 'the key',
	name string NOT NULL DISABLE NOVALIDATE COMMENT 'the author name',
	rating int,
	shade string DEFAULT 'plain' DISABLE NOVALIDATE,
	PRIMARY KEY (author_id) DISABLE NOVALIDATE
) COMMENT 'people who write'`),
		at("book", `CREATE TABLE dbmeta_fixture.book (
	book_id int NOT NULL DISABLE NOVALIDATE,
	author_id int NOT NULL DISABLE NOVALIDATE,
	title string NOT NULL DISABLE NOVALIDATE,
	published date,
	PRIMARY KEY (book_id) DISABLE NOVALIDATE,
	CONSTRAINT book_author_fk FOREIGN KEY (author_id)
		REFERENCES dbmeta_fixture.author (author_id) DISABLE NOVALIDATE,
	CONSTRAINT book_title_uq UNIQUE (title) DISABLE NOVALIDATE,
	CONSTRAINT book_title_ck CHECK (length(title) > 0) DISABLE NOVALIDATE
)`),
		at("region", `CREATE TABLE dbmeta_fixture.region (
	country string NOT NULL DISABLE NOVALIDATE,
	area string NOT NULL DISABLE NOVALIDATE,
	PRIMARY KEY (country, area) DISABLE NOVALIDATE
)`),
		at("shipment", `CREATE TABLE dbmeta_fixture.shipment (
	shipment_id int NOT NULL DISABLE NOVALIDATE,
	country string NOT NULL DISABLE NOVALIDATE,
	area string NOT NULL DISABLE NOVALIDATE,
	amount decimal(12, 2) NOT NULL DISABLE NOVALIDATE,
	PRIMARY KEY (shipment_id) DISABLE NOVALIDATE,
	CONSTRAINT shipment_region_fk FOREIGN KEY (country, area)
		REFERENCES dbmeta_fixture.region (country, area) DISABLE NOVALIDATE
)`),
		at("recent", `CREATE VIEW dbmeta_fixture.recent AS
	SELECT book_id, title FROM dbmeta_fixture.book`),

		// A NOT NULL and a DEFAULT, which Hive records as constraints
		// where PostgreSQL records them on the column. Columns reads
		// both through KEY_CONSTRAINTS.
		at("ticket", `CREATE TABLE dbmeta_fixture.ticket (
	ticket_id int NOT NULL DISABLE NOVALIDATE,
	note string DEFAULT 'none' DISABLE NOVALIDATE
)`),

		// A partitioned table. A partition column is not a column of the
		// table in Hive: it is in PARTITION_KEYS and PartitionedTables
		// is the only query that sees it.
		at("archive", `CREATE TABLE dbmeta_fixture.archive (
	archive_id int,
	note string
) PARTITIONED BY (year int COMMENT 'the year')`),

		// A table stored another way, so AccessMethods reports more than
		// one SerDe and does not merely say that every table is the
		// default.
		at("ledger", `CREATE TABLE dbmeta_fixture.ledger (
	entry_id int,
	note string
) STORED AS ORC`),
	},
	Teardown: []Step{
		at("schema", `DROP DATABASE IF EXISTS dbmeta_fixture CASCADE`),
	},
}
