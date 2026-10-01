# D151. The rqlite tests use dbimp's driver

Status: Amends D148.

## The decision

D148 tested rqlite through the driver of `github.com/rqlite/gorqlite`,
because dbimp had no rqlite driver yet. dbimp v0.8.0 released one on
2026-10-01, and usql imports it. The test module and dbrun now use it, and
gorqlite is gone from `test/go.mod` and from the depguard list. It is the
move Couchbase made in D101.

The driver takes the url that dbrun prints, `rqlite://user:password@host:port`,
so the rqlite entry's dsn is that form too. It reads no `/status`, so the
ordinary user connects with no change to the url.

## The helper for a boolean is gone

gorqlite gave every number as a `float64`, and D148 added `NumberAsBool` and
`NullNumberAsBool` to the root package for the sqlite3 model to read a
boolean from one. dbimp's driver gives a number the Go type of its column's
affinity (dbimp D140), so an integer arrives as an `int64`, which
`database/sql` reads into a `bool` itself. The two helpers were removed, and
the sqlite3 model scans its booleans directly again, as it did before D148.

rqlite 9.4.5 and 10.3.6, and SQLite on both of its drivers, pass on
2026-10-01 with the new driver and with no helper.
