# D64. The Verified tier is checked against the document

Status: Decided.

`TestEveryVerifiedReleaseIsDocumented` fails when a release the Go list calls
Verified is not named in `docs/COVERAGE.md`.

## Why the tier needed anything

D40 gives three tiers and two of them have a machine behind them. CI runs
Tested and Nightly, and `TestWorkflowReadsTheList` fails when the workflow
and `container/container.go` disagree. Verified has nothing: it means a person
ran the release on a development machine, and no test can prove that.

What a test can prove is that the two lists agree, and that is the failure that
happens. Oracle 18c spent months connecting to the container database while
every other release connected to a pluggable one, and nothing noticed, because
nothing compared the list to the document.

## What it does not do

It checks one direction. A release the Go list calls Verified has to appear in
the document, so adding one and not writing it down fails. It cannot check the
other direction, because the document says what a release was verified against
in prose a person wrote, and parsing that to find a claim with no release
behind it is guessing.

It also cannot prove anybody ran anything, and it does not pretend to. Recording
a date and a checked-in log was considered and not taken: it is stronger
evidence and one more thing to keep current by hand, and the failure it
catches is not the one that happened.
