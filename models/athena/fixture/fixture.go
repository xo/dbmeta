// Package fixture holds a known good Amazon Athena schema, in the Trino based
// SQL dialect of Athena engine version 3.
//
// Like every fixture here it is exported API and additive: a later release can
// add an object and will not rename or remove one. See the PostgreSQL fixture
// for the rules, which are the same.
//
// It is not versioned. Athena is a service with no release that SQL reads. The
// Resolve methods keep the shape the other fixtures have so that a caller can
// treat them alike.
//
// # Where it builds
//
// The test account cannot create a Glue database, so the fixture builds its
// tables in the Glue database that the connection names, which is dbmeta. Its
// Schema is that name. The Redshift Spectrum test reads the same database and
// expects the table spectrum_t in it, so the teardown drops only what the setup
// made.
//
// A table that holds no data needs a location in S3, and the account allows one
// only under a prefix of its bucket. The caller gives that prefix to the Resolve
// methods, and each step names the table as the last part of it. No step writes
// a file, except the table that a CTAS makes, which writes to the output location
// of the workgroup because the workgroup forces one.
//
// # What it cannot build
//
// Athena has no constraint, no index, no sequence, no trigger, no user defined
// type and no role, so the fixture has none. A Glue column has no NOT NULL and no
// default, so every column of the fixture is nullable and has no default. It
// builds an Iceberg table, a table made by CTAS and a table partitioned by two
// columns, which are the three ways that a table differs here.
package fixture

import (
	"strings"

	"github.com/xo/dbmeta"
)

// Step is one statement of a fixture. The text {location} in it stands for the
// S3 prefix that the caller gives to the Resolve methods, which ends in a slash.
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

// Fixture is a Glue database, with the statements that build it and drop it.
type Fixture struct {
	Name     string
	Schema   string
	Setup    []Step
	Teardown []Step
}

// ResolveSetup returns the statements that build the fixture. The location is
// the S3 prefix, such as s3://bucket/tables/dbmeta/, that the tables are put
// under. A missing final slash is added.
func (f Fixture) ResolveSetup(versions dbmeta.VersionSet, location string) ([]Result, error) {
	return resolve(f.Setup, versions, location)
}

// ResolveTeardown returns the statements that drop it.
func (f Fixture) ResolveTeardown(versions dbmeta.VersionSet, location string) ([]Result, error) {
	return resolve(f.Teardown, versions, location)
}

func resolve(steps []Step, versions dbmeta.VersionSet, location string) ([]Result, error) {
	if !strings.HasSuffix(location, "/") {
		location += "/"
	}
	out := make([]Result, 0, len(steps))
	for _, step := range steps {
		query, err := step.Stmt.Build(versions)
		if err != nil {
			return nil, err
		}
		query = strings.ReplaceAll(query, "{location}", location)
		out = append(out, Result{Name: step.Name, Query: query})
	}
	return out, nil
}

func at(name, query string) Step {
	return Step{Name: name, Stmt: dbmeta.Always(query)}
}

// Everything is a Glue database holding one of every object the Athena queries
// read.
//
// The names match the other fixtures, so a test that reads author and book on
// PostgreSQL reads the same two here.
var Everything = Fixture{
	Name:   "everything",
	Schema: "dbmeta",
	Setup: []Step{
		// A table with a comment, two column comments and two columns with
		// none. A Glue table has no key.
		at("author table", "CREATE EXTERNAL TABLE dbmeta.author (\n"+
			"	author_id int COMMENT 'the key',\n"+
			"	name string COMMENT 'the full name',\n"+
			"	rating int,\n"+
			"	shade string\n"+
			") COMMENT 'people who write books'\n"+
			"STORED AS PARQUET\n"+
			"LOCATION '{location}author/'"),

		at("book table", "CREATE EXTERNAL TABLE dbmeta.book (\n"+
			"	book_id int,\n"+
			"	author_id int,\n"+
			"	title string,\n"+
			"	published date\n"+
			")\n"+
			"STORED AS PARQUET\n"+
			"LOCATION '{location}book/'"),

		at("view", "CREATE VIEW dbmeta.recent AS\n"+
			"	SELECT book_id, title FROM dbmeta.book WHERE published IS NOT NULL"),

		at("region table", "CREATE EXTERNAL TABLE dbmeta.region (\n"+
			"	country string,\n"+
			"	area string\n"+
			")\n"+
			"STORED AS PARQUET\n"+
			"LOCATION '{location}region/'"),
		at("shipment table", "CREATE EXTERNAL TABLE dbmeta.shipment (\n"+
			"	shipment_id int,\n"+
			"	country string,\n"+
			"	area string,\n"+
			"	amount int\n"+
			")\n"+
			"STORED AS PARQUET\n"+
			"LOCATION '{location}shipment/'"),

		// A Hive table partitioned by two columns, with one partition added.
		// A partition column is the last in the table and has no comment here.
		at("sales table", "CREATE EXTERNAL TABLE dbmeta.sales (\n"+
			"	amount bigint,\n"+
			"	body string\n"+
			") COMMENT 'sales by day and region'\n"+
			"PARTITIONED BY (sold_on string COMMENT 'the day', region string)\n"+
			"STORED AS PARQUET\n"+
			"LOCATION '{location}sales/'"),
		at("sales partition", "ALTER TABLE dbmeta.sales ADD PARTITION (sold_on = '2026-01-01', region = 'north')\n"+
			"	LOCATION '{location}sales/2026-01-01/north/'"),

		// A table that CTAS makes. It writes one file, under the output location
		// of the workgroup.
		at("ctas table", "CREATE TABLE dbmeta.tally WITH (format = 'PARQUET') AS\n"+
			"	SELECT * FROM (VALUES (1, 'one')) AS t (tally_id, label)"),

		// An Iceberg table, partitioned by a transform of a column.
		at("iceberg table", "CREATE TABLE dbmeta.ticket (\n"+
			"	ticket_id int,\n"+
			"	note string,\n"+
			"	created timestamp\n"+
			")\n"+
			"PARTITIONED BY (day(created))\n"+
			"LOCATION '{location}ticket/'\n"+
			"TBLPROPERTIES ('table_type' = 'ICEBERG')"),
	},
	Teardown: []Step{
		at("drop ticket", "DROP TABLE IF EXISTS dbmeta.ticket"),
		at("drop tally", "DROP TABLE IF EXISTS dbmeta.tally"),
		at("drop sales", "DROP TABLE IF EXISTS dbmeta.sales"),
		at("drop shipment", "DROP TABLE IF EXISTS dbmeta.shipment"),
		at("drop region", "DROP TABLE IF EXISTS dbmeta.region"),
		at("drop view", "DROP VIEW IF EXISTS dbmeta.recent"),
		at("drop book", "DROP TABLE IF EXISTS dbmeta.book"),
		at("drop author", "DROP TABLE IF EXISTS dbmeta.author"),
	},
}
