# D82. CI compiles once and every job runs the binary

Status: Decided.

One job builds `dbrun` and the test binary and uploads them. Every job in
both release matrices downloads those two and compiles nothing. Ken asked
whether it was possible to build `dbrun` once, and the answer is that it is,
along with the thing that costs four times as much.

## What it cost before

Measured on the nightly run of 2026-09-26, job `mariadb-11.4`, which is an
ordinary one rather than the worst:

| | |
| --- | --- |
| the step | 103 seconds |
| building `dbrun`, and starting the server | 47 seconds |
| compiling the tests | 55 seconds |
| running the tests | 1.4 seconds |

Every job downloaded 63 modules and compiled them. There were 23 jobs.

The test module links every driver dbmeta tests against, and two of them are
cgo: `mattn/go-sqlite3` builds the SQLite source and `duckdb/duckdb-go` links
a prebuilt library of about a hundred megabytes. That is the whole cost and it
is paid once per job for a binary that is identical in all of them.

## Why the cache did not do this already

`actions/setup-go` caches and the cache was hitting. It was 33 megabytes,
which is the root module, because the key is the hash of the root `go.sum`
and the root module has no dependencies at all. Whichever job saved first
saved the smallest possible cache, and a key cannot be written twice, so no
later job was able to replace it with a useful one.

A key that also covers `test/go.sum` helps, and it is not enough. A restored
build cache still relinks, and the link of a 121 megabyte binary is not free.
Building once and shipping the result skips the question.

## How

`DBMETA_TEST_BINARY` names a test binary built by `go test -c`. When it is
set, `dbrun test` runs that binary rather than `go test`. Nothing sets it for
a person, so `dbrun test postgres` still compiles what they just changed,
which is what they want.

This keeps D68 intact. CI still runs `dbrun test`, which is the same entry
point and the same code path, so CI cannot start a container a way nobody
else does. What changed is which binary runs the tests, not who starts the
server.

The matrix jobs no longer set up Go at all. They still check out, because the
tests read `testdata/` at run time, and they `chmod +x` what they download,
because an artifact does not carry the mode bit.

`TestTheMatrixJobsCompileNothing` fails when a matrix job runs `go run` or
`go test`, or does not set `DBMETA_TEST_BINARY`. That test exists because
this is invisible from a passing run: a job that compiles gives the right
answer and costs ninety seconds to give it.

## What this does not fix

The artifact is 121 megabytes and about 46 compressed, downloaded once per
job. That is real and it is far less than what it replaces.

Wall clock is not 23 times better. The jobs already ran in parallel, so the
run was bounded by the slowest job rather than by the sum, and SAP HANA at
five minutes is mostly a server starting. What this recovers is the ninety
seconds inside every job, the runner minutes behind them, and the second wave
when the matrix is wider than the concurrency limit.
