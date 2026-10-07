// Package fixture holds a known good Apache Drill server.
//
// A fixture is a set of objects that the metadata queries read, so that a
// test asks a real server for metadata and gets an answer worth checking.
//
// # The Metastore
//
// INFORMATION_SCHEMA lists no file table unless the Drill Metastore is on and
// the table was analyzed. So the first step turns the Metastore on, as the
// administrator, with ALTER SYSTEM. It is a change to the whole server, and
// the ordinary user cannot make it. The fixture then makes each table with
// CREATE TABLE AS in dfs.tmp, and runs ANALYZE TABLE ... REFRESH METADATA for
// each. D178 records the price.
//
// # Four tables and a view
//
// The fixture makes the four core tables of D53, which are author, book,
// region and shipment, and the view recent. Each table is a Parquet file in
// the directory of dfs.tmp, and every column is cast so that its type does not
// depend on the first row.
//
// # Safe to run again
//
// Setup runs Teardown first, which turns the Metastore on. A table that was analyzed and then dropped stays
// in the Metastore, so Teardown drops the metadata before the table. Each of
// those statements answers ok false and no error when the object is not there.
//
// # What Drill cannot build
//
// A table has no key, no index, no constraint, no trigger, no sequence and no
// comment, and Drill has no type, role or privilege that a statement makes. A
// function needs a JAR file. INSERT, UPDATE and DELETE fail. A column that is
// NOT NULL is an artifact of Parquet written from literals. Every query that
// reads one of those is unanswered, and docs/COVERAGE.md says why.
package fixture

// Step is one statement.
type Step struct {
	Name  string
	Query string
}

// Fixture is a set of objects, with the statements that make and remove them.
type Fixture struct {
	Name     string
	Schema   string
	Setup    []Step
	Teardown []Step
}

func at(name, query string) Step { return Step{Name: name, Query: query} }

// metastore turns the Metastore on.
var metastore = at("metastore", "ALTER SYSTEM SET `metastore.enabled` = true")

// tables are the four core tables.
var tables = []string{"author", "book", "region", "shipment"}

// teardown drops what a table and the view leave behind, view first.
func teardown() []Step {
	// ANALYZE TABLE ... DROP METADATA is refused while the Metastore is off.
	steps := []Step{metastore, at("recent", "DROP VIEW IF EXISTS dfs.tmp.recent")}
	for _, name := range tables {
		steps = append(steps,
			at(name+" metadata", "ANALYZE TABLE dfs.tmp."+name+" DROP METADATA IF EXISTS"),
			at(name, "DROP TABLE IF EXISTS dfs.tmp."+name))
	}
	return steps
}

// setup builds the Metastore, the tables, the view and the statistics.
func setup() []Step {
	steps := []Step{
		at("author", "CREATE TABLE dfs.tmp.author AS SELECT CAST(author_id AS BIGINT) AS author_id, name,"+
			" CAST(rating AS BIGINT) AS rating, shade FROM (VALUES (1, 'Ursula', 5, 'red'),"+
			" (2, 'Terry', CAST(NULL AS INTEGER), 'blue')) AS t(author_id, name, rating, shade)"),
		at("book", "CREATE TABLE dfs.tmp.book AS SELECT CAST(book_id AS BIGINT) AS book_id,"+
			" CAST(author_id AS BIGINT) AS author_id, title, CAST(published AS DATE) AS published"+
			" FROM (VALUES (1, 1, 'A Wizard of Earthsea', '1968-01-01')) AS t(book_id, author_id, title, published)"),
		at("region", "CREATE TABLE dfs.tmp.region AS SELECT country, area, CAST(population AS BIGINT) AS population"+
			" FROM (VALUES ('nz', 'north', 1)) AS t(country, area, population)"),
		at("shipment", "CREATE TABLE dfs.tmp.shipment AS SELECT CAST(shipment_id AS BIGINT) AS shipment_id,"+
			" country, area, CAST(amount AS DECIMAL(10, 2)) AS amount, CAST(weight AS DOUBLE) AS weight,"+
			" CAST(fragile AS BIGINT) AS fragile FROM (VALUES (1, 'nz', 'north', 10.5, 1.5, 1))"+
			" AS t(shipment_id, country, area, amount, weight, fragile)"),
		at("recent", "CREATE VIEW dfs.tmp.recent AS SELECT shipment_id, amount FROM dfs.tmp.shipment WHERE amount > 5"),
	}
	for _, name := range tables {
		steps = append(steps, at(name+" statistics", "ANALYZE TABLE dfs.tmp."+name+" REFRESH METADATA"))
	}
	return steps
}

// Everything holds the four core tables of D53 and a view. The Setup drops
// them first, so it is safe to run twice.
var Everything = Fixture{
	Name:     "everything",
	Schema:   "dfs.tmp",
	Setup:    append(teardown(), setup()...),
	Teardown: teardown(),
}
