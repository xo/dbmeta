# D243. MaxCompute and Tablestore use the drivers of dbimp

Status: Amends D154, D237, D238, D239 and D241.

## The decision

The tests of MaxCompute open `github.com/xo/dbimp/maxcompute`, and the tests of
Tablestore open `github.com/xo/dbimp/tablestore`. dburl v0.54.0 names both for its
schemes, and dbimp v0.23.0 holds them. `test/go.mod` pins that version.

`test/go.mod` no longer requires `aliyun-odps-go-sdk`, `aliyun-tablestore-go-sql-driver`
or `aliyun-tablestore-go-sdk`. SQL cannot make a Tablestore table, so the fixture
makes its tables, secondary indexes and search indexes with the connector of
`github.com/xo/dbimp/tablestore`: `CreateTable`, `CreateIndex`, `CreateSearchIndex`,
`ListTables`, `ListSearchIndexes`, `DeleteSearchIndex` and `DeleteTable`. The service
refuses a table that has a search index, so the teardown deletes each search index
first. `DeleteTable` deletes the secondary indexes with the table. The depguard list
in `test/.golangci.yml` changed the same way.

## The Dialect of Tablestore

dburl v0.54.0 names the dialect of the scheme `tablestore`, so `dbmeta.Tablestore` is
now `"tablestore"` and no longer `"ots"`. The schemes `ot` and `ots` stay as aliases of
`tablestore`. The model package did not change, and the section of the parity golden
file is now `[tablestore/same/reader]`.

## The connection strings

The two forms changed, and each credential file changed with them.

- MaxCompute: `maxcompute://<key id>:<secret>@service.<region>.maxcompute.aliyun.com/<project>`.
  The path is the project. The old form had `/api?project=<project>`, and the driver
  adds the `/api` itself.
- Tablestore: `tablestore://<key id>:<secret>@<instance>.<region>.ots.aliyuncs.com/<instance>`.
  The old scheme was `ots`.

`dbrun` masks the key id and the secret of both, and `keyIDIsSecret` names the aliases.

## What changes in the tests

The driver of dbimp adds the semicolon that MaxCompute needs, so a statement without
one runs. `TestMaxComputeVersion` and `TestMaxComputeUnanswered` no longer expect a
refusal for it. The model still ends each statement with the semicolon and still says
`TerminatorRequired`, because the service needs it and another driver does not add it
(D238).

The driver of dbimp answers `SHOW TABLES` and `DESC` with rows, where the driver of
Alibaba returned none. The model does not read them, because INFORMATION_SCHEMA holds
more, so `TestMaxComputeUnanswered` no longer lists them.

The MaxCompute driver retries a reset connection itself, so the wrapper of D239 is gone
and `test/maxcompute_driver_test.go` with it. The Tablestore driver does not retry a
statement that can have reached the server, which is right for a library, and the
endpoint in Jakarta still resets a connection now and then. The tests keep a wrapper
for Tablestore in `test/tablestore_driver_test.go`. It wraps the connector of the
driver and retries a transient error. The model has no retry.

## What was measured

`dbrun test tablestore` and `dbrun test maxcompute` passed on the hosted services on
2026-10-30, including conformance and the parity of the reader. No golden file changed
except the name of the Tablestore section.
