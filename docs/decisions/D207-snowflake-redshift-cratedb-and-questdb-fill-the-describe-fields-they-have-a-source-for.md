# D207. Snowflake, Redshift, CrateDB and QuestDB fill the describe fields they have a source for

Status: Decided, amended by D212.

## The decision

D198, D199 and D201 gave the root types new fields and new kinds, and only the
PostgreSQL model filled them. Ken said on 2026-10-10 to finish it. This
decision audits four more models and fills each field that one statement can
produce at a cost that follows the rows returned (D47, rule 13). A field with
no such source stays NULL, and the reason is written here and in
`docs/COVERAGE.md`.

Snowflake ran on the trial account, release 10.36.101, as the role
DBMETA_ROLE. Redshift ran on Redshift Serverless 1.0.434008, as the administrator and as three made principals. CrateDB ran on
6.3.7 and 6.4.5. QuestDB ran on 9.4.3 and 10.0.1. Gemini and DeepSeek were
asked about the fields that looked absent, as rule 14 requires.

## The audit

| Product | Field or kind | Source, or why none |
| --- | --- | --- |
| Snowflake | `Table.Owner` | `TABLE_OWNER` |
| Snowflake | `Table.Persistence` | `IS_TEMPORARY` and `IS_TRANSIENT`: temporary, transient or permanent. NULL for a view |
| Snowflake | `Table.Size` | `BYTES`. A bound, because time travel and fail safe data are not counted |
| Snowflake | `Table.Rows` | `ROW_COUNT` |
| Snowflake | `Table.Options` | `RETENTION_TIME` and `CLUSTERING_KEY`, as `retention_time=1, cluster_by=LINEAR(id)` |
| Snowflake | `Function.Prosrc` | `function_definition` and `procedure_definition` |
| Snowflake | `Column.Collation`, `Constraint.Enforced` | done by D190 and D203 |
| Snowflake | `Column.Storage`, `Compression`, `StatsTarget` | no source. Snowflake records no choice |
| Snowflake | `Schema.Access`, `Index` fields | no source. A grant on a schema is not in INFORMATION_SCHEMA, and the only index is a hybrid table's |
| Snowflake | `Partitions`, `Policies`, `NotNulls`, `Inherits` | no analogue. A row access policy is SHOW and a tag, and the model does not read them |
| Redshift | `Table.Owner` | `pg_get_userbyid(relowner)` |
| Redshift | `Table.Persistence` | a table in a `pg_temp` schema is temporary, and the rest is permanent |
| Redshift | `Table.Size`, `Table.Rows` | no usable source. SVV_TABLE_INFO has `size` in blocks of 1 MB and `tbl_rows`, and it refuses a user who is not a superuser, so a join to it makes `tables` fail for them. `pg_class.reltuples` is 0.001 for a table of 3 rows |
| Redshift | `Table.Options` | no usable source. The distribution style and the sort key are in SVV_TABLE_INFO only |
| Redshift | `Column.Compression` | `pg_attribute.attencodingtype`, named by the numbers that `pg_table_def` shows |
| Redshift | `Function.Prosrc`, `Schema.Access` | `pg_proc.prosrc` and `pg_namespace.nspacl` |
| Redshift | `Column.Storage`, `StatsTarget` | no source. `attstorage` and `attstattarget` are there, and Redshift has no TOAST, so they hold PostgreSQL 8.0's defaults and say nothing |
| Redshift | `Constraint.Enforced` | no source. A key is not enforced, and no catalog column says so |
| Redshift | `Index` fields and `Partitions` | Redshift has no index and no partition |
| CrateDB | `Table.Owner` | no source. `relowner` is 0, `tableowner` is empty, and `information_schema.views.owner` is empty |
| CrateDB | `Table.Options` | `information_schema.tables`: shards, replicas, `clustered_by`, `partitioned_by` and `column_policy` |
| CrateDB | `Table.Size`, `Table.Rows` | the exact values are in `sys.shards`, and an ordinary user is refused the schema `sys`. The estimate is `pg_class.reltuples`. See the cost |
| CrateDB | `Index.Valid`, `Clustered`, `ReplicaIdentity`, `ConstraintType` | `pg_index`, which holds the primary keys only |
| CrateDB | `Partitions` | new rows from `information_schema.table_partitions`, with the values of each partition |
| CrateDB | `RoleSettings` | `sys.users.session_settings` is an object, and one statement can return it only as JSON text. Left unanswered |
| CrateDB | `Column` fields, `Constraint.Enforced`, `NotNulls`, `Policies`, `Inherits` | no source or no object |
| QuestDB | `Table.Rows` | `table_row_count` of `tables()` |
| QuestDB | `Table.Options` | `walEnabled`, `dedup`, `maxUncommittedRows`, `o3MaxLag` and the time to live of `tables()` |
| QuestDB | `Table.Size` | `table_storage()` `diskSize`. See the cost |
| QuestDB | `Partitions`, `Index` fields | `table_partitions()` and `table_columns()` take the table name as a constant, and a bind parameter is refused. One statement cannot read the tables |
| QuestDB | `Owner`, `Persistence`, `Column` fields | no source. The open source edition has no owner and no temporary table |

Gemini named `table_storage()` for the row count and the size of QuestDB, and
named no all tables source for its indexes and partitions, which is what was
measured. DeepSeek ran out of its tokens on the QuestDB question and on the
first CrateDB one, and its short answer for CrateDB named
`information_schema.tables.table_owner` and `sys.default_privileges`. Neither
exists on 6.4.5. Gemini timed out twice on CrateDB. Each lead that was run
against a server is written above.

For Snowflake, DeepSeek listed the columns that are used above, with `OWNER` for
`TABLE_OWNER`, and called the compression, the index and the partition absent.
Both are right. For Redshift, Gemini said that a user can read SVV_TABLE_INFO
when it is granted SELECT on it. That is the lead behind the open item at the
end. It was not run, because a grant on a system view changes the account.

## The statements

Every statement returns the same columns on every release of its product, and
each new column is a field in the `Fields` of the binding.

- Snowflake `Tables` adds five columns of `INFORMATION_SCHEMA.TABLES`. `BYTES`
  and `ROW_COUNT` are cast to `BIGINT`.
- Redshift `Tables` adds the owner and the persistence. `Columns` adds a `CASE`
  on `attencodingtype`. A number that was not measured comes back as the
  number, never as a guessed name. `Functions` adds `prosrc` and `Schemas` adds
  `nspacl`.
- CrateDB `Tables` builds the options text, `Indexes` adds four columns of
  `pg_index`, and `Partitions` is a new binding.
- QuestDB `Tables` adds `rows` and `options` from `tables()`.

The first Redshift statement joined SVV_TABLE_INFO for the size, the rows and
the options, and the join was taken out. Four things were measured.

1. Redshift refused the leader node functions `pg_get_userbyid` and
   `obj_description` beside it.
2. With the join, `c.relname LIKE 's5'` found no row, and `= 's5'` and `LIKE
   's5%'` found the table.
3. With the join, the name column came back padded with spaces, and the
   conformance test reported every table of the fixture as missing.
4. The owner, the grantee and the stranger of the parity test were refused
   with "permission denied for relation svv_table_info". The same user reads
   `pg_class`. So the join made `tables`, a core query, fail for
   every user who is not a superuser. D61 asks whether a query began to depend
   on who asks, and this one did.

So `Table.Size`, `Table.Rows` and `Table.Options` stay NULL on Redshift. The
cost of the join was about 0.5 s for one table and for 202, and that no longer
matters.

## The cost

Times are wall time in the client, which includes the round trip.

| Product | Scale | Without the fields | With the fields |
| --- | --- | --- | --- |
| Snowflake | 300 tables in a schema, all of them | 0.83 to 1.28 s | 0.87 to 0.94 s |
| Snowflake | the same, one table by name | 0.84 s | 0.84 s |
| Redshift, with SVV_TABLE_INFO, which was not kept | 202 tables, all of them | 0.28 s | 0.79 to 0.83 s, and 2.05 s the first time |
| Redshift, the same | one table by name | 0.28 s | 0.79 s |
| CrateDB, joined to `pg_class` | 600 tables and 1300 relations, all of them | 4.3 ms | 11.2 ms |
| CrateDB, joined to `pg_class` | the same, one table by name | 2.9 ms | 9.9 ms |
| CrateDB, the options only | the same | no difference | no difference |
| QuestDB, `table_storage()` joined | 1500 tables, all of them | 5 ms | 45 to 60 ms |
| QuestDB, the same | one table by name | 3.4 ms | 40 to 51 ms |
| QuestDB, `table_row_count` and the options | 1500 tables | 5 ms | 5 ms |

The Snowflake fields are columns of the row the statement already reads, and
they cost nothing that a measurement separates.

The Redshift fields that stay are columns of rows that `pg_class`,
`pg_attribute`, `pg_proc` and `pg_namespace` already give, and cost the same as
before. The join to SVV_TABLE_INFO cost about 0.5 s for one table and for 202,
which does not follow the rows returned, and it is not kept.

The CrateDB join to `pg_class` and the QuestDB join to `table_storage()` do not
narrow with the filter. They scan the whole catalog for one table, and the read
of one table gets three times and ten times slower. D47 does not allow that, so
`Table.Size` and `Table.Rows` are not added for either, and the options, which
cost nothing, are. CrateDB has the exact numbers in `sys.shards` as well, and
that schema is refused to an ordinary user.

## What each release fills

- Snowflake 10.36.101: every field above.
- Redshift 1.0.434008: every field above that has a source.
- CrateDB 6.3.7 and 6.4.5 fill the same fields. 6.3.7 has no collations, which
  is an older difference.
- QuestDB 9.4.3 and 10.0.1 fill the same fields. `tables()` has the same
  columns on both.

## Parity and conformance

Snowflake: the golden is unchanged. The role, the grantee and the user read
the same rows, and the same values, as the administrator for the new fields,
because INFORMATION_SCHEMA shows each role what it has a privilege on.

Redshift: the golden is unchanged. The owner, the grantee and the stranger
read the same rows, and the same values, as the administrator for the new
fields. The first version failed this test for all three, as written above.

CrateDB: the golden is unchanged. `tables` reads only `information_schema`,
`indexes` reads `pg_index` and `partitions` reads
`information_schema.table_partitions`, and each is open to every user. The
read of `sys.shards` that was tried first made `tables` fail for both users
with "Schema 'sys' unknown", and was the reason for not adding it.

QuestDB: the golden is unchanged. A WAL table applies an insert after it
returns, so the row count is NULL for a moment and the two reads of the parity
run differed. The QuestDB setup now waits for the row of `book`.

The conformance golden is unchanged, because the canonical projection reads
the columns and constraints of the core tables only.

## The hazards of D197

None of the four has an INCLUDE column, and none has a role setting that the
models read. Redshift has default privileges. The fixture makes one with no
schema, the scan reads it into `DefaultACL.Schema`, and the test checks that it
is NULL.

## What was left open

- The size, the rows and the options of a Redshift table are in SVV_TABLE_INFO,
  which only a superuser reads. A grant on the view to a role, or a model for
  the administrator, can fill them. Ken decides.
- `Constraint.Enforced` is NULL for CrateDB and Redshift. Both enforce or do
  not enforce a key in a way that no catalog column states. D203 filled it for
  Snowflake because INFORMATION_SCHEMA has the column. Ken can decide that a
  fact of the product is enough.
- `RoleSettings` for CrateDB, as JSON text.
- The size of a CrateDB table, for a superuser only.
