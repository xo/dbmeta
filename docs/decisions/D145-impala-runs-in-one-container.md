# D145. Impala runs in one container

Status: Amends D118.

## The decision

D118 gave Apache Impala no entry. Ken asked on 2026-09-30 for dbmeta to
answer for Impala, because usql deletes its own readers in W21, and chose
to build one image from Apache's images, because dbrun starts one container
for a server and Apache's quickstart runs four.

`test/cmd/dbrun/image/impala.Containerfile` builds on
`apache/impala:<release>-impalad_coord_exec`, which already holds
statestored and catalogd, because they are the impalad binary under other
names, and Java 8. It takes the Hive metastore and Hadoop from
`<release>-impala_quickstart_hms`. One script starts the metastore on Derby,
waits for its port, starts the statestore and the catalog, and runs the
daemon. 4.4.1 and 4.5.2 start, answer on HiveServer2's port 21050, and
create and list a table, measured on 2026-09-30.

## What had to change

- Impala refuses the local filesystem as the default filesystem and aborts
  on it as a configuration error. The tables are files on the container's
  own filesystem, as the quickstart keeps them on a volume, so the catalog
  and the daemon run with -abort_on_config_error=false.
- Each JVM is held to 512 MB and the daemon's query memory to 1 GB, for
  dbmeta's limit of 4 GB.

## The model

Impala has no catalog a SELECT can read. Ken chose that a query can walk
SHOW statements for it, and D146 holds that. `models/impala` reads 4.4.1
and 4.5.2, which are Tested.
