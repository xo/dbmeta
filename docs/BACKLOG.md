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
driver and `usql` imports it, move the tests to it. Then run the Cassandra and
ScyllaDB tests on every release again, as D101 did for Couchbase. Hard rule 10
requires the package that `usql` uses, so the move waits for `usql`.

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

### Move the Neo4j ceiling each month

The Neo4j ceiling is the newest monthly release, 2026.09.0 (D106). A monthly
release stops being rebuilt when the next one arrives. When 2026.10 arrives,
replace 2026.09.0 in `container/neo4j.go` and in the row in `EVALUATION.md`.

## Consumers

### usql reads metadata through dbmeta

[`USQL.md`](USQL.md) holds what `usql` answers today and what changes when it
reads `dbmeta`, under How usql would use dbmeta. No `usql` command reads
`dbmeta` yet.

### dbtpl reads metadata through dbmeta

[`DBTPL.md`](DBTPL.md) holds which of the nine reads `dbtpl` needs each model
answers, and whether `dbtpl` could generate from each database. `dbtpl` does
not read `dbmeta` yet.
