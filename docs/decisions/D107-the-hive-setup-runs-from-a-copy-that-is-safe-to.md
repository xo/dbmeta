# D107. The Hive setup runs from a copy that is safe to run twice

Status: Amends D105.

D105 runs a failed setup again, and it said that every setup was already safe
to run twice. Hive's was not. The next push run, of 1335428, failed on
`hive-4.2.1` with the retry in place. The first attempt failed on the
SerDeException that D105 records. The second and third failed on "Table
hive.SYS.HIVE_LOCKS already exists", because the first attempt had created
that table.

`hive-schema-4.2.0.hive.sql` makes every table with `CREATE ... IF NOT
EXISTS` except one, `CREATE EXTERNAL TABLE HIVE_LOCKS`. beeline stops at the
first error, so a second run of the script failed there and never reached the
rest. That also broke a plain stop and start of a Hive server, measured on
2026-09-27. CI never starts a server twice, so nothing found it. The comment
in `container/hive.go` said that every statement was safe, and it was wrong.

The setup now copies the script with `sed`, adds `IF NOT EXISTS` to each
`CREATE TABLE` and `CREATE EXTERNAL TABLE` that lacks it, and runs the copy.
The copy ran twice in one container with exit status 0 both times. A fresh
start, a stop, a second start and `dbrun test` passed three times in a row.

The SerDeException, "proto.class has to be set", is still not explained. It
comes from one of the `PROTO_` tables at the end of the script, which dbmeta
does not read. Now that the script is safe to run twice, the retry of D105 can
get past it.
