// Package fixture builds the collections the Apache Solr queries read.
//
// It creates the core objects of D53, so that a test has rows worth checking
// and not an empty catalog. Hard rule 9 requires it.
//
// Solr has no DDL in its SQL. A collection, a field and an alias come from
// the HTTP API, and a document comes from the update handler. So each step is
// a request that a test sends to the server as the administrator.
//
// # One configuration set for each collection
//
// A collection made from _default shares one managed schema with every other
// collection made from it, so a field added to one appears in all of them.
// Each collection here has a configuration set of its own, a copy of
// _default, so that its fields are its own.
//
// # Four collections and an alias
//
// The fixture makes author, book, region and shipment, which are the four core
// tables, and the alias recent of the collection book, which is the core view.
// Every collection has the fields id, _version_, _root_, _nest_path_ and
// _text_ of _default, and a document needs an id, so the id of each document is
// the key of its row.
//
// # Safe to run again
//
// A step that names a collection is skipped when the collection exists. The
// alias step runs every time, because CREATEALIAS replaces an alias.
//
// # What Solr cannot build
//
// A collection has no foreign key, no default, no index that SQL lists, no
// trigger, no sequence, no comment and no type of its own. The core objects
// that need one are not here, and docs/COVERAGE.md says why.
package fixture

import "strings"

// Step is one request to the HTTP API of Solr.
type Step struct {
	// Name says what the step does.
	Name string
	// Collection is the collection the step makes or fills. A test skips the
	// step when the collection already exists. It is empty for a step that
	// runs every time.
	Collection string
	// Method and Path are the request. Path starts with /solr.
	Method string
	Path   string
	// Body is the JSON body, empty for none.
	Body string
	// Exists is text in the error of the server that says the step was done
	// before. A test treats such an error as success.
	Exists string
}

// Fixture is a set of collections, with the requests that make them.
type Fixture struct {
	// Name is the name of the fixture.
	Name string
	// Catalog and Schema are the names that the model reports for every
	// collection. See D179.
	Catalog string
	Schema  string
	// Setup makes the collections, in order.
	Setup []Step
	// Teardown removes what Setup made, in order.
	Teardown []Step
}

// collection returns the steps that make one collection with its fields and
// its documents.
func collection(name, fields, docs string) []Step {
	set := "dbmeta_" + name
	return []Step{
		{
			Name: "make the configuration set of " + name, Collection: name,
			Method: "POST", Path: "/solr/admin/configs?action=CREATE&name=" + set + "&baseConfigSet=_default",
			Exists: "already exists",
		},
		{
			Name: "make the collection " + name, Collection: name,
			Method: "POST", Path: "/solr/admin/collections?action=CREATE&name=" + name +
				"&numShards=1&replicationFactor=1&collection.configName=" + set + "&wt=json",
		},
		{
			Name: "add the fields of " + name, Collection: name,
			Method: "POST", Path: "/solr/" + name + "/schema", Body: `{"add-field": [` + fields + `]}`,
		},
		{
			Name: "add the documents of " + name, Collection: name,
			Method: "POST", Path: "/solr/" + name + "/update?commit=true", Body: docs,
		},
	}
}

// field is the JSON of one single-valued field with doc values, so that SQL
// can read it without a limit.
func field(name, kind string) string {
	return `{"name": "` + name + `", "type": "` + kind + `", "multiValued": false, "docValues": true, "stored": true, "indexed": true}`
}

// fields joins the JSON of several fields.
func fields(list ...string) string { return strings.Join(list, ", ") }

// steps joins several lists of steps.
func steps(lists ...[]Step) []Step {
	var out []Step
	for _, l := range lists {
		out = append(out, l...)
	}
	return out
}

// Everything holds the four core collections of D53 and the alias recent. The
// names of the configuration sets are dbmeta_ and the name of the collection.
var Everything = Fixture{
	Name:    "everything",
	Catalog: "solr",
	Schema:  "solr",
	Setup: steps(
		collection("author",
			fields(field("author_id", "plong"), field("name", "string"), field("rating", "plong"), field("shade", "string")),
			`[{"id": "1", "author_id": 1, "name": "Ursula", "rating": 5, "shade": "red"},`+
				` {"id": "2", "author_id": 2, "name": "Terry", "shade": "blue"}]`),
		collection("book",
			fields(field("book_id", "plong"), field("author_id", "plong"), field("title", "string"), field("published", "pdate")),
			`[{"id": "1", "book_id": 1, "author_id": 1, "title": "A Wizard of Earthsea", "published": "1968-01-01T00:00:00Z"}]`),
		collection("region",
			fields(field("country", "string"), field("area", "string"), field("population", "plong")),
			`[{"id": "nz-north", "country": "nz", "area": "north", "population": 1}]`),
		collection("shipment",
			fields(field("shipment_id", "plong"), field("country", "string"), field("area", "string"),
				field("amount", "plong"), field("weight", "pdouble"), field("fragile", "boolean")),
			`[{"id": "1", "shipment_id": 1, "country": "nz", "area": "north", "amount": 10, "weight": 1.5, "fragile": true}]`),
		[]Step{{
			Name:   "make the alias recent",
			Method: "POST", Path: "/solr/admin/collections?action=CREATEALIAS&name=recent&collections=book",
		}},
	),
	Teardown: []Step{
		{Name: "remove the alias recent", Method: "POST", Path: "/solr/admin/collections?action=DELETEALIAS&name=recent"},
		{Name: "remove the collection shipment", Method: "POST", Path: "/solr/admin/collections?action=DELETE&name=shipment"},
		{Name: "remove the collection region", Method: "POST", Path: "/solr/admin/collections?action=DELETE&name=region"},
		{Name: "remove the collection book", Method: "POST", Path: "/solr/admin/collections?action=DELETE&name=book"},
		{Name: "remove the collection author", Method: "POST", Path: "/solr/admin/collections?action=DELETE&name=author"},
	},
}
