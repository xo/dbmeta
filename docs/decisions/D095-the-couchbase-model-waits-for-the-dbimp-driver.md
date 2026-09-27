# D95. The Couchbase model waits for the dbimp driver

Status: Amends D94, amended by D101 and D104.

D94 tied the Couchbase model to a rewrite of `xo/n1ql`. Ken decided on
2026-09-27 that the first driver in `github.com/xo/dbimp` is a new Couchbase
driver for SQL++ over HTTP, and that it replaces `xo/n1ql`. dbimp's D23
records it. So the model waits for that driver instead.

Nothing else in D94 changes. `dbrun` starts the same three releases, the DSN
names the query service, and the three faults D94 found in `go_n1ql` are the
requirements the new driver is written to. dbimp's D8 names them.

When the driver works, `dburl`, `usql` and `dbmeta` move to it together,
because hard rule 10 requires the package that `usql` uses. The model is then
written against it and measured, as D93 measured the cql model on
`xo/cql`.

## Couchbase 7.2 sends its columns in name order

The `n1ql` session measured the query service itself on 2026-09-27. 7.2.9
sends the fields of each result object in name order, and 7.6.12 and 8.0.3
send them in the order the statement selects them. So on 8.0.3 the name order
that D94 found came from `go_n1ql` decoding into a map, and on 7.2.9 it comes
from the server, where no driver can undo it. Ken chose, for dbimp, to accept
name order on 7.2 rather than send a PREPARE to learn the order.

Every Scan in a dbmeta model reads a column by its position. So a Couchbase
model has three choices on 7.2, and it makes one when it is written: raise
the floor to 7.6, name every column so that name order and field order agree,
or read 7.2 by name. This was measured by the `n1ql` session and not yet here.

dbimp's D24 goes further. A dbimp driver can replace one that `usql` imports
today, such as the ones for ClickHouse, Trino, Presto and DynamoDB, to cut
`usql`'s dependencies. Each such switch means that dbmeta measures the model
again on the new driver. Nothing changes here until `usql` switches.
