# Progress

This file records where the work stands, so that a session that ends or
crashes can resume. Update it when a piece of work starts or ends. Work that
is known and not done goes in [`BACKLOG.md`](BACKLOG.md), and a decision goes
in [`decisions/`](decisions/README.md).

## Where the work stands

dbmeta has twelve releases. v0.1.0 was tagged on dd70b7a on 2026-10-02, v0.2.0
on 39b74b2 and v0.3.0 on 7cbe9bf on 2026-10-07, v0.4.0 on 9388928, v0.5.0 on
8f3115b, v0.6.0 on 90d0261 and v0.7.0 on abe2ca1 on 2026-10-08, and v0.8.0 on
cf48e36, v0.9.0 on 609c60b and v0.10.0 on 288a868 on 2026-10-09. v0.11.0 on
3df0d73 and v0.12.0 on 029c5ef on 2026-10-10. The Tested
tier passed in CI on each tagged commit. Ken chose that CI stands in for a run
of every tier, and the Verified tier was not run again for any of them.

v0.12.0 holds the four decisions of 2026-10-10 (D212): `Constraint.Enforced` is
true for Firebird and MariaDB, ArangoDB counts the rows of a collection, and
Redshift reads the size, rows and options of a table from `SVV_TABLE_INFO`.

v0.11.0 holds the refresh of every model with the fields of D198 to D203 where
the product has a source: the MySQL family (D205), SQL Server and Oracle
(D206), Snowflake, Redshift, CrateDB and QuestDB (D207), ClickHouse, DuckDB,
SQLite, Databend and Vertica (D208), Firebird, SAP HANA, Exasol, Hive and
Impala (D209), and the audit of the products without a relational catalog
(D210). `Policy.Enabled` is new (D211).

v0.10.0 holds the Snowflake dialect finished (D203), the Redshift dialect
finished (D204), `ListHas` and `LikeFold` in the root package and the Cassandra
filter that ignores case (D202).

v0.9.0 holds the sections of `\d+` (D199), `Binding.Keep` for the Cassandra
filters (D200) and the third group of data for usql's describe commands (D201).

v0.8.0 holds the NULL fields (D197) and the first fields for tables, indexes,
columns and functions (D198), which usql needs for its describe commands.

v0.7.0 holds the Cassandra rename (D196), the URL dsn and the ordinary user of
the Cassandra and ScyllaDB entries (D195), and the release read for every user
(D191, D192).

v0.6.0 holds the GizmoSQL (D187) and Avatica (D186) dialects, the first run of
Snowflake against a trial account (D190), the OpenSearch 2.19.6 columns (D189),
and the decisions that leave Pinot and DynamoDB without a model (D185, D184)
and drop H2, VoltDB, chai and csvq (D188). dbrun connects a hosted service
with the driver DSN.

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

- The dialects are built or decided for everything that usql supports, except
  H2, VoltDB, chai, csvq, Pinot, DynamoDB, Phoenix and the hosted services
  other than Redshift and Snowflake (D184, D185, D186, D188, D194).
- Redshift's Spectrum tables cannot be measured, because the namespace has no
  IAM role.
- The describe commands of usql are at the level of psql 18 for PostgreSQL. The
  next work is what usql finds against the new releases, and the other
  databases, one at a time, which usql said will follow.
- Known and not done is in `BACKLOG.md`.

## Waiting

- Redshift Spectrum waits for an IAM role on the namespace, which Ken owns.
- usql sends what it finds against the new fields, and the next database to
  bring to psql's level.
- The arrow-go handshake that GizmoSQL needs is fixed upstream, and until then
  the tests open the session themselves.
