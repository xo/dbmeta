# D31. Models register from internal, one file each, gated by build tags

Status: Decided.

The wiring layer copies `usql` and sits in `internal/`, with one file per
model. `internal/postgres.go` registers the PostgreSQL model,
`internal/mysql.go` the MySQL one, and so on.

Say models, not drivers. `usql` calls them drivers because it opens
connections. `dbmeta` never opens one, so the word here is model. Crib the
`usql` implementation and rename as you go.

## The tags

Copy the `usql` tiers exactly: `none`, `base`, `most` and `all`. A build picks
how many models it compiles in, and a consumer that wants three databases does
not carry thirty.

`usql/internal` shows the three shapes in use:

```go
//go:build (!no_base || postgres) && !no_postgres
//go:build (all || most || couchbase) && !no_couchbase
//go:build (all || odbc) && !no_odbc
```

The first is the base tier, built by default and turned off with `no_base`. The
second is the `most` tier. The third is the `all` tier, which `usql` uses for
the three it does not build by default: `charts`, `odbc` and `godror`.

`dbmeta` has no equivalent of that third tier. There is no bad model here,
because a model is SQL rather than a driver with a C dependency, and D29 keeps
cgo out entirely. Use `none`, `base`, `most` and `all` only.

The `usql` base tier is these eight: `csvq`, `clickhouse`, `oracle`, `duckdb`,
`sqlserver`, `postgres`, `mysql`, `sqlite3`. This is not the same set
as the primary databases in the phase plan. It has `clickhouse` and `csvq`,
which the phases do not mention, and it does not have Cassandra, which phase 3
does.

## The risk this carries

Both external reviews objected to build tags in a library, and the objection is
recorded because Ken chose this deliberately to match `usql`.

Their point: a build tag is global to a build, not per dependency. A consumer
cannot ask for `dbmeta` with the `most` tier while another dependency asks for
`base`. The tier is chosen by whoever builds the final binary. That works for
`usql`, which already sets these tags and is a binary, and it is surprising for
a library consumed by something else.

DeepSeek added the sharper version, which is an interaction with D34. A model
excluded by a build tag is not present at all, so it cannot report a
capability and it cannot return `ErrNotSupported`. That is a third state beyond
the two D34 names, and the API must distinguish all three:

1. The database does not have that object.
2. The model exists and the database cannot answer.
3. The model was not compiled into this binary.

The third needs its own error. Call it what it is, and do not let it surface as
either of the others. A user whose build is missing a model must be told that,
not told their database lacks a feature.

## gen.go

A `gen.go` generates the `internal/<model>.go` files. It reads the metadata
each model declares about itself, and it produces the wiring plus the
documentation that has to agree with it, including the model links and the
support table in `README.md`.

The point is that the list of models exists once. A table in `README.md` that
someone edits by hand drifts from the code within two releases, and D21 and D24
both require that table to be accurate.
