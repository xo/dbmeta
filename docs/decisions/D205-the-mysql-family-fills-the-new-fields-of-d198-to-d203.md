# D205. The MySQL family fills the new fields of D198 to D203

Status: Decided.

## The decision

D198, D199, D201 and D203 added fields and kinds that only PostgreSQL filled.
Ken said on 2026-10-10 to refresh every model. This decision does it for the
MySQL family: MariaDB and MySQL (`models/mysql`), TiDB, SingleStore and
Vitess. TiDB, SingleStore and Vitess share the statements of `models/mysql`
with `Query.Share`, so one change reaches all five products, and a fragment
keyed on a product tells them apart. Only the fields with a source in one
statement are filled. The others stay NULL, and the reason is below.

## The audit

| Field | MariaDB | MySQL | TiDB | SingleStore | Vitess |
| --- | --- | --- | --- | --- | --- |
| `Table.Owner` | no source | no source | no source | no source. `CREATE_USER` names who made the table, which is not an owner | no source |
| `Table.Persistence` | `TABLE_TYPE` is TEMPORARY for a temporary table of this session | `permanent`. A temporary table is not in the catalog | NULL. A global temporary table is a base table and a local one is not listed | `TABLE_TYPE` is TEMPORARY TABLE | `permanent` |
| `Table.AccessMethod` | `ENGINE` | `ENGINE` | `ENGINE`, which is InnoDB on every table | `STORAGE_TYPE`, because `ENGINE` is MemSQL on every table | `ENGINE` |
| `Table.Size` | `DATA_LENGTH + INDEX_LENGTH` | the same | the same | the same | the same |
| `Table.Rows` | `TABLE_ROWS` | the same | the same | the same | the same |
| `Table.Options` | `CREATE_OPTIONS` | the same | always empty | always NULL | the same as MySQL |
| `Table.RowSecurity` | no source | no source | no source | no source | no source |
| `Index.Using` | `INDEX_TYPE` | `INDEX_TYPE` | `INDEX_TYPE`, BTREE for every index | `INDEX_TYPE` | `INDEX_TYPE` |
| `Index.Size` | needs PROCESS or the mysql schema | the same | no source | no source | the same as MySQL |
| `Index.Clustered` | needs PROCESS | needs PROCESS | no source | no source | needs PROCESS |
| `Index.Valid` | no source. `IGNORED` is a choice | no source. `IS_VISIBLE` is a choice | no source | no source | no source |
| `Index.Predicate`, `Owner`, `Persistence`, `Options`, `Definition`, `Deferrable`, `InitiallyDeferred`, `ReplicaIdentity` | no source | no source | no source | no source | no source |
| `Column.Storage`, `StatsTarget` | no source | no source | no source | no source | no source |
| `Column.Compression` | `COLUMN_TYPE` carries the comment of a compressed column, from 10.3 | no column compression | no source | no source | no source |
| `Constraint.Enforced` | no source. There is no `ENFORCED` column from 10.6 to 12.3 | `TABLE_CONSTRAINTS.ENFORCED` from 8.0.16 | no source. TiDB claims 8.0.11 | no source. SingleStore claims 5.7 | the tablet is MySQL 8.4, so it reads `ENFORCED` |
| `Function.Leakproof` | false | false | not answered | false | not answered |
| `Function.Prosrc` | `ROUTINE_DEFINITION` | the same | not answered | the same | not answered |
| `Partitions` | `PARTITIONS` | `PARTITIONS` | `PARTITIONS` | no source. It lists nothing | `PARTITIONS`, through the tablet |
| `PartitionedTables.DirectSize`, `TotalSize` | sum of `DATA_LENGTH + INDEX_LENGTH` | the same | the same | not answered | the same |
| `PartitionedTables.AccessMethod`, `Table` | no source here. `PARTITIONS` has no engine, and a join to `TABLES` opens every table | the same | the same | not answered | the same |
| `Policies`, `NotNulls`, `Inherits`, `Rules` | none of the family has them (D43) | | | | |
| Schema, Database, Sequence, Type fields | no source. `SEQUENCES` has no cache size on MariaDB | | | | |

Gemini and DeepSeek were asked on 2026-10-10, as rule 14 requires, about the
size and the clustered flag of an index, the compression of a column, an
ignored index, and the owner of a table. They agreed on `innodb_index_stats`,
`INNODB_INDEXES` and `INNODB_SYS_INDEXES`, and that no owner exists. I ran the
leads. As a user with SELECT on one schema, both InnoDB views answer
"Access denied; you need (at least one of) the PROCESS privilege(s)", and
`innodb_index_stats` is a table of the mysql schema. A field that a lesser
principal cannot read makes the statement refuse for that principal, which
is what D61 found for six MariaDB queries. So `Index.Size` and
`Index.Clustered` stay NULL, and `docs/BACKLOG.md` says what a fragment needs.

## What the statements read

`Tables` adds five columns: `persistence`, `access_method`, `size`, `rows` and
`options`. `size` is `CAST(DATA_LENGTH + INDEX_LENGTH AS SIGNED)`, and the
three numbers are NULL for a view on MariaDB and MySQL. SingleStore answers 0
for a view, so the statement names a view and answers NULL for it. `options` is
`NULLIF(CREATE_OPTIONS, '')`. MariaDB writes `row_format=COMPRESSED
key_block_size=4` and `partitioned`, separated by a space and not by a comma.
The text is the catalog text and `Options` keeps it, as the PostgreSQL model
keeps `reloptions`.

`Columns` adds `compression`. MariaDB from 10.3 writes `/*M!100301
COMPRESSED*/` into `COLUMN_TYPE`, and the statement turns it into `zlib`, the
only method. `Field.Min` and the key are the gate, so MySQL pads NULL.

`Indexes` adds `using`, which is the `INDEX_TYPE` the statement already reads
for `Type`. `Constraints` adds `enforced`, with a fragment gated on the MySQL
key at 8.0.16. `Functions` adds `prosrc`, which is `ROUTINE_DEFINITION`, the
same text as `Source`.

`Partitions` is new. `PARTITIONS` has one row for each partition, or for each
subpartition when a table has them. The statement is a `UNION ALL` of two reads.
The first folds the rows of one partition into a row with the type `partition`
and `Partitioned` true when it has subpartitions. The second keeps each
subpartition as a row with the type `subpartition`. `Bound` is
`PARTITION_DESCRIPTION`: `2027` for `LESS THAN (2027)`, `MAXVALUE`, or `1,2,3`
for a list. It is NULL for a hash or key partition, which has no bound, and for
a subpartition. The parameters are `schema`, `parent` for the table, `name` for
the partition and `with_system`. The rows come in partition order.

`PartitionedTables` adds `direct_size` and `total_size`. They are the same sum,
because a partition holds its subpartitions and the view lists the leaves.

## What each release fills

| Release | What is NULL |
| --- | --- |
| MariaDB 10.6 to 12.3 and 13.0 | `Constraint.Enforced`. `Column.Compression` is filled from 10.3 |
| MySQL 8.4, 9.7 and 26.7 | `Column.Compression` and the fields in the audit with no source |
| TiDB 7.5.8, 8.1.2 and 8.5.8 | `Persistence`, `Options`, `Enforced` and the fields with no source. A table that was never analyzed holds 0 in `Size` and `Rows` |
| SingleStore 9.0 and 9.1 | `Options`, `Enforced`, and no `Partitions` |
| Vitess 23.0.7 and 24.0.4 | the same as MySQL 8.4, and no function |

The kinds answered grow by one: MariaDB 30, MySQL 27, TiDB 20 (19 on 7.5.8 and
8.1.2), Vitess 21, SingleStore 23.

## The cost

`Table.Size` and `Rows` make MariaDB open each table to read its statistics,
which the old statement did not do. The cost grows with the tables that the
statement returns and never with the rest of the catalog, which D47 allows.
A filter on the schema or the name narrows the rows before the table is opened.
Every number is the time of the whole statement in milliseconds, with the
fields and without them. "Without" selects the name, the type and the comment.

| Product | Tables | Without | With, cold | With, warm | One table by name |
| --- | --- | --- | --- | --- | --- |
| MariaDB 12.3 | 3000 InnoDB tables with two secondary keys each | 1.9 | 1341 | 30 | 0.17 |
| MySQL 26.7 | 2000 of the same, with `information_schema_stats_expiry` at 0 | 7.2 | 14.5 to 16.9 | | 0.24 |
| TiDB 8.5.8 | 2000 of the same | 3.3 | 58 | 59 | 1.2 |
| SingleStore 9.1 | 600 rowstore tables | 80 | 66 | 44 | 57 |

Cold means a flushed table cache on MariaDB. The cost is the open of each
table, about 0.45 ms. A caller that lists thousands of tables on a cold server
pays it whether it reads the fields or not. MySQL keeps the statistics in a
cache that expires in a day, so it pays little. SingleStore shows no
difference that the noise does not hide. `PARTITIONS` on MariaDB 12.3 took 45
ms cold and 28 ms warm for the 3000 tables, and 0.3 ms for one table, and on
TiDB it took 61 ms for 2000 tables. Vitess passes the statement to a MySQL, so
its cost is MySQL's and I did not measure it apart. `Partitions` is the same
read as `PartitionedTables`, split in two.

## Parity and conformance

`TestPrivilegeParity` on MariaDB 10.6 and 13.0 and on MySQL 26.7 gives the
sections that were recorded before, and `-update` changed nothing. A grantee
reads the same values as the administrator for every new field, because each is
a column of `TABLES`, `STATISTICS`, `COLUMNS`, `TABLE_CONSTRAINTS`, `ROUTINES`
and `PARTITIONS`, and those views filter themselves. The privilege that
`Index.Size` and `Clustered` need is the reason they are not here. The TiDB
and SingleStore sections are unchanged. The conformance golden is unchanged,
because the canonical projection reads the core columns and constraints only.

## The tests

`test/mysql_fields_test.go` holds the checks, which MariaDB, MySQL, TiDB, Vitess
and SingleStore share, and each product has a test of its own that says what it
expects (`tidb_fields_test.go`, `vitess_fields_test.go`,
`singlestore_fields_test.go`). The MySQL fixture gains six steps: a table with
`ROW_FORMAT=COMPRESSED`, a memory table with a hash index, a table with a full
text index, a list partitioned table, a table with subpartitions, and a table
with a compressed column from MariaDB 10.3. TiDB leaves out the subpartitioned
table, because it ignores SUBPARTITION BY. SingleStore gains a table with a
full text index. The tests read every field back as a typed value and check the
fields with no source for NULL.

I ran `dbrun test` on every release of the Tested tier and of the Nightly tier:
MariaDB 10.6, 10.11, 11.4, 11.8, 12.3 and 13.0, MySQL 8.4, 9.7 and 26.7, TiDB
7.5.8, 8.1.2 and 8.5.8, SingleStore 9.0 and 9.1, and Vitess 23.0.7 and 24.0.4.
All pass.

## What was left out

A temporary table of MariaDB is listed only to the session that made it, with
the type `temporary`. `Table.Type` already says so, and `Persistence` agrees.
No test builds one, because the fixture runs on a pool of connections.

`Column.DataType` still holds the comment that MariaDB puts in `COLUMN_TYPE`.
`docs/BACKLOG.md` records it.
