// Package fixture builds the schema the Firebird queries read.
//
// It creates one of every object those queries report, so a test has rows
// worth checking rather than an empty catalog. Hard rule 9 requires it, and
// D53 requires the core objects to match every other fixture, so that the
// cross family comparison reads the same schema everywhere.
//
// # There is no schema to build in
//
// Firebird has no schemas before 6.0, so this fixture creates its objects in
// the database itself and [Fixture.Schema] is empty. Every other fixture
// names dbmeta_fixture and SQLite names main, and Firebird can name neither,
// because there is nowhere to put a name.
//
// A name written without quotes is folded to upper case, which is the
// standard's rule and Oracle's. The statements here are lower case and the
// catalog reports them upper case, and the tests fold before comparing, the
// way the conformance test already does for Oracle.
//
// # What Firebird cannot be asked for
//
// No DROP ... IF EXISTS before 5.0, so the teardown is run for its effect and
// its errors are ignored. That is how a caller must run it on 3.0 and 4.0.
//
// No publication before 4.0. The two steps that fill RDB$PUBLICATION_TABLES
// carry a gate and are skipped on 3.0, which is the same release the
// Publications query reports [dbmeta.ErrVersionTooOld] for.
//
// No user is created here. A Firebird user belongs to the server rather than
// to the database, so creating one changes a second database that the test
// did not open. test/parity_test.go creates its principals and removes them,
// because that is a test that has to, and an ordinary fixture must not.
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

// from4 is a step Firebird 3.0 cannot run.
func from4(name, query string) Step {
	return Step{Name: name, Stmt: dbmeta.Stmt{{{Min: dbmeta.V(4, 0), Query: query}}}}
}

// Everything holds one of every object the Firebird queries read.
var Everything = Fixture{
	Name: "everything",
	// Empty, and it has to be. See the package comment.
	Schema: "",
	Setup: []Step{
		// A domain, which Firebird has as a first class object and which the
		// author table then uses, so Columns reports a type that came from
		// one.
		at("domain", `CREATE DOMAIN dbmeta_shade AS VARCHAR(16) DEFAULT 'plain'
	CHECK (VALUE IN ('plain', 'bold'))`),

		// The core objects D53 asks every fixture for.
		at("author", `CREATE TABLE author (
	author_id INTEGER NOT NULL,
	name VARCHAR(128) NOT NULL,
	rating INTEGER,
	shade dbmeta_shade,
	CONSTRAINT author_pk PRIMARY KEY (author_id)
)`),
		at("book", `CREATE TABLE book (
	book_id INTEGER NOT NULL,
	author_id INTEGER NOT NULL,
	title VARCHAR(255) NOT NULL,
	published DATE,
	CONSTRAINT book_pk PRIMARY KEY (book_id),
	CONSTRAINT book_author_fk FOREIGN KEY (author_id) REFERENCES author (author_id),
	CONSTRAINT book_title_uq UNIQUE (title),
	CONSTRAINT book_title_ck CHECK (CHAR_LENGTH(title) > 0)
)`),
		// An index somebody created, as against one a constraint created.
		at("book_published", `CREATE INDEX book_published ON book (published)`),
		// A composite primary key, and a composite foreign key into it.
		at("region", `CREATE TABLE region (
	country VARCHAR(64) NOT NULL,
	area VARCHAR(64) NOT NULL,
	CONSTRAINT region_pk PRIMARY KEY (country, area)
)`),
		at("shipment", `CREATE TABLE shipment (
	shipment_id INTEGER NOT NULL,
	country VARCHAR(64) NOT NULL,
	area VARCHAR(64) NOT NULL,
	amount DECIMAL(12, 2) NOT NULL,
	CONSTRAINT shipment_pk PRIMARY KEY (shipment_id),
	CONSTRAINT shipment_region_fk FOREIGN KEY (country, area)
		REFERENCES region (country, area)
)`),
		at("recent", `CREATE VIEW recent AS SELECT book_id, title FROM book`),

		// An identity column, which Firebird gained in 3.0. It is on a table
		// of its own so that the core tables stay the same shape everywhere.
		at("ticket", `CREATE TABLE ticket (
	ticket_id INTEGER GENERATED BY DEFAULT AS IDENTITY,
	note VARCHAR(64),
	CONSTRAINT ticket_pk PRIMARY KEY (ticket_id)
)`),

		at("sequence", `CREATE SEQUENCE dbmeta_counter`),
		at("exception", `CREATE EXCEPTION dbmeta_nope 'the fixture says no'`),

		// A procedure and a function, so Functions has a row of each kind and
		// RoutineParameters has both an input and an output.
		at("procedure", `CREATE PROCEDURE book_count (max_id INTEGER)
RETURNS (total INTEGER)
AS BEGIN
	SELECT COUNT(*) FROM book WHERE book_id <= :max_id INTO :total;
	SUSPEND;
END`),
		at("function", `CREATE FUNCTION doubled (n INTEGER) RETURNS INTEGER
DETERMINISTIC
AS BEGIN
	RETURN n * 2;
END`),
		// A package, whose members Functions leaves out the way
		// models/oracle does.
		at("package", `CREATE PACKAGE dbmeta_util
AS BEGIN
	FUNCTION tripled(n INTEGER) RETURNS INTEGER;
END`),
		at("package body", `CREATE PACKAGE BODY dbmeta_util
AS BEGIN
	FUNCTION tripled(n INTEGER) RETURNS INTEGER
	AS BEGIN
		RETURN n * 3;
	END
END`),

		// A trigger on a table, and one on the database, which is what
		// EventTriggers reads.
		at("trigger", `CREATE TRIGGER book_bi FOR book
ACTIVE BEFORE INSERT POSITION 0
AS BEGIN
	IF (NEW.book_id IS NULL) THEN NEW.book_id = NEXT VALUE FOR dbmeta_counter;
END`),
		at("database trigger", `CREATE TRIGGER dbmeta_on_connect
ACTIVE ON CONNECT POSITION 0
AS BEGIN
END`),

		// A role and a grant to it, so Roles, RoleGrants and Privileges have
		// rows that the fixture put there.
		at("role", `CREATE ROLE dbmeta_reader`),
		at("grant table", `GRANT SELECT ON author TO dbmeta_reader`),
		at("grant column", `GRANT UPDATE (rating) ON author TO dbmeta_reader`),
		at("grant execute", `GRANT EXECUTE ON PROCEDURE book_count TO dbmeta_reader`),

		// Comments, one of each kind the Comments query reads.
		at("table comment", `COMMENT ON TABLE author IS 'people who write'`),
		at("column comment", `COMMENT ON COLUMN author.name IS 'the author name'`),
		at("view comment", `COMMENT ON VIEW recent IS 'the newest books'`),
		at("domain comment", `COMMENT ON DOMAIN dbmeta_shade IS 'how a book is printed'`),
		at("sequence comment", `COMMENT ON SEQUENCE dbmeta_counter IS 'the book numbers'`),
		at("index comment", `COMMENT ON INDEX book_published IS 'by date'`),
		at("procedure comment", `COMMENT ON PROCEDURE book_count IS 'how many books'`),
		at("function comment", `COMMENT ON FUNCTION doubled IS 'twice a number'`),
		at("trigger comment", `COMMENT ON TRIGGER book_bi IS 'fills the key'`),
		at("role comment", `COMMENT ON ROLE dbmeta_reader IS 'may read'`),
		at("exception comment", `COMMENT ON EXCEPTION dbmeta_nope IS 'the refusal'`),
		at("package comment", `COMMENT ON PACKAGE dbmeta_util IS 'small sums'`),

		// Replication, which arrived in 4.0. Firebird has one publication per
		// database and no CREATE PUBLICATION: a table joins it with ALTER
		// DATABASE.
		from4("publish", `ALTER DATABASE INCLUDE TABLE author TO PUBLICATION`),
	},
	Teardown: []Step{
		at("publish", `ALTER DATABASE EXCLUDE TABLE author TO PUBLICATION`),
		at("database trigger", `DROP TRIGGER dbmeta_on_connect`),
		at("trigger", `DROP TRIGGER book_bi`),
		at("recent", `DROP VIEW recent`),
		at("package body", `DROP PACKAGE BODY dbmeta_util`),
		at("package", `DROP PACKAGE dbmeta_util`),
		at("procedure", `DROP PROCEDURE book_count`),
		at("function", `DROP FUNCTION doubled`),
		at("book_published", `DROP INDEX book_published`),
		at("shipment", `DROP TABLE shipment`),
		at("region", `DROP TABLE region`),
		at("book", `DROP TABLE book`),
		at("ticket", `DROP TABLE ticket`),
		at("author", `DROP TABLE author`),
		at("domain", `DROP DOMAIN dbmeta_shade`),
		at("sequence", `DROP SEQUENCE dbmeta_counter`),
		at("exception", `DROP EXCEPTION dbmeta_nope`),
		at("role", `DROP ROLE dbmeta_reader`),
	},
}
