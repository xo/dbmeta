# D49. One method on the interface, and a NOT NULL is not a constraint row

Status: Amended by D121.

Two answers, both unanimous between Gemini and DeepSeek, both recorded in full
in this file, under each decision, before they were taken.

## Queryer has one method

`DB` is gone. `Queryer` replaces it and declares `QueryContext` and nothing
else.

The old interface declared four methods and `dbmeta` called two. One of the
other two was `ExecContext`, so the interface of a read only library advertised
the single operation hard rule 8 forbids it from performing. `Dialect.Version`
used `QueryRowContext` and now reads its one row through `QueryContext`, which
costs four lines and removes a method from the contract.

`database/sql.DB`, `Tx` and `Conn` all satisfy one method.
`database/sql.Stmt` satisfies neither this nor the old four, because its
`QueryContext` takes no statement.

The name changed with the shape. It is a queryer, not a database, and calling
it `DB` invited the reading that it stands in for `sql.DB`. D121 later changed
the spelling to `Queryer`, which is how Go's own packages spell it.

## The interface is documentation, not a seam for a mock

This is the part worth writing down, because the goal it was added for cannot
be reached and someone will try again.

A fake cannot satisfy it. `sql.Rows` is a struct with unexported fields that
only `database/sql` constructs, from a registered driver, so nothing outside
that package can return one. That was as true of the four method version.

Both reviews rejected defining a `Rows` interface to fix that. It would change
the `Scan` signature of every binding, give up `ColumnTypes`, `RawBytes` and
`NextResultSet`, allocate per row, and break every consumer that hands a
`*sql.Rows` to something else.

Mock at the driver level. `database/sql/driver` is the seam Go provides and
`examplefake_test.go` already uses it, replaying recorded rows with no database.

What the interface does buy is the thing to keep: one method states in the type
system that this library reads and does nothing else. A reader establishes that
from the declaration rather than by searching for `ExecContext`.

## A NOT NULL constraint is not reported on any release

PostgreSQL 18 records a NOT NULL constraint in `pg_constraint` with `contype`
`n`. Every earlier release records it only as `pg_attribute.attnotnull`.

`Constraints` and `ConstraintColumns` both exclude it, on every release. The
fact is reported by `Column.Nullable`, which is filled everywhere.

Reporting it would make the same schema answer differently on release 17 and
release 18, for a reason that has nothing to do with what either server can do.
That is the leak this library exists to prevent.

Synthesizing the rows on older releases from `attnotnull` was rejected, and the
reason is sharper than inventing names. Release 18 lets a NOT NULL constraint
be named explicitly, so a synthesized `<table>_<column>_not_null` would be
wrong some of the time, which is worse than absent.

## The gap this found in the padding rule

The padding rule governs the column set. It says nothing about rows, and until
now nothing needed it to.

A release that starts recording an existing fact as a catalog row is a version
leak the padding rule does not catch. The rule to add: a query returns the same
rows for the same schema on every release, and where a release records
something new about an object that was always true, report it the way every
release can report it or do not report it at all.

`TestNotNullIsNotAConstraintRow` holds this one, and it runs on all ten
releases, so it fails on 18 alone if the filter is ever removed.

## What is still not answered

A migration tool on release 18 that wants the name of a NOT NULL constraint, in
order to drop it by name, cannot get it here. Both reviews called that
PostgreSQL specific and outside the normalized model.

If it is wanted later, the shape that does not leak is a field on `Column`
holding the constraint name where the release has one and absent where it does
not, with `Field.Min` saying which. That is the padding rule doing its job, and
it is a decision rather than a translation.
