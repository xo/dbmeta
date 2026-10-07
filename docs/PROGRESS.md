# Progress

This file records where the work stands, so that a session that ends or
crashes can resume. Update it when a piece of work starts or ends. Work that
is known and not done goes in [`BACKLOG.md`](BACKLOG.md), and a decision goes
in [`decisions/`](decisions/README.md).

## Where the work stands

dbmeta has four releases and a fifth in preparation. v0.1.0 was tagged on
dd70b7a on 2026-10-02, v0.2.0 on 39b74b2 on 2026-10-07, v0.3.0 on 7cbe9bf on
2026-10-07, and v0.4.0 on 9388928 on 2026-10-08. The Tested tier passed in CI
on each tagged commit. Ken chose that CI stands in for a run of every tier,
and the Verified tier was not run again for any of them.

v0.5.0 holds the OpenSearch dialect (D181), the first run of Redshift against
Redshift Serverless (D182), and the amendment of D176 (D183).

v0.4.0 holds the Drill (D178), Elasticsearch (D177) and Solr (D179) dialects,
the wider role of the OpenSearch entry, and the fixed VoltDB entry (D180).

v0.3.0 holds the Druid dialect (D171), the move of the Trino and Presto tests
to dbimp's driver (D172), `api` for more products, the ClickHouse entry for
dbimp's driver, and the ODBC decision (D173).

v0.2.0 holds the container sweep of 2026-10-07, InfluxDB 3 Triggers (D170),
the DSN and `api` change for the products of dbimp (D167), and the ready
check of InfluxDB 1, which a nightly run of 2026-10-06 needed after its
connection was reset while the image restarted its server.

The commits of v0.1.0:

- 380e597 holds D169, which adds `Query.Each`, `Dialect.Pattern` and
  `dbmeta.Open` for usql, and makes `dbrun start` print the URL that usql
  takes. usql has the hash and builds on it.
- f8a6338 holds D167, which makes the DSN what `sql.Open` takes and adds the
  `api` field, D168, which is Ken's review of the six dialects of
  2026-10-01, and DuckDB's DollarQuotes. dbimp has the hash.
- 724e64e holds D166: Oracle 19c, Pinot and the InfluxDB release wait.

Staged and not committed:

- `api` for the products whose DSN is already the driver URL and that publish
  an HTTP port: rqlite, Couchbase, Databend, the BigQuery emulator and
  InfluxDB 3 (D167). A test checks it.

The druid dialect (D171) is built and merged, and it is staged and not
committed. It answers 7 of the 56 kinds on 37.0.0 and 38.0.0, on dbimp
v0.11.0 and dburl v0.42.0. The ordinary user cannot read the version, so only
the administrator has one. 38.0.0 was measured for the first time, and it
answers as 37.0.0 does.

The Trino and Presto tests moved to dbimp's driver, `github.com/xo/dbimp/trino`
at v0.12.0, which dburl v0.43.0 names for both schemes (D172). Trino 476 and
483 and Presto 0.299 pass live on it. Both entries print
`trino://user@host:port/memory/default` as their DSN and URL, and neither has
an ordinary user. The vendor clients are out of the test module.

The ClickHouse measurement for dbimp is done and sent. It is at
`scratchpad/clickhouse-wire.md` in this session, and the raw outputs and the
program are beside it. dbmeta offered to measure the HTTP interface next, and
Ken has not answered.

The ClickHouse entry now publishes the HTTP port 8123 as its second port,
prints an `api` for each release, and makes an ordinary user, `dbmeta_user`,
in Init, as dbimp asked for its ClickHouse driver. All four releases pass the
test module, and a stop and a start runs Init again without a fault. The user
has the name that every other entry uses, and not `dbimp_user`, which
dbimp's own recorded tests make for themselves.

The ClickHouse tests use dbimp's driver, `github.com/xo/dbimp/clickhouse` at
v0.13.0, which dburl v0.44.0 names (D174). The entry's DSN and URL are the
HTTP form on the second port. All four releases pass.

Elasticsearch and OpenSearch can walk SHOW statements (D175). Ken answered
the questions of the surveys in D176. The `DynamoDB` dialect is now
`dynamodb`, and the DSN of every DynamoDB entry ends with `tls=false`. The
test module pins dbimp v0.14.0 and dburl v0.46.0.

In progress:

- Drill (D178), Elasticsearch (D177), Solr (D179) and OpenSearch (D181) are
  built and pass on every release they answer. VoltDB has no model, because
  its catalog kinds are unanswered (D180), and its entry is fixed. The
  OpenSearch role is wider and the DynamoDB DSN ends with `tls=false`. v0.4.0
  holds Drill, Elasticsearch and Solr.
- On OpenSearch 2.19.6 Columns has no answer through dbimp's driver until dbimp
  reads a number where the schema says keyword. The 2.19.6 column checks skip
  under `describeReadable`.
- Elasticsearch, Solr and OpenSearch report an unknown version. Ken asked
  dbimp to support `SELECT version()` for them (D183). When dbimp tags it, each
  model gets a `VersionQuery`.
- Redshift ran once against Redshift Serverless and passes, with 11 of the 56
  kinds and three parity principals (D182). Snowflake is not set up.
- Ken's order for the next models: DynamoDB (survey first, and check the
  trailing semicolon), then Pinot, then Avatica.
- A daily check of the nightly workflow runs at 18:07 local time in this
  session, and expires after 7 days, on 2026-10-14. It watches the InfluxDB 1
  fix.

## Waiting

- Solr needs the cost check of D47 against thousands of collections.
- Snowflake and Redshift need a connection string, and VoltDB needs a license
  file.
