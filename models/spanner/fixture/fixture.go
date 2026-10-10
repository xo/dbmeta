// Package fixture holds a known good Spanner schema, in the GoogleSQL dialect.
//
// Like every fixture here it is exported API and additive: a later release can
// add an object and will not rename or remove one. See the PostgreSQL fixture
// for the rules, which are the same.
//
// It is not versioned. Spanner has no release that SQL reads, and every step
// runs on Spanner Omni 2026.r4-lts. The Resolve methods keep the shape the
// other fixtures have so that a caller can treat them alike.
//
// # Every step is DDL
//
// Spanner takes a schema change as a long running operation, and each one
// takes seconds. A caller runs the steps as one batch, with START BATCH DDL
// before them and RUN BATCH after them, on a single connection, and gets one
// operation for all of them. The steps are in the order that Spanner needs: a parent before its child, a table before the index on
// it. The teardown is in the reverse order, and it drops what a table needs
// dropped first, which is its indexes, and revokes every grant before it drops
// a role.
//
// # What it cannot build
//
// A named schema holds every object but a few. A role and a locality group are
// not in a schema, so the fixture gives them a name with a prefix. A change
// stream and a vector index are in the default schema, which Spanner names with
// the empty string, because Spanner Omni refuses either in a named schema. A
// stream in the default schema can name a table of a named schema. One table is
// in the default schema as well, so that a test can read both.
//
// It builds no placement, because a placement needs an instance partition that
// a single server does not have, and no model, because Spanner Omni refuses
// CREATE MODEL. It cannot make a trigger, a rule, a policy or a type, because
// Spanner has none, and a proto bundle needs a compiled descriptor that SQL
// cannot build. It builds no property graph, because no query reads one.
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
	Query   string
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

// The names that live outside the schema.
const (
	// Reader, Staff and Stranger are database roles. Staff is a member of Reader.
	Reader = "dbmeta_reader"
	Staff  = "dbmeta_staff"
	// Stranger holds no grant and belongs to no role.
	Stranger = "dbmeta_stranger"
	// Group is a locality group, which stores on disk.
	Group = "dbmeta_spinning"
	// Plain is a table in the default schema, and Vector is the vector index on
	// it.
	Plain  = "dbmeta_plain"
	Vector = "dbmeta_plain_vector"
)

// Everything is a schema holding one of every object the Spanner queries read.
//
// The names match the other fixtures, so a test that reads author and book on
// PostgreSQL reads the same two here.
var Everything = Fixture{
	Name:   "everything",
	Schema: "dbmeta_fixture",
	Setup: []Step{
		at("schema", "CREATE SCHEMA dbmeta_fixture"),

		// A single column key, a not null column, a nullable column and a
		// column with a default, and a check constraint that is not on the
		// core book.
		at("author table", "CREATE TABLE dbmeta_fixture.author (\n"+
			"	author_id INT64 NOT NULL,\n"+
			"	name STRING(100) NOT NULL,\n"+
			"	rating INT64,\n"+
			"	shade STRING(10) DEFAULT ('red'),\n"+
			"	CONSTRAINT shade_check CHECK (shade IN ('red', 'green', 'blue'))\n"+
			") PRIMARY KEY (author_id)"),

		// Spanner has no unique constraint, so the unique title is a unique
		// index. The foreign key backs itself with an index that Spanner makes.
		at("book table", "CREATE TABLE dbmeta_fixture.book (\n"+
			"	book_id INT64 NOT NULL,\n"+
			"	author_id INT64 NOT NULL,\n"+
			"	title STRING(200) NOT NULL,\n"+
			"	published DATE,\n"+
			"	CONSTRAINT book_author_fk FOREIGN KEY (author_id)\n"+
			"		REFERENCES dbmeta_fixture.author (author_id),\n"+
			"	CONSTRAINT title_not_empty CHECK (title <> '')\n"+
			") PRIMARY KEY (book_id)"),
		at("book unique index", "CREATE UNIQUE INDEX dbmeta_fixture.book_title ON dbmeta_fixture.book (title)"),
		at("book index", "CREATE INDEX dbmeta_fixture.book_published ON dbmeta_fixture.book (published DESC)"),
		// A null filtered index, which keeps a predicate, and a stored column.
		at("book null filtered index", "CREATE NULL_FILTERED INDEX dbmeta_fixture.book_published_filtered\n"+
			"	ON dbmeta_fixture.book (published) STORING (title)"),

		at("view", "CREATE VIEW dbmeta_fixture.recent SQL SECURITY INVOKER AS\n"+
			"	SELECT b.book_id, b.title FROM dbmeta_fixture.book AS b WHERE b.published IS NOT NULL"),

		// A composite primary key and a composite foreign key, named the way
		// every other fixture names them.
		at("region table", "CREATE TABLE dbmeta_fixture.region (\n"+
			"	country STRING(2) NOT NULL,\n"+
			"	area STRING(20) NOT NULL\n"+
			") PRIMARY KEY (country, area)"),
		at("shipment table", "CREATE TABLE dbmeta_fixture.shipment (\n"+
			"	shipment_id INT64 NOT NULL,\n"+
			"	country STRING(2) NOT NULL,\n"+
			"	area STRING(20) NOT NULL,\n"+
			"	amount INT64 NOT NULL,\n"+
			"	CONSTRAINT shipment_region_fk FOREIGN KEY (country, area)\n"+
			"		REFERENCES dbmeta_fixture.region (country, area)\n"+
			") PRIMARY KEY (shipment_id)"),

		// A table interleaved in its parent, and an index interleaved in it.
		at("chapter table", "CREATE TABLE dbmeta_fixture.chapter (\n"+
			"	book_id INT64 NOT NULL,\n"+
			"	chapter_no INT64 NOT NULL,\n"+
			"	heading STRING(200)\n"+
			") PRIMARY KEY (book_id, chapter_no),\n"+
			"INTERLEAVE IN PARENT dbmeta_fixture.book ON DELETE CASCADE"),
		at("chapter interleaved index", "CREATE INDEX dbmeta_fixture.chapter_heading\n"+
			"	ON dbmeta_fixture.chapter (book_id, heading), INTERLEAVE IN dbmeta_fixture.book"),

		at("sequence", "CREATE SEQUENCE dbmeta_fixture.ticket\n"+
			"	OPTIONS (sequence_kind = 'bit_reversed_positive', start_with_counter = 1000)"),

		// A stored generated column, an identity column, a default that reads a
		// sequence, and a hidden TOKENLIST column.
		at("sales table", "CREATE TABLE dbmeta_fixture.sales (\n"+
			"	sold_on DATE NOT NULL,\n"+
			"	region STRING(20) NOT NULL,\n"+
			"	amount INT64 NOT NULL,\n"+
			"	doubled INT64 AS (amount * 2) STORED,\n"+
			"	line_id INT64 NOT NULL GENERATED BY DEFAULT AS IDENTITY (BIT_REVERSED_POSITIVE),\n"+
			"	ticket_id INT64 DEFAULT (GET_NEXT_SEQUENCE_VALUE(SEQUENCE dbmeta_fixture.ticket)),\n"+
			"	body STRING(MAX),\n"+
			"	tokens TOKENLIST AS (TOKENIZE_FULLTEXT(body)) HIDDEN\n"+
			") PRIMARY KEY (sold_on, region)"),
		at("sales search index", "CREATE SEARCH INDEX dbmeta_fixture.sales_search ON dbmeta_fixture.sales (tokens)"),

		// A row deletion policy and a commit timestamp column.
		at("event table", "CREATE TABLE dbmeta_fixture.event (\n"+
			"	event_id INT64 NOT NULL,\n"+
			"	kind STRING(20),\n"+
			"	happened TIMESTAMP NOT NULL OPTIONS (allow_commit_timestamp = true)\n"+
			") PRIMARY KEY (event_id),\n"+
			"ROW DELETION POLICY (OLDER_THAN(happened, INTERVAL 30 DAY))"),

		// A locality group, a table stored in it, and a synonym for the table.
		at("locality group", "CREATE LOCALITY GROUP "+Group+" OPTIONS (storage = 'hdd')"),
		at("ledger table", "CREATE TABLE dbmeta_fixture.ledger (\n"+
			"	account STRING(20) NOT NULL,\n"+
			"	entry INT64 NOT NULL,\n"+
			"	amount INT64 NOT NULL\n"+
			") PRIMARY KEY (account, entry),\n"+
			"OPTIONS (locality_group = '"+Group+"')"),
		at("ledger synonym", "ALTER TABLE dbmeta_fixture.ledger ADD SYNONYM dbmeta_fixture.old_ledger"),

		at("function", "CREATE FUNCTION dbmeta_fixture.double_it(x INT64) RETURNS INT64\n"+
			"	SQL SECURITY INVOKER AS (x * 2)"),

		// A table in the default schema, which Spanner names with the empty
		// string, and a vector index on it. Spanner Omni refuses a vector index
		// in a named schema.
		at("plain table", "CREATE TABLE "+Plain+" (\n"+
			"	plain_id INT64 NOT NULL,\n"+
			"	embedding ARRAY<FLOAT32>(vector_length=>3)\n"+
			") PRIMARY KEY (plain_id)"),
		at("plain vector index", "CREATE VECTOR INDEX "+Vector+" ON "+Plain+" (embedding)\n"+
			"	WHERE embedding IS NOT NULL\n"+
			"	OPTIONS (distance_type = 'COSINE', tree_depth = 2, num_leaves = 10)"),

		// Three change streams: one that names a table and a column and leaves
		// deletes out, one that names a table and no column, and one for
		// everything.
		at("change stream", "CREATE CHANGE STREAM dbmeta_book_changes\n"+
			"	FOR dbmeta_fixture.book (title), dbmeta_fixture.author\n"+
			"	OPTIONS (retention_period = '36h', exclude_delete = true)"),
		at("key change stream", "CREATE CHANGE STREAM dbmeta_key_changes\n"+
			"	FOR dbmeta_fixture.region ()"),
		at("all change stream", "CREATE CHANGE STREAM dbmeta_all_changes FOR ALL"),

		at("reader role", "CREATE ROLE "+Reader),
		at("staff role", "CREATE ROLE "+Staff),
		at("stranger role", "CREATE ROLE "+Stranger),
		at("role grant", "GRANT ROLE "+Reader+" TO ROLE "+Staff),
		at("schema grant", "GRANT USAGE ON SCHEMA dbmeta_fixture TO ROLE "+Reader),
		at("table grant", "GRANT SELECT ON TABLE dbmeta_fixture.author TO ROLE "+Reader),
		at("column grant", "GRANT SELECT (title) ON TABLE dbmeta_fixture.book TO ROLE "+Reader),
		at("view grant", "GRANT SELECT ON VIEW dbmeta_fixture.recent TO ROLE "+Reader),
		at("change stream grant", "GRANT SELECT ON CHANGE STREAM dbmeta_book_changes TO ROLE "+Reader),
		at("function grant", "GRANT EXECUTE ON TABLE FUNCTION READ_dbmeta_book_changes TO ROLE "+Reader),
	},
	Teardown: []Step{
		at("revoke function", "REVOKE EXECUTE ON TABLE FUNCTION READ_dbmeta_book_changes FROM ROLE "+Reader),
		at("revoke change stream", "REVOKE SELECT ON CHANGE STREAM dbmeta_book_changes FROM ROLE "+Reader),
		at("revoke view", "REVOKE SELECT ON VIEW dbmeta_fixture.recent FROM ROLE "+Reader),
		at("revoke column", "REVOKE SELECT (title) ON TABLE dbmeta_fixture.book FROM ROLE "+Reader),
		at("revoke table", "REVOKE SELECT ON TABLE dbmeta_fixture.author FROM ROLE "+Reader),
		at("revoke schema", "REVOKE USAGE ON SCHEMA dbmeta_fixture FROM ROLE "+Reader),
		at("revoke role", "REVOKE ROLE "+Reader+" FROM ROLE "+Staff),
		at("drop stranger", "DROP ROLE "+Stranger),
		at("drop staff", "DROP ROLE "+Staff),
		at("drop reader", "DROP ROLE "+Reader),
		at("drop all change stream", "DROP CHANGE STREAM dbmeta_all_changes"),
		at("drop key change stream", "DROP CHANGE STREAM dbmeta_key_changes"),
		at("drop change stream", "DROP CHANGE STREAM dbmeta_book_changes"),
		at("drop plain vector index", "DROP INDEX "+Vector),
		at("drop plain", "DROP TABLE "+Plain),
		at("drop function", "DROP FUNCTION dbmeta_fixture.double_it"),
		at("drop ledger synonym", "ALTER TABLE dbmeta_fixture.ledger DROP SYNONYM dbmeta_fixture.old_ledger"),
		at("drop ledger", "DROP TABLE dbmeta_fixture.ledger"),
		at("drop locality group", "DROP LOCALITY GROUP "+Group),
		at("drop event", "DROP TABLE dbmeta_fixture.event"),
		at("drop sales search index", "DROP SEARCH INDEX dbmeta_fixture.sales_search"),
		at("drop sales", "DROP TABLE dbmeta_fixture.sales"),
		at("drop sequence", "DROP SEQUENCE dbmeta_fixture.ticket"),
		at("drop chapter index", "DROP INDEX dbmeta_fixture.chapter_heading"),
		at("drop chapter", "DROP TABLE dbmeta_fixture.chapter"),
		at("drop shipment", "DROP TABLE dbmeta_fixture.shipment"),
		at("drop region", "DROP TABLE dbmeta_fixture.region"),
		at("drop view", "DROP VIEW dbmeta_fixture.recent"),
		at("drop book null filtered index", "DROP INDEX dbmeta_fixture.book_published_filtered"),
		at("drop book index", "DROP INDEX dbmeta_fixture.book_published"),
		at("drop book unique index", "DROP INDEX dbmeta_fixture.book_title"),
		at("drop book", "DROP TABLE dbmeta_fixture.book"),
		at("drop author", "DROP TABLE dbmeta_fixture.author"),
		at("drop schema", "DROP SCHEMA dbmeta_fixture"),
	},
}
