# D62. CQL cannot compute, so the Cassandra model computes in Scan

Status: Amended by D93.

A CQL statement selects columns and nothing else. There is no CASE, no
expression, no function that turns one value into another. Every other model
computes a field in the statement and this one cannot, so the Cassandra model
selects the raw catalog column and `Scan` maps it.

Four things follow, and each was measured against Cassandra 5.0.9 and 3.11.19
rather than read.

## A derived field is derived in Go

`Column.Nullable` and `Column.PrimaryKey` are both read from
`system_schema.columns.kind`. A column of kind `partition_key` or `clustering`
is in the primary key, and the primary key is the only thing in Cassandra that
cannot be null. The statement selects `kind` twice, once under each name, so
the column count still matches the field count, and `Scan` turns each into its
boolean.

That is a deviation and it is confined. The invariant the tests enforce is
that a query returns as many columns as it declares fields, not that a column
maps to a field untouched. `Scan` is Go, it already exists on every binding,
and there is nowhere else for the work to go.

## A filter cannot be optional, so no query filters

Every other model writes `(@schema IS NULL OR col LIKE @schema)`. CQL has no
`OR`, no `IS NULL` outside a materialized view definition, and a partition key
takes only `=` or `IN`. There is no `NOT IN` either, so the system keyspaces
cannot be excluded.

So every query returns every row. The filter parameters are still declared,
and every description says plainly that Cassandra ignores it. Declaring them
is what keeps a caller passing one from getting `ErrUnknownParam`, and `usql`
already narrows the result itself to match `psql`, so it gets the right output
from the whole one.

The alternative considered and rejected was declaring no parameters, which is
more honest and makes every consumer special case Cassandra. Over-returning is
the lesser fault: it never hides a row, and the cost is bounded because
`system_schema` is small. On the server this was written against it is 48
tables and 313 columns.

## There is no order across partitions

CQL orders rows within one partition, by a clustering column. A result that
spans partitions arrives in token order, and the same query can return the
same rows in another order on another cluster. No query here writes `ORDER BY`,
because writing one would not make the answer ordered and would suggest it
was.

## Padding is a type hint, and every alias is quoted

`NULL AS "name"` is refused: "Cannot infer type for term NULL in selection
clause (try using a cast to force a type)". The form that works is
`(text)NULL`, which is CQL's type hint, and it is what `docs/NULLS.md` asks
for: a fact Cassandra does not have is NULL and never a literal. A literal is
used only where it is true of every row the query returns, such as the `type`
of a row from `system_schema.tables`.

Every alias is quoted. `schema` is a reserved word and `SELECT keyspace_name
AS schema` is a syntax error, which cost an afternoon because the driver
reported an empty result rather than the error. `table`, `index`, `default`,
`set` and `primary` are reserved too. Quoting every alias sidesteps the whole
class, and it also stops CQL lower casing one, which it does to an unquoted
identifier.

## The driver cannot report a null, so a pad is discarded

`(text)NULL` selects, and it does not arrive as a null. gocql decodes a null
of any type as the zero value of that type and go-cql-driver hands that to
`database/sql`, so scanning one into `sql.Null[string]` gives a valid empty
string. Verified directly: `SELECT (text)NULL` comes back `Valid` with `""`.

Left alone that makes every padded field look present and empty, which is
what `docs/NULLS.md` exists to prevent. It showed up in the conformance
projection as `has_default=true` on every column of a database that has no
defaults.

So a padded column is scanned into `pad`, a `sql.Scanner` that discards, and
the field keeps its zero value, which is the invalid Null. The column is still
selected, because the statement should say what it returns and because a query
returns as many columns as it declares fields.

This does not rescue a real catalog column that is null, and nothing can with
this driver. `docs/COVERAGE.md` says so.

## A fixed filter is allowed, and 4.0 decides which one

An optional filter is impossible and a fixed one is not, which is how the
constraint queries return the primary key and not every column. The predicate
tests `position` rather than `kind`: a key column carries its position within
the partition key or the clustering key counting from zero, and everything
else carries -1.

`kind IN ('partition_key', 'clustering')` reads better and 3.11 refuses it,
with "IN predicates on non-primary-key columns (kind) is not yet supported",
because that arrived in 4.0. The two forms select the same rows on 5.0, 102 of
them, and no regular or static column has a position at or above zero on
either release. One form that works everywhere beats a fragment that makes the
same query mean two things on two releases.
