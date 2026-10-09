# D206. SQL Server and Oracle fill the describe fields

Status: Decided.

## The decision

D197 to D203 added fields and kinds that only the PostgreSQL model filled. Ken
said on 2026-10-10 to refresh every model. This decision does it for SQL Server
and Oracle. Each product fills a field when one statement reads a source for it
and the cost stays bounded (D47, rule 13). A product with no source leaves the
field NULL, and a `bool` that has no source stays false. Nothing is invented.

I wrote every statement against a running server. SQL Server ran on 2017,
2019, 2022 and 2025, and Oracle ran on 11g, 21c, 23ai and 26ai. The old SQL
Server releases that need a Windows machine did not run. Their branches are
checked by `TestDescribeFieldsNameNothingTheReleaseLacks` against a version
set, and nothing else. Oracle 18c and 19c are Verified and did not run.

Gemini and DeepSeek agreed with the audit. DeepSeek named column statistics and
`sys.column_store_segments` for SQL Server. Segments hold an encoding for a
segment of a column and not a setting of a column, so Column.Compression stays
NULL. Gemini named `USER_SEGMENTS` and not `ALL_SEGMENTS` for Oracle, which
matches what the server has.

## The audit

"Source" says what the statement reads. "No source" says why the field is NULL
or false.

| Product | Field | Source |
| --- | --- | --- |
| SQL Server | Table.Owner | `sys.tables.principal_id`, and the owner of the schema when it is NULL |
| SQL Server | Table.Persistence | temporary for a `#` name in tempdb, unlogged for a memory optimized table with `durability` of SCHEMA_ONLY, permanent otherwise |
| SQL Server | Table.AccessMethod | `sys.indexes.type_desc` of index 0 or 1: heap, clustered or clustered columnstore. memory optimized from 2014 |
| SQL Server | Table.Size | `sys.allocation_units` through `sys.partitions` and `sys.internal_partitions`, in bytes. NULL for a view and a memory optimized table |
| SQL Server | Table.Rows | `sys.partitions.rows`, summed |
| SQL Server | Table.Options | `lock_escalation_desc` when it is not TABLE, and the data compression of the partitions |
| SQL Server | Table.RowSecurity, RowSecurityForced | an enabled row in `sys.security_policies` with a predicate in `sys.security_predicates`, from 2016 |
| SQL Server | Index.Persistence | the same rule as the table |
| SQL Server | Index.Size | the same allocation units, for the index |
| SQL Server | Index.Predicate | `sys.indexes.filter_definition` |
| SQL Server | Index.Valid | `is_disabled = 0 AND is_hypothetical = 0` |
| SQL Server | Index.Clustered | `type` of 1 or 5 |
| SQL Server | Index.Deferrable, InitiallyDeferred | false for the index of a primary key or a unique constraint. NULL for any other index |
| SQL Server | Index.Options | `fill_factor`, `is_padded`, `ignore_dup_key`, the two lock flags and the data compression, when they are not the default |
| SQL Server | Index.ConstraintType | p for `is_primary_key` and u for `is_unique_constraint` |
| SQL Server | IndexColumn.Include | `is_included_column` |
| SQL Server | Constraint.Enforced | `NOT is_disabled` for a foreign key and a check. true for a key and a default |
| SQL Server | Sequence.CacheSize | `sys.sequences.cache_size` |
| SQL Server | PartitionedTable.Owner, AccessMethod, DirectSize, TotalSize | the same sources as the table |
| SQL Server | Partitions (new kind) | `sys.partitions` with `sys.partition_functions` and `sys.partition_range_values` |
| SQL Server | Policies (new kind) | `sys.security_policies` and `sys.security_predicates`, from 2016 |
| SQL Server | Column.Storage, Compression, StatsTarget | no source. A column has no storage mode. Compression belongs to a partition. A statistics target is per statistics object and the server picks it |
| SQL Server | Table.Rows for a memory optimized table | NULL, because `sys.partitions` holds 0 for it |
| SQL Server | Index.Owner, ReplicaIdentity, Definition, Using, ConstraintDefinition, ConstraintPeriod, TableVisible | no source. An index has no owner. No function writes the statement of an index |
| SQL Server | Function.Leakproof, Prosrc | no source. `Definition` holds the whole statement |
| SQL Server | NotNulls, Inherits | no source. SQL Server has no name for a NOT NULL and no inheritance |
| SQL Server | Schema.Access, Database.LocaleProvider, Locale, ICURules, Type.Size, Tablespace and the rest | no source or no change |
| Oracle | Table.Owner | the schema, because a table belongs to the user whose schema it is in |
| Oracle | Table.Persistence | `all_tables.temporary` |
| Oracle | Table.AccessMethod | `iot_type` and `cluster_name`: heap, index organized or cluster |
| Oracle | Table.Size | `user_segments`, summed. Only for a table of the connected user |
| Oracle | Table.Rows | `all_tables.num_rows`. NULL until the table is analyzed |
| Oracle | Table.Options | `on_commit` from `duration`, nologging, pctfree, compress, row_movement and read_only, when they are not the default |
| Oracle | Table.RowSecurity, RowSecurityForced | an enabled policy in `all_policies` |
| Oracle | Index.Owner, Persistence | `owner` and `temporary` |
| Oracle | Index.Size | `user_segments`, only for an index of the connected user |
| Oracle | Index.Valid | `status` of VALID or UNUSABLE. NULL for a partitioned index |
| Oracle | Index.Clustered | `index_type` of IOT, which holds the table |
| Oracle | Index.Deferrable, InitiallyDeferred | the primary key or the unique constraint that owns the index |
| Oracle | Index.Options | invisible, compress and nologging |
| Oracle | Index.ConstraintType | p or u of the owning constraint |
| Oracle | Constraint.Enforced | `status = 'ENABLED'` |
| Oracle | Sequence.CacheSize | `all_sequences.cache_size` |
| Oracle | Partitions (new kind) | `all_tab_partitions`, with `high_value` |
| Oracle | Policies (new kind) | `all_policies` |
| Oracle | NotNulls (new kind) | `all_constraints` of type C whose `search_condition_vc` is `"COLUMN" IS NOT NULL`, from 12c |
| Oracle | Column.Storage, Compression, StatsTarget | no source. Compression is per table or per partition and the statistics target is per statistics |
| Oracle | Index.Predicate, Definition, Using, ConstraintDefinition | no source. `DBMS_METADATA.GET_DDL` writes a statement for one object at a time, needs a privilege for another schema, and is a call for each row |
| Oracle | Function.Leakproof, Prosrc | no source |
| Oracle | Inherits | no source. Oracle has no table inheritance |

## What each decision means

`Constraint.Enforced` means the server checks a new row. It does not mean the
rows that were there before are known to satisfy the constraint. PostgreSQL
keeps that second fact in `convalidated`, and the NotNulls kind keeps it as
`Validated`. D203 reads Snowflake the same way: a key that is declared and
never checked is false.

SQL Server has two flags. `is_disabled` is true after `NOCHECK CONSTRAINT`. It
means the server checks no new row, so Enforced is false. `is_not_trusted` is
true after `WITH NOCHECK` or after a disable and enable, and it means the
existing rows were not checked. The server still checks every new row, so
Enforced is true, and the flag is not returned. A primary key, a unique
constraint and a default cannot be switched off, so they are true.

Oracle has `STATUS` and `VALIDATED`. `ENABLED` checks every new row and
`DISABLED` checks none, so `Enforced` is `STATUS = 'ENABLED'`. A constraint that
is `ENABLE NOVALIDATE` is enforced. A `DISABLE RELY` constraint is not, even
though the optimizer trusts it.

No `Validated` field exists on `Constraint`, so both products leave the second
fact out. That is a gap in the root type and not in the models. It is in
docs/BACKLOG.md.

`Table.Owner` is the schema owner on Oracle. A table belongs to a user and
the schema is the user. So the owner of the table and the owner of the schema
are the same name, and there is no other owner to report. On SQL Server a table
has its own owner only when `ALTER AUTHORIZATION` set one. `principal_id` is
NULL otherwise, so the owner is the owner of the schema.

`Table.Size` is bytes of the table alone, as PostgreSQL counts `pg_table_size`
without the indexes. Indexes have a size of their own. SQL Server adds the
pages of in row, overflow and LOB data and the internal tables of a columnstore
index. Oracle adds the segments of the table, its partitions and its
subpartitions, and not the LOB segments.

`Table.Rows` is an estimate on both. SQL Server keeps a count in
`sys.partitions` that is approximate for a heap. Oracle keeps `num_rows` from the
last statistics gathering and it is NULL for a table that was never analyzed.
Neither is turned into 0.

`Index.Clustered` has a looser meaning than in PostgreSQL. PostgreSQL records
that the table was last clustered on the index, and the next write breaks the
order. A clustered index of SQL Server and the primary key index of an index
organized table of Oracle hold the table itself. The word is the same and the
fact is stronger. A consumer that needs the PostgreSQL meaning must read the
product first.

`Index.Valid` is false for a disabled SQL Server index and an unusable Oracle
index. The planner does not use either one. A SQL Server disabled index also
frees its pages, so its size is NULL.

`Policy` gets one row for each predicate. The shapes differ, so each product
needs a rule.

- SQL Server filter predicate: Command select, Using is the predicate. A filter
  also hides the rows from the read that UPDATE and DELETE make first, and it
  has no effect on INSERT. There is no better word in the list of five
  commands.
- SQL Server block predicate: AFTER INSERT is insert and AFTER UPDATE is update,
  both with WithCheck. BEFORE UPDATE is update and BEFORE DELETE is delete, both
  with Using.
- SQL Server and Oracle: Permissive is false, because the predicates of several
  policies are joined with AND. Roles is NULL, because a policy applies to
  everybody. A disabled policy is not a row. `Policy` has no field to say it is
  off, so a row claims that the policy is active.
- Oracle keeps no predicate text. A policy names a function and the function
  returns the predicate each time a statement runs. Using and WithCheck hold the
  function as `owner.package.function`. A policy for all four statements is one
  row with Command all. Any other policy is one row for each statement it
  names. CHK_OPTION says WithCheck applies to update.

`RowSecurityForced` is true wherever RowSecurity is true. Both products apply a
policy to the owner of the table. I measured SQL Server with a sysadmin
connection, which is `dbo`: a table with a filter predicate returned 0 rows to
it. Oracle applies a policy to the owner too, and exempts only a user that
holds `EXEMPT ACCESS POLICY`.

`Partition.Bound` differs for each product, because neither one has the
PostgreSQL syntax. SQL Server has no name for a partition, so the number is the
name, and Bound is an interval such as `[100, 200)` for RANGE RIGHT and
`(10, 20]` for RANGE LEFT, with MINVALUE and MAXVALUE for the ends. Oracle
returns `HIGH_VALUE` as the dictionary keeps it, which is a LONG that SQL cannot
concatenate. `Partitioned` is true when a partition has subpartitions. The
subpartitions are not rows, because `ALL_TAB_SUBPARTITIONS` has its own
`HIGH_VALUE` and a LONG cannot pass through a UNION.

## Sources that were not used

SQL Server does not read `sys.dm_db_partition_stats`, which the brief named.
That view needs `VIEW DATABASE STATE`, so a statement that names it is refused
for any principal without it. Refusing `Tables` for a lesser user is worse than
a size that comes from `sys.allocation_units`. The two agree: for the three
tables of the test the sum of `total_pages` equals `reserved_page_count`, once
`sys.internal_partitions` is added for the columnstore table.

Oracle does not read `DBA_SEGMENTS` or `DBA_POLICIES`. Both need a role, and a
statement that names a view the user cannot read fails whole. `USER_SEGMENTS` is
readable by everybody and holds the segments of the connected user. So a size is
known for the connected user and NULL for the tables of any other schema. A
table with no rows has no segment from 11.2 when segment creation is deferred,
and its size is 0. 11.2.0.2 Express creates the segment at once.
`ALL_POLICIES` exists for an ordinary user and it replaces `DBA_POLICIES`.

## Cost

D47 asks for a catalog of thousands of objects. Times are in milliseconds, for
the whole schema and for one table, on a warm server, as the median of three.

SQL Server 2022 had 4000 tables and 8000 indexes in one schema, one row each in
the tables.

| Statement | Before | After |
| --- | --- | --- |
| Tables, 4000 rows | 25 | 100 to 111 |
| Tables, one table | 10 | 6 to 7 |
| Indexes, 8000 rows | 53 | 130 to 142 |
| Indexes, one table | 11 | 7 to 8 |
| Constraints, 4000 rows | not measured, the new column is in the row | 30 |
| Partitions, none | not asked | 37, and 2 for one table |
| Policies, none | not asked | 0.2 |

The first version of the size read the allocation units for each table with a
correlated subquery, and it took 16 seconds for the 4000 tables. A single
seek in `sys.allocation_units` cost 0.6 ms, because the join on a CASE of the
two container columns is a nested loop. A scan of the whole view takes 11 ms.
So the statement now joins two derived tables, grouped by object and index, with
one join for in row data and overflow data and one for LOB data. That made 4000
tables take about 110 ms. The scan of the allocation units is the price of a
size, and it grows with the database. For one table SQL Server pushes the filter
into the derived table, and the statement takes 6 ms.

Oracle 21c Express had 3000 tables and 6000 indexes in one schema.

| Statement | Before | After |
| --- | --- | --- |
| Tables, 3000 rows | 53 | 325 |
| Tables, one table | 72 | 84 |
| Indexes, 6000 rows | 159 | 71 to 260 |
| Indexes, one table | 45 | 27 to 32 |
| Constraints, 3000 rows | not measured, the new column is in the row | 78 |
| Partitions, none | not asked | 31, and 1.7 for one table |
| Policies, none | not asked | 51, and 30 for one table |
| NotNulls, none | not asked | 2 |

The Oracle tables statement reads `USER_SEGMENTS` and `ALL_POLICIES` once each,
whatever the filter, so the cost of one table does not go below 80 ms. Both views
are small next to `ALL_OBJECTS`. The statement is six times slower for the whole
schema and 12 ms slower for one table. A caller that needs no size pays for it
and this is the cost of the field. If it matters later, the fix is a second
kind for sizes.

## What each release fills

| Release | Differences |
| --- | --- |
| SQL Server 2017 and newer | everything in the audit |
| SQL Server 2016 | the same. 2017 is the oldest that was run |
| SQL Server 2014 and 2012 | no `Policies`, `RowSecurity` and `RowSecurityForced` are NULL |
| SQL Server 2008 R2 | also no `Size`, no `PartitionedTable.DirectSize` and `TotalSize`, no `memory optimized` and no `unlogged` |
| Oracle 11g | no `NotNulls`. Express has no partitioning and no virtual private database, so `Partitions` and `Policies` read nothing |
| Oracle 12c to 21c | everything except `Domains`. 21c was run, and 21c Express has partitioning and virtual private database. 12c to 19c were not run |
| Oracle 23ai and 26ai | everything, with `Domains` |

SQL Server answers 34 of the 65 on 2016 and newer, 32 on 2012 and 2014, and 30
on 2008 R2. Oracle answers 29 on 23ai and 26ai, 28 from 12c to 21c and 27 on
11g.

## Parity and conformance

`TestPrivilegeParity` was recorded for both products and read.

SQL Server: the login with `CONTROL` on the schema and the contained user read
the same rows as the administrator, and the golden did not change. A principal
with only `SELECT` on a schema reads the table, and reads `RowSecurity` as false
and no policies for a table that has a filter. Policies and predicates are
visible only with `VIEW DEFINITION` on the policy or the schema. I measured it
with a login that had `SELECT` on the schema, and then with `VIEW DEFINITION`
as well. With the second grant the row security and the three predicates
appeared. This is a gap in what a lesser user can see. It is not an error.

Oracle: the `local` user reads the same rows as the administrator and the
values of `Tables` and `Indexes` differ, so the golden has two new lines. The
cause is `Size`. The user reads a size for its own tables, and the administrator
reads NULL for them. A user with `SELECT` on a table of another schema reads the
table and does not read its policy, because `ALL_POLICIES` answered no row for
it. I measured that with a user that has only `SELECT` on a table with a
policy. `RowSecurity` is false for it.

Both products can therefore say false for a table that has a policy, to a user
that cannot see the policy. A caller that needs the truth reads as a principal
that can see the policy.

The conformance golden did not change. The canonical projection reads the
columns and the constraints of the core tables, and the new fields are not
in it.

## What the fixtures hold

SQL Server: a filtered index with an included column and a fill factor, a
disabled index, a check constraint set to `NOCHECK`, a sequence with a cache, a
table in three partitions with page compression, and from 2016 a table with a
security policy of a filter and two block predicates. The master database has no
memory optimized filegroup, so no memory optimized table is in the fixture. I
measured one in a scratch database and the model answered unlogged and
`memory optimized`, with size and rows NULL. A temporary table was measured in
tempdb the same way, and it read temporary.

Oracle: an invisible index, an unusable index, a disabled check constraint, a
sequence with a cache, a global temporary table, an index organized table, a
table with `PCTFREE` and `NOLOGGING`, a range partitioned table and a table
with a virtual private database policy. The partitioned table and the policy
catch the refusal of an edition that lacks them. The tests look for them and
say what they skipped.

The Oracle fixture made `book_published_ix` in the schema of the user that ran
it, `SYSTEM`, and not in `DBMETA_FIXTURE`. The index name had no schema. It is
qualified now. No other fixture step was wrong.

## What was left out

- A `Validated` field on `Constraint`, for both products. See docs/BACKLOG.md.
- The sizes of `PartitionedTable` on Oracle. A size needs the connected user
  and a sum over partitions and subpartitions.
- The subpartitions of Oracle as rows of `Partitions`.
- `Index.Definition` and `Using` on both. SQL Server has no function that writes
  an index. Oracle has `DBMS_METADATA.GET_DDL`, which is a call for each row.
- A disabled policy. `Policy` has no field for the state.
- A memory optimized table in the fixture.
- Windows machines for the old SQL Server releases, and Oracle 18c and 19c.
- SQL Server 2025 ran after `dbrun` made the container again as mine, because
  the one that user:ken made was stopped. It was removed with the others.
