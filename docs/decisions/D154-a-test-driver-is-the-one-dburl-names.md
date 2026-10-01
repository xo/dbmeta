# D154. A test driver is the one dburl names

Status: Amends D52 and D80, amended by D157.

## The decision

D52 made the test driver for each database the one `usql` imports. Ken
decided on 2026-10-01 that dburl names it instead. `usql` is downstream of
`dbmeta`, and `dburl` is upstream of both, so the registry in `dburl`, and
not what one consumer imports, says which driver a dialect is reached with.

The `test` module imports, for each dialect, the package that
`Scheme.GoPackage` names for each dburl scheme whose `Dialect` is that
dialect. Where two schemes of one dialect name two packages, the tests run on
both, as subtests named for the driver: `postgres` and `pgx` for PostgreSQL,
and `sqlite3` and `moderncsqlite` for SQLite. `Scheme.RequiresCGO` says
whether a package needs a C compiler, which the `test` module allows (D48).

The version is not in the registry, and the `test` module picks its own, as
every module does. It no longer reads the version from `usql`'s `go.mod`,
which D80 asked for.

`usql` reads the same registry. A difference between what `usql` imports and
what dburl names is a fault in `usql` or in dburl, and the fix goes to the
project at fault. It is not a reason for the tests here to follow `usql`.

## What stays

The reason for D52 stands. A metadata query touches exactly what differs
between drivers, the type of a value and what happens to a NULL, so a query
that works on one driver and fails on the one a consumer opens is a query
that does not work. The consumer's driver is the one dburl names.

The exceptions stand, and each has its own decision:

- Oracle is tested on the `go-ora/v3` that dburl names, at the commit that
  fixes its panic on 11g and 18c, because no tag holds the fix yet (D157).
  This was an exception until D157. It is listed here because the pin is
  not a tag.
- `godror` is not tested. dburl names it for the scheme godror, and it needs
  Oracle's client libraries rather than only a C compiler (D48).
- `mymysql` is not tested, on the measurement in D52: it reaches neither
  MySQL release and cannot build the fixture on MariaDB.
- Parity runs on the first driver of a dialect alone (D52).

If dburl names a driver that does not exist yet, ask Ken. rqlite was the case:
dburl named dbimp's driver before dbimp wrote it, Ken chose gorqlite for the
meantime (D148), and the tests moved to dbimp's driver when it was released
(D151).

## What was measured

On 2026-10-01 every driver the `test` module imports was compared with the
`GoPackage` of each scheme of its dialect in dburl, for all 28 dialects a
model reads. They match, apart from the exceptions above. So the decision
changes the rule and none of the drivers.
