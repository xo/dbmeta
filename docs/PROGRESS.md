# Progress

This file records where the work stands, so that a session that ends or
crashes can resume. Update it when a piece of work starts or ends. Work that
is known and not done goes in [`BACKLOG.md`](BACKLOG.md), and a decision goes
in [`decisions/`](decisions/README.md).

## Where the work stands

On 2026-10-02 dbmeta has its first release, v0.1.0, tagged on the commit
that holds this text. Ken chose v0.1.0, and chose that CI stands in for the
run of every tier before this release: the Tested tier passed on 380e597,
and the Nightly tier passed on 3a0ff0f. The Verified tier was not run again
for it.

The last three commits:

- 380e597 holds D169, which adds `Query.Each`, `Dialect.Pattern` and
  `dbmeta.Open` for usql, and makes `dbrun start` print the URL that usql
  takes. usql has the hash and builds on it.
- f8a6338 holds D167, which makes the DSN what `sql.Open` takes and adds the
  `api` field, D168, which is Ken's review of the six dialects of
  2026-10-01, and DuckDB's DollarQuotes. dbimp has the hash.
- 724e64e holds D166: Oracle 19c, Pinot and the InfluxDB release wait.

## Waiting

- Nothing waits for Ken.
