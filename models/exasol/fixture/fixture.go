// Package fixture builds the schema the Exasol queries read.
//
// It creates one of every object those queries report, so a test has rows
// worth checking rather than an empty catalog. Hard rule 9 requires it, and
// D53 requires the core objects to match every other fixture, so that the
// cross family comparison reads the same schema everywhere.
//
// A name written without quotes is folded to upper case, which is the
// standard's rule and Oracle's. The statements here are lower case and the
// catalog reports them upper case, and the tests fold before comparing.
//
// # What Exasol cannot be asked for here
//
// No index. Exasol has no CREATE INDEX. It builds and drops its own indices
// as queries need them, so the core object book_published cannot exist here,
// and Indexes reports the indices the engine made for the keys rather than
// one the fixture asked for. That is also why this fixture is not in the
// root module's fixture test, which requires the core index.
//
// No unique or check constraint. Both are refused on 2025.2 and 2026.2 with
// "Feature not supported", so book keeps its primary and foreign key and
// loses the other two. Exasol has primary key, foreign key and not null, and
// nothing else.
//
// No sequence, trigger, domain or user defined type. Exasol has none of
// them. An identity column is the only generator, and ticket has one.
//
// No user. A user belongs to the database rather than to a schema, and
// creating one outlives the schema, so the fixture creates a role and
// test/parity_test.go creates its own principals.
//
// A view comment is written inside CREATE VIEW. COMMENT ON VIEW is refused:
// Exasol says a view takes its comments from its own statement.
package fixture

import (
	"errors"

	"github.com/xo/dbmeta"
)

// Step is one statement the fixture runs.
type Step struct {
	Name string
	Stmt dbmeta.Stmt
}

// Result is what a step resolved to for one server.
type Result struct {
	Name    string
	Query   string
	Skipped bool
	Reason  string
}

// Fixture is a database and the statements that build and remove it.
type Fixture struct {
	Name     string
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

func at(name, query string) Step {
	return Step{Name: name, Stmt: dbmeta.Always(query)}
}

// adapter answers the requests Exasol sends a virtual schema adapter, with
// one table that is always the same.
//
// It is written in Lua because Lua runs inside the database. Every other
// language Exasol offers for a script needs a language container, and the
// nano image ships none. A virtual schema is what the three foreign data
// queries read, and an adapter that answers is the only way to have one
// without a second database to point at.
const adapter = `CREATE LUA ADAPTER SCRIPT dbmeta_fixture.fixed_adapter AS
function adapter_call(request_json)
	local cjson = require('cjson')
	local req = cjson.decode(request_json)
	local meta = {tables = {{name = 'REMOTE_ITEM', columns = {
		{name = 'ITEM_ID', dataType = {type = 'DECIMAL', precision = 18, scale = 0}}}}}}
	if req.type == 'createVirtualSchema' or req.type == 'refresh'
		or req.type == 'setProperties' then
		return cjson.encode({type = req.type, schemaMetadata = meta})
	elseif req.type == 'dropVirtualSchema' then
		return cjson.encode({type = req.type})
	elseif req.type == 'getCapabilities' then
		return cjson.encode({type = req.type, capabilities = {}})
	elseif req.type == 'pushdown' then
		return cjson.encode({type = req.type, sql = 'SELECT 1 FROM DUAL'})
	end
	error('unknown request ' .. req.type)
end
/`

// Everything holds one of every object the Exasol queries read.
var Everything = Fixture{
	Name:   "everything",
	Schema: "DBMETA_FIXTURE",
	Setup: []Step{
		at("schema", `CREATE SCHEMA dbmeta_fixture`),

		// The core objects D53 asks every fixture for, less the index and
		// the unique and check constraints, which Exasol does not have.
		at("author", `CREATE TABLE dbmeta_fixture.author (
	author_id INTEGER NOT NULL,
	name VARCHAR(128) NOT NULL,
	rating INTEGER,
	shade VARCHAR(16) DEFAULT 'plain',
	CONSTRAINT author_pk PRIMARY KEY (author_id)
)`),
		at("book", `CREATE TABLE dbmeta_fixture.book (
	book_id INTEGER NOT NULL,
	author_id INTEGER NOT NULL,
	title VARCHAR(255) NOT NULL,
	published DATE,
	CONSTRAINT book_pk PRIMARY KEY (book_id),
	CONSTRAINT book_author_fk FOREIGN KEY (author_id)
		REFERENCES dbmeta_fixture.author (author_id)
)`),
		// A composite primary key, and a composite foreign key into it.
		at("region", `CREATE TABLE dbmeta_fixture.region (
	country VARCHAR(64) NOT NULL,
	area VARCHAR(64) NOT NULL,
	CONSTRAINT region_pk PRIMARY KEY (country, area)
)`),
		at("shipment", `CREATE TABLE dbmeta_fixture.shipment (
	shipment_id INTEGER NOT NULL,
	country VARCHAR(64) NOT NULL,
	area VARCHAR(64) NOT NULL,
	amount DECIMAL(12, 2) NOT NULL,
	CONSTRAINT shipment_pk PRIMARY KEY (shipment_id),
	CONSTRAINT shipment_region_fk FOREIGN KEY (country, area)
		REFERENCES dbmeta_fixture.region (country, area)
)`),
		// The comment is part of the statement, because COMMENT ON VIEW is
		// refused.
		at("recent", `CREATE VIEW dbmeta_fixture.recent AS
	SELECT book_id, title FROM dbmeta_fixture.book
	COMMENT IS 'the newest books'`),

		// A partitioned and distributed table. Exasol partitions and
		// distributes by column, and the two are separate clauses. The
		// column is not called year, which Exasol reserves.
		at("archive", `CREATE TABLE dbmeta_fixture.archive (
	archive_id INTEGER NOT NULL,
	filed_year INTEGER NOT NULL,
	PARTITION BY filed_year,
	DISTRIBUTE BY archive_id
)`),

		// An identity column, on a table of its own so that the core
		// tables stay the same shape everywhere.
		at("ticket", `CREATE TABLE dbmeta_fixture.ticket (
	ticket_id DECIMAL(18, 0) IDENTITY,
	note VARCHAR(64),
	CONSTRAINT ticket_pk PRIMARY KEY (ticket_id)
)`),

		// A SQL function, which is the one routine Exasol keeps apart from
		// its scripts.
		at("function", `CREATE FUNCTION dbmeta_fixture.doubled (n DECIMAL(18, 0))
RETURN DECIMAL(18, 0)
IS
BEGIN
	RETURN n * 2;
END doubled;`),
		// A scalar UDF, a set UDF that returns one value, which is what
		// Exasol has for an aggregate, and a set UDF that emits rows.
		at("scalar script", `CREATE LUA SCALAR SCRIPT dbmeta_fixture.tripled (n DOUBLE)
RETURNS DOUBLE AS
function run(ctx)
	return ctx.n * 3
end
/`),
		at("aggregate script", `CREATE LUA SET SCRIPT dbmeta_fixture.total (n DOUBLE)
RETURNS DOUBLE AS
function run(ctx)
	local s = 0
	repeat
		if ctx.n ~= null then s = s + ctx.n end
	until not ctx.next()
	return s
end
/`),
		at("emitting script", `CREATE LUA SET SCRIPT dbmeta_fixture.spread (n DOUBLE)
EMITS (v DOUBLE) AS
function run(ctx)
	repeat ctx.emit(ctx.n) until not ctx.next()
end
/`),
		// A scripting script, which is a program run with EXECUTE SCRIPT
		// rather than a function called in a query.
		at("scripting script", `CREATE LUA SCRIPT dbmeta_fixture.hello () AS
output('hello')
/`),

		// A connection, an adapter and a virtual schema built with them,
		// which gives the foreign data queries a wrapper, a server and a
		// table to report.
		at("connection", `CREATE CONNECTION dbmeta_remote_conn
	TO 'https://example.invalid' USER 'remote' IDENTIFIED BY 'remote'`),
		at("adapter", adapter),
		at("virtual schema", `CREATE VIRTUAL SCHEMA dbmeta_remote
	USING dbmeta_fixture.fixed_adapter
	WITH CONNECTION_NAME = 'DBMETA_REMOTE_CONN' FLAVOR = 'fixed'`),

		// A role and grants to it, so Roles, RoleGrants and Privileges
		// have rows the fixture put there.
		at("role", `CREATE ROLE dbmeta_reader`),
		at("grant table", `GRANT SELECT ON dbmeta_fixture.author TO dbmeta_reader`),
		at("grant function", `GRANT EXECUTE ON dbmeta_fixture.doubled TO dbmeta_reader`),
		// A connection granted to the role, which is what UserMappings
		// reports.
		at("grant connection", `GRANT CONNECTION dbmeta_remote_conn TO dbmeta_reader`),

		// Comments, one of each kind the Comments query reads.
		at("schema comment", `COMMENT ON SCHEMA dbmeta_fixture IS 'the fixture'`),
		at("table comment", `COMMENT ON TABLE dbmeta_fixture.author IS 'people who write'`),
		at("column comment", `COMMENT ON COLUMN dbmeta_fixture.author.name IS 'the author name'`),
		at("function comment", `COMMENT ON FUNCTION dbmeta_fixture.doubled IS 'twice n'`),
		at("script comment", `COMMENT ON SCRIPT dbmeta_fixture.tripled IS 'three times n'`),
		at("role comment", `COMMENT ON ROLE dbmeta_reader IS 'reads authors'`),
		at("connection comment", `COMMENT ON CONNECTION dbmeta_remote_conn IS 'nowhere at all'`),
	},
	Teardown: []Step{
		// The virtual schema first, because its adapter is in the fixture
		// schema and dropping that schema would strand it.
		at("virtual schema", `DROP VIRTUAL SCHEMA IF EXISTS dbmeta_remote CASCADE`),
		at("schema", `DROP SCHEMA IF EXISTS dbmeta_fixture CASCADE`),
		at("role", `DROP ROLE IF EXISTS dbmeta_reader`),
		at("connection", `DROP CONNECTION IF EXISTS dbmeta_remote_conn`),
	},
}
