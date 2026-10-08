# D196. The Cassandra dialect is cassandra

Status: Amends D81, D93 and D195.

## The decision

`dbmeta.Cassandra` is `"cassandra"`. D81 made it `"cql"`, because that was the
name of the driver and of the dburl scheme. Ken decided on 2026-10-08 that
Cassandra is named the way the other databases are, and the driver moved with
the name.

The repository `github.com/xo/cql` is now `github.com/xo/cassandra`, and v0.1.0
of the new module registers the driver `cassandra`. Its DSN is a URL only,
`cassandra://user:pass@host:port/keyspace?key=value`, with one host in the host
part and any more as repeated `host` keys. It has no host list form and no
`cql://` alias. dburl v0.47.0 renamed the scheme: Name, Driver and Dialect are
`cassandra`, and the aliases `ca`, `cass`, `cql`, `datastax`, `scy` and
`scylla` all reach the driver as `cassandra://`.

## What changed

- The `Cassandra` constant, and with it the model registry key, the `dbrun`
  drivers map and the section names of the golden files, read `cassandra`.
- The test module requires `github.com/xo/cassandra` v0.1.0 and dburl v0.47.0,
  and no longer requires `github.com/xo/cql`. The tests open the driver
  `cassandra`.
- The variable that `dbrun` gives the tests is `DBMETA_CASSANDRA`, and it was
  `DBMETA_CQL`. dbrun derives it from the dialect.
- D93 named `github.com/xo/cql` as the package that dburl names (D154). The
  registry now names `github.com/xo/cassandra`, and the move to dbimp's driver
  that the backlog waited for is no longer planned. The backlog item is gone.
- The key `"cql"` in a version set stays. It is the version of the language CQL,
  and not the name of the dialect.
- ScyllaDB shares the dialect and the model, as before.

## Why

A consumer takes the dialect from `URL.Dialect` of dburl and finds the model by
it. usql took dburl v0.47.0 and found no model for `cassandra` until
this change. One word for the driver, the scheme and the dialect is the rule
for every other database (D99).
