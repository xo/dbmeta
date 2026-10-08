# D198. Describe fields for relations, indexes, columns and functions

Status: Decided.

## The decision

usql brings its describe commands up to psql 18, and it measured what the
fields of dbmeta lack on postgres-18 against psql 18.6. These fields are new.
The PostgreSQL model fills them. Every other model leaves them NULL, or false
for `Leakproof`, and no other model changed.

| Kind | Field | Type | What it holds |
| --- | --- | --- | --- |
| Tables | `Owner` | `sql.Null[string]` | `pg_get_userbyid(relowner)` |
| Tables | `Persistence` | `sql.Null[string]` | permanent, unlogged or temporary, as psql spells it |
| Tables | `AccessMethod` | `sql.Null[string]` | `pg_am.amname` through `relam`, from release 12 |
| Tables | `Size` | `sql.Null[int64]` | `pg_table_size`, in bytes |
| Tables | `Rows` | `sql.Null[int64]` | `reltuples`, rounded to a whole number |
| Indexes | `Owner`, `Persistence`, `Size` | as for Tables | the same expressions on the index relation |
| Indexes | `Predicate` | `sql.Null[string]` | `pg_get_expr(indpred, indrelid, true)` for a partial index |
| Indexes | `Valid`, `Clustered`, `ReplicaIdentity` | `sql.Null[bool]` | `indisvalid`, `indisclustered`, `indisreplident` |
| Indexes | `Deferrable`, `InitiallyDeferred` | `sql.Null[bool]` | `condeferrable` and `condeferred` of the constraint that owns the index |
| Columns | `Storage` | `sql.Null[string]` | plain, main, external or extended, from `attstorage` |
| Columns | `Compression` | `sql.Null[string]` | pglz or lz4, from `attcompression`, from release 14 |
| Columns | `StatsTarget` | `sql.Null[int64]` | `attstattarget`, absent for the default |
| Functions and Aggregates | `Leakproof` | `bool` | `proleakproof` |
| Functions and Aggregates | `Prosrc` | `sql.Null[string]` | `prosrc` for every language |

## Why each type

`Size` is a number of bytes and not the text of `pg_size_pretty`. Database and
Tablespace carry the text, because psql prints it for them and nothing needs
the number. A table size is different. A caller adds sizes up, sorts by them
and picks a unit. Rule 2 says never to return prose as the only form of a fact.
A caller that wants psql's text formats the number. The expression is
`pg_table_size`, which is what psql 18 prints for `\dt+` and `\di+`. It counts
the main data, the free space map, the visibility map and the TOAST table, and
not the indexes. A view, a foreign table and a partitioned table have no file
of their own, so PostgreSQL answers 0 for them. dbmeta passes the 0 through,
as psql does. The function returns NULL when the relation was dropped while the
statement ran, so the type must allow NULL.

`Rows` is `reltuples` and it is an estimate, never a count. Before release 14 a
table that was never analyzed holds 0. From release 14 it holds -1, so a 0
means an empty table. dbmeta passes the value through and does not turn -1
into NULL, because that hides the difference. CLUSTER and VACUUM store a
count too, so a table that was never analyzed can hold 0 on release 14.

`Persistence`, `Storage` and `Compression` keep psql's words. `Compression` is
NULL when the column uses the server default, which is what psql prints, and on
every release before 14. `Field.Present` says which of the two it is.
`StatsTarget` is `NULLIF(attstattarget, -1)`, because releases before 17 store
-1 for the default and release 17 stores NULL.

`Valid`, `Clustered` and `ReplicaIdentity` are `sql.Null[bool]` and not `bool`,
because another model that pads a `bool` with false says that its index is
not valid. `Deferrable` and `InitiallyDeferred` are NULL for an index that no
constraint owns. The join reads `pg_constraint` on `conindid` and `conrelid`,
and only for the types p, u and x, because a foreign key names the unique index
it reads in the same column and gives one index two rows. A primary key,
a unique constraint and an exclusion constraint each own one index, so one
index stays one row.

`Leakproof` is a plain `bool`, as Ken asked. A product with no such property
answers false, which is true of it.

`Prosrc` is new beside `Source`, and `Source` did not change. `Source` still
holds `prosrc` for the internal and C languages only (D147). `Prosrc` holds it
for every language, so it holds the body of a SQL or PL/pgSQL function. An
aggregate holds `-` there, which is what the catalog holds.

## The relation types

`Table.Type` for each `relkind`, after this change. Only `p` changed.

| relkind | Type | Listed by Tables |
| --- | --- | --- |
| r | table | yes |
| p | partitioned table, which was table | yes |
| v | view | yes |
| m | materialized view | yes |
| S | sequence | yes |
| f | foreign table | yes |
| c | none | no, a composite type is `Types` |
| i and I | none | no, an index is `Indexes` |
| t | none | no, a TOAST table is never listed |

psql says `partitioned table` for `p`, and `partitioned_tables` is a kind of
its own, so a caller can now tell the two apart. The change has a cost. The
`types` parameter of Tables matches on `Table.Type`, so `types=table` no longer
returns a partitioned table. A caller that wants psql's `\dt` passes
`table,partitioned table`. usql must do that.

## What each release fills

| Release | What is NULL |
| --- | --- |
| 9.6 and 11 | `AccessMethod` and `Compression` |
| 12 and 13 | `Compression` |
| 14 and later | nothing, except what the data leaves empty |

`Field.Min` is 12 for `access_method` and 14 for `compression`, and a test
checks that it agrees with the fragment. Every statement keeps the same set of
columns on every release.

## The cost

Measured on PostgreSQL 15.19 with `EXPLAIN (ANALYZE)` in a scratch database of
5000 tables with a primary key each, 1000 more indexes, 2000 views, 2000
functions and 200000 rows in one table. `pg_class` held 23410 rows.

| Statement | Before | After | Where the difference is |
| --- | --- | --- | --- |
| Tables, 7000 relations | 25 ms | 143 ms | `pg_table_size`, about 17 microseconds for each relation |
| Indexes, 6000 indexes | 25 ms | 80 ms | `pg_table_size`, about 9 microseconds for each index. The join to `pg_constraint` is one hash join and adds about 1 ms |
| Columns, 52406 columns | 86 ms | 87 to 95 ms | none that a measurement separates from noise |
| Functions, 2000 and 3244 system | 9.7 ms | 8.6 ms | none |

`pg_table_size` reads the sizes of the files of one relation and never reads
the data. The 26 MB table and an empty one cost the same. Its cost grows with
the number of relations the statement returns, which is what D47 allows, and a
caller pays it whether or not it reads the field. A filter on the schema or the
name narrows the rows before the function runs, because the function is in the
select list. The other fields are columns of a row the statement already read.

## Parity and conformance

`TestPrivilegeParity` on PostgreSQL 15 gives the same sections as before. The
administrator, the owner and the grantee all see the same values for the new
fields, because `pg_table_size` and the catalog columns check no privilege. The
conformance golden is unchanged. `canonical.go` records that the canonical
projection drops `Column.Storage`, `Column.Compression` and
`Column.StatsTarget`, because only PostgreSQL has them.

## CockroachDB

CockroachDB shares the postgres statements for Tables, Columns, Indexes and
Functions. The coordinator ran the test module against CockroachDB 26.2.7 and
26.3.2 on 2026-10-09, and both pass. One function was missing: 26.2 has no
`pg_table_size`, so the statements read `size` through a Choice that gates on
the CockroachDB release, and `size` is NULL before 26.3. The other new columns
exist there, and the shared statements answer them as the catalog allows. The
fixture steps that build the new objects are left out of the CockroachDB
fixture with the reason "not measured on CockroachDB", and the new tests skip on
them, so the values of those fields are not checked on CockroachDB.

## What was left out

The fixture has no invalid index, because only a failed CREATE INDEX
CONCURRENTLY builds one. The test reads `Valid` as true.
