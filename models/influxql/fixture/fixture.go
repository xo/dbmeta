// Package fixture holds a known good InfluxQL database.
//
// A fixture is a set of objects that the metadata queries read, so that a
// test asks a real server for metadata and gets an answer worth checking.
//
// InfluxDB makes a measurement when a point first names it, and InfluxQL has
// no statement that writes a point. dbimp's influxdb driver takes INSERT
// followed by line protocol, as the influx shell does, and writes it through
// the server's write API to the database of the connection. So each step is
// one such INSERT. A point with the same tags and time replaces the one
// before it, so the fixture is safe to run again and needs no teardown.
//
// The measurements are named as the core tables of the other fixtures, so a
// test that reads author and book elsewhere reads the same two here. A tag
// or a field is a column, and every measurement has a time column.
//
// The fixture makes no user, no grant, no database and no retention
// policy. InfluxDB 2 and 3 refuse CREATE USER, GRANT and CREATE DATABASE,
// and no statement names the release, so a step for InfluxDB 1 alone
// cannot be skipped elsewhere. The dbrun entry makes the database dbmeta and
// the user dbmeta_user, with READ on dbmeta on InfluxDB 1, and that is what
// Roles and Privileges read.
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

// Everything holds one of every object the InfluxQL queries read that a
// statement can make: measurements with tags and fields of each type. The
// database, the user and the settings are the server's own.
var Everything = Fixture{
	Name:   "everything",
	Schema: "dbmeta",
	Setup: []Step{
		at("author", `INSERT author,author_id=1 name="author 1",rating=3i,shade="red" 1767225600000000000`),
		at("book", `INSERT book,book_id=1,author_id=1 title="a book",published="2026-01-01" 1767225600000000000`),
		at("region", `INSERT region,country=nz,area=north population=1i 1767225600000000000`),
		at("shipment", `INSERT shipment,shipment_id=1,country=nz,area=north amount=10i,weight=1.5,fragile=true 1767225600000000000`),
	},
}
