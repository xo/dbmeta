# D287. A dump writes system versioned tables and the values of CLR types

Status: Decided.

## Decision

A dump of SQL Server writes a system versioned table with its history, and it
writes a spatial or hierarchyid value as the binary form of the type. A dump of
the northwind fixture failed on both. The `usql` session found them, and Ken
asked for the fix as a point release on 2026-11-09.

## System versioned tables

A system versioned table has a history table that the server fills. The reader
used to skip the table with the reason "this kind of relation is not read" and
to dump the history table as a plain table. A copy had the history and not the
table.

- `Table.HistorySchema` and `Table.HistoryTable` name the history table. They
  come from `sys.tables.history_table_id` with one join, and they are absent for
  any other table and before SQL Server 2016.
- `Column.Generated` is now `row start` or `row end` for a period column, and
  `rowversion` for a column that the server numbers.
- `Table.Versioning` in the snapshot holds the history table and the two period
  columns. The history table stays a table of the snapshot.

The script makes the current table with `PERIOD FOR SYSTEM_TIME` and with
versioning off. The server refuses a value for a period column while the table
has a period, so the Data stage drops it. The rows of both tables load, and the
current rows keep the instants of the source. The Post stage makes the period
again and runs `SET (SYSTEM_VERSIONING = ON (HISTORY_TABLE = ...))`, after the
constraints and the indexes.

Another product gets two plain tables and a note on the statement.

## Values of CLR types

The driver gives `geography`, `geometry` and `hierarchyid` as their binary form.
The loader read the bytes as text, and the first NUL stopped the dump.

- `Convert` keeps a value that is not text as bytes, for the kinds that fall
  through to text. A value is binary when it holds a control character other than
  a tab or a line break, or when it is not UTF-8.
- `hierarchyid` is a binary kind now, and its bytes can hold no NUL.
- The style hook `CastBinary` names the type to cast to. SQL Server writes
  `CAST(0x... AS geography)` in a literal and `CAST(@p1 AS geography)` in
  `InsertArgs`.
- A column that the server numbers is left out of the INSERT for the same
  product, and its values move to another product.

## What it does not do

- A hidden period column is written as a visible one.
- A table of a schema that the filter leaves out loses its versioning, and the
  snapshot says so in `Skipped`.
- The alias type `hr.email` and the table type `hr.code_list` are written since D288.
