# D210. The products without a relational catalog fill the describe fields they have a source for

Status: Decided.

## The decision

D198, D199 and D201 gave the root types new fields, and wave 1 refreshed the
relational models (D205 to D207). This decision audits the products that have
no relational catalog or a thin SQL layer. It fills a field only where one
statement produces it at a cost that follows the rows returned (D47, rule
13). Every other field stays NULL and the reason is written here.

The servers were Cassandra 3.11.19, 5.0.9 and ScyllaDB 2025.1.15, Couchbase
7.6.12 and 8.0.3, Druid 38.0.0, Avatica 1.29.0 (HSQLDB), Drill 1.22.0, YDB
26.3.1.19, Neo4j 5.26.31, Trino 483, Presto 0.299, ArangoDB 3.12.12 and
InfluxDB 3.12.0. Gemini was asked about the products that looked absent
and answered none for SurrealDB, Elasticsearch, OpenSearch and Solr. DeepSeek
spent its tokens on thinking twice and gave no answer. The leads that came
from the catalogs themselves were each run against a server.

## The audit

| Product | Field or kind | Source, or why none |
| --- | --- | --- |
| Cassandra, ScyllaDB | `Table.Options` | `compaction`, `compression`, `gc_grace_seconds` and `default_time_to_live` of the row of `system_schema.tables`, as `compaction=SizeTieredCompactionStrategy, compression=LZ4Compressor, gc_grace_seconds=864000, default_time_to_live=0` |
| Cassandra, ScyllaDB | `Index.Using`, `Index.Options` | `class_name` and the other entries of the `options` map of `system_schema.indexes`, which are set for a custom index such as the storage attached index of 5.0 |
| Cassandra, ScyllaDB | `Function.Prosrc` | `body`, the same text as `Source`. An aggregate has none |
| Cassandra, ScyllaDB | `Table.Size`, `Table.Rows` | `system_views.disk_usage` (4.0 and later) and `system.size_estimates` are other tables, and CQL has no join, so one statement cannot add them to a table row |
| Cassandra, ScyllaDB | `Column.Compression`, `Storage`, `StatsTarget` | no source. Compression is a table setting, which is in `Table.Options`, and a column has none |
| Cassandra, ScyllaDB | `Owner`, `Persistence`, `Partitions`, `Policies`, `NotNulls`, `Inherits`, `Constraint.Enforced` | no source or no object. A partition key is a column, and the catalog has no flag for a primary key that is enforced |
| Couchbase | `Index.Predicate`, `Index.Valid` | `condition` and `state` of `system:indexes`, on 7.6 and 8.0 |
| Couchbase | `Index.Definition`, `Index.Options` | `metadata.definition` and `with` of `system:indexes`, on 8.0 only. 7.6 has neither, so they are NULL there |
| Couchbase | `Function.Prosrc` | `definition.text`, the same text as `Source` |
| Couchbase | `Table.Rows`, `Table.Size` | `count` and `size` of `system:keyspaces_info`. See the cost: the server asks the data service once for each collection |
| Couchbase | `Index.Using` | no source. `Type` already holds the service, which is gsi |
| Druid | `Table.Options` | `IS_JOINABLE` and `IS_BROADCAST` of `INFORMATION_SCHEMA.TABLES`, as `joinable=NO, broadcast=NO` |
| Druid | `Table.Size`, `Table.Rows` | `sys.segments` summed by datasource. See the cost: the statement reads every segment of the cluster, also for one table |
| Avatica (HSQLDB) | `Table.Owner` | `SCHEMA_OWNER` of `SCHEMATA`, because HSQLDB gives every object in a schema to the owner of the schema. It is NULL for another user's schema, as `Schema.Owner` is |
| Avatica (HSQLDB) | `Table.Persistence` | `TABLE_TYPE`: permanent, or temporary for a global temporary table. NULL for a view |
| Avatica (HSQLDB) | `Table.Rows` | `CARDINALITY` of `SYSTEM_TABLESTATS`. See the cost: the join is quadratic |
| Avatica (HSQLDB) | `Table.Size` | `USED_SPACE` of `SYSTEM_TABLESTATS` is NULL for a memory only database, so no value was measured |
| Drill | `Table.Rows` | `NUM_ROWS` of `INFORMATION_SCHEMA.TABLES`, which `ANALYZE TABLE` stores in the Metastore. It is NULL for a view and a system table |
| Drill | `Table.Options` | `LOCATION` and `TABLE_SOURCE` are where the table is and not a setting, so they are not used |
| YDB | `Table.Owner` | `Sid` of `.sys/auth_owners` |
| YDB | `Table.Size`, `Table.Rows` | `DataSize` and `RowCount` of `.sys/partition_stats`, summed over the partitions of a table, in the read that `Tables` already made. YDB updates them about every half minute, so a new table reads 0 for a while. `Size` leaves out the index tables |
| Neo4j | `Index.Valid`, `Index.Definition`, `Index.Using` | `state = 'ONLINE'`, `createStatement` and `indexProvider` of `SHOW INDEXES` |
| Neo4j | `Index.Options` | `options.indexConfig` of `SHOW INDEXES`, written as sorted `key=value` pairs by the model, because Cypher has no list sort. NULL for an index with no setting |
| Neo4j | `Table.Rows` | `db.stats.retrieve('GRAPH COUNTS')` needs `db.stats.collect()` first, which is a second statement. A count of the nodes of each label is a scan |
| Trino, Presto | all of them | no source. `system.metadata.tables_authorization` holds the owner and it is empty for the memory connector, so no value was measured. `SHOW STATS` is one statement for each table. The memory connector has no `$partitions` table, and Presto has no `tables_authorization` |
| ArangoDB | `Table.Rows` | `COLLECTION_COUNT(name)` works in AQL. See the cost. It is not added, because a cluster counts with a round trip for each shard and no cluster was measured |
| ArangoDB | the rest | `COLLECTIONS()` returns an id and a name. The index list and the figures are in the HTTP API, and Ken chose on 2026-10-01 that a walk is not allowed (D163) |
| InfluxDB 3 | `Table.Rows`, `Table.Size` | `system.parquet_files` holds them for a table, and holds nothing until the first snapshot of the write buffer. A write was not in it after 12 seconds, so a value is a count of the persisted rows only. The fixture cannot wait for a snapshot |
| InfluxQL | all of them | no source. A cardinality is one statement for each database |
| SurrealDB | all of them | no source. `INFO FOR DB` returns the text of each `DEFINE` and a count is one `INFO FOR TABLE` for each table |
| Elasticsearch, OpenSearch | all of them | no source. The SQL layer has no count or size, and the cat APIs are HTTP |
| Solr | all of them | no source |
| GizmoSQL | all of them | left to the columnar agent, because it shares the DuckDB model |

## The statements

Every statement returns the same columns on every release of its product.

- Cassandra `Tables` adds four columns of the row, and the model writes the
  options text. The map columns arrive as maps from the driver, so `textMap`
  reads them. `Indexes` adds the `options` map. `Functions` adds `body` once
  more as `prosrc`, and `Aggregates` pads it. The padding is written
  as a stand in column on ScyllaDB, which accepts no literal in a select
  list, so the scan reads that column for a function only.
- Couchbase `Indexes` adds four columns. A field a release lacks reads
  `IFMISSING(..., NULL)`, because N1QL leaves a missing field out of the row.
  `CONCAT2` joins the options.
- Druid `Tables` adds one `CASE`.
- Avatica `Tables` adds a join to `SCHEMATA` and a `CASE`.
- Drill `Tables` adds `NUM_ROWS` cast to `BIGINT`.
- YDB `Tables` reads a new relation, `tableStats`, in place of `tablePaths`.
  `Roles` still reads `tablePaths`.
- Neo4j `Indexes` yields four more columns and the scan writes the options.

## The cost

Times are wall time in the client.

| Product | Scale | Without | With |
| --- | --- | --- | --- |
| Couchbase, `keyspaces_info` joined | 302 collections, all of them | 2 ms | 3.98 s |
| Couchbase, the same | one collection by name | 1.2 ms | 14.7 ms |
| Druid, `sys.segments` joined | 3000 segments in one datasource and 4 others, all tables | 15 ms | 27 ms |
| Druid, the same | one table by name | 15 ms | 26 ms |
| Avatica, `SYSTEM_TABLESTATS` joined | 2000 tables, all of them | 2.3 ms | 190 ms |
| Avatica, the same | 4000 tables | 2.1 ms | 710 ms |
| Avatica, the same | one table by name | 0.4 ms | 1.8 ms |
| Avatica, `SCHEMATA` joined (kept) | 4000 tables | 2.1 ms | 3.1 ms |
| YDB, the aggregate (kept) | 601 tables | 3.4 ms | 4.0 to 5.3 ms |
| ArangoDB, `COLLECTION_COUNT` | 1509 collections | 3.6 ms | 11.6 ms |

Couchbase asks the data service for each collection, which is 13 ms each, so
the cost follows the rows and every row is a round trip. D47 does not allow
that. Druid reads every segment of the cluster for one table, so the cost
follows the catalog and the rows returned do not decide it. Avatica doubled
its tables and the time grew almost four times, which is a quadratic join.
Nothing was added for these three. ArangoDB is linear on a single server and
is left out for the reason in the audit. The fields that were kept read a
column of the row that the statement already reads, or one small join.

## Parity and conformance

`TestPrivilegeParity` passed for every product above except YDB. The YDB
golden holds the text of the refusal that an ordinary user gets, and that text
names the line and the column of the failing read in the statement. The
column moved, and `parity.txt` has that one line changed. No principal gained
or lost a refusal. A Druid ordinary user reads the same `sys.segments` rows as
the administrator for its datasources, and the Avatica owner is NULL for a
user that does not own the schema. The conformance golden did not change.

## What each release fills

- Cassandra 3.11, 5.0 and ScyllaDB 2025.1: the table options and the function
  `Prosrc`. The index class and options are filled where the server has a
  custom index, which the fixture builds on 5.0 only.
- Couchbase 8.0: `Predicate`, `Valid`, `Definition`, `Options` and
  `Prosrc`. 7.6: `Predicate`, `Valid` and `Prosrc`. 7.2 is below the floor of
  the model.
- Druid 37 and 38: the table options. Avatica 1.28 and 1.29: the owner and the
  persistence. Drill 1.21 and 1.22: the rows. YDB 26.2 and 26.3: the owner, the
  size and the rows. Neo4j 5.26 and 2026.09: the four index fields.

The Nightly releases (Cassandra 4.0 and 4.1, ScyllaDB 2026.1 and 2026.2) were
not run locally. The statements are the same as on the releases that ran.

## Open

ArangoDB `COLLECTION_COUNT` is cheap on one server and unmeasured on a
cluster. It is in `docs/BACKLOG.md` for Ken.
