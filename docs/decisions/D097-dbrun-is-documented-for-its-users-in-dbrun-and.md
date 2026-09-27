# D97. dbrun is documented for its users, in DBRUN and CONTAINERS

Status: Decided.

Ken asked on 2026-09-27 for better documentation of `dbrun`: how to add a
container or a machine, and how to use the command on a machine that several
people and coding agents share. It is written for a coding agent and for a
developer alike.

## Two documents, by task

`docs/DBRUN.md` is for anybody who starts a database. It opens with the rules
for a shared machine, then gives the common tasks, then the reference: the
commands, selectors, names, ports, credentials, output, exit status, the
environment and what `dbrun` does not do. `docs/CONTAINERS.md` is for the
rarer task of adding a server, with its rules, one procedure for each kind of
entry, the fields, the readiness faults that were really made, and the tests
that catch a skipped step.

The `usql` and `dbimp` sessions were asked, and both chose these two names
and this split. A reader looks for the command by its own name, and a reader
who starts a database never needs to add an entry. `TESTING.md` was offered
and rejected, because both read it as how dbmeta tests itself. Gemini and
DeepSeek both proposed two documents with the rules first. Gemini also
proposed paired wrong and right examples, which `CONTAINERS.md` has as the
table of real faults.

## What happened to the design document

`docs/RUNNER.md` held the design of `dbrun`, and its name no longer said what
it held. Its instructions moved into the two documents. Its reasons were
already decisions: D68 for one command and one name, D70 for Go, for the
command line over a client library, for the Oracle mirror, for `--render` and
for the selector `--releases`, D75 for the limit of four, D82 for the prebuilt
test binary, and D86 for machines. The reasons that no decision held are
kept here:

- Bare `dbrun` prints the help. The shell script it replaced started every
  release of every database and tested each one when it was run bare, which
  is half an hour of containers that nobody who typed the bare command meant.
- A bare product names its newest release. That is what somebody who is
  looking at one thing wants, it does not go stale when a release ships, and
  it keeps one server on the machine rather than six. The command prints
  which release it picked, because a silent default is the part that bites.
- `status` opens a connection to a machine before it prints a URL, and trusts
  a container that runs. `start` does not return until a container answers,
  but a Windows machine runs for the whole hour that it installs itself, and
  the first version printed a URL that refused every connection.
- A port is the place of a server in the list, so a release added earlier
  moves the port of every server after it. Adding Trino 476 left the running
  Trino 483 on its old port, and every test failed with connection refused
  against a server that `podman ps` showed as up. `start` rebuilds such a
  container and `status` reports it.

## The rules for a shared machine, and what they cannot yet say

The rules in `DBRUN.md` ask each session to start only what it tests, to keep
the machine at four servers, to leave another session's servers alone, and to
leave the machine as it found it. `dbrun` itself cannot yet keep the third
rule. A server carries no owner, and D75 stops the longest running server of
any session when a start brings the count to five. That stopped a Couchbase
server that another session was using on 2026-09-27. So the rules tell a session not
to start a fifth server when any of the four is not its own. The open
question at the end of this file asks what `dbrun` does about it.
