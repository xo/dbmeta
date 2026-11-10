# D296. A dump writes the rest of what the catalogs hold

Status: Amends D294.

The maintainers asked on 2026-11-10 that the dump be correct for every product that
has the dump fixture, and that the dump backlog be finished. This is what closed. The
files in `test/testdata/dump_fixture/` are empty for every product, and
`docs/BACKLOG.md` holds what stays open.

## New kinds

- `Events`, the scheduled events of MySQL and MariaDB.
- `XMLSchemaCollections`, `PartitionFunctions` and `PartitionSchemes` of SQL Server.
- `MaterializedViewLogs`, the logs of the changes to an Oracle table.

## New fields

- `Column.OnUpdate`, `Column.DefaultName`, `IndexColumn.PrefixLength`,
  `Index.Invisible`, `Table.Collation`, `Table.Definition`,
  `Trigger.ActionOrder`, `Trigger.SQLMode`, `Trigger.Placement`,
  `Sequence.LastValue`, `EventTrigger.Definition`, `Policy.PolicySchema` and
  `PartitionedTable.KeyColumns`.

## MySQL and MariaDB

- A column keeps INVISIBLE and ON UPDATE, an index keeps its prefix length, its
  functional key parts and its INVISIBLE or IGNORED state, and a table keeps its
  engine, collation and options.
- A trigger keeps its position among the triggers of its event with FOLLOWS, and
  runs under its own `sql_mode`. An event is written back.
- MySQL and MariaDB report RESTRICT and NO ACTION for the same key, so the reader
  reports RESTRICT for both.
- The order of the views and the routines of a snapshot is the same for two scripts of
  one snapshot, because the words that decide it are sorted.

## SQL Server

- A default constraint that a person named keeps its name. A sequence keeps its
  position, by drawing the values that it gave, because RESTART WITH changes its
  start. A trigger keeps its place as first or last for an event.
- The database trigger, the security policy, the XML schema collection, the
  partition function and the partition scheme are written, and a table is laid out by
  its scheme. Every partition is put on the filegroup PRIMARY.

## PostgreSQL

- An ordinary aggregate, an event trigger and the row level security policies are
  written, and the value of a sequence that no column owns is set.

## Oracle

- A DDL trigger and a materialized view log are written. The table that holds a log is
  left out. A log that holds named columns is listed as skipped.

## SQLite

- The checks of a table are found in its text, and a unique index that a person made
  is an index and no constraint. The counter of AUTOINCREMENT is set. The model does not
  call that counter a sequence, so the dump reads `sqlite_sequence` itself. A view can read
  another view, so the views are ordered by the names in their text.

## DuckDB

- A type that a person made from another type is written. A sequence keeps its
  position, by drawing its values.

## What stays open

`docs/BACKLOG.md` lists it: roles and grants, the subpartitions of Oracle, a struct type
of DuckDB, and the products that the fixture does not cover.
