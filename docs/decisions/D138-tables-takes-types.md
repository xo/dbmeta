# D138. Tables takes types, bound as one string

Status: Decided.

## What was found

`dbmeta.Args` had a `Types` field, and `Map` sent it as `types`, but no
model's Tables took that parameter. A caller that passed it got
ErrUnknownParam from every model, and `docs/COMMANDS.md` told callers to use
it. usql found it on 2026-09-30 and filtered on `Table.Type` itself.

## The decision

Ken chose on 2026-09-30 to implement it. Every model's Tables takes `types`,
declared by `dbmeta.TypesParam`, and returns only the relations whose
`Table.Type` is one of the types named. An empty list means every type.

No driver here binds a slice, so a list is bound as one string, with its
items joined by commas. `bind` does the join for a `[]string`. A statement
matches one item by the commas on either side of it, which `dbmeta.InList`
writes for a product that joins strings with `||`. The items are `Table.Type`
values, and none of them holds a comma.

ClickHouse and QuestDB refuse a LIKE whose pattern is not a constant, so
ClickHouse splits the list and searches it, and QuestDB searches it with
strpos. A product that joins strings another way writes the same
condition itself: MySQL, TiDB and Vitess with CONCAT, and SQL Server with `+`.
Oracle and Exasol read the empty string as NULL, so their test for an empty
list is IS NULL. Firebird casts the parameter, as its other filters do. A
statement that is a union filters each branch by the type that branch gives.
The shared information_schema model makes the condition a clause, so a profile
can spell it its own way. Cassandra declares the parameter and narrows
nothing, as it does with every filter (D62).

`scanEveryQuery` checks it on every product it runs against. It reads every
table, and then each type on its own, the first two together, and a type
that does not exist, and fails when a filtered read does not return exactly
the rows of the types it named. `TestEveryTablesTakesTypes` in `all` fails
when a model's Tables does not take it.
