# D248. Oracle 19c starts on a copy of a kept database

Status: Decided.

The first start of `oracle-19c` runs DBCA, which builds the whole database
and takes a quarter of an hour. Removing the container threw that work away,
so every new container paid it again. Ken asked for a way to keep the database
and copy it, because a copy on btrfs costs almost nothing.

## Decision

`container.Server.Seed` names the path inside the container that holds the
database. `oracle-19c` sets it to `/opt/oracle/oradata`, and no other release
sets it. `dbrun` does the rest:

1. If no golden copy exists, `dbrun start` starts the container on an empty
   directory, waits for the line `DATABASE IS READY TO USE!` in its log, stops
   it with a 180 second grace period, and keeps the directory. This happens
   once.
2. Every other start makes a copy of the golden directory with
   `cp -a --reflink=auto` and mounts the copy over `/opt/oracle/oradata`.
3. `dbrun remove` deletes the container and its copy. `dbrun remove --seed`
   also deletes the golden copy, which the next start builds again.

The directories are `$XDG_DATA_HOME/dbmeta/seed/oracle-19c/golden` and `live`,
and `DBMETA_SEED_STATE` moves them.

4. A container runs on a smaller image than the one that builds the database.
   Oracle's build makes a 7.18 GB image, and DBCA needs the JDK and the
   assistants in it. A container that starts on a kept database needs none of
   that. `container.Server.RunImage` names the smaller image,
   `localhost/dbmeta/oracle:19.3.0-slim`, and `dbrun` builds it from the first
   one with `image/oracle19/slim.Containerfile` and `--squash-all`. It is
   2.28 GB.

## Why this works

The entrypoint, `/opt/oracle/scripts/base/runOracle.sh`, skips DBCA when the
file `/opt/oracle/oradata/.ORCLCDB.created` and the directory
`/opt/oracle/oradata/ORCLCDB` both exist. It then links the `spfile`, the
password file and the network files from `oradata/dbconfig/ORCLCDB`. So a
copy of the whole directory is a complete database. The image declares no
volume and the entrypoint keeps nothing else.

## Facts that decided the details

- The golden copy must be stopped cleanly. A copy of a database that was killed
  makes every container run instance recovery when it starts.
- The wait is on the log line and not on the readiness check. The check passes
  while the entrypoint is still moving the `spfile` into `oradata/dbconfig`.
  A copy taken then has no `spfile`.
- The files belong to a user id that only the user namespace of podman maps.
  So the copy and the delete run as `podman unshare`, which keeps every owner.
  Only the empty golden directory takes the `:U` mount option, because it is
  made by the user and the user in the image cannot write it. A copy keeps the
  owner of its source.
- `--reflink=auto` shares the data on btrfs and copies it on any other file
  system. The result is right on both and only fast on btrfs.
- Docker does not seed. It has no user namespace to run the copy in, so a
  docker runner builds the database in the container as before. CI never runs
  19c, which is Verified.

## Measured

- The first build took 13 minutes 45 seconds on a development machine. The
  golden copy is 3.7 GB.
- `dbrun remove` and then `dbrun start` took 30 seconds. The root test package
  passed on the copy in 69 seconds, and the version query answered, so the new
  host name of each container is no problem.
- The first build ran out of memory under the usual 4g, in the step that
  creates the instance, and the kernel killed DBCA. `oracle-19c` now takes 8g.
- The second build failed in the step that creates the pluggable database, with
  ORA-65169 on a file copy and ORA-12801 from a parallel query server. The file
  system is mounted with `compress=zstd:3`. The third build, with nothing
  changed, passed. The cause is not known, so a failed build keeps its
  container as `oracle-19c-seedfail` for its logs and its alert log.

## The smaller image

Ken set the bar at about 2 GB and said that 2.28 GB is fine. It keeps the
Oracle home that the server reads and removes the rest: the other products in
the home, the static libraries, the math and backup libraries, and the spatial,
text and OLAP directories. The database still holds those components as
dictionary objects, and nothing here reads them.

- The root test package passes on it, against the database the full image
  built, and so does `dbrun test oracle-19c`.
- The Java virtual machine stays. A cut that also removed `javavm` and `jlib`
  made the image 1.5 GB. It was not tried against the tests, so it is not used.
- `libedtn19.a` stays, because the entrypoint reads it to learn the edition and
  refuses to start without it.
- The database needs 8g. Its SGA is 6 GB, because DBCA sized it from the memory
  of the host, and a container limited to 4g kills the log writer.
- The host has a limit of 65536 asynchronous I/O slots, `fs.aio-max-nr`. Eight
  Oracle databases at once used 65295 of them, and the next one failed with
  ORA-27090. Stop an idle Oracle before you run another.

## Consequences

- The password stays the one the golden copy was built with, which is
  `container.Password`. `ORACLE_PWD` has no effect on a copy.
- `dbrun build oracle-19c` builds the full image and then the smaller one. It
  does not change the golden copy. Run `dbrun remove --seed oracle-19c` after
  it to build the database again.
- A container that was created before this change has its database inside
  itself and keeps it. Only a container that `dbrun` creates again uses a copy.
