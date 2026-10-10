// Package fixture holds a known good BigQuery schema, in the GoogleSQL dialect.
//
// Like every fixture here it is exported API and additive: a later release can
// add an object and will not rename or remove one. See the PostgreSQL fixture
// for the rules, which are the same.
//
// It is not versioned. BigQuery is a service with no release that SQL reads.
// The Resolve methods keep the shape the other fixtures have so that a caller
// can treat them alike.
//
// # Where it builds
//
// The test account cannot create a dataset, so the fixture builds its objects in
// the dataset that the connection names, which is dbmeta. Its Schema is that
// name. Each table expires after seven days, and the teardown drops everything
// the setup made, so a run that ends early leaves tables that expire alone.
//
// # What it cannot build
//
// BigQuery refused every generated column that was tried, so the fixture has
// none. It builds an identity column, which is the nearest. It builds no
// sequence, trigger, type or role, because BigQuery has none. It builds a row
// access policy, which no query reads, so a test can show that the table still
// reports no row security. It builds no table with a policy tag, because that
// needs a taxonomy in Data Catalog.
//
// # The quota of a search index
//
// BigQuery limits the DDL statements that make or drop a search index or a
// vector index on one table in a day, and it counts by the name of the table. A
// day with many runs reaches the limit, and CREATE SEARCH INDEX or CREATE VECTOR
// INDEX then fails with quotaExceeded. The
// teardown does not drop the two indexes, because dropping the table drops them
// and a drop counts against the same limit. A test that meets the limit on an
// index step skips that index and says so.
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

// Everything is a dataset holding one of every object the BigQuery queries
// read.
//
// The names match the other fixtures, so a test that reads author and book on
// PostgreSQL reads the same two here.
var Everything = Fixture{
	Name:   "everything",
	Schema: "dbmeta",
	Setup: []Step{
		// A single column key, a not null column that has a description, a
		// nullable column and a column with a default. BigQuery records a key
		// and never checks it.
		at("author table", "CREATE TABLE author (\n"+
			"	author_id INT64 NOT NULL,\n"+
			"	name STRING NOT NULL OPTIONS (description = 'the full name'),\n"+
			"	rating INT64,\n"+
			"	shade STRING DEFAULT 'red',\n"+
			"	PRIMARY KEY (author_id) NOT ENFORCED\n"+
			") OPTIONS (description = 'people who write books')"),

		at("book table", "CREATE TABLE book (\n"+
			"	book_id INT64 NOT NULL,\n"+
			"	author_id INT64 NOT NULL,\n"+
			"	title STRING NOT NULL,\n"+
			"	published DATE,\n"+
			"	PRIMARY KEY (book_id) NOT ENFORCED,\n"+
			"	CONSTRAINT book_author_fk FOREIGN KEY (author_id)\n"+
			"		REFERENCES author (author_id) NOT ENFORCED\n"+
			")"),

		at("view", "CREATE VIEW recent OPTIONS (description = 'recent books') AS\n"+
			"	SELECT book_id, title FROM book WHERE published IS NOT NULL"),

		// A composite primary key and a composite foreign key, named the way
		// every other fixture names them.
		at("region table", "CREATE TABLE region (\n"+
			"	country STRING NOT NULL,\n"+
			"	area STRING NOT NULL,\n"+
			"	PRIMARY KEY (country, area) NOT ENFORCED\n"+
			")"),
		at("shipment table", "CREATE TABLE shipment (\n"+
			"	shipment_id INT64 NOT NULL,\n"+
			"	country STRING NOT NULL,\n"+
			"	area STRING NOT NULL,\n"+
			"	amount INT64 NOT NULL,\n"+
			"	PRIMARY KEY (shipment_id) NOT ENFORCED,\n"+
			"	CONSTRAINT shipment_region_fk FOREIGN KEY (country, area)\n"+
			"		REFERENCES region (country, area) NOT ENFORCED\n"+
			")"),

		// A table partitioned by a date and clustered by a column, with two
		// partition options. The search index covers the body.
		at("sales table", "CREATE TABLE sales (\n"+
			"	sold_on DATE NOT NULL,\n"+
			"	region STRING NOT NULL,\n"+
			"	amount INT64 NOT NULL,\n"+
			"	body STRING\n"+
			") PARTITION BY sold_on CLUSTER BY region\n"+
			"OPTIONS (partition_expiration_days = 3650, require_partition_filter = TRUE)"),
		at("sales rows", "INSERT INTO sales (sold_on, region, amount, body) VALUES\n"+
			"	(DATE '2026-01-01', 'north', 10, 'first sale'),\n"+
			"	(DATE '2026-01-02', 'south', 20, 'second sale')"),
		at("sales search index", "CREATE SEARCH INDEX sales_search ON sales (body)"),
		at("materialized view", "CREATE MATERIALIZED VIEW sales_total AS\n"+
			"	SELECT sold_on, SUM(amount) AS total FROM dbmeta.sales GROUP BY sold_on"),

		// An identity column.
		at("ticket table", "CREATE TABLE ticket (\n"+
			"	ticket_id INT64 GENERATED ALWAYS AS IDENTITY (START WITH 10 INCREMENT BY 2),\n"+
			"	note STRING\n"+
			")"),

		// A table of vectors and the vector index on it. The rows are what a
		// vector index needs to build.
		at("plain table", "CREATE TABLE plain (\n"+
			"	plain_id INT64,\n"+
			"	embedding ARRAY<FLOAT64>\n"+
			")"),
		at("plain rows", "INSERT INTO plain (plain_id, embedding)\n"+
			"	SELECT x, [CAST(x AS FLOAT64), 1.0, 2.0] FROM UNNEST(GENERATE_ARRAY(1, 6000)) AS x"),
		at("plain vector index", "CREATE VECTOR INDEX plain_vector ON plain (embedding)\n"+
			"	OPTIONS (index_type = 'IVF', distance_type = 'COSINE')"),

		// A snapshot and a clone of author, and an external table over a
		// public file. BigQuery does not read the file until something queries
		// the table.
		at("author snapshot", "CREATE SNAPSHOT TABLE author_snap CLONE author"),
		at("author clone", "CREATE TABLE author_clone CLONE author"),
		at("external table", "CREATE EXTERNAL TABLE states (name STRING, abbr STRING)\n"+
			"	OPTIONS (format = 'CSV', uris = ['gs://cloud-samples-data/bigquery/us-states/us-states.csv'])"),

		// A row access policy. No INFORMATION_SCHEMA view that a dataset
		// principal can read lists it.
		at("row access policy", "CREATE ROW ACCESS POLICY recent_only ON author FILTER USING (rating > 3)"),

		// A SQL function with a description, a function in JavaScript, a table
		// function, an aggregate function and a procedure.
		at("function", "CREATE FUNCTION double_it(x INT64) RETURNS INT64\n"+
			"	OPTIONS (description = 'twice the argument') AS (x * 2)"),
		at("javascript function", "CREATE FUNCTION shout(s STRING) RETURNS STRING DETERMINISTIC LANGUAGE js\n"+
			"	AS 'return s.toUpperCase();'"),
		at("table function", "CREATE TABLE FUNCTION recent_books(since DATE) AS\n"+
			"	SELECT book_id, title FROM book WHERE published >= since"),
		at("aggregate function", "CREATE AGGREGATE FUNCTION my_sum(x INT64) RETURNS INT64 AS (SUM(x))"),
		at("procedure", "CREATE PROCEDURE addup(IN a INT64, IN b INT64, OUT c INT64)\n"+
			"BEGIN\n"+
			"	SET c = a + b;\n"+
			"END"),
	},
	Teardown: []Step{
		at("drop procedure", "DROP PROCEDURE IF EXISTS addup"),
		at("drop aggregate function", "DROP FUNCTION IF EXISTS my_sum"),
		at("drop table function", "DROP TABLE FUNCTION IF EXISTS recent_books"),
		at("drop javascript function", "DROP FUNCTION IF EXISTS shout"),
		at("drop function", "DROP FUNCTION IF EXISTS double_it"),
		at("drop row access policy", "DROP ALL ROW ACCESS POLICIES ON author"),
		at("drop external table", "DROP EXTERNAL TABLE IF EXISTS states"),
		at("drop author clone", "DROP TABLE IF EXISTS author_clone"),
		at("drop author snapshot", "DROP SNAPSHOT TABLE IF EXISTS author_snap"),
		at("drop plain", "DROP TABLE IF EXISTS plain"),
		at("drop ticket", "DROP TABLE IF EXISTS ticket"),
		at("drop materialized view", "DROP MATERIALIZED VIEW IF EXISTS sales_total"),
		at("drop sales", "DROP TABLE IF EXISTS sales"),
		at("drop shipment", "DROP TABLE IF EXISTS shipment"),
		at("drop region", "DROP TABLE IF EXISTS region"),
		at("drop view", "DROP VIEW IF EXISTS recent"),
		at("drop book", "DROP TABLE IF EXISTS book"),
		at("drop author", "DROP TABLE IF EXISTS author"),
	},
}
