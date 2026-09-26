// Package fixture builds the schema the SAP HANA queries read.
//
// It creates one of every object those queries report, so a test has rows
// worth checking rather than an empty catalog. Hard rule 9 requires it, and
// D53 requires the core objects to match every other fixture, so that the
// cross family comparison reads the same schema everywhere.
//
// A name written without quotes is folded to upper case, which is the
// standard's rule and Oracle's and Firebird's. The statements here are lower
// case and the catalog reports them upper case, and the tests fold before
// comparing.
//
// # What SAP HANA cannot be asked for here
//
// No user. A HANA user belongs to the tenant database and creating one is a
// write to the security catalog that outlives the schema, so the fixture
// creates a role and test/parity_test.go creates its own principal.
//
// No remote source, virtual table or remote subscription. Smart data access
// needs a second database to federate to, and the four queries that read it
// are verified to run and return nothing. That is the same shape as
// ClickHouse and its named collections. See docs/COVERAGE.md.
//
// No text configuration. CREATE FULLTEXT INDEX works, and a text
// configuration is a repository object created outside SQL, so
// TextSearchConfigs is verified to run and returns nothing.
//
// No column level grant. GRANTED_PRIVILEGES carries a COLUMN_NAME column
// and there is no GRANT syntax that fills it: every spelling of a column
// list is a syntax error on 2.0 SPS 08. So Privileges reports an empty
// column_access for every object, and the query keeps the column because
// the catalog has it.
//
// No data statistics object. CREATE STATISTICS needs the column store and a
// table with rows in it, and the fixture inserts none.
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

// Everything holds one of every object the SAP HANA queries read.
var Everything = Fixture{
	Name:   "everything",
	Schema: "DBMETA_FIXTURE",
	Setup: []Step{
		at("schema", `CREATE SCHEMA dbmeta_fixture`),

		// The core objects D53 asks every fixture for.
		at("author", `CREATE COLUMN TABLE dbmeta_fixture.author (
	author_id INTEGER NOT NULL,
	name NVARCHAR(128) NOT NULL,
	rating INTEGER,
	shade NVARCHAR(16) DEFAULT 'plain',
	CONSTRAINT author_pk PRIMARY KEY (author_id)
)`),
		at("book", `CREATE COLUMN TABLE dbmeta_fixture.book (
	book_id INTEGER NOT NULL,
	author_id INTEGER NOT NULL,
	title NVARCHAR(255) NOT NULL,
	published DATE,
	CONSTRAINT book_pk PRIMARY KEY (book_id),
	CONSTRAINT book_author_fk FOREIGN KEY (author_id)
		REFERENCES dbmeta_fixture.author (author_id),
	CONSTRAINT book_title_uq UNIQUE (title),
	CONSTRAINT book_title_ck CHECK (LENGTH(title) > 0)
)`),
		// An index somebody created, as against one a constraint created.
		at("book_published", `CREATE INDEX book_published ON dbmeta_fixture.book (published)`),
		// A composite primary key, and a composite foreign key into it.
		at("region", `CREATE COLUMN TABLE dbmeta_fixture.region (
	country NVARCHAR(64) NOT NULL,
	area NVARCHAR(64) NOT NULL,
	CONSTRAINT region_pk PRIMARY KEY (country, area)
)`),
		at("shipment", `CREATE COLUMN TABLE dbmeta_fixture.shipment (
	shipment_id INTEGER NOT NULL,
	country NVARCHAR(64) NOT NULL,
	area NVARCHAR(64) NOT NULL,
	amount DECIMAL(12, 2) NOT NULL,
	CONSTRAINT shipment_pk PRIMARY KEY (shipment_id),
	CONSTRAINT shipment_region_fk FOREIGN KEY (country, area)
		REFERENCES dbmeta_fixture.region (country, area)
)`),
		at("recent", `CREATE VIEW dbmeta_fixture.recent AS
	SELECT book_id, title FROM dbmeta_fixture.book`),

		// A row store table, so AccessMethods has both kinds to report and
		// does not merely say that every table is a column table.
		at("ledger", `CREATE ROW TABLE dbmeta_fixture.ledger (
	entry_id INTEGER NOT NULL,
	note NVARCHAR(64),
	CONSTRAINT ledger_pk PRIMARY KEY (entry_id)
)`),

		// A partitioned table, which HANA allows on a column table.
		at("archive", `CREATE COLUMN TABLE dbmeta_fixture.archive (
	archive_id INTEGER NOT NULL,
	year INTEGER NOT NULL
) PARTITION BY HASH (archive_id) PARTITIONS 4`),

		// An identity column, on a table of its own so that the core
		// tables stay the same shape everywhere.
		at("ticket", `CREATE COLUMN TABLE dbmeta_fixture.ticket (
	ticket_id INTEGER GENERATED BY DEFAULT AS IDENTITY,
	note NVARCHAR(64),
	CONSTRAINT ticket_pk PRIMARY KEY (ticket_id)
)`),

		at("sequence", `CREATE SEQUENCE dbmeta_fixture.dbmeta_counter
	START WITH 1 INCREMENT BY 1`),

		// A procedure and a function, so Functions has a row of each kind
		// and RoutineParameters has an input, an output and a return.
		at("procedure", `CREATE PROCEDURE dbmeta_fixture.book_count (
	IN max_id INTEGER, OUT total INTEGER)
LANGUAGE SQLSCRIPT
AS BEGIN
	SELECT COUNT(*) INTO total FROM dbmeta_fixture.book WHERE book_id <= :max_id;
END`),
		at("function", `CREATE FUNCTION dbmeta_fixture.doubled (n INTEGER)
RETURNS result INTEGER
LANGUAGE SQLSCRIPT
DETERMINISTIC
AS BEGIN
	result := :n * 2;
END`),

		at("trigger", `CREATE TRIGGER dbmeta_fixture.book_bi
BEFORE INSERT ON dbmeta_fixture.book
REFERENCING NEW ROW newrow
FOR EACH ROW
BEGIN
	-- HANA refuses an empty trigger body, so this one does the least a
	-- body can do rather than nothing.
	DECLARE ignored INTEGER;
	ignored := 1;
END`),

		// A full text index, which is the object a text configuration
		// serves and the only part of that family HANA creates in SQL.
		at("fulltext", `CREATE FULLTEXT INDEX book_title_ft
	ON dbmeta_fixture.book (title)`),

		// A role and grants to it, so Roles, RoleGrants and Privileges
		// have rows the fixture put there.
		at("role", `CREATE ROLE dbmeta_reader`),
		at("grant table", `GRANT SELECT ON dbmeta_fixture.author TO dbmeta_reader`),
		at("grant execute", `GRANT EXECUTE ON dbmeta_fixture.book_count TO dbmeta_reader`),

		// Comments, one of each kind the Comments query reads.
		at("table comment", `COMMENT ON TABLE dbmeta_fixture.author IS 'people who write'`),
		at("column comment", `COMMENT ON COLUMN dbmeta_fixture.author.name IS 'the author name'`),
		at("view comment", `COMMENT ON VIEW dbmeta_fixture.recent IS 'the newest books'`),
	},
	Teardown: []Step{
		at("schema", `DROP SCHEMA dbmeta_fixture CASCADE`),
		at("role", `DROP ROLE dbmeta_reader`),
	},
}
