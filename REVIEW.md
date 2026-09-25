# Two Questions, and the Reviews Behind Them

Both are decided and both are implemented. D49 records what was taken. This
keeps the argument, because the reasoning is what stops either decision being
undone by someone who sees only the result.

Status: accepted in full, both of them, exactly as the reviews recommended.

Gemini and DeepSeek were asked both, independently, and agreed on every point.
Where the answers here are unanimous that is said, because unanimity between
two models that disagree freely elsewhere is worth something.

## 1. The DB interface: narrow it, and give up on mocking without a driver

### What was there

```go
type DB interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	PrepareContext(ctx context.Context, query string) (*sql.Stmt, error)
}
```

Every read takes it. So `dbmeta` is already doing the thing `dbtpl` does, and
the question is whether it is doing it correctly rather than whether to start.

### Two facts, measured

`dbmeta` calls two of the four methods. `QueryContext` once, in the row
iterator. `QueryRowContext` once, to read the server version. `ExecContext` and
`PrepareContext` are never called anywhere in the module.

`ExecContext` should not be in the interface of a read only library at all. It
is the one method that writes, and hard rule 8 forbids calling it. Its presence
says the opposite of what the library promises.

### The interface cannot be mocked, which is the goal it was added for

`*sql.Rows` and `*sql.Row` are concrete structs with unexported fields.
Only `database/sql` can construct one, and only from a registered driver. A
fake that satisfies this interface has no way to return a `*sql.Rows`, so it
cannot be written.

That is why `dbmeta`'s own tests mock one level lower, at
`database/sql/driver`, with a fake driver replaying recorded rows. See
`examplefake_test.go`. The interface delivers the documentation benefit and
delivers none of the mocking benefit.

### What both reviews said

Unanimous on all four points.

Narrow it to one method, `QueryContext`, and rewrite the version read to use
it. That is a few lines: call `Next` once, scan, close.

Do not define a `Rows` interface of `Next`, `Scan`, `Columns`, `Err` and
`Close` to make mocking possible. It costs interoperability with everything
that takes a `*sql.Rows`, it gives up `ColumnTypes`, `RawBytes` and
`NextResultSet`, it allocates per row, and it would change the signature of
every model's `Scan` function, which is `func(*sql.Rows) (T, error)` in all 54
of them.

Mock at the driver level. It is the standard Go answer and it is what this
module already does.

Rename it. DeepSeek's point: it is a querier, not a database, and `Querier` is
the honest name for a one method interface.

```go
type Querier interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}
```

`*sql.DB`, `*sql.Tx` and `*sql.Conn` all satisfy that. `*sql.Stmt` does not,
because its `QueryContext` takes no query string, and it never did satisfy the
current four method version either.

### What this says about dbtpl

The design choice in `dbtpl` was right in intent and does not reach the goal
either, for the same reason: an interface over `database/sql` that returns
`*sql.Rows` documents the surface and does not enable a mock. Narrowing it and
keeping it is the answer in both places.

The documentation argument survives intact and is the real reason to keep an
interface. One method says, in the type system, that this library reads and
does nothing else. A reader does not have to grep for `ExecContext` to find out
whether it writes.

### What shipped

```go
type Querier interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}
```

`Dialect.Version` reads its one row through `QueryContext`, so the interface
needs no second method. `TestQuerierIsOneMethod` passes a type that has only
that method, so the whole library is checked against the contract rather than
trusted to it.

## 2. NOT NULL as a constraint row: filter it out

### What changed

PostgreSQL 18 records a NOT NULL constraint as a row in `pg_constraint`, with
`contype` `n`. Every release before 18 recorded it only as
`pg_attribute.attnotnull`, a boolean on the column.

The `Constraints` query now maps `n` to the string `not null`. On release 18 a
table with three NOT NULL columns yields three extra `Constraint` rows, named
`author_name_not_null` and so on. On release 17 the same schema yields none.

### Why that is a problem

The padding rule says a query returns the same column set on every release. It
says nothing about rows, and this is the first case where that gap matters. The
schema is identical on 17 and 18. Only PostgreSQL's internal storage differs,
and `dbmeta` exists to hide exactly that.

### What both reviews said

Unanimous: filter `contype = 'n'` out of the query on every release, so
`Constraints` never reports a NOT NULL anywhere.

`Column.Nullable` already carries the fact, is populated on every release, and
is the normalized answer.

Both rejected synthesizing the rows on older releases from `attnotnull`.
Gemini called it inventing fake constraint names. DeepSeek added the sharper
reason: PostgreSQL 18 allows a NOT NULL constraint to be named explicitly, so
the name is not reliably `<table>_<column>_not_null`, and truncation and
collision rules differ. A synthesized name would be wrong some of the time,
which is worse than absent.

Both rejected adding a boolean to `Constraint`. The boolean exists, on
`Column`, which is where it belongs.

DeepSeek answered the code generator question concretely: a generator reads
`Column.Nullable` to emit `col type NOT NULL` and to choose a pointer or an
optional type, and it reads `Constraint` rows for the names and definitions of
primary keys, foreign keys, unique constraints and checks. It never discovers
NOT NULL from a constraint row.

### The one thing this leaves out

A migration tool on PostgreSQL 18 that wants the name of a NOT NULL constraint,
in order to drop it by name, cannot get it. Both reviews said that is a
PostgreSQL specific need and belongs outside the normalized model.

If Ken wants it later, the shape that does not leak is a separate kind, or a
field on `Column` holding the constraint name where the release has one and
absent where it does not. That is the padding rule doing its job, and it is a
decision rather than a translation.

### What shipped

`AND r.contype <> 'n'` in `Constraints` and in `ConstraintColumns`, which had
the same leak. `TestNotNullIsNotAConstraintRow` asserts no such row appears and
that `Column.Nullable` still carries the fact, and it runs on all ten releases,
so it fails on 18 alone if the filter is removed.

D49 also records the rule this found: the padding rule governs the column set
and said nothing about rows, and a release that starts recording an existing
fact as a catalog row leaks past it.
