# D111. A large project keeps one file per decision

Status: Amends D50.

Ken decided on 2026-09-27 that a large project keeps each decision in a file
of its own. He said that a markdown file many thousands of lines long is not
readable. `docs/PLAN.md` held all 109 decisions in 7,979 lines and 397 KB,
which is about 100,000 tokens, while the median decision is 59 lines.

## The layout

Each decision is `docs/decisions/D<nnn>-<title>.md`, with the number in three
digits so that a listing sorts in order. The file opens with its number and
title, a blank line, and its status:

    # D89. Agent skills are committed as copies, and one command installs them

    Status: Decided.

`docs/decisions/README.md` is the index, and GitHub shows it as the page of
the folder. `docs/PLAN.md` keeps the plan: the purpose, the architecture, what
exists, the testing plan and the open questions. It is 615 lines.

A reference by number, such as D47, still works, because it names the number
and not a place in a file. `TestEveryDecisionReferenceExists` reads the folder.

## Why D50's reason no longer holds

D50 kept one file because an amendment would live in a different file from the
decision it amends, and a reader who landed on the older one would get a rule
that no longer holds. `TestAnAmendmentPointsBothWays` answers that. It fails
unless both decisions name each other in their status, and each file now opens
with its status, so a reader sees an amendment before anything else.
`TestTheDecisionIndexIsComplete` checks every row of the index against the
file, title and status alike, and prints the row to add.

The move found one fault. D77's heading said "amended by D84", and its row in
the old index said only "Amends D66", because the old test checked that a row
existed and not what it said. Both now say "Amends D66, amended by D84".

## Which projects

A large project splits its decisions: dbmeta, dbimp and usql. A small library
keeps its decisions in `docs/PLAN.md`, with the index at the top, until it
grows. Ken chose that line.

## How it was moved

A script cut `docs/PLAN.md` at each decision heading, moved each decision to its
file, lowered each heading inside it by two levels, and made each relative
link one folder deeper. It was checked by counting the words of the old file
against the new files. The only words missing were the introduction of the old
index and the paragraph about status, which the new index holds.
