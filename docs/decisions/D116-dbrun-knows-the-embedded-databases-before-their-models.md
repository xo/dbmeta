# D116. dbrun knows the embedded databases before their models

Status: Amended by D119 and D142.

Ken asked on 2026-09-28 for `dbrun` to know the other embedded databases,
before a dialect or a model is written for any of them. An embedded database
runs in the process that opens it and has no server, as SQLite and DuckDB do.
Ken chose four, from the embedded drivers `usql` ships: chai, csvq, ql and
moderncsqlite. H2 is left out, because its driver talks to an H2 server, so
it would be a container.

## The entries

`dbrun` made an embedded entry for each model that declares itself embedded.
None of the four has a model, so `dbrun` holds a short list of them, and an
entry leaves that list when its model is written.

| Entry | Dialect | Storage | URL |
| --- | --- | --- | --- |
| chai | `chai` | a directory | `chai:<dir>` |
| csvq | `csvq` | a directory of CSV files | `csvq:<dir>` |
| ql | `ql` | a file, `ql.ql` | `ql:<file>` |
| moderncsqlite | `sqlite3` | a file, `moderncsqlite.db` | `moderncsqlite:<file>` |

Each dialect is the one dburl names for the scheme, as Databend's was in
D112, and `dialect.go` gains `Chai`, `CSVQ` and `QL`. dburl gives
moderncsqlite the dialect `sqlite3`, because it is SQLite without cgo. Ken
chose that it has an entry and a file of its own beside the sqlite3 entry.
Its test variable is `DBMETA_MODERNCSQLITE`, so that it does not take
`DBMETA_SQLITE3`.

Each file or directory is under `DBMETA_EMBEDDED_STATE`. A directory is
removed with what it holds, and `test` makes a directory before it runs.

## csvq starts from sample files

csvq reads a directory of CSV files as its tables, so an empty directory is a
database with nothing in it. Ken asked for sample files that are extracted to
the directory. `dbrun` embeds four, the core tables of D53: `author.csv`,
`book.csv`, `region.csv` and `shipment.csv`, with a few rows that agree with
each other. `start`, `test` and `usql` write each one that is missing, and
keep a file that is there already, so that a test that changed one keeps its
change. `TestCsvqStartsFromTheSamples` holds that.

## What was measured

On 2026-09-28, through a `usql` built with the four drivers: a table was made,
written and read on chai, ql and moderncsqlite, and csvq joined `book` to
`author` across two of the samples. chai refuses a table with no primary key.
ql writes two hidden files beside its file. Each entry passed `dbrun test`,
and CI runs the four in the job that starts nothing.
