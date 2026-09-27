# D33. Results stream. The package does not materialize them

Status: Decided.

`dbmeta` executes a model against the database and reads the result one record
at a time. It does not load a result set into memory and hand back a slice.

Alongside that, provide standard Go iterators, so a model can be consumed from
outside the package in the ordinary way.

This reverses part of D18, which said to return a typed slice such as
`[]Table` and to reach for an iterator only when a result is large enough to
matter. That guidance is withdrawn. The iterator is the shape.

Two consequences follow and both are improvements.

Question 8 answers itself. A caller that asks for the columns of 200 tables
issues one query with a schema filter and walks the rows. There is nothing to
batch and nothing to cache, so `dbmeta` holds no state, has no staleness rules
and no invalidation to get wrong.

It also removes the last reason the `usql` cursor types existed. D18 rejected
them for being a cursor over a slice already in memory. A real iterator over
live rows is the thing that cursor was pretending to be.

Note what this requires of the error handling. An error can arrive part way
through a result, so the iterator must be able to report one. Use
`iter.Seq2[T, error]`, and make the caller able to tell the end of a result
from a failure in the middle of one.

## The iterator holds a connection, and that is a hazard

Both external reviews raised the same failure and it is the sharpest finding
against this decision. A live iterator holds an open `*sql.Rows`, and an open
`*sql.Rows` holds a connection from the pool.

So this deadlocks:

```go
for table, err := range meta.Tables(ctx, f) {
    for col, err := range meta.Columns(ctx, filterFor(table)) { // second connection
        ...
    }
}
```

The outer loop holds one connection while the inner one asks for another. With
a pool of N, N nested iterations exhaust it. With `MaxOpenConns(1)`, which is
ordinary for SQLite3 and common in tests, the first nested call deadlocks
immediately.

Three things address it and all three are required.

1. Document it. The package documentation must say that an iterator holds a
   connection until it is exhausted or stopped, and must show the filter form
   that avoids nesting. Asking for every column in a schema in one call is
   almost always what the caller wanted.
2. Define the lifecycle. Stopping early, by `break` or by a `yield` returning
   false, must close the rows and release the connection. An iterator that
   leaks a connection on `break` is worse than a slice. Cancelling the context
   must do the same.
3. Make the escape hatch available. A caller that genuinely needs the whole
   result in memory collects the iterator into a slice with `slices.Collect`,
   which releases the connection at once. That is the caller's choice and it
   needs no API of its own.

This is the cost of D33 and it is accepted. Note that materializing by default
would trade this hazard for unbounded memory on a large catalog, which is the
worse failure.

## Reading more than one catalog needs one snapshot

Gemini raised this and the plan did not cover it. Reading tables and then
reading their columns are two queries, and a table can be dropped between them.

`dbmeta` does not solve this and must not pretend to. The `DB` interface from
D17 is satisfied by `*sql.Tx` as well as `*sql.DB`, so a caller that needs a
consistent view opens a transaction and passes that in. Say so in the package
documentation, because a caller who does not know cannot guess.
