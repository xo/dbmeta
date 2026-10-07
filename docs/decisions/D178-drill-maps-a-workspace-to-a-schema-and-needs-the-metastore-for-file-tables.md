# D178. Drill maps a workspace to a schema and needs the Metastore for file tables

Status: Decided.

## The decision

Ken asked on 2026-10-08 for an Apache Drill dialect. `models/drill` answers 10
of the 56 on 1.21.2 and 1.22.0, through dbimp's drill driver at v0.14.0, which
dburl v0.46.0 names. The dialect is `drill`, and `dialect.go` holds it. Ken
accepted the price of the Drill Metastore in D176, and reviews the mapping
below.

Drill answers SQL with Apache Calcite. It has an INFORMATION_SCHEMA with
CATALOGS, SCHEMATA, TABLES, COLUMNS, VIEWS and PARTITIONS, and a sys schema.
Every kind is one statement over one of them. No kind is a walk.

## The mapping

| Drill | Kind | Why |
| --- | --- | --- |
| the catalog DRILL | the catalog of every row, and the one database | CATALOGS has one row. D176 says the server is the catalog |
| a workspace or a system schema, such as `dfs.tmp` | schema | SCHEMATA. The name keeps the dot, because a name in Drill is the plugin and the workspace |
| `information_schema` and `sys` | schemas, system | listed only with with_system, as in every other model |
| a file table that the Metastore holds | table of the type `table` | see the price below |
| a view | table of the type `view`, and a view | TABLES and VIEWS |
| a table of `information_schema` or `sys` | table of the type `system table` | TABLES has the type SYSTEM TABLE |
| a column | column | COLUMNS. The columns of a view read ANY and nullable |
| NUM_NULLS, MIN_VAL and MAX_VAL of COLUMNS | column statistics | they exist after ANALYZE TABLE. The null fraction is NUM_NULLS over NUM_ROWS of TABLES |
| a row of sys.functions | function | one row for each overload. `source` is built-in or a JAR |
| a row of sys.options | setting | `kind` is the type, `optionScope` is the context and `accessibleScopes` is the access |
| `USER` and `SESSION_USER` | current user | they differ only with impersonation |
| `CURRENT_SCHEMA` | current schema | the join with SCHEMATA has no row when the session has no default schema |
| `SELECT version FROM sys.version` | the version | every user can read it. It is the statement that usql runs |

## The price of the Metastore

INFORMATION_SCHEMA lists no file table by default. A file table appears only
when the option `metastore.enabled` is on and the table went through
`ANALYZE TABLE ... REFRESH METADATA`. A model cannot set the option, which
needs an administrator. If the Metastore is off, Tables answers the views and
the system tables and no file table, and Drill gives no error. Ken accepted
this price. The fixture turns the Metastore on as the administrator and runs
ANALYZE for each table. COVERAGE.md records the limit, and
`TestDrillMetastoreOff` asserts it.

## The choices to review

1. Databases reads CATALOGS and counts as answered. The one row is DRILL.
2. The version comes from SQL and not from HTTP. D176 chose HTTP for the
   products that have no SQL source. Drill has `sys.version`, and both users can
   read it, so the ordinary user gets a version here.
3. Setting.Context is `optionScope` and Setting.Access is `accessibleScopes`.
   Setting.Display is absent, because `description` is not a value shown with
   its unit. The `status` column and the description have no field.
4. A function row is one overload, and ArgTypes is the signature text. The
   source is a field of its own. Language is always java.
5. The schema owner is NULL, because SCHEMATA holds the text `<owner>`.
6. CurrentSchema returns no row when the session names no default schema. The
   alternative is a row with an empty name, which hides a fact.
7. ColumnStats has no distinct count, because NDV is NULL on both releases. The
   null fraction needs the join with TABLES.
8. `IS NOT NULL` on a column of COLUMNS folds to true in Drill's planner, so the
   statistics statement tests `NUM_NULLS >= 0`.
9. Aggregates, RoutineParameters, PartitionedTables and Comments stay
   unanswered, with the reasons in COVERAGE.md. A kind whose only source scans
   the data is unsupported (D162).

## What it cost to measure

The cost check of D47 ran on 1826 relations, 1500 of them views. COLUMNS takes
2.2 seconds for every column and for one table, and the statistics join adds
about 0.4 seconds. The cost grows with the number of views and not with the
data, and EXPLAIN shows a HashJoin and the filter pushed into the scan.

## The terminator and the syntax

Drill refuses a semicolon at the end of a statement, so the model sets
`TerminatorStripped` (D176 and the usql finding). The syntax has block comments
and backticks.
