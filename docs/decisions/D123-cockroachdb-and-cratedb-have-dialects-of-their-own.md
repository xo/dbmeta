# D123. CockroachDB and CrateDB have dialects of their own

Status: Amends D112 and D118.

## The decision

Ken decided on 2026-09-29 that CrateDB is reached as a PostgreSQL wire
compatible product, with pgx, and that dbimp writes no CrateDB driver. He
asked for dbmeta to read the PostgreSQL wire compatible products, CockroachDB
first and then CrateDB.

Each has a dialect of its own, `cockroachdb` and `cratedb`, and a model package
of its own. dburl's D30 gave them those dialects, and dburl v0.35.0 carries it.
Ken chose this over making both flavors of the postgres model, which was
drafted first, on the same day.

## A model shares what answers

CockroachDB imitates PostgreSQL's catalog, so most of the postgres model's
statements answer on it unchanged. A model copies none of them.
`Query.Share(from, to)` registers for one dialect the binding that another
dialect registered, and the model imports the model it shares from, so that
the other registers first. The model registers a statement of its own only
where the other's does not answer, and it does not register one where no
statement can.

The version set makes the shared statements choose the right fragments. The
main version is the PostgreSQL release the product claims, which is
`server_version`. The product's own release is under a key of its own, such
as `cockroachdb`, and a fragment of the model's own gates on that key. The one
version statement is `version()` beside `current_setting('server_version')`.

## What was measured

On 2026-09-29 each release was started through dbrun and read with pgx.

| Release | `version()` | `server_version` | Postgres statements that ran as they were |
| --- | --- | --- | --- |
| CockroachDB 24.3.36 | CockroachDB CCL v24.3.36 | 13.0.0 | 49 of 55 |
| CockroachDB 26.2.7 | CockroachDB CCL v26.2.7 | 13.0.0 | 50 of 55 |
| CockroachDB 26.3.2 | CockroachDB CCL v26.3.2 | 18.0.0 | 51 of 55 |
| CrateDB 6.4.5 | CrateDB 6.4.5 | 14.0 | 11 of 55 |

The number a product claims says nothing about its release. CockroachDB 26.2
claims PostgreSQL 13 and 26.3 claims 18. What it does say is which catalog the
release imitates, and that is what the shared statements gate on.

## CockroachDB

`models/cockroachdb` answers 54 of the 55 on all three releases. 48 are the
postgres model's statements, 6 are its own and `column_stats` is not
answered. `docs/COVERAGE.md` says what each lacks. The fixture is the
PostgreSQL fixture step by step, less the four steps CockroachDB cannot build.
The tests of `test/postgres_test.go` run against it on pgx. The conformance
report is PostgreSQL's line for line, and parity finds what PostgreSQL finds.

With `with_system` off, the shared statements hide what PostgreSQL keeps for
itself and not `crdb_internal`, so CockroachDB's virtual tables are listed.
That is what `psql` shows. Whether to hide them is open, and it is at the end
of `docs/PLAN.md`.

The entry takes the dialect, and each release its cadence as its tier (D120):
26.2.7 and 26.3.2 on every push, and 24.3.36 at night.

## CrateDB

The CrateDB entry publishes 5432 rather than 4200, and its connection strings
are `postgres://` URLs. Its check and its setup still run crash on 4200 inside
the container. crate has no password, and its URL writes an empty one,
`crate:@`, because usql read `crate@` as the user `postgres`. That was a fault
in dburl's passfile package, which dburl's D31 fixed.

CrateDB answers 11 of the postgres model's 55 statements. The other 44 fail on
relations and functions of `pg_catalog` that it lacks, such as `pg_cast`,
`pg_operator`, `pg_trigger` and `pg_get_viewdef`. pgx works for the extended
protocol, parameters, prepared statements, pipelined batches and the types
measured. Transactions do not work on either driver, because CrateDB does not
parse ROLLBACK and lib/pq's BEGIN fails. COPY and LISTEN are refused.

Of those 11, 3 answer correctly on both releases and are shared: settings,
role grants and the current user. The others answer wrongly or fail on 6.3. A
schema's owner reads "unknown (OID=0)", the system filter does not hide `sys`
and `blob`, a generated column reads as an ordinary one, and 6.3 has no
`pg_get_constraintdef`. So `models/cratedb` reads
CrateDB's own catalog for 23 statements, from `information_schema`,
`pg_catalog` and `sys`. It answers 26 of the 55 on 6.4.5 and 25 on 6.3.7,
which has no collation view. `docs/COVERAGE.md` says what each lacks and why
the other 29 are not answered.

A statement of the model's own gates on the key `cratedb`, because both
releases claim PostgreSQL 14.0. `constraints` reads a definition from 6.4,
which added `pg_get_constraintdef`, and `collations` answers from 6.4.

The fixture builds every object on both releases. Conformance agrees with the
relational databases, except for the foreign keys and the unique constraint
that CrateDB does not have. Parity found that a user who is not a superuser is
refused the `sys` schema, so `roles` reads `pg_roles`, and `privileges` is
refused for that user, which `docs/COVERAGE.md` records. The entry takes the
dialect, and both releases keep their cadence as their tier, Tested (D120).

## Rejected

A flavor of the postgres model for each, with a product key. It was drafted,
and dburl's D30 and Ken chose dialects of their own. A second product in one
model would have needed every version gate of the postgres model to name the
PostgreSQL key, because CockroachDB 26.3 claims 18 and a gate on the number
alone would treat it as PostgreSQL 18.

A copy of the postgres statements in each model. 48 statements answer on
CockroachDB as they are, and a copy would drift from the original.

dbimp's HTTP driver for CrateDB, which dbimp's D73 and D76 planned. The HTTP
interface builds each whole result in memory and has no paging, and pgx
already handles the protocol, the parameters and the types.
