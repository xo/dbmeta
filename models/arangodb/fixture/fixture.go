// Package fixture builds the collections the ArangoDB queries read.
//
// It creates one of every object those queries report, so a test has rows
// worth checking rather than an empty catalog. Hard rule 9 requires it, and
// D53 asks for the core objects of every fixture.
//
// It builds in the database dbmeta, which the dbrun setup makes, because the
// ordinary user can reach no other database. ArangoDB has no schema, so the
// core collections sit in the database itself.
//
// # Two kinds of step
//
// AQL has no DDL. dbimp's driver takes CREATE and DROP of a collection and of
// an index, so a step of that kind is a statement the driver runs. Everything
// else, such as a schema rule, a function, a view, a graph and an analyzer,
// only the HTTP API makes, so a step of that kind is a request. A test sends
// it to the server and the database that the DSN names.
//
// # What the queries cannot read
//
// The fixture makes the core view recent, an index, an edge collection, a
// graph and an analyzer, which no query reports. They are there so that a
// test can show that the model leaves each one out, rather than missing it by
// chance: AQL lists no view, no index and no analyzer that fits a kind, and
// COLLECTIONS() does not say that wrote holds edges.
package fixture

import (
	"errors"

	"github.com/xo/dbmeta"
)

// Request is one call to the HTTP API. Path is under /_db/<database>.
type Request struct {
	Method string
	Path   string
	Body   string
}

// Step is one statement the driver runs, or one request.
type Step struct {
	Name    string
	Stmt    dbmeta.Stmt
	Request *Request
}

// Result is what a step resolved to for one server.
type Result struct {
	Name    string
	Query   string
	Request *Request
	Skipped bool
	Reason  string
}

// Fixture is a set of collections and the steps that build and remove them.
type Fixture struct {
	Name     string
	Catalog  string
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
		if step.Request != nil {
			out = append(out, Result{Name: step.Name, Request: step.Request})
			continue
		}
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

// stmt is a statement that every release runs.
func stmt(name, query string) Step {
	return Step{Name: name, Stmt: dbmeta.Stmt{{{Query: query}}}}
}

// call is a request to the HTTP API.
func call(name, method, path, body string) Step {
	return Step{Name: name, Request: &Request{Method: method, Path: path, Body: body}}
}

// schema is the request that sets the JSON schema rule of a collection.
func schema(collection, rule, level string) Step {
	return call(collection+" schema", "PUT", "/_api/collection/"+collection+"/properties",
		`{"schema": {"rule": `+rule+`, "level": "`+level+`", "message": "`+collection+` breaks its rule"}}`)
}

// Everything is the collections, rules and functions the ArangoDB queries
// read, and the objects they leave out.
var Everything = Fixture{
	Name:    "everything",
	Catalog: "",
	Schema:  "dbmeta",
	Setup: []Step{
		// The core tables of D53, as collections, and their columns as the
		// properties of a rule. A property the rule requires, whose type
		// takes no null, is NOT NULL.
		stmt("author", "CREATE COLLECTION IF NOT EXISTS author"),
		stmt("book", "CREATE COLLECTION IF NOT EXISTS book"),
		stmt("region", "CREATE COLLECTION IF NOT EXISTS region"),
		stmt("shipment", "CREATE COLLECTION IF NOT EXISTS shipment"),
		schema("author", `{"type": "object", "properties": {"author_id": {"type": "integer"},`+
			` "name": {"type": "string"}, "rating": {"type": ["integer", "null"]},`+
			` "shade": {"type": "string"}}, "required": ["author_id", "name"]}`, "strict"),
		schema("book", `{"type": "object", "properties": {"book_id": {"type": "integer"},`+
			` "author_id": {"type": "integer"}, "title": {"type": "string"},`+
			` "published": {"type": "string", "format": "date"}},`+
			` "required": ["book_id", "author_id", "title"]}`, "strict"),
		schema("region", `{"type": "object", "properties": {"country": {"type": "string"},`+
			` "area": {"type": "string"}}, "required": ["country", "area"]}`, "strict"),
		schema("shipment", `{"type": "object", "properties": {"shipment_id": {"type": "integer"},`+
			` "country": {"type": "string"}, "area": {"type": "string"}, "amount": {"type": "number"}},`+
			` "required": ["shipment_id", "country", "area", "amount"]}`, "strict"),

		// A collection whose rule has each shape the columns query reads
		// apart: _key, which is the primary key, a property with no type, a
		// type that is a list, and a property the rule requires that can
		// still be null. Its level, new, checks a document only when it is
		// inserted.
		stmt("note", "CREATE COLLECTION IF NOT EXISTS note"),
		schema("note", `{"type": "object", "properties": {"_key": {"type": "string"},`+
			` "body": {}, "isbn": {"type": ["string", "null"]}, "tags": {"type": "array"}},`+
			` "required": ["isbn"], "additionalProperties": true}`, "new"),

		// A collection with no rule, which has no column and no constraint.
		stmt("loose", "CREATE COLLECTION IF NOT EXISTS loose"),

		// The core index book_published, which no query reads.
		stmt("book_published", "CREATE INDEX IF NOT EXISTS book_published ON book (published)"),

		// An edge collection and a graph over it. Tables reports wrote as a
		// collection, because COLLECTIONS() does not say it holds edges.
		stmt("wrote", "CREATE COLLECTION IF NOT EXISTS wrote EDGE"),
		call("authorship", "POST", "/_api/gharial",
			`{"name": "authorship", "edgeDefinitions": [{"collection": "wrote", "from": ["author"], "to": ["book"]}]}`),

		// The core view recent, as an ArangoSearch view on book, and an
		// analyzer for it. Neither is a collection.
		call("analyzer", "POST", "/_api/analyzer",
			`{"name": "dbmeta_text", "type": "text", "properties": {"locale": "en", "case": "lower", "stemming": true},`+
				` "features": ["frequency", "position", "norm"]}`),
		call("recent", "POST", "/_api/view",
			`{"name": "recent", "type": "arangosearch", "links": {"book": {"fields": {"title": {"analyzers": ["dbmeta_text"]}}}}}`),

		// A deterministic function with two parameters, and one that is not
		// deterministic.
		call("full_title", "POST", "/_api/aqlfunction",
			`{"name": "dbmeta::full_title", "code": "function (title, subtitle) { return title + ': ' + subtitle; }",`+
				` "isDeterministic": true}`),
		call("roll", "POST", "/_api/aqlfunction",
			`{"name": "dbmeta::roll", "code": "function () { return Math.random(); }", "isDeterministic": false}`),

		// Documents, so that the collections hold something.
		stmt("author rows", "UPSERT {_key: '1'} INSERT {_key: '1', author_id: 1, name: 'Ursula', rating: 5}"+
			" UPDATE {} IN author"),
		stmt("book rows", "UPSERT {_key: '1'} INSERT {_key: '1', book_id: 1, author_id: 1,"+
			" title: 'A Wizard of Earthsea', published: '1968-01-01'} UPDATE {} IN book"),
	},
	Teardown: []Step{
		// A view and a graph go before the collections they name, and an
		// analyzer after the view that uses it. Each is best effort, because
		// the teardown runs before a setup too.
		call("recent", "DELETE", "/_api/view/recent", ""),
		call("authorship", "DELETE", "/_api/gharial/authorship", ""),
		call("analyzer", "DELETE", "/_api/analyzer/dbmeta_text?force=true", ""),
		call("functions", "DELETE", "/_api/aqlfunction/dbmeta?group=true", ""),
		stmt("wrote", "DROP COLLECTION IF EXISTS wrote"),
		stmt("loose", "DROP COLLECTION IF EXISTS loose"),
		stmt("note", "DROP COLLECTION IF EXISTS note"),
		stmt("shipment", "DROP COLLECTION IF EXISTS shipment"),
		stmt("region", "DROP COLLECTION IF EXISTS region"),
		stmt("book", "DROP COLLECTION IF EXISTS book"),
		stmt("author", "DROP COLLECTION IF EXISTS author"),
	},
}
