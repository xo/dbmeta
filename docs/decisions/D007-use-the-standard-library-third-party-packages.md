# D7. Use the standard library. Third party packages are a last resort

Status: Decided.

`dbmeta` uses the Go standard library for almost everything. An agent must not
add a third party package without asking Ken first.

There are exactly two dependencies. `database/sql` is in the standard library.
`github.com/xo/dburl` was the one approved outside package under D19, and it
handles everything to do with a connection string.

The survey below shows that the reader half of the code already meets this rule
after two small changes. A third case needs a decision.

The `Writer` interface is the only user of `github.com/xo/dburl` in
`metadata.go`. All eight of its methods take a `*dburl.URL`, at lines 148 to 162. This no longer matters for the dependency list, because D19 makes `dburl`
a direct dependency. The writer still leaves under D5, for the `tblfmt` and
`usql/env` reasons rather than the `dburl` one.

The postgres reader uses `github.com/lib/pq` in one place. It calls `pq.Array`
to scan the `TopN` and `TopNFreqs` columns of the column statistics query, at
`postgres/metadata.go:257`. Replace it. Either return the array from SQL as one
delimited string and split it in Go, or write a small scanner for the postgres
array format.

The impala reader is the hard case. It does not run SQL. It calls
`driver.NewMetadata` from `github.com/sclgo/impala-go` and reads the metadata
through the driver. An agent must not port it as it stands. Either rewrite it
as SQL queries, or leave impala out until Ken decides.

The mysql reader imports `github.com/gohxs/readline` and
`usql/drivers/completer`. Both belong to the completion feature, not to
metadata. Leave them in `usql`.

Check the models too. No metadata query needs a UUID column or anything else
that pulls in a package, so make sure that every model imports nothing
outside the standard library and the root package.
