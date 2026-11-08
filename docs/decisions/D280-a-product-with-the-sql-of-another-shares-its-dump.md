# D280. A product with the SQL of another shares its dump

Status: Decided.

D274 built the dump for PostgreSQL, SQLite, DuckDB and MySQL. Ken asked for as many
of the products that dbrun starts as one pass can reach. Five of them speak the SQL
of one of the four, and one has a SQL of its own.

## Decision

`dump.RegisterLike` registers a product with the type functions and the style of
another one, and lets the model change the copy. The model calls it from `init`,
after the model it is like has registered. The pairs are:

- CockroachDB is like PostgreSQL.
- TiDB, SingleStore and Vitess are like MySQL.
- rqlite and libSQL are like SQLite.

SingleStore changes the copy. It has no foreign key, and it refuses a unique key
that does not hold every column of its primary key, which is its shard key. The new
style field `RefuseConstraint` returns the reason, and the script holds a statement
with that reason as its note and no SQL. dbmeta does not write the unique key as an
ordinary index, because that changes what the table allows without a word.

SQL Server has type functions and a style of its own, in `models/sqlserver`. The
facts that the work found:

- `data_type` of the model now holds the digits of a second for `datetime2`,
  `datetimeoffset` and `time`, because the dump cannot keep them without it.
- The `default` of a computed column is its expression, because a computed column
  has no default.
- A foreign key has `on_update` and `on_delete`. The model read neither before, so
  every foreign key of SQL Server lost its rule in a dump.
- `ViewDependencies` reads `sys.sql_expression_dependencies`.
- A default constraint is the default of its column, so the reader does not keep it
  as a constraint. An included column of an index is read as a plain column.
- The new style fields `PrimaryKeyNotNull`, `NoRestrict`, `Boolean`,
  `IdentityInsert` and `TypedNullBinary` hold what is peculiar to it. The loader
  gives `Prologue` and `Epilogue` for `SET IDENTITY_INSERT`, and the client must run
  them on the connection that runs the inserts.

A product with the SQL of another is Source and Target as soon as it is registered.
Its test is a side in `test/dump_cross_test.go`, which runs the schema and the rows
against every other side.
