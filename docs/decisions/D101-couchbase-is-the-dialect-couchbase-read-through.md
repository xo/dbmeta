# D101. Couchbase is the dialect couchbase, read through the dbimp driver

Status: Amends D95.

dbimp v0.1.0 shipped its Couchbase driver, `github.com/xo/dbimp/couchbase`, on
2026-09-27, and Ken asked for it to be swapped in. D95 held the Couchbase
model for this driver.

## The dialect is renamed

dburl v0.33.0 points its Couchbase scheme at the new driver. The scheme is now
`couchbase`, with `n1ql` and `n1` as aliases, and its `Dialect` is
`couchbase`. `dbmeta.Dialect` is the dburl driver name, so `dbmeta.N1QL`,
which was `n1ql`, is now `dbmeta.Couchbase`, which is `couchbase`. It is the
same kind of fix as D81, which renamed the Cassandra dialect to `cql` for the
same reason. Nothing read the old constant but the container entry, because
dbmeta has no Couchbase model yet. The environment variable of the tests is
`DBMETA_COUCHBASE`, which follows from the name.

## The connection string

The driver takes `couchbase://user:password@host:8093/`, which is also the
dburl form, so the DSN and the URL of a Couchbase server are one string. The
`http://` DSN that `go_n1ql` took is gone. `dbrun` now has a driver for the
dialect, so `version` and a readiness probe from the host can reach it.

Measured on 8.0.3 through the driver, as the administrator and as
`dbmeta_user`: a query of `system:keyspaces` returns its columns in the order
the statement selects them, a field that is missing arrives as NULL, and
`SELECT RAW ds_version()` scans into a string as `8.0.3-5933-enterprise`.
Those are the three faults of `go_n1ql` that D94 found, and none is left.
dbimp's own measurement is that 7.2.9 sends columns in name order, which no
driver can undo, and D95 holds the three choices that leaves the model.

`usql` still imports `go_n1ql`, and hard rule 10 asks for the package `usql`
uses. dburl already names the new one, and `usql` moves to it the way it moves
to `xo/cql`, as D93 recorded for Cassandra.
