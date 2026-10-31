# D245. Each model says how a column aligns

Status: Decided.

## The decision

Each model records on which side a client prints a column, as a function from
the column type that a driver reports. `Info.ColumnAlign` holds it, and
`Dialect.ColumnAlign(*sql.ColumnType) (Align, bool)` calls it. The result is
`AlignLeft` or `AlignRight`. The second result is false when the model records
nothing for the column, and then a client aligns by the values.

usql asked for the answer, so that each of its drivers can give tblfmt a column
alignment function. usql asked for a table of type names. Ken chose a function
instead, because a name does not work everywhere. go-ora reports no name at all,
and SQLite keeps the text of the declaration.

The root package does not import tblfmt. It defines `Align`, and a client maps it
to its own type.

## The rule

The rule is the one of psql. A numeric type prints on the right, and every other
type prints on the left. The numeric types are the integers, the floats,
DECIMAL and NUMERIC, money, and the numeric identifiers such as OID. `RightAligned`
builds the function from a list of the numeric names, in any case and without the
size, so DECIMAL(10,2) matches DECIMAL. A column with no type name says nothing.

## What was measured

Each name comes from a live server through the driver that dburl names, with one
column of each type. `TestColumnAlignIsMeasured` in the `test` module does it
again against every server that is running, so a driver that renames a type
fails the test. The test skips a dialect when its variable, such as
`DBMETA_POSTGRES`, is not set.

These models record the numeric names, and every other type aligns left:

- PostgreSQL, on pgx and lib/pq: INT2, INT4, INT8, FLOAT4, FLOAT8, NUMERIC, MONEY,
  OID, XID, XID8 and CID. pgx has no name for money and reports its OID, 790, so
  the list holds 790 too. lib/pq reports no name for XID8, so a client aligns it
  by the values. QuestDB speaks the same protocol and reports INT2, INT4, INT8,
  FLOAT4, FLOAT8 and NUMERIC.
- MySQL and MariaDB, on go-sql-driver/mysql: TINYINT, SMALLINT, MEDIUMINT, INT, BIGINT,
  the same five with UNSIGNED in front, FLOAT, DOUBLE, DECIMAL and YEAR.
- SQL Server, on go-mssqldb: TINYINT, SMALLINT, INT, BIGINT, REAL, FLOAT, DECIMAL,
  MONEY and SMALLMONEY. The driver says DECIMAL for a NUMERIC column too.
- Oracle, on go-ora v3: no names, because its DatabaseTypeName returns an empty
  string. The model reads the Go type that go-ora scans the column into. It is int64
  or float64 for NUMBER, and float32 or float64 for BINARY_FLOAT and BINARY_DOUBLE.
- SQLite, on mattn/go-sqlite3 and modernc.org/sqlite: the text of the declared type,
  such as `decimal(10,2)`. The model follows the affinity rules of SQLite, with
  DECIMAL and NUMERIC as numbers. BOOLEAN, DATE and DATETIME have a numeric affinity
  and align left. An expression has no declared type and says nothing.
- DuckDB: TINYINT, SMALLINT, INTEGER, BIGINT, HUGEINT, the unsigned forms, FLOAT, DOUBLE
  and DECIMAL, with the size in the name.
- ClickHouse: INT8 to INT256, UINT8 to UINT256, FLOAT32, FLOAT64 and DECIMAL. The driver
  takes Nullable and LowCardinality off the name.
- Databend: INT8 to INT64, UINT8 to UINT64, FLOAT32, FLOAT64 and DECIMAL.
- InfluxDB 3: the Arrow names INT8 to INT64, UINT8 to UINT64, FLOAT32, FLOAT64, DECIMAL128
  and DECIMAL256.
- Vertica: INT, FLOAT and NUMERIC. The driver says INT for every integer width and NUMERIC
  for MONEY.
- Exasol: DECIMAL and DOUBLE. The driver says DECIMAL for every integer width.
- Firebird: SHORT, LONG, INT64, INT128, FLOAT, DOUBLE and DECFLOAT. The driver names a
  NUMERIC or a DECIMAL after the integer that stores it.
- SAP HANA: TINYINT, SMALLINT, INTEGER, BIGINT, REAL, DOUBLE, DECIMAL, FIXED8, FIXED12 and
  FIXED16. The driver names a DECIMAL with a size after the bytes that store it.
- Trino and Presto: TINYINT, SMALLINT, INTEGER, BIGINT, REAL, DOUBLE and DECIMAL.
- Apache Hive: TINYINT, SMALLINT, INT, BIGINT, FLOAT, DOUBLE and DECIMAL.
- Apache Drill: INT, BIGINT, FLOAT4, FLOAT8 and VARDECIMAL. Drill refuses a SMALLINT.
- Apache Druid: BIGINT, INTEGER, FLOAT, DOUBLE and DECIMAL.
- Avatica: TINYINT, SMALLINT, INTEGER, BIGINT, DOUBLE, DECIMAL and NUMERIC. The driver says
  DOUBLE for a REAL.
- YDB: Int8 to Int64, Uint8 to Uint64, Float, Double and Decimal. The driver wraps a column
  that can be NULL in Optional, so the model takes the name out first.
- Apache Cassandra: TINYINT, SMALLINT, INT, BIGINT, VARINT, FLOAT, DOUBLE and
  DECIMAL. ScyllaDB shares the dialect and the driver, and was not measured on its own. No probe reached COUNTER, which lives only in a table, so it is not listed.
- Elasticsearch: BYTE, SHORT, INTEGER, LONG, FLOAT, HALF_FLOAT, SCALED_FLOAT, DOUBLE and
  UNSIGNED_LONG. OpenSearch reports BYTE, SHORT, INTEGER, LONG, FLOAT and DOUBLE, because it
  reports a scaled_float as DOUBLE and has no unsigned_long.
- Apache Solr: BIGINT and DOUBLE. A boolean field reports VARCHAR.
- Couchbase: NUMBER.
- BigQuery: INTEGER, FLOAT, NUMERIC and BIGNUMERIC, measured on the emulator.

These models copy the function of another model, and the probe passed on each: CockroachDB
and CrateDB copy PostgreSQL. TiDB, SingleStore and Vitess copy MySQL. libSQL and rqlite copy
SQLite.

These models say nothing, because the driver reports no name and no Go type for any column:
GizmoSQL, Neo4j, SurrealDB and ArangoDB. A client aligns their columns by the values.

## What does not fit

- Boolean aligns left, as in psql. It is BOOL in PostgreSQL, BIT in SQL Server and
  BOOLEAN in DuckDB.
- Interval aligns left. It is text to a driver.
- YEAR in MySQL is a number, and the model aligns it right. psql has no such type.
- A BIT column in MySQL aligns left, because it holds bits and not a number.
- A driver can return a DECIMAL as text or as bytes. The alignment follows the type,
  and not the Go type of the value.
- A Solr boolean field reports VARCHAR, so it aligns left, which is where a boolean goes.

## What this does not cover yet

These dialects have no function, so `Dialect.ColumnAlign` returns false for them:

- Hosted services that need a credential this machine does not hold: Athena, Cosmos,
  Databricks, MaxCompute, Redshift, Snowflake, Spanner and Tablestore.
- Spanner has an emulator, but it speaks the protocol of Spanner Omni, and go-sql-spanner
  reports no type names. The driver of dbimp that dburl names was not reachable.
- Apache Impala, because the build of its image failed on this machine.
- InfluxQL, because it reads a measurement and no probe wrote one.

Each one needs its names measured on a server, which D43 and rule 14 of AGENTS.md ask for
for a new dialect.
