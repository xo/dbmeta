# D229. The hosted tests move to the drivers of dbimp and Spanner stays on go-sql-spanner

Status: Amends D154, D220, D222, D224 and D228.

## The decision

dburl v0.50.0 names `github.com/xo/dbimp/bigquery`, `athena` and `databricks`, and
dbimp v0.17.0 tags them. The tests of BigQuery, Athena and Databricks, and `dbrun`,
now open those drivers, so D154 is met for them. The test module no longer
requires `gorm.io/driver/bigquery`, `github.com/uber/athenadriver` or
`github.com/databricks/databricks-sql-go`.

The credential files that dbsetup wrote hold the URL that dburl reads, and dburl
turns it into the DSN that each driver takes. `dbrun` hands the tests that DSN,
and the parity tests read the file of each reader, `<name>-reader`, which holds
the key of the reader.

Spanner is the exception. dburl v0.50.0 names `github.com/xo/dbimp/spanner`, and
the Spanner tests still open `github.com/googleapis/go-sql-spanner`. D154 has one
more exception now, and the reasons are below.

## What changed for each product

- Databricks. No model changed. The driver returns an array as a slice, where
  the earlier driver returned its JSON text, so one test writes the slice as JSON.
  `dbrun` no longer cuts the scheme from the URL.
- Athena. The driver binds with the ExecutionParameters of the service, writes
  each value as a literal that Athena parses, and returns a NULL as nil. So the
  Athena model lost `Info.Literal` and the function that wrote the literals, and it
  uses the placeholder `?`. `TestAthenaFilterLiterals` stays, and it asserts that a
  quote, a backslash and an injection stay values. The tests no longer change the
  scheme, the key of the workgroup or `missingAsNil`. The driver returns the
  properties of SHOW TBLPROPERTIES as one text with a tab, and no model reads it.
- BigQuery. Nothing changed. The URL names the key file in `credential_file`, and
  the reader has its own URL.

Conformance and parity did not change for any of the three.

## Why Spanner stays

dbimp tagged a driver for Cloud Spanner, and it ran the whole model on the hosted
service. The tests of a measurement on 2026-10-11 passed, including conformance with
no change to a golden file. Three facts stop the move.

- The driver of dbimp and go-sql-spanner both register the name `spanner` with
  `database/sql`. A binary that imports both fails at start with "Register called
  twice". Ken decided that the Spanner Omni tests open go-sql-spanner by hand, and
  the hosted tests open the driver of dbimp. Both are in the one test binary, and in
  `dbrun`, so this cannot be built.
- The driver of dbimp speaks REST, and Spanner Omni speaks gRPC only. It cannot
  reach Omni, which is the Tested release in CI.
- The driver of dbimp has no key for a database role, and it refuses a key that it
  does not know. D219 uses a role as the principal of two parity scenes and of
  `TestSpannerEnforcesRoles`. The driver also sends one DDL statement for each
  request and has no batch, so the fixture of about 60 statements took 461 seconds
  where the batch of go-sql-spanner takes seconds.

So the tests keep go-sql-spanner for Cloud Spanner and for Omni, and `dbrun` turns
the URL of the credential file into its DSN, in `test/internal/spannerdsn`. The
reader reads its file the same way. What is needed to end the exception is in the
backlog.

## The parity of Cloud Spanner

The credential file of Spanner now names the database `dbmeta` of the instance
`dbimp-spanner` in the project that dbimp shares. On it, the role `dbmeta_reader`
reads the same two schemas as the administrator, the default one and
`dbmeta_fixture`, where the earlier database gave it fewer. So `schemas` is no longer
in the section `spanner/same/role` of `parity.txt`, which matches the section for
Spanner Omni. The cause is the database and not the driver, because the driver is the
same. The earlier database is not available to measure again.

## What stays

D220, D222 and D224 are the history of how each model was measured, and they are
not rewritten. D216 and D219 stand. `dbrun` still sets `GOOGLE_APPLICATION_CREDENTIALS`
for a driver that reads it, which go-sql-spanner does.
`DBMETA_TEST_RUN` is new in `dbrun test`. It limits a run to the tests that its
pattern names, so that a person can run one test against a service that bills.
