# D50. Documentation lives in docs, and the decision log stays one file

Status: Amended by D110 and D111.

Three files in the repository root: `README.md`, because GitHub renders it,
`CLAUDE.md`, because an agent reads it first, and `CONTRIBUTING.md`, because
GitHub gives it its own behaviour. Everything else is in `docs/`.

Both reviews agreed on that much and on nothing else, and the measurement that
prompted it was 6116 lines of Markdown in 11 root files against 8261 lines of
Go.

## The decision log is one file with an index

Gemini wanted this file split into one record per decision, the ADR
convention, on the grounds that 3384 lines is about 25,000 tokens and an agent
loads all of it to answer one question.

That is not taken, and the evidence is in this repository.

Six of the 49 decisions amend, supersede or withdraw an earlier one. D48 amends
D26. D24 supersedes D22. D11 is amended by D26. D19 is half overtaken. D6 and
D38 are amended. One in eight, and the rate rises rather than falls, because a
project that runs long enough learns things.

Split, an amendment lives in a different file from the decision it amends. An
agent greps a topic, lands on the older record, reads a rule that was
overturned, and gets no signal that it was. A slow answer becomes a wrong
answer, which is worse than a slow one.

There are 217 references to a decision by bare number in this file and 336 more
in the other documents and in the Go source. Split, every one of those is a
filename to guess, because `D8` does not say whether the file is
`0008-metadata.md` or `0008-abandon-the-subpackages.md`.

Gemini's cost is real and the index is the answer to it. The table at the top
of this file gives the number, the title and the status of every decision in 55
lines. An agent reads the table, jumps to one decision, and loads that. Nobody
had tried it before deciding to split.

## The status column is the part that matters

A decision is read through its status. `Decided` means it stands.
`Amended by D26` means read both. `Superseded by D24` means read the other one.
Put the status in the heading when you add a decision, and the index picks it
up.

## One rule per file where the rule is expensive

`NULLS.md` stays its own file. Gemini wanted it deleted and merged into
both `CLAUDE.md` and `CONTRIBUTING.md`, which would put the most expensive rule
this project has learned in two places and guarantee they drift. It is linked
as a requirement from both instead.

## Two indexes, because there are two readers

`README.md` lists the documents for a person arriving from pkg.go.dev.
`CLAUDE.md` holds a routing table for an agent: what to read before touching a
given thing. They answer different questions and neither replaces the other.

## What was not archived, and a correction

Both reviews were told `QUERIES.md` and `EVALUATION.md` were written once and
never updated, and both suggested archiving or renumbering them on that basis.
The description was wrong and the advice followed from it.

`QUERIES.md` is cited from `object.go`, `models/postgres/postgres.go` and the
information_schema test. It is the survey that justifies the object set, and a
reference document that is still correct does not need updating to be live.
`EVALUATION.md` is the procedure for choosing the supported versions of a
database that is not covered yet, which is a thing the project will do dozens
more times.

Both stay in `docs/` as reference. Nothing is archived, because nothing here is
stale.

## REVIEW.md is gone

It held the argument behind decisions already taken, which is what this file
holds. Two places for the same reasoning is the failure this decision is about,
so its contents were folded into the decisions they argue for and the file was
removed.

An open question goes in the open questions section at the end of this file. A
decided one becomes a decision. There is no third state that needs a document.

## The thing neither review raised

`COVERAGE.md`, `USQL.md` and `DBTPL.md` are generated from measurement and go
stale silently. Filing them better does not fix that. A test that fails when
the counts drift would, and `container/workflow_test.go` already does exactly
that for the CI matrix. That is worth more than any amount of organizing and it
is not done yet.
