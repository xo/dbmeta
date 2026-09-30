# D148. rqlite shares the sqlite3 model

Status: Amended by D151.

## The decision

Ken asked for an rqlite dialect on 2026-09-30. rqlite runs SQLite behind an
HTTP API, so `models/rqlite` shares every statement of the sqlite3 model with
`Query.Share`, as D123 lets a model do. It writes no statement of its own.
Its dialect is `dbmeta.Rqlite`, which is `rqlite`, the name dburl gives the
scheme.

It answers 14 of the 56, on 9.4.5 and 10.3.6, which are both Tested. They
are the releases D112 chose for dbimp, and each took the cadence it recorded
while it was Staged (D120). The conformance report is line for line SQLite's.

## The driver

dburl names `github.com/xo/dbimp/rqlite` as the driver, and dbimp has not
written it. usql imports no rqlite driver. DIALECT.md says to ask Ken when a
driver is missing, and Ken chose on 2026-09-30 to test through the
`database/sql` driver of `github.com/rqlite/gorqlite` until dbimp has one.
The tests and dbrun move to dbimp's driver when it exists, as the Couchbase
tests moved in D101. `docs/BACKLOG.md` holds the move.

gorqlite reads `/status` on open to find the other nodes of a cluster, and an
ordinary user cannot read it, so the tests turn discovery off.

## A boolean arrives as a float64

rqlite answers in JSON, and gorqlite decodes every number as a `float64`,
although rqlite sends the column type `integer`. `database/sql` converts a
`float64` to an integer and never to a `bool`, so every boolean field of the
sqlite3 model failed to scan. The statements were right, and the fault is the
driver's.

The root package now has `NumberAsBool` and `NullNumberAsBool`, beside
`NullAsEmpty`. Each reads a bool, a number or the text of either, and the
sqlite3 model scans its boolean fields through them. The SQLite drivers
return an integer, which they read the same way as before. A consumer that
reaches rqlite through gorqlite gets a working answer too.

## What rqlite adds

rqlite adds users, which it reads from the file that `-auth` names, and no
SQL statement reaches that file. Its SQLite has the user authentication
extension compiled in, with `auth_enabled()`, `auth_user_add()` and the rest,
and `auth_enabled()` returns 0, so the extension holds no user. Roles,
Privileges and CurrentUser stay unanswered. Gemini and DeepSeek were asked
about the 42 unanswered kinds, as hard rule 14 requires. Both said rqlite
adds nothing that SQL can read. `docs/COVERAGE.md` already rejects each SQLite
lead they named but `sqlite_stat4`, and rqlite's SQLite is built without it.

The version is the release of SQLite that the server runs, because every
statement depends on it. No SQL statement names the rqlite release, and only
the HTTP API reports it.

## Parity

The entry declares an ordinary user who can query and execute and nothing
else. rqlite has no grant on a table, so the user reads every answer the
administrator reads, and the parity section is empty.
