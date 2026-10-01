# D26. No database driver in the dbmeta module

Status: Amends D11, amended by D48.

The `dbmeta` module depends on the standard library alone. It does not
depend on a database driver, and its `go.mod` does not name one.

A package that uses `dbmeta` brings its own driver and chooses its own version
of it. `dbmeta` must not constrain that choice, and must not force an upgrade
on a consumer who is holding a driver back.

This is why `dbmeta` takes a connection through an interface rather than
opening one. D17 already defines that interface with four context methods, and
`database/sql.DB` and `database/sql.Tx` both satisfy it. The caller opens the
connection with whatever driver it likes and hands it over.

## This conflicts with D11, and D11 gives way

D11 pins `dbtpl` as a tool in `go.mod`. That cannot stay in the root module.

`dbtpl` depends on four database drivers, which its own `go.mod` lists:
`github.com/go-sql-driver/mysql`, `github.com/lib/pq`,
`github.com/mattn/go-sqlite3` and `github.com/microsoft/go-mssqldb`. Pinning
`dbtpl` in the root module puts all four into the root `go.mod` and `go.sum`,
which is exactly what this decision forbids.

State the risk accurately, because it is smaller than it first looks and the
decision does not rest on exaggerating it. Go prunes the module graph for
modules declaring go 1.17 or later, so a dependency that provides no imported
package is dropped from a consumer's build list. This almost certainly does
not force a consumer of `dbmeta` onto `lib/pq v1.12.3`.

It is still the wrong shape. The root `go.mod` and `go.sum` then list four
drivers that the library never imports. Vulnerability scanners report them,
`go mod graph` shows them, and a build of `dbmeta` itself downloads them. The
stated intent is that the module carries no driver, and a separate module
delivers that without relying on a pruning rule to hide it.

## Where the drivers and the generator go

Everything that needs a driver lives outside the root module, in a module of
its own with its own `go.mod`. That module holds:

1. Every database driver used for testing.
2. The container harness from D23.
3. The integration tests that connect to a real server.

This gives `dbtest` a second and stronger reason to exist. D23 justified it as
a container harness. It is also the boundary that keeps drivers out of
`dbmeta`. Without it, either this decision or D11 has to break.

D27 proposes how that splits between a nested module here and the sibling
repository `xo/dbtest`.

## The root module still needs testing

Most of `dbmeta` can be tested without a driver at all, and that work belongs
in the root module.

Fragment merging is pure logic. Given a set of fragments and a server version,
the selector produces one SQL string. Test that against golden files, with no
database present. This covers the part of D8 most likely to break, and it
covers every version, including the ones no container can reach.

Version parsing and selection are the same. Feed integers, assert which model
is chosen, assert the fallback below the floor and the behavior above the
ceiling that D21 defines.

The rule for the root module is that a test there never opens a connection. A
test that needs a server goes in the other module.
