# D139. A column has a collation

Status: Decided.

## What was found

psql's `\d` prints a Collation column between the type and the nullability.
`dbmeta.Column` had no collation, so usql left the column out. usql found it
on 2026-09-30.

## The decision

Ken chose on 2026-09-30 to add it. `Column.Collation` is a
`sql.Null[string]`, and every model's Columns selects it as the last column,
`collation`. It passes the cost test in D47, because it is a column of the
same catalog row, or one join to the collation catalog on PostgreSQL.

A model reports the column's collation as the catalog records it:

- PostgreSQL and CockroachDB read `pg_collation` by `attcollation`, so a text
  column that chose none reports `default`, and a type that cannot be
  collated reports none.
- MySQL, MariaDB, TiDB and Vitess read `collation_name`.
- SQL Server reads `sys.columns.collation_name`.
- Oracle reads `collation` from 12.2, where it arrived, and pads it with NULL
  before that, with `Field.Min` set.
- Firebird reads `RDB$COLLATIONS` by the column's collation or its domain's.
- The shared information_schema model reads `collation_name`, which the
  standard defines, as a clause a profile can override.

Every other model selects NULL, and the field says why: the product has no
collation on a column, or, for SQLite, keeps it only in the DDL text.

psql prints a collation only where it differs from the type's default. That
is a presentation, and a caller decides it (rule 13).
