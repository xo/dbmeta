# D146. A query can walk several statements

Status: Amends D47 and D67, amended by D159 and D175.

## The decision

D47 answers each kind with one statement, because a second statement, or a
round trip for each row, costs more as the catalog grows. Impala has no
catalog that a SELECT reads: it lists the tables of one database with SHOW
TABLES IN, and the columns of one table with DESCRIBE. Ken chose on
2026-09-30 to allow a query to walk SHOW statements for Impala, one for each
database, rather than to answer only what one statement can.

`dbmeta.Binding.Walk` is a function a model writes, which runs its
statements through the Queryer and yields rows. It runs each statement to
its end before the next, so it holds one connection at a time. Query.All
runs it with every parameter filled in, and Query.Build returns
ErrSeveralStatements, because there is no one statement to return. A SHOW
statement takes no pattern, so a walk matches the caller's patterns in Go,
with `dbmeta.Like`, which is SQL's LIKE.

## What it is not

It is not a way to answer a kind that one statement answers. Every other
model keeps one statement for each kind, and D47's cost test still holds for
them. A walk exists where the product offers nothing else, and its cost is
written beside it: one statement for each database, and one for each table
where the walk goes deeper. Ken chose that Tables reads DESCRIBE FORMATTED
for each table, for its type and its comment, which is one more statement
for each table.
