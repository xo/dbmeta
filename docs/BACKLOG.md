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

### Take the dburl release after v0.35.0

dburl commit 62a1de0 on main changes three URLs that dbrun hands out. It is
not tagged yet. When it is tagged, move the test module to it and change
these:

1. Spanner (dburl D35). The URL is now
   `spanner://host:port/project/instance/database`, and a query option passes
   to the driver. `container/spanner.go` prints the old form,
   `spanner://admin@dbmeta/dbmeta/dbmeta`, which the release refuses. Print
   `spanner://127.0.0.1:<port>/dbmeta/dbmeta/dbmeta?usePlainText=true`, which
   reaches the emulator without `SPANNER_EMULATOR_HOST`.
2. GizmoSQL (dburl D36). `gizmosql://` has the dialect `gizmosql` and opens
   the flightsql driver, on the default port 31337. `container/gizmosql.go`
   prints `flightsql://` today.
3. QuestDB (dburl D36). `questdb://` has the dialect `questdb` and opens pgx
   on 8812, the PostgreSQL port of QuestDB. `container/questdb.go` publishes
   only 9000, the HTTP port, so dbrun cannot give a `questdb://` URL until it
   publishes 8812 too.

`cratedb://` no longer adds the port 5432, so pgx reads `PGPORT` when a URL
names no port. dbrun always names the port, so nothing changes for it.

## Consumers

### usql reads metadata through dbmeta

[`USQL.md`](USQL.md) holds what `usql` answers today and what changes when it
reads `dbmeta`, under How usql would use dbmeta. No `usql` command reads
`dbmeta` yet.

### dbtpl reads metadata through dbmeta

[`DBTPL.md`](DBTPL.md) holds which of the nine reads `dbtpl` needs each model
answers, and whether `dbtpl` could generate from each database. `dbtpl` does
not read `dbmeta` yet.
