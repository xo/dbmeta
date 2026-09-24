// Package fixture holds a known good SQLite schema.
//
// Like every fixture here it is exported API and additive: a later release may
// add an object and will not rename or remove one. See the PostgreSQL fixture
// for the rules, which are the same.
//
// It is not versioned. Every other fixture is, because a server is upgraded
// separately from the code that reads it and a step can be too new for the
// server it runs against. SQLite is a library, so the version is whichever one
// the caller linked, and there is nothing to skip. The Resolve methods keep
// the shape the other fixtures have so that a caller can treat them alike.
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
		sqlstr, err := step.Stmt.SQL(versions)
		if err != nil {
			return nil, err
		}
		out = append(out, Result{Name: step.Name, SQL: sqlstr})
	}
	return out, nil
}

func at(name, sqlstr string) Step {
	return Step{Name: name, Stmt: dbmeta.Always(sqlstr)}
}

// Everything is a schema holding one of every object the SQLite queries read.
//
// SQLite has one schema per file and it is called main, so the objects are not
// namespaced and the teardown drops each one by name. A caller running this
// against a file it cares about will lose those objects, so run it against a
// temporary file or against :memory:.
//
// The names match the other fixtures, so a test that reads author and book on
// PostgreSQL reads the same two here.
var Everything = Fixture{
	Name:   "everything",
	Schema: "main",
	Setup: []Step{
		at("author table", "CREATE TABLE author (\n"+
			"	author_id INTEGER PRIMARY KEY AUTOINCREMENT,\n"+
			"	name TEXT NOT NULL,\n"+
			"	rating INTEGER,\n"+
			"	shade TEXT DEFAULT 'red' CHECK (shade IN ('red', 'green', 'blue'))\n"+
			")"),

		at("book table", "CREATE TABLE book (\n"+
			"	book_id INTEGER PRIMARY KEY,\n"+
			"	author_id INTEGER NOT NULL REFERENCES author(author_id) ON DELETE CASCADE,\n"+
			"	title TEXT NOT NULL UNIQUE,\n"+
			"	published DATE,\n"+
			// a generated column, so that the query for one has something to
			// find. STORED rather than VIRTUAL, so that both forms are
			// covered when the sales table below adds the other.
			"	title_length INTEGER GENERATED ALWAYS AS (LENGTH(title)) STORED,\n"+
			"	CONSTRAINT title_not_empty CHECK (title <> '')\n"+
			")"),
		at("book index", "CREATE INDEX book_published ON book (published)"),
		at("book descending index", "CREATE INDEX book_title_desc ON book (title DESC)"),

		at("view", "CREATE VIEW recent AS\n"+
			"	SELECT book_id, title FROM book WHERE published IS NOT NULL"),

		at("trigger", "CREATE TRIGGER book_touch AFTER UPDATE ON book\n"+
			"BEGIN\n"+
			"	UPDATE author SET rating = rating WHERE author_id = NEW.author_id;\n"+
			"END"),

		// a second table with a virtual generated column and a composite
		// primary key, which the constraint query reports as one row
		at("sales table", "CREATE TABLE sales (\n"+
			"	sold_on DATE NOT NULL,\n"+
			"	region TEXT NOT NULL,\n"+
			"	amount INTEGER NOT NULL,\n"+
			"	doubled INTEGER GENERATED ALWAYS AS (amount * 2) VIRTUAL,\n"+
			"	PRIMARY KEY (sold_on, region)\n"+
			")"),
	},
	Teardown: []Step{
		at("drop trigger", "DROP TRIGGER IF EXISTS book_touch"),
		at("drop view", "DROP VIEW IF EXISTS recent"),
		at("drop sales", "DROP TABLE IF EXISTS sales"),
		at("drop book", "DROP TABLE IF EXISTS book"),
		at("drop author", "DROP TABLE IF EXISTS author"),
	},
}
