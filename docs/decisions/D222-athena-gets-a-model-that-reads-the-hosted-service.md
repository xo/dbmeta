# D222. Athena gets a model that reads the hosted service

Status: Amends D194.

## The decision

Ken asked on 2026-10-10 for a model for Amazon Athena, and D218 made it one of
the hosted services that get a model. The model is `models/athena`. It answers 8
of the 65 kinds: databases, schemas, tables, columns, views, partitioned tables,
the current schema and the current user.

It reads the hosted service in the Trino based dialect of Athena engine version
3. Nothing else runs it, so the tier of the entry is Verified and CI never runs it.

## What it reads

Athena has no catalog of its own. The AWS Glue Data Catalog holds the databases,
tables and columns, and Athena shows them through the INFORMATION_SCHEMA of the
catalog `awsdatacatalog`: schemata, tables, columns and views. The catalog
`system`, which the Trino model reads, answers "Queries of this type are not
supported". Roles and table_privileges answer NOT_SUPPORTED. SHOW CATALOGS, SHOW
SESSION and SHOW STATS are syntax errors. SHOW FUNCTIONS, SHOW CREATE TABLE, SHOW
TBLPROPERTIES, SHOW PARTITIONS and DESCRIBE answer, and they are statements that a
SELECT cannot read. D146 allows a walk for Impala, InfluxQL, Elasticsearch and
OpenSearch only, so the table comment, the location, the partition values and the
function list are not answered. Ken can allow a walk for Athena, and the report of
the session that wrote this holds what it adds.

A partitioned table is a Hive table with a partition column, which COLUMNS marks
with the extra_info `partition key`. An Iceberg table partitioned by a transform has
no such mark, and it is not listed.

## The driver, and why the model writes its own literals

dburl v0.49.0 names `github.com/uber/athenadriver` for the awsathena scheme, and the
test module uses v1.1.15 of it (D154). A later dburl names
`github.com/xo/dbimp/athena`, which is not tagged yet. The test module follows dburl
when a tag names it, as it did for rqlite (D148, D151) and BigQuery (D220).

The driver has three faults that matter here.

- It accepts only the scheme `s3`, it names the workgroup with the key
  `workgroupName`, and it stops the whole query with "Missing data at column" for a
  NULL, unless the connection sets `missingAsNil=true`. The credential files that
  `dbsetup` wrote use `workgroup`, so the tests rename the key. The tests set
  `missingAsNil`.
- It binds a parameter by writing the value into the statement, the way a MySQL
  driver does. It writes a backslash before a quote and before a backslash, and
  Athena reads a backslash as itself, so a value with either is a different value or a
  syntax error. A boolean becomes 1 or 0, and Athena refuses `1 = true`.

So the dialect sets `Info.Literal` and renders each filter as a Trino literal, the
way Hive does (D78). The driver then binds nothing. A caller that opens the driver
must set `missingAsNil=true`, and `docs/DBRUN.md` says so.

## The version

Athena has no release that SQL reads. The engine version belongs to the workgroup,
which only the Athena API reports. `SELECT version()` is refused. usql reads
`node_version` from `system.runtime.nodes`, and Athena refuses that too, so usql
prints no version. The model declares no version query, and the version is unknown,
as D220 decided for BigQuery.

## The fixture and the account

The account cannot create a Glue database, so the fixture builds its tables in the
database `dbmeta`. The Redshift Spectrum test reads the same database and needs the
table `spectrum_t` in it, so the teardown drops only what the setup made. The
fixture holds the tables author, book, region and shipment with the shape of D53
minus every constraint, the view recent, a table partitioned by two columns, a table
that CTAS makes and an Iceberg table. The tables are external, so the setup writes no
file. The workgroup forces the output location, so CTAS cannot name one and writes
under `results/dbmeta/tables/`. No step needs an S3 client, so the test module has no
dependency on the AWS SDK of its own. The driver brings the AWS SDK for Go v1.

The Redshift Spectrum test expects one external table, so it fails while this
fixture is up. The two tests must not run at the same time.

## Parity

Athena has IAM and no user in SQL. The principals are the administrator and the
reader, an IAM user that can query the Glue database and cannot create or drop a
table. `current_user` is the number of the account for both, so `CurrentUser` cannot
tell them apart. `TestPrivilegeParity` records the answers.

## The cost

Athena bills the bytes a query scans, with a minimum of 10 MB for each query, and the
INFORMATION_SCHEMA queries scan none. Each statement is a job that takes between one
and six seconds, so a run of the tests takes about five minutes. The fixture writes
one row.
