# D28. Root tests use a fake driver replaying captured data

Status: Decided.

Tests in the root module use a fake `database/sql/driver` implementation
written in a `_test.go` file, replaying captured responses from `testdata`.

This is the right shape and both reviews agreed. `database/sql/driver` is in
the standard library, so the fake costs no dependency and D26 holds without
strain. The tests are fast, they need no container, and they can cover a server
version whose image no longer starts, which matters because D20 supports
PostgreSQL back to 9.6 and those images were last built in 2022.

Note that this is a replay driver, not an expectation mock. It answers with
recorded bytes from a real server. Gemini suggested `DATA-DOG/go-sqlmock`
instead, which is a different tool that asserts which queries were issued. It
is also a third party dependency, which D26 forbids. Do not use it.

## What a replay test cannot catch

Both reviews produced overlapping lists. This matters, because a green test
suite that proves less than it appears to is worse than a smaller one.

A replay test cannot catch:

1. Invalid SQL. The server never parses the query, so a syntax error or a
   missing catalog column passes. This is the largest gap and it is the exact
   class of fault that the `pg_attrdef.adsrc` removal created.
2. Driver level type translation. A driver converts wire types into
   `driver.Value`, which permits only a few Go types. A capture freezes the
   translation that one driver version performed. If a consumer uses a
   different driver, or a later version changes how it surfaces a type, the
   test still passes.
3. Permissions and visibility. `information_schema` hides what the connected
   user cannot see, so the answer depends on the user, and a capture records
   one user.
4. Session state. Real drivers set things on connect, and results depend on
   `search_path`, timezone, character set, collation, `sql_mode` and the Oracle
   and SQL Server equivalents.
5. Driver specific error types and codes, connection pooling, retries on a bad
   connection, and any ordering that the server does not guarantee.

The conclusion is not to abandon the fake. It is that the fake proves the Go
side and proves nothing about the SQL. Live tests in the other module prove the
SQL. Neither replaces the other, and the plan must not let the fast one create
the impression that the slow one is optional.

## Captures must carry their provenance

A capture with no provenance becomes folklore. Every capture records:

1. The database engine, and the exact server version, build and edition.
2. The driver module and its exact version.
3. The connected user and the grants it held.
4. The session settings in effect.
5. The query, its parameters, and the resulting columns, rows or error.
6. The container image digest that produced it, which D12 already requires.
7. The commit of the capture tool, and a checksum.

Detect a stale capture mechanically rather than by review. Key each capture to
a hash of the query that produced it. When the SQL changes and the capture does
not, fail. Re-capture against the live matrix on a schedule and fail on a
normalized difference.

Never hand edit a capture. A capture is a generated artifact. A corpus that
someone has corrected by hand records a server that does not exist.
