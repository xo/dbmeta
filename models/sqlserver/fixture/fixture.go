// Package fixture holds a known good Microsoft SQL Server schema.
//
// Like every fixture here it is exported API and additive: a later release may
// add an object and will not rename or remove one. See the PostgreSQL fixture
// for the rules, which are the same.
//
// It is versioned on the SQL Server release, and one step needs release 13,
// which is SQL Server 2016.
package fixture

import (
	"errors"

	"github.com/xo/dbmeta"
)

// Releases a step needs. SQL Server 2016 is 13.
var v13 = dbmeta.V(13)

// Step is one statement of a fixture, with its alternatives by version.
type Step struct {
	Name string
	Stmt dbmeta.Stmt
}

// Result is what a step resolved to for one server.
type Result struct {
	Name    string
	SQL     string
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

// ResolveSetup returns the statements that build the fixture on this server.
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
		sqlstr, err := step.Stmt.SQL(versions)
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
			out = append(out, Result{Name: step.Name, SQL: sqlstr})
		}
	}
	return out, nil
}

func at(name, sqlstr string) Step {
	return Step{Name: name, Stmt: dbmeta.Always(sqlstr)}
}

func from(name string, since dbmeta.Version, sqlstr string) Step {
	return Step{Name: name, Stmt: dbmeta.Stmt{{{Min: since, SQL: sqlstr}}}}
}

// Everything is a schema holding one of every object the SQL Server queries
// read.
//
// The names match the other fixtures, because the cross family conformance
// test reads the same core objects everywhere. See D53.
//
// Each statement is run on its own. SQL Server needs CREATE SCHEMA and
// CREATE VIEW to be the first statement of a batch, which a caller running one
// statement per Exec satisfies without a GO separator.
var Everything = Fixture{
	Name:   "everything",
	Schema: "dbmeta_fixture",
	Setup: []Step{
		at("schema", `CREATE SCHEMA dbmeta_fixture`),

		at("sequence", `CREATE SEQUENCE dbmeta_fixture.counter
	AS bigint START WITH 10 INCREMENT BY 2`),

		// An alias type, which is what SQL Server has instead of a domain.
		at("alias type", `CREATE TYPE dbmeta_fixture.shortname FROM nvarchar(64) NOT NULL`),

		at("author table", `CREATE TABLE dbmeta_fixture.author (
	author_id int NOT NULL CONSTRAINT author_pk PRIMARY KEY,
	name nvarchar(255) NOT NULL,
	rating int NULL,
	shade nvarchar(9) NULL CONSTRAINT author_shade_default DEFAULT 'red'
)`),

		at("book table", `CREATE TABLE dbmeta_fixture.book (
	book_id int NOT NULL CONSTRAINT book_pk PRIMARY KEY,
	author_id int NOT NULL CONSTRAINT book_author_fk
		REFERENCES dbmeta_fixture.author(author_id),
	title nvarchar(255) NOT NULL CONSTRAINT book_title_unique UNIQUE,
	published date NULL,
	CONSTRAINT title_not_empty CHECK (title <> '')
)`),
		at("book index", `CREATE INDEX book_published ON dbmeta_fixture.book (published)`),

		at("view", `CREATE VIEW dbmeta_fixture.recent AS
	SELECT book_id, title FROM dbmeta_fixture.book WHERE published IS NOT NULL`),

		// A composite primary key and a composite foreign key.
		at("region table", `CREATE TABLE dbmeta_fixture.region (
	country nvarchar(64) NOT NULL,
	area nvarchar(64) NOT NULL,
	CONSTRAINT region_pk PRIMARY KEY (country, area)
)`),
		at("shipment table", `CREATE TABLE dbmeta_fixture.shipment (
	shipment_id int NOT NULL CONSTRAINT shipment_pk PRIMARY KEY,
	country nvarchar(64) NOT NULL,
	area nvarchar(64) NOT NULL,
	amount int NOT NULL,
	CONSTRAINT shipment_region_fk FOREIGN KEY (country, area)
		REFERENCES dbmeta_fixture.region(country, area)
)`),

		// An identity column and a computed column, on a table of their own
		// rather than on a core table. See D53.
		at("extras table", `CREATE TABLE dbmeta_fixture.extras (
	extras_id int IDENTITY(1,1) NOT NULL CONSTRAINT extras_pk PRIMARY KEY,
	title nvarchar(255) NOT NULL,
	slug AS LOWER(title) PERSISTED
)`),

		at("procedure", `CREATE PROCEDURE dbmeta_fixture.addup
	@a int, @b int, @total int OUTPUT
AS
BEGIN
	SET @total = @a + @b
END`),
		at("function", `CREATE FUNCTION dbmeta_fixture.shout(@s nvarchar(255))
RETURNS nvarchar(255)
AS
BEGIN
	RETURN UPPER(@s)
END`),

		at("trigger", `CREATE TRIGGER dbmeta_fixture.book_touch
ON dbmeta_fixture.book AFTER UPDATE
AS
BEGIN
	SET NOCOUNT ON
END`),

		// Comments are extended properties here, and there is no COMMENT ON.
		at("table comment", `EXEC sp_addextendedproperty
	@name = N'MS_Description', @value = N'people who write',
	@level0type = N'SCHEMA', @level0name = N'dbmeta_fixture',
	@level1type = N'TABLE', @level1name = N'author'`),
		at("column comment", `EXEC sp_addextendedproperty
	@name = N'MS_Description', @value = N'surrogate key',
	@level0type = N'SCHEMA', @level0name = N'dbmeta_fixture',
	@level1type = N'TABLE', @level1name = N'author',
	@level2type = N'COLUMN', @level2name = N'author_id'`),

		// Rows and statistics over them, so that ColumnStats has something to
		// report. sys.dm_db_stats_properties arrived in release 13.
		at("author rows", `INSERT INTO dbmeta_fixture.author (author_id, name, rating)
	SELECT TOP 200 ROW_NUMBER() OVER (ORDER BY (SELECT NULL)),
		'author ' + CAST(ROW_NUMBER() OVER (ORDER BY (SELECT NULL)) AS nvarchar(8)),
		ROW_NUMBER() OVER (ORDER BY (SELECT NULL)) % 5
	FROM sys.all_objects`),
		from("statistics", v13,
			`CREATE STATISTICS author_name_rating ON dbmeta_fixture.author (name, rating)`),
		at("analyze", `UPDATE STATISTICS dbmeta_fixture.author`),
	},
	Teardown: []Step{
		at("drop trigger", `DROP TRIGGER IF EXISTS dbmeta_fixture.book_touch`),
		at("drop function", `DROP FUNCTION IF EXISTS dbmeta_fixture.shout`),
		at("drop procedure", `DROP PROCEDURE IF EXISTS dbmeta_fixture.addup`),
		at("drop view", `DROP VIEW IF EXISTS dbmeta_fixture.recent`),
		at("drop extras", `DROP TABLE IF EXISTS dbmeta_fixture.extras`),
		at("drop shipment", `DROP TABLE IF EXISTS dbmeta_fixture.shipment`),
		at("drop region", `DROP TABLE IF EXISTS dbmeta_fixture.region`),
		at("drop book", `DROP TABLE IF EXISTS dbmeta_fixture.book`),
		at("drop author", `DROP TABLE IF EXISTS dbmeta_fixture.author`),
		at("drop type", `DROP TYPE IF EXISTS dbmeta_fixture.shortname`),
		at("drop sequence", `DROP SEQUENCE IF EXISTS dbmeta_fixture.counter`),
		at("drop schema", `DROP SCHEMA IF EXISTS dbmeta_fixture`),
	},
}
