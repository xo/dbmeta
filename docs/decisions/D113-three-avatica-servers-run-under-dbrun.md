# D113. Three Avatica servers run under dbrun for dbimp's driver

Status: Amended by D119 and D155.

dbimp asked on 2026-09-28 for `dbrun` entries for its Avatica driver, which
Ken placed ninth in dbimp's order (dbimp D74). Avatica is the wire protocol of
Apache Calcite, JDBC calls over HTTP in protobuf or JSON, and several products
speak it. dbimp tests one driver against each of them. Ken agreed the same day
to the three that dbimp named. dbmeta has no model for any of them, and none
has a dialect yet, because dbimp settles the name in its step 9 (D112).

## The entries

| Product | Releases | User | Notes |
| --- | --- | --- | --- |
| Avatica | 1.28.0, 1.29.0 | none | The standalone server over HSQLDB in memory, which the Calcite project builds. It speaks protobuf. The entrypoint of 1.28.0 and 1.27.0 runs `/usr/bin/java`, which those images lack, so the entry names `/opt/java/openjdk/bin/java` itself |
| Apache Phoenix | 2.0-5.0 | none | The Phoenix Query Server on HBase 2.0 and Phoenix 5.0, from `boostport/hbase-phoenix-all-in-one`. It checks no user without Kerberos |
| Apache Druid | 36.0.0, 37.0.0 | `admin`, and `dbmeta_user`, who can read every datasource | Every service in one container, with the smallest configuration and the basic security extension. The Avatica endpoint is on the Router at `/druid/v2/sql/avatica-protobuf/` |

Every release is Tested. Each was started fresh, measured with a query through
`apache/calcite-avatica-go`, the Go driver usql uses today, stopped and started
again, and removed.

## Phoenix is an exception to step 2

The Apache Phoenix project publishes no image. The only image that runs
ZooKeeper, HBase, Phoenix and the Query Server in one container is
`boostport/hbase-phoenix-all-in-one`, which the Go driver's own tests use. It
was last pushed on 2023-03-14, so step 2 of `EVALUATION.md` does not admit it.
Ken made it an exception on 2026-09-28. It answered in 20 seconds and used
1.7 GB. A table made through it kept its row across a stop and a start.

## Druid in one container

The Druid image runs one service in each container. Its scripts that run them
all together need Python or Perl, and the image is Debian distroless with
busybox, which has neither. So the start command runs ZooKeeper and the five
services with bash, each in the background, with the nano quickstart
configuration. It answered in 13 seconds and used 2.6 GB on 37.0.0 and 2.7 GB
on 36.0.0, inside the 4 GB limit. The ordinary user reads through SQL and
through Avatica with basic authentication, a wrong password gets 401, and the
user is refused the security API with 403.

## The standalone server has no user

The Avatica server checks no user of its own, and passes each connection's
user and password to HSQLDB. A user that SA made through the server, with
requests built by hand in protobuf, was not found by a second connection. So
the entry makes none, and `BACKLOG.md` holds the next attempt, with the Go
driver instead of hand built requests.
