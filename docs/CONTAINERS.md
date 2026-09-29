# Adding a Container or a Machine

This document tells you how to add a server that `dbrun` can start: a new
release of a product that is already listed, a new product, an image that this
repository builds, or a virtual machine. [`DBRUN.md`](DBRUN.md) tells you how
to use `dbrun` once the entry exists, and its rules for a shared machine apply
to every step here.

A server that `dbrun` starts is data in the Go package `container`. It is the
only list of the releases that `dbrun` starts, whether dbmeta is tested
against them or they are Staged. `dbrun` and the CI
workflow both read it, and tests fail when either one drifts from it (D42,
D69). The package starts nothing and imports no container client.

Adding a server is one step of adding a database. [`DIALECT.md`](DIALECT.md)
holds the rest, and its step 4 points here. Another repository that needs a
server, such as `dbimp` or a driver, gets its entry here too, through a dbmeta
session or through Ken. Such an entry is Staged while no model reads it, so
dbmeta's CI does not run it, and the other repository runs it with
`dbrun test staged` or by name (D119).

## Rules

1. Add the entry to `container/` before anything else that needs the server.
   Nothing else starts one (D68).
2. A new product needs Ken's agreement first. D66 holds the order for a dbmeta
   model, and dbimp's order holds it for a server that one of its drivers
   needs (D112, D113). D90 says which products qualify.
3. Choose the releases with [`EVALUATION.md`](EVALUATION.md), and record the
   floor, the ceiling and which step decided in the doc comment of the file.
4. Name every release in one of the tiers `Tested`, `Nightly`, `Verified` or
   `Staged`. CI runs the first two, and a person runs the third before a
   release. A release that no model reads is `Staged`, and CI never runs it.
   Add it with `staged` rather than `add`, and give it the cadence it would
   have if a model read it, `Tested`, `Nightly` or `Verified`. dbimp runs its
   Staged releases by that cadence, and it becomes the tier when the model
   arrives (D120).
   `TestAReleaseIsStagedExactlyWhenNoModelReadsIt` holds that a release is
   Staged exactly when no model reads it (D119).
5. Publish one port, the one the tests connect to. If a client needs a second
   interface of the same server, set `second` to its port inside the
   container. dbrun publishes it on the first host port plus 1000, and the
   URL computes it with `container.SecondHostPort`. QuestDB does this for its
   PostgreSQL interface, and Pinot for its controller (D124). `dsn --json`
   prints the address in the `secondAddress` field. Any other interface, such as Couchbase
   Analytics, is not reachable from the host.
6. Use `container.Password` for the administrator. Use another password only
   when the product cannot take one at start, and say why in the file.
7. Give the server `container.MemoryLimit`, which is 4 GB. Ask for more only
   after you measured the product needing it, and write the measurement beside
   the number, as SAP HANA does.
8. Write a readiness check that passes only when a test can connect and log
   in as the user the tests use.
9. Make the setup, `Init`, safe to run twice, because `start` runs it every
   time. Check before you create. `start` also runs it again after a failure,
   up to three times (D105).
10. Pin an image by its digest as well as its tag when somebody other than the
    vendor built it (D88).
11. Read an image without starting a container. Use
    `podman image inspect <image>` for its entrypoint, ports and environment,
    and `podman unshare sh -c 'podman image mount <image>'` for its files.
12. Before you finish, start the server, stop it, start it again, and remove
    it. A second start is a different path from the first. It found two
    faults in the Couchbase entry (D96).
13. Check the manifest format of an image that somebody other than the
    vendor pushed. CI runs docker, and the Docker on GitHub's runners refuses
    a Docker image manifest of schema 1, which podman still pulls. Such an
    image is copied into `docker.io/usql` with
    `podman push --format v2s2`, which Ken does, because it needs his login
    (D100).

## A new release of a product

1. Find the tag on the registry and check that the image is still rebuilt,
   with the command in step 2 of `EVALUATION.md`. If the product's images are
   never rebuilt, apply the rule under step 2 for such a product instead
   (D112).
2. Add the release to the list of its product in `container/<product>.go`,
   with its tier. `list.add` takes the tier and the releases, and `list.on`
   changes one release where the product differs from itself.
3. If the release moves the floor or the ceiling, update the doc comment of
   the file and the row in `EVALUATION.md`.
4. If `README.md` has a tier table for the product, add the release to it.
   `TestTheReadmeTierTablesMatchTheList` fails until you do.
5. Run the checks under Check it.

Do not edit the CI workflow. Its matrix comes from the list (D69).

A release added to the list moves the host port of every server after it,
because a port is 55000 plus the place of the server in `container.All`. A
container that another session built before your change keeps its old port.
`dbrun start` rebuilds it and `dbrun status` reports it, so tell the person
that the ports moved.

## A new product

1. Ask Ken whether the product is next.
2. Choose its releases with `EVALUATION.md`, and add a row to the table
   Databases evaluated so far. `TestEveryProductIsEvaluated` fails until the
   row exists.
3. If dburl names a scheme for the product, add a constant to the `Dialect`
   block in `dialect.go`, with the dburl driver name, if dbmeta has none yet.
   If the server is for a dbimp driver whose name dbimp has not settled,
   leave `dialect` empty. `dbrun` then names the test variable for the
   product, such as `DBMETA_ARANGODB` (D112).
4. Read the image, as rule 11 says. Find the port the tests use, the
   administrator, how a password reaches it, the settings the tests need, and
   the tools inside it that a readiness check or a setup can run.
5. Write `container/<product>.go`. It holds a doc comment, the `product`
   value, and the list of releases. The field reference below says what each
   field is for, and `container/couchbase.go` is a short example with a setup.
6. Add the list to `All` in `container/container.go`, at the end, so that no
   running server gets a new port.
7. If the product has no shell to run a readiness check, leave `ready` empty.
   `dbrun` then connects from the host through the driver in the `drivers`
   map in `test/cmd/dbrun/test.go`, which needs an entry for the dialect and
   a dbmeta model, because it runs the version query. A product with no model
   needs a `ready` command.
8. Add the administrator to the credentials table in `DBRUN.md`. If the
   setup, the start command or the image creates an ordinary user, declare it
   in `users`, so that `dsn --json` prints it. `container/couchbase.go` does
   this.
9. Run the checks under Check it.
10. Write a decision in `decisions/` for anything you chose rather than found,
    such as a setting you turned on or a user you created.

## An image this repository builds

Build an image only when the published one cannot do what the tests need and
no setting or argument can change it. Cassandra's image refuses a user
defined function, a materialized view and a role, and Oracle publishes no free
19c image, so both are built. D118 added more: an image with no shell, such as
the Cloud Spanner emulator's, a product that publishes a jar and no image,
such as H2 and Fuseki, and two programs that must run in one container, such
as PostgREST with PostgreSQL and ksqlDB with Kafka.

1. Write `test/cmd/dbrun/image/<product>.Containerfile`. It takes the release
   as the build argument `RELEASE`. Check the result in the build itself, as
   the Cassandra file does, because a `sed` that matches nothing changes
   nothing and reports nothing.
2. Name the image `localhost/dbmeta/<product>` in the entry. `dbrun` embeds
   every file in that directory and builds the one named for the product,
   so nothing else changes. `TestEveryContainerfileHasItsImage` fails when a
   file and its entry disagree. Oracle 19c keeps `localhost/oracle/database`,
   the name Oracle's build script gives it, and has its own build.
3. Keep the password out of the Containerfile. Package the program there,
   and configure it in the entry's command, where `container.Password` is.

`start` and `test` build a missing image. `dbrun build <name>` builds it again,
which is what you run after you change the file.

## A virtual machine

A machine is a `container.Machine`. `dbrun` provisions it once and keeps it,
because building it takes an hour or more. Its tier is always `Verified`,
because CI cannot run one. It has a fixed port of its own, and its screen
viewer has a second.

For a SQL Server release that has no Linux image, 2008 R2 to 2016, follow
[`WINDOWS.md`](WINDOWS.md). The entry is a `WindowsSpec` in
`container/windows.go`, and `dbrun provision <name>` builds it (D57).

For a vendor's appliance, such as the Exasol Community Edition, the entry is an
`ApplianceSpec` in the product's own file. It names the file the vendor
publishes, its SHA-256, the page to download it from, the disks in it, and
its memory and processors. Nothing here can download it, so a person does, and
`dbrun provision <name> --from <file>` checks and imports it (D85, D86).

Name every Verified release in `COVERAGE.md`.
`TestEveryVerifiedReleaseIsDocumented` fails until you do.

## An embedded database

An embedded database has no entry in `container/`, because it has no server
(D42). A model that declares itself embedded gets a `dbrun` entry by itself.
If the database has no model yet, add it to `unmodeled` in
`test/cmd/dbrun/target.go`, with its dialect in `dialect.go` as dburl names
it, and remove it from there when its model is written (D116). If it starts
from sample files, as csvq does, put them in `test/cmd/dbrun/sample/<name>/`.

## Field reference

The fields of `product` in `container/container.go`, which `list.add` copies
into each release:

| Field | What it holds |
| --- | --- |
| `dialect` | the dbmeta dialect, which is the dburl name. It is empty until dbimp settles it, and `dbrun` then names the test variable for the product (D112) |
| `also` | every other dialect the server answers. dbrun sets the test variable of each to the same DSN (D114) |
| `name` | the product, the first half of every server name |
| `image` | the image without its tag, with the registry written out, such as `docker.io/library/postgres` |
| `major` | turns a release into the name a person uses. Only Oracle needs it. |
| `tagPrefix` | text before the release in the tag, such as `v` for SurrealDB |
| `tagSuffix` | text after the release in the tag, such as `-latest` for SQL Server |
| `port` | the port inside the container that the tests connect to |
| `env` | the environment the image needs, such as its password |
| `ready` | a command, run inside the container, that exits 0 once the server answers |
| `init` | a command, run inside the container on every start after `ready` passes |
| `initInput` | text sent to `init` on its standard input, for an image with no shell, such as SurrealDB's |
| `runFlags` | flags for the run command, before the image name, such as `--entrypoint` where Apache Pinot, Apache Druid and Avatica replace the image's entrypoint, or `--add-host` where Databend blocks its telemetry (D112) |
| `args` | arguments after the image name, for the image's entrypoint or the one `runFlags` names |
| `memory` | a memory limit above `MemoryLimit`, with its measurement |
| `startup` | a wait longer than 90 seconds, with its measurement |
| `settle` | how long `ready` has to keep passing. Presto and Trino need it (D83). |
| `dsn` | builds the connection string the Go driver takes, for a host port |
| `url` | builds the dburl URL, where it differs from the DSN |
| `users` | the ordinary users that the setup, the start command or the image creates, as `Principal` values with their own DSN, which `dsn --json` prints after the administrator (D102) |

## Readiness checks and setups that went wrong

Each of these was a real fault. The right form is what the entry does now.

| Wrong | What happened | Right |
| --- | --- | --- |
| Couchbase's check asked `/pools` for a 200 | An initialized cluster answers 401 without credentials, so a server started a second time never became ready. | Accept 200 or 401, which says the cluster manager answers. |
| Vertica's check logged in as `dbadmin` on the local socket | It passed before the entrypoint created the user the tests use, and the first connection was refused. | Connect over TCP as that user. |
| Cassandra's check connected without a password | With `PasswordAuthenticator` the server accepts a connection before it can check one. | Log in. |
| Couchbase's setup waited for `SELECT 1` | On a second start the query service answered before the bucket was warm, and 7.2 refused an INSERT. | Wait for a statement that reads the bucket. |
| ScyllaDB's salted password was passed as it is | The entrypoint writes its arguments into a file that a shell reads, and the shell took each `$` as a variable. | Escape each `$` once. |
| SAP HANA ran with `--ulimit nofile=1048576:1048576` | Rootless podman refuses a limit above the host's hard limit, and the container already had it. | No flag. |
| ScyllaDB 2026.3 was expected to create `cassandra` | It creates no default superuser. | Name the superuser at start with `--auth-superuser-name`. |
| SurrealDB stored its data in `/data` | The image runs as a user that cannot write `/data`, and RocksDB refused to start. | Store it in `/tmp`, in the container's own layer, which keeps it across a stop. |
| SurrealDB's setup was going to run `surreal sql` | It exits 0 when a statement fails, so a failed setup reads as a success. | Run `surreal import` on `/dev/stdin`, which exits 1. |

## Check it

Run the tests of the list from the root of the repository, and then the rest
from the `test` directory:

```bash
go test ./container/...
cd test
go run ./cmd/dbrun list <product> --releases
go run ./cmd/dbrun start <name>
go run ./cmd/dbrun stop <name>
go run ./cmd/dbrun start <name>
go run ./cmd/dbrun test <name>
go run ./cmd/dbrun remove <name>
```

Then run the checks in `AGENTS.md` under Before you commit.

## The tests that catch a skipped step

| Test | Fails when |
| --- | --- |
| `TestEveryProductIsEvaluated` | a product has no row in `EVALUATION.md` |
| `TestWorkflowReadsTheList` | the workflow stopped reading the list |
| `TestEveryServerNamesItsPrincipals` | a server does not name its administrator, or an ordinary user's DSN does not hold that user's name |
| `TestWorkflowImagesAreQualified` | an image in the workflow has no registry written out |
| `TestWorkflowImagesAreInTheList` | the workflow names an image that the list does not |
| `TestTheReadmeNamesEveryTier` | the list has a tier that `README.md` does not explain |
| `TestTheReadmeTierTablesMatchTheList` | `README.md` puts a release in a tier the list does not |
| `TestEveryVerifiedReleaseIsDocumented` | a Verified release is not named in `COVERAGE.md` |
| `TestEveryMachineIsUsable` | a machine shares a name or a port with another server, lacks its dialect, product or release, or is not Verified |
| `TestAReleaseIsStagedExactlyWhenNoModelReadsIt` | a release that no model reads is not Staged, or one that a model reads is (D119) |
| `TestEveryContainerfileHasItsImage` | a Containerfile and the entry that names its image disagree (D118) |
| `TestEveryWindowsMachineIsUsable` | a Windows machine is missing what its install needs |
