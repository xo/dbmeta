# D292. A dump writes range types and the state of a trigger

Status: Decided.

The dump fixture of D291 showed two gaps in the dump of PostgreSQL. It left out
range types, so a table with a range column failed, and every object after it
failed too. It also wrote a disabled trigger as an enabled one.

## Range types

The dump reads a range type from `Types`, which gets six new fields. They come
from one join with `pg_range`, so they pass the cost test of D47.

- `RangeSubtype`, the type of the bounds.
- `RangeOpClass`, `RangeCollation`, `RangeCanonical` and `RangeSubtypeDiff`,
  which are absent when the type takes the default.
- `RangeMultirange`, the name of the multirange type. It is absent below
  release 14.

A snapshot holds a range type as `ranges` of its schema. The script writes it
with `CREATE TYPE AS RANGE`, in the order of the other types, so that a
composite type or a table can use it. The multirange type is not written,
because the range type makes it, and the name is set with `multirange_type_name`.

A server also makes a function in the language `internal` for the constructor of
a range type and of its multirange. The dump leaves out such a function.

A user function that is the canonical function or the difference function of a
range type is not written before the type. This is a known gap, and the
backlog holds it.

## The state of a trigger

A trigger that is disabled, or set for the replica role, is written with
`ALTER TABLE ... DISABLE TRIGGER` and its relatives. The style of a product
holds the statement as `TriggerState`.
