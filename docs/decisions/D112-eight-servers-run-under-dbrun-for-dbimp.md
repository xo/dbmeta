# D112. Eight more servers run under dbrun for dbimp's drivers

Status: Amends D109, amended by D114.

Ken asked on 2026-09-27 for `dbrun` entries for ArangoDB, Databend, InfluxDB,
CrateDB, TDengine, Apache Pinot, rqlite and libSQL. All eight are drivers that
dbimp plans, in the order of dbimp's D73. dbmeta has no model for any of them,
and the entries are for dbimp's tests, as the SurrealDB and Neo4j entries are
(D103, D106). Four agents read the registries and the images, and each entry
was then measured through `dbrun`: a fresh start, the ordinary user, a stop and
a second start. None was started by hand.

## The dialects wait for dbimp

dbimp settles the name and the URL of each driver in its own step 9, and Ken
accepts each one (dbimp D26, D28 and D30). So the dialect of seven of the
eight is empty, and `dbrun` names the test variable for the product, as
`DBMETA_ARANGODB`. D109 removed that fallback when Neo4j got its name, and
this decision brings it back. Databend is the exception. dburl already has
the scheme `databend` and usql has the driver `databend-go`, so its dialect is
`databend` and its DSN is the `databend://` URL that driver takes. The others
have a plain `http://` DSN until their names are settled.

## An image that is never rebuilt

Step 2 of `EVALUATION.md` takes the floor from the oldest release whose image
is still rebuilt. Databend, rqlite, libSQL, TDengine and Apache Pinot build
each tag once, on the day of the release, and never again. Read strictly, the
rule allows only the newest release of each. Ken chose on 2026-09-27 that the
floor of such a product is the newest release of the line before the newest,
and the ceiling is the newest release. A product with one line, libSQL, has one
release. `EVALUATION.md` holds the rule under step 2.

## Databend's telemetry is blocked

From v1.2.881, Databend sends a report to `telemetry.databend.com` at every
start and stop, with the operating system, the processors and the memory of
the machine. Only a paid Enterprise key turns it off. Ken chose on 2026-09-27
that the container resolves that host to 127.0.0.1, with `--add-host`, so the
report goes nowhere. The server starts and answers with it.

## The entries

| Product | Releases | Ordinary user | Notes |
| --- | --- | --- | --- |
| ArangoDB | 3.12.12 | `dbmeta_user`, read and write on the database `dbmeta`, nothing on `_system` | Only 3.12 is built. ArangoDB reads the memory of the host, so the entry tells it 4 GB. The telemetry option in the documents is obsolete in 3.12.12 |
| InfluxDB | 3.9.13, 3.11.5, and 3.10.6 nightly | none | InfluxDB 3 Core has admin tokens and no users. A fixed token is written to a file before the server starts |
| CrateDB | 6.3.7, 6.4.5 | `dbmeta_user`, DQL, DML and DDL on the schema `dbmeta` | `crate` takes no password. Host based authentication is on, so the ordinary user's password is checked |
| TDengine | 3.3.8.8, 3.4.2.8 | `dbmeta_user`, SYSINFO 0, who makes `dbmeta` itself | The Community Edition has no GRANT. The ordinary user can make and drop users, measured on both releases |
| Apache Pinot | 1.4.0, 1.5.1 | `dbmeta_user`, who may query `baseballStats` | The Quickstart runs every part in one container. The broker checks the users. The entry is the first to replace the image's entrypoint |
| Databend | 1.2.881, 1.2.948-nightly | `dbmeta_user`, every privilege on `dbmeta` through a role | Telemetry blocked. The weekly release moves almost every day |
| rqlite | 9.4.5, 10.3.6 | `dbmeta_user`, query and execute only | The users are in a file that the start command writes |
| libSQL | 0.24.33 | none | Basic authentication has one user. A lesser one needs a signed JWT and a key pair |

Every release in the table is Tested except InfluxDB 3.10.6, which is
Nightly. dbimp asked for the floor and the ceiling of each on every push.

## What went wrong on the way

Three faults were found by measuring, and each is fixed:

- The first ArangoDB entry passed `--server.telemetrics-api false`, which
  3.12.12 logs as obsolete.
- The first Databend setup put a statement with single quotes inside a body
  in single quotes, so the shell split it and every attempt failed. The
  statement is now escaped for the shell.
- The first Pinot file was written through a shell heredoc that ended early,
  because the Go source held its own `EOF` line.

One fault is not fixed. The first start of Databend after the pull of the
image did not answer: its bootstrap script started the query server before
the metadata server was the leader. It did not happen again in four fresh
starts. `BACKLOG.md` holds it.
