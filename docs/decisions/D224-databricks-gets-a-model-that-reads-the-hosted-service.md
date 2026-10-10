# D224. Databricks gets a model that reads the hosted service

Status: Amends D194, amended by D229.

## The decision

Ken asked on 2026-10-11 for a model for Databricks SQL on Unity Catalog, and D218
made it one of the hosted services that get a model. The model is
`models/databricks`. It answers 17 of the 65 kinds: databases, schemas, tables,
columns, views, constraints, constraint columns, functions, routine parameters,
comments, partitioned tables, privileges, policies, collations, foreign servers, the
current schema and the current user.

It reads the hosted service and nothing else. Nothing else runs it, so the tier of
the entry is Verified and CI never runs it.

## What it reads

The INFORMATION_SCHEMA of the catalog that the connection is in. The statements
write `information_schema.tables` with no catalog in front, and Databricks resolves
that name to the current catalog, so a connection reads one catalog, as a BigQuery
connection reads one dataset (D220). `Databases` is the exception, because
`system.information_schema.catalogs` lists every catalog that the principal can see,
and the view of the session lists only its own.

INFORMATION_SCHEMA shows a principal only the objects that it can use. A principal
with no EXECUTE on a function does not see the function, and a principal with no
READ VOLUME on a volume does not see the volume, so the reader of the test account
reads no routine and no volume. `docs/COVERAGE.md` says so.

## What is not in INFORMATION_SCHEMA

A column default, an identity column, a generated column and a CHECK constraint are
not in any relation. COLUMNS reports `column_default` as NULL and `is_identity` and
`is_generated` as NO for columns that have them, and CHECK_CONSTRAINTS is empty. The
four are in the table properties, which SHOW CREATE TABLE and SHOW TBLPROPERTIES
read. The size, the row count, the location and the clustering columns of a table
are in DESCRIBE DETAIL. Those are statements and not relations, and D146 allows a
walk for Impala, InfluxQL, Elasticsearch and OpenSearch alone, so the model does not
answer them. The tests assert each absence, so a deliberate gap is not mistaken for
a query that forgot. Ken can allow a walk, and the backlog holds what it adds.

The model passes on what the catalog says. It does not turn the NO of an identity
column into a guess, and it does not hide that the catalog is wrong about it.

## The analogues

Each is a choice that Ken can reverse:

- A connection of Lakehouse Federation is a foreign server. CREATE SERVER is a
  synonym of CREATE CONNECTION. The account cannot create a connection, so the query
  was shown to run and to return no row.
- A row filter and a column mask are policies. A row filter has the command all and a
  column mask has the command select. Neither has a name of its own, so the name is
  the function.
- A partition column of a Delta table is a partition by the value of the column, so
  the strategy is list, and a table partitioned by two columns has a row for each. A
  table with liquid clustering has no row, because the clustering columns are in
  DESCRIBE DETAIL.
- A catalog is a database, as in BigQuery (D220).

## The leads that were wrong, and the ones left out

Gemini and DeepSeek sorted the unanswered kinds. Both failed on the long question,
and each answered a short one. DeepSeek named five relations that do not exist:
`system.information_schema.table_sizes`, `roles`, `applicable_roles` and `sequences`,
and a column `clustering_columns` of TABLES. Gemini named CHECK_CONSTRAINTS,
COLUMNS.column_default and the identity columns of COLUMNS as reserved and empty,
which is what the service answers. The test asserts that the four relations do not
exist.

The relations that exist and hold nothing here are external_locations,
storage_credentials, shares, recipients and providers of the system catalog. An
external location is nearly a tablespace and a share is nearly a publication. The
account holds none of either and cannot create one, so no row can be checked, and
the model leaves both out. `system.access` and `system.storage` refuse the principals.

## The driver

dburl v0.49.0 names `github.com/databricks/databricks-sql-go` for the databricks
scheme, and the test module uses v1.16.0 of it (D154). A later dburl names
`github.com/xo/dbimp/databricks`. That driver is in the working tree of dbimp and no
tag holds it, since v0.16.1 has none, so the test module follows dburl when a tag
names it, as it did for rqlite (D148, D151) and BigQuery (D220).

dburl v0.49.0 reads the scheme in an older form, `databricks://<token>:<workspace>@<warehouse>`,
and writes the token into the host of the DSN. The form of the credential file is
`databricks://token:<token>@<host>:443/sql/1.0/endpoints/<id>`, which is the form that dbimp's driver reads
too, except for the path. So `dbrun` hands the driver the string of the file without
its scheme. The driver writes the text of an error to its own log, and a connection
error held the token, so the tests turn the log off.

## The version

`SELECT current_version()` answers a struct. Its `dbsql_version` is the release of the
SQL channel, such as 2026.38, and its `dbr_version` is the runtime of a cluster, which
is NULL on a warehouse. The model reads whichever is set, and parses it as a release
with two numbers. `SELECT version()` answers the release of Spark and a hash, such as
4.2.0, which usql prints because its databricks driver declares no version. The two
numbers differ, and `docs/USQL.md` records it.

## Parity

Databricks has no containment inside a workspace. The principals are the
administrator, a service principal that owns the schema, and the reader, a service
principal that holds USE CATALOG, USE SCHEMA and SELECT on it. `TestPrivilegeParity`
records the answers.

## The cost

The warehouse is a 2X-Small serverless warehouse, shared with dbimp, that stops after
ten minutes and has a daily quota on the free edition. The fixture builds no
materialized view, because DROP MATERIALIZED VIEW leaves a hidden table and an event
log table behind, and the pipeline spends the quota. The fixture runs about 30
statements, and a run of the tests takes about ten minutes.
