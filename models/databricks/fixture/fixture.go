// Package fixture holds a known good Databricks schema, in the SQL of
// Databricks SQL on Unity Catalog.
//
// Like every fixture here it is exported API and additive: a later release can
// add an object and will not rename or remove one. See the PostgreSQL fixture
// for the rules, which are the same.
//
// It is not versioned. The service has one release at a time, and the Resolve
// methods keep the shape the other fixtures have so that a caller can treat them
// alike.
//
// # Where it builds
//
// The test account owns one schema, dbmeta, and cannot create another, so the
// fixture builds its objects in the schema that the connection names. Its Schema
// is that name. The steps write no schema in front of a name, so they follow the
// catalog and the schema of the connection. The teardown drops everything the
// setup made and never the schema.
//
// # What it cannot build
//
// A CHECK constraint is only a table property, and the fixture builds one so that
// a test can show that no query lists it. A column default, an identity column
// and a generated column are built for the same reason: INFORMATION_SCHEMA
// reports none of them. The fixture builds no index, sequence, trigger, user
// defined type or role, because Databricks has none. It builds no connection,
// because the account cannot create one, so the foreign servers query is shown to
// run and to return nothing.
//
// It builds no materialized view. A materialized view runs a pipeline on
// serverless compute, which spends the daily quota of the free edition, and it
// makes two tables of its own in the schema, a hidden table and an event log
// table, which DROP MATERIALIZED VIEW leaves behind under names that no teardown
// can know. The model was measured once against a materialized view, and Tables
// reported it as a materialized view and Views as a view.
package fixture

import "github.com/xo/dbmeta"

// Step is one statement of a fixture.
type Step struct {
	Name string
	Stmt dbmeta.Stmt
}

// Result is what a step resolved to.
type Result struct {
	Name    string
	Query   string
	Skipped bool
	Reason  string
}

// Fixture is a schema, with the statements that build it and drop it.
type Fixture struct {
	Name     string
	Schema   string
	Setup    []Step
	Teardown []Step
}

// ResolveSetup returns the statements that build the fixture.
func (f Fixture) ResolveSetup(versions dbmeta.VersionSet) ([]Result, error) {
	return resolve(f.Setup, versions)
}

// ResolveTeardown returns the statements that drop it.
func (f Fixture) ResolveTeardown(versions dbmeta.VersionSet) ([]Result, error) {
	return resolve(f.Teardown, versions)
}

func resolve(steps []Step, versions dbmeta.VersionSet) ([]Result, error) {
	out := make([]Result, 0, len(steps))
	for _, step := range steps {
		query, err := step.Stmt.Build(versions)
		if err != nil {
			return nil, err
		}
		out = append(out, Result{Name: step.Name, Query: query})
	}
	return out, nil
}

func at(name, query string) Step {
	return Step{Name: name, Stmt: dbmeta.Always(query)}
}

// Everything is a schema holding one of every object the Databricks queries
// read.
//
// The names match the other fixtures, so a test that reads author and book on
// PostgreSQL reads the same two here.
var Everything = Fixture{
	Name:   "everything",
	Schema: "dbmeta",
	Setup: []Step{
		// A single column key, a not null column that has a comment, a nullable
		// column and a column with a default. Databricks records a key and never
		// checks it.
		at("author table", "CREATE TABLE author (\n"+
			"	author_id BIGINT NOT NULL COMMENT 'the key',\n"+
			"	name STRING NOT NULL COMMENT 'the full name',\n"+
			"	rating INT,\n"+
			"	shade STRING DEFAULT 'red',\n"+
			"	CONSTRAINT author_pk PRIMARY KEY (author_id)\n"+
			") TBLPROPERTIES ('delta.feature.allowColumnDefaults' = 'supported')\n"+
			"COMMENT 'people who write books'"),

		at("book table", "CREATE TABLE book (\n"+
			"	book_id BIGINT NOT NULL,\n"+
			"	author_id BIGINT NOT NULL,\n"+
			"	title STRING NOT NULL,\n"+
			"	published DATE,\n"+
			"	CONSTRAINT book_pk PRIMARY KEY (book_id),\n"+
			"	CONSTRAINT book_author_fk FOREIGN KEY (author_id) REFERENCES author (author_id)\n"+
			")"),
		// A CHECK constraint of Delta, which only a table property holds.
		at("book check", "ALTER TABLE book ADD CONSTRAINT title_not_empty CHECK (title <> '')"),

		at("view", "CREATE VIEW recent COMMENT 'recent books' AS\n"+
			"	SELECT book_id, title FROM book WHERE published IS NOT NULL"),

		// A composite primary key and a composite foreign key, named the way
		// every other fixture names them.
		at("region table", "CREATE TABLE region (\n"+
			"	country STRING NOT NULL,\n"+
			"	area STRING NOT NULL,\n"+
			"	CONSTRAINT region_pk PRIMARY KEY (country, area)\n"+
			")"),
		at("shipment table", "CREATE TABLE shipment (\n"+
			"	shipment_id BIGINT NOT NULL,\n"+
			"	country STRING NOT NULL,\n"+
			"	area STRING NOT NULL,\n"+
			"	amount INT NOT NULL,\n"+
			"	CONSTRAINT shipment_pk PRIMARY KEY (shipment_id),\n"+
			"	CONSTRAINT shipment_region_fk FOREIGN KEY (country, area) REFERENCES region (country, area)\n"+
			")"),

		// A table partitioned by two columns, which are the last two it names.
		at("sales table", "CREATE TABLE sales (\n"+
			"	amount INT NOT NULL,\n"+
			"	body STRING,\n"+
			"	sold_on DATE NOT NULL,\n"+
			"	region STRING NOT NULL\n"+
			") PARTITIONED BY (sold_on, region)\n"+
			"COMMENT 'sales by day and region'"),

		// A table with liquid clustering, which no relation reports.
		at("clicks table", "CREATE TABLE clicks (\n"+
			"	click_id BIGINT,\n"+
			"	region STRING,\n"+
			"	at TIMESTAMP\n"+
			") CLUSTER BY (region, at)"),

		// An identity column and a generated column, which INFORMATION_SCHEMA does
		// not report.
		at("ticket table", "CREATE TABLE ticket (\n"+
			"	ticket_id BIGINT GENERATED ALWAYS AS IDENTITY (START WITH 10 INCREMENT BY 2),\n"+
			"	note STRING,\n"+
			"	slug STRING GENERATED ALWAYS AS (lower(note))\n"+
			")"),

		// A shallow clone of author. It has the keys of author, under a name of its own.
		at("author clone", "CREATE TABLE author_clone SHALLOW CLONE author"),

		// A volume, a grant and a tag.
		at("volume", "CREATE VOLUME stash COMMENT 'files of the fixture'"),
		at("author tag", "ALTER TABLE author SET TAGS ('team' = 'books')"),
		at("author column tag", "ALTER TABLE author ALTER COLUMN name SET TAGS ('pii' = 'yes')"),
		at("book grant", "GRANT SELECT ON TABLE book TO `account users`"),

		// A SQL function with a comment, a function with a default and a mode, a
		// table function, a function in Python and a procedure.
		at("function", "CREATE FUNCTION double_it(x BIGINT COMMENT 'the number') RETURNS BIGINT\n"+
			"	COMMENT 'twice the argument' RETURN x * 2"),
		at("function with a default", "CREATE FUNCTION addup(a INT, b INT DEFAULT 5) RETURNS INT\n"+
			"	LANGUAGE SQL DETERMINISTIC RETURN a + b"),
		at("table function", "CREATE FUNCTION recent_books(since DATE) RETURNS TABLE (book_id BIGINT, title STRING)\n"+
			"	RETURN SELECT book_id, title FROM book WHERE published >= since"),
		at("python function", "CREATE FUNCTION shout(s STRING) RETURNS STRING LANGUAGE PYTHON\n"+
			"	AS $$return s.upper()$$"),
		at("procedure", "CREATE PROCEDURE bump(IN a INT, OUT b INT) LANGUAGE SQL SQL SECURITY INVOKER\n"+
			"	AS BEGIN SET b = a + 1; END"),

		// A row filter and a column mask. Each is a function that the table names.
		at("row filter function", "CREATE FUNCTION region_filter(r STRING) RETURNS BOOLEAN RETURN r = 'north'"),
		at("sales row filter", "ALTER TABLE sales SET ROW FILTER region_filter ON (region)"),
		at("mask function", "CREATE FUNCTION mask_name(n STRING) RETURNS STRING RETURN '***'"),
		at("author mask", "ALTER TABLE author ALTER COLUMN name SET MASK mask_name"),
	},
	Teardown: []Step{
		at("drop author mask", "ALTER TABLE author ALTER COLUMN name DROP MASK"),
		at("drop sales row filter", "ALTER TABLE sales DROP ROW FILTER"),
		at("drop mask function", "DROP FUNCTION IF EXISTS mask_name"),
		at("drop row filter function", "DROP FUNCTION IF EXISTS region_filter"),
		at("drop procedure", "DROP PROCEDURE IF EXISTS bump"),
		at("drop python function", "DROP FUNCTION IF EXISTS shout"),
		at("drop table function", "DROP FUNCTION IF EXISTS recent_books"),
		at("drop function with a default", "DROP FUNCTION IF EXISTS addup"),
		at("drop function", "DROP FUNCTION IF EXISTS double_it"),
		at("drop volume", "DROP VOLUME IF EXISTS stash"),
		at("drop author clone", "DROP TABLE IF EXISTS author_clone"),
		at("drop ticket", "DROP TABLE IF EXISTS ticket"),
		at("drop clicks", "DROP TABLE IF EXISTS clicks"),
		at("drop sales", "DROP TABLE IF EXISTS sales"),
		at("drop shipment", "DROP TABLE IF EXISTS shipment"),
		at("drop region", "DROP TABLE IF EXISTS region"),
		at("drop view", "DROP VIEW IF EXISTS recent"),
		at("drop book", "DROP TABLE IF EXISTS book"),
		at("drop author", "DROP TABLE IF EXISTS author"),
	},
}
