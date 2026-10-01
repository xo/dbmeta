# Progress

This file records where the work stands, so that a session that ends or
crashes can resume. Update it when a piece of work starts or ends. Work that
is known and not done goes in [`BACKLOG.md`](BACKLOG.md), and a decision goes
in [`decisions/`](decisions/README.md).

## Where the work stands

On 2026-10-02 `main` and `origin/main` are at 724e64e, and CI passed on
3a0ff0f, the commit before it.

Staged and not committed:

- D167. The DSN is what `sql.Open` takes. `container.Server.ConnectURL` is
  gone. The DSN of libSQL, Neo4j, ArangoDB, SurrealDB and InfluxDB 1 and 2 is
  now their URL, and a new `api` field, on the server and on each principal,
  holds the `http://` address that dbimp's tools read. dbimp agreed to the
  name and changes its docs/DRIVER.md when this lands.
- D168, Ken's review of the six dialects of 2026-10-01. ArangoDB makes a
  database the schema and answers 7 kinds. SurrealDB's Schemas lists only the
  database of the connection.
- Measured live on 2026-10-02: neo4j-2026.09.0, arangodb-3.12.12 and
  surrealdb-3.3.0 pass. The parity and conformance files were recorded again
  for ArangoDB, which only sorted their sections.

- DuckDB's Syntax sets DollarQuotes, at usql's request. DuckDB 1.5.5 reads
  `$$x;y$$` and `$tag$a;b$tag$` as strings, measured on 2026-10-02.

## Waiting

- Nothing waits for Ken.
- When this is pushed, send dbimp the commit hash for the `api` field.

