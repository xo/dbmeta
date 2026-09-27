# D47. dbmeta supplies the data. The consumer decides what to show

Status: Decided.

`dbmeta` returns the facts a database holds. It never withholds one, reorders
one, or formats one so that somebody's output looks right. Deciding what to
print, what to leave out and how to lay it out belongs to the consumer, every
time.

`usql` aims to be compatible with `psql`, so it will show at least what `psql`
shows and will drop columns `psql` does not print. That is `usql`'s work, not
this library's. `dbtpl` wants the same facts in a different shape and shows
none of them. One set of queries serves both because neither presentation is
baked in.

## What psql still decides

D2 stands, narrowed. `psql` sets the object model: which kinds exist, what they
are called, and the shape of an answer when two databases disagree. It does not
set the column set of a kind, and it never did: `psql` prints what a person
reads at a terminal.

That distinction is what D46 got wrong by omission. `psql` renders a constraint
and a routine signature as formatted text because a human is reading them, and
`dbmeta` copied the text. A code generator cannot use it.

## When a field may be added

The test is cost, and it is checkable.

A field may be added when one statement can produce it: a column already in a
selected row, a column reached by a join, or a correlated subquery whose plan
stays bounded as the catalog grows. A field may not be added when it needs a
second statement, a per row round trip, or a scan whose cost grows with the
whole catalog rather than with the rows returned.

Gemini wanted correlated subqueries banned outright, on the grounds that they
cause N+1 plans. That is wrong and DeepSeek said so. N+1 is a client issuing
one query per row, which the one statement rule already forbids. A correlated
subquery is one statement. The rule would also outlaw code that already works:
the PostgreSQL model aggregates enum labels that way, and the whole SQLite
model rests on correlated table valued pragma joins, which is the only way
SQLite exposes the columns of more than one table at a time.

Check a new field with `EXPLAIN` against a catalog with thousands of tables,
not against a fixture with five.

## Prose and parts

Where a fact exists as both, the parts are authoritative and the prose is kept.

Both reviews said to drop the prose. They are right that parts cannot be
recovered from it, and wrong about the cost of removing it. `psql` prints
`pg_get_constraintdef`, `usql` must show at least what `psql` shows, and making
every consumer rebuild that string for every dialect is work this library
already did once. `Constraint.Definition` and `Function.ArgTypes` stay.

A new fact arrives as parts. Prose is never the only form of anything.

Raw DDL is not prose. A view's definition and a trigger's body are what the
catalog stores, not a rendering of it, so they are returned as text and that is
the structured answer.

## Granularity

A child of an object is its own kind with flat rows, never a slice on the
parent. `ConstraintColumns` and `RoutineParameters` are separate queries
carrying the parent's identity and an ordinal, and the consumer groups them.

Both reviews agreed and the reason is the iterator. Filling a slice on the
parent needs either a second statement per parent, which D33 forbids, or
`string_agg` and `json_agg`, which differ in every dialect and lose types.
Relational databases return flat rows, so `dbmeta` yields flat rows.

## Session state and statistics are in scope

Gemini drew the line at durable DDL in the catalog and would reject the current
schema as session state and column statistics as runtime data. That line is
wrong for this library, because it excludes two of the five things the
consumers measured in D46 actually asked for, and `usql` would lose the `\ss`
command by migrating.

The line is this instead. `dbmeta` answers anything the database will tell it
about itself in one read only statement. A kind says in its documentation
whether it is durable, session dependent or runtime, so a caller knows what it
is holding. `CurrentSchema` is session dependent and says so. `ColumnStats` is
runtime, may be stale, and says so.

What is still out of scope: anything that writes, anything that needs a second
statement, and anything about the data rather than the schema. Row counts are
statistics and are in. The rows themselves are not metadata.

## Compatibility once a kind ships

An exported field is API. Do not rename one, do not change its type, and do not
change what it means. Add a field rather than repurpose one.

Adding a field to a shipped struct is allowed, and it is why this decision is
written down before five kinds arrive at once. A caller that builds one of
these structs with an unkeyed literal breaks, and `go vet` already reports an
unkeyed literal of a struct from another package, so the compiler and the
standard tooling cover it.

Both reviews wanted an unexported `_ struct{}` field on every struct to force
keyed literals. That is a real technique and it is not taken, because `go vet`
runs the same check by default and nobody constructs these structs anyway: they
come out of a `Scan` that `dbmeta` wrote.

A field only some databases can fill is normal and is already the rule. The
model that cannot fill it selects `NULL AS "name"`, the field carries `Desc`
saying why, and `Field.Present` tells a caller whether the NULL means absent or
genuinely null. That is the padding rule and nothing here changes it.
