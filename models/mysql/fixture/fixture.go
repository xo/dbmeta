// Package fixture holds known good MariaDB and MySQL schemas.
//
// Like every fixture here it is exported API and additive: a later release may
// add an object and will not rename or remove one. See the PostgreSQL fixture
// for the rules, which are the same.
//
// It is versioned per product, because MariaDB and MySQL share this dialect
// and their release numbers have no relation to each other. A step that needs
// MariaDB 10.3 gates on the "mariadb" key and never runs on MySQL, whatever
// number MySQL reports. A step the server cannot run is skipped rather than
// refused. See D44.
package fixture

import (
	"errors"

	"github.com/xo/dbmeta"
)

// Version keys, one per product. They repeat the ones the model declares,
// because a fixture must not import the model it builds a schema for.
const (
	mariaKey = "mariadb"
	mysqlKey = "mysql"
)

// What a step needs, per product.
var (
	// A check constraint is recorded from MariaDB 10.2 and from MySQL 8.0.16.
	// MySQL below that accepts the syntax and ignores it, which is worse than
	// refusing, so the step is skipped there too.
	maria10_2 = dbmeta.Gate{Key: mariaKey, Min: dbmeta.V(10, 2)}
	mysql8_0  = dbmeta.Gate{Key: mysqlKey, Min: dbmeta.V(8, 0, 16)}
	// A sequence and an aggregate are MariaDB only at every release.
	maria10_3 = dbmeta.Gate{Key: mariaKey, Min: dbmeta.V(10, 3)}
	maria11_5 = dbmeta.Gate{Key: mariaKey, Min: dbmeta.V(11, 5)}
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
		case errors.Is(err, dbmeta.ErrNotSupported):
			out = append(out, Result{
				Name:    step.Name,
				Skipped: true,
				Reason:  "this product has no such object at any release",
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

// when returns a step that runs on a server meeting any of the gates. A step
// with no gate met is skipped.
func when(name, sqlstr string, gates ...dbmeta.Gate) Step {
	c := make(dbmeta.Choice, len(gates))
	for i, g := range gates {
		c[i] = dbmeta.Fragment{Min: g.Min, Key: g.Key, SQL: sqlstr}
	}
	return Step{Name: name, Stmt: dbmeta.Stmt{c}}
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

		when("check constraint",
			"ALTER TABLE dbmeta_fixture.book ADD CONSTRAINT title_not_empty CHECK (title <> '')",
			maria10_2, mysql8_0),

		// a sequence can be created from MariaDB 10.3, but the view that
		// lists them arrived in 11.5, so creating one below that builds an
		// object no query can read. MySQL has no sequences at all.
		when("sequence",
			"CREATE SEQUENCE dbmeta_fixture.counter START WITH 10 INCREMENT BY 2",
			maria11_5),

		// CREATE AGGREGATE FUNCTION arrived in MariaDB 10.3. MySQL has no form
		// of it at any release, which the key says and the number cannot.
		when("aggregate",
			"CREATE AGGREGATE FUNCTION dbmeta_fixture.total(x INT) RETURNS INT\n"+
				"BEGIN\n"+
				"	DECLARE sum INT DEFAULT 0;\n"+
				"	DECLARE CONTINUE HANDLER FOR NOT FOUND RETURN sum;\n"+
				"	LOOP\n"+
				"		FETCH GROUP NEXT ROW;\n"+
				"		SET sum = sum + x;\n"+
				"	END LOOP;\n"+
				"END",
			maria10_3),

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
