// Package fixture holds a known good Alibaba Cloud MaxCompute project, in the
// SQL of MaxCompute.
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
// The test account owns one project, dbmeta, and the fixture builds its objects
// in it. The project has no schema, because it is not schema enabled, so Schema
// is the name of the project. The steps write no name in front of an object, so
// they follow the project of the connection. The teardown drops everything the
// setup made and never the project.
//
// Every statement ends with a semicolon, because the service refuses one that
// does not.
//
// # What it cannot build
//
// MaxCompute has no foreign key, so the fixture has none, and no index, no
// sequence, no trigger and no user defined type. It builds a primary key on a
// transactional table, which INFORMATION_SCHEMA does not report, so that a test
// can show that no query lists it. It builds no external table, because that
// needs a RAM role and an OSS bucket that the account does not have, so the
// query for external tables is not written. It builds no user, because a user
// is a RAM identity that only the account owner can make, and it builds a role.
//
// # What a person must do first
//
// The views of INFORMATION_SCHEMA exist only after the administrator runs
// `install package Information_Schema.systables;` once for the project. A
// principal that is not the administrator needs CreateInstance on the project
// and read on the package. See docs/COVERAGE.md.
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

// Fixture is a project, with the statements that build it and drop it.
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

// Everything is a project holding one of every object the MaxCompute queries
// read.
//
// The names match the other fixtures, so a test that reads author and book on
// PostgreSQL reads the same two here.
var Everything = Fixture{
	Name:   "everything",
	Schema: "dbmeta",
	Setup: []Step{
		// A table with a comment, two column comments, a column that is NOT NULL
		// and a column with a default.
		at("author table", "CREATE TABLE IF NOT EXISTS author (\n"+
			"	author_id BIGINT NOT NULL COMMENT 'the key',\n"+
			"	name STRING NOT NULL COMMENT 'the full name',\n"+
			"	rating INT,\n"+
			"	shade STRING DEFAULT 'red'\n"+
			") COMMENT 'people who write books';"),

		at("book table", "CREATE TABLE IF NOT EXISTS book (\n"+
			"	book_id BIGINT NOT NULL,\n"+
			"	author_id BIGINT NOT NULL,\n"+
			"	title STRING NOT NULL,\n"+
			"	published DATE\n"+
			");"),

		at("view", "CREATE VIEW IF NOT EXISTS recent COMMENT 'recent books' AS\n"+
			"	SELECT book_id, title FROM book WHERE published IS NOT NULL;"),

		at("region table", "CREATE TABLE IF NOT EXISTS region (\n"+
			"	country STRING NOT NULL,\n"+
			"	area STRING NOT NULL\n"+
			");"),
		at("shipment table", "CREATE TABLE IF NOT EXISTS shipment (\n"+
			"	shipment_id BIGINT NOT NULL,\n"+
			"	country STRING NOT NULL,\n"+
			"	area STRING NOT NULL,\n"+
			"	amount INT NOT NULL\n"+
			");"),

		// A table partitioned by two columns, which are the last two it names,
		// with one partition added.
		at("sales table", "CREATE TABLE IF NOT EXISTS sales (\n"+
			"	amount INT NOT NULL,\n"+
			"	body STRING\n"+
			") COMMENT 'sales by day and region'\n"+
			"PARTITIONED BY (sold_on STRING COMMENT 'the day', region STRING);"),
		at("sales partition", "ALTER TABLE sales ADD IF NOT EXISTS PARTITION (sold_on = '2026-01-01', region = 'north');"),

		// A transactional table, which is the only kind that has a primary key.
		at("ticket table", "CREATE TABLE IF NOT EXISTS ticket (\n"+
			"	ticket_id BIGINT NOT NULL,\n"+
			"	note STRING COMMENT 'what happened',\n"+
			"	PRIMARY KEY (ticket_id)\n"+
			") COMMENT 'a transactional table'\n"+
			"TBLPROPERTIES ('transactional' = 'true');"),

		// A materialized view of one table.
		at("materialized view", "CREATE MATERIALIZED VIEW IF NOT EXISTS book_titles COMMENT 'titles of books' AS\n"+
			"	SELECT title FROM book;"),

		// A SQL function. The service has no function without a resource other
		// than this.
		at("function", "CREATE SQL FUNCTION double_it(@x BIGINT) AS @x * 2;"),

		// A role, and the grants it holds on a table, and on a column.
		at("role", "CREATE ROLE report_reader;"),
		at("book grant", "GRANT SELECT, DESCRIBE ON TABLE book TO ROLE report_reader;"),
		at("author column grant", "GRANT SELECT ON TABLE author (name) TO ROLE report_reader;"),
	},
	Teardown: []Step{
		at("drop role", "DROP ROLE report_reader;"),
		at("drop function", "DROP FUNCTION double_it;"),
		at("drop materialized view", "DROP MATERIALIZED VIEW IF EXISTS book_titles;"),
		at("drop ticket", "DROP TABLE IF EXISTS ticket;"),
		at("drop sales", "DROP TABLE IF EXISTS sales;"),
		at("drop shipment", "DROP TABLE IF EXISTS shipment;"),
		at("drop region", "DROP TABLE IF EXISTS region;"),
		at("drop view", "DROP VIEW IF EXISTS recent;"),
		at("drop book", "DROP TABLE IF EXISTS book;"),
		at("drop author", "DROP TABLE IF EXISTS author;"),
	},
}
