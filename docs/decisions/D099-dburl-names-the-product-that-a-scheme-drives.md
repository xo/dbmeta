# D99. dburl names the product that a scheme drives

Status: Decided.

Ken decided on 2026-09-27, answering the question that was open since D80,
that dburl is where a consumer learns the dialect of a scheme. dburl already
does it. This session sent the dburl session a request for the field, and the
dburl session answered that it had shipped: `Scheme.Dialect`, in dburl
v0.30.0, which dburl's D19 records as requested by dbmeta. The request was not
needed. `scheme.go` was read earlier the same day, with its `Dialect` lines in
it.

## What the field holds

`Scheme.Dialect` is the Driver name of the scheme that is canonical for the
product, which is the value `dbmeta.Dialect` holds. It is `postgres` for pgx,
postgres, pq, cockroachdb and redshift, `sqlite3` for moderncsqlite and
sqlite3, `oracle` for godror and oracle, and `mysql` for memsql, tidb, vitess
and mysql. Every scheme but `file` has one, and a test in dburl requires it. So the three schemes the question named map with no list here,
which is what hard rule 1 asks.

## How a consumer reads it

A parsed `URL` carries it from dburl v0.32.0. `Parse` sets `URL.Dialect` to
the `Scheme.Dialect` of the parsed scheme, for a scheme registered at run time
too, and a `file:` URL takes the Dialect of the scheme it resolves to. dburl's
D24 records it. Ken chose a field on `URL` over a lookup by scheme name,
because a lookup has to be passed `UnaliasedDriver`, and passing `Driver` is
the easy mistake.

Mapping `URL.Driver` to a dialect is wrong, not only incomplete. dburl's D22
makes `postgres://` open pgx and return the Driver `pgx`, moves lib/pq to
`pq://`, and points cockroachdb and redshift at pgx. `URL.Dialect` is
`postgres` for all of them. Hard rule 1 in `CLAUDE.md` names `URL.Dialect` as
what selects a model.

## What was weighed

Each consumer keeping its own mapping costs `usql` three entries and costs
every later consumer the same three again, and two copies drift. `dbmeta`
accepting the alias is what hard rule 1 forbids. dburl holding the fact
answers every consumer at once, and it is the same kind of fact as
`GoPackage`, which D80 welcomed.
