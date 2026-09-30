# Backlog

This document lists work that is known and not done. Each item names the
decision or the measurement that found it.

A decision is not a backlog item. It goes in [`decisions/`](decisions/README.md).
A question for Ken goes at the end of [`PLAN.md`](PLAN.md), under Open questions
for Ken. When an item here is done, delete it, and record in `decisions/`
anything that was decided on the way (D110).

## Drivers

### Move the rqlite tests to the dbimp driver

The test module reads rqlite through `github.com/rqlite/gorqlite`, because
dbimp has no rqlite driver yet, and Ken chose that driver on 2026-09-30
(D148). dbimp started its driver the same day. When it exists, move the
tests and dbrun to it, drop gorqlite from `test/go.mod` and from the
depguard list in `test/.golangci.yml`, and run both releases again. dbimp's
entry takes a url of the form `rqlite://user:password@host:port`. Hard rule
10 asks for the package that `usql` uses, and usql will import dbimp's.

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

### Find why Oracle 11g reads its catalog slowly

With the system objects included, Oracle 11g XE took more than a minute each
for tables, types, privileges and column_stats, measured on 2026-09-29.
18c reads the same queries in seconds. Read the plans on 11g, and find
whether one view or one join is the cost. docs/COVERAGE.md has the timings,
under Oracle.

### Measure SingleStore's correlated statistics

Gemini and Gemini Pro named SingleStore's histograms for extended
statistics (D141). PostgreSQL's extended statistics cover several columns,
and it is not known whether CORRELATED_COLUMN_STATISTICS does. Build correlated statistics in the
SingleStore fixture, read the view, and decide whether it answers
ExtendedStats.

### Find why HANA's count of views moved once

On 2026-09-30 the tier run failed once on `hana-2.00.088`. checkTypes in
`test/scan_test.go` read 896 views, and then 895 when it asked for views
alone. The same test passed when it ran again on its own. Nothing in the
fixture makes a view between the two reads, so HANA made or dropped a view
of its own. Find which one, and whether checkTypes has to leave it out.

### Measure Oracle's column groups for ExtendedStats

D43's pass for Oracle left out `ALL_STAT_EXTENSIONS`, because the view holds
an expression and ExtendedStat had no field for one. `ExtendedStat.Definition`
holds it now (D147). Build a column group in the Oracle fixture, read the
view, and decide what Oracle records for Ndistinct, Dependencies and MCV. If
nothing answers them, the kind stays unanswered, because the three are not
nullable. docs/COVERAGE.md has the first finding, under Oracle.

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
