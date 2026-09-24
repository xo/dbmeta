// Package fixture holds known good MariaDB and MySQL schemas.
//
// Like every fixture here it is exported API and additive: a later release may
// add an object and will not rename or remove one. See the PostgreSQL fixture
// for the rules, which are the same.
//
// It is versioned on the MariaDB release, because sequences arrived in 10.3
// and check constraints in 10.2, and because MySQL has neither. A step the
// server cannot run is skipped rather than refused.
package fixture

import (
	"errors"

	"github.com/xo/dbmeta"
)

// Releases a step needs.
var (
	v10_2 = dbmeta.V(10, 2)
	v10_3 = dbmeta.V(10, 3)
	v11_5 = dbmeta.V(11, 5)
)

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

func from(name string, min dbmeta.Version, sqlstr string) Step {
	return Step{Name: name, Stmt: dbmeta.Stmt{{{Min: min, SQL: sqlstr}}}}
}

// Everything is a schema holding one of every object the MariaDB queries read.
//
// A schema and a database are the same thing here, so the fixture creates a
// database and everything lives in it. Dropping it removes all of this.
var Everything = Fixture{
	Name:   "everything",
	Schema: "dbmeta_fixture",
	Setup: []Step{
		at("schema", "CREATE SCHEMA dbmeta_fixture"),

		at("author table", "CREATE TABLE dbmeta_fixture.author (\n"+
			"	author_id integer AUTO_INCREMENT PRIMARY KEY,\n"+
			"	name varchar(255) NOT NULL,\n"+
			"	rating integer,\n"+
			"	shade enum('red','green','blue') DEFAULT 'red'\n"+
			") COMMENT 'people who write'"),
		at("author column comment",
			"ALTER TABLE dbmeta_fixture.author MODIFY author_id integer AUTO_INCREMENT COMMENT 'surrogate key'"),

		at("book table", "CREATE TABLE dbmeta_fixture.book (\n"+
			"	book_id integer AUTO_INCREMENT PRIMARY KEY,\n"+
			"	author_id integer NOT NULL,\n"+
			"	title varchar(255) NOT NULL,\n"+
			"	published date,\n"+
			"	CONSTRAINT book_author_fk FOREIGN KEY (author_id) REFERENCES dbmeta_fixture.author(author_id),\n"+
			"	CONSTRAINT book_title_unique UNIQUE (title)\n"+
			")"),
		at("book index", "CREATE INDEX book_published ON dbmeta_fixture.book (published)"),

		at("view", "CREATE VIEW dbmeta_fixture.recent AS\n"+
			"	SELECT book_id, title FROM dbmeta_fixture.book WHERE published IS NOT NULL"),

		at("function", "CREATE FUNCTION dbmeta_fixture.shout(s varchar(255))\n"+
			"	RETURNS varchar(255) DETERMINISTIC RETURN UPPER(s)"),
		at("procedure", "CREATE PROCEDURE dbmeta_fixture.touch()\n"+
			"	BEGIN SELECT 1; END"),

		at("trigger", "CREATE TRIGGER dbmeta_fixture.book_touch BEFORE UPDATE ON dbmeta_fixture.book\n"+
			"	FOR EACH ROW SET NEW.title = NEW.title"),

		at("partitioned table", "CREATE TABLE dbmeta_fixture.sales (\n"+
			"	sold_on date NOT NULL,\n"+
			"	amount integer NOT NULL\n"+
			") PARTITION BY RANGE (YEAR(sold_on)) (\n"+
			"	PARTITION p2026 VALUES LESS THAN (2027),\n"+
			"	PARTITION pmax VALUES LESS THAN MAXVALUE\n"+
			")"),

		// a check constraint is recorded from MariaDB 10.2 and MySQL 8.0.16
		from("check constraint", v10_2,
			"ALTER TABLE dbmeta_fixture.book ADD CONSTRAINT title_not_empty CHECK (title <> '')"),

		// a sequence can be created from MariaDB 10.3, but the view that
		// lists them arrived in 11.5, so creating one below that builds an
		// object no query can read
		from("sequence", v11_5,
			"CREATE SEQUENCE dbmeta_fixture.counter START WITH 10 INCREMENT BY 2"),

		// CREATE AGGREGATE FUNCTION arrived in MariaDB 10.3, and MySQL has no
		// form of it at all. The gate skips MySQL as well, because its release
		// numbers are below 10.3.
		from("aggregate", v10_3,
			"CREATE AGGREGATE FUNCTION dbmeta_fixture.total(x INT) RETURNS INT\n"+
				"BEGIN\n"+
				"	DECLARE sum INT DEFAULT 0;\n"+
				"	DECLARE CONTINUE HANDLER FOR NOT FOUND RETURN sum;\n"+
				"	LOOP\n"+
				"		FETCH GROUP NEXT ROW;\n"+
				"		SET sum = sum + x;\n"+
				"	END LOOP;\n"+
				"END"),

		// A server is global rather than part of a schema, so dropping the
		// schema does not remove it and the teardown drops it by name. Nothing
		// connects to the host named here, and nothing has to: the metadata is
		// written when the server is created.
		at("foreign server", "CREATE SERVER dbmeta_fixture_remote\n"+
			"	FOREIGN DATA WRAPPER mysql\n"+
			"	OPTIONS (HOST '127.0.0.1', DATABASE 'dbmeta_fixture', USER 'nobody', PORT 3306)"),
	},
	Teardown: []Step{
		at("drop server", "DROP SERVER IF EXISTS dbmeta_fixture_remote"),
		at("drop schema", "DROP SCHEMA IF EXISTS dbmeta_fixture"),
	},
}
