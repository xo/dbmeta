// Package fixture builds the schema the Trino queries read.
//
// It creates one of every object those queries report, so a test has rows
// worth checking rather than an empty catalog. Hard rule 9 requires it, and
// D53 requires the core objects to match every other fixture, so that the
// cross family comparison reads the same schema everywhere.
//
// # It builds in the memory catalog
//
// Trino stores nothing itself. Every table belongs to a connector, and the
// only writable one the official image configures is memory, so that is where
// the fixture goes. The catalog name is part of every statement here, because
// a Trino table is catalog.schema.name and there is no shorter way to say it.
//
// # What Trino cannot be asked for
//
// No primary key, no foreign key, no unique constraint and no check. Trino
// carries no constraint of any kind, so nothing here enforces that a book has
// an author. No sequence, no trigger, no index and no user defined type.
//
// No default either. Trino parses DEFAULT and the memory connector does not
// keep one, so every column here is left without one and the columns query
// reports none.
//
// No role and no grant, and that one is the connector rather than the engine.
// The memory connector answers "does not support role management" to CREATE
// ROLE, so Roles, RoleGrants and Privileges are verified to run and return no
// rows. A connector with sql-standard security, such as Hive, populates them.
// That is the same shape as ClickHouse and its named collections. See
// docs/COVERAGE.md.
//
// No materialized view. The memory connector refuses to create one, so
// system.metadata.materialized_views stays empty and Views reports the plain
// views only.
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

// Everything is a schema holding one of every object the Trino queries read.
var Everything = Fixture{
	Name:   "everything",
	Schema: "dbmeta_fixture",
	Setup: []Step{
		at("schema", `CREATE SCHEMA memory.dbmeta_fixture`),

		// The core objects D53 asks every fixture for. NOT NULL is the only
		// column property Trino keeps, so it is the only one asserted.
		at("author", `CREATE TABLE memory.dbmeta_fixture.author (
	author_id integer NOT NULL,
	name varchar(128) NOT NULL,
	rating integer,
	shade varchar(16)
) COMMENT 'the authors'`),

		// The column comment, which is the one thing system.jdbc.columns
		// carries and information_schema.columns has no column for.
		at("author name comment",
			`COMMENT ON COLUMN memory.dbmeta_fixture.author.name IS 'the author name'`),

		at("book", `CREATE TABLE memory.dbmeta_fixture.book (
	book_id integer NOT NULL,
	author_id integer NOT NULL,
	title varchar(255) NOT NULL,
	published date
) COMMENT 'the books'`),

		at("region", `CREATE TABLE memory.dbmeta_fixture.region (
	country varchar(64) NOT NULL,
	area varchar(64) NOT NULL
) COMMENT 'the regions'`),

		at("shipment", `CREATE TABLE memory.dbmeta_fixture.shipment (
	shipment_id integer NOT NULL,
	country varchar(64) NOT NULL,
	area varchar(64) NOT NULL,
	amount decimal(12, 2) NOT NULL
) COMMENT 'the shipments'`),

		// A view, with a comment, so Views has a row and the comment join has
		// something to find.
		at("recent", `CREATE VIEW memory.dbmeta_fixture.recent COMMENT 'the newest books'
	AS SELECT book_id, title FROM memory.dbmeta_fixture.book`),

		// A second schema, so a catalog filter and a schema filter are asked
		// something they can answer wrongly.
		at("other schema", `CREATE SCHEMA memory.dbmeta_other`),
		at("other table", `CREATE TABLE memory.dbmeta_other.elsewhere (id integer)`),
	},
	Teardown: []Step{
		at("recent", `DROP VIEW IF EXISTS memory.dbmeta_fixture.recent`),
		at("author", `DROP TABLE IF EXISTS memory.dbmeta_fixture.author`),
		at("book", `DROP TABLE IF EXISTS memory.dbmeta_fixture.book`),
		at("region", `DROP TABLE IF EXISTS memory.dbmeta_fixture.region`),
		at("shipment", `DROP TABLE IF EXISTS memory.dbmeta_fixture.shipment`),
		at("schema", `DROP SCHEMA IF EXISTS memory.dbmeta_fixture`),
		at("other table", `DROP TABLE IF EXISTS memory.dbmeta_other.elsewhere`),
		at("other schema", `DROP SCHEMA IF EXISTS memory.dbmeta_other`),
	},
}
