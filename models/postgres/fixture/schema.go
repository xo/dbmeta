package fixture

import "github.com/xo/dbmeta"

// Releases a step needs.
var (
	v10 = dbmeta.V(10)
	v11 = dbmeta.V(11)
	v12 = dbmeta.V(12)
	v14 = dbmeta.V(14)
	v16 = dbmeta.V(16)
)

// Everything is a schema containing one of every object kind the PostgreSQL
// queries read, so that a query returns rows rather than an empty result.
//
// It is additive. A later release can add an object and will not rename or
// remove one that is already here.
//
// Five objects are skipped below release 10, because PostgreSQL did not have
// them: identity columns, publications, and the rest that came with logical
// replication. A generated column is skipped below release 12, statistics on
// an expression below 14, and a collation with tailoring rules below 16. The
// queries that read those objects are refused or pad on the same releases, so
// the fixture and the queries agree.
var Everything = Fixture{
	Name:   "everything",
	Schema: "dbmeta_fixture",
	Setup: []Step{
		at("schema", `CREATE SCHEMA dbmeta_fixture`),
		at("schema comment", `COMMENT ON SCHEMA dbmeta_fixture IS 'every object kind dbmeta reads'`),

		at("domain", `CREATE DOMAIN dbmeta_fixture.positive AS integer CHECK (VALUE > 0)`),
		at("enum type", `CREATE TYPE dbmeta_fixture.colour AS ENUM ('red', 'green', 'blue')`),
		at("composite type", `CREATE TYPE dbmeta_fixture.point AS (x integer, y integer)`),
		at("sequence", `CREATE SEQUENCE dbmeta_fixture.counter START 10 INCREMENT 2`),
		at("sequence comment", `COMMENT ON SEQUENCE dbmeta_fixture.counter IS 'a sequence'`),

		at("author table", `CREATE TABLE dbmeta_fixture.author (
	author_id serial PRIMARY KEY,
	name text NOT NULL,
	rating dbmeta_fixture.positive,
	shade dbmeta_fixture.colour DEFAULT 'red'
)`),
		at("author comment", `COMMENT ON TABLE dbmeta_fixture.author IS 'people who write'`),
		at("author column comment", `COMMENT ON COLUMN dbmeta_fixture.author.author_id IS 'surrogate key'`),

		at("book table", `CREATE TABLE dbmeta_fixture.book (
	book_id serial PRIMARY KEY,
	author_id integer NOT NULL REFERENCES dbmeta_fixture.author(author_id),
	title text NOT NULL UNIQUE,
	published date,
	CONSTRAINT title_not_empty CHECK (title <> '')
)`),
		at("book index", `CREATE INDEX book_published ON dbmeta_fixture.book (published DESC)`),
		at("book expression index", `CREATE INDEX book_lower_title ON dbmeta_fixture.book (lower(title))`),
		// An INCLUDE column is in the index and has no sort order, so its
		// descending property is NULL. A partial index has a predicate.
		from("book covering index", v11, `CREATE INDEX book_covering ON dbmeta_fixture.book (author_id) INCLUDE (title)`),
		at("book partial index", `CREATE INDEX book_recent ON dbmeta_fixture.book (published) WHERE published > '2000-01-01'`),

		// A role has no schema and belongs to no database. A default
		// privilege without IN SCHEMA holds for every schema, and a setting
		// without IN DATABASE holds for every database, so both read NULL
		// where a catalog entry names one.
		at("fixture role", `CREATE ROLE dbmeta_fixture_role`),
		at("role setting for every database", `ALTER ROLE dbmeta_fixture_role SET search_path TO dbmeta_fixture, public`),
		at("default privilege for every schema", `ALTER DEFAULT PRIVILEGES FOR ROLE dbmeta_fixture_role GRANT SELECT ON TABLES TO PUBLIC`),

		// An identity column and a generated column go on a table of their
		// own rather than on author or book. The core tables are the ones the
		// cross family conformance test compares, and they hold the same
		// columns on every database so that a difference in the report is a
		// difference in the model. See the fixture test in the root module.
		at("extras table", `CREATE TABLE dbmeta_fixture.extras (
	extras_id integer PRIMARY KEY,
	title text NOT NULL
)`),

		// The view selects the same two columns on every database, for the
		// same reason.
		at("view", `CREATE VIEW dbmeta_fixture.recent AS
	SELECT book_id, title FROM dbmeta_fixture.book WHERE published IS NOT NULL`),
		at("materialized view", `CREATE MATERIALIZED VIEW dbmeta_fixture.author_count AS
	SELECT author_id, count(*) AS books FROM dbmeta_fixture.book GROUP BY author_id`),

		at("function", `CREATE FUNCTION dbmeta_fixture.touch() RETURNS trigger
	LANGUAGE plpgsql AS $$ BEGIN RETURN NEW; END $$`),
		at("function comment", `COMMENT ON FUNCTION dbmeta_fixture.touch() IS 'a trigger function'`),

		// EXECUTE FUNCTION is release 11 syntax. EXECUTE PROCEDURE still works
		// above it, but writing the deprecated form everywhere tests
		// syntax that nobody writes on a current server.
		choose("trigger",
			dbmeta.Fragment{Query: `CREATE TRIGGER book_touch BEFORE UPDATE ON dbmeta_fixture.book
	FOR EACH ROW EXECUTE PROCEDURE dbmeta_fixture.touch()`},
			dbmeta.Fragment{Min: v11, Query: `CREATE TRIGGER book_touch BEFORE UPDATE ON dbmeta_fixture.book
	FOR EACH ROW EXECUTE FUNCTION dbmeta_fixture.touch()`},
		),

		// the pair that separates default privileges from revoked ones. A NULL
		// access list means the default privileges apply and an empty one
		// means everything was revoked, and those must not read alike.
		at("default privileges table", `CREATE TABLE dbmeta_fixture.default_privs (id integer)`),
		at("revoked privileges table", `CREATE TABLE dbmeta_fixture.revoked_privs (id integer)`),
		at("revoke", `REVOKE ALL ON dbmeta_fixture.revoked_privs FROM CURRENT_USER`),

		// identity columns arrived in release 10
		from("identity column", v10,
			`ALTER TABLE dbmeta_fixture.extras ADD COLUMN serial_no integer GENERATED BY DEFAULT AS IDENTITY`),

		// a generated column arrived in release 12
		from("generated column", v12,
			`ALTER TABLE dbmeta_fixture.extras ADD COLUMN slug text GENERATED ALWAYS AS (lower(title)) STORED`),

		// partitioning and logical replication arrived in release 10
		from("partitioned table", v10, `CREATE TABLE dbmeta_fixture.sales (
	sold_on date NOT NULL,
	amount integer NOT NULL
) PARTITION BY RANGE (sold_on)`),
		from("partition", v10, `CREATE TABLE dbmeta_fixture.sales_2026
	PARTITION OF dbmeta_fixture.sales FOR VALUES FROM ('2026-01-01') TO ('2027-01-01')`),
		from("publication", v10, `CREATE PUBLICATION dbmeta_fixture_pub FOR TABLE dbmeta_fixture.book`),
		from("extended statistics", v10,
			`CREATE STATISTICS dbmeta_fixture.book_stats ON author_id, published FROM dbmeta_fixture.book`),
		// statistics on an expression arrived in release 14, and they are
		// what pg_get_statisticsobjdef_columns prints
		from("expression statistics", v14,
			`CREATE STATISTICS dbmeta_fixture.title_stats ON lower(title), author_id FROM dbmeta_fixture.book`),

		// A configuration of its own, so that TextSearchConfigMaps has rows
		// in the fixture's schema. A copy takes every mapping of simple.
		at("text search configuration",
			`CREATE TEXT SEARCH CONFIGURATION dbmeta_fixture.plain (COPY = pg_catalog.simple)`),
		// tailoring rules arrived in release 16
		from("collation with rules", v16,
			`CREATE COLLATION dbmeta_fixture.tailored (provider = icu, locale = 'und', rules = '&a < g')`),

		// A composite primary key and a composite foreign key, so that
		// ConstraintColumns has more than one column per constraint to order.
		// A single column key cannot show that the ordinals line up.
		at("region table", `CREATE TABLE dbmeta_fixture.region (
	country text NOT NULL,
	area text NOT NULL,
	PRIMARY KEY (country, area)
)`),
		at("shipment table", `CREATE TABLE dbmeta_fixture.shipment (
	shipment_id serial PRIMARY KEY,
	country text NOT NULL,
	area text NOT NULL,
	amount integer NOT NULL,
	CONSTRAINT shipment_region_fk FOREIGN KEY (country, area)
		REFERENCES dbmeta_fixture.region(country, area)
)`),

		// A routine with named parameters, a default and an output parameter,
		// so that RoutineParameters has every mode to report. touch() above
		// takes none.
		at("routine with parameters", `CREATE FUNCTION dbmeta_fixture.addup(
	a integer, b integer DEFAULT 1, OUT total integer
) AS $$ SELECT a + b $$ LANGUAGE sql`),

		// Rows, and statistics over them. ColumnStats returns nothing for a
		// column that was never analyzed, so without this the query runs and
		// proves nothing.
		at("author rows", `INSERT INTO dbmeta_fixture.author (name, rating)
	SELECT 'author ' || g, (g % 5) + 1 FROM generate_series(1, 200) g`),
		at("analyze", `ANALYZE dbmeta_fixture.author`),

		// A table of its own for what describe commands print about a
		// relation, so that the core tables stay the same on every
		// database. It is unlogged, one column stores outside the table and
		// has its own statistics target, and its indexes are the replica
		// identity, the clustered one, and the one a deferrable constraint
		// owns. See D198.
		at("scratch table", `CREATE UNLOGGED TABLE dbmeta_fixture.scratch (
	scratch_id integer NOT NULL,
	payload text,
	stamp timestamptz
)`),
		at("scratch storage and statistics", `ALTER TABLE dbmeta_fixture.scratch
	ALTER COLUMN payload SET STORAGE EXTERNAL,
	ALTER COLUMN payload SET STATISTICS 500`),
		// column compression arrived in release 14
		from("scratch compression", v14,
			`ALTER TABLE dbmeta_fixture.scratch ALTER COLUMN payload SET COMPRESSION pglz`),
		at("scratch replica identity index", `CREATE UNIQUE INDEX scratch_key ON dbmeta_fixture.scratch (scratch_id)`),
		at("scratch replica identity", `ALTER TABLE dbmeta_fixture.scratch REPLICA IDENTITY USING INDEX scratch_key`),
		at("scratch clustered index", `CREATE INDEX scratch_stamp ON dbmeta_fixture.scratch (stamp)`),
		at("scratch cluster", `CLUSTER dbmeta_fixture.scratch USING scratch_stamp`),
		at("scratch deferrable constraint", `ALTER TABLE dbmeta_fixture.scratch
	ADD CONSTRAINT scratch_payload_unique UNIQUE (payload) DEFERRABLE INITIALLY DEFERRED`),

		// LEAKPROOF needs a superuser, which the fixture runs as.
		at("leakproof function", `CREATE FUNCTION dbmeta_fixture.same(integer) RETURNS integer
	LANGUAGE sql IMMUTABLE LEAKPROOF AS 'SELECT $1'`),
	},
	Teardown: []Step{
		from("drop publication", v10, `DROP PUBLICATION IF EXISTS dbmeta_fixture_pub`),
		at("drop schema", `DROP SCHEMA IF EXISTS dbmeta_fixture CASCADE`),
		at("drop fixture role", `DO $$ BEGIN
	IF EXISTS (SELECT FROM pg_catalog.pg_roles WHERE rolname = 'dbmeta_fixture_role') THEN
		DROP OWNED BY dbmeta_fixture_role;
		DROP ROLE dbmeta_fixture_role;
	END IF;
END $$`),
	},
}
