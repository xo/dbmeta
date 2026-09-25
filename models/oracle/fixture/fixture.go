// Package fixture builds the schema the Oracle queries read.
//
// It creates one of every object those queries report, so a test has rows
// worth checking rather than an empty catalog. Hard rule 9 requires it, and
// D53 requires the core objects to match every other fixture, so that the
// cross family comparison reads the same schema everywhere.
//
// # A schema is a user
//
// Oracle has no schema object separate from the user that owns it, so the
// fixture creates a user and its objects live there. That also means the
// teardown is one statement: dropping the user cascades to everything it owns.
//
// # Dropping what may not exist
//
// Oracle had no DROP ... IF EXISTS before 23ai, so a teardown that runs
// against a clean database raises ORA-01918 and stops. Every drop here is
// wrapped in a block that catches its own error, which is the idiom Oracle
// users have written for twenty years and is what a version gate would have
// to produce anyway.
package fixture

import (
	"errors"

	"github.com/xo/dbmeta"
)

// 12c is where an identity column arrived. Before that the same effect needs a
// sequence and a trigger, which is what every Oracle schema did for years.
var v12 = dbmeta.V(12)

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

// ignoring wraps a statement so that its own failure does not stop the run.
//
// This is the Oracle idiom for a conditional drop. There is no DROP ... IF
// EXISTS before 23ai, and a teardown has to work on a database where the
// object is already gone.
func ignoring(name, query string) Step {
	return at(name, `BEGIN EXECUTE IMMEDIATE '`+query+`'; EXCEPTION WHEN OTHERS THEN NULL; END;`)
}

// Everything is a schema holding one of every object the Oracle queries read.
var Everything = Fixture{
	Name:   "everything",
	Schema: "DBMETA_FIXTURE",
	Setup: []Step{
		// A schema is a user, and the user needs somewhere to put its
		// segments. UNLIMITED TABLESPACE is the blunt way and it is the right
		// one for a fixture that is dropped afterwards.
		at("user", `CREATE USER dbmeta_fixture IDENTIFIED BY "P4ssw0rd"`),
		at("quota", `GRANT UNLIMITED TABLESPACE TO dbmeta_fixture`),
		at("create session", `GRANT CREATE SESSION TO dbmeta_fixture`),

		at("author", `CREATE TABLE dbmeta_fixture.author (
	author_id NUMBER(10) CONSTRAINT author_pk PRIMARY KEY,
	name VARCHAR2(100) NOT NULL,
	rating NUMBER(3),
	shade VARCHAR2(20) DEFAULT 'red',
	CONSTRAINT author_rating_ck CHECK (rating IS NULL OR rating BETWEEN 0 AND 5)
)`),
		at("book", `CREATE TABLE dbmeta_fixture.book (
	book_id NUMBER(10) CONSTRAINT book_pk PRIMARY KEY,
	author_id NUMBER(10) NOT NULL CONSTRAINT book_author_fk REFERENCES dbmeta_fixture.author (author_id),
	title VARCHAR2(200) NOT NULL CONSTRAINT book_title_uq UNIQUE,
	published DATE
)`),
		// A two column key, so that ConstraintColumns has an order to report.
		at("region", `CREATE TABLE dbmeta_fixture.region (
	country VARCHAR2(2) NOT NULL,
	area VARCHAR2(20) NOT NULL,
	CONSTRAINT region_pk PRIMARY KEY (country, area)
)`),
		at("shipment", `CREATE TABLE dbmeta_fixture.shipment (
	shipment_id NUMBER(10) CONSTRAINT shipment_pk PRIMARY KEY,
	country VARCHAR2(2) NOT NULL,
	area VARCHAR2(20) NOT NULL,
	amount NUMBER(12,2) NOT NULL,
	CONSTRAINT shipment_region_fk FOREIGN KEY (country, area)
		REFERENCES dbmeta_fixture.region (country, area)
)`),
		at("recent", `CREATE VIEW dbmeta_fixture.recent AS
	SELECT book_id, title FROM dbmeta_fixture.book WHERE published IS NOT NULL`),

		// An identity column is 12c and later. 11g gets the column without
		// the identity, so the table is there either way and only the one
		// property differs.
		{Name: "extras", Stmt: dbmeta.Stmt{{
			{Query: `CREATE TABLE dbmeta_fixture.extras (
	extras_id NUMBER(10) PRIMARY KEY,
	label VARCHAR2(40),
	shouted VARCHAR2(80) GENERATED ALWAYS AS (UPPER(label))
)`},
			{Min: v12, Query: `CREATE TABLE dbmeta_fixture.extras (
	extras_id NUMBER GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY,
	label VARCHAR2(40),
	shouted VARCHAR2(80) GENERATED ALWAYS AS (UPPER(label))
)`},
		}}},

		// On published rather than title. Oracle backs a unique constraint
		// with an index of its own, so a second index on title is ORA-01408,
		// "such column list already indexed". This one has to be a column no
		// constraint already covers to be a distinct index at all.
		at("index", `CREATE INDEX book_published_ix ON dbmeta_fixture.book (published)`),
		at("sequence", `CREATE SEQUENCE dbmeta_fixture.counter START WITH 10 INCREMENT BY 2`),

		at("table comment", `COMMENT ON TABLE dbmeta_fixture.author IS 'people who write'`),
		at("column comment", `COMMENT ON COLUMN dbmeta_fixture.author.name IS 'what they are called'`),

		at("function", `CREATE FUNCTION dbmeta_fixture.shout (word IN VARCHAR2) RETURN VARCHAR2 IS
BEGIN
	RETURN UPPER(word);
END;`),
		at("procedure", `CREATE PROCEDURE dbmeta_fixture.addup (a IN NUMBER, b IN NUMBER, total OUT NUMBER) IS
BEGIN
	total := a + b;
END;`),
		at("trigger", `CREATE TRIGGER dbmeta_fixture.book_touch
	BEFORE UPDATE ON dbmeta_fixture.book
	FOR EACH ROW
BEGIN
	NULL;
END;`),

		// Rows and statistics over them, so that ColumnStats reports
		// something.
		at("author rows", `INSERT INTO dbmeta_fixture.author (author_id, name, rating)
	SELECT LEVEL, 'author ' || LEVEL, MOD(LEVEL, 5) FROM dual CONNECT BY LEVEL <= 200`),
		at("analyze", `BEGIN
	DBMS_STATS.GATHER_TABLE_STATS('DBMETA_FIXTURE', 'AUTHOR');
END;`),
	},
	Teardown: []Step{
		// One statement removes the lot, because everything the fixture built
		// belongs to the user it built.
		ignoring("drop user", `DROP USER dbmeta_fixture CASCADE`),
	},
}
