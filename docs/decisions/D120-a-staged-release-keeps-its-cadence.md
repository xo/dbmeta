# D120. A Staged release keeps its cadence

Status: Amends D119.

## The problem

D119 moved every release that no model reads to the Staged tier, and dbmeta's
CI stopped running them. dbimp's CI runs its own drivers against some of those
servers. It chose which releases run on each push and which only at night, and
it read that choice from the Tested and Nightly tiers. Once they were Staged,
`dbrun list --json --names staged` gave dbimp one flat list, and the split was
gone. On its next pin of dbmeta, dbimp's tests for SurrealDB, Neo4j and InfluxDB
would have stopped running while its CI still passed.

dbimp asked on 2026-09-28 how its CI chooses. It offered three answers: mark
each release as a floor, a ceiling or one between them; run every Staged
release on each push; or keep its own list. The first does not fit InfluxDB,
whose push releases are the newest of lines 1 and 2 and the floor and the
ceiling of line 3 (dbimp's D79). The third puts release names in dbimp's
workflow, where they go stale when dbmeta's ranges move.

## The decision

Ken chose on 2026-09-28 that a Staged release keeps a cadence.
`container.Server.Cadence` is the tier the release would have if a model read
it: Tested, Nightly or Verified. `dbrun list --json` prints it as `cadence`.
A project that runs Staged releases runs the tested ones on each push and the
nightly ones at night, and its workflow still names no release.

The cadence is the split that each product's decision already made. D103 gave
SurrealDB 2.7.0 and 3.3.0 on each push and 3.1.6 and 3.2.4 at night. dbimp's
D79 gave InfluxDB its split, which D114 recorded. D118 gave CockroachDB,
TiDB, MongoDB, Elasticsearch and Solr one release each at night. Every other
Staged release is Tested. Stardog, GraphDB and Volt Active Data need a licence
file, and a hosted service needs a secret, so their cadence is Verified.

When a model arrives, the cadence becomes the tier. This replaces the part of
D119 that said the change which adds a model picks the tier. The choice was
made when the release was added, and it is not made again.

Two tests hold it. `TestAReleaseIsStagedExactlyWhenNoModelReadsIt` fails when
a Staged release has no cadence, or a release that is not Staged has one.
`TestEveryStagedTargetHasACadence` in dbrun holds the same for the libraries
and the hosted services.

## What dbimp does

dbimp pins the dbmeta commit that adds the cadence, and not 33e2993, which is
the commit that made its releases Staged. Its `releases` job selects from
`dbrun list --json staged`, keeps the products it has a driver for, and keeps
`cadence == "tested"` on a push and `tested` or `nightly` at night.

## Rejected

A floor and ceiling marker, which dbimp recommended. The cadence says the same
thing for every product it fits, and it also fits InfluxDB. It needs no rule to
work the ends out.
