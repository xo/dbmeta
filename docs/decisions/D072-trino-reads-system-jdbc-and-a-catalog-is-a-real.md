# D72. Trino reads system.jdbc, and a catalog is a real level

Status: Decided.

Three things had to be decided for `models/trino` rather than found by running
statements. The rest of what it can and cannot answer is measurement and lives
in `COVERAGE.md`.

## The catalog is a real level, and this is the first model to use it

Every other model returns an empty catalog or repeats the database name into
it, because the products have two levels of namespace and `psql` has three.
Trino has all three: a table is `catalog.schema.name`, a catalog is a
configured connector rather than a database, and one server reaches many at
once.

So `models/trino` is the first to answer a catalog filter. `dbmeta.Args` has
carried the field since it was written and nothing had used it. A consumer
that passes `Args{Catalog: "memory"}` to any other model gets every catalog,
because there is only one; passing it to this one narrows.

`Databases` reads `system.metadata.catalogs`, so `\l` lists the catalogs a
server can reach. That is the closest thing Trino has to the question `psql`
asks, and it is more useful than reporting nothing.

## The source is system.jdbc, not the per catalog information_schema

Trino ships an `information_schema` inside every catalog and a `system`
catalog beside them. They differ in reach, and the difference decides this.

A query against `memory.information_schema.tables` sees the memory catalog
alone. The catalog cannot come from a bind parameter, because it is an
identifier in the `FROM` clause, so a filter naming a second catalog would
return no rows rather than an answer. That is a wrong answer wearing the
clothes of an empty one, which rule 13 does not allow. `system.jdbc` spans
every catalog the server has.

It is also richer. `system.jdbc.columns` carries the column comment in
`remarks`, and `information_schema.columns` has no column for a comment.

Views is the exception and it has to be. No cross catalog source holds a view
definition: `system.metadata.materialized_views` covers materialized views
only, and a plain view's definition lives in its own catalog's
`information_schema.views`. Reading every catalog would be one statement per
catalog, which rule 13 forbids. So Views reads the session catalog, and its
parameter description says so where a caller reads it rather than in a
document they will not open.

## A boolean is not an integer

Every other model writes `@with_system = 1` and the server coerces. Trino
applies no implicit conversion between a boolean and an integer and refuses
the statement outright:

```
Cannot apply operator: boolean = integer
```

Seven queries were written the other way first and all seven failed on the
first run against a server, which is the cheapest way for that to be found and
the reason rule 9 exists.

## What this does not decide

The floor. `container/trino.go` holds it with the measurement behind it, and
476 is set by what the memory connector can build rather than by what the
catalog can answer.

Whether `dbtpl` could generate from Trino. It could not, and `DBTPL.md` says
why: no foreign key means no relationship to follow. That is a fact about the
product rather than a decision about the model.
