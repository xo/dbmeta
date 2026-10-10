# D208. ClickHouse, DuckDB, SQLite3, Databend and Vertica fill the describe fields they have a source for

Status: Decided.

## The decision

D198, D199 and D201 gave the root types new fields and new kinds, and only the
PostgreSQL model filled them. D205 to D207 did the same audit for eleven other
products. This decision does it for the columnar family: ClickHouse, DuckDB,
SQLite3, Databend and Vertica. SQLite3 shares its statements with libSQL and
rqlite, and DuckDB shares its statements with GizmoSQL, so one change reaches
each pair or trio. A field is filled when one statement can produce it at a
cost that follows the rows returned (D47, rule 13). A field with no such source
stays NULL, and the reason is written here and in `docs/COVERAGE.md`.

The releases were ClickHouse 25.3, 25.8, 26.8 and 26.9, Databend 1.2.881 and
1.2.951, Vertica 25.1 (see the end for the older ones), SQLite 3.53.4 on both
drivers, and DuckDB 1.5.5. Gemini and DeepSeek were asked about the fields that
looked absent, as rule 14 requires.

## The audit

| Product | Field or kind | Source, or why none |
| --- | --- | --- |
| ClickHouse | `Table.Owner` | no source. `system.tables.definer` is the SQL SECURITY user of a view, which is not an owner |
| ClickHouse | `Table.Persistence` | `is_temporary`. A view has none. A Memory table is permanent here, because its definition stays and only its rows go on a restart |
| ClickHouse | `Table.AccessMethod` | `engine` |
| ClickHouse | `Table.Size`, `Table.Rows` | `total_bytes` and `total_rows`. NULL for an engine that keeps no number, such as a view |
| ClickHouse | `Table.Options` | `partition_key`, `sorting_key`, `primary_key`, `sampling_key` and `storage_policy`, as `partition_by=..., order_by=...`. The SETTINGS clause is only in `engine_full` and is not read |
| ClickHouse | `Table.RowSecurity` | no source a statement can use. `system.row_policies` is closed to a user who has no grant on it, so a join makes `tables` fail for that user |
| ClickHouse | `Column.Compression` | `compression_codec`, such as `CODEC(ZSTD(3))`. Empty means the default and is NULL |
| ClickHouse | `Column.Storage`, `StatsTarget` | no source. `system.columns.statistics` names the kinds of statistics and has no target |
| ClickHouse | `Index.Size`, `Options`, `Using` | `data_compressed_bytes` plus `marks_bytes`, `granularity=N`, and `type_full` |
| ClickHouse | `Index.Owner`, `Persistence`, `Predicate`, `Valid`, `Clustered`, `Definition` | no source |
| ClickHouse | `Constraint.Enforced` | the type of `system.constraints`: true for CHECK, false for ASSUME, which the server never tests |
| ClickHouse | `Function.Prosrc` | no source. `create_query` is the whole statement, which is `Definition` |
| ClickHouse | `PartitionedTable.AccessMethod`, `DirectSize`, `TotalSize` | `engine` and `total_bytes`, twice, because a partition is not a table and there is no second level |
| ClickHouse | `Partitions` | new. `system.parts` grouped by partition id. `Bound` is the value of the partition expression. The rows of a partition have no field |
| ClickHouse | `Policies` | new. `system.row_policies`. The command is always select, and the roles are the apply_to columns |
| ClickHouse | `NotNulls`, `Inherits` | no object. Nullable is part of the type |
| DuckDB | `Table.Persistence`, `Table.Rows` | `temporary` and `estimated_size`, which is an estimate |
| DuckDB | `Table.Size`, `AccessMethod`, `Options`, `Owner` | no source. The size of a table is in `pragma_storage_info`, which takes the table as an argument and cannot be joined for every table |
| DuckDB | `Index.Persistence`, `Definition`, `Using` | the `temp` database, `sql` and `art` |
| DuckDB | `Index.Predicate` | no source. DuckDB refuses a partial index ("Creating partial indexes is not supported currently") |
| DuckDB | `Index.ConstraintType` | no source. A primary key or a unique constraint has no row in `duckdb_indexes` |
| DuckDB | `Function.Prosrc` | `macro_definition`, the same text as `Source` |
| DuckDB | `NotNulls` | new. Each NOT NULL is a named row in `duckdb_constraints`, such as `author_name_not_null` |
| DuckDB | `Column.Compression` | no source. The compression of a column is chosen per segment and is in `pragma_storage_info` |
| DuckDB | `Constraint.Enforced` | NULL. DuckDB checks every constraint, and no catalog column says so |
| SQLite3 | `Table.Persistence` | `permanent` for a table, because the statement reads `sqlite_schema`, which is the main database. NULL for a view |
| SQLite3 | `Table.Options` | no usable source. `pragma_table_list` has `strict` and `wr` (WITHOUT ROWID). Joined for every table it costs time that grows with the square of the catalog |
| SQLite3 | `Table.Size` | no source. `dbstat` scans the file, and the mattn driver is built without it |
| SQLite3 | `Table.Rows` | no source. SQLite keeps no row count |
| SQLite3 | `Table.AccessMethod` | no source. The module of a virtual table is only in the text of its CREATE statement |
| SQLite3 | `Table.Type` of a shadow table | left as `table`. `pragma_table_list` has the type `shadow`, and relabeling a row is a decision for Ken (see the end) |
| SQLite3 | `Index.Predicate` | the text after WHERE in the CREATE INDEX statement, taken only where `pragma_index_list` says `partial` |
| SQLite3 | `Index.Clustered` | true for the primary key index of a WITHOUT ROWID table. See the statements below |
| SQLite3 | `Index.Definition`, `Using`, `ConstraintType`, `Persistence` | the CREATE INDEX text, `btree` (`diskann` on libSQL), `p` or `u` from the origin, and `permanent` |
| SQLite3 | `Index.Size`, `Valid`, `Owner` | no source |
| SQLite3 | `Constraint.Enforced` | NULL. A foreign key is checked only on a connection that turned `foreign_keys` on, and a check constraint is not listed |
| Databend | `Table.Owner`, `AccessMethod`, `Rows` | `owner`, `engine` and `num_rows` |
| Databend | `Table.Persistence` | `transient` or `permanent`. NULL for a view |
| Databend | `Table.Size` | `data_compressed_size`. Indexes are not in it |
| Databend | `Table.Options` | `cluster_by`. The rest of `table_option` holds the path of the snapshot, which changes at every write |
| Databend | `Function.Prosrc` | `system.user_functions.definition`, the same text as `Source` |
| Databend | `Policies`, `Table.RowSecurity` | no source. Databend 1.2.951 has no system table for a row access policy, and SHOW cannot be filtered |
| Databend | `Column` fields, `Index` fields, `Constraint.Enforced` | no source |
| Vertica | `Table.Owner`, `Persistence`, `Options` | `owner_name`, `is_temp_table`, and `partition_expression` as `partition_by=...` |
| Vertica | `Table.Size`, `Table.Rows` | no usable source. See the cost |
| Vertica | `Table.RowSecurity` | no usable source. See the parity |
| Vertica | `Column.Compression` | no usable source. `projection_columns.encoding_type` costs a scan of the catalog. See the cost |
| Vertica | `Index.Size`, `Valid`, `Options` | the sum of `used_bytes` of the projection on every node, `is_up_to_date`, and `segmented_by=...` or `unsegmented` |
| Vertica | `Constraint.Enforced` | `is_enabled`. A foreign key has none, so it is NULL |
| Vertica | `Function.Prosrc` | `function_definition`, the same text as `Source` |
| Vertica | `Partitions` | new. `v_monitor.partitions` joined to the projections for the name of the table |
| Vertica | `Policies` | new. The row policies of `v_catalog.access_policy`. A policy has no name, so the name is its object id |

A projection is not a table. Vertica reports its projections as indexes, as it
did before, and the rows of a projection are in `projection_storage.row_count`.
`Index` has no field for rows, so they are not read.

Gemini said that ClickHouse has no owner, no statistics target and no flag for
row security, and named `system.parts.partition` and `rows` for the partitions.
It said that Vertica has no table level view for the size and the rows, and
that a statement must add up `projection_storage`. Both agree with what was
measured. DeepSeek ran out of tokens on every question about ClickHouse and
Vertica. For Databend it said that `system.policy_references` lists the row
access policies and the masking policies. The table does not exist on 1.2.951,
and `SHOW ROW ACCESS POLICIES` is a syntax error there. Gemini returned the
error 429 for Databend, so the question was not answered.

## The statements

Every statement returns the same columns on every release of its product.

- ClickHouse `Tables` adds five columns of `system.tables`. `total_bytes` and
  `total_rows` are cast to `Int64`. `Columns` adds `compression`. `Indexes`
  adds three columns of `system.data_skipping_indices`. `Constraints` adds
  `enforced`, under the same 26.8 gate as the rest of it. `PartitionedTables`
  adds four columns. `Partitions` and `Policies` are new.
- DuckDB `Tables` adds `persistence` and `rows` to both halves of its UNION, and
  the view half casts a NULL to the type of the table half. `Indexes` adds
  three columns, `Functions` adds `prosrc`, and `NotNulls` is new.
- SQLite3 `Tables` adds `persistence`. `Indexes` adds six columns and one join
  to `sqlite_schema` for the text of the statement. The clustered flag asks
  whether the entries of the index end with the rowid: `pragma_index_xinfo`
  lists a column with `cid = -1` for the rowid of every index of a table that
  has one, and a WITHOUT ROWID table has none, because its primary key index is
  the table. The predicate is cut at the first ` WHERE ` of the statement, read
  with its tabs and line breaks turned to spaces so that the offsets stay the
  same. A string in the key list that holds the text ` where ` breaks it.
  SQLite takes no subquery in an index, so a WHERE inside the key list is the
  only case, and it was not seen.
- Databend `Tables` adds six columns of `system.tables`. `Functions` and
  `Aggregates` add `prosrc` to the three halves of their UNION.
- Vertica `Tables` adds three columns to the three halves of its UNION.
  `Indexes` adds three columns, `Constraints` adds `enforced`, `Functions`
  adds `prosrc`, and `Partitions` and `Policies` are new.

## The cost

Times are wall time in the client, which includes the round trip. Each line is
the statement with and without the new columns, on a scratch schema.

| Product | Scale | Without the fields | With the fields |
| --- | --- | --- | --- |
| ClickHouse `Tables` | 1500 tables, all of them | 5.4 ms | 9.8 ms |
| ClickHouse `Tables` | the same, one table by name | 1.5 ms | 1.7 to 4.4 ms |
| ClickHouse `Partitions` | 1500 tables of 3 partitions, all of them | not applicable | 24 ms |
| ClickHouse `Partitions` | the same, one table by name | not applicable | 2 to 3 ms |
| ClickHouse `Indexes` | 1500 tables, with and without the sizes | 1.5 ms | 1.5 ms |
| DuckDB `Tables` | 3000 tables, all of them | 11.7 ms | 10.3 ms |
| DuckDB `Tables` | the same, one table by name | 7.1 ms | 5.6 ms |
| DuckDB `Indexes` | 1000 indexes, all of them and one | 3.9 and 2.6 ms | 3.2 and 2.6 ms |
| SQLite3 `Indexes`, mattn | 3000 tables with an index each, all of them | 5.5 ms | 11.9 ms |
| SQLite3 `Indexes`, mattn | the same, one table by name | 0.18 ms | 1.1 ms |
| SQLite3 `Indexes`, modernc | the same, all and one | 7.6 and 0.44 ms | 18.9 and 2.4 ms |
| SQLite3 `Tables` with `pragma_table_list`, which was not kept | 300 tables | 0.07 ms | 0.86 ms |
| SQLite3, the same | 3000 tables | 0.07 ms | 88 ms |
| SQLite3, the same | 9000 tables | 0.18 ms | 2.2 s |
| SQLite3, the same | one table by name, at 300, 3000 and 9000 tables | 0.03 ms | 0.03, 0.23 and 0.71 ms |
| Databend `Tables` | 600 tables, all of them | 18 ms | 18 ms |
| Databend `Tables` | the same, one table by name | 7.4 ms | 7.1 ms |
| Vertica `Tables` with the size and the row security, which were not kept | 300 tables, all of them | 10.6 ms | 20 ms |
| Vertica, the same | one table by name, at 300 and at 3000 tables | 5.5 and 5.0 ms | 18 and 53 ms |
| Vertica `Columns` with the encoding, which was not kept | 900 columns, all of them | 8.4 ms | 50 ms |
| Vertica, the same | one table by name, at 300 and at 3000 tables | 5.8 and 7.2 ms | 47 and 156 ms |
| Vertica `Indexes` with the size | 300 projections, one by name, at 300 and at 3000 tables | 12 and 31 ms | 20 and 62 ms |

The ClickHouse, DuckDB and Databend fields are columns of the row that the
statement already reads, and they cost nothing that a measurement separates.

The SQLite join to `pragma_table_list` grows with the square of the catalog,
because the pragma lists every table at each call. A read of one table also
grows with the catalog. D47 does not allow that, so `Table.Options` stays
NULL. The index statement adds a join of `sqlite_schema` to itself by name,
which SQLite does with a scan or a transient index. A read of one table costs
one more pass over the catalog, and `sqlite_schema` has no index by name, so
the statement already does one pass to find the table. The two passes are
6 times the cost of one at 3000 tables, and still 1.1 ms. This is the one
place that D47 is stretched, and it is a constant factor on a scan that
SQLite cannot avoid. Ken accepted it on 2026-10-10.

The Vertica size of a table and the encoding of a column each read a view of
the whole catalog for every table or column returned. A read of one table got
three times slower at 300 tables and ten times slower at 3000 tables, and the
encoding got 8 and 22 times slower. The row security alone cost 6 ms against
5 ms, so the size is what grew. D47 does not allow that. The size of a projection also doubles an index statement that already
reads `projections` whole, and it is kept because a projection has no other
size, and the cost is a constant factor on a read the model had.

## What each release fills

- ClickHouse 25.3, 25.8, 26.8 and 26.9 fill the same fields. The constraint
  and its `enforced` flag need 26.8, as the rest of `constraints` does.
- Databend 1.2.881 and 1.2.951 fill the same fields.
- SQLite 3.53.4 fills the same fields on mattn and on modernc. The mattn
  driver has no `dbstat`, and modernc has it. The model reads neither.
- DuckDB 1.5.5 and the DuckDB inside GizmoSQL fill the same fields.
- Vertica 25.1: every field above.

## Parity and conformance

ClickHouse: the golden has two new lines for the ordinary user, who is refused
`system.parts` and `system.row_policies` as it is refused
`system.data_skipping_indices`. The administrator reads both. The fields of
`tables` and `columns` read the same values for every principal.

Vertica: the grantee reads fewer rows of `policies` than the administrator,
because `access_policy` lists only what a user can see. The first version of
`Tables` had `row_security`, from an EXISTS over the same view. The grantee read
`false` where the administrator read `true`, which is a wrong answer and not a
missing one, and D61 is what asks whether a query began to depend on who is
asking. The field was taken out.

Databend: the golden is unchanged. SQLite3 and DuckDB have no user and the rule
cannot reach them.

The conformance golden is unchanged, because the canonical projection reads the
columns and constraints of the core tables only.

## The hazards of D197

None of the five has an INCLUDE column or a role setting that the models read.
ClickHouse `Partition.Bound` and the Vertica policy `Name` are text, and the
fixture reads each of them back. A SQLite view has no persistence, and the
fixture reads it back as NULL.

## What was left open

- A shadow table of SQLite (the tables an fts5 table keeps) has the type
  `shadow` in `pragma_table_list` and is `table` here. Ken decided on
  2026-10-10 to keep `table`, because `Table.Type` is a small closed
  vocabulary that consumers match on.
- The SETTINGS clause of a ClickHouse table is in `engine_full` as text. A
  statement cannot split it.
- The rows and the size of a Vertica table, and the encoding of a column, can
  be read for one table with a call of its own, which this model does not make.
- The module of a SQLite virtual table is in the CREATE statement only.

## Ken's answers of 2026-10-10

- The SQLite index predicate stays, and the stretch of D47 is accepted.
- A shadow table of SQLite keeps the type `table`.
- The DuckDB `NotNulls` kind stays. It has a name and a column, the same shape
  as the kind of PostgreSQL 18.
- A `Policy` gets an `Enabled` field, so a disabled policy of SQL Server,
  Oracle, Vertica and ClickHouse can be told from an active one.
