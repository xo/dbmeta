// Package fixture builds the directory the YDB queries read.
//
// It creates one of every object those queries report, so a test has rows
// worth checking rather than an empty catalog. Hard rule 9 requires it, and
// D53 requires the core objects to match every other fixture, so that the
// cross family comparison reads the same schema everywhere.
//
// # A directory is a schema
//
// The fixture is the directory dbmeta/dbmeta_fixture, because the ordinary
// user that dbrun makes can read and describe the directory dbmeta and
// everything in it. A table names its directory in its path, and YDB makes
// a directory when a table needs one.
//
// YQL has no statement that makes or removes a directory. So the teardown
// leaves dbmeta_fixture behind, empty, and the empty directory step makes a
// directory by making a table in it and dropping the table.
//
// # What YDB cannot be asked for
//
// No foreign key, no check and no unique constraint, because YDB has none.
// A unique index is the nearest, and book has one on title. No sequence of
// its own: a Serial column makes one, and shipment has one, and no view
// lists it. No trigger, no function and no user defined type.
//
// The view, the topic, the changefeed, the column table and the indexes are
// built and only some are read. No view lists a view, a topic or a
// changefeed, and no view says which tables implement an index. They are
// here so that a test can assert that each one is left out.
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

// Fixture is a directory and the statements that build and remove what is
// in it.
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

// Schema is the directory the fixture builds in, relative to the database.
const Schema = "dbmeta/dbmeta_fixture"

// path quotes the path of an object in the fixture's directory.
func path(name string) string {
	return "`" + Schema + "/" + name + "`"
}

// Grantee is the user the fixture makes, and Readers is its group. YDB
// allows no underscore in a user name or a group name.
const (
	Grantee = "dbmetagrantee"
	Readers = "dbmetareaders"
)

// Everything is a directory holding one of every object the YDB queries
// read.
var Everything = Fixture{
	Name:   "everything",
	Schema: Schema,
	Setup: []Step{
		// The core objects D53 asks every fixture for. Every row table
		// needs a primary key, and a key column must be NOT NULL to be
		// reported so.
		at("author", `CREATE TABLE `+path("author")+` (
	author_id Int32 NOT NULL,
	name Utf8 NOT NULL,
	rating Int32,
	shade Utf8 DEFAULT 'red',
	PRIMARY KEY (author_id)
)`),

		// book_published is an index somebody made, and book_title is the
		// nearest YDB has to a unique constraint.
		at("book", `CREATE TABLE `+path("book")+` (
	book_id Int32 NOT NULL,
	author_id Int32 NOT NULL,
	title Utf8 NOT NULL,
	published Date32,
	PRIMARY KEY (book_id),
	INDEX book_published GLOBAL ON (published),
	INDEX book_title GLOBAL UNIQUE SYNC ON (title)
)`),

		at("region", `CREATE TABLE `+path("region")+` (
	country Utf8 NOT NULL,
	area Utf8 NOT NULL,
	PRIMARY KEY (country, area)
)`),

		// A Serial column, which makes a sequence that no view lists.
		at("shipment", `CREATE TABLE `+path("shipment")+` (
	shipment_id Serial NOT NULL,
	country Utf8,
	area Utf8,
	amount Decimal(12, 2),
	PRIMARY KEY (shipment_id),
	INDEX shipment_region GLOBAL ON (country, area)
)`),

		// A view, which no view lists.
		at("recent", `CREATE VIEW `+path("recent")+` WITH (security_invoker = TRUE) AS
	SELECT book_id, title FROM `+path("book")),

		// A column table, which a column shard holds, in two partitions.
		at("ledger", `CREATE TABLE `+path("ledger")+` (
	entry_id Int64 NOT NULL,
	amount Double,
	PRIMARY KEY (entry_id)
) PARTITION BY HASH(entry_id) WITH (STORE = COLUMN, AUTO_PARTITIONING_MIN_PARTITIONS_COUNT = 2)`),

		// A topic and a changefeed, which no view lists either.
		at("events", `CREATE TOPIC `+path("events")),
		at("changefeed", `ALTER TABLE `+path("author")+
			` ADD CHANGEFEED author_changes WITH (FORMAT = 'JSON', MODE = 'UPDATES')`),

		// A directory that holds nothing, which Schemas cannot tell from a
		// view and leaves out. A table makes it and is dropped.
		at("empty directory", `CREATE TABLE `+path("empty/gone")+` (id Int32 NOT NULL, PRIMARY KEY (id))`),
		at("empty directory drop", `DROP TABLE `+path("empty/gone")),

		// A user, a group, a membership and a grant, so that roles, role
		// grants and privileges have a row each.
		at("user", `CREATE USER `+Grantee+` PASSWORD 'P4ssw0rd!x'`),
		at("group", `CREATE GROUP `+Readers),
		at("member", `ALTER GROUP `+Readers+` ADD USER `+Grantee),
		at("permission", `GRANT SELECT ON `+path("author")+` TO `+Readers),

		// Rows, so that a table has data in it.
		at("author rows", `UPSERT INTO `+path("author")+` (author_id, name, rating)`+
			` VALUES (1, 'Ursula', 5)`),
		at("book rows", `UPSERT INTO `+path("book")+` (book_id, author_id, title, published)`+
			` VALUES (1, 1, 'A Wizard of Earthsea', Date32('1968-01-01'))`),
		at("region rows", `UPSERT INTO `+path("region")+` (country, area) VALUES ('US', 'west')`),
		at("shipment rows", `INSERT INTO `+path("shipment")+` (country, area, amount)`+
			` VALUES ('US', 'west', Decimal('12.50', 12, 2))`),
	},
	Teardown: []Step{
		// The changefeed goes with its table and an index goes with its
		// table. The directories stay, because YQL cannot remove one.
		at("recent", `DROP VIEW IF EXISTS `+path("recent")),
		at("events", `DROP TOPIC IF EXISTS `+path("events")),
		at("author", `DROP TABLE IF EXISTS `+path("author")),
		at("book", `DROP TABLE IF EXISTS `+path("book")),
		at("region", `DROP TABLE IF EXISTS `+path("region")),
		at("shipment", `DROP TABLE IF EXISTS `+path("shipment")),
		at("ledger", `DROP TABLE IF EXISTS `+path("ledger")),
		at("empty directory", `DROP TABLE IF EXISTS `+path("empty/gone")),
		at("user", `DROP USER IF EXISTS `+Grantee),
		at("group", `DROP GROUP IF EXISTS `+Readers),
	},
}
