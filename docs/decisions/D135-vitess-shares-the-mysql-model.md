# D135. Vitess shares the mysql model and names a schema by its keyspace

Status: Decided.

## The decision

Ken asked on 2026-09-29 for Vitess to be built as a dialect of its own, as
dburl's D37 made it. `models/vitess` shares the mysql model's statements where
they answer, the way `models/tidb` does (D133), and writes one statement of
its own, for sequences.

Vitess answers `VERSION()` with `8.4.6-Vitess`, which names only the MySQL
release it claims. Its own release is in `@@version_comment`, as
`Version: 24.0.3`, and the version query reads both. The main version is the
MySQL release, set under the mysql model's `mysql` key too, so that a shared
statement takes the fragments it would take on MySQL 8.4.6. Vitess's own
release is under the key `vitess`.

## A schema is a keyspace

A keyspace is stored in one MySQL database for each shard, named
`vt_<keyspace>_<shard>`, and information_schema names that database and never
the keyspace. Ken first chose on 2026-09-30 that the model reports that name.
It was then measured that vtgate does not accept it in a query. A SELECT from
`vt_dbmeta_q_0.t` failed with VT05003, unknown database, and a SELECT from
`dbmeta_q.t` succeeded, on 24.0.3. So a client could not query a table under
the name the model reported, and Ken changed the choice the same day: the
model reports the keyspace.

`mysql.Keyspace` is the expression that reads the keyspace from the name of
the database. It removes `vt_` and the last underscore and the shard, so
`vt_dbmeta_fixture_0` is `dbmeta_fixture` and `vt_dbmeta_-80` is `dbmeta`. A
name that does not have that form is its own, such as `mysql` or `_vt`. Every
schema that a shared statement selects, and every schema filter, goes through
it on Vitess, and so does the current schema. A test queries every table the
model lists, under the name it lists, and vtgate accepts each one.

The expression depends on the name Vitess gives the database. A tablet started
with `--init_db_name_override` stores a keyspace under another name, and the
model then reports that name, which is what information_schema says.

## The shared statements carry alternatives for Vitess

`_vt` holds the state of a tablet, and the mysql model's schema filter listed
it as a user's. So `notSystem` in the mysql model has an alternative under the
`vitess` key that adds it. `schemaAs` and `schemaLike` have the alternatives
that read the keyspace. A Vitess alternative must never sit in a choice
beside one on the `mysql` key, because a Vitess version set reports both keys
and the two would be ambiguous.

## What is not shared

Roles, role grants and privileges run on Vitess, and are not shared. They list
the accounts of the tablet's MySQL, such as `vt_dba` and `vt_app`, which
Vitess uses itself and no client of vtgate logs in as. vtgate refuses CREATE
TRIGGER, CREATE FUNCTION and CREATE SERVER, so the queries that read those
objects are not shared either.

vtgate refuses ALTER USER, so `ChangePassword` is nil. vttestserver starts
vtcombo with no authentication, so there is no second principal, and Vitess is
exempt from parity (D61). docs/COVERAGE.md has the rest.
