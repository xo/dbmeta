# D303. A basic dump of ClickHouse and Spanner

Status: Decided.

D302 put the warehouses with a DDL of their own in tier 2. ClickHouse and Spanner both have a
container that `dbrun` starts, so this decision builds them. Snowflake, BigQuery and Databricks are
hosted services that need a credential that the maintainers enter (D117), so they wait.

## ClickHouse

A database of ClickHouse is the schema, so a schema of the snapshot is made with CREATE DATABASE.

- A table needs an engine and an order. The style writes `ENGINE = MergeTree ORDER BY` with the
  primary key of the source. A table with no key, or with a key that can be NULL, gets `tuple()`,
  because ClickHouse refuses a key that can be NULL.
- Whether a column can be NULL is part of its type. The column hook wraps the type in `Nullable`, except for an
  array, which cannot be. The parser takes `Nullable` and `LowCardinality` off, because they say how a column is
  stored and not what it holds.
- Every constraint is left out and the script says so. The primary key is written as the order. ClickHouse
  has no foreign key and no unique key. Its indexes skip data and have another form, so the dump
  writes none.
- The catalog flags the columns of the primary key and holds no constraint for it. The new field
  `Style.PrimaryKeyFromColumns` makes the reader build the constraint from the flags, in the order of the
  columns. The order of the key in the table can differ.
- A string literal escapes the backslash and the quote with a backslash.
- ClickHouse has no binary type, so bytes go in a String, which is Lossy. It has no time of day and no
  interval. A Date is written as Date32, which holds the years that PostgreSQL holds. A timestamp is a
  DateTime64, with six digits when the source names none.

## Spanner

- The default schema has the name that is the empty string, and a named schema is made with CREATE SCHEMA.
- The primary key is written after the columns. A table with no key gets an empty key, which holds one
  row, so a second row fails and the dump says nothing false.
- Every integer is INT64. A NUMERIC has 38 digits and 9 after the point, so a decimal with more digits
  after the point has no equivalent. A TIMESTAMP is an instant in UTC, so a timestamp with no zone is
  written as the instant with the same clock in UTC.
- A timestamp literal with no offset is read in the time zone of the session, so the style writes the
  offset with `TimeLiteral`. This moved every time by the offset of Pacific time before it was found.
- The driver gives a NUMERIC as a rational number that no other product reads, so the style reads it
  as text.
- A basic dump writes no constraint other than the key and no index.
- Spanner Omni takes seconds for each change of schema, so the test of a pair takes about thirty seconds.
  The test needs the build tag `omni` (D231).

## Test

`TestABasicDumpMovesATable` runs both against PostgreSQL, MySQL, DuckDB, CrateDB, QuestDB and one another.
The pair of QuestDB to Spanner is skipped, because QuestDB has no key and Spanner holds one row of a table
with no key. The test reads a timestamp by its instant, so a machine outside UTC does not move it.
