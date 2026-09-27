# D46. Five object kinds are missing, and two consumers say which

Status: Decided.

`dbmeta` answered 48 object kinds and neither consumer could move onto it.
Measuring both said why, and the two lists overlap. Add these five before
telling anyone to migrate.

The evidence is in `USQL.md` and `DBTPL.md`, both measured rather than read.

## The three that both consumers need

**Routine parameters.** `usql` calls them `FunctionColumns` and `dbtpl` calls
them `ProcParams`. `dbmeta` has `Function.ArgTypes`, which is one string. A
person can read it and a code generator cannot use it. The kind needs a name, a
position, a direction, a type and a size, which is the union of what the two
ask for.

**Constraint columns.** `usql` calls them `ConstraintColumns` and `dbtpl` calls
them `TableForeignKeys`. `dbmeta` has `Constraint.Definition`, which is again
one string. The kind needs the column, its position, and for a foreign key the
catalog, schema, table and column it points at. A composite key needs the
position, or it cannot be put back together.

This is the largest gap. `dbtpl` generates code from a foreign key, and parsing
`author_id -> author(author_id)` out of a string is not something to ship.

**Column statistics.** `usql` reads them for `\ss`, which `psql` does not have
and `usql` added. Six of its drivers implement it. The kind needs the average
width, the null fraction, the distinct count, the minimum, maximum and mean,
and the most common values with their frequencies.

`\ss` is the one command that would regress on every database if `usql` moved
today, so this is not optional either.

## The two that only dbtpl needs

**Enum values as rows.** `Types` reports `Kind` as `enum` and joins the labels
into `Type.Elements` with commas. `dbtpl` generates a Go constant per label and
needs a row each, with the sort order. Splitting the string is not good enough,
because a label can contain a comma. MySQL has no enum type, only an enum
column, so the kind has to allow a name that came from a column.

**The definition of a view.** `dbtpl.Table.ViewDef` holds the SQL of a view.
`dbmeta.Table` has no such field and no query returns one.

## Two smaller things to decide with them

A routine needs a stable identity. `dbtpl` reads routines and then reads the
parameters of each, joined on the PostgreSQL oid. `Function` has no identifier,
and the name is not enough, because PostgreSQL allows two functions with one
name and different arguments. Decide what a parameters query joins on before
writing it.

`Column.IsPrimaryKey`. `dbtpl` reads it per column and `dbmeta` answers it
through `Constraints` or `Indexes`, which is a second query and a join in Go.
Ask whether the column should carry it.

## What this does not change

None of these is a new kind of thing. Every one follows the existing shape: a
struct in `object.go`, a `NewQuery` value, a binding per model, a fixture object
to read, and an entry in `COVERAGE.md` for the databases that cannot answer.
The padding rule and the version gates apply unchanged.

Adding them raises the count from 48 to 53. It does not raise what any database
answers by itself, because a model has to implement each one, and a database
without column statistics reports `ErrNotSupported` like anything else.

## Why they were missed

The 48 came from `psql`, which is D2 and is still right. `psql` prints a
constraint and a routine signature as text, because a person is reading it. A
code generator and a completer need the parts. Following `psql` for the shape
of an answer was correct, and following it for the granularity of an answer was
not, in these two places.

That is worth remembering for the next consumer. The object set was complete
against `psql` and it had never been checked against anything else.

## Done

All five shipped, and the count went to 54. D47 sets the policy that allowed them
and the cost test for the next one. Four of the five turned out to be standard
`information_schema` views, so the shared model answers them too, which nobody
predicted. `COVERAGE.md` has the matrix and `USQL.md` and `DBTPL.md` say what
each consumer can now do.
