# D276. The MySQL model reads what a dump needs of a column and a key

Status: Decided.

A dump to the same dialect (D274) must write back what the source said. The
snapshot of a MySQL or MariaDB database lost three facts in the round trip,
that the tests of the script found.

## What was lost

A foreign key. The catalog keeps what it does on update and on delete in
`REFERENTIAL_CONSTRAINTS`, and the model did not read it, so `ON DELETE CASCADE`
came back as nothing.

A generated column. The model reported every column whose `EXTRA` has the word
GENERATED as `s`, which includes a column whose default is an expression, and it
did not read `GENERATION_EXPRESSION`, so the expression of a stored or virtual
column was not in the snapshot.

A default. MySQL gives the text of a string default with no quotes, and gives an
expression default with its quotes escaped and no brackets. MariaDB gives SQL.
Nothing said which a default was, so a script cannot quote it.

## Decision

`Constraint.OnUpdate` and `Constraint.OnDelete` hold the rules of a foreign key.
The MySQL model reads them from `REFERENTIAL_CONSTRAINTS` for MySQL and MariaDB,
and they are absent for any other constraint and for any other model.

For a generated column the `Default` of the MySQL model is the generation
expression, as it is for PostgreSQL, and `Generated` is `s` for a stored column
and `v` for a virtual one. A column whose default is an expression is not
generated. Its `Default` is the expression with the escape removed and in
brackets, except for `CURRENT_TIMESTAMP`, which MySQL takes without brackets.

`Column.DefaultIsLiteral` is true for a MySQL default that is a value to quote,
and false for MariaDB, where the default is SQL. It is absent in every other
model, where `Default` is already SQL.

The gates are the keys of the two products, as D44 requires, so TiDB, Vitess and
SingleStore, which share the queries, do not read a table that they can lack.

## Consequence

A consumer that read `Generated` as `s` for a column with an expression default
now reads nothing for it. That was a fault, because the column is not generated.
The `ON UPDATE` of a timestamp column and the length of an index prefix are still
not read, and `docs/BACKLOG.md` holds them.
