# D240. The MaxCompute fixture stays up because the views lag

Status: Amends D237.

## The decision

The views of INFORMATION_SCHEMA do not show a change at once. The documentation
says that the metadata views lag the project by about three hours, and that the
views TASKS_HISTORY and TUNNELS_HISTORY lag by the same time. Measured on
2026-10-29, the project had the package at 00:43 UTC, a table at 00:50 and the
whole fixture at 01:10. Every view was empty at 02:01 and every one of those
objects was there at 02:03. So a new object showed after about an hour in that
run, and the documentation allows more.

If a run built the fixture and dropped it, the next run waits that long, and
the drop shows for as long. So the fixture stays up. The setup runs every
step again and a step that finds its object there is not an error. A run that
finds the last table of the fixture, `ticket`, in the views builds nothing. A run
that does not find it builds the fixture and waits up to the time in
`DBMETA_MAXCOMPUTE_WAIT`, three minutes by default, and then fails with a message
that says to run the tests again later. `DBMETA_MAXCOMPUTE_DROP` makes the run drop
the fixture when it ends.

The tests need more than ten minutes, because each statement is a job of one to
seven seconds and the endpoint resets some connections. Run them with
`GOFLAGS=-timeout=60m`. D239 says why the tests retry a reset.

## What the reader needs

The parity test connects as the reader. The service refuses the reader every
statement until the role reader has CreateInstance on the project. The
documentation says that it also needs read on the package for the views. Both are grants:

    `grant createinstance on project dbmeta to role reader;`
    `grant read on package Information_Schema.systables to role reader;`

The session that wrote the model did not make them, because a grant to a role is a
change of the account and it asked nobody. Ken asked the session that sets up the
account to make them, and it did on 2026-10-29. The service has no SQL grant for a
role and its API cannot edit one, so that session deleted the role and made it again
with a longer policy. `DBMETA_UPDATE=1` then recorded the reader in
`test/testdata/parity.txt`.
