# D119. A release that no model reads is Staged

Status: Amends D40, D103, D106, D112, D113, D114 and D116.

## The problem

`container` names every server that `dbrun` starts, and D103, D106, D112,
D113, D114 and D118 added many that no dbmeta model reads. They are there for
dbimp's drivers, for the wire compatible flavors that usql reaches, and for
the emulators of hosted services. Most of them were Tested.

CI runs `dbrun test` on each Tested release. For a release that no model
reads, that starts the container, runs the setup, and runs integration tests
that find no variable they read, so nothing in dbmeta is tested. On
2026-09-28 that was 95 of 158 servers and libraries, and the Tested tier had
grown from 63 releases to 120. Ken asked for them to be kept out of CI, while
the record still says that each one was measured and works.

## The decision

A fourth value of `container.Tier` holds them: `Staged`. Ken chose the name on
2026-09-28.

A Staged release is one that `dbrun` starts and that no model reads yet. Each
one was measured through `dbrun` when it was added. It started fresh, answered
its check, ran its setup, and its users did what its file says. It stopped,
started again, answered again, and was removed. That is all the tier promises.
It is not a release that dbmeta supports, and CI never runs one. A person runs
`dbrun test staged` to measure them again.

The rule has no exception. A release that a model reads is Tested, Nightly or
Verified. A release that no model reads is Staged. It applies to every kind of
target:

1. A container entry. `TestAReleaseIsStagedExactlyWhenNoModelReadsIt` in
   `container` fails in either direction, and a machine must have a model.
2. A hosted service. Neon, PlanetScale and Redshift are read by the postgres
   and mysql models, so they stay Verified. The rest are Staged.
   `TestAServiceIsStagedExactlyWhenNoModelReadsIt` in `hosted` holds it.
3. An embedded library. moderncsqlite is read by the sqlite3 model and stays
   Tested. chai, csvq and ql are Staged.

Stardog, GraphDB and Volt Active Data are Staged too, although each also needs
a licence file. The missing model is the reason CI does not run them, and it
would be the reason even if CI had the files.

A release moves out of Staged in the change that adds its model. That change
sets the tier to Tested, Nightly or Verified, and the test above fails until
it does. The range of releases does not change, because each product's file
already records it by the rules in `docs/EVALUATION.md`.

## What it changed

CI builds its matrix from `dbrun list --json --names tested` and `nightly`, as
D69 set, so nothing in the workflow changed. The Tested tier went from 120
releases to 35, and Nightly from 36 to 26. The six releases of the three
licensed products are Staged too, and dbrun lists them only with their files.

The decisions that made these releases Tested or Nightly keep what they say
about the range, the users and what was measured. Only the tier is amended:
D103 for SurrealDB, D106 for Neo4j, D112 for the eight servers it added, D113
for Avatica, Phoenix and Druid, D114 for InfluxDB, and D116 for the embedded
libraries that have no model. D40 set three tiers and D42 added Nightly, so
there are now four values, and Archived is still not one of them.

## Rejected

Three other forms were weighed with Gemini and DeepSeek, and both agreed with
this one.

1. Keep these Tested and let CI skip a release that has no model. The tier
   is what says how thoroughly a release is tested, and a Tested release that
   CI skips makes the tier say something false.
2. Make them Verified. Verified means a person runs the release before every
   dbmeta release, and nobody would run 95 releases that test nothing.
3. Record the date each one was measured, with a test that fails when the date
   is old. A test that fails because time passed breaks a change that has
   nothing to do with it. The measurement is recorded in D118, in
   `docs/EVALUATION.md` and in each product's file.

A weekly job in CI that runs `dbrun test staged` was also weighed. Ken asked
for these to be kept out of CI, so there is none.
