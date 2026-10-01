# Progress

This file records where the work stands, so that a session that ends or
crashes can resume. Update it when a piece of work starts or ends. Work that
is known and not done goes in [`BACKLOG.md`](BACKLOG.md), and a decision goes
in [`decisions/`](decisions/README.md).

## Where the work stands

On 2026-10-02 `main` and `origin/main` are at f8a6338, and CI passed on it.

That commit holds D167, which makes the DSN what `sql.Open` takes and adds
the `api` field, D168, which is Ken's review of the six dialects of
2026-10-01, and DuckDB's DollarQuotes. dbimp has the commit hash and changes
its docs/DRIVER.md to read `api`.

Staged and not committed:

- D169. `Query.Each` takes an `Args` and passes only what the query takes,
  `Args` gains `AccessMethod`, `Server` and `Database`, `Dialect.Pattern`
  turns a psql pattern into a schema pattern and a name pattern, and
  `dbmeta.Open` reads the version and calls `New`. usql asked for all three
  so that its metadata layer stays thin.
- `dbrun start` prints the URL that usql takes after the test variable.

## Waiting

- Nothing waits for Ken.
