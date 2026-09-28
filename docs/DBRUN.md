# dbrun

`dbrun` starts, stops and removes the databases this project uses: the ones
dbmeta is tested against, and the Staged ones that no model reads yet (D119).
It is the only thing that starts one (D68). This document tells you how to use
it and how to share the machine with the other people and coding agents that
use it at the same time.

Read the rules first. Then find your task under Common tasks, and use the
reference sections below them when you need a detail. To add a release, a
product or a virtual machine to what `dbrun` can start, read
[`CONTAINERS.md`](CONTAINERS.md) instead. The reasons behind the design are in
`decisions/`: D68, D69, D75, D82, D86, D97, D98, D105, D108, D115, D116, D117, D118,
D119, D120 and D122.

Run every command from the `test` directory of a dbmeta checkout:

```bash
cd test && go run ./cmd/dbrun help
```

To run it from anywhere, build it once with
`go build -o ~/bin/dbrun ./cmd/dbrun`. The examples below write `dbrun` for
either form.

The built program is faster. `go run` compiles dbrun on every command, which
takes a second or more, and that is more than a status of every target now
costs (D122). If you run many commands, build it once and run the program.
Build it again after you pull, because a program built earlier holds the list
of releases it was built with.

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
3. Test on the newest release of a product, unless the task names another
   release. `dbrun start <product>`, such as `dbrun start postgres`, starts
   the newest. Do not move to an older release because the newest is taken.
   If the newest runs for another owner, `dbrun` shares it. If it is stopped,
   `dbrun` creates it again as yours. Use a tier or `all` only when the task
   is a release run.
4. Keep the machine at eight running servers or fewer. The eight include the
   servers of every session. `dbrun` keeps the limit for containers: a start
   that brings the count to nine stops your own oldest server, and refuses
   when none of the eight is yours. A virtual machine is not counted by
   `dbrun`, so count it yourself.
5. Never stop or restart a running server that another owner started.
   `dbrun` refuses to, and `--force` overrides the refusal. Pass `--force`
   only when the person tells you to. Treat a running server with no owner as
   somebody else's too, although `dbrun` lets you stop it (D98). A stopped
   container belongs to nobody, because its owner may be a session that ended
   or a computer that restarted (D108).
6. When your task ends, leave the machine as you found it. Stop or remove each
   server that you started. Leave running each server that was running before
   you began. If you stopped one of those by mistake, start it again.
7. Never run a command that acts on every container, such as
   `podman stop --all`, `podman rm --all` or `podman system prune`.
8. Tell the person which servers you left running when you finish.
9. If you are a coding agent, set `DBMETA_OWNER_NAME` to the name of your
   session on every `dbrun` command, such as
   `DBMETA_OWNER_NAME=dbimp go run ./cmd/dbrun start neo4j`. `status` then
   shows your name beside your session, so the person can see which session
   owns which server. Do not set `DBMETA_OWNER` for this. The session is what
   tells two agents apart (D115).

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

`DBMETA_OWNER_NAME` adds a friendly name, such as `dbimp`, as a second label,
`dbmeta.owner.name`. It is only shown: `status` and every refusal print it
with the owner, as `dbimp (claude-code:05124815)`. The owner label alone
decides who may act on a server, so two sessions that give the same name are
still two owners (D115).

The owner changes what a command does:

| When the server belongs to | `start` | `stop`, `remove` | `test` |
| --- | --- | --- | --- |
| you | starts or shares it | acts | removes it at the end, unless `--keep` |
| another owner, running | shares it, and says whose it is | refuses | shares it, and keeps it |
| another owner, stopped container | creates it again as yours | `remove` acts | creates it again as yours, and removes it at the end |
| another owner, stopped machine | refuses | refuses | refuses |
| no owner | starts or shares it | acts | removes it at the end only if the test started it |

A start that needs room stops only your own oldest server. It never stops a
server of another owner or a server with no owner, unless you pass `--force`.

A server has no owner when it was created before owners existed. A label
cannot be added to a container that exists, and rebuilding one loses what is
in it, so it keeps no owner until you remove it and start it again.

A stopped container has no owner that counts. A session that ends leaves its
stopped servers behind, and so does a restart of the computer, and a refusal
on those sent agents to an older release. So a start of a stopped container
that another owner created removes it and creates it again, with you as its
owner. It loses what was in it, and the setup and the tests build that again.
A stopped machine keeps its owner, because it takes an hour to create (D108).

Two sessions that test one release share one server. The first session's
`start` creates it, and the second session's `start` finds it running and
uses it. Only the owner stops it.

`status` shows the owner of each running server, as `(yours)`, as
`name (owner)` when the owner gave a name, as the owner alone, or as
`(no owner)`. `status -a` shows a stopped server as `(stopped, <owner>)`, or
as `(stopped, nobody)` when it has no owner.

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
| `status` | Prints each running server, its URL and its owner. For a machine, it prints whether the database answers yet. `-a` or `--all` also prints each stopped server, marked `stopped`, with who made it, the way `podman ps -a` does. | no |
| `version` | Connects and prints the version that dbmeta reads. It needs a dbmeta model for the product. | no |
| `dsn` | Prints the URL of the server, running or not. | no |
| `usql` | Runs the `usql` on your `PATH` with the URL of the server, and nothing else. | no |
| `test` | Runs dbmeta's integration tests against the server, starting and removing it around them. | yes |
| `logs` | Prints what the server wrote. `-f` follows it. | no |
| `list` | Prints what a selector names, and touches nothing. The embedded libraries come first, then the servers, containers and machines together, then the hosted services, each by product and then by release, oldest first. | no |
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
| `staged` | every release that no model reads yet, which CI never runs (D119) |
| `all` | every release of every product |
| `sqlite3`, `duckdb`, `moderncsqlite`, `ql`, `chai`, `csvq` | an embedded database, which is a file or a directory and not a server |

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
| SurrealDB | `root` | `container.Password` |
| Neo4j | `neo4j` | `container.Password` |
| ArangoDB, TDengine | `root` | `container.Password` |
| Databend | `root` | `container.Password` |
| rqlite, libSQL, Apache Pinot | `admin` | `container.Password` |
| InfluxDB 1 | `admin` | `container.Password` |
| InfluxDB 2 and 3 | the admin token `_admin` | `container.InfluxDBToken`, which is `apiv3_` and `container.Password`. `/query` takes it as the password |
| CrateDB | `crate` | none. CrateDB takes no password for its superuser. |
| Apache Druid | `admin` | `container.Password` |
| CouchDB, QuestDB, TerminusDB | `admin` | `container.Password` |
| Qdrant, Weaviate, Meilisearch, Typesense | `admin`, a name for the administrator's key | `container.Password`, which is the key. None of the four has users, and each checks a key |
| Chroma | `admin` | none. The server of Chroma 1 checks nothing, and the name is checked by nothing. |
| CockroachDB, TiDB, YDB | `root` | `container.Password`, which Init sets |
| MongoDB | `admin`, in the database `admin` | `container.Password` |
| Elasticsearch | `elastic` | `container.Password` |
| Dgraph | `groot` | `container.Password`, which Init sets |
| GizmoSQL | `admin` | `container.Password` |
| Virtuoso | `dba` | `container.Password` |
| Milvus | `root` | `container.Password` |
| Alternator | `cassandra`, the access key | the salted hash of `cassandra`, which is the secret key |
| Spanner, BigQuery, Vitess | `admin`, `admin`, `root` | none. The two emulators and vttestserver check nothing, and the name is checked by nothing. `dbrun usql spanner` needs `SPANNER_EMULATOR_HOST` set to the published address, because dburl drops the host |
| OpenSearch | `admin` | `container.Password`. The first start uses a stronger one that the installer accepts, and replaces it before the server starts (D118) |
| DynamoDB, Cosmos | `dbmeta`, the account key | none checked. Cosmos has the key `container.CosmosKey`, which Microsoft publishes |
| Solr, Drill, Fuseki, ksqlDB, H2 | `admin`, and `sa` on H2 | `container.Password` |
| PostgREST | `dbmeta_admin`, a role a token names | a signed token, which the DSN carries as the password. There is no anonymous role |
| Stardog, GraphDB, VoltDB | `admin` | `container.Password`. Each appears only with its licence file |
| Avatica, Apache Phoenix | `SA`, `phoenix` | none. Neither checks a user, and the name is checked by nothing. |
| Cassandra, ScyllaDB | `cassandra` | `cassandra` |
| Apache Hive | `hive` | none. The image configures no authentication. |
| Trino, Presto | `trino`, `presto` | none |

Several products also have an ordinary user that their setup creates, each
named `dbmeta_user` with `container.Password`. On Couchbase it is
`container.CouchbaseUser` (D96). On SurrealDB it is `container.SurrealDBUser`,
a user on the database `dbmeta` in the namespace `dbmeta` (D103). On Neo4j it
is `container.Neo4jUser`, with the role `publisher`, and the setup also makes
the database `dbmeta` (D106). ArangoDB, CrateDB, Databend, TDengine, rqlite,
Apache Pinot and Apache Druid have one too, and D112 and D113 say what each
may do. InfluxDB 1 and 2
have `container.InfluxDBUser`, who may only read `dbmeta` (D114). InfluxDB 3
Core and libSQL have none, because neither can make a principal with fewer
rights than its administrator. CouchDB and TerminusDB have `dbmeta_user`,
who may read `dbmeta` and not change its design. Qdrant, Weaviate,
Meilisearch and Typesense have a key that may only read, which goes by the
name `dbmeta_user` in the DSN. CockroachDB and TiDB have `dbmeta_owner`,
who owns `dbmeta`, and `dbmeta_user`, who may only read it. MongoDB,
Elasticsearch and Dgraph have `dbmeta_user`, who may only read. YDB has
`dbmetauser`, because YDB allows no underscore in a user name. Virtuoso, Milvus, Alternator, OpenSearch, Solr, Drill, H2,
Fuseki, PostgREST, Stardog, GraphDB and VoltDB have `dbmeta_user`, who may
only read. QuestDB, Chroma, GizmoSQL, Spanner, BigQuery, Vitess, DynamoDB,
Cosmos and ksqlDB have none, and D118 says why. `dsn --json`
prints each in the `principals` field, after the administrator, with its own
connection string (D102). Every other ordinary user is created by the test
that needs it and dropped when that test ends.

## Licence files

Stardog, GraphDB and Volt Active Data do not start without a licence file
that a person downloads, and each vendor gives one only after a signup. A
coding agent never signs up or downloads one. Ken provisions each file, the
same way he provisions a hosted credential.

`dbrun` finds a product's file in the first of two places that has one:

1. The path in `DBMETA_<PRODUCT>_LICENSE`, such as `DBMETA_STARDOG_LICENSE`.
2. The file `<product>` in `$XDG_CONFIG_HOME/dbmeta/licenses`, such as
   `~/.config/dbmeta/licenses/stardog`.

`dbrun` lists the product's releases only while it finds the file, and mounts
the file read only where the product reads it. No model reads any of the
three, so every release of them is Staged, and CI has no file for any of them
either. See D118 and D119.

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
| `directory` | true for an embedded database that is a directory, chai and csvq (D116) |
| `tier` | `tested`, `nightly`, `verified` or `staged` |
| `cadence` | for a Staged release only, `tested`, `nightly` or `verified`: how often it would be tested if a model read it. A project that runs Staged releases, such as dbimp, runs the tested ones on each push and the nightly ones at night (D120) |
| `dialect` | the dbmeta dialect, which is the dburl driver name, such as `couchbase`. It is empty until dbimp settles the name, as for ArangoDB (D112) |
| `env` | the environment variable that dbmeta's tests read the DSN from, such as `DBMETA_COUCHBASE` |
| `also`, `alsoEnv` | every other dialect the server answers, and the variable of each, which dbrun sets to the same DSN. InfluxDB 3 answers `influxql` beside `influxdb` (D114) |
| `dsn` | the connection string that the Go driver takes |
| `url` | the dburl URL, which is what `usql` takes |
| `viewer` | for a machine, the port of its screen viewer |
| `principals` | every user a test reaches the server as, the administrator first. Each has `role`, which is `administrator` or `user`, `user`, `dsn` and `url` |

The `dsn` and `url` fields can differ. The `dsn` field is what `sql.Open`
takes for the driver that dbmeta tests with, and the `url` field is what
`dburl` parses. On MySQL and Cassandra the driver takes a form that is not a
URL. On Couchbase both are the same `couchbase://` URL. On SurrealDB and Neo4j
the `dsn` is the plain `http://` address, and the `url` names the database in
its path, such as `neo4j://neo4j:<password>@127.0.0.1:<port>/dbmeta` (D109).

A plain `dsn` prints the name and the URL on one line, separated by spaces. It
does not print the bare URL. Use `dsn --json` in a script.

`status --json` prints the running servers with every field above, and five
more. With `-a`, it prints the stopped servers too.

| Field | What it holds |
| --- | --- |
| `state` | `running` for a container, `answering` or `starting` for a machine, `moved` for a container whose port no longer matches the list, `stopped` for one that is stopped, which only `-a` prints, and `embedded` for an embedded database |
| `owner` | who started the server, and empty when it has no owner |
| `ownerName` | the friendly name the owner gave itself with `DBMETA_OWNER_NAME`, when it gave one |
| `mine` | whether the owner is you |
| `started` | when the server last started, in RFC 3339, as `inspect` writes it in JSON under podman and docker |

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
   that the tests read, and it is safe to run twice. If it fails, `dbrun`
   prints what it said and runs it again, up to three times, 10 seconds apart
   (D105).

A server waits 90 seconds by default. A product that needs longer names its
own time. SAP HANA, Oracle, Apache Hive, Vertica, Apache Pinot, Apache Phoenix
and Apache Druid wait 5 minutes. `--timeout` sets another.

A server that answered is not always a server that is fully warm. Couchbase
updates its indexes after a write rather than with it, so a test that writes
and then reads asks for `scan_consistency=request_plus` (D96).

## Lifecycle

| | container | machine | embedded | hosted |
| --- | --- | --- | --- | --- |
| `start` | creates or resumes it | starts the provisioned machine | prints where the file will be, and gives csvq its sample files | prints where the connection string came from |
| `stop` | stops it and keeps it | stops it and keeps it | nothing to do | nothing to do |
| `remove` | deletes it | deletes it, after asking | deletes the file | nothing to do |
| after `test` | removed, unless `--keep` | kept, because it takes an hour to rebuild, unless `--remove` | kept, unless `--remove` | nothing to keep |

An embedded database runs in the process that opens it and has no server.
SQLite and DuckDB have dbmeta models. moderncsqlite, ql, chai and csvq have
none yet, and `dbrun` knows them so that a test or `usql` can find them
(D116). moderncsqlite is SQLite without cgo, with a file of its own beside
the sqlite3 one. chai and csvq are a directory rather than a file.

The files live under `$XDG_DATA_HOME/dbmeta/embedded` and stay after a test,
so that `dbrun usql sqlite3` opens what the test built. csvq reads a
directory of CSV files as its tables, so `start`, `test` and `usql` put four
sample files in it: `author.csv`, `book.csv`, `region.csv` and
`shipment.csv`, the core tables of D53. A file that is there already is kept.
`dbrun remove csvq` deletes the directory, and the next command gives the
samples again.

## Hosted services

A hosted service runs somewhere else, such as Snowflake, BigQuery or Neon, and
`dbrun` reaches it with a connection string that you provision. `hosted/`
names every service `dbrun` knows, with the form of its connection string.
A service appears in `list`, `status` and every other command only while its
connection string resolves, and a person without an account never sees it
(D117).

`dbrun` reads the connection string from the first of these that has one:

1. The variable `DBMETA_<NAME>_DSN`, such as `DBMETA_SNOWFLAKE_DSN`.
2. The file `<name>` in `$XDG_CONFIG_HOME/dbmeta/credentials`. Make it
   readable by you alone, with `chmod 600`, or `dbrun` refuses it.
3. The program `dbmeta-credential-<name>` on your path, which prints the
   connection string. Write one to read a password manager.

If the driver reads a secret by itself, as BigQuery and Spanner read the key
file that `GOOGLE_APPLICATION_CREDENTIALS` names, the connection string holds
no secret.

`dsn` masks the secret, and `dsn --reveal` prints it. `usql` passes the
connection string in a temporary usql configuration file rather than on the
command line, so that no process list shows it. `test` sets `DBMETA_<NAME>`,
named for the service rather than its dialect. A hosted service that a
model reads, such as Neon, is Verified, and the rest are Staged, so CI never
runs one (D119). Cloud Spanner, DynamoDB, BigQuery and Cosmos
DB also have an emulator that runs as a container.

Do not put a connection string in a file in this repository, and do not paste
one into a command that others can see.

## Environment variables

| Variable | What it does |
| --- | --- |
| `DBMETA_RUNNER` | `podman` or `docker`. Without it, `dbrun` uses podman, and docker when podman is absent. CI sets `docker`. |
| `DBMETA_OWNER` | Who you are, for the owner label of each server you create. Without it, `dbrun` uses the session of a coding agent, and then the login name. |
| `DBMETA_<NAME>_DSN` | The connection string of a hosted service, such as `DBMETA_SNOWFLAKE_DSN`. The service appears only while it is set, or while its credential file or helper has one (D117). |
| `DBMETA_<PRODUCT>_LICENSE` | The path of the licence file of a product that needs one, such as `DBMETA_STARDOG_LICENSE`. The product appears only while it or `$XDG_CONFIG_HOME/dbmeta/licenses/<product>` names a file (D118). |
| `XDG_CONFIG_HOME` | Where `dbmeta/credentials` lives, the directory of credential files for the hosted services. It defaults to `~/.config`. |
| `DBMETA_OWNER_NAME` | The friendly name of your session, such as `dbimp`, which `status` shows beside the owner. A coding agent sets it on every command (D115). |
| `DBMETA_TEST_BINARY` | A test binary built with `go test -c`. `dbrun test` runs it instead of compiling the tests. CI sets it (D82). |
| `DBMETA_VM_STATE` | Where the disks of the virtual machines live. They are tens of gigabytes each. |
| `DBMETA_ORACLE_STATE` | Where the Oracle 19c checkout and installer archive live. |
| `DBMETA_EMBEDDED_STATE` | Where the files and directories of the embedded databases live. |
| `DBMETA_<DIALECT>` | The DSN that dbmeta's tests read, such as `DBMETA_COUCHBASE`. A server with no dialect yet uses its product, such as `DBMETA_ARANGODB` (D112), and moderncsqlite uses `DBMETA_MODERNCSQLITE` (D116). `dbrun test` sets it, and each `alsoEnv` variable too (D114). A test skips when it is not set. |

## Using dbrun from another repository

Another repository, such as `usql`, `dbimp` or a driver, uses the same servers
in the same way. Check out dbmeta, run `dbrun` from its `test` directory, and
read the connection string from `dsn --json`. A CI workflow checks out a pinned
dbmeta commit from `main`. A server that dbmeta's list does not name yet gets
its entry in dbmeta first. [`CONTAINERS.md`](CONTAINERS.md) holds the steps,
and the work happens in a dbmeta session or goes to Ken. A server that no
dbmeta model reads is Staged, so dbmeta's CI never runs it. Select it by
name, or with the selector `staged` (D119). To run the ones you need on each
push and the rest at night, filter `dbrun list --json staged` on `cadence`,
as dbimp does (D120).

`dbrun usql` runs the `usql` on your `PATH` and passes only the URL. To run a
`usql` that you built, or to pass it flags, read the URL yourself:

```bash
./usql -c 'select 1' "$(dbrun dsn --json postgres-18 | jq -r '.[0].url')"
```

usql's own container tests, in `drivers` and `drivers/metadata`, start their
own containers through dockertest. Those containers run outside `dbrun`, and
outside its limit of eight.

## When something goes wrong

- A server never answered. The error ends with the last 20 lines of the
  container's log. Run `dbrun logs <name>` to read all of it. Then run `dbrun remove <name>` and `dbrun start <name>` to build it fresh. A
  container that did not start stays behind until you remove it.
- A server runs and every connection is refused. Run `dbrun status`. If it
  says the server runs on another port than the list asks for, run
  `dbrun start <name>`, which rebuilds the container. If the server runs and
  belongs to another owner, `start` refuses. Ask its owner.
- A start is refused because eight servers run and none is yours. Stop one of
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
- Products that have no entry in `container/` yet, such as CockroachDB,
  Flight SQL, H2, Impala, Netezza, VoltDB and YDB.
  `EVALUATION.md` names the candidates, under Candidates carried over from
  usql.
- A product that needs two containers that work together, such as ksqlDB with
  Kafka. `dbrun` runs one container for each server.

## What dbrun does not do

Ken chose on 2026-09-27 to leave these as they are for now (D98):

- Every failure exits 1.
- A plain `dsn` prints two columns rather than the bare URL.
- `dsn` prints one scheme for each product.
- A server cannot be pinned against being stopped for room. Only its owner
  stops it, which covers most of the need.
