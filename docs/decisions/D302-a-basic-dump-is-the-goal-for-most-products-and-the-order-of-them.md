# D302. A basic dump is the goal for most products, and the order of them

Status: Decided.

The six products with a full dump are PostgreSQL, MySQL and MariaDB, SQLite, DuckDB, SQL Server
and Oracle. D280 added six more that speak the SQL of one of them. The maintainers asked on
2026-11-11 for a dump of the schema of a table and its data for most of the other
products. Advanced objects such as views, routines, grants and partitions are not needed there.

## Decision

A basic dump is the columns of a table, with their types, nullability, defaults and primary key,
and the rows. A product has it when the model answers `Tables` and `Columns`, the types have
a Parse and a Render, and the style has a way to write a table and a value. A product that cannot hold a
constraint says so with `Style.RefuseConstraint`, and the script names what it left out.

Two models answered the question of which products come next, and the maintainers agreed with
the order. The source and the target are separate questions. A source needs only a
model that answers `Tables` and `Columns` and a `SELECT`. A target needs a way to create a table
and to load rows.

1. Tier 1 reuses a style that exists: CockroachDB, TiDB, Vitess, SingleStore, libSQL and rqlite (D280),
   and CrateDB, QuestDB and Redshift. Redshift is a hosted service, so it waits for a credential
   that the maintainers enter.
2. Tier 2 is the warehouses with a DDL of their own: ClickHouse, Snowflake, BigQuery, Databricks
   and Spanner.
3. Tier 3 is ordinary SQL with less demand: SAP HANA, Firebird, Exasol, Vertica, Databend,
   Avatica and Phoenix, and Cassandra and ScyllaDB.
4. Tier 4 is source only: Trino, Presto, Athena, Hive, Impala, Elasticsearch, OpenSearch, Solr,
   Druid, Drill, InfluxDB, InfluxQL, MaxCompute, Tablestore, SurrealDB and ArangoDB. A target
   needs a bulk interface or an ingestion spec, which a list of statements cannot hold.
5. A product that answers no `Columns` is neither a source nor a target: Neo4j, Couchbase and
   Cosmos DB. YDB answers none today, so it can be a target and never a source.

The statements stay statements (D274). A warehouse loads rows with INSERT batches first, and
a load of staged files is later work.

## CrateDB

CrateDB has its own SQL on the wire protocol of PostgreSQL. It has a primary key and a check, and it has no
foreign key, no unique key, no secondary index and no schema to create.

- The types are its own. It has no date, time, interval, binary or JSON type. A date is a
  timestamp at midnight, which is Lossy. The others have no equivalent and take the Text level
  when the client allows it. It keeps milliseconds, so more digits of a second are Lossy.
  A numeric needs its digits, so a source with none has no exact type.
- The model of `Columns` gives the digits of a numeric and the length of a string as part of
  `data_type`, as psql does, and the expression of a generated column as its `default`, as the postgres
  model does. Before this, both were lost.
- The catalog lists a field of an object column as a column, such as `o['x']`. The new field
  `Style.NestedColumn` makes the reader leave it out.
- A row shows after a refresh, so a client runs `REFRESH TABLE` before it reads rows that it wrote.

## QuestDB

QuestDB has its own SQL on the wire protocol of PostgreSQL. A table has columns. It has no NOT NULL, no default, no key, no
schema and no index that a table writes.

- The catalog sends the type of PostgreSQL for each column. A STRING, a SYMBOL, a UUID and an IPv4 all
  read as character varying, and a DATE reads as a timestamp. A DECIMAL and an array read as
  `unknown`, so a QuestDB table with one has no readable type for it.
- BOOLEAN, BYTE and SHORT have no NULL, so a NULL becomes false or 0, which is Lossy. INT and LONG use their
  least value as NULL.
- A decimal literal needs the suffix m. The new field `Style.DecimalLiteral` writes it.
- A table is written with its columns only. A key of the source is left out and the script says so.
- QuestDB keeps a plan for the text of a query, and refuses it when the table of that name was made
  again with other types. The test starts each query with a comment that no other query has.
- Rows apply in the background, so a client waits until a count shows them.

## Test

`TestABasicDumpMovesATable` makes one table with rows in a product, writes the script and the rows for
every other product in the two forms, and compares the rows. It runs PostgreSQL, MySQL, DuckDB,
CrateDB and QuestDB against one another. SQLite is left out because the driver of a machine
outside UTC moves a time that the test reads.
