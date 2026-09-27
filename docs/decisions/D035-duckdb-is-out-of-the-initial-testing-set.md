# D35. DuckDB is out of the initial testing set

Status: Superseded by D48.

Nothing in this decision is still in force. DuckDB is tested, it is reached
through `duckdb/duckdb-go` in the `test` module, and `models/duckdb` answers 20
of the 55. It runs in CI in the job that starts no container, beside SQLite,
because neither has a server. The design question this decision deferred was
answered by allowing cgo in the `test` module, not by finding a way around a
driver.

The rest is the record of what was decided before that.

DuckDB leaves the four databases that D24 puts in CI. It is not tested at the
start.

The reason is D29. `dbmeta` is pure Go, and DuckDB is the one primary database
with no pure Go driver, so it cannot be reached the way the others are.

It is not dropped. DuckDB is one of the eight base models in `usql`, so it has
to work. Its design is decided before any work starts on the models outside the
base tier, and that decision reopens the choice in D29: reach it through its
command line client, cover it by captured data alone, or something else.

Do not treat this as permission to skip it quietly. A base model that no test
touches is a gap, and the support table that `gen.go` writes must say so.
