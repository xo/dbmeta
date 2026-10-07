// Package fixture holds a known good Apache Druid cluster.
//
// A fixture is a set of objects that the metadata queries read, so that a
// test asks a real server for metadata and gets an answer worth checking.
//
// Druid has no DDL. A datasource exists when an ingestion task writes its
// first segment, and the SQL API takes INSERT and REPLACE only as a task of
// the multi-stage engine, through POST /druid/v2/sql/task. dbimp's druid
// driver reads and never writes (dbimp D163), so a test sends each step to the
// Router as an HTTP request and waits for the task to end.
//
// # Four datasources
//
// The fixture makes the four core tables of D53 and nothing else. A task
// takes from 5 to 12 seconds, and the nano quickstart has two slots, one for
// the controller and one for the worker, so two tasks at once never end. A
// test runs the steps one after the other.
//
// # Safe to run again
//
// A step names the datasource it makes. A test skips a step when that
// datasource already has a segment, so a second run waits for nothing. Each
// step uses REPLACE ... OVERWRITE ALL, so a step that runs again writes the
// same rows.
//
// # What Druid cannot build
//
// A datasource has no key, no index, no constraint, no trigger and no
// comment, and Druid has no view, sequence or type that a statement makes.
// The core view recent and the core objects that need them are not here.
// Every query that reads one of those is unanswered, and docs/COVERAGE.md says
// why.
package fixture

// Step is one ingestion task, as the SQL that the task API takes.
type Step struct {
	// Name is the datasource the step makes.
	Name string
	// Query is the REPLACE statement.
	Query string
}

// Fixture is a set of datasources, with the tasks that make them.
type Fixture struct {
	Name   string
	Schema string
	Setup  []Step
}

// replace writes one datasource from a list of values. Each column is cast,
// so that its type does not depend on the first row. `__time` is the same
// instant for every row.
func replace(name, columns, casts, values string) Step {
	return Step{
		Name: name,
		Query: "REPLACE INTO " + name + " OVERWRITE ALL SELECT TIME_PARSE('2026-01-01') AS __time, " +
			casts + " FROM (VALUES " + values + ") AS t(" + columns + ") PARTITIONED BY ALL",
	}
}

// Everything holds the four core datasources of D53. A datasource keeps no
// key and no constraint, so the columns are the whole of each.
var Everything = Fixture{
	Name:   "everything",
	Schema: "druid",
	Setup: []Step{
		replace("author", "author_id, name, rating, shade",
			"CAST(author_id AS BIGINT) AS author_id, name, CAST(rating AS BIGINT) AS rating, shade",
			"(1, 'Ursula', 5, 'red'), (2, 'Terry', CAST(NULL AS INTEGER), 'blue')"),
		replace("book", "book_id, author_id, title, published",
			"CAST(book_id AS BIGINT) AS book_id, CAST(author_id AS BIGINT) AS author_id, title, published",
			"(1, 1, 'A Wizard of Earthsea', '1968-01-01')"),
		replace("region", "country, area, population",
			"country, area, CAST(population AS BIGINT) AS population",
			"('nz', 'north', 1)"),
		replace("shipment", "shipment_id, country, area, amount, weight, fragile",
			"CAST(shipment_id AS BIGINT) AS shipment_id, country, area, CAST(amount AS BIGINT) AS amount,"+
				" CAST(weight AS DOUBLE) AS weight, CAST(fragile AS BIGINT) AS fragile",
			"(1, 'nz', 'north', 10, 1.5, 1)"),
	},
}
