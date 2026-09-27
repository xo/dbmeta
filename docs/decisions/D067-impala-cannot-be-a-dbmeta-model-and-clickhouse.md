# D67. Impala cannot be a dbmeta model, and ClickHouse goes first

Status: Amends D66.

Impala is off the list. It has no catalog a statement can read, and the reader
D66 wanted moved here does not use SQL, so hard rule 1 forbids moving it.
ClickHouse takes first place.

## What the server said

A four container quickstart was stood up at 4.5.2 and asked directly. There is
no `information_schema` and no `sys` database: `SHOW DATABASES LIKE` returns
zero rows for both, and selecting from `information_schema.tables` is an error.
`SHOW` is a statement rather than a relation, so `SELECT * FROM (SHOW
DATABASES) t` does not parse either.

Metadata comes from `SHOW` and `DESCRIBE`, one statement per scope. So dbmeta
could answer three of the 55: `Schemas` from `SHOW DATABASES`, which returns a
name and a comment in one statement, `CurrentSchema` from `current_database()`
and `CurrentUser` from `user()`. `Tables` would need `SHOW TABLES IN` once per
database and `Columns` would need `DESCRIBE` once per table, which is the per
row round trip rule 13 forbids.

## Why usql is not blocked after all

D66 put Impala first because `usql` could not retire its metadata package
until the last of its five readers moved here. That reader does not run a
statement. It calls `GetSchemas`, `GetTables` and `GetColumns` on the driver,
which are HiveServer2 metadata operations in the protocol rather than queries.

Hosting that here would mean importing the Impala driver into the root module,
and hard rule 1 forbids a database driver there outright. So the reader cannot
move, and it is already where it belongs: it is a property of the wire
protocol, which is the driver's business. `usql` keeps it and loses nothing.

## The cost, for completeness

`apache/impala` publishes no whole server. It publishes components, and a
running Impala is four containers, a Hive Metastore, statestored, catalogd and
impalad, on a shared network with a warehouse volume. That is a harness on the
scale of the Windows machines in `test/vm`, in exchange for three queries.

A three query model is a small job if somebody ever wants `\dn` against
Impala, and it buys nothing today and needs the harness anyway.

## What this changes

ClickHouse is first. It is one container that starts in seconds, it is the
only driver in usql's default build with no model, and `system.tables`,
`system.columns`, `system.databases`, `system.functions`, `system.settings`
and `system.data_skipping_indices` are a real catalog rather than an
`information_schema` emulation. The rest of D66's order stands.

## The rule this adds

Before scheduling a dialect, check that the product has a catalog a single
statement can read. A product whose metadata is a protocol operation or a
`SHOW` per object cannot be a model here, however popular it is and however
well it runs in a container. D66 ordered by whether a product could be started
and that was one question short.
