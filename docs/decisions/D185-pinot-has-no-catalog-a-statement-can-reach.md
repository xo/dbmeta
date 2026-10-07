# D185. Apache Pinot has no catalog that a statement can reach

Status: Decided.

## The decision

Ken asked on 2026-10-08 for an Apache Pinot dialect. An agent measured
Pinot 1.4.0 and 1.5.1 on the same day, as the administrator and as
`dbmeta_user`, on the single stage engine and on the multi stage engine. SQL
on the Broker reaches no catalog. Ken chose to build no model. This is the
outcome of D166, which left Pinot out, and it is the same as D180 for VoltDB
and D184 for DynamoDB.

## What was measured

- `SHOW TABLES`, `SHOW SCHEMAS` and `SHOW FUNCTIONS` fail with code 150,
  "Non-query expression encountered in illegal context".
- `DESCRIBE` fails with code 150 on the single stage engine and with code 710,
  "Column not found", on the multi stage engine.
- `INFORMATION_SCHEMA.TABLES`, `INFORMATION_SCHEMA.SCHEMATA`, `sys.tables`,
  `system.tables`, `$sys.tables`, `metadata.tables` and
  `pg_catalog.pg_tables` fail with code 190, "table does not exist", or with
  "Object not found" on the multi stage engine.
- `SELECT version()`, `current_schema()` and `current_database()` fail with
  code 700, "No match found for function signature".
- A trailing semicolon is accepted. A model does not need
  `TerminatorStripped`.
- `SELECT * FROM <table> LIMIT 0` returns the column names and types of a
  table whose name the caller already knows. Nothing lists table names, so
  even Columns has no starting point.
- `dbmeta_user` gets the same answers as the administrator.

The catalog is in the Controller REST API only: `GET /tables`, `GET /schemas`,
`GET /tables/{table}/schema` and `GET /version`. The Broker has none of them.
A `Queryer` cannot send an HTTP request, and dbimp's driver reads only SQL.

## What a model needs

A model needs one of two things. dbimp can answer `SHOW TABLES`, `DESCRIBE` and
`SELECT version()` from the Controller, which is the SQL layer of the backlog.
The model then needs a walk, which D146 allows for Impala, InfluxQL,
Elasticsearch and OpenSearch only, so Ken must widen it. The other way is a
source in the root package that a model can use besides a `Queryer`. Ken chose
neither on 2026-10-08.

The entries of Pinot stay Staged with the cadence Tested, for dbimp's driver.
