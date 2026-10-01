// Package fixture builds the objects the Neo4j queries read.
//
// It creates one of every object those queries report, so a test has rows
// worth checking rather than an empty catalog. Hard rule 9 requires it, and
// D53 asks for the core objects of every fixture.
//
// It builds in the database dbmeta, which the dbrun setup makes, and runs as
// the administrator. A label is a table here, so the core tables are the
// labels author, book, region and shipment. A label exists while a node
// carries it, so each one gets nodes. The foreign keys of the core schema are
// the relationship types written_by and ships_to, which join the nodes.
//
// # What Neo4j cannot be asked for
//
// No view, so the core object recent does not exist here. No foreign key: a
// relationship joins two nodes and constrains no property. No check
// constraint, no default, no comment, no trigger, no sequence and no type.
//
// No user defined function or procedure. Each is a Java plugin, a jar in the
// plugins directory of the server, and Cypher cannot create one. Functions,
// aggregates and routine parameters are read from the built in ones.
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

// Fixture is a database and the statements that build and remove its
// objects.
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

// v526 is the floor of the model, which the fixture shares.
var v526 = dbmeta.V(5, 26)

func at(name, query string) Step {
	return Step{Name: name, Stmt: dbmeta.Stmt{{{Min: v526, Query: query}}}}
}

// Everything is the objects the Neo4j queries read, in the database dbmeta.
var Everything = Fixture{
	Name:   "everything",
	Schema: "dbmeta",
	Setup: []Step{
		// The keys of the core tables. A node key is unique and present on
		// every node, so it is the primary key.
		at("author_pkey", "CREATE CONSTRAINT author_pkey IF NOT EXISTS"+
			" FOR (a:author) REQUIRE a.author_id IS NODE KEY"),
		at("book_pkey", "CREATE CONSTRAINT book_pkey IF NOT EXISTS"+
			" FOR (b:book) REQUIRE b.book_id IS NODE KEY"),
		at("region_pkey", "CREATE CONSTRAINT region_pkey IF NOT EXISTS"+
			" FOR (r:region) REQUIRE (r.country, r.area) IS NODE KEY"),
		at("shipment_pkey", "CREATE CONSTRAINT shipment_pkey IF NOT EXISTS"+
			" FOR (s:shipment) REQUIRE s.shipment_id IS NODE KEY"),

		// The other kinds of constraint: unique, existence, a type, and a
		// key on a relationship.
		at("book_title_key", "CREATE CONSTRAINT book_title_key IF NOT EXISTS"+
			" FOR (b:book) REQUIRE b.title IS UNIQUE"),
		at("author_name_not_null", "CREATE CONSTRAINT author_name_not_null IF NOT EXISTS"+
			" FOR (a:author) REQUIRE a.name IS NOT NULL"),
		at("shipment_amount_not_null", "CREATE CONSTRAINT shipment_amount_not_null IF NOT EXISTS"+
			" FOR (s:shipment) REQUIRE s.amount IS NOT NULL"),
		at("book_title_type", "CREATE CONSTRAINT book_title_type IF NOT EXISTS"+
			" FOR (b:book) REQUIRE b.title IS :: STRING"),
		at("ships_to_key", "CREATE CONSTRAINT ships_to_key IF NOT EXISTS"+
			" FOR ()-[t:ships_to]-() REQUIRE t.tracking IS RELATIONSHIP KEY"),

		// The core index book_published, on one property, and an index in
		// each other shape: two properties, a text index, a full text index
		// on two labels, and an index on a relationship type.
		at("book_published", "CREATE INDEX book_published IF NOT EXISTS FOR (b:book) ON (b.published)"),
		at("book_author_title", "CREATE INDEX book_author_title IF NOT EXISTS"+
			" FOR (b:book) ON (b.author_id, b.title)"),
		at("book_title_text", "CREATE TEXT INDEX book_title_text IF NOT EXISTS FOR (b:book) ON (b.title)"),
		at("book_search", "CREATE FULLTEXT INDEX book_search IF NOT EXISTS"+
			" FOR (n:book|author) ON EACH [n.title, n.name]"),
		at("written_by_since", "CREATE INDEX written_by_since IF NOT EXISTS"+
			" FOR ()-[w:written_by]-() ON (w.since)"),

		// The nodes, which make the labels exist, and the relationships,
		// which make the types exist.
		at("author rows", "MERGE (a:author {author_id: 1}) SET a.name = 'Ursula', a.rating = 5"+
			" MERGE (b:author {author_id: 2}) SET b.name = 'Gene'"),
		at("book rows", "MERGE (b:book {book_id: 1}) SET b.title = 'A Wizard of Earthsea',"+
			" b.author_id = 1, b.published = date('1968-01-01')"+
			" MERGE (c:book {book_id: 2}) SET c.title = 'Shadow of the Torturer', c.author_id = 2"),
		at("written_by", "MATCH (b:book), (a:author) WHERE b.author_id = a.author_id"+
			" MERGE (b)-[w:written_by]->(a) SET w.since = 1968"),
		at("region rows", "MERGE (r:region {country: 'US', area: 'CA'}) SET r.name = 'California'"),
		at("shipment rows", "MERGE (s:shipment {shipment_id: 1}) SET s.country = 'US', s.area = 'CA', s.amount = 10"),
		at("ships_to", "MATCH (s:shipment {shipment_id: 1}), (r:region {country: 'US', area: 'CA'})"+
			" MERGE (s)-[:ships_to {tracking: 'T1'}]->(r)"),

		// A role with a grant and a denial, held by the ordinary user the
		// dbrun setup makes, and a home database for that user. The setup
		// resets the user on its next start.
		at("role", "CREATE ROLE dbmeta_reader IF NOT EXISTS"),
		at("grant", "GRANT MATCH {*} ON GRAPH dbmeta NODES book TO dbmeta_reader"),
		at("deny", "DENY READ {rating} ON GRAPH dbmeta NODES author TO dbmeta_reader"),
		at("role grant", "GRANT ROLE dbmeta_reader TO dbmeta_user"),
		at("home", "ALTER USER dbmeta_user SET HOME DATABASE dbmeta"),
	},
	Teardown: []Step{
		at("home", "ALTER USER dbmeta_user REMOVE HOME DATABASE"),
		// Dropping the role takes its grants and its members with it.
		at("role", "DROP ROLE dbmeta_reader IF EXISTS"),
		at("nodes", "MATCH (n) WHERE n:author OR n:book OR n:region OR n:shipment DETACH DELETE n"),
		at("book_published", "DROP INDEX book_published IF EXISTS"),
		at("book_author_title", "DROP INDEX book_author_title IF EXISTS"),
		at("book_title_text", "DROP INDEX book_title_text IF EXISTS"),
		at("book_search", "DROP INDEX book_search IF EXISTS"),
		at("written_by_since", "DROP INDEX written_by_since IF EXISTS"),
		at("author_pkey", "DROP CONSTRAINT author_pkey IF EXISTS"),
		at("book_pkey", "DROP CONSTRAINT book_pkey IF EXISTS"),
		at("region_pkey", "DROP CONSTRAINT region_pkey IF EXISTS"),
		at("shipment_pkey", "DROP CONSTRAINT shipment_pkey IF EXISTS"),
		at("book_title_key", "DROP CONSTRAINT book_title_key IF EXISTS"),
		at("author_name_not_null", "DROP CONSTRAINT author_name_not_null IF EXISTS"),
		at("shipment_amount_not_null", "DROP CONSTRAINT shipment_amount_not_null IF EXISTS"),
		at("book_title_type", "DROP CONSTRAINT book_title_type IF EXISTS"),
		at("ships_to_key", "DROP CONSTRAINT ships_to_key IF EXISTS"),
	},
}
