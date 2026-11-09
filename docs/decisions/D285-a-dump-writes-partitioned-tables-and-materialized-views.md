# D285. A dump writes partitioned tables and materialized views

Status: Decided.

## Decision

The dump reads a partitioned table and a materialized view, and writes them back
for the same product. Ken asked for both on 2026-11-09, after usql found that
dbmeta v0.39.1 skipped them. The default case of the relation switch in
`dump/read.go` put them in `Skipped`.

## Why it was wrong

A partitioned table of PostgreSQL was skipped, and its partitions were dumped as
plain tables with their rows. In the copy an INSERT into the parent failed, a
trigger that wrote to the parent broke every later write, and a row no longer
went to its partition by its date. A materialized view was skipped with no
definition at all.

## What the snapshot holds

- `Table.Partitioning` is the strategy and the key of a split table.
- `Table.PartitionOf` is set on a PostgreSQL partition. It names the parent and
  holds the bound.
- `Partitioning.Parts` lists the partitions that live inside a MySQL table.
- `View.Materialized` and `View.Indexes` hold a materialized view and its indexes.

The version of the format stays 1. The new fields are all optional and an older
file reads as before.

## What the script does

For the same product, the script writes the parent with `PARTITION BY` and each
partition with `PARTITION OF` after its parent. PostgreSQL makes the keys and the
indexes of a partition from the parent, so the script leaves out any key, index
or trigger of a partition that the parent already has. It writes an index of a
partitioned table without `ONLY`, so that it reaches every partition.

A materialized view is written in the Post stage with `WITH DATA`, after the
rows, after the views that it reads and before its own indexes.

For another product the script writes one table for each partitioned table, and a
plain view for each materialized view, with a note on the statement. The rows are
the same in both cases.

## Rejected

- pg_dump writes each partition as a table and then `ATTACH PARTITION`. That needs
  the keys of the partition and the names of its indexes to match, and an index
  attach for each one. `PARTITION OF` gives the same result with fewer statements.
- A `NO DATA` view and a `REFRESH` at the end, as pg_dump does. The Post stage
  already runs after the rows, so one statement does the same work.
- Writing the partitions of SQL Server and Oracle. They are a function and a scheme
  or a clause of storage. They stay plain tables until a client asks.

## Not done

The snapshot does not say whether a materialized view held rows in the source.
A view that was empty on purpose is written with its rows. The subpartitions of
MySQL are not written.
