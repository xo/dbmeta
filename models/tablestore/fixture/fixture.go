// Package fixture holds a known good set of Tablestore tables, for the SQL
// feature of the service.
//
// Like every fixture here it is exported API and additive: a later release can
// add an object and will not rename or remove one. See the PostgreSQL fixture
// for the rules, which are the same.
//
// It is not versioned. Tablestore has no release that SQL reads. The Resolve
// methods keep the shape the other fixtures have so that a caller can treat them
// alike.
//
// # Two layers
//
// SQL cannot create a Tablestore table. CREATE TABLE in the SQL feature makes a
// mapping over a table that already exists, and the table is made with the
// Tablestore API. The package is pure Go with no dependency, so it cannot call
// the API. It holds the tables as data, in Base, and the test module makes them
// with the Go SDK. Setup then maps each one with SQL, and Teardown drops the
// mappings. The caller deletes the tables, after the mappings.
//
// # Where it builds
//
// The tables are in the instance that the connection names. The instance is the
// only namespace, so Schema is empty. Each name starts with dbmeta_fx_, so that
// no other table of the instance collides with it.
//
// # What it cannot build
//
// Tablestore SQL has no view, no sequence, no trigger, no user defined type and
// no role, and a column has no default and no comment. A mapped column is
// nullable unless it is in the key. The fixture builds a secondary index and a
// search index, which SHOW INDEX lists and SHOW FULL TABLES does not, so a test
// can assert that an index is not a table.
package fixture

import (
	"strings"

	"github.com/xo/dbmeta"
)

// Type is the type of a column in the Tablestore API.
type Type string

// The column types the fixture uses.
const (
	String  Type = "string"
	Integer Type = "integer"
)

// Column is a column of a base table. A key column holds a value in every row.
type Column struct {
	Name string
	Type Type
}

// Index is a secondary index. The index has a key of its own, which is the first
// column, and Tablestore adds the key of the table.
type Index struct {
	Name    string
	Columns []string
}

// SearchField is a column that a search index covers, as a keyword.
type SearchField struct {
	Name string
}

// SearchIndex is a search index.
type SearchIndex struct {
	Name   string
	Fields []SearchField
}

// BaseTable is a Tablestore table, which the API makes and SQL maps.
type BaseTable struct {
	Name    string
	Keys    []Column
	Columns []Column

	// Indexes are the secondary indexes of the table.
	Indexes []Index
	// Searches are the search indexes of the table.
	Searches []SearchIndex
}

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

// Fixture is a set of tables, with the statements that map them and drop the
// mappings.
type Fixture struct {
	Name string
	// Schema is empty: the instance is the only namespace.
	Schema string

	// Base holds the tables that the API makes before Setup.
	Base []BaseTable

	Setup    []Step
	Teardown []Step
}

// ResolveSetup returns the statements that map the tables. Run them after the
// API made the tables in Base.
func (f Fixture) ResolveSetup(versions dbmeta.VersionSet) ([]Result, error) {
	return resolve(f.Setup, versions)
}

// ResolveTeardown returns the statements that drop the mappings. Delete the
// tables with the API after them.
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

// Prefix starts the name of every table of the fixture.
const Prefix = "dbmeta_fx_"

// Names returns the names of the tables of the fixture, in the order of Base.
func (f Fixture) Names() []string {
	out := make([]string, len(f.Base))
	for i, t := range f.Base {
		out[i] = t.Name
	}
	return out
}

// Everything is a set of four tables, a secondary index and a search index.
//
// The names follow the other fixtures where they can: author, book and region,
// each with the prefix.
var Everything = Fixture{
	Name: "everything",
	Base: []BaseTable{
		{
			Name:    Prefix + "author",
			Keys:    []Column{{"author_id", Integer}},
			Columns: []Column{{"name", String}, {"rating", Integer}},
			Indexes: []Index{{Name: Prefix + "author_name", Columns: []string{"name"}}},
		},
		{
			Name:     Prefix + "book",
			Keys:     []Column{{"book_id", Integer}},
			Columns:  []Column{{"author_id", Integer}, {"title", String}, {"published", String}},
			Searches: []SearchIndex{{Name: Prefix + "book_title", Fields: []SearchField{{"title"}}}},
		},
		// A table with a composite key.
		{
			Name:    Prefix + "region",
			Keys:    []Column{{"country", String}, {"area", String}},
			Columns: []Column{{"population", Integer}},
		},
		// A table that refers to region. Tablestore has no foreign key, so the
		// reference is a pair of columns and nothing more.
		{
			Name:    Prefix + "shipment",
			Keys:    []Column{{"shipment_id", Integer}},
			Columns: []Column{{"country", String}, {"area", String}, {"amount", Integer}},
		},
	},
	Setup: []Step{
		at("map author", "CREATE TABLE "+Prefix+"author (author_id BIGINT, name MEDIUMTEXT, rating BIGINT, PRIMARY KEY(author_id))"),
		at("map book", "CREATE TABLE "+Prefix+"book (book_id BIGINT, author_id BIGINT, title MEDIUMTEXT, published MEDIUMTEXT, PRIMARY KEY(book_id))"),
		at("map region", "CREATE TABLE "+Prefix+"region (country VARCHAR(1024), area VARCHAR(1024), population BIGINT, PRIMARY KEY(country, area))"),
		at("map shipment", "CREATE TABLE "+Prefix+"shipment (shipment_id BIGINT, country MEDIUMTEXT, area MEDIUMTEXT, amount BIGINT, PRIMARY KEY(shipment_id))"),
	},
	Teardown: []Step{
		at("unmap shipment", "DROP MAPPING TABLE IF EXISTS "+Prefix+"shipment"),
		at("unmap region", "DROP MAPPING TABLE IF EXISTS "+Prefix+"region"),
		at("unmap book", "DROP MAPPING TABLE IF EXISTS "+Prefix+"book"),
		at("unmap author", "DROP MAPPING TABLE IF EXISTS "+Prefix+"author"),
	},
}

// Is reports whether name is a table of the fixture.
func Is(name string) bool {
	return strings.HasPrefix(name, Prefix)
}
