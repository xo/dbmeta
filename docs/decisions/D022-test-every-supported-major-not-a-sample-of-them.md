# D22. Test every supported major, not a sample of them

Status: Superseded by D24.

D24 overrides where this runs. CI tests the latest version only, and the full
matrix runs on a development machine. The technical argument below is why the
matrix must exist at all, and it is unchanged. Only the venue changed.

Run one job per supported major version of each primary database. Under D20
that is PostgreSQL 14, 15, 16, 17 and 18, which is five runs.

The two reviews disagreed here, and the disagreement was settled against the
PostgreSQL source rather than by preferring one model.

Gemini argued for testing the floor, the middle and the ceiling only, on the
grounds that catalog changes are monotonic: a column added in one release stays
in the next. DeepSeek said that is false and that a sampled matrix misses a
change in an untested middle version.

DeepSeek is right, and the source proves it. PostgreSQL removes catalog
columns. Commit `fe5038236c` in the PostgreSQL tree is titled "Remove obsolete
pg_attrdef.adsrc column". A query written for release 11 fails on release 12
with `column "adsrc" does not exist`. Testing 11 and 13 does not find it.

`describe.c` shows the same thing directly. It carries 3 gates of the form
`pset.sversion < N`, at 11, 12 and 15. A gate that asks whether the server is
below a version exists because something present in the older release is absent
in the newer one. Catalog change is not monotonic.

The release 15 tree carried 11 such gates. The drop is not evidence that the
catalog settled down. Release 20 removed every code path for a server below 10,
so the gates went with the code rather than with the problem.

The deprecation rule in D21 is what keeps this affordable. It removes a version
every year, so the matrix stays near five rather than growing without bound.

DeepSeek also suggested generating the CI matrix from the version gates in the
code, so that every distinct minimum version becomes a job. Take this once the
fragments exist. It makes the matrix follow the queries automatically, and it
cannot drift from them.

Add one job beyond the supported set: the current beta of the next release,
allowed to fail without failing the build. It gives warning of a catalog change
before the release lands.
