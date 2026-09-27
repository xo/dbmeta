# D105. dbrun runs a setup again after a failure, and prints the log of a server that never answered

Status: Amended by D107.

Ken reported timeouts for `oracle-21c` and `hive-4.2.1` on 2026-09-27. The
push run of 323cd36 failed on both. In the 28 runs before it, `oracle-21c`
did not fail and `hive-4.2.1` failed once.

## Oracle 21c was stuck, not slow

`oracle-21c` answers in 30 to 50 seconds on a GitHub runner, measured on six
runs that passed. The run that failed said only "it never answered in 5m0s".
That is a stuck database and not a slow one, so a longer budget does not help.
The error did not carry the container's log, so the cause was not recorded.

When a container never answers, `dbrun` now ends the error with the last 20
lines of its log. The next failure says why.

## Hive failed in its setup

Both Hive failures were in the setup, after HiveServer2 first answered. One
was a SerDeException, `proto.class has to be set`. The other was a
ParseException in `hive-schema-*.hive.sql`, which is the same script on every
run. Three fresh starts on a development machine, which has 32 cores,
succeeded. A GitHub runner has 4. So a server that has just begun to answer
can still refuse a statement on a slow machine.

Every setup already runs on every start, and CONTAINERS.md requires each one
to be safe to run twice. So when a setup fails, `dbrun` prints what it said,
waits 10 seconds and runs it again, up to three times. A setup that fails
three times is reported with the output of the last attempt. The first two
failures stay in the log of the run, so a fault that a retry hides is still
recorded.

A settle, as D83 gave Presto and Trino, was rejected. A settle waits for the
readiness check to keep passing, and Hive's readiness check passed. It was a
DDL statement that failed.
