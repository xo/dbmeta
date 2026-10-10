# D230. The common facts of the usql drivers move here

Status: Amends D143.

## The decision

usql kept a few facts about each product in its drivers, as code that names a database.
Ken asked on 2026-10-11 for usql to apply them for every driver with no such code, and for
dbmeta to hold the facts. Gemini and DeepSeek agreed with the split below.

Four facts live here. The DSN query keys of a Go driver do not. The keys that usql
sets for MySQL, SQLite, Cassandra, Couchbase and Hive belong to the drivers, and hard rule 1
keeps every connection string detail out of this module. They go to dburl, which owns the
URL. Oracle's ORACLE_SID fallback and the UNIQUEIDENTIFIER override of SQL Server stay in
usql.

## What is added

`Info` gains four fields.

- `Product` is the text to print for the version of a product that has no statement for it.
- `EveryStatementIsAQuery` says the server sends no count, so a client runs every statement
  as a query. It is true for ArangoDB, Neo4j, SurrealDB, InfluxDB and InfluxQL.
- `WritesNeedAutocommit` says a write must not run inside a transaction. It is true for Trino
  and Presto, which start a transaction and then refuse the write with
  AUTOCOMMIT_WRITE_CONFLICT. Every other product is found by a BeginTx that fails with
  ErrNotSupported, and usql keeps that rule.
- `ScanTypes` says a client scans each column into the Go type that the driver reports. It
  is true for MySQL, TiDB, Vitess, SingleStore and Databend. MariaDB is the mysql dialect.

`Info.Placeholder` already existed and is unchanged. `Dialect.Placeholder(n)` is the helper
that turns a position into the text: `?`, `$1`, `:1` or `@p1`. The Snowflake and Hive models
set it to a question mark, where it was a function that panicked. A query of either model
binds with `Info.Literal` and never calls it, so nothing else changes.

## Dialects with no model

Some products that usql reaches have no model, and no model is created for them. The root
package holds one small table, `unmodeled`, with a row for each. DynamoDB has the text
"Amazon DynamoDB", Pinot has "Apache Pinot" and csvq has the placeholder `?`. The `Pinot`
dialect is new and its value is the dburl dialect `pinot`. A value on a model wins over the
row, and a test fails when a row names a dialect that has a model.

The accessors are `Dialect.Placeholder`, `Dialect.Product`, `Dialect.EveryStatementIsAQuery`,
`Dialect.WritesNeedAutocommit` and `Dialect.ScanTypes`. The last three return false for a
dialect with no model.

## What stays in usql

The InfluxDB version, which usql reads with GET /ping, and the csvq version, which is
`SELECT @#VERSION`, stay in usql until those products have models. The trailing semicolon
that Athena and Cosmos DB strip also stays. It is not part of this decision.
