// Package fixture builds the schema the Vertica queries read.
//
// It creates one of every object those queries report, so a test has rows
// worth checking rather than an empty catalog. Hard rule 9 requires it, and
// D53 requires the core objects to match every other fixture, so that the
// cross family comparison reads the same schema everywhere.
//
// Vertica folds an unquoted name to lower case, the way PostgreSQL does, and
// the catalog reports it that way.
//
// # What changes between releases
//
// A CHECK constraint arrived in 9.1 and a comment on a table column in 10.1,
// and 7.2 and 9.1 comment a projection column instead. PL/vSQL and a trigger
// are measured on 25.1 and absent on 10.1, which refuses the language by
// name, and the release between is not measured, so both steps gate on the
// release they were seen in. A step the server is too old for is skipped.
//
// # What Vertica cannot be asked for here
//
// No SQL function. `CREATE FUNCTION ... AS BEGIN RETURN ...; END` needs a
// semicolon inside its body, and vertica-sql-go, the driver usql uses, splits a
// statement at every semicolon before it sends anything, so the server
// receives half a function and refuses it. vsql creates the same statement
// without complaint. Functions reads the functions the packages Vertica
// installs at startup provide, and the PL/vSQL procedure, whose body is dollar
// quoted and survives the split.
//
// One user, on 25.1, to carry a parameter of its own, and teardown drops it.
// test/parity_test.go creates the principals it compares.
//
// No HCatalog schema. CREATE HCATALOG SCHEMA is refused on every image here
// with "HCatalog connector library is not installed", so ForeignServers is
// verified to run and return nothing.
//
// The view is created before book_published. A table with a projection
// somebody made gets no default superprojection on 7.2 and 9.1 until it holds
// rows, and a view over a column no projection carries is refused.
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

// at builds a step whose statement is the same on every server.
func at(name, query string) Step {
	return Step{Name: name, Stmt: dbmeta.Always(query)}
}

// from builds a step that needs a server at min or newer, and is skipped
// below it.
func from(name string, since dbmeta.Version, query string) Step {
	return Step{Name: name, Stmt: dbmeta.Stmt{{{Min: since, Query: query}}}}
}

// Everything holds one of every object the Vertica queries read.
var Everything = Fixture{
	Name:   "everything",
	Schema: "dbmeta_fixture",
	Setup: []Step{
		at("schema", `CREATE SCHEMA dbmeta_fixture`),

		// The core objects D53 asks every fixture for.
		at("author", `CREATE TABLE dbmeta_fixture.author (
	author_id INTEGER NOT NULL,
	name VARCHAR(128) NOT NULL,
	rating INTEGER,
	shade VARCHAR(16) DEFAULT 'plain',
	CONSTRAINT author_pk PRIMARY KEY (author_id)
)`),
		at("book", `CREATE TABLE dbmeta_fixture.book (
	book_id INTEGER NOT NULL,
	author_id INTEGER NOT NULL,
	title VARCHAR(255) NOT NULL,
	published DATE,
	CONSTRAINT book_pk PRIMARY KEY (book_id),
	CONSTRAINT book_author_fk FOREIGN KEY (author_id)
		REFERENCES dbmeta_fixture.author (author_id),
	CONSTRAINT book_title_uq UNIQUE (title)
)`),
		from("book check", dbmeta.V(9, 1), `ALTER TABLE dbmeta_fixture.book
	ADD CONSTRAINT book_title_ck CHECK (LENGTH(title) > 0)`),
		// A composite primary key, and a composite foreign key into it.
		at("region", `CREATE TABLE dbmeta_fixture.region (
	country VARCHAR(64) NOT NULL,
	area VARCHAR(64) NOT NULL,
	CONSTRAINT region_pk PRIMARY KEY (country, area)
)`),
		at("shipment", `CREATE TABLE dbmeta_fixture.shipment (
	shipment_id INTEGER NOT NULL,
	country VARCHAR(64) NOT NULL,
	area VARCHAR(64) NOT NULL,
	amount NUMERIC(12, 2) NOT NULL,
	CONSTRAINT shipment_pk PRIMARY KEY (shipment_id),
	CONSTRAINT shipment_region_fk FOREIGN KEY (country, area)
		REFERENCES dbmeta_fixture.region (country, area)
)`),
		at("recent", `CREATE VIEW dbmeta_fixture.recent AS
	SELECT book_id, title FROM dbmeta_fixture.book`),
		// The core index. Vertica has no index: a projection is a stored,
		// sorted copy of some of a table's columns, and it is what the
		// optimizer picks the way another database picks an index.
		at("book_published", `CREATE PROJECTION dbmeta_fixture.book_published AS
	SELECT book_id, published FROM dbmeta_fixture.book ORDER BY published`),

		// A partitioned table.
		at("archive", `CREATE TABLE dbmeta_fixture.archive (
	archive_id INTEGER NOT NULL,
	filed_year INTEGER NOT NULL
) PARTITION BY filed_year`),

		// An identity column and a sequence of its own.
		at("ticket", `CREATE TABLE dbmeta_fixture.ticket (
	ticket_id IDENTITY(1, 1),
	note VARCHAR(64),
	CONSTRAINT ticket_pk PRIMARY KEY (ticket_id)
)`),
		at("sequence", `CREATE SEQUENCE dbmeta_fixture.dbmeta_counter START 1 INCREMENT 1`),

		// A table whose rows are a file, which is Vertica's foreign table.
		at("external", `CREATE EXTERNAL TABLE dbmeta_fixture.outside (item_id INTEGER)
	AS COPY FROM '/tmp/dbmeta_outside.csv' DELIMITER ','`),

		// A stored procedure, with its body dollar quoted so that the
		// driver does not split it.
		from("procedure", dbmeta.V(25, 1), `CREATE PROCEDURE dbmeta_fixture.book_count (max_id INT)
LANGUAGE PLvSQL AS $$
BEGIN
	PERFORM SELECT COUNT(*) FROM dbmeta_fixture.book WHERE book_id <= max_id;
END;
$$`),
		// A trigger runs a procedure on a schedule.
		from("schedule", dbmeta.V(25, 1), `CREATE SCHEDULE dbmeta_fixture.nightly USING CRON '0 3 * * *'`),
		from("trigger", dbmeta.V(25, 1), `CREATE TRIGGER dbmeta_fixture.count_nightly
	ON SCHEDULE dbmeta_fixture.nightly
	EXECUTE PROCEDURE dbmeta_fixture.book_count(1) AS DEFINER`),

		// Two roles, one granted to the other, and grants to them, so Roles,
		// RoleGrants and Privileges have rows the fixture put there.
		at("role", `CREATE ROLE dbmeta_reader`),
		at("second role", `CREATE ROLE dbmeta_writer`),
		at("grant role", `GRANT dbmeta_reader TO dbmeta_writer`),
		at("grant schema", `GRANT USAGE ON SCHEMA dbmeta_fixture TO dbmeta_reader`),
		at("grant table", `GRANT SELECT ON dbmeta_fixture.author TO dbmeta_reader`),

		// A user with a parameter of its own, which RoleSettings reads from
		// user_configuration_parameters. That view is measured on 25.1.
		from("user", dbmeta.V(25, 1), `CREATE USER dbmeta_tuned`),
		from("user setting", dbmeta.V(25, 1),
			`ALTER USER dbmeta_tuned SET WithClauseRecursionLimit = 4`),

		// A column access policy, which Privileges reports as a policy.
		at("policy", `CREATE ACCESS POLICY ON dbmeta_fixture.author FOR COLUMN name
	CASE WHEN ENABLED_ROLE('dbmeta_reader') THEN name ELSE '***' END ENABLE`),

		// Comments, one of each kind the Comments query reads.
		at("schema comment", `COMMENT ON SCHEMA dbmeta_fixture IS 'the fixture'`),
		at("table comment", `COMMENT ON TABLE dbmeta_fixture.author IS 'people who write'`),
		from("column comment", dbmeta.V(10, 1),
			`COMMENT ON COLUMN dbmeta_fixture.author.name IS 'the author name'`),
		at("view comment", `COMMENT ON VIEW dbmeta_fixture.recent IS 'the newest books'`),
		at("sequence comment", `COMMENT ON SEQUENCE dbmeta_fixture.dbmeta_counter IS 'counts'`),
		at("projection comment", `COMMENT ON PROJECTION dbmeta_fixture.book_published IS 'by date'`),
	},
	Teardown: []Step{
		at("schema", `DROP SCHEMA IF EXISTS dbmeta_fixture CASCADE`),
		at("user", `DROP USER IF EXISTS dbmeta_tuned CASCADE`),
		at("second role", `DROP ROLE IF EXISTS dbmeta_writer CASCADE`),
		at("role", `DROP ROLE IF EXISTS dbmeta_reader CASCADE`),
	},
}
