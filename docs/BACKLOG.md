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
the package that `usql` uses, so ask Ken whether the move waits for `usql`.

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

### Try an ordinary user on the standalone Avatica server

The standalone Avatica server checks no user, and passes the user and the
password of each connection to HSQLDB, which has users. A user that SA made
through the server was then not found by a second connection, and the
requests were built by hand (D113). Build them with the Go driver
apache/calcite-avatica-go instead, and find whether HSQLDB then checks the
user. If it does, the entry can make one.

### Answer Triggers on InfluxDB 3

A processing engine trigger whose specification is `table:<name>` runs on
each write to that table, and `system.processing_engine_triggers` lists it
(D152). The influxdb entry configures no plugin directory, and the server
refuses a trigger with HTTP 400 without one. Give the entry a plugin
directory and a plugin that does nothing, make a trigger in its setup, and
measure whether the model can answer Triggers from the view.

### Build the Apache Druid model

Ken asked for a Druid dialect on 2026-10-01 and chose to wait. dburl has no
druid scheme, and nothing decides whether Druid is reached as avatica
through calcite-avatica-go, or by a driver of dbimp's own on Druid's SQL
API (dbimp D74). When Ken and dbimp settle the name and the driver, build
the model. dbrun starts Druid 36.0.0 and 37.0.0 already (D113).

### Build the Apache Pinot model

Pinot 1.4 and 1.5 keep their catalog only in the Controller's REST API. The
Broker's SQL has no information_schema, no SHOW and no DESCRIBE, and the
Controller of 1.5.1 has no /sql/ddl, measured on 2026-10-01. dbimp's driver
would have to answer metadata statements such as SHOW TABLES and DESCRIBE
from the Controller, and its DSN does not name the Controller today (dbimp
D129). Ken has not decided that work in dbimp. When a release of the driver
has such statements, build a model that walks them, as Impala's does (D146).

### Run the Snowflake and Redshift models

`models/snowflake` and `models/redshift` were written from the vendors'
documentation and have never run (D144). When a person provisions a
connection string that dbrun resolves (D117), run their tests, fix each
statement that fails, write their parity targets, and remove "Written, not
run" from README.md and docs/COVERAGE.md.

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
