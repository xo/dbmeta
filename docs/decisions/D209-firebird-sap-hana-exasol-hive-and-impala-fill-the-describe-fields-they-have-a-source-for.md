# D209. Firebird, SAP HANA, Exasol, Hive and Impala fill the describe fields they have a source for

Status: Decided.

## The decision

D198, D199 and D201 gave the root types new fields and new kinds, and only the
PostgreSQL model filled them. Ken said on 2026-10-10 to finish it. This decision
audits five more models and fills each field that one statement can produce at
a cost that follows the rows returned (D47, rule 13). A field with no such
source stays NULL, and the reason is written here and in `docs/COVERAGE.md`.

The statements ran on Firebird 3.0, 4.0 and 5.0, SAP HANA 2.00.088, Exasol
2026.2.0, Apache Hive 4.0.1 and 4.2.1, and Apache Impala 4.4.1 and 4.5.2. The
two Nightly SAP HANA releases, 2.00.076 and 2.00.082, and the Verified Exasol
release, 2025.2.1, were not run. Gemini and DeepSeek were asked about the fields
that looked absent, as rule 14 requires. The answers are after the audit.

## The audit

| Product | Field or kind | Source, or why none |
| --- | --- | --- |
| Firebird | `Table.Owner` | `RDB$OWNER_NAME` |
| Firebird | `Table.Persistence` | `RDB$RELATION_TYPE`: temporary for the two global temporary types, NULL for a view, permanent for the rest |
| Firebird | `Table.Options` | `on_commit` from the relation type, `external_file` from `RDB$EXTERNAL_FILE`, and `sql_security` from `RDB$SQL_SECURITY` on 4.0 and later |
| Firebird | `Table.AccessMethod`, `Size`, `Rows` | no source. One engine, and no size or row count in the catalog or in `MON$` |
| Firebird | `Index.Valid` | `RDB$INDEX_INACTIVE` |
| Firebird | `Index.Predicate` | `RDB$CONDITION_SOURCE` from 5.0, without the WHERE word |
| Firebird | `Index.Deferrable`, `InitiallyDeferred` | `RDB$DEFERRABLE` and `RDB$INITIALLY_DEFERRED` of the owning constraint, which are always NO |
| Firebird | `Index.ConstraintType` | `RDB$CONSTRAINT_TYPE` of the owning constraint as p, u or f |
| Firebird | `Index.Owner`, `Persistence`, `Size`, `Clustered`, `ReplicaIdentity`, `Options`, `Definition`, `Using` | no source. `Using` repeats `Type`, which is always btree |
| Firebird | `Column.Storage`, `Compression`, `StatsTarget` | no source |
| Firebird | `Constraint.Enforced` | no catalog column. See what was left open |
| Firebird | `Function.Prosrc` | `RDB$FUNCTION_SOURCE` and `RDB$PROCEDURE_SOURCE`, the same text as `Source` |
| Firebird | `NotNulls` | new. `RDB$RELATION_CONSTRAINTS` of the type NOT NULL, with the column in `RDB$CHECK_CONSTRAINTS` |
| Firebird | `Partitions`, `Policies`, `Inherits` | Firebird has none of the three |
| SAP HANA | `Table.Owner` | `OWNER_NAME` of `SYS.OWNERSHIP`. `SYS.TABLES` has no owner column, and the schema owner is a different fact |
| SAP HANA | `Table.Persistence` | `IS_LOGGED` false is unlogged, and a NO LOGGING table is also marked temporary, so it is tested first. Then `IS_TEMPORARY` is temporary. NULL for a view |
| SAP HANA | `Table.AccessMethod` | `TABLE_TYPE`, row or column |
| SAP HANA | `Table.Options` | `COMMIT_ACTION` for a global temporary table, `IS_INSERT_ONLY`, `AUTO_MERGE_ON` and a `LOAD_UNIT` that is not the default |
| SAP HANA | `Table.Size`, `Rows` | `TABLE_SIZE` and `RECORD_COUNT` of `SYS.M_TABLES`. Not added. See the cost |
| SAP HANA | `Index.Owner` | `OWNER_NAME` of `SYS.OWNERSHIP` for the index |
| SAP HANA | `Index.Using` | `INDEX_TYPE`, lower cased, the same text as `Type`. The same rule filled it for MySQL in D205 |
| SAP HANA | `Index.ConstraintType` | p from `CONSTRAINT`, and u when `SYS.CONSTRAINTS` also names the index, because `CONSTRAINT` says NOT NULL UNIQUE for a plain unique index as well |
| SAP HANA | `Index.Size` | `INDEX_SIZE` and `MEMORY_SIZE_IN_TOTAL` of the `M_` views. Not added. See the cost |
| SAP HANA | `Index.Valid`, `Clustered`, `Predicate`, `ReplicaIdentity`, `Deferrable`, `InitiallyDeferred`, `Options`, `Definition`, `Persistence` | no source |
| SAP HANA | `Column.Compression` | `COMPRESSION_TYPE` of `SYS.TABLE_COLUMNS`, lower cased. A row table says none |
| SAP HANA | `Column.Storage`, `StatsTarget` | no source. `LOAD_UNIT` is where a column is loaded from, and not how it is stored |
| SAP HANA | `Constraint.Enforced` | `IS_ENFORCED` of `SYS.REFERENTIAL_CONSTRAINTS` for a foreign key. No column for the other kinds |
| SAP HANA | `Function.Prosrc` | no source. `Definition` holds the whole statement |
| SAP HANA | `Partitions` | new. `SYS.TABLE_PARTITIONS` and the kind of each level from `SYS.PARTITIONED_TABLES` |
| SAP HANA | `NotNulls`, `Policies`, `Inherits` | none. HANA records NOT NULL on the column and not as a named constraint |
| Exasol | `Table.Owner` | `TABLE_OWNER` and `VIEW_OWNER` |
| Exasol | `Table.Persistence` | permanent for a table, NULL for the rest. Exasol has no temporary or unlogged table |
| Exasol | `Table.Rows` | `TABLE_ROW_COUNT` |
| Exasol | `Table.Size` | `MEM_OBJECT_SIZE` of `EXA_ALL_OBJECT_SIZES`, by object id. A two row table is 11027 bytes there and 24 bytes in `RAW_OBJECT_SIZE`, so the memory size is the bytes the table takes |
| Exasol | `Table.Options` | `COLUMN_IS_DISTRIBUTION_KEY` and `COLUMN_PARTITION_KEY_ORDINAL_POSITION` of `EXA_ALL_COLUMNS` |
| Exasol | `Table.AccessMethod` | no source. One engine |
| Exasol | `Index.Owner`, `Size` | `INDEX_OWNER` and `MEM_OBJECT_SIZE` of `EXA_ALL_INDICES` |
| Exasol | `Index.Valid`, `Clustered`, `Predicate`, `Using`, `Options`, `Definition`, `Persistence` | no source |
| Exasol | `Constraint.Enforced` | `CONSTRAINT_ENABLED` |
| Exasol | `NotNulls` | new. `EXA_ALL_CONSTRAINTS` of the type NOT NULL and `EXA_ALL_CONSTRAINT_COLUMNS` |
| Exasol | `Column` fields | no source |
| Exasol | `Function.Prosrc` | no source. The catalog keeps the whole statement |
| Exasol | `Partitions`, `Policies`, `Inherits` | none. A partition is a key and not an object, and `PartitionedTables` reports it |
| Hive | `Table.Owner` | `TBLS.OWNER` |
| Hive | `Table.Persistence` | permanent for a table, NULL for a view. A temporary table is not in the metastore |
| Hive | `Table.AccessMethod` | `INPUT_FORMAT` of the storage descriptor, NULL for a view |
| Hive | `Table.Size`, `Rows` | the `totalSize` and `numRows` table parameters. They are statistics, so a table with none, a view and the value -1 give NULL |
| Hive | `Table.Options` | no source. The table parameters mix statistics, times and settings, and no key marks a setting |
| Hive | `Constraint.Enforced` | the ENABLE bit of `ENABLE_VALIDATE_RELY`, which is 4. The value 5 is ENABLE and RELY |
| Hive | `Partitions` | new. `PARTITIONS`. `PART_NAME`, such as `year=2026`, is the name and the bound |
| Hive | `NotNulls` | new. `KEY_CONSTRAINTS` of the type 3, with the column from `COLUMNS_V2`. `Validated` is the VALIDATE bit, which is 2 |
| Hive | `Column`, `Index` fields | no source. Hive 3 removed indexes |
| Hive | `Function.Prosrc` | no source. The metastore keeps the Java class, and `Source` holds its name |
| Hive | `Policies`, `Inherits` | none |
| Impala | `Table.Owner`, `AccessMethod`, `Size`, `Rows` | the Owner and InputFormat rows and the `totalSize` and `numRows` parameters of the `DESCRIBE FORMATTED` that the walk already runs. -1 gives NULL |
| Impala | `Table.Persistence` | permanent for a table, NULL for a view |
| Impala | `Table.Options` | no source, for the reason given for Hive |
| Impala | every other new field and kind | no source. The walk reads no constraint, index, partition or function body, and adding any of them is one more statement for each table, which D146 allows only where a walk already pays it |

## What the products said

Gemini answered for SAP HANA, Exasol and Hive, and the leads were the ones in the
audit, all run against a server. For SAP HANA it named `SYS.OWNERSHIP`,
`M_TABLES`, `TABLE_COLUMNS.COMPRESSION_TYPE` and `TABLE_PARTITIONS`. DeepSeek
named `SYS.TABLES.SCHEMA_NAME` as the owner, `M_TABLE_STATISTICS` for the rows,
and a `SYS.RLS_` family for policies, and none of the three exists or answers.
For Firebird, Gemini said that no table keeps a row count or a size, and
DeepSeek spent its tokens thinking twice and returned nothing. For Hive Gemini
added `sys.partition_key_vals` for the partition values, which `PART_NAME`
already carries. The Impala kinds were not asked, because the walk is the only
source and D146 sets its limit.

## The statements

Every statement returns the same columns on every release of its product, and
each new column is a field in the `Fields` of the binding.

- Firebird `Tables` adds six columns of `RDB$RELATIONS`. `Indexes` joins
  `RDB$RELATION_CONSTRAINTS` once for the constraint of each index, and reads
  `RDB$CONDITION_SOURCE` through a fragment for 5.0. `Functions` adds the body.
  `NotNulls` is new.
- SAP HANA `Tables` joins `SYS.OWNERSHIP`, and `Columns` reads one more column.
  `Indexes` joins `SYS.OWNERSHIP` and a derived table of the unique constraints.
  `Constraints` reads `IS_ENFORCED`, and `Partitions` is new. The bound of a
  partition joins one text for each of the three levels.
- Exasol `Tables` joins `EXA_ALL_OBJECT_SIZES` by object id and a derived table of
  the key columns. The derived table takes the same filters as the tables, which
  is why it is not a correlated read. `Indexes` and `Constraints` add columns,
  and `NotNulls` is new.
- Hive `Tables` pivots the comment, `numRows` and `totalSize` in one pass over
  `TABLE_PARAMS`, and joins the storage descriptor. `Constraints` adds a flag.
  `Partitions` and `NotNulls` are new.
- Impala `describe` reads three more rows of the output that it already has.

## The cost

Times are wall time in the client, which includes the round trip. Each is the
best of three.

| Product | Scale | Without the fields | With the fields |
| --- | --- | --- | --- |
| Firebird | 3000 tables, all of them | 5.0 ms | 8.7 ms |
| Firebird | 6000 indexes | 20.6 ms | 32.4 ms |
| Firebird | `NotNulls`, 3000 rows | new | 14.2 ms |
| SAP HANA | 3000 tables, all of them | 4.7 ms | 20.1 ms |
| SAP HANA | the same, one table by name | 0.2 ms | 9.7 ms, and 4.4 ms with 12 tables in the database |
| SAP HANA | 6000 indexes | 11.0 ms | 28.5 ms |
| SAP HANA | the same, one table by name | 0.2 ms | 12.8 ms |
| SAP HANA, with `SYS.M_TABLES`, which was not kept | 3000 tables, all of them | 4.7 ms | 94 ms |
| SAP HANA, the same | one table by name | 0.2 ms | 65 ms |
| SAP HANA, with the index sizes, which were not kept | 6000 indexes, one table | 0.2 ms | 108 ms |
| Exasol | 3000 tables, all of them | 353 ms | 898 ms |
| Exasol | the same, one table by name | 4 ms | 96 ms |
| Exasol, with a correlated read of the key columns, which was not kept | one table by name | 4 ms | 577 ms |
| Exasol | 3000 indexes, and 6000 constraints | 287 ms and 122 ms | 279 ms and 127 ms |
| Hive | 2000 tables, all of them | 670 ms | 788 ms |
| Hive, with two correlated reads of `TABLE_PARAMS`, which was not kept | 2000 tables, all of them | 670 ms | 1320 ms |
| Impala | any | no difference: no statement is added |

SAP HANA ran on a database with 3000 tables and 6000 indexes made for the
measurement, and the scratch schema was dropped after it. The one table cost is
a fixed 5 to 10 ms for the extra join to `SYS.OWNERSHIP`, which grows by about 5
ms from 12 tables to 3000. The join to `SYS.M_TABLES` was measured four ways. With
a literal, a plain `= ?` and a plain `LIKE ?`, the planner reads one row. With
the `(? = '' OR x LIKE ?)` form that every filter here uses, and with a `CASE` in
its place, it reads all of them, for 62 ms when one table is asked for and for
94 ms when 3000 are. D47 does not allow a scan that follows the whole catalog,
so `Table.Size`, `Table.Rows` and `Index.Size` stay NULL. A user who has no
grant reads `M_TABLES` and gets no rows, so a refusal was not the reason.

Exasol keeps a fixed cost that comes from the left join to `EXA_ALL_OBJECT_SIZES`.
Any join to it, by id or by name or as a scalar read, costs 83 to 87 ms for one
table at 3000 tables and 41 ms when the database holds only the fixture, so it
grows with the catalog. An inner join takes 10 ms, but it drops a table that the
user can see in `EXA_ALL_TABLES` and cannot see in `EXA_ALL_OBJECT_SIZES`, and no
parity principal proved that the two views agree for every user. The left join is
kept and the cost is written down. If Ken wants the 10 ms, the test is the parity
run with an inner join. The key columns are read in a derived table with the same
filters as the tables, and that took one table from 577 ms to 96 ms.

Hive reads every `sys` table through a JDBC storage handler that does not push a
filter down, so one table by name costs 594 ms before the change, which is a
scan of the whole metastore. The pivot of three parameters in one pass costs the
same as the old read of one, and the first version with two correlated reads was
twice as slow.

## What each release fills

- Firebird 3.0 fills every field above except `Index.Predicate` and the
  `sql_security` option, which need 4.0 and 5.0. 4.0 adds the option. 5.0 adds
  the predicate. A partial index in the fixture is skipped before 5.0.
- SAP HANA 2.00.088 fills every field above. 2.00.076 and 2.00.082 were not run.
  Every statement reads columns that SPS 07 has, and the existing queries of the
  model have no version fragment for that reason, but this is not measured.
- Exasol 2026.2.0 fills every field above. 2025.2.1 was not run.
- Hive 4.0.1 and 4.2.1 fill the same fields. 4.0.1 refuses a transactional
  table in the image, so the fixture makes its constraint table with ENABLE and
  no transactional property, which both accept.
- Impala 4.4.1 and 4.5.2 fill the same fields. `book` has no statistics, and its
  rows and size are NULL, which the test reads back.

## Parity and conformance

The parity golden has no change for Firebird, Exasol and Hive. The grantee of
Firebird and the owner and the grantee of Exasol read the same rows, and the same
values, as the administrator for the new fields, and Hive has no authorization
in the image. For SAP HANA the golden is unchanged on a clean server. A first
run on a server where the measurement had left a schema and two users found two
differences, `schemas` and `access_methods`, which were the extra schema that the
grantee has no grant on, and the first parity run was repeated on a fresh server.
Impala has no principal. The conformance golden is unchanged for all five,
because the canonical projection reads the columns and constraints of the core
tables only.

## The hazards of D197

Every new scan target is a nullable type. Firebird, Exasol and Hive have text
columns that a left join leaves NULL, and each test reads a view, whose owner is
there and whose other fields are NULL. HANA reads `OWNER_NAME` through a left
join, and a table the user cannot see in `SYS.OWNERSHIP` gives NULL, which
the type holds. None of the five has an INCLUDE column.

## What was left open

- `Constraint.Enforced` for Firebird. A constraint cannot be disabled, so all
  are enforced, but no catalog column says so, and D207 left the same question
  open for CrateDB and Redshift. Ken decides whether the fact of the product is
  enough. The change is one literal in `models/firebird/relation.go`.
- The Exasol left join, which costs 85 ms at 3000 tables. See the cost.
- The SAP HANA size and row count of a table, and the size of an index. See the
  cost. A caller who needs them can read `SYS.M_TABLES` for the tables it names.
- `Table.Options` for Hive and Impala. A list of the keys that are settings is a
  choice of the author and not a fact of the catalog.
