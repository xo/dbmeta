# D17. Every metadata read takes a context

Status: Decided.

There is no API surface in `dbmeta` that reads metadata without a
`context.Context`. This is not a preference and it has no exceptions.

Follow idiomatic Go. The context is the first parameter and it is named `ctx`.
It is never stored in a struct. It is never `context.TODO()` in shipped code.
A function that does not reach a database does not take one.

```go
Tables(ctx context.Context, f Filter) (*TableSet, error)
```

Four rules follow from this.

1. Use only the context methods of `database/sql`. Call `QueryContext`,
   `ExecContext`, `QueryRowContext`, and `PrepareContext`. Never call `Query`,
   `Exec`, `QueryRow`, or `Prepare`.
2. Define the `DB` interface with those four methods only. The `usql` version
   has eight, because it carries both forms. Four is enough, `database/sql.DB`
   and `database/sql.Tx` both satisfy it, and a smaller interface is easier to
   fake in a test.
3. Do not create a root context inside the library. `context.Background()` and
   `context.TODO()` belong to the caller.
4. Do not take a timeout as an option. A caller that wants one wraps the
   context with `context.WithTimeout` before the call.

Rules 3 and 4 exist because the code being moved breaks both. In
`usql/drivers/metadata/reader.go`, `LoggingReader.Query` accepts no context. It
manufactures one at line 264 with
`context.WithTimeout(context.Background(), r.timeout)` when a timeout option is
set, and at line 268 it calls `r.db.Query(q, v...)` with no context at all when
one is not. A caller therefore cannot cancel a metadata query. `WithTimeout` at
line 218 is the option that must not carry over.

This matters for more than tidiness. A metadata query can be slow, and `usql`
runs them while a person waits. Completion in `usql` sets a three second
timeout for exactly that reason. With a context the caller sets that deadline,
and pressing an interrupt key can stop the query.
