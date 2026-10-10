# Contributing to dbmeta

Read three things before you change anything.

[`docs/NULLS.md`](docs/NULLS.md) is the shortest document here and the one that
cost the most to learn. Never collapse a NULL. It is a hard requirement and it
is not negotiable.

[`AGENTS.md`](AGENTS.md) holds the rules: 16 of them, plus the Go conventions,
the lint policy and how to run the tests. It is written for an AI coding agent
and everything in it applies to a person. `CLAUDE.md` holds one line that
imports it, for Claude Code.

[`docs/decisions/`](docs/decisions/) holds every decision this project has made,
one file each, with the reasoning and what was rejected. The index in
[`docs/decisions/README.md`](docs/decisions/README.md) lists all 214 with their
status. Read the status: 79 of them amend or replace an earlier one.

Do not decide an open question on your own. The open questions are at the end
of `docs/PLAN.md`. Ask Ken.

## Before you send a change

```bash
gofmt -l . && go vet ./... && go build ./... && go test -race -count=2 ./...
golangci-lint run ./... && (cd test && golangci-lint run ./...)
```

`gofmt -l .` must print nothing. Run the tests with `-count=2`, because that is
what CI runs and a weaker command has let failures through twice.

The `test` directory is a separate module and `./...` does not reach it. It
holds the database drivers, and it is the only place in this repository that
can use cgo.

```bash
cd test && go run ./cmd/dbrun test tested
```

That runs the integration tests against every entry CI tests on every push. It
starts a container for each server, one at a time, and removes it after, and
it starts nothing for the embedded databases. To test one product, name it, as
in `dbrun test postgres`. `dbrun test tested nightly verified` runs every
release that a model reads, which is what has to pass before a release.
`dbrun test all` also runs the Staged servers, which no model reads, and which
are there for dbimp's drivers and for the flavors usql reaches. CI never runs
them, and `dbrun test staged` measures them again (D119). Each records the cadence it will have, which
becomes its tier when its model arrives (D120).

`dbrun` does everything to a database: `start` one and leave it up, `stop` it,
`status` to see what is running, `dsn` for a URL to paste, `usql` for a shell
on it, `version` to see what dbmeta reads. Run `go run ./cmd/dbrun help`.
Install it once if you use it often:

```bash
cd test && go build -o ~/bin/dbrun ./cmd/dbrun
```

Nothing else starts a container, which is D68. Read the rules at the top of
`docs/DBRUN.md` before you start one, because other people and agents share
the machine. `docs/CONTAINERS.md` says how to add a container or a machine.

## Adding a database

[`docs/DIALECT.md`](docs/DIALECT.md) is every step, in order: choosing the
version range, asking two other models and checking what they say, writing the
model and its fixture, the tests, parity, and the four places that hold a
count. It ends with the tests that fail when a step is skipped.

## Agent skills

The repository carries two agent skills. A skill is a set of instructions
that a coding agent loads for a task. `simple-english` sets how prose is
written, and `go-pedantry` sets how Go is written.
`TestProseIsSimpleEnglish` checks the rules of `simple-english` that a
machine can check, over every document, every comment and every error and
test message. If it reports a sentence, rewrite the sentence. See D156.

`skills-lock.json` names the source of each skill. The `skills` command from
npm writes that file, and version 1.7.0 is the one measured here. It writes
each skill into two folders. Codex and the other agents read
`.agents/skills/<name>`, and Claude Code reads `.claude/skills/<name>`.

To add a skill or to update one, run this in the repository root. The example
updates `simple-english`, and `skills-lock.json` holds the source for each
skill:

```bash
npx skills@1.7.0 add AminBlg/SimpleEnglish --skill simple-english --agent codex claude-code --copy -y
```

Keep `--copy`. Without it, the command writes `.claude/skills/<name>` as a
symbolic link. A Windows checkout writes a symbolic link as a text file, and
Claude Code then loads no skill and says nothing. `TestSkillsAreCopies` fails
on a link, and it fails when the two folders differ. See D89.

`.claude/settings.local.json` holds the Claude Code permissions of one person.
The root `.gitignore` ignores it.

## What the reviewer will ask

Does a new query run against a real server, on the oldest supported release and
the newest? A query that has never run against a database is not finished.

Does a new object kind ship a fixture object to read, and a row in
[`docs/COVERAGE.md`](docs/COVERAGE.md) for every database that cannot answer it?

Does a padded column select `NULL AS "name"` rather than a literal, and does
the field say which release it arrived in?

Is a lint finding a real defect or a linter making idiomatic Go worse? Only the
first gets a code change. The second gets a line in `.golangci.yml` with the
reason.
