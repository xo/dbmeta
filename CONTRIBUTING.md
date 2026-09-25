# Contributing to dbmeta

Read three things before you change anything.

[`docs/NULLS.md`](docs/NULLS.md) is the shortest document here and the one that
cost the most to learn. Never collapse a NULL. It is a hard requirement and it
is not negotiable.

[`CLAUDE.md`](CLAUDE.md) holds the rules: 15 of them, plus the Go conventions,
the lint policy and how to run the tests. It is written for an AI coding agent
and everything in it applies to a person.

[`docs/PLAN.md`](docs/PLAN.md) holds every decision this project has made, with
the reasoning and what was rejected. The table at the top lists all 65 with
their status. Read the status: six of them amend or replace an earlier one.

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
may use cgo.

```bash
cd test && ./run.sh tested
```

That starts a container per database release, runs the integration tests
against each, and removes it. `./run.sh` with no argument runs every supported
release of every product, which is what has to pass before a release.

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
