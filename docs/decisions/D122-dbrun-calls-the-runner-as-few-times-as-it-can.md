# D122. dbrun calls the runner as few times as it can

Status: Decided.

## The problem

`dbrun` runs podman or docker as a program, and each call costs about 30
milliseconds before it does anything. `dbrun status -a` asked about every
target in turn, with one `inspect --format` for each fact: the state, the
owner, the name of the owner and the ports. For some 160 targets that was
several calls each, and it took ten seconds. `dbrun remove` of 92 servers that
had no container took eight seconds, because it asked about each one and ran
`rm` for each.

On 2026-09-29 `status` and `version` became two calls: `ps --all` lists the
containers, and one `inspect` of the ones that exist returns the rest as JSON.
A command that acts remembers from the same listing which targets have no
container, and inspects each one that exists once. `status -a` went from 9.9
seconds to 0.07, and the removal of 92 absent servers from 8.2 seconds to
0.03.

## The decision

Ken asked on 2026-09-29 that every change to `dbrun` that runs podman or
docker makes a best effort to call it as few times as it can. That is the
intent, and a reviewer cannot measure effort. So these are the rules a
reviewer checks a change by:

1. If `dbrun` reads facts about a container, it reads the JSON that the runner
   writes, and takes every fact from that one answer. It does not run one
   `--format` template for each fact. If podman and docker write the JSON
   differently, `dbrun` makes the two the same in Go, as it does for the slash
   that docker writes before a name.
2. If `dbrun` reads several containers, it names all of them in one command,
   such as one `inspect`. It does not call the runner once for each container
   in a loop.
3. If a listing or an inspect already answered a question, `dbrun` takes the
   answer from what it read, and does not ask the runner again.
4. If a runner command names a container, `dbrun` forgets what it read about
   that container before the command runs. `start`, `stop`, `rm`, `run` and
   `exec` are such commands, and the next read asks the runner. A correct
   answer comes before a fast one.
5. If a change must call the runner once for each target, a comment beside
   the call says why no single call can do it.

Two tests hold the counts. `TestStatusCallsTheRunnerTwice` runs a status of
every target against a fake runner that counts its calls, and fails if there
are more than two. `TestActingOnAbsentServersCallsTheRunnerOnce` does the same
for `stop` and `remove` of servers that have no container. A change that adds
a call there fails the test, and it changes the test only with a reason.

## How it is built

`runner.seen` holds what the runner said about each container, keyed by
name, and a name that maps to nothing is a container the runner does not
have. `remember` fills it from one listing at the start of each command.
`look` answers from it, and asks the runner only about a container it has not
read. `forget` runs before every runner command other than `inspect`, `ps`,
`image` and `build`, and drops each container the command names. Before a
server starts, `dbrun` reads the owners of every running server with one
`inspect`, where it made three calls for each.

The same change fixed a false message. `remove` of a server that had no
container ran `rm` anyway and said "removed". It now returns and says
nothing, the way it already did for a machine.

## Rejected

Gemini and DeepSeek were asked about the wording, and both said that "best
effort" does not belong in the rules, because a reviewer cannot check it. It
is kept above as Ken's intent, and the rules are what it means.

DeepSeek asked for a budget of calls for every command. Only `status` and the
removal of absent servers have one, because those are the two that were
measured slow. Another command gets a budget when it is measured.
