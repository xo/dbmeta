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

Open points:

- Not done: InfluxDB 3 has no `api` field, because its DSN is the driver URL
  and nothing sets one. D167 says every product with an HTTP interface has
  one, which is true only where the DSN was `http://` or was set by hand.

## Waiting

- Nothing waits for Ken.
