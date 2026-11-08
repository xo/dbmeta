# D281. Only the core products run in CI

Status: Amends D42.

## Decision

CI runs the core products and nothing else. The core products are PostgreSQL,
MySQL, MariaDB, SQL Server and Oracle, and the embedded databases SQLite,
moderncsqlite and DuckDB. Every other release is Verified. A person runs it on a
development machine before a release, and CI never starts it.

This moves the releases of ClickHouse, CockroachDB, TiDB, SingleStore, Vitess,
SAP HANA, Vertica, Trino, Presto, Firebird, Cassandra, Hive, Impala and the
others from Tested and Nightly to Verified. Their models and their tests do not
change. Only the tier changes.

## Reasons

A product that is not core fails in CI for reasons that have nothing to do with
a query. The image is large, the server needs a license or a long start, or the
registry is slow. A failure of that kind hides a real failure in a core product.

GitHub limits the minutes and the concurrent jobs of an account, and it can
lower them. A matrix of 90 jobs spends both for products that few people run.

## Also changed

A newer push to a branch cancels the run that is still going for that branch.
A scheduled run is never canceled. The two server matrices run eight jobs at a
time, so they cannot take every concurrent job of the account. Each of those
jobs stops after 45 minutes.

## What a Verified release must have

`docs/COVERAGE.md` names every Verified release, and
`TestEveryVerifiedReleaseIsDocumented` fails when it does not. Run
`dbrun test <release>` for each one before a release, and record the result.
