# Backlog

This document lists work that is known and not done. Each item names the
decision or the measurement that found it.

A decision is not a backlog item. It goes in [`decisions/`](decisions/README.md).
A question for Ken goes at the end of [`PLAN.md`](PLAN.md), under Open questions
for Ken. When an item here is done, delete it, and record in `decisions/`
anything that was decided on the way (D110).

## Drivers

### Move the Cassandra tests to the dbimp driver

The test module reads Cassandra and ScyllaDB through `github.com/xo/cql`
(D93). Ken decided on 2026-09-27 that `xo/cql` gets no further work, and that
a clean driver in `github.com/xo/dbimp` replaces it. When dbimp has that
driver, move the tests to it and run the Cassandra and ScyllaDB tests on every
release again. Couchbase is the precedent: D101 moved its tests to dbimp's
driver before `usql` imported it, and `usql` followed. Hard rule 10 asks for
the package that dburl names (D154), and dburl names `github.com/xo/cql` for
Cassandra today. So the move waits until dburl names dbimp's driver.

## Servers

### Find why the Hive setup fails on a slow machine

The Hive setup fails on a GitHub runner, soon after HiveServer2 first answers,
with a SerDeException: "proto.class has to be set". It comes from one of the
`PROTO_` tables at the end of the schema script, which dbmeta does not read.
It never failed on a development machine. D105 runs the setup again after a
failure, and D107 made the script safe to run twice, so a retry can get past
it. The cause is not known.

### Read the log when Oracle 21c does not answer

`oracle-21c` did not answer in five minutes on one CI run, and it answers in
under a minute on every other run. The cause was not recorded. D105 now
prints the last 20 lines of the container's log when a server never answers.
When it happens again, read that log and decide whether the entry needs a
change.

### Find why oracle-26ai and QuestDB failed once in CI

On 2026-10-08 the first CI run on 90d0261 failed two jobs, and both passed when
rerun, so neither touches that change. `oracle-26ai` failed in the image's
setup, when it reset the SYSTEM password, with ORA-65048 on `ALTER USER SYSTEM`.
`questdb-9.4.3` failed in `TestQuestDBFixtureObjects` at `questdb_test.go:216`,
with the view `recent` listed and the view `book` missing. The cause of
neither is known. If one fails again, read its log and decide whether the entry
or the fixture needs a wait.

### Find why Databend's first start can fail

The first start of `databend-1.2.948` after the pull of the image did
not answer. The image's bootstrap script starts the query server one second
after the metadata server, and the metadata server was still waiting to become
the leader, so the query server gave up with "cannot connect to
http://0.0.0.0:9191". It did not happen again in four fresh starts (D112). If
it comes back in CI, the entry can start the two servers itself and wait for
the metadata server between them.

### Move the ceilings that move

Three ceilings move faster than the others. Databend publishes a weekly
release almost every day, ArangoDB's release is the newest 3.12 patch, and
Neo4j's is the newest monthly release (D106, D112). When a newer release is
published, replace it in the file in `container/` and in the row in
`EVALUATION.md`.

### Consider a SQL layer for what a product does not answer

This is not a priority (D166). Apache Pinot keeps its catalog only in the
Controller's REST API, and its Broker's SQL has no information_schema, no
SHOW and no DESCRIBE, measured on 1.4 and 1.5 on 2026-10-01. InfluxDB names
its release only in `GET /ping`. A generic SQL layer, in a driver or beside
one, can answer such statements, for example SHOW TABLES, DESCRIBE and
`SELECT version()`, from what the product does expose. If Ken takes it up,
a Pinot model can walk those statements, as Impala's does (D146), and the
InfluxDB models can read the release with a statement.

### Finish the Redshift measurements

Redshift ran once against Redshift Serverless (D182). It has no conformance
target. The SVV views that filter by user are not read, so privileges, role
grants and collations are open, and Spectrum external tables are not measured.
`ChangePassword` cannot run, because Redshift forbids quotes, slashes, at
signs and spaces in a password. The identity flag `a` of `columns.identity` is
a reading of the documentation and was not measured. A run reads about 36,000
columns and costs trial credit, so keep each pass small.

### Measure Snowflake parity

`models/snowflake` ran against a trial account (D190). Parity is not measured,
because the role of the test account holds no CREATE USER or CREATE ROLE grant
and so cannot make a second principal. When Ken grants both to a role the
tests use, add a parity target with an owner, a grantee and a stranger, as
Redshift has (D182), and remove the Snowflake entry from `parityExempt`. The
same run can measure `ChangePassword` on a user the test makes, and a
conformance target.

## Consumers

### Move the rules of the products with no model out of usql

ODBC, csvq, Athena and Cosmos keep rules of their own in usql's drivers,
such as ODBC's usql_trim, csvq's SHOW routing and the trailing semicolon
Athena and Cosmos strip, and have no model here, so D143's Info fields do
not reach them. Ken decided on 2026-09-30 that this waits for a larger
restructuring of usql, whose aim is to strip every database specific helper
out of usql/drivers. dbmeta then needs a form for a dialect with rules and
no queries.

### usql reads metadata through dbmeta

usql's W21 moved its describe commands, its version and \password onto
dbmeta, and its W31 wires up every psql describe command in
[`COMMANDS.md`](COMMANDS.md). Both are staged in usql and not committed.
When W31 lands, update the column "usql today" in COMMANDS.md, and what
[`USQL.md`](USQL.md) says usql answers.

### Move usql's \copy statements here

usql's \copy builds an INSERT with the dialect's placeholder, and probes the
columns of a table with SELECT * FROM t WHERE 1=0. They are the last SQL in
usql's drivers. usql's W29 records moving them to dbmeta, and Ken asked on
2026-09-30 for it to wait for a design. It is not a read, so it needs a
decision beside D56 before any of it lands here.

### dbtpl reads metadata through dbmeta

[`DBTPL.md`](DBTPL.md) holds which of the nine reads `dbtpl` needs each model
answers, and whether `dbtpl` can generate from each database. `dbtpl` does
not read `dbmeta` yet.
