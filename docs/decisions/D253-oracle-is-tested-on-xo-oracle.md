# D253. Oracle is tested on xo/oracle

Status: Amends D154 and D157, amended by D255.

dburl v0.55.0 names `github.com/xo/oracle` as the driver of the oracle scheme,
where it named `github.com/sijms/go-ora/v3` before. D154 says that the test
driver of a dialect is the one that dburl names, so the test module moved to
it. The maintainers decided this on 2026-11-03.

## The decision

The test module requires `github.com/xo/oracle` at v0.1.1, and it no longer
requires `go-ora`. D157 pinned go-ora at a commit because the fix for its
panic on 11g and 18c had no tag. That reason is gone, because the new driver
is the one that dburl names and it has a tag.

The driver name for `sql.Open` is still `oracle`. Two things change:

- The URL options use the names of pgx. The Autonomous Database Free image
  needs TLS without a check of the certificate, so its URL ends in
  `sslmode=require`, where it ended in `ssl=true&ssl+verify=false`.
- The driver reports the name of every type, and it scans NUMBER and FLOAT
  into a decimal type. The alignment function of the model reads the name
  first, and it keeps the Go type rule for go-ora, which reports no name.

## What was measured

The whole test module passed on Oracle 26ai in 478 seconds and on Oracle 11g
in 261 seconds. The alignment test passed on 19c.

The whole suite on 19c did not run to the end. The maintainers stopped it on 2026-11-03,
because the tables statement below made the scan test last more than 30
minutes. The suites of the other releases show that the driver works, and the
slow statement is the same on both drivers.

On 19c, the statement that reads tables takes 77 seconds on the server, and
the old driver and the new one take the same time, 76.5 and 77.0 seconds. The
server is busy on its CPU while it runs. The same statement takes under a
second on 26ai. The cause is not the driver, and it is not found. The
statement is in `docs/BACKLOG.md`.
