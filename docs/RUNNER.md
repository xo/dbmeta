# The Test Runner

This is the design of `dbrun`, the command that starts the databases dbmeta is
tested against. It lives in `test/cmd/dbrun` and it replaced a shell script.

Read it before changing how a database is started. The decisions themselves
live in `PLAN.md`: D68 says every container is started by this command and
named `<product>-<release>`, and D69 says the CI workflow builds its matrix
from the same list.

## What the shell got wrong

`run.sh` grew a subcommand at a time and it showed. Each of these is why a
line in the Go is the way it is.

Run it bare and it started every release of every database and ran the whole
test suite against each, which is half an hour of containers and is almost
never what somebody wanted. That was a destructive default for a command
people had started typing to look at one thing.

`vms` was a separate mode with its own loop, its own readiness wait and its
own output, so a Windows machine was not a server you could `start`, `stop` or
ask the `version` of. It was a different noun reached a different way, and the
only reason was that it was added later.

A selector was a tier, a product or an exact name. There was no way to say
"the newest PostgreSQL", which is what somebody debugging wants nine times in
ten, so they typed `postgres-18` and it went stale the month 19 shipped.

SQLite and DuckDB were not in it at all. They have no server, so they fell out
of a command organised around starting one, and `./run.sh test sqlite3` did
nothing.

And it was 300 lines of bash doing arrays, string splitting on a unit
separator, and a JSON encoder by hand. The unit separator was there because an
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

An **embedded database** is a library with no server at all: SQLite and
DuckDB. There is nothing to start, and the database is a file. Which dialects
those are is not decided here: every model declares it with
`dbmeta.Info.Embedded` and `dbrun` reads that, so one added later appears
without a second list to edit.

The file is treated the way a running container is. It lives under
`$XDG_DATA_HOME/dbmeta/embedded`, beside the machine disks, and it is kept
after a test so that `dbrun usql sqlite3` opens what the test built. Only
`remove`, or `test --remove`, deletes it.

`dsn` and `status` print a real connection string for one rather than prose.
The DSN is the bare path, because that is what `sql.Open` takes for both
drivers, and the URL names the scheme: `sqlite3:/path` and `duckdb:/path`.
Both products also answer to `file:`, which `dburl` resolves by reading the
file header and, when the file is not there yet, by the extension. `.db` is
SQLite there, so a DuckDB database is written `.duckdb` and naming the scheme
says which one in either state.

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
| `postgres --releases` | every PostgreSQL release |
| `all` | every release of every product |
| `tested` | the tier CI runs on every push |
| `nightly` | the tier CI runs at night |
| `verified` | the tier a person runs before a release |
| nothing | the help |

A bare product meaning the newest release is the change that matters. It is
what somebody wants when they are looking at something, it does not go stale,
and it keeps one server on the machine instead of six. The command says which
release it picked before it acts, because the answer changes the month a
release ships and a silent default is the part that would bite.

The draft of this wrote `--all` for both "every release of this product" and
"every product", and two reviews said the same thing about it: one word doing
two jobs reads as one job until it does not. So widening a product is
`--releases`, and everything is the selector `all`. A selector is a noun and a
flag modifies it, which is also why `all` is not a flag.

Bare is the help. A command that starts twenty containers and runs for half an
hour must be asked for in as many words.

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
| `logs` | show what the server said |
| `list` | show what a selector expands to, touching nothing |
| `build` | build the images this repository makes, for Cassandra and Oracle 19c |
| `provision` | build a Windows machine, which takes an hour |
| `help` | this |

`test` is what bare used to do and now has to be asked for. `test all` is the
old bare behaviour and is what a person runs before a release.

`test sqlite3` and `test duckdb` run the tests with no server, because those
two need none. Every other subcommand answers for them too: `status` prints
the URL and marks it `(embedded)`, `dsn` prints the path, `version` reports
what the driver links, and `start` prints where the file will be rather than
failing.

## How each kind answers each subcommand

| | container | machine | embedded |
| --- | --- | --- | --- |
| `start` | create or start | start the provisioned machine | nothing to do, prints where the file goes |
| `stop` | stop | stop | nothing to do |
| `remove` | delete | delete, and warn that it is an hour to rebuild | delete the file |
| `status` | running, with URL | answering, with URL and the viewer port, or starting | the URL, marked `(embedded)` |
| `version` | connect and read | connect and read | read what the driver links |
| `test` | start, wait, test, remove | start, wait, test, keep | test, keep the file |
| `provision` | not applicable | build it | not applicable |

Three differences are real and stay. A machine is kept after `test`, because
rebuilding it is an hour, where a container is removed because rebuilding it
is a minute. A machine waits on a query rather than on a port, because Windows
boots long before SQL Server listens.

And `status` opens a connection to a machine before it prints a URL, where for
a container it prints one on the strength of the container running. A running
container means the server answers, because `start` does not return until it
does. A running machine means nothing of the sort: `dockurr/windows` is up for
the whole hour Windows takes to install itself and for every reboot after it,
and the first version of this printed a URL that refused connections
throughout. A machine that does not answer within five seconds is reported as
starting.

## Why Go

The list of servers was already Go, in `container`. The shell read it through
`tool/servers`, which encoded each command as fields separated by U+001F
because an argument can contain a space, and `run.sh` read them back into
arrays. That whole layer existed to carry a `[]string` across a language
boundary, and it disappeared when there was no boundary.

`version` already shelled out to a second Go program, `tool/version`, because
it needs a driver. `test` shelled out to `go test`. So the shell was already a
launcher for Go, in a language where quoting a command is a thing you can get
wrong.

It also gets a real `--help`, real flags, and one place for the readiness loop
that each of the three kinds used to write for itself.

It is in the `test` module, because `version` opens a connection, which means
a driver, which hard rule 1 keeps out of the root module.

## Where it lives

`test/cmd/dbrun`, run as `go run ./cmd/dbrun` from `test`, or built once with
`go build -o ~/bin/dbrun ./cmd/dbrun`. `tool/servers`, `tool/vms` and
`tool/version` folded into it and are gone, and so is `run.sh`. No shim was
kept: a shim that only execs the Go is one more name for the same thing, and
the workflow and the documents were the only callers.

## Podman or docker

Either, by the command line rather than a Go client. `DBMETA_RUNNER` names
one, and otherwise podman is preferred and docker is used when podman is
absent, so the command works on a machine with either and nobody has to say
which. CI sets `DBMETA_RUNNER=docker`.

A Go client library was considered and rejected. It would be a dependency in
the `test` module for something the two commands already do identically, it
would have to speak two socket protocols to cover both, and the few places
they differ are one line each: `image exists` against `image inspect`, and
`rm --storage`, which podman has and docker has no need of.

## Building an image

Two images are built here rather than pulled. Cassandra's published image
refuses a user defined function, a materialized view and a role, which three
queries read, so `cmd/dbrun/image/cassandra.Containerfile` turns them on. And
Oracle publishes no free 19c image at all, only Dockerfiles and an installer
archive of three gigabytes.

The archive is looked for in the current directory, in `~/Downloads` and in
the home directory, and fetched to `~/Downloads` when it is not in any of
them. Not from Oracle: their download needs an account, an accepted licence
and a browser session, so no command can fetch it. It comes from a mirror, and
the SHA-256 that Oracle publishes is what says the file is theirs and arrived
whole. A mirrored copy that fails that check is deleted rather than kept, so
the next run fetches it again instead of failing the same way forever. The
licence still governs what the file is used for and is still the caller's to
accept.

Both were shell scripts and both are folded in. `start` and `test` build the
image when it is missing, so neither caller has to remember, and `build` makes
it whether or not it is there, which is what somebody wants after changing a
Containerfile. The Containerfile is embedded with `//go:embed`, so the command
carries its own build input. What stays shell is Oracle's own build script,
which is theirs, which changes with their layout, and which rewriting here
would mean owning a build we do not control.

## Provisioning

`provision` builds a Windows machine: it downloads a SQL Server installer of a
few hundred megabytes, writes an OEM directory with an unattended answer file,
starts `dockurr/windows` with that directory mounted, and waits up to an hour
while Windows installs itself and then SQL Server.

The draft of this recommended leaving it a 200 line shell script and having
`provision` exec it, on the grounds that the interface is what D68 is about
and that changing the implementation costs an hour per attempt. That was
wrong on the second point. `--render` writes the OEM directory and stops, so
the templating is checked in a second rather than an hour, and the port was
verified by rendering every release both ways and diffing: twelve files, all
identical to the byte. The hour is the install, and the port does not touch
the install.

So it is Go, and the payload is embedded with `//go:embed`. The script had to
locate its own directory to find the payload, which is why `./test/run.sh
--help` once failed with a path error. A binary that carries the payload has
nothing to find.

One bug was worth the diff. 2008 R2 wants the section header `[SQLSERVER2008]`
rather than `[OPTIONS]`, and the file carries a comment above the header
saying so. The shell used `sed 's/^\[OPTIONS\]$/[SQLSERVER2008]/'`, anchored
to a whole line. The first Go version used `strings.Replace` with a count of
one, which rewrote the comment and left the header alone. See `WINDOWS.md`.

## What this does not change

The list of releases stays in `container/container.go` and stays the only
copy. D42 decided that, D69 kept it, and this is a different program reading
the same list.

The CI workflow keeps calling the runner rather than starting containers
itself, which is D69 and D68 together.

Nothing about what the tests do. This is how a database gets started, not
what is asked of it once it is up.
