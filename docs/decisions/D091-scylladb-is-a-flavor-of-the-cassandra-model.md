# D91. ScyllaDB is a flavor of the Cassandra model

Status: Amends D66, amended by D92.

Ken asked on 2026-09-27 for ScyllaDB to be tested against the `cql` dialect.
`models/cassandra` now reads it. Cassandra is the reference product and
ScyllaDB is the flavor, the way MariaDB and MySQL share `models/mysql`. It
answers 18 of the 55 on ScyllaDB and 17 on Cassandra. D66 did not list it,
and it took no place in that order, because it is not a new model.

## Why a flavor and not a model

ScyllaDB keeps `system_schema` as Cassandra 3.0 laid it out, and the driver is
the same. dburl reads `scylla` and `scy` as aliases of the `cql` scheme, so a
consumer already reaches this dialect with no change, which is what hard rule
1 asks. Of the 17 queries Cassandra answers, 13 read the same tables on
ScyllaDB. A second model copies those 13, and two copies drift apart.

## The releases

`docker.io/scylladb/scylla` is the vendor's own image. Step 2 of
`EVALUATION.md` gives 2025.1 to 2026.3: 2025.1 was rebuilt on 2026-09-01 and
2026.3 on 2026-09-13, and 2025.2 to 2025.4 are no longer rebuilt. 2025.1 and
2026.3 are Tested, and 2026.1 and 2026.2 are Nightly. 6.2 was the last open
source release, and releases from 2025.1 are source available with a free
tier. D90 says that qualifies. The image is not pinned by digest, because the
vendor rebuilds each tag with its point releases, the same as the other
vendor images here.

## How the model finds ScyllaDB

A fragment for ScyllaDB gates on the `scylla` key, which hard rule 3
requires. The key has to come from the one version statement, and that
statement has to run on both products.

The three columns Cassandra's query read cannot tell the two apart.
ScyllaDB's `release_version` is 3.0.8 on every release measured, and 3.0.8 is
also a real Cassandra release. The version query is now `SELECT JSON * FROM
system.local`. It names no column, so it runs on both products, and a row
with the `supported_features` column is ScyllaDB. The main version stays the
Cassandra release that the row reports, because that is the catalog the
server offers.

The ScyllaDB release is in `system.versions`, and Cassandra has no such table.
No statement that runs on both products can read it, so the `scylla` key is
an unknown version. A gate can say "ScyllaDB" and cannot say "ScyllaDB 2026.1
or newer". No query needs that today: all four releases answer every query
the same way. The open question at the end of this file asks what to do when
one does.

`usql` reads the three columns and prints "Cassandra 3.0.8" for a ScyllaDB
server, which names the wrong product. `docs/USQL.md` records it.

## No literal in a select list

ScyllaDB refuses every literal in a select list. 2025.1 refuses `(text)NULL`,
`(boolean)false` and `CAST(false AS boolean)` as syntax errors, and 2026.3
refuses every NULL. The Cassandra model padded about 70 columns that way.

Each padded or fixed column is now a fragment pair. Cassandra selects the
literal, so that its statement still says what the field holds. ScyllaDB
selects a real column of the same table under the same name. Scan discards the
column on both products and sets the known value itself. One Scan reads both,
and hard rule 3 holds, because both statements return the same columns.

The alternative was a Scan that knows which product it reads. Scan receives
only the rows, so it cannot know. Setting a known value in Go is also what
`NULLS.md` asks of this driver already, because the driver cannot report a
null.

## RoleSettings from a service level

`system.role_attributes` holds a value set on a role, and `ATTACH SERVICE
LEVEL` is what sets one. A service level gives a role's sessions a timeout and
a share of the server, so the attribute is per role configuration of the same
kind as `ALTER ROLE ... SET`. D88 accepted Vertica's per user parameters as
role settings on the same argument. The row is per attribute, because CQL
cannot group, and Cassandra reports that it cannot answer.

## The superuser

2026.3 creates no default superuser. The container names one at startup with
`--auth-superuser-name` and a salted password, `cassandra` and `cassandra`,
the same pair that Cassandra creates, so that the test helpers log in to both
products. The image's entrypoint writes its arguments into a file that a shell
reads, and the shell expanded each dollar sign in the hash. So the hash is
written with each dollar sign escaped once.

## What a second opinion found

Gemini and DeepSeek agreed that 35 of the 38 unanswered kinds are absent, and
named three leads, which were `Databases`, `Languages` and
`RoutineParameters`. None of them is a source, and `COVERAGE.md` says why for
each. `RoleSettings` was found here, while reading the tables in the system
keyspace.
