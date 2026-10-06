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

## Waiting

- Nothing waits for Ken.
