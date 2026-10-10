// Package fixture holds a known good Azure Cosmos DB database, for the API for
// NoSQL.
//
// Like every fixture here it is exported API and additive: a later release can
// add an object and will not rename or remove one.
//
// # It is requests and not statements
//
// The SQL of Cosmos DB reads documents and writes nothing, and the driver of
// dbimp reads only. A container, a stored procedure, a user and a permission
// are resources of the REST API, so a step here is one request. A step holds the
// method, the path below the database, and the JSON body. The caller signs and
// sends it, because that needs the key of the account, and this package holds
// no secret and no code that sends.
//
// A body can name the link of a container that an earlier step made, because a
// permission names its resource by the link that the server gave it. The text
// $(container NAME) stands for the link of the container NAME. The caller
// replaces it with the _self member of the answer that made the container.
//
// # Where it builds
//
// The account has the database dbmeta, with 400 request units a second that its
// containers share. The fixture builds its containers there and its Schema is
// that name. A connection that names the container book in its path reads the
// stored procedure, the trigger and the function of that container too.
//
// # What it cannot build
//
// The account has no vector search and no change feed of all versions, so the
// server refused a vector embedding policy, a vector index and a change feed
// retention. The fixture builds none, and those columns of a container are
// verified to run and read NULL. It builds no conflict resolution policy,
// because only an account with several write regions takes one. It builds no
// analytical store, which the account does not have. A container with a
// hierarchical partition key is refused at the version of the API that the
// driver names, so the partition key has one path.
package fixture

// Database is the database that the fixture builds in. It is the dbsetup
// database of the account.
const Database = "dbmeta"

// Step is one request of a fixture.
type Step struct {
	Name string
	// Method is the HTTP verb.
	Method string
	// Path is the path of the resource, below the database.
	Path string
	// Body is the JSON that the request sends, and empty for none.
	Body string
}

// Fixture is a database and the requests that build it and remove it.
type Fixture struct {
	Name     string
	Schema   string
	Setup    []Step
	Teardown []Step
}

// The containers of the fixture, which are the core objects of D53 that a
// document store can hold. The view recent has no analogue.
const (
	Author   = "author"
	Book     = "book"
	Region   = "region"
	Shipment = "shipment"
)

// The users of the fixture.
const (
	Reader  = "reader"
	Auditor = "auditor"
)

// post is a step that makes a resource.
func post(name, path, body string) Step {
	return Step{Name: name, Method: "POST", Path: path, Body: body}
}

// drop is a step that removes a resource.
func drop(name, path string) Step {
	return Step{Name: name, Method: "DELETE", Path: path}
}

// Everything is a database holding one of every object the Cosmos DB queries
// read.
var Everything = Fixture{
	Name:   "everything",
	Schema: Database,
	Setup: []Step{
		// The core tables of D53, as containers. A container has no columns,
		// so what makes each one what it is lives in its policies.

		// A full text policy, and the default indexing policy.
		post(Author, "/colls", `{
			"id": "author",
			"partitionKey": {"paths": ["/id"], "kind": "Hash"},
			"fullTextPolicy": {"defaultLanguage": "en-US", "fullTextPaths": [{"path": "/bio", "language": "en-US"}]}
		}`),

		// Two unique keys, one of them over two paths, and an indexing policy
		// that indexes two paths and excludes the rest, with a composite index
		// over both. The core index book_published is the path /published.
		post(Book, "/colls", `{
			"id": "book",
			"partitionKey": {"paths": ["/author_id"], "kind": "Hash"},
			"uniqueKeyPolicy": {"uniqueKeys": [{"paths": ["/isbn"]}, {"paths": ["/title", "/edition"]}]},
			"indexingPolicy": {
				"indexingMode": "consistent",
				"automatic": true,
				"includedPaths": [{"path": "/title/?"}, {"path": "/published/?"}],
				"excludedPaths": [{"path": "/*"}],
				"compositeIndexes": [[
					{"path": "/title", "order": "ascending"},
					{"path": "/published", "order": "descending"}
				]]
			}
		}`),

		// No indexing at all, which the server allows only with no time to live.
		post(Region, "/colls", `{
			"id": "region",
			"partitionKey": {"paths": ["/id"], "kind": "Hash"},
			"indexingPolicy": {"indexingMode": "none", "automatic": false}
		}`),

		// A time to live of thirty days, the planar geometry type with a spatial
		// index, which needs the bounding box of the plane, and a computed
		// property.
		post(Shipment, "/colls", `{
			"id": "shipment",
			"partitionKey": {"paths": ["/region_id"], "kind": "Hash"},
			"defaultTtl": 2592000,
			"geospatialConfig": {"type": "Geometry"},
			"indexingPolicy": {
				"indexingMode": "consistent",
				"automatic": true,
				"includedPaths": [{"path": "/*"}],
				"excludedPaths": [],
				"spatialIndexes": [{
					"path": "/location/*",
					"types": ["Point"],
					"boundingBox": {"xmin": 0, "ymin": 0, "xmax": 100, "ymax": 100}
				}]
			},
			"computedProperties": [{"name": "carrier_lower", "query": "SELECT VALUE LOWER(c.carrier) FROM c"}]
		}`),

		// A function, a stored procedure and a trigger, in the container book.
		post("function", "/colls/book/udfs", `{
			"id": "full_title",
			"body": "function (title, subtitle) { return title + ': ' + subtitle; }"
		}`),
		post("stored procedure", "/colls/book/sprocs", `{
			"id": "book_count",
			"body": "function () { getContext().getResponse().setBody(1); }"
		}`),
		post("trigger", "/colls/book/triggers", `{
			"id": "stamp_book",
			"body": "function () { var doc = getContext().getRequest().getBody(); doc.stamped = true; getContext().getRequest().setBody(doc); }",
			"triggerType": "Pre",
			"triggerOperation": "Create"
		}`),

		// Two users, one with a read permission on the container book and one
		// with full access to the container author. A permission names a
		// container or something in it, and the server refuses a database.
		post("reader", "/users", `{"id": "reader"}`),
		post("auditor", "/users", `{"id": "auditor"}`),
		post("read permission", "/users/reader/permissions", `{
			"id": "read_books",
			"permissionMode": "Read",
			"resource": "$(container book)"
		}`),
		post("all permission", "/users/auditor/permissions", `{
			"id": "all_data",
			"permissionMode": "All",
			"resource": "$(container author)"
		}`),
	},
	Teardown: []Step{
		// A user takes its permissions with it, and a container takes its
		// scripts with it.
		drop("reader", "/users/reader"),
		drop("auditor", "/users/auditor"),
		drop(Author, "/colls/author"),
		drop(Book, "/colls/book"),
		drop(Region, "/colls/region"),
		drop(Shipment, "/colls/shipment"),
	},
}
