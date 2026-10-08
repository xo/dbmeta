# D200. A binding can keep rows for a product that cannot filter

Status: Amends D62.

## The decision

D62 left every Cassandra query unfiltered. CQL has no OR and no IS NULL, so
the statement cannot take an optional filter. The parameters schema, name,
parent and with_system were declared and ignored, and the caller narrowed the
result. That was wrong for a consumer that does not name a product.

`dbmeta.Binding` has a new field, `Keep func(row T, args map[string]any) bool`.
`Query.All` calls it after `Scan` for each row, with the arguments with their
defaults filled in, which is the map that `Walk` receives. A row for which it
returns false is not yielded. `Query.Each` calls `All`, so it keeps the same
rows. A model that filters in its statement leaves `Keep` nil, and nothing
changes for it. `Keep` is not called for a row that `Scan` failed to read, so
that error still reaches the caller. It does not apply when `Walk` is set,
because a walk filters in its own code.

The Cassandra model sets `Keep` on every kind it registers, and the match is
`dbmeta.Like`, as the walk models of Elasticsearch and OpenSearch use. `schema`
matches the keyspace, `name` matches the name of the object, and `parent`
matches the table of a column, an index, a constraint or a trigger. A role, a
role grant, a role setting and a setting belong to no keyspace, so for them a
`schema` pattern other than empty or `%` matches nothing. The match is case
sensitive, as `LIKE` is in PostgreSQL and as a quoted Cassandra name is.

`with_system` hides the keyspaces that the product keeps for itself. The model
holds the list as data and `docs/COVERAGE.md` says where each name came from.

## Why

`usql` runs `\dt ks.*` and `\d ks.t` through `dbmeta`, and its formatter does
not narrow. `usql` must not name a database, and it will not narrow with a
`LIKE` of its own, because a model that filters can compare case
insensitively. Only the model knows the rule of its product. So the model
filters, and the library applies the model's rule to the rows. Ken's
coordinator and `usql` agreed on this shape.

## What was rejected

A `Narrow` helper that a consumer calls on the result. The consumer must know
which products need it, and that is a database name in `usql`.

A `Binding.Ignores` list that names the parameters a statement does not apply.
It tells the consumer what the model does not do, and leaves the work to the
consumer. It also cannot say how to match.

## Cost

One statement still reads every row of the catalog and the filter runs in Go,
so the work grows with the whole catalog and not with the rows returned. D47's
cost test refuses that for a model that can filter in SQL. For Cassandra
it holds, because the catalog is small and there is no other way. A scratch
keyspace of 300 tables and 1200 columns on Cassandra 3.11 read 336 tables in 3
ms and 1434 columns in 9 ms. `docs/COVERAGE.md` has the figures.

## A behavior change

With every filter at its default, a caller used to get every row, including the
system keyspaces. Now `with_system` is false by default, as it is for every
other model, so those rows are hidden until a caller passes `with_system`.
Nothing in `dbmeta` expected them. The tests in the `test` module that read the
Cassandra model pass a schema or read the fixture, and the parity and
conformance files did not change. The paragraph about ignored filters in
`docs/COVERAGE.md` and the package comment of `models/cassandra` expected an
unfiltered result, and both are rewritten. A consumer that needs the system
keyspaces passes `with_system`.
