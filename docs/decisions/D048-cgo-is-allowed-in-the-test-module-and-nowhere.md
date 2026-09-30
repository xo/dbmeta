# D48. cgo is allowed in the test module, and nowhere else

Status: Amends D26 and D29, supersedes D35.

The root module has no driver and no cgo, and that does not change. A consumer
builds it with `CGO_ENABLED=0` and cross compiles it, because there is nothing
in it but the standard library.

The `test` module may use cgo. Its own `go.mod` keeps it out of everything a
consumer builds, which is the reason the module exists, and that isolation is
as true for a C compiler as it is for a driver version.

## What changed

Hard rule 10 said no cgo anywhere, including the `test` module, and named
`mattn/go-sqlite3` as a driver never to use. That was wrong about SQLite.
`mattn/go-sqlite3` builds the real SQLite source, it is what most people run,
and testing only `modernc.org/sqlite` tests a reimplementation rather than the
database.

## Test both drivers where both exist

Where a database has a canonical cgo driver and a pure Go one, test both. They
are not interchangeable. The cgo driver compiles the upstream source and the
pure Go one is a translation of it, they ship different library versions, and a
consumer that cannot use cgo runs the second. A query that works on one and not
the other is a fault worth finding.

SQLite is the case today: `mattn/go-sqlite3` and `modernc.org/sqlite`. The
SQLite tests run against each in turn, as subtests named for the driver, so a
failure says which one.

## Which drivers need cgo

Two, and only two are known: `mattn/go-sqlite3` and the coming DuckDB driver.
Everything else on the list has a pure Go driver that is the right choice on
its own merits, and using the pure Go one there is not a concession.

`godror` stays banned, and the reason is different from cgo. It needs Oracle
client libraries installed on the machine, not just a C compiler, so it cannot
be built by a contributor who has not first installed a product. `sijms/go-ora`
speaks the wire protocol and needs nothing.

## What CI has to do

The `test` module needs a C compiler, which `ubuntu-latest` has. Nothing in
the root module does, and the unit job must keep building with `CGO_ENABLED=0`
so that the promise about the root module is checked rather than assumed.
