# dbrun

`dbrun` starts, stops and removes the databases that dbmeta is tested against.
It is the only thing that starts one (D68). This document tells you how to use
it and how to share the machine with the other people and coding agents that
use it at the same time.

Read the rules first. Then find your task under Common tasks, and use the
reference sections below them when you need a detail. To add a release, a
product or a virtual machine to what `dbrun` can start, read
[`CONTAINERS.md`](CONTAINERS.md) instead. The reasons behind the design are in
`PLAN.md`: D68, D69, D75, D82, D86 and D97.

Run every command from the `test` directory of a dbmeta checkout:

```bash
cd test && go run ./cmd/dbrun help
```

To run it from anywhere, build it once with
`go build -o ~/bin/dbrun ./cmd/dbrun`. The examples below write `dbrun` for
either form.

## Rules for a shared machine

Several sessions use this machine at once, and each database is a real
container that holds memory. These rules keep one session from breaking the
work of another. They apply to a person and to a coding agent alike.

1. Start a database only with `dbrun`. Never run `podman run` or `docker run`
   yourself, not even a short `--rm` container to read a file from an image.
   To read a file in an image, run
   `podman unshare sh -c 'podman image mount <image>'`, which starts nothing.
2. Run `dbrun status` before you start anything, so that you know what is
   already running.
3. Start only the releases your task tests, by their exact names, such as
   `postgres-18`. Use a tier or `all` only when the task is a release run.
4. Keep the machine at four running servers or fewer. The four include the
   servers of every session. `dbrun` keeps the limit for containers: a start
   that brings the count to five stops your own oldest server, and refuses
   when none of the four is yours. A virtual machine is not counted by
   `dbrun`, so count it yourself.
5. Never stop, remove or restart a server that another owner started.
   `dbrun` refuses to, and `--force` overrides the refusal. Pass `--force`
   only when the person tells you to. Treat a server with no owner as
   somebody else's too, although `dbrun` lets you stop it (D98).
6. When your task ends, leave the machine as you found it. Stop or remove each
   server that you started. Leave running each server that was running before
   you began. If you stopped one of those by mistake, start it again.
7. Never run a command that acts on every container, such as
   `podman stop --all`, `podman rm --all` or `podman system prune`.
8. Tell the person which servers you left running when you finish.

A server that dbmeta's own tests need is started for them by `dbrun test`,
and removed again at the end unless you pass `--keep`. The next section says
who owns a server and what that changes.

## Owners and sharing

Every server that `dbrun` creates carries its owner, as the container label
`dbmeta.owner` (D98). The owner is, in this order:

1. `DBMETA_OWNER`, when it is set.
2. The session of a coding agent, from `CLAUDE_CODE_SESSION_ID`, written
   `claude-code:<session>`.
3. The login name of a person, written `user:<name>`.

The owner changes what a command does:

| When the server belongs to | `start` | `stop`, `remove` | `test` |
| --- | --- | --- | --- |
| you | starts or shares it | acts | removes it at the end, unless `--keep` |
| another owner, running | shares it, and says whose it is | refuses | shares it, and keeps it |
| another owner, stopped | refuses | refuses | refuses |
| nobody | starts or shares it | acts | as for your own |

A start that needs room stops only your own oldest server. It never stops a
server of another owner or a server with no owner, unless you pass `--force`.

A server has no owner when it was created before owners existed. A label
cannot be added to a container that exists, and rebuilding one loses what is
in it, so it keeps no owner until you remove it and start it again.

Two sessions that test one release share one server. The first session's
`start` creates it, and the second session's `start` finds it running and
uses it. Only the owner stops it.

`status` shows the owner of each running server, as `(yours)`, the owner's
name, or `(no owner)`.

## Common tasks

Start a server, keep it running, and connect to it with `usql`:

```bash
dbrun start postgres-18
dbrun usql postgres-18
```

Get the connection string for a program or a test. `dsn --json` prints one
object for each server, and the `dsn` field is the form the Go driver takes:

```bash
dbrun dsn --json postgres-18 | jq -r '.[0].dsn'
```

Run dbmeta's integration tests against one release. `test` starts the server,
waits for it, runs the tests and removes the server:

```bash
dbrun test clickhouse-25.8
```

Run the tests and keep the server for a closer look afterwards:

```bash
dbrun test clickhouse-25.8 --keep
```

See every release of a product and its tier, running or not:

```bash
dbrun list couchbase --releases
```

Stop a server and keep it, so that the next `start` resumes it in seconds. Or
remove it, which deletes the container:

```bash
dbrun stop postgres-18
dbrun remove postgres-18
```

Read what a server said while it started:

```bash
dbrun logs postgres-18
```

## Commands

| Command | What it does | Changes anything |
| --- | --- | --- |
| `start` | Starts the server, waits until it answers, runs its setup, and prints its connection string. The server stays running. | yes |
| `stop` | Stops the server and keeps the container, so that `start` resumes it. | yes |
| `remove` | Stops the server and deletes the container. A machine asks first, because it takes an hour to rebuild. | yes |
| `status` | Prints each running server, its URL and its owner. For a machine, it prints whether the database answers yet. | no |
| `version` | Connects and prints the version that dbmeta reads. It needs a dbmeta model for the product. | no |
| `dsn` | Prints the URL of the server, running or not. | no |
| `usql` | Runs the `usql` on your `PATH` with the URL of the server, and nothing else. | no |
| `test` | Runs dbmeta's integration tests against the server, starting and removing it around them. | yes |
| `logs` | Prints what the server wrote. `-f` follows it. | no |
| `list` | Prints what a selector names, and touches nothing. | no |
| `build` | Builds the images this repository makes, which are for Cassandra and Oracle 19c. `start` and `test` build one when it is missing. | yes |
| `provision` | Builds a Windows machine, which takes about an hour, or imports a vendor appliance with `--from`. | yes |
| `help` | Prints the help. Running `dbrun` with no command does the same. | no |

## Selectors

A command acts on the servers that its selectors name. Every command except
`status` and `version` needs at least one.

| You type | You get |
| --- | --- |
| `postgres-18` | that release |
| `postgres` | the newest PostgreSQL release, and only that one. `dbrun` prints which release it picked. |
| `postgres --releases` | every PostgreSQL release |
| `tested` | every release that CI runs on each push |
| `nightly` | every release that CI runs once a night |
| `verified` | every release that a person runs before a release |
| `all` | every release of every product |
| `sqlite3`, `duckdb` | an embedded database, which is a file and not a server |

## Names, ports and credentials

A server is named `<product>-<release>`, such as `clickhouse-26.9` or
`couchbase-8.0.3`. The name is the container name, the selector and what every
message calls it.

The host port of a container is 55000 plus the place of the release in
`container.All`. It stays the same from one start to the next. Do not write a
port into a script or a test. Read it from `dsn --json`, because a release
added earlier in the list moves every port after it. When that happens,
`start` rebuilds a container whose port no longer matches, and `status` says
so. A virtual machine has a fixed port of its own, named in
`container/machine.go` and `container/windows.go`.

The connection string holds the administrator of each product. Most use the
password in `container.Password`:

| Product | Administrator | Password |
| --- | --- | --- |
| PostgreSQL | `postgres` | `container.Password` |
| MariaDB, MySQL | `root` | `container.Password` |
| SQL Server | `sa` | `container.Password` |
| Oracle | `system` | `container.Password` |
| ClickHouse | `default` | `container.Password` |
| Firebird | `SYSDBA` | `container.Password` |
| SAP HANA | `SYSTEM` | `container.Password` |
| Exasol | `sys` | `exasol`, the vendor default, on the nano image and on the Community Edition machine |
| Vertica | `dbmeta`, a pseudo superuser | `container.Password` |
| Couchbase | `Administrator` | `container.Password` |
| Cassandra, ScyllaDB | `cassandra` | `cassandra` |
| Apache Hive | `hive` | none. The image configures no authentication. |
| Trino, Presto | `trino`, `presto` | none |

One product also has an ordinary user that its setup creates. On Couchbase it
is `container.CouchbaseUser`, which is `dbmeta_user`, with
`container.Password` (D96). Every other ordinary user is created by the test
that needs it and dropped when that test ends. `dsn` prints only the
administrator's connection string.

## Output

Every command prints for a person by default. `list`, `dsn`, `status` and
`version` also print JSON with `--json`, and `--names` shortens the JSON of
`list` and `dsn` to a list of names, which is what a CI matrix takes.

The JSON is a list of objects, one for each server that the selector names,
with these fields:

| Field | What it holds |
| --- | --- |
| `name` | the server name, such as `couchbase-8.0.3` |
| `product` | the product, such as `couchbase` |
| `release` | the release, such as `8.0.3`. An embedded database has none. |
| `kind` | `container`, `machine` or `embedded` |
| `tier` | `tested`, `nightly` or `verified` |
| `dialect` | the dbmeta dialect, which is the dburl driver name, such as `n1ql` |
| `env` | the environment variable that dbmeta's tests read the DSN from, such as `DBMETA_N1QL` |
| `dsn` | the connection string that the Go driver takes |
| `url` | the dburl URL, which is what `usql` takes |
| `viewer` | for a machine, the port of its screen viewer |

The `dsn` and `url` fields can differ. The `dsn` field is what `sql.Open`
takes for the driver that dbmeta tests with, and the `url` field is what
`dburl` parses. On Couchbase the `dsn` is an `http://` address of the query
service and the `url` uses `couchbase://`.

A plain `dsn` prints the name and the URL on one line, separated by spaces. It
does not print the bare URL. Use `dsn --json` in a script.

`status --json` prints the running servers with every field above, and four
more:

| Field | What it holds |
| --- | --- |
| `state` | `running` for a container, `answering` or `starting` for a machine, `moved` for a container whose port no longer matches the list, and `embedded` for SQLite and DuckDB |
| `owner` | who started the server, and empty when it has no owner |
| `mine` | whether the owner is you |
| `started` | when the server last started, as the container runner writes it |

`version --json` prints one object for each running server, with `name`,
`display` for the line a person reads, and `versions`, which maps each key
the server reports to its version. The empty key is the main version. A
server whose version cannot be read has an `error` field instead, and the
command then exits 1.

## Exit status

`dbrun` exits 0 when every server it was asked about succeeded, and 1 for any
failure. It does not tell an unknown name apart from a server that did not
start. An error goes to standard error with the prefix `dbrun:`, and each
server that failed also prints a line on standard output that names it.

`start` exits 0 only after the server answered its readiness check and its
setup finished. `test` exits 1 when a test failed.

## What start guarantees

`start` returns after three things, in this order:

1. The container runs, or `dbrun` created it from the image, which it pulls or
   builds when it is missing.
2. The server answered. A container answers its readiness command, which runs
   inside it. A machine, and a container whose image has no shell, answers a
   connection from the host through the driver and the version query.
3. The setup of the product finished, if the product has one. The setup runs
   inside the container on every start. It creates the users or the catalog
   that the tests read, and it is safe to run twice.

A server waits 90 seconds by default. A product that needs longer names its
own time, such as SAP HANA at 108 seconds. `--timeout` sets another.

A server that answered is not always a server that is fully warm. Couchbase
updates its indexes after a write rather than with it, so a test that writes
and then reads asks for `scan_consistency=request_plus` (D96).

## Lifecycle

| | container | machine | embedded |
| --- | --- | --- | --- |
| `start` | creates or resumes it | starts the provisioned machine | prints where the file will be |
| `stop` | stops it and keeps it | stops it and keeps it | nothing to do |
| `remove` | deletes it | deletes it, after asking | deletes the file |
| after `test` | removed, unless `--keep` | kept, because it takes an hour to rebuild | the file is kept |

An embedded database is SQLite or DuckDB. It has no server. The file lives
under `$XDG_DATA_HOME/dbmeta/embedded` and stays after a test, so that
`dbrun usql sqlite3` opens what the test built.

## Environment variables

| Variable | What it does |
| --- | --- |
| `DBMETA_RUNNER` | `podman` or `docker`. Without it, `dbrun` uses podman, and docker when podman is absent. CI sets `docker`. |
| `DBMETA_OWNER` | Who you are, for the owner label of each server you create. Without it, `dbrun` uses the session of a coding agent, and then the login name. |
| `DBMETA_TEST_BINARY` | A test binary built with `go test -c`. `dbrun test` runs it instead of compiling the tests. CI sets it (D82). |
| `DBMETA_VM_STATE` | Where the disks of the virtual machines live. They are tens of gigabytes each. |
| `DBMETA_ORACLE_STATE` | Where the Oracle 19c checkout and installer archive live. |
| `DBMETA_EMBEDDED_STATE` | Where the SQLite and DuckDB files live. |
| `DBMETA_<DIALECT>` | The DSN that dbmeta's tests read, such as `DBMETA_N1QL`. `dbrun test` sets it, and a test skips when it is not set. |

## Using dbrun from another repository

Another repository, such as `usql`, `dbimp` or a driver, uses the same servers
in the same way. Check out dbmeta, run `dbrun` from its `test` directory, and
read the connection string from `dsn --json`. A CI workflow checks out a pinned
dbmeta commit from `main`. A server that dbmeta's list does not name yet gets
its entry in dbmeta first. [`CONTAINERS.md`](CONTAINERS.md) holds the steps,
and the work happens in a dbmeta session or goes to Ken.

`dbrun usql` runs the `usql` on your `PATH` and passes only the URL. To run a
`usql` that you built, or to pass it flags, read the URL yourself:

```bash
./usql -c 'select 1' "$(dbrun dsn --json postgres-18 | jq -r '.[0].url')"
```

usql's own container tests, in `drivers` and `drivers/metadata`, start their
own containers through dockertest. Those containers run outside `dbrun`, and
outside its limit of four.

## When something goes wrong

- A server never answered. Run `dbrun logs <name>` to read what it said. Then
  run `dbrun remove <name>` and `dbrun start <name>` to build it fresh. A
  container that did not start stays behind until you remove it.
- A server runs and every connection is refused. Run `dbrun status`. If it
  says the server runs on another port than the list asks for, run
  `dbrun start <name>`, which rebuilds the container.
- A start is refused because four servers run and none is yours. Stop one of
  your own servers, or ask the owner of one. `dbrun` names each server and its
  owner in the refusal.
- A `stop` or `remove` is refused because the server belongs to another owner.
  Leave it, and ask its owner.
- A server was stopped while you used it. Its owner stopped it, somebody
  passed `--force`, or it had no owner and somebody stopped it by name. Run
  `dbrun start <name>` to resume it, and tell the person.
- The setup failed. The error ends with the last lines of the setup's output.

## What dbrun does not start

- Products that are only a cloud service, such as Snowflake, BigQuery,
  Databricks, Athena and Spanner. There is no server to run.
- Products that dbmeta has not evaluated yet, such as CockroachDB, Databend,
  Flight SQL, H2, Impala, Netezza, VoltDB, YDB and Avatica.
  `EVALUATION.md` names the candidates, under Candidates carried over from
  usql.
- A product that needs two containers that work together, such as ksqlDB with
  Kafka. `dbrun` runs one container for each server.

## What dbrun does not do

Ken chose on 2026-09-27 to leave these as they are for now (D98):

- Every failure exits 1.
- A plain `dsn` prints two columns rather than the bare URL.
- `dsn` prints only the administrator, and one scheme for each product.
- A server cannot be pinned against being stopped for room. Only its owner
  stops it, which covers most of the need.
