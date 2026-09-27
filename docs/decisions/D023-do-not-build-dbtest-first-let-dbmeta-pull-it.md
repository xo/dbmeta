# D23. Do not build dbtest first. Let dbmeta pull it into existence

Status: Decided.

Both reviews were asked whether the container package `dbtest` must come first,
and both said no. Build `dbmeta`, and grow `dbtest` only when a real `dbmeta`
test cannot be written without it.

The reason both gave is the same. Infrastructure written before it has a
consumer gets the wrong shape. The existing shell scripts in `usql/contrib`
already work and already cover 27 databases, so nothing is blocked today.

## The split

1. `dbtest` starts as a thin wrapper over the `podman` command, a few hundred
   lines at most. It starts a container, waits for the database to accept a
   connection, returns a connection string, and stops the container.
2. `dbmeta` generates and tests one driver at one version against it.
3. `dbtest` then gains the one thing the shell scripts cannot do, which is two
   versions of the same database at once. That forces unique container names
   and dynamic host ports.
4. `dbmeta` adds the version matrix for that driver.
5. Repeat per driver. Extract a stable `dbtest` after three drivers, not
   before.

Wait on everything else: volumes, networks, log streaming, a Podman REST
client, cross platform support, and the 20 databases that `dbmeta` does not
cover yet.

## How to tell the abstraction went wrong

Watch for these. Any one of them means stop and simplify.

1. `dbtest` is larger than `dbmeta`.
2. `dbtest` exposes a general `RunContainer(image, env, ports)` API. The right
   shape is `dbtest.StartPostgres(ctx, version)` returning a connection.
3. `dbmeta` maps a container port to build its own connection string.
4. Adding a database means changing the core of `dbtest`.
5. An interface in `dbtest` has exactly one implementation.

## What this harness needs that a general one does not

Both reviews produced nearly the same list, and every item maps onto a decision
already in this file.

1. Two or more versions of one database at once. This is the version axis of
   D8 and the shell scripts cannot do it.
2. A pinned image digest, not a tag. A tag moves, and generated code must be
   reproducible. Record which digest produced which model. This is the gap D12
   already names.
3. A readiness check. A container reports itself started well before the
   database accepts a connection, and a generator that connects too early
   crashes.
4. A seeded schema that exercises the metadata surface: types, arrays, enums,
   domains, constraints, comments, indexes, partitions, generated columns and
   extensions. A general harness gives an empty database, and an empty database
   tests nothing.
5. Named users with fixed grants. Introspection returns different answers to
   different users, which the testing plan already requires and which two of
   the five open `usql` pull requests exist because of.

## The third party question is still Ken's

The two reviews split on whether to grant D7 an exception for a test only
container library.

Gemini said grant it and use `testcontainers-go`, on the grounds that D7
protects consumers of the library and a test harness is never compiled into a
consumer's binary.

DeepSeek said keep the shell scripts and add a thin wrapper over the `podman`
command, and grant an exception only if the ban is meant for production
dependencies alone.

The thin wrapper is the smaller commitment and it matches D23 above, because it
is what step 1 describes either way. Note one fact in favor of it: the reviews
also observed that `testcontainers-go` reaches Podman through a Docker
compatibility socket, which is an extra moving part for a project that has
already chosen Podman.
