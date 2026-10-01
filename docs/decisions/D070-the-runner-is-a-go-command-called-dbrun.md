# D70. The runner is a Go command called dbrun

Status: Amends D68 and D12.

`test/cmd/dbrun` starts every database this project tests against. It replaced
`test/run.sh`, and no shim was left behind: a shim that only execs the Go is
one more name for the same thing, and the workflow and the documents were the
only callers. `tool/servers`, `tool/vms` and `tool/version` are gone with it,
and so is `test/vm/provision.sh`.

The design document of `dbrun` held the design, and this records what it
decided. D97 later folded that document into `DBRUN.md` and `CONTAINERS.md`.

## Why a language

The list of servers was already Go, in `container`. `tool/servers` printed it
with fields separated by U+001F, because an argument can contain a space, and
the shell read them back into arrays. That whole layer existed to carry a
`[]string` across a language boundary. `version` already ran a second Go
program because it needs a driver, and `test` ran `go test`. The shell was a
launcher for Go written in the one language where quoting a command is
something you can get wrong.

It is in the `test` module, because `version` opens a connection, which means
a driver, which hard rule 1 keeps out of the root module.

## The command line, not a Go client

`DBMETA_RUNNER` names podman or docker outright. Otherwise podman is preferred
and docker is used when podman is absent, so the command works on a machine
with either and nobody has to say which.

A Go client library was considered and rejected. It is a dependency for
something the two commands already do identically, it has to speak two socket
protocols to cover both, and the places they differ are one line each:
`image exists` against `image inspect`, and `rm --storage`, which podman needs
and docker has no state for.

## What folded in

Two images are built rather than pulled, and both were shell scripts.
Cassandra's published image refuses a user defined function, a materialized
view and a role, which three queries read. Oracle publishes no free 19c image
at all, only Dockerfiles and a three gigabyte installer archive that no command
can fetch from them, because their download needs an account and a browser
session. It is fetched from a mirror when it is not already on the machine,
and the SHA-256 Oracle publishes is what says the file is theirs. A copy that
fails the check is deleted, so a bad download is not kept. `start` and `test`
build a missing image so that neither caller has to remember, and `build`
rebuilds one on demand. The Cassandra Containerfile is embedded with
`//go:embed`, so the command carries its own build input. What stays shell is
Oracle's own build script, which is theirs, and rewriting it here means owning
a build we do not control.

`provision` folded in too, and that reverses the design's own recommendation.
It said to leave provisioning a script and exec it, because the interface is
what D68 is about and because changing the implementation costs an hour per
attempt. The second half was wrong. `--render` writes the OEM directory and
stops, so the templating is checked in a second, and the port was verified by
rendering all four releases both ways and diffing: twelve files, identical to
the byte. Then a machine was built from the Go and answered
`dbmeta.SQLServer.Version`. The hour is the Windows install, and the port does
not touch the Windows install.

The OEM payload is embedded. The script had to locate its own directory to
find it, which is the same fault that made `./test/run.sh --help` fail with a
path error. A binary that carries its payload has nothing to find.

## The one bug the diff caught

2008 R2 wants the section header `[SQLSERVER2008]` rather than `[OPTIONS]`,
and `ConfigurationFile.ini` carries a comment above the header saying so. The
shell wrote `sed 's/^\[OPTIONS\]$/[SQLSERVER2008]/'`, anchored to a whole
line. The Go wrote `strings.Replace` with a count of one, which rewrote the
comment and left the header alone. It renders, it installs for forty minutes,
and it fails at the end.

A translation is not finished because it compiles. It is finished when its
output is compared to the output of the thing it replaced.

## One selector changed

The draft wrote `--all` for both "every release of this product" and "every
product". Two model reviews said the same thing about it, which is that one
word doing two jobs reads as one job until it does not. Widening a product is
now `--releases` and everything is the selector `all`. A selector is a noun
and a flag modifies it.

## What this does not change

The release list is still `container/container.go` and still the only copy.
D42 decided that, D69 kept it, and this is a different program reading the
same list.

Nothing about what the tests do. This is how a database gets started, not what
is asked of it once it is up.
