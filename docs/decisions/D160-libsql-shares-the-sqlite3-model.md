# D160. libSQL shares the sqlite3 model

Status: Amends D153, amended by D167.

## The decision

libSQL is the fork of SQLite by Turso, and sqld is the server that serves it
over HTTP. `models/libsql` shares every statement of the sqlite3 model with
`Query.Share`, as rqlite does (D148), and writes no statement of its own. Its
dialect is `dbmeta.LibSQL`, which is `libsql`, the name dburl gives the
scheme. The driver is `github.com/xo/dbimp/libsql`, which dburl names, from
dbimp v0.10.0 (D154).

It answers 14 of the 56 on 0.24.33, which is Tested, the cadence it recorded
while it was Staged (D120). The conformance report is line for line SQLite's,
and every check the SQLite tests make runs on libSQL too.

## How libSQL maps onto the kinds

libSQL is SQLite, so the mapping is SQLite's. A schema is an attached
database, and on sqld that is `main`. A database is the same thing, read from
`pragma_database_list`. sqld calls a namespace a database too, and
`libsql_server_database_name()` names the one a request uses, but the list of
namespaces is in the admin HTTP API, which no SQL statement reaches. So a
namespace is not a database here. A table, a column, an index, a constraint,
a trigger and a view are SQLite's. A function is a row of
`pragma_function_list`, which on sqld includes about 200 functions that sqld
registers, most of them from the sqlean extensions.

sqld has no user, role or grant in SQL. A token carries the claim rw or ro
(D153), so `Roles`, `Privileges` and `CurrentUser` stay unanswered.

## The vector index

libSQL adds one object that a query reads: the vector index, which
`CREATE INDEX ... (libsql_vector_idx(c))` makes. The shared statements read
it wrongly in two ways. They called it `btree`, and they listed the tables
libSQL keeps for it, `libsql_vector_meta_shadow` and one shadow table for
each index, as tables a person made.

The fix is a fragment for libSQL in two pieces of the sqlite3 model's
statements, which gates on the version key `libsql`. The libsql model
records that key with an unknown version, because no SQL statement names the
sqld release, as the cassandra model records ScyllaDB's (D91). With the key,
the system filter leaves out the tables libSQL keeps for a vector index, and
the type of a vector index is `diskann`. `docs/COVERAGE.md` says how each is
found.

This is a fragment rather than a statement of libSQL's own. Statements of
its own copy eight of the sqlite3 model's statements to change one or two
pieces of each. D123 says to register a statement of your own only where the
shared one does not answer, and here the shared one answers once it knows the
product. A key for one product that shares a model is what hard rule
3 and D44 describe.

## The tests connect with the URL

D153 kept the dsn of the libSQL entry at the `http://` address, which dbimp's
recorder reads, and the url at the `libsql://` form. dbimp's driver takes only
the url, and dbrun set `DBMETA_LIBSQL` to the dsn and connected `dbrun
version` with it.

`container.Server` now has `ConnectURL`, which the libSQL entry sets. dbrun
then sets the variable of the tests to the url and connects `dbrun version`
with it. `dbrun dsn` prints the same as before, so dbimp sees no change. The
Avatica entries have an `http://` dsn for the same reason (D155), and they
can set the field when a model reads them. The coordinator chose this on
2026-10-01 over making the dsn the url, as D151 did for rqlite. That
changes what dbimp's recorder reads.

D167 later removed the flag. The DSN of the entry is now the URL that the
driver takes, and the `http://` address is its `api`.

## What was measured

On 0.24.33, on 2026-10-01, as both principals. sqld refuses `ANALYZE` and
`PRAGMA optimize`, so `sqlite_stat1` and `sqlite_stat4` never exist although
the build has `ENABLE_STAT4`. sqld refuses `CREATE FUNCTION ... LANGUAGE
wasm`. `RANDOM ROWID` and `ALTER COLUMN` work, and neither leaves a fact that
a pragma reports. The ordinary user reads every answer the administrator
reads, so the parity section is empty.
