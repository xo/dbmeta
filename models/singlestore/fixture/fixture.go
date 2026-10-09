// Package fixture builds the objects the SingleStore queries read.
//
// It creates one of every object those queries report, so a test has rows
// worth checking rather than an empty catalog. Hard rule 9 requires it, and
// D53 requires the core objects to match every other fixture.
//
// SingleStore refuses a foreign key, a trigger, a CHECK, a foreign server
// and PARTITION BY, so the MySQL fixture cannot be shared, and its routines
// are written in SingleStore's own language. book is a reference table,
// which is copied to every node, because a unique key of a sharded table
// must hold the shard key, and book_title_unique is on title alone.
//
// A role, a group and a user belong to the server and not to the database,
// so the teardown drops each by name.
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
// every object the SingleStore queries read.
var Everything = Fixture{
	Name:   "everything",
	Schema: "dbmeta_fixture",
	Setup: []Step{
		at("schema", `CREATE DATABASE dbmeta_fixture`),
		at("author", `CREATE TABLE dbmeta_fixture.author (
	author_id INT AUTO_INCREMENT PRIMARY KEY COMMENT 'surrogate key',
	name VARCHAR(255) NOT NULL,
	rating INT,
	shade ENUM('red', 'green', 'blue') DEFAULT 'red'
) COMMENT 'people who write'`),
		at("book", `CREATE REFERENCE TABLE dbmeta_fixture.book (
	book_id INT AUTO_INCREMENT PRIMARY KEY,
	author_id INT NOT NULL,
	title VARCHAR(255) NOT NULL,
	published DATE,
	UNIQUE KEY book_title_unique (title),
	KEY book_published (published)
)`),
		// A two column primary key on a rowstore table, which is also the
		// shard key.
		at("region", `CREATE ROWSTORE TABLE dbmeta_fixture.region (
	country VARCHAR(64) NOT NULL,
	area VARCHAR(64) NOT NULL,
	PRIMARY KEY (country, area)
)`),
		at("shipment", `CREATE TABLE dbmeta_fixture.shipment (
	shipment_id INT AUTO_INCREMENT PRIMARY KEY,
	country VARCHAR(64) NOT NULL,
	area VARCHAR(64) NOT NULL,
	amount INT NOT NULL
)`),
		// A table with a full text index, so that an index has the type
		// FULLTEXT.
		at("article", `CREATE TABLE dbmeta_fixture.article (
	id INT NOT NULL,
	body TEXT,
	SHARD KEY (id),
	FULLTEXT (body)
)`),
		at("view", `CREATE VIEW dbmeta_fixture.recent AS
	SELECT book_id, title FROM dbmeta_fixture.book WHERE published IS NOT NULL`),

		// A function, a procedure with parameters, and an aggregate, which
		// is four functions and a state.
		at("function", `CREATE FUNCTION dbmeta_fixture.shout(s VARCHAR(255)) RETURNS VARCHAR(255)`+
			` AS BEGIN RETURN UPPER(s); END`),
		at("procedure", `CREATE PROCEDURE dbmeta_fixture.addup(a INT, b INT) RETURNS INT`+
			` AS BEGIN RETURN a + b; END`),
		at("aggregate initialize", `CREATE FUNCTION dbmeta_fixture.total_init() RETURNS INT`+
			` AS BEGIN RETURN 0; END`),
		at("aggregate iterate", `CREATE FUNCTION dbmeta_fixture.total_iter(s INT, v INT) RETURNS INT`+
			` AS BEGIN RETURN s + v; END`),
		at("aggregate merge", `CREATE FUNCTION dbmeta_fixture.total_merge(a INT, b INT) RETURNS INT`+
			` AS BEGIN RETURN a + b; END`),
		at("aggregate terminate", `CREATE FUNCTION dbmeta_fixture.total_term(s INT) RETURNS INT`+
			` AS BEGIN RETURN s; END`),
		at("aggregate", `CREATE AGGREGATE dbmeta_fixture.total(INT) RETURNS INT WITH STATE INT`+
			` INITIALIZE WITH total_init ITERATE WITH total_iter`+
			` MERGE WITH total_merge TERMINATE WITH total_term`),

		// A role granted to a group, and the group to a user.
		at("role", `CREATE ROLE 'dbmeta_fixture_reader'`),
		at("privilege", `GRANT SELECT ON dbmeta_fixture.* TO ROLE 'dbmeta_fixture_reader'`),
		at("group", `CREATE GROUP 'dbmeta_fixture_group'`),
		at("role grant", `GRANT ROLE 'dbmeta_fixture_reader' TO 'dbmeta_fixture_group'`),
		at("user", `CREATE USER 'dbmeta_fixture_member'@'%' IDENTIFIED BY 'P4ssw0rd'`),
		at("group grant", `GRANT GROUP 'dbmeta_fixture_group' TO 'dbmeta_fixture_member'@'%'`),

		// Rows, and statistics over them.
		at("author rows", `INSERT INTO dbmeta_fixture.author (name, rating)`+
			` VALUES ('Ursula', 5), ('Octavia', 4), ('Iain', NULL)`),
		at("analyze", `ANALYZE TABLE dbmeta_fixture.author`),
		// A correlation between two columns, which ExtendedStats reads.
		// CORRELATE COLUMN refuses to run with no database selected, even on
		// a qualified table, so the step before it selects one. A caller runs
		// the setup on one connection for that reason.
		at("use", `USE dbmeta_fixture`),
		at("correlation", `ANALYZE TABLE dbmeta_fixture.author CORRELATE COLUMN name WITH COLUMN rating USING COEFFICIENT 0.5`),
	},
	Teardown: []Step{
		at("user", `DROP USER IF EXISTS 'dbmeta_fixture_member'@'%'`),
		at("group", `DROP GROUP IF EXISTS 'dbmeta_fixture_group'`),
		at("role", `DROP ROLE IF EXISTS 'dbmeta_fixture_reader'`),
		at("schema", `DROP DATABASE IF EXISTS dbmeta_fixture`),
	},
}
