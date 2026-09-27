# D53. One canonical expectation, checked in, that every database must meet

Status: Decided.

`TestConformance` builds the same core schema on every database, projects each
answer onto the facts that are portable, and compares the result against one
checked in file. `testdata/conformance.txt` has a section per database.

It does not replace `TestMySQLAgainstMariaDB`, which compares raw values within
one family and catches what this cannot. Goldens catch a regression and
pairwise catches a divergence, and they are different faults.

## The fixtures came first, and that was the largest part

Nothing had ever checked the fixtures against each other and they had drifted:
32 steps on PostgreSQL, 19 on MariaDB, 14 on SQLite, 12 on DuckDB, and three of
the four built a region and a shipment table where SQLite did not.

A comparison built on drifted fixtures reports the fixtures. So the fixtures
were aligned first: SQLite gained region and shipment, PostgreSQL's identity
and generated columns moved off author and book onto a table of their own, and
every view selects the same two columns.

`TestEveryFixtureBuildsTheCoreObjects` holds it, in the root module, needing no
database. It matches a `CREATE` rather than the name anywhere, which the first
version did not: renaming the region table did not fail the test, because
shipment's foreign key still said `REFERENCES region(country, area)`.

## Values are compared, not only names

Gemini said no value can be compared across families and DeepSeek said a subset
can. DeepSeek is right and the evidence is local: the fault that justified the
MariaDB comparison was a NULL that would not scan, which is a value fault that
names and row counts would have missed.

The portable subset is what the standard makes every database record the same
way: whether a column accepts NULL, where it sits, whether it is in the primary
key, whether it has an explicit default, and which columns a constraint covers
in what order with what it references.

## Why one file rather than ten pairwise comparisons

Five databases pairwise is ten comparisons and a failure does not say which
side is wrong. One expectation is five comparisons and every failure names the
database.

It works only because the canonical projection is release independent. A raw
golden would need one file per product per release, because PostgreSQL 9.6 and
18 disagree about raw values. Nullability and ordinal position do not change
between releases, so one file covers every release of every database, and the
PostgreSQL job checks it on all ten.

## The mechanism that stops a model being made to lie

This is the part that matters, and Gemini's answer to it was a principle where
DeepSeek's was a mechanism. The mechanism is taken.

Every function in `canonical.go` takes a value and returns a new one. None
takes a pointer to a model struct and none writes to a field.

`canonicalFields` names every field the projection drops, folds or maps, with
the reason, and `TestCanonicalFieldsAreRecorded` checks it by reflection. A
field on the model that is not on the canonical struct and not in the list
fails the build. It found eight undocumented drops the first time it ran.

The raw values stay asserted by each database's own tests, which this cannot
weaken. Where the projection maps a difference away, the raw behaviour is
pinned by a test named in the entry.

## What it found

Eleven differences, all real, none of them faults:

SQLite reports a primary key column as nullable, because an `INTEGER PRIMARY
KEY` there genuinely accepts NULL unless declared otherwise.

PostgreSQL reports a default on a primary key because `serial` is a `nextval`
default, where `AUTO_INCREMENT` is not a default at all.

MariaDB records the four character string `NULL` as the default of a nullable
column declared without one, where MySQL and everything else report no default.
`TestMySQLNullDefault` pins both, and it is the entry that justifies the one
mapping in `canonicalFields`.

MariaDB reports a view's columns as not nullable where the others say nullable.

Only PostgreSQL and DuckDB report the columns of a check constraint.

23 of the canonical lines are identical across all four.
`TestConformanceAgreementHolds` fails if that number falls, so something that
was uniform becoming non uniform is a decision somebody makes rather than a
thing that happens.

## Rejected: a step carrying every dialect's DDL

Gemini proposed `Exec map[Dialect]string` on a fixture step. DeepSeek's
objection is the one that decided it: a missing dialect key skips the step
silently, the expectation is regenerated, the test passes, and that database is
never exercised.

Silence is the failure mode this project has been bitten by most. Separate
fixtures fail loudly, and the core object test now catches what they miss.

## What productSpecific keeps doing

`TestMySQLAgainstMariaDB` keeps its hand written list of columns that
legitimately differ, and the list keeps its second job, which neither review
noticed: it is where `int(11)` against `int`, and MySQL parenthesizing a view
definition, got written down at all. A difference absorbed silently into a
file stops being knowledge. `canonicalFields` is the same idea for the
canonical comparison, which is why every entry carries a reason rather than a
flag.
