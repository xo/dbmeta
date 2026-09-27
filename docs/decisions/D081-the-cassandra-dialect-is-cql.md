# D81. The Cassandra dialect is cql

Status: Decided.

`dbmeta.Cassandra` is `"cql"`. It was `"cassandra"` and that was wrong.

`Dialect` is documented as the `dburl` driver name and twelve of the thirteen
were. `cassandra` is not a driver name. It is an alias of the `cql` scheme,
alongside `ca`, `datastax`, `scy` and `scylla`, and `cql` is what
`github.com/MichaelS11/go-cql-driver` passes to `sql.Register` and what `usql`
registers at `drivers/cassandra/cassandra.go`. Nothing anywhere answers to
`cassandra`, so the old value named a driver that does not exist.

The `dburl` session found it. D80 sent a request there for a field saying
which product a scheme drives, Ken asked whether the thirteen values matched
the registry, and that session checked all of them. This was the one.

## Why dburl could not absorb it instead

`Scheme.Driver` has to be the exact string the Go driver registers, because
that is what a caller hands to `sql.Open`. Renaming the scheme to `cassandra`
would emit a name nothing answers to and every Cassandra connection would
fail. Adding a second scheme named `cassandra` would do the same thing with
more steps. The fault was here.

## What changed with it

The constant name stays `Cassandra`, which is the whole point of the name and
the value differing.

The golden section in `test/testdata/parity.txt` is named from the dialect, so
`[cassandra/same/grantee]` is now `[cql/same/grantee]`.

`dbrun` builds the environment variable from the dialect, as
`"DBMETA_" + strings.ToUpper(string(d))`, so the variable that carries the
Cassandra DSN is `DBMETA_CQL` and no longer `DBMETA_CASSANDRA`. The three
places in the `test` module that read the old name were changed with it. CI
needed nothing, because it runs `dbrun test` and never writes the name.

Nothing else moved. The model package is still `models/cassandra`, the
container product is still `cassandra`, the build tag is still `cassandra`,
and `testdata/conformance.txt` is keyed by the test's own name rather than by
the dialect, so its `[cassandra]` section is unchanged. Those are four
different strings that happen to have agreed, and only one of them was the
dialect.

Verified against Cassandra 5.0.9 on 2026-09-26. The parity, smoke, fixture and
conformance tests all pass with the renamed section and the renamed variable.

## The lesson, which is the reason this is a decision and not a commit

A value that is also a word is not checkable by reading it. `cassandra` looks
right in every place it appears, and it was wrong in exactly one of them. The
registry is what told the difference, which is D80 paying for itself the week
it was written.

Read these as constants and never as literals. A consumer that wrote
`dbmeta.Dialect("cassandra")` breaks here and one that wrote
`dbmeta.Cassandra` does not.
