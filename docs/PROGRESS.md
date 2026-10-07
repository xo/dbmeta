# Progress

This file records where the work stands, so that a session that ends or
crashes can resume. Update it when a piece of work starts or ends. Work that
is known and not done goes in [`BACKLOG.md`](BACKLOG.md), and a decision goes
in [`decisions/`](decisions/README.md).

## Where the work stands

dbmeta has two releases. v0.1.0 was tagged on dd70b7a on 2026-10-02, and
v0.2.0 on 39b74b2 on 2026-10-07. The Tested tier passed in CI on 39b74b2.
Ken chose that CI stands in for a run of every tier, and the Verified tier was
not run again for either release.

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

In progress:

- A daily check of the nightly workflow runs at 18:07 local time in this
  session, and expires after 7 days, on 2026-10-14. It watches the InfluxDB 1
  fix.

## Waiting

- Nothing waits for Ken.
