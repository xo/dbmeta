# The Test Runner

This is the design for the command that starts the databases dbmeta is tested
against. It is `test/run.sh` today and it is becoming a Go program.

Read it before changing how a database is started. The decisions themselves
live in `PLAN.md`: D68 says every container is started by this command and
named `<product>-<release>`, and D69 says the CI workflow builds its matrix
from the same list.

## What is wrong with it today

`run.sh` grew a subcommand at a time and it shows.

Run it bare and it starts every release of every database and runs the whole
test suite against each, which is half an hour of containers and is almost
never what somebody wanted. That is a destructive default for a command people
now type to look at one thing.

`vms` is a separate mode with its own loop, its own readiness wait and its own
output, so a Windows machine is not a server you can `start`, `stop` or ask
the `version` of. It is a different noun reached a different way, and the only
reason is that it was added later.

A selector is a tier, a product or an exact name. There is no way to say "the
newest PostgreSQL", which is what somebody debugging wants nine times in ten,
so they type `postgres-18` and it goes stale the month 19 ships.

SQLite and DuckDB are not in it at all. They have no server, so they fall out
of a command organised around starting one, and `./run.sh test sqlite3` does
nothing.

And it is 300 lines of bash doing arrays, string splitting on a unit
separator, and a JSON encoder by hand. The unit separator is there because an
argument can contain a space, which is the kind of thing a shell makes hard
and a language makes free.

## The model

Three kinds of thing, one name space.

A **container server** is a database in a container: PostgreSQL, MariaDB,
MySQL, SQL Server from 2017, Oracle, Cassandra, ClickHouse. It is described by
`container.Server` and started, stopped and removed by the container runner.

A **machine server** is a database on a Windows virtual machine: SQL Server
2008 R2 through 2016, which Microsoft never shipped on Linux. It is described
by `container.WindowsVM`. It is a container too, of a sort, because
`dockurr/windows` runs it, but it is provisioned once over an hour and then
kept, rather than created fresh each time.

An **embedded database** is a library with no server at all: SQLite and DuckDB.
There is nothing to start. It is in the list so that `test sqlite3` works and
so that `status` can say what it is rather than leaving somebody wondering
why the name is missing.

They share one name space and it already fits: a machine is
`sqlserver-2016` and a container is `sqlserver-2017`, because Microsoft's Linux
images begin at 2017 and nothing overlaps. Nothing had to be renamed to make
this work, which is a sign the naming was right.

## Names

Every server is `<product>-<release>`. `postgres-18`, `clickhouse-26.9`,
`oracle-26ai`, `sqlserver-2016`, `cassandra-5.0`. An embedded database is just
`<product>`: `sqlite3`, `duckdb`.

The name is the container name, the selector and what every message calls it.
There is one string and no translation table.

## Selectors

| You type | You get |
| --- | --- |
| `postgres-18` | that release |
| `postgres` | the newest PostgreSQL, and only that one |
| `postgres --all` | every PostgreSQL release |
| `--all` | every release of every product |
| `tested` | the tier CI runs on every push |
| `nightly` | the tier CI runs at night |
| nothing | the help |

A bare product meaning the newest release is the change that matters. It is
what somebody wants when they are looking at something, it does not go stale,
and it keeps one server on the machine instead of six.

`--all` after a product widens it to every release of that product. `--all`
alone widens to everything. The word means the same thing in both, which is
"do not narrow this for me".

Bare is the help. A command that starts twenty containers and runs for half an
hour should be asked for in as many words.

## Subcommands

| | |
| --- | --- |
| `start` | start it, leave it running, print its URL |
| `stop` | stop it, keeping it so `start` resumes it |
| `remove` | stop and delete it |
| `status` | what is running, with a URL for each |
| `version` | connect and print what dbmeta reads from it |
| `dsn` | print the URL, running or not |
| `usql` | connect to it with usql |
| `test` | run the integration tests against it |
| `provision` | build a Windows machine, which takes an hour |
| `help` | this |

`test` is what bare used to do and now has to be asked for. `test --all` is
the old bare behaviour and is what a person runs before a release.

`test sqlite3` and `test duckdb` run the tests with no server, because those
two need none. Every other subcommand answers for them too: `status` says they
are embedded, `version` reports what the driver links, and `start` says there
is nothing to start rather than failing.

## How each kind answers each subcommand

| | container | machine | embedded |
| --- | --- | --- | --- |
| `start` | create or start | start the provisioned machine | nothing to do, says so |
| `stop` | stop | stop | nothing to do |
| `remove` | delete | delete, and warn that it is an hour to rebuild | nothing to do |
| `status` | running, with URL | running, with URL and the viewer port | says embedded |
| `version` | connect and read | connect and read | read what the driver links |
| `test` | start, wait, test, remove | start, wait, test, keep | test |
| `provision` | not applicable | build it | not applicable |

Two differences are real and stay. A machine is kept after `test`, because
rebuilding it is an hour, where a container is removed because rebuilding it
is a minute. And a machine waits on a query rather than on a port, because
Windows boots long before SQL Server listens.

## Why Go

The list of servers is already Go, in `container`. The shell reads it through
`tool/servers`, which encodes each command as fields separated by U+001F
because an argument can contain a space, and `run.sh` reads them back into
arrays. That whole layer exists to carry a `[]string` across a language
boundary, and it disappears when there is no boundary.

`version` already shells out to a second Go program, `tool/version`, because
it needs a driver. `test` shells out to `go test`. So the shell is already a
launcher for Go, in a language where quoting a command is a thing you can get
wrong.

It also gets a real `--help`, real flags, and one place to put the readiness
loop that each of the three kinds currently writes for itself.

It stays in the `test` module, because `version` opens a connection, which
means a driver, which hard rule 1 keeps out of the root module.

## Where it lives

`test/tool/db`, run as `go run ./tool/db` from `test`, with a thin
`test/run.sh` that execs it so the name people and CI already type keeps
working. `tool/servers` and `tool/vms` fold into it, because their only
consumer is the shell that is going away. `tool/version` folds in too.

## The open question: provisioning

`test/vm/provision.sh` builds a Windows machine. It downloads a SQL Server
installer of a few hundred megabytes, checks it, writes an OEM directory with
an unattended answer file, starts `dockurr/windows` with that directory
mounted, and waits up to an hour while Windows installs itself and then SQL
Server. It is 200 lines of shell and it works.

Three ways to go.

**Leave it a script and call it.** `db provision sqlserver-2016` execs
`vm/provision.sh 2016`. The surface is uniform and the implementation stays
where it is known to work. Two languages for one job, and a reader has to
follow a hop to see what happens.

**Port it to Go.** One language. It is a download, a digest check, a template
and a container run, none of which Go is bad at, and the digest check and the
retry logic are better in Go than in bash. It is also a rewrite of the one
part of this that is hard to test, because trying it costs an hour per
attempt.

**Leave it entirely separate.** `provision` is not a subcommand at all and
people run `vm/provision.sh`. Honest about it being a different job, and it
leaves the gap D68 was written to close: a thing you do to a server that is
not done through the one command.

The recommendation is the first. Fold the interface now and leave the
implementation alone, because the interface is what D68 is about and the
implementation is an hour per test cycle to change. Revisit if the Go tool
ends up needing the same download and verify logic for something else, which
it might: the Oracle 19c image is built from a downloaded archive by
`test/oracle/build-19c.sh` and has the same shape.

## What this does not change

The list of releases stays in `container/container.go` and stays the only
copy. D42 decided that, D69 kept it, and this is a different program reading
the same list.

The CI workflow keeps calling the runner rather than starting containers
itself, which is D69 and D68 together.

Nothing about what the tests do. This is how a database gets started, not
what is asked of it once it is up.
