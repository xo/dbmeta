// Package fixture builds the indices the OpenSearch queries read.
//
// It creates one of every object those queries report, so a test has rows
// worth checking rather than an empty catalog. Hard rule 9 requires it, and
// D53 asks for the core objects of every fixture.
//
// OpenSearch has no DDL in SQL. Its SQL reads only, so every step is a request
// to the HTTP API, and a test sends it to the server that the DSN names, as the
// user of the DSN. The package imports nothing.
//
// # The names
//
// Every index starts with dbmeta. The role dbmeta_role of the entry can read
// the indices whose names start with dbmeta and no others, so an ordinary
// user can read the fixture and nothing else. The one exception is
// secret_idx, which the ordinary user cannot read, and which a parity test
// reads as an administrator to show what the user sees of it.
//
// # The core objects
//
// The four core tables of D53 are the indices dbmeta_author, dbmeta_book,
// dbmeta_region and dbmeta_shipment, and the core view is the alias
// dbmeta_recent over dbmeta_book. A field of an index is a column, and the
// mapping has no key, so the key columns are plain numbers. SQL cannot tell the
// alias from an index, so it is a table on 2.19.6 and absent on 3.9.0.
//
// # More than the core
//
// dbmeta_author has a multi-field, name.raw, and a comment in the _meta of its
// mapping. dbmeta_shipment has an object, dims, and a nested field, tags.
// dbmeta_types has one field of each of several types that SQL reads
// differently from the mapping. dbmeta_empty has a mapping with no field.
//
// # What OpenSearch cannot build
//
// An index has no primary key, no foreign key, no NOT NULL, no default and no
// constraint, and SQL has no index, trigger, sequence or comment of a field.
// Every query that reads one of those is unanswered, and docs/COVERAGE.md says
// why.
package fixture

// Request is one call to the HTTP API.
type Request struct {
	// Method is the HTTP method.
	Method string
	// Path is the path and the query of the URL.
	Path string
	// Body is the JSON body, or empty.
	Body string
}

// Step is one request, with the name that a failure reports.
type Step struct {
	Name    string
	Request Request
}

// Fixture is a set of indices and the steps that build and remove them.
type Fixture struct {
	Name string
	// Schema is empty, because OpenSearch has no schema.
	Schema string
	// Prefix is what every index of the fixture starts with.
	Prefix   string
	Setup    []Step
	Teardown []Step
}

// call is a step.
func call(name, method, path, body string) Step {
	return Step{Name: name, Request: Request{Method: method, Path: path, Body: body}}
}

// create makes an index with a mapping.
func create(index, mapping string) Step {
	return call(index, "PUT", "/"+index, `{"mappings":`+mapping+`}`)
}

// drop deletes an index by name. A missing index is not an error to a test
// that ignores it.
func drop(index string) Step {
	return call(index, "DELETE", "/"+index, "")
}

// Everything holds the four core indices, the alias and the extra indices.
var Everything = Fixture{
	Name:   "everything",
	Schema: "",
	Prefix: "dbmeta",
	Setup: []Step{
		create("dbmeta_author", `{"_meta":{"comment":"the writers"},"properties":{`+
			`"author_id":{"type":"long"},`+
			`"name":{"type":"text","fields":{"raw":{"type":"keyword"}}},`+
			`"rating":{"type":"integer"},`+
			`"shade":{"type":"keyword"}}}`),
		create("dbmeta_book", `{"properties":{`+
			`"book_id":{"type":"long"},`+
			`"author_id":{"type":"long"},`+
			`"title":{"type":"keyword"},`+
			`"published":{"type":"date"}}}`),
		create("dbmeta_region", `{"properties":{`+
			`"country":{"type":"keyword"},`+
			`"area":{"type":"keyword"},`+
			`"population":{"type":"long"}}}`),
		create("dbmeta_shipment", `{"properties":{`+
			`"shipment_id":{"type":"long"},`+
			`"country":{"type":"keyword"},`+
			`"area":{"type":"keyword"},`+
			`"amount":{"type":"long"},`+
			`"weight":{"type":"double"},`+
			`"fragile":{"type":"boolean"},`+
			`"dims":{"properties":{"w":{"type":"integer"},"h":{"type":"integer"}}},`+
			`"tags":{"type":"nested","properties":{"k":{"type":"keyword"}}}}}`),
		call("dbmeta_recent", "POST", "/_aliases",
			`{"actions":[{"add":{"index":"dbmeta_book","alias":"dbmeta_recent",`+
				`"filter":{"exists":{"field":"published"}}}}]}`),
		create("dbmeta_types", `{"properties":{`+
			`"ip":{"type":"ip"},`+
			`"scaled":{"type":"scaled_float","scaling_factor":100},`+
			`"half":{"type":"half_float"},`+
			`"nanos":{"type":"date_nanos"},`+
			`"point":{"type":"geo_point"},`+
			`"blob":{"type":"binary"},`+
			`"span":{"type":"integer_range"},`+
			`"tiny":{"type":"byte"},`+
			`"small":{"type":"short"},`+
			`"real":{"type":"float"},`+
			`"flag":{"type":"boolean"}}}`),
		create("dbmeta_empty", `{"properties":{}}`),
		create("secret_idx", `{"properties":{"code":{"type":"keyword"}}}`),
		call("documents", "POST", "/_bulk?refresh=true",
			`{"index":{"_index":"dbmeta_author","_id":"1"}}
{"author_id":1,"name":"Ursula","rating":5,"shade":"red"}
{"index":{"_index":"dbmeta_author","_id":"2"}}
{"author_id":2,"name":"Terry","shade":"blue"}
{"index":{"_index":"dbmeta_book","_id":"1"}}
{"book_id":1,"author_id":1,"title":"A Wizard of Earthsea","published":"1968-01-01"}
{"index":{"_index":"dbmeta_region","_id":"1"}}
{"country":"nz","area":"north","population":1}
{"index":{"_index":"dbmeta_shipment","_id":"1"}}
{"shipment_id":1,"country":"nz","area":"north","amount":10,"weight":1.5,"fragile":true,"dims":{"w":2,"h":3},"tags":[{"k":"a"}]}
{"index":{"_index":"secret_idx","_id":"1"}}
{"code":"x"}
`),
	},
	// The teardown removes what the setup made, in the reverse order. Deleting
	// an index removes its alias.
	Teardown: []Step{
		drop("secret_idx"),
		drop("dbmeta_empty"),
		drop("dbmeta_types"),
		drop("dbmeta_shipment"),
		drop("dbmeta_region"),
		drop("dbmeta_book"),
		drop("dbmeta_author"),
	},
}
