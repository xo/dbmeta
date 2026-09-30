# D133. TiDB shares the mysql model

Status: Decided.

## The decision

Ken asked on 2026-09-29 for TiDB to be built as a dialect of its own, as
dburl's D37 made it. `models/tidb` shares the mysql model's statements where
they answer, the way `models/cockroachdb` shares the postgres model's (D123),
and writes a statement of its own only where TiDB's catalog differs.

TiDB answers `VERSION()` with `8.0.11-TiDB-v8.5.8`. The version set's main
version is the MySQL release TiDB claims, 8.0.11, and it is set under the
mysql model's `mysql` key too, so that a shared statement takes the fragments
it takes on MySQL 8.0.11. TiDB's own release is under the key `tidb`.

## A shared statement can carry an alternative for TiDB

Two things in the mysql model's statements differ on TiDB and nowhere else,
and copying every statement for them copies about sixteen. So the mysql
model holds an alternative for TiDB, under the `tidb` key, which no server that
model reads reports:

- The schema filter. TiDB spells INFORMATION_SCHEMA and PERFORMANCE_SCHEMA in
  capitals and compares a schema name with its case, and METRICS_SCHEMA, with
  637 tables, is its own. The filter written for MySQL listed all of them as a
  user's. `notSystem` in the mysql model writes both alternatives.
- The connection limit of a user. `mysql.user` has no `max_user_connections`
  before TiDB 8.5, measured on 7.5.8 and 8.1.2, and TiDB has no limit per user
  there, so the value is 0, which is what MySQL writes for no limit.

A TiDB alternative must never sit in a choice beside one on the `mysql` key,
because a TiDB version set reports both keys and the two are ambiguous.

## What TiDB writes on its own

- Settings, from `information_schema.variables_info`, because TiDB has no
  `performance_schema.global_variables`.
- Sequences, from `information_schema.sequences`, because the mysql model reads
  sequences only on MariaDB.
- Privileges, from 8.5. `information_schema.TABLE_PRIVILEGES` is empty before
  it, although `mysql.tables_priv` holds the grant, so the query is too old
  there rather than answering no rows.

TiDB has no stored function, procedure or trigger, and no foreign server, so
those queries are not shared. The fixture is the MySQL fixture less the steps
that build them, with a sequence, a role, a user and the grants between them
added. docs/COVERAGE.md has the rest.
