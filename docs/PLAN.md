# dbmeta Plan

This document records the plan for `github.com/xo/dbmeta`. The decisions are
in [`decisions/`](decisions/README.md), one file each (D111). Other coding
agents must read this file before they change code in this repository.

`AGENTS.md`, in the repository root, holds the rules for writing code here,
and its table names every document in `docs/` and the task each one is for.
Read `NULLS.md` before writing a query for any database.

Parts of this document were written at the start of the project, before any
code existed. Those parts say so, and they are kept as the record of where the
work began. The rest describes the project as it is.

## Purpose

`dbmeta` holds database metadata queries and models in one Go module. Other
projects consume it. The first two consumers are `usql`, an interactive SQL
command line client, and `dbtpl`, a code generator that reads a database
schema. The goal is to cover as many databases as `usql` supports.

The work is a move of the existing `usql/drivers/metadata` package into this
module, not a new design from nothing. The `usql` backlog records the move as
item 7.

## Architecture

The module has two layers.

The root package `dbmeta` gives consumers one driver agnostic API. Driver
agnostic means the caller asks for tables without knowing which database
answers. This layer holds the object types, the `Query` values, the one
method `Queryer` interface, the `Args` filter and the error values.

The package `dbmeta/models/<model>` holds the queries for one database. For
example, `dbmeta/models/sqlite3` holds the SQLite3 queries. Each model scans
its rows into the root package's types, so every model returns the same Go
types.

There is no version directory below the driver. A database changes its metadata
between releases, and D8 holds those differences as data inside the one driver
package rather than as a package per release. The data is written by hand, and
nothing generates it (D71).

A third layer joins the two. Something must convert the per driver model rows
into the common types of the root package. It also merges the version fragments
for the connected server. That code is in the root package, and it is written:
D3 puts the driver agnostic API there, D8 holds the fragments as data, and
`Stmt.Build` merges them for a detected version. Each model converts its own
rows in the `Scan` function it registers.

## Decisions

Every decision is a file in [`decisions/`](decisions/), one per decision, and
[`decisions/README.md`](decisions/README.md) is the index. D111 moved them out
of this file.

## What exists today

`models/` holds 25 native models: cassandra, clickhouse, cockroachdb,
couchbase, cratedb, databend, duckdb, exasol, firebird, hana, hive, impala,
mysql, oracle, postgres, presto, questdb, redshift, singlestore, snowflake,
sqlite3, sqlserver, tidb, trino and vertica. ScyllaDB is a flavor of the
Cassandra model and MySQL a flavor of the MariaDB one. `models/informationschema` is the shared
model for any database with a standard `information_schema`, and no native
model builds on it. `COVERAGE.md` holds what each one answers.

`container/` names every release the tests run against, and `dbrun` starts
each one. `dbrun` also starts servers for dbimp's drivers that have no model:
SurrealDB, Neo4j, the five products of D112 that have no model (ArangoDB,
InfluxDB, Apache Pinot, rqlite and libSQL) and the three Avatica servers of
D113. It knows the embedded databases too, including two, chai and csvq, that
have no model yet (D116 and D119). `README.md` holds the support tiers.

`usql` reads `dbmeta` in work that is staged and not committed, and `dbtpl`
does not read it yet. `USQL.md` and `DBTPL.md` hold what each gains, and `BACKLOG.md` holds that work.

## What existed at the start

This section describes `usql` and `dbtpl` as they were on 2026-09-24, when the
project began, and it is kept as the record of what was moved. It is not the
present state of either.

### `usql/drivers/metadata`

This is the code to move. It contains:

- `metadata.go`, 1175 lines. It defines 15 object types, a `*Set` cursor type
  for each, a one method reader interface for each, one `Filter` type, and the
  `Writer` interface.
- `reader.go`, 273 lines. It defines `PluginReader`, which composes readers by
  type assertion, and `LoggingReader`, which adds logging, dry run, and a
  timeout.
- `writer.go`, 838 lines. It defines `DefaultWriter`, which renders the output
  of the `\d` family of commands through `tblfmt`.

The 15 object types are Catalog, Schema, Table, Column, ColumnStat, Index,
IndexColumn, Trigger, Constraint, ConstraintColumn, Function, FunctionColumn,
Sequence, PrivilegeSummary, and the shared `Bool`.

### `dbtpl/loader`

`dbtpl` reads a schema through a `Loader` struct of functions. It covers five
databases: postgres, mysql, sqlserver, sqlite3, and oracle. It needs four
things that `usql` does not model:

1. Enums and enum values.
2. Procedures and procedure parameters.
3. View DDL operations: create, schema, truncate, and drop. DDL means data
   definition language, the SQL that creates and drops objects.
4. A column order query for postgres.

The union of the `usql` object set and the `dbtpl` object set is the target
object set for `dbmeta`.

### Coupling from the metadata package back into `usql`

The reader half of the package needs six symbols from `usql`. All six are small
and `dbmeta` can define its own:

- `text.ErrNotSupported`
- `text.NotSupportedByDriver`
- `text.RelationNotFound`
- `text.ErrWrongNumberOfArguments`
- `drivers.DB`, an interface of eight methods that `database/sql.DB` and
  `database/sql.Tx` both satisfy
- `env.Vars()`, used by `writer.go` only

### Driver coverage at the start

14 of the 43 `usql` drivers have metadata support:

clickhouse, databend, duckdb, impala, mymysql, mysql, netezza, oracle, pgx,
postgres, snowflake, sqlite3, sqlserver, trino.

29 drivers have none:

adodb, athena, avatica, bigquery, cassandra, chai, cosmos, couchbase, csvq,
databricks, dynamodb, exasol, firebird, flightsql, godror, h2, hive, ignite,
maxcompute, moderncsqlite, odbc, ots, presto, sapase, saphana, spanner,
vertica, voltdb, ydb.

The 14 supported drivers are also split across two places. Five live under
`usql/drivers/metadata/`. The rest live next to their driver, for example
`usql/drivers/sqlite3/sqshared/reader.go` at 331 lines and
`usql/drivers/sqlserver/reader.go` at 239 lines. The move must collect both.

## How a model is written

Nothing generates one. D71 has the whole of it: no `tool` directive, no
`go:generate`, no generated header, and no `dbtpl` in the loop. This section
once described `dbtpl query` and its flags, and following it produced nothing
because none of it was ever wired up.

A model is written against a running server. Start one with `dbrun`, write the
statement, run it, read the columns back, and declare the fields to match. The
NULL rules in `docs/NULLS.md` are the part that takes the care: a field gets
`sql.Null[T]` whenever the column can be absent, and a release with no source
for a column selects `NULL AS "name"` rather than a literal.

## Known defects to fix once

### The NULL scan defect

Four open pull requests against `usql` fix the same fault in four places: 526
for information schema functions, 570 and 524 for oracle, and 583 for postgres.
Commit `200c0c8` fixed a fifth case by wrapping `routine_definition` in
`COALESCE`.

The cause is plain. The readers scan into struct fields of type `string`, as in
`rows.Scan(&rec.Catalog, &rec.Schema, &rec.Name, &rec.Type)`. When the database
returns SQL NULL for one of those columns, the scan fails.

Note for agents: this is not the same fault as `usql` backlog item 14. That
item describes the display path, where `usql` builds a scan destination with
`reflect.New(ct.ScanType())`. The helper `usql/drivers/columns.go:53`,
`NullSafeColumnType`, fixes the display path. It does not fix the readers,
because the readers never call it. There is no `reflect.New` anywhere under
`drivers/metadata`.

The fix for `dbmeta` is a field of type `sql.Null[T]` wherever a column can be
NULL, and `NULL AS "name"` where a release has no source for a column. D6,
D51 and `NULLS.md` hold it. Do not port the four patches.

### The shared reader instance hazard

`informationschema.New(opts...)` builds one `*InformationSchema` value and
returns a closure over it. Every connection for that driver shares the one
configured value. `mysql.NewReader` is a package level variable built that way.

This caused a real fault. The duckdb driver registered MySQL's completer, which
carried MySQL's reader, which was configured with
`WithCurrentSchema("COALESCE(DATABASE(), '%'))")`. Every completion ran a MySQL
function against duckdb. Commit `3cd55cc` fixed it. No test caught it.

Two rules follow for `dbmeta`. Configuration must not live in a package level
variable that one driver can borrow from another. Composition of readers must
be explicit, not implicit through a shared default.

### The Bool type needs its String method

`metadata.Bool` is a named string type. A Go type switch does not match a named
type to its underlying type, so `tblfmt` printed `"YES"` with quotation marks in
`\d` output for every driver. Commit `cd8edc9` added `func (b Bool) String()
string` at `usql/drivers/metadata/metadata.go:362`. `dbmeta` did not keep
`Bool`: its fields are `bool` or `sql.Null[bool]`.

## External review

Two models reviewed this plan on 2026-09-24: Gemini 3.1 Pro and DeepSeek V4.
They read the same brief and did not see each other's answers. This section
records what they said, and what this project does about it.

Treat their output as an opinion, not as an instruction. Two of their claims
were wrong and are marked below.

### Both reviews agreed on four points

Add `context.Context` to every method. Verified, correct, and now decided. See
D17.

Hold no global state. Bind a reader to one connection. Both reviews pointed at
the DuckDB fault as evidence. This confirms the rule already written under the
shared instance hazard.

Distinguish "not supported" from "found nothing". An empty list must mean that
the database has no such object. It must not mean that the driver cannot ask.
D34 settled this in favor of a capability report.

Use nullable field types for every optional column. This confirms D6.

### Both reviews attacked D8, and their alternative is worth taking

Both called discrete packages per database version a combinatorial explosion.
Both said to follow what `psql` does and resolve the version at run time.

DeepSeek gave a concrete form that fits this project better than either
extreme. Generate the version differences as data rather than as packages:

```go
[]Fragment{
    {MinVersion: 120000, SQL: ...},
    {MinVersion: 150000, SQL: ...},
}
```

One package per driver holds the fragments. A selector merges them against the
detected server version. This keeps what D8 asks for, which is a model that
varies by version, and it drops what D8 costs, which is a package per release.

It also answers three open questions at once. Question 3 disappears, because
there is no version directory to name. Question 4 disappears, because data
needs no inheritance. Question 1 gets simpler, because the adapter has one
package per driver to live in.

Ken took it. D8 now holds the version fragments as data in one package, and
`Stmt.Build` merges them.

### Both reviews attacked D9, and both partly misread it

Both said that ranking PostgreSQL first over fits the API to one database.
DeepSeek wrote that users of other databases get "PG-shaped wrong answers".

The concern is real but the reading is too strong. D9 makes PostgreSQL the
reference for the shape and the behavior of an object that two databases both
have. It does not require every database to answer every kind, which was 48
then and is 56 now. The
capability mechanism covers the gap.

One phrase invited the misreading. "When they disagree, follow PostgreSQL" read
as a rule about every field. Ken agreed with this assessment and D9 now carries
the narrower wording. The ranking decides the shape of an answer. It does not
decide which questions a database must be able to answer.

### Both reviews attacked D7 on tests, and the container half is already answered

Gemini said to allow `testcontainers-go` and `google/go-cmp` in tests. DeepSeek
called stdlib only testing dogmatic.

Half of this objection is already answered. Containers are started by `dbrun`,
which drives the podman or docker command line, so no Go container library is
needed. See D70. The `usql` tests use `ory/dockertest` only because they predate that
choice.

The other half is real but small. Comparing a large metadata struct without
`go-cmp` means writing the comparison and the diff by hand. That is a cost, not
a blocker.

### Two claims from the reviews are wrong

Gemini called `dbtpl` generating `dbmeta` a cyclic dependency. It is not.
`dbtpl` is a build time tool, pinned under D11. The runtime dependency runs one
way, from `dbtpl` to `dbmeta`. This is bootstrapping, which is ordinary.

Gemini said to abandon `dbtpl` and hand write the queries with `go:embed`. That
contradicted D2, D10 and D11, which Ken had decided. The review did not know
that. D71 later made both points moot: nothing here is generated, and the
models are written by hand.

### Points raised that the plan did not cover

DeepSeek raised these. Four are now answered and one is answered in part, as
each says.

1. An error taxonomy. Separate "not supported" from "permission denied", from
   "connection failed", and from "version too old". The `usql` readers return
   one undifferentiated error today, and the oracle privilege pull requests
   exist because of it. D34 and D63 answer it, with `ErrNotSupported`,
   `ErrVersionTooOld` and a query that is not built for a dialect.
2. Reproducible generation. D12 generates against a live container. Most
   `usql/contrib` configs name an untagged image, so the same command run twice
   can read two different servers. Pin the image digest and record which digest
   produced which model. D71 answers it, because nothing is generated, and
   D88 pins the digest of an image that somebody other than the vendor built.
3. Identifier handling. Quoting, case folding, reserved words, and the maximum
   identifier length differ per database and the plan says nothing about them.
   D127 answers quoting and D143 answers case folding. Reserved words and the
   length are still open.
4. Visibility. `information_schema` hides objects that the connected user
   cannot see, so two users get two answers from the same query. Metadata tests
   must fix the user, and the API must say which user it reflects. D61
   answers it: every query is measured as each kind of principal, and the
   differences are recorded.
5. Repeated queries. A caller that asks for the columns of 200 tables must not
   send 200 queries. Decide whether the API batches, caches, or leaves this to
   the caller. D33 answers it: the caller issues one query with a schema
   filter, so there is nothing to batch and nothing to cache.

## Plan

Ken set these phases. They replace any earlier ordering in this file.

D13 sets the order within them. Build the models for the primary databases
first, starting with PostgreSQL and then MariaDB. Write the root `dbmeta`
package after those models exist, so that the API describes what the databases
can answer instead of guessing at it.

The API surface is the contract that binds all three phases. Phase 1 defines
it. Phase 2 and phase 3 implement the same surface for other databases. Where a
database cannot answer, it reports that it cannot, and it does not change the
shape of the answer.

Read one phrase carefully. "Information schema compatible" describes the API,
not the source of the data. Three of the phase 3 databases have no
`information_schema` at all. They still present the same surface.

### Phase 1. Translate the PostgreSQL queries from the PostgreSQL source

Done. `models/postgres` answers all 56 kinds on 10 to 18, and 51 on 9.6, which
has no publications, publication tables, subscriptions, extended statistics
or partitioned tables.

This phase produces the primary platonic model. Every later phase copies its
API.

Translate from the PostgreSQL source, not from `xo/pgdesc`. The file is
`src/bin/psql/describe.c`. Use `pgdesc` as a second opinion when a translation
is unclear, and remember that it is old and that its `TODO` file lists seven
known faults.

Size of the work, measured on the local checkout:

- `describe.c` is 7400 lines.
- It holds 49 entry points. They are the `describe*`, `list*`,
  `permissionsList` and `objectDescription` functions.
- It holds 68 version gates of the form `pset.sversion >= 110000`. They span
  release 11 to release 19.

These numbers moved when the checkout was updated, and one of the moves is not
a drift. Release 20 removed every `psql` code path for a server below release
10, in commit `831bec45924` on 2026-07-02. The gates below 11 are gone from the
current source. `QUERIES.md` part 2 explains what that does to D20, and the
short version is that the current tree can no longer tell you what release 9.6
needs.

The version gates are the work, not a detail. `psql` writes one query and
switches fragments on the integer server version. D8 does the same: each gate
becomes a version fragment in the one model.

The local checkout at `/home/ken/src/postgres` sits at
`REL_19_BETA1-1062-gd9de60c5e47` on `master`, which is release 20 under
development. The installed client is 18.6.

One checkout is not enough. D20 sets the floor at 9.6, and the current tree has
no code for anything below release 10. Translating the older releases means
checking out a release 15 or older tree and reading `describe.c` there.

Record which tree each fragment came from, beside the fragment. A reader who
cannot tell which source a gate was translated from cannot check it.

Read `SHOW server_version_num` to select the fragments at run time. It returns the
same integer that `describe.c` compares against.

### Phase 2. Build the information schema reader, with MariaDB as the reference

Done. `models/mysql` is the native MariaDB model, and `models/informationschema`
is the shared model.

This phase produces the second platonic model. It must present the same API
surface as phase 1.

Use MariaDB as the reference database. Its `information_schema` is the one to
read against while the code takes shape.

Carry over the variation mechanism from
`usql/drivers/metadata/informationschema`. See D9 for its four parts: the
feature flags, the named clauses, the placeholder function, and the schema
lists. Add the version dimension that D8 requires, and make the description a
value the caller owns rather than a package level variable.

Expect gaps. `information_schema` does not describe most of the objects that
`psql` describes. Report each gap through the capability mechanism. Do not
invent a query that returns a partly filled object.

### Phase 3. Extend to the remaining reference databases

Add these, each presenting the same API surface: SQLite3, DuckDB, Microsoft SQL
Server, Oracle, and Cassandra.

Done. Each has a native model. None extends the shared model: DuckDB reads its
own catalog functions and SQL Server reads `sys`, because each answers more
that way.

As planned, they split into two groups by how much of phase 2 each was able
to reuse.

DuckDB and Microsoft SQL Server have an `information_schema`, and both used
the shared reader in `usql`.

SQLite3, Oracle and Cassandra have no `information_schema`. Each needs its own
queries against its own catalog. `usql` shows what SQLite3 and Oracle use.
SQLite3 reads `sqlite_master`, `sqlite_temp_master`, and the `pragma_*` table
functions such as `pragma_table_info` and `pragma_index_xinfo`. Oracle reads
the `all_*` and `dba_*` views, such as `all_tab_columns` and `all_indexes`.

Cassandra is the hardest of the five and has no reader in `usql` at all. It is
not a relational database, it speaks CQL rather than SQL, and it keeps its
metadata in the `system_schema` keyspace. Treat it as the test of whether the
API surface holds for a database that does not fit the relational model. If it
does not hold, that is a finding to report, not a reason to bend the surface
for the other five.

### Phase 4. Test, measure, and fuzz

This phase is new work. `usql` has no benchmark and no fuzz target anywhere in
the repository. Its metadata tests total about 3300 lines across seven files,
and the largest are `clickhouse_test.go` at 1660 lines and
`informationschema/metadata_test.go` at 1096 lines. Read them for the cases
they cover, not for the harness they use.

Four kinds of work belong here.

Correctness tests run the same root API calls against every supported database
and every supported version. The output shape must match across all of them,
because the API surface is the contract. Compare against golden files, so that
a change in one query shows up as a diff and not as a silent difference.

Version tests protect D8. For each database with more than one model, run the
same call against each version and assert that the caller sees the same types.
Assert also that the fragments resolve for each release, and that a server
older than the floor reports `ErrVersionTooOld` (D63).

Benchmarks measure the cost per call and the cost per row. Metadata queries run
inside an interactive client, where a slow response is visible to a person.
Measure the query, the scan, and the conversion into the root types as three
separate figures, so that a regression points at one of the three.

Fuzz targets take the inputs that come from a person rather than from the
database. The `Filter` values come first, because a caller passes names and
patterns straight into a query. Fuzz the pattern handling, the clause
substitution that D9 describes, and the version parser. A fuzz target must
assert that no input causes a panic and that no input reaches the SQL unquoted.

Split the work by where it runs. D42 and D69 set what CI runs: the Tested tier
on every push and the Nightly tier once a night. `dbrun test` runs any of them
on a development machine with one command.

One warning about dependencies. The `usql` metadata tests import
`github.com/ory/dockertest/v4` and `github.com/google/go-cmp/cmp`. D7 forbids
both here. Start the databases with `dbrun`, which D68 requires, and compare
with `reflect.DeepEqual` or with a written comparison. Do not copy the `usql` test harness.

### Phase 5. Integrate into usql

Change `usql` to import `dbmeta`, then delete the moved code from `usql`.

D5 keeps `writer.go` in `usql`. That file is 838 lines and it is the only one
that uses `tblfmt` and `usql/env`. Leaving the writer behind is what keeps
those two out of this module. It also uses `dburl` across the eight `Writer`
methods at lines 148 to 162, which does not matter here, because `dbmeta`
never parses a URL (D99).

Coordinate through the `usql` session. It owns that repository and has agreed
to report before any of the five open pull requests against `drivers/metadata`
lands. Check that list again before you start, because a merge during this
phase changes code that you are deleting.

Settle the completion path in this phase. `usql/drivers/completer` holds a
reader, and it drove the fault described under the shared instance hazard.
Decide whether completion reads through the `dbmeta` API or keeps its own path.

### Phase 6. Expand to the other databases

Add the databases that `usql` supports and that the earlier phases did not
cover. D66, with the decisions that amend it, holds the order, and every
database it names is done. `EVALUATION.md` lists the candidates that are
left.

## Testing plan for versions and flavors

D8 adds a version axis. D14 adds a flavor axis. A model must be tested on both,
and the two axes multiply, so the plan must keep the matrix small on purpose.

### The two axes

A version is the same product at a different age, such as PostgreSQL 13 and
PostgreSQL 18.

A flavor is a different product that claims the same driver, such as MariaDB
and MySQL under the `mysql` driver.

Neither axis contains the other. A query can work on every MariaDB release and
fail on MySQL 8. A query can work on both products at release 8 and fail on
both at release 5.

### dburl holds the flavor taxonomy. Import it.

Do not invent a list of flavors and do not copy one. D19 makes
`github.com/xo/dburl` the place the taxonomy lives, so read it from there
rather than copying it here. D19 records why `dbmeta` does not import it.
`dburl` separates two kinds of flavor and puts them on different fields of a
parsed URL. See D19 for which field carries which.

An alias is a product that `dburl` treats as the same driver. The `mysql`
scheme carries the aliases `mariadb`, `maria`, `percona`, and `aurora`. The
`sqlserver` scheme carries `azuresql`. The `oracle` scheme carries `ora`,
`oci`, `oci8`, `odpi`, and `odpi-c`.

A wire compatible is a separate product that speaks another product's protocol.
Until v0.36.0, `dburl` recorded a parent for each one, and gave it the
parent's dialect. From v0.36.0 each has a dialect of its own (dburl D37,
D125). The parents were these:

- `cockroachdb` and `redshift` have the parent `postgres`.
- `memsql`, `tidb` and `vitess` have the parent `mysql`.
- `oleodbc` has the parent `adodb`.

The taxonomy sets an expectation to test, not a promise that holds. CockroachDB
claims the PostgreSQL protocol and does not implement the whole PostgreSQL
catalog. Redshift forked from PostgreSQL 8.0. Each one is a test case that can
fail, and a failure is a finding to record rather than a fault to hide.

### Which combinations to run

Every release sits in one of five tiers (D40, D42, D119). Tested runs in CI on
every push, and Nightly runs once a night. Verified runs on a development
machine before a release and never in CI. Staged is a release that `dbrun`
starts and no model reads yet, and CI never runs it. Archived has no tests. `container/` holds
the list, and the workflow reads it through `dbrun list --json --names`
(D69). CI compiles the tests once and every job runs the binary (D82). The
embedded databases run in the same matrix and start nothing. A separate job
compares MariaDB with MySQL (D44). CockroachDB, CrateDB, TiDB and Vitess
have models of their own. CockroachDB's shares most of the postgres model's
statements, and TiDB's and Vitess's most of the mysql model's. Redshift is a
hosted service with a dialect of its own (D117, D118, D123, D125, D133,
D135).

The Verified tier must run before a release (D40). D64 checks that each
Verified release is documented.

### What a test asserts

Every tier asserts the same three things.

1. The call returns without an error, or returns a "not supported" error that
   names the object. It never returns an empty list to mean "cannot ask".
2. The shape of the result matches the golden file for that object. The shape
   is the contract, and it must not change with the database.
3. No field that the database can return as NULL reaches a caller as a zero
   value that is indistinguishable from a real value.

One more thing is asserted across releases. Given a server version, the
fragments resolve to the newest one that is not newer than the server, and a
server older than the floor reports `ErrVersionTooOld` (D63).

### Requirements on the container harness

D12 first planned to reuse the podman configuration in `usql/contrib`. `dbrun`
replaced it (D68, D70): it starts every server from `container/`, each entry
names its tag, and an image that somebody other than the vendor built is also
pinned by digest (D88). MySQL and MariaDB each have their own entries.

### Fixing the user, and why it matters

`information_schema` hides the objects that the connected user cannot see. Two
users therefore get two answers from the same query against the same database.
Every test must connect as a named user with a fixed set of grants, and the
golden files must record which user produced them.

This is not a detail. Two of the five open pull requests against
`usql/drivers/metadata` exist because an Oracle query needed a privilege that
an ordinary user does not have. D61 now measures every principal of every
dialect.

## Open questions for Ken

An open question lives here until it is answered, and then it becomes a
decision in `decisions/` (D111). The argument behind a decision belongs with
the decision, which is why there is no separate document for it.

No question is open. The four that were are answered in D128, D129, D130 and
D131.

None of the older questions is open.
The floor question that the upstream change reopened has been answered:
D20 keeps 9.6, and D40 adds the tiers and the removal trigger that the review
asked for in exchange.

Everything else raised in this document has been answered, and every decision
has a status in its file.

Nothing is deferred either. Both of the items that were are closed, and each
is worth a line here because both were read as live design space after they had
stopped being it.

D4 and question 4 as it was: whether `dbmeta` exports interfaces at all, and
under what names. It waited for D13 to deliver the models, and D13 delivered
eight. The root package exports `Queryer` and `AnyQuery` and no per object
interface, an object kind is a `Query` value, and D49 settled the one that
remained. D4 says so and points at the decisions that hold the current shape.

D35 was listed here and it is not deferred any more. D48 answered it: cgo is
allowed in the `test` module, so DuckDB is reached through its driver like
every other database. The question was worded as "how is DuckDB reached, given
pure Go only", and the answer is that pure Go only was never a constraint on
the `test` module. It is a property of the root module, which has no driver in
it at all.

D60 was the last one and it is answered. D61 measured what a principal
actually gets, and an Oracle local user that owns the objects receives the
administrator's answer to every query, so there was no gap to close. The model
stays `ALL_` only.

Raise a new question here rather than deciding one alone.
