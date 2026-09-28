// Package fixture builds the schema the CrateDB queries read.
//
// It creates one of every object those queries report, so a test has rows
// worth checking rather than an empty catalog. Hard rule 9 requires it, and
// D53 requires the core objects to match every other fixture, so that the
// cross family comparison reads the same schema everywhere.
//
// # A schema exists while a table is in it
//
// CrateDB has no CREATE SCHEMA. The first table made in a schema makes it,
// and the schema is gone when the last table is dropped. So the setup opens
// with no schema step and the teardown drops every table.
//
// # What CrateDB cannot be asked for
//
// No foreign key and no unique constraint, so book carries an author_id that
// nothing enforces and a title that may repeat. No COMMENT statement, no
// sequence, no trigger, no user defined type, no enum and no domain. A
// primary key column is NOT NULL whether it says so or not.
//
// # A foreign table opens no connection
//
// The foreign server names a JDBC address that answers nothing. CrateDB
// connects only when the foreign table is read, and no query here reads it.
//
// # Every statement is visible at once
//
// CrateDB makes a write visible to a read after a refresh, which runs about
// once a second. REFRESH TABLE makes the rows visible at once, so ANALYZE
// reads them.
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

// Fixture is a schema and the statements that build and remove it.
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

// Everything is a schema holding one of every object the CrateDB queries
// read.
var Everything = Fixture{
	Name:   "everything",
	Schema: "dbmeta_fixture",
	Setup: []Step{
		// The core objects D53 asks every fixture for. The check on rating
		// is a column constraint and the one on title is a table
		// constraint, so that both forms are in pg_constraint.
		at("author", `CREATE TABLE dbmeta_fixture.author (
	author_id integer PRIMARY KEY,
	name text NOT NULL,
	rating integer CHECK (rating > 0),
	shade text DEFAULT 'red'
)`),
		at("book", `CREATE TABLE dbmeta_fixture.book (
	book_id integer PRIMARY KEY,
	author_id integer NOT NULL,
	title text NOT NULL,
	published timestamp,
	CONSTRAINT book_title_ck CHECK (title <> '')
)`),
		// A two column primary key, so that the constraint columns and index
		// columns queries have an order to report.
		at("region", `CREATE TABLE dbmeta_fixture.region (
	country text,
	area text,
	PRIMARY KEY (country, area)
)`),
		at("shipment", `CREATE TABLE dbmeta_fixture.shipment (
	shipment_id integer PRIMARY KEY,
	country text NOT NULL,
	area text NOT NULL,
	amount numeric(12, 2) NOT NULL
)`),
		at("view", `CREATE VIEW dbmeta_fixture.recent AS
	SELECT book_id, title FROM dbmeta_fixture.book`),

		// A partitioned table, whose partition column is generated, so that
		// the partitioned tables query has a row and the columns query has a
		// generated column.
		at("partitioned table", `CREATE TABLE dbmeta_fixture.sales (
	sold_on timestamp NOT NULL,
	amount integer,
	year integer GENERATED ALWAYS AS extract(YEAR FROM sold_on)
) PARTITIONED BY (year)`),

		// A user defined function. JavaScript is the only language CrateDB
		// takes, and the image ships it.
		at("function", `CREATE FUNCTION dbmeta_fixture.addup(integer, integer)
	RETURNS integer LANGUAGE JAVASCRIPT
	AS 'function addup(a, b) { return a + b; }'`),

		// A role, a user, a role grant and a privilege on a table, so the
		// roles, role grants and privileges queries have a row. A role
		// cannot log in and a user can.
		at("role", `CREATE ROLE dbmeta_reader`),
		at("user", `CREATE USER dbmeta_grantee WITH (password = 'P4ssw0rd')`),
		at("role grant", `GRANT dbmeta_reader TO dbmeta_grantee`),
		at("privilege", `GRANT DQL ON TABLE dbmeta_fixture.book TO dbmeta_reader`),
		// A denied privilege, which is a state PostgreSQL has no word for.
		at("denied privilege", `DENY DML ON TABLE dbmeta_fixture.book TO dbmeta_grantee`),

		// A publication of one table, for logical replication.
		at("publication", `CREATE PUBLICATION dbmeta_fixture_pub FOR TABLE dbmeta_fixture.book`),

		// A foreign server, a user mapping and a foreign table. The address
		// answers nothing, which is fine, because nothing reads the table.
		at("foreign server", `CREATE SERVER dbmeta_fixture_srv FOREIGN DATA WRAPPER jdbc`+
			` OPTIONS (url 'jdbc:postgresql://127.0.0.1:1/')`),
		at("user mapping", `CREATE USER MAPPING FOR crate SERVER dbmeta_fixture_srv`+
			` OPTIONS ("user" 'crate')`),
		at("foreign table", `CREATE FOREIGN TABLE dbmeta_fixture.remote (id integer)`+
			` SERVER dbmeta_fixture_srv OPTIONS (schema_name 'doc', table_name 'remote')`),

		// Rows, and statistics over them, so the column statistics query
		// has values to report.
		at("author rows", `INSERT INTO dbmeta_fixture.author (author_id, name, rating)`+
			` VALUES (1, 'Ursula', 5), (2, 'Octavia', 4), (3, 'Iain', 4)`),
		at("book rows", `INSERT INTO dbmeta_fixture.book (book_id, author_id, title, published)`+
			` VALUES (1, 1, 'A Wizard of Earthsea', '1968-01-01')`),
		at("region rows", `INSERT INTO dbmeta_fixture.region (country, area) VALUES ('US', 'west')`),
		at("shipment rows", `INSERT INTO dbmeta_fixture.shipment (shipment_id, country, area, amount)`+
			` VALUES (1, 'US', 'west', 12.50)`),
		at("refresh", `REFRESH TABLE dbmeta_fixture.author, dbmeta_fixture.book,`+
			` dbmeta_fixture.region, dbmeta_fixture.shipment`),
		at("analyze", `ANALYZE`),
	},
	Teardown: []Step{
		// The publication and the foreign table name objects that the drops
		// after them remove, so they go first. The schema goes with its last
		// table.
		at("publication", `DROP PUBLICATION IF EXISTS dbmeta_fixture_pub`),
		at("foreign table", `DROP FOREIGN TABLE IF EXISTS dbmeta_fixture.remote`),
		at("user mapping", `DROP USER MAPPING IF EXISTS FOR crate SERVER dbmeta_fixture_srv`),
		at("foreign server", `DROP SERVER IF EXISTS dbmeta_fixture_srv`),
		at("function", `DROP FUNCTION IF EXISTS dbmeta_fixture.addup(integer, integer)`),
		at("view", `DROP VIEW IF EXISTS dbmeta_fixture.recent`),
		at("sales", `DROP TABLE IF EXISTS dbmeta_fixture.sales`),
		at("shipment", `DROP TABLE IF EXISTS dbmeta_fixture.shipment`),
		at("region", `DROP TABLE IF EXISTS dbmeta_fixture.region`),
		at("book", `DROP TABLE IF EXISTS dbmeta_fixture.book`),
		at("author", `DROP TABLE IF EXISTS dbmeta_fixture.author`),
		at("user", `DROP USER IF EXISTS dbmeta_grantee`),
		at("role", `DROP ROLE IF EXISTS dbmeta_reader`),
	},
}
