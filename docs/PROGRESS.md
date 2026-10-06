# Progress

This file records where the work stands, so that a session that ends or
crashes can resume. Update it when a piece of work starts or ends. Work that
is known and not done goes in [`BACKLOG.md`](BACKLOG.md), and a decision goes
in [`decisions/`](decisions/README.md).

## Where the work stands

dbmeta has one release, v0.1.0, tagged on dd70b7a on 2026-10-02. Ken chose
that CI stands in for the run of every tier before it. The Tested tier passed
on 380e597 and the Nightly tier passed on 3a0ff0f. The Verified tier was not
run again for it.

`main` is ahead of the tag by the commit that fixes the ready check of
InfluxDB 1. The nightly run of 2026-10-06 failed on influxdb-1.11.8 with a
connection reset on the first request, and every nightly run before it
passed. The image starts a temporary server with authentication off, and the
old check passed against it. The check now also needs a query with no
credentials to be refused, which only the real server does. Both InfluxDB 1
releases pass with it. Ken started the test workflow by hand on that commit.

The commits before the tag:

- 380e597 holds D169, which adds `Query.Each`, `Dialect.Pattern` and
  `dbmeta.Open` for usql, and makes `dbrun start` print the URL that usql
  takes. usql has the hash and builds on it.
- f8a6338 holds D167, which makes the DSN what `sql.Open` takes and adds the
  `api` field, D168, which is Ken's review of the six dialects of
  2026-10-01, and DuckDB's DollarQuotes. dbimp has the hash.
- 724e64e holds D166: Oracle 19c, Pinot and the InfluxDB release wait.

Staged and not committed:

- dbimp asked for Druid, Drill, Solr, Elasticsearch, OpenSearch and DynamoDB
  (with Alternator) to print `druid://` and the like as `dsn` and `url`, with
  the `http://` address as `api` (D167). DynamoDB's DSN is now
  `dynamodb://key:secret@host:port?region=us-east-1`.
- dburl will change the GoPackage of its avatica scheme to dbimp's driver
  (dburl D47, not released). dbmeta has no Avatica model, so it needs nothing.

- The container sweep of 2026-10-07, which the tags decided and the tests
  confirmed. Tested live on the new tags: InfluxDB 3.11.6 and 3.12.0, rqlite
  10.5.2, Vitess 23.0.7 and 24.0.4, YDB 26.3.1.19, Databend 1.2.951 and
  Exasol nano.6. New pins without a live test, because no model reads them:
  Druid 38.0.0, Weaviate 1.39.10 and 1.40.0, Meilisearch, Qdrant, MongoDB
  9.0.2, OpenSearch 3.9.0 and GizmoSQL 1.40.0 and 1.41.0. The InfluxDB 3
  window moved up one line, so 3.9.13 is out, and MongoDB dropped 7.0.43.
- InfluxDB 3 answers Triggers (D170). The entry has a plugin directory, and
  its setup makes a do nothing trigger on `author`.
- Not done: InfluxDB 3 has no `api` field, because its DSN is the driver URL
  and nothing sets one. D167 says every product with an HTTP interface has
  one, which is true only where the DSN was `http://` or was set by hand.

## Waiting

- Nothing waits for Ken.
