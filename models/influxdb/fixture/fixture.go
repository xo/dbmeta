// Package fixture holds a known good InfluxDB 3 database.
//
// A fixture is a set of objects that the metadata queries read, so that a
// test asks a real server for metadata and gets an answer worth checking.
//
// InfluxDB 3 makes a table when a point first names it, and its SQL writes
// nothing. dbimp's influxdb driver takes INSERT followed by line protocol,
// as the influx shell does, and writes it through the server's write API. So
// each step is one such INSERT. There is no DROP in SQL, so the fixture has
// no teardown, and it is safe to run again: a point with the same tags and
// time replaces the one before it.
//
// The measurements are named as the core tables of the other fixtures, so a
// test that reads author and book elsewhere reads the same two here. A tag
// or a field is a column, and every measurement has a time column.
package fixture

import "github.com/xo/dbmeta"

// Step is one statement of a fixture.
type Step struct {
	Name string
	Stmt dbmeta.Stmt
}

// Result is what a step resolved to.
type Result struct {
	Name  string
	Query string
}

// Fixture is a set of objects, with the statements that build them.
type Fixture struct {
	Name   string
	Schema string
	Setup  []Step
}

// ResolveSetup returns the statements that build the fixture.
func (f Fixture) ResolveSetup(versions dbmeta.VersionSet) ([]Result, error) {
	out := make([]Result, 0, len(f.Setup))
	for _, step := range f.Setup {
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

// Everything holds one of every object the InfluxDB 3 queries read that a
// statement can make: a measurement with tags and fields of each type.
// Functions, settings and the schemas are the server's own.
var Everything = Fixture{
	Name:   "everything",
	Schema: "iox",
	Setup: []Step{
		at("author", `INSERT author,author_id=1 name="author 1",rating=3i,shade="red" 1767225600000000000`),
		at("book", `INSERT book,book_id=1,author_id=1 title="a book",published="2026-01-01" 1767225600000000000`),
		at("region", `INSERT region,country=nz,area=north population=1i 1767225600000000000`),
		at("shipment", `INSERT shipment,shipment_id=1,country=nz,area=north amount=10i,weight=1.5,fragile=true 1767225600000000000`),
	},
}
