# Questions, and the Reviews Behind Them

Each entry keeps the argument behind a decision, because the reasoning is what
stops a decision being undone by someone who sees only the result.

| Question | Status |
| --- | --- |
| The `DB` interface and mockability | Decided, implemented. D49 |
| NOT NULL as a constraint row | Decided, implemented. D49 |
| Where the documentation lives | Open. Waiting on Ken |

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

## 3. Where the documentation lives

Open. Nothing has been moved.

### The measurement

6116 lines of Markdown in 11 files in the repository root, against 8261 lines
of Go and 4371 of tests. Documentation is about half the size of the code and
growing faster than it.

`PLAN.md` is 3384 of those lines, more than half the documentation and a third
of the size of the library.

### Where the two reviews agree

Move almost everything into `docs/`. Both said 11 files in the root is too
many, and both named the same three that stay: `README.md`, because GitHub
renders it; `CLAUDE.md`, because an agent reads it first by convention; and
`CONTRIBUTING.md`, because GitHub gives it its own behaviour.

Neither thinks the current flat root is right at this size.

### Where they disagree, and it is not close

**Splitting the decision log.** Gemini says split `PLAN.md` into 49 files under
`docs/decisions/`, one per decision, the ADR convention. Its argument is the
context window: 3384 lines is about 25,000 tokens, and an agent that loads the
whole file to answer one question has spent its budget on 48 decisions it did
not need.

DeepSeek says do not, and gives a failure mode Gemini did not consider. A
decision that amends an earlier one lives in a different file. An agent greps a
topic, lands on the older file, reads a rule that was overturned, and has no
signal that it was. A slow answer becomes a wrong answer, which is worse.

This repository is the evidence, and it is one sided.

Six of the 49 decisions already amend, supersede or withdraw an earlier one.
D48 amends D26. D24 supersedes D22. D11 is amended by D26. D19 is half
overtaken by the code. D6 and D38 are amended. That is one decision in eight,
and the rate is rising rather than falling, because a project learns.

There are 217 references to a decision by number inside `PLAN.md` and 336 more
in the other documents and in the Go source. D8 is referenced 24 times and D24
fifteen. Under the split every one of those becomes a filename an agent has to
guess, because `D8` does not tell it whether the file is `0008-metadata.md` or
`0008-abandon-the-subpackages.md`.

DeepSeek's counter proposal is an index at the top of the single file: decision
number, one line summary, status, and where to find it. Sixty lines that make
the other 3324 cheap to grep. An agent runs one grep, gets one line number, and
reads one range.

**NULLS.md.** Gemini says delete it and merge the rule into `CLAUDE.md` and
`CONTRIBUTING.md`. DeepSeek says that guarantees drift on the one rule that was
most expensive to learn, and that duplicating it into two files is the worst
possible treatment of it. Keep the file, link it as a requirement from both.

**The index.** Gemini says put it in `CLAUDE.md` because that is what an agent
reads. DeepSeek says the choice is a false one: a human arriving from
pkg.go.dev needs a list in `README.md` and an agent needs a routing table in
`CLAUDE.md`, and they are different documents for different readers. The error
is the framing rather than the location.

**The finished documents.** Gemini says convert `QUERIES.md`, `EVALUATION.md`
and `REVIEW.md` into numbered decisions. DeepSeek splits it: `EVALUATION.md`
becomes a decision because it is one, `QUERIES.md` is a dated survey that was
never a decision and numbering it launders stale content into authority, and
this file should be folded into the decisions it argues for rather than kept as
a third copy of the same reasoning.

### What I think, for the record

DeepSeek is right about the split and the evidence is local. One decision in
eight amends another and 553 references point at decisions by bare number. A
layout whose failure mode is an agent confidently reading a superseded rule is
worse than one whose failure mode is a large file, and the large file has a
cheap fix that nobody has tried yet.

Gemini is right that 3384 lines is too much to load to answer one question.
That is an argument for an index, not for 49 files.

On `QUERIES.md` I would go further than either. It is a design era survey that
`COVERAGE.md` has overtaken, and its front matter should say so with a date,
wherever it ends up.

The one thing neither addressed: `COVERAGE.md`, `USQL.md` and `DBTPL.md` are
all generated from measurement and all go stale silently. A note saying what
was measured and when, and a test that fails when the counts drift, is worth
more than any amount of filing. `container/workflow_test.go` already does that
for the CI matrix, and the same trick applies here.

### What it would cost to decide late

Little. Moving a file is a `git mv` and a link sweep, and the links are
checkable. This is the one open question here that does not get more expensive
with a tagged release, so there is no reason to rush it.
