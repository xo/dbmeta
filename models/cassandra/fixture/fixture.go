// Package fixture builds the keyspace the Cassandra queries read.
//
// It creates one of every object those queries report, so a test has rows
// worth checking rather than an empty catalog. Hard rule 9 requires it, and
// D53 requires the core objects to match every other fixture, so that the
// cross family comparison reads the same schema everywhere.
//
// # It needs the image this repository builds
//
// A user defined function, a materialized view and a role are all refused by
// the Apache image as it ships. test/cassandra/Containerfile turns them on and
// test/cassandra/build.sh makes the image. Against the published image the
// fixture fails rather than quietly building less, which is the answer hard
// rule 9 wants: a query that has never run against a real object is not
// finished.
//
// # What Cassandra cannot be asked for
//
// No foreign key, so book names an author and nothing enforces it. No default,
// no sequence, no check. A trigger needs a Java class already on the server's
// classpath, so the fixture creates none and the triggers query returns no
// rows anywhere.
//
// # Dropping what may not exist
//
// CQL has DROP ... IF EXISTS for everything here, so the teardown is plain and
// needs none of the trickery the Oracle fixture wraps its drops in.
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

// Fixture is a keyspace and the statements that build and remove it.
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

// Everything is a keyspace holding one of every object the Cassandra queries
// read.
var Everything = Fixture{
	Name:   "everything",
	Schema: "dbmeta_fixture",
	Setup: []Step{
		// SimpleStrategy with one replica, because the tests run one node.
		// A keyspace is the only namespace Cassandra has, so this is what
		// every other fixture calls a schema.
		at("keyspace", `CREATE KEYSPACE dbmeta_fixture WITH replication =`+
			` {'class': 'SimpleStrategy', 'replication_factor': 1}`),

		// A user defined type, which is the only kind of type CQL lets
		// anybody declare. It is used by author below, so that the column
		// type is a real reference rather than an orphan.
		at("type", `CREATE TYPE dbmeta_fixture.address (street text, city text)`),

		// The core objects D53 asks every fixture for. There is no foreign
		// key in Cassandra, so book carries an author_id that nothing
		// enforces, and no default, so shade is a plain column.
		at("author", `CREATE TABLE dbmeta_fixture.author (
	author_id int PRIMARY KEY,
	name text,
	rating int,
	shade text,
	home frozen<address>
) WITH comment = 'the authors'`),

		// book has a compound primary key so that the constraint columns
		// query has an order to report: book_id partitions and title
		// clusters.
		at("book", `CREATE TABLE dbmeta_fixture.book (
	book_id int,
	author_id int,
	title text,
	published date,
	PRIMARY KEY (book_id, title)
) WITH comment = 'the books'`),

		// A two column partition key, which is a different shape again from
		// a partition key and a clustering column.
		at("region", `CREATE TABLE dbmeta_fixture.region (
	country text,
	area text,
	PRIMARY KEY ((country, area))
) WITH comment = 'the regions'`),

		at("shipment", `CREATE TABLE dbmeta_fixture.shipment (
	shipment_id int PRIMARY KEY,
	country text,
	area text,
	amount decimal
) WITH comment = 'the shipments'`),

		// A secondary index, so the indexes and index columns queries have a
		// row. It is on a regular column, because a primary key column is
		// already indexed by being the key.
		at("index", `CREATE INDEX book_author ON dbmeta_fixture.book (author_id)`),

		// A materialized view, which is what Views returns. Every column of
		// the view's primary key has to be declared not null, and the base
		// table's whole primary key has to be in it.
		at("view", `CREATE MATERIALIZED VIEW dbmeta_fixture.recent AS
	SELECT book_id, title FROM dbmeta_fixture.book
	WHERE book_id IS NOT NULL AND title IS NOT NULL
	PRIMARY KEY (title, book_id)`),

		// A function and an aggregate built on it. Java is the only language
		// left: scripted functions were removed after 4.0.
		at("function", `CREATE FUNCTION dbmeta_fixture.plus_one(n int)
	CALLED ON NULL INPUT RETURNS int LANGUAGE java
	AS 'return n == null ? null : n + 1;'`),
		at("state function", `CREATE FUNCTION dbmeta_fixture.sum_state(state int, n int)
	CALLED ON NULL INPUT RETURNS int LANGUAGE java
	AS 'return (state == null ? 0 : state) + (n == null ? 0 : n);'`),
		at("aggregate", `CREATE AGGREGATE dbmeta_fixture.total(int)
	SFUNC sum_state STYPE int INITCOND 0`),

		// Two roles and a grant between them, so that the roles, role grants
		// and privileges queries each have a row. This needs the
		// authenticator and the authorizer the built image turns on.
		at("role", `CREATE ROLE dbmeta_reader WITH PASSWORD = 'P4ssw0rd' AND LOGIN = true`),
		at("group role", `CREATE ROLE dbmeta_group WITH LOGIN = false`),
		at("role grant", `GRANT dbmeta_group TO dbmeta_reader`),
		at("permission", `GRANT SELECT ON KEYSPACE dbmeta_fixture TO dbmeta_reader`),

		// Rows, so that a query reading data rather than catalog has
		// something, and so that the column statistics a cluster keeps are
		// not all zero.
		at("author rows", `INSERT INTO dbmeta_fixture.author (author_id, name, rating, shade)`+
			` VALUES (1, 'Ursula', 5, 'red')`),
		at("book rows", `INSERT INTO dbmeta_fixture.book (book_id, author_id, title, published)`+
			` VALUES (1, 1, 'A Wizard of Earthsea', '1968-01-01')`),
		at("region rows", `INSERT INTO dbmeta_fixture.region (country, area)`+
			` VALUES ('US', 'west')`),
		at("shipment rows", `INSERT INTO dbmeta_fixture.shipment`+
			` (shipment_id, country, area, amount) VALUES (1, 'US', 'west', 12.50)`),
	},
	Teardown: []Step{
		// The keyspace takes its tables, view, index, type, function and
		// aggregate with it. The roles are outside it and go separately, and
		// the grant goes with the role that holds it.
		at("keyspace", `DROP KEYSPACE IF EXISTS dbmeta_fixture`),
		at("role", `DROP ROLE IF EXISTS dbmeta_reader`),
		at("group role", `DROP ROLE IF EXISTS dbmeta_group`),
	},
}
