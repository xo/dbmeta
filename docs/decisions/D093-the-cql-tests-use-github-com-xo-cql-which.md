# D93. The cql tests use github.com/xo/cql, which reports a NULL

Status: Amends D62.

The `test` module now reads Cassandra and ScyllaDB through
`github.com/xo/cql` v0.1.0 in place of `github.com/MichaelS11/go-cql-driver`.
Ken asked for it on 2026-09-27, once the fork was tagged. dburl's registry
already names `github.com/xo/cql` as the package for the `cql` scheme, and
hard rule 10 reads the package from the registry. `usql`'s `go.mod` still
names the old driver, and the `cql` session's plan moves `usql` too. Until it
does, the package here is the registry's and not yet `usql`'s.

## A NULL now arrives as a NULL

D62 found that the old driver sent an empty string for every CQL null, so
`SELECT (text)NULL` came back valid and empty. The new driver reports a null
as one. Measured on Cassandra 3.11 and 5.0: `SELECT (text)NULL` scans into an
invalid `sql.Null[string]`, and a real comment scans as itself.

So a real catalog column that is null now reaches the caller as NULL. D62
recorded that as out of reach with the old driver. A padded column still scans
into `pad`, because on ScyllaDB the padded column is a real column that stands
in for a literal, and D91 has why.

## The fault it found

Settings scanned its padded `type` column into a plain string, which only
worked because a padded NULL arrived as an empty string. On Cassandra 5.0 the
new driver sent the NULL, and Settings failed with "converting NULL to string
is unsupported". It now scans into the field, which is a `sql.Null[string]`.

No test caught it. The smoke test runs each statement and counts its
columns, which does not reach Scan. `TestCassandraScansEveryQuery` now reads
every query the model answers through its Scan, on whichever product is
running, and fails when a query is left out of its list.

## What changed in the record

The parity record changed wording and nothing else: the new driver writes
"running query:" where the old one wrote "RowData error:", and every refusal
is the same refusal. The conformance record did not change.
