# D216. Spanner gets a model that reads Spanner Omni

Status: Amends D194, amended by D219.

## The decision

Ken asked on 2026-10-10 for a model for Google Cloud Spanner, in the GoogleSQL
dialect. D194 said that dbmeta builds no model for a hosted service other than
Amazon Redshift and Snowflake. This decision makes Spanner the third one. It
changes D194 for Spanner only. Athena, BigQuery, Cosmos, Databricks,
MaxCompute and Tablestore stay without a model.

The model is `models/spanner`. It answers 21 of the 65 kinds. A database in the
PostgreSQL dialect has the catalog of PostgreSQL, and `models/postgres` reads
it, so this model does not.

## What it was measured on

Spanner Omni 2026.r4-lts, which is the engine of the hosted service as a
container. Cloud Spanner itself is not measured. Ken will have the `dbsetup`
session make an instance, and the model must be run against it before anyone
calls it finished for the hosted service.

The release is Tested, which is the cadence it recorded while it was Staged
(D119, D120). The hosted entry of Cloud Spanner is Verified, because a model reads
it now (D119). The image is public: a request for its manifest needs no
credential, so CI pulls it as it pulls the other images.

## The driver

dburl v0.49.0 names `github.com/googleapis/go-sql-spanner` for the spanner
scheme, and the test module uses it at v1.26.0 (D154). A later dburl names
`github.com/xo/dbimp/spanner`, which does not exist, because dbimp dropped its
driver when it found that Spanner Omni has no REST interface. The test module
follows dburl when a tag names a driver that dbimp ships.

## The version

Spanner has no version function, and Spanner Omni tells its release only to its
own program, as `spanner --version`. The one number that SQL reads and that
moves with the service is the highest optimizer version in
`SPANNER_SYS.SUPPORTED_OPTIMIZER_VERSIONS`, which is 9 on 2026.r4-lts. The model
uses it as the version. It gates no fragment today. usql declares no `Version`
for Spanner, so it runs the generic `SELECT version();`, which Spanner refuses.

## The analogues

Three kinds are an analogue of the object that PostgreSQL has, and each is a
choice that Ken can reverse:

- A change stream is a publication. It names tables and columns and says which
  kinds of change it reports, and `exclude_insert`, `exclude_update` and
  `exclude_delete` are the insert, update and delete flags.
- A locality group is a tablespace. It says whether the data is on solid state
  or on disk.
- The check constraint named `CK_IS_NOT_NULL_` that Spanner makes for each NOT
  NULL column is a not null constraint. Spanner refuses that prefix in a name a
  person writes, so the prefix is safe to read. `Constraints` leaves these out
  and `NotNulls` reads them, as for PostgreSQL 18.

Two lookalikes are not an analogue, and the model leaves them out as kinds. An
interleaved table is not inheritance, because the child has no column of its
parent, so the parent is in `Table.Options` and `Inherits` is not answered. A
row deletion policy deletes old rows and restricts no access, so it is in
`Table.Options` and `Policies` is not answered.

## What it does not answer

SESSION_USER fails on Spanner Omni with "the user name is unknown", so
`CurrentUser` has no source that was measured. `Databases` has none either,
because no function returns the name of the database. Table size is in
`SPANNER_SYS.TABLE_SIZES_STATS_1HOUR`, which holds one row each hour and was
empty on a server that was up for minutes, so `Size` is absent until it is
measured. docs/COVERAGE.md and docs/BACKLOG.md hold the rest.

## Parity

Spanner Omni, started with no authentication method, enforces no database role.
A session that names a role, even one that does not exist, reads and writes as
the administrator. There is no second principal to measure, so Spanner is in
`parityExempt` with that reason. A deployment started with `--auth-methods
password` has users, and the Go driver cannot sign in with the opaque password
protocol, so that is not measured either.
