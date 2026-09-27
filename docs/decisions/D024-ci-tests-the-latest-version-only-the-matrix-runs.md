# D24. CI tests the latest version only. The matrix runs locally

Status: Supersedes D22, superseded by D42.

CI tests the major databases at their latest version and nothing else. Every
other version, and every flavor, is tested on a development machine.

This overrides D22, which proposed one CI job per supported major. Read D22 for
why a sampled matrix misses catalog changes. That reasoning still holds. The
matrix is not cancelled, it moves off CI, because running every version of
every database on every push costs more than the project will pay.

The CI databases are PostgreSQL, MySQL and SQLite3. D35 removed DuckDB, because
D29 makes the project pure Go and DuckDB has no pure Go driver. More can be
added later.

CI runs two jobs, not one. The first opens no connection and covers every
release through a fake driver replaying recorded data. The second starts a real
PostgreSQL 18 as a service container and runs the integration tests against it.
Only the newest release runs there, which is what this decision requires. The
other nine run with `dbrun` before a release.

One detail the container needs. Its health check must force TCP, with
`pg_isready -U postgres -h 127.0.0.1`. Checking the socket reports ready during
the bootstrap phase, before the server restarts to accept connections, and a
job that starts then fails with a connection reset.

## What the runner actually provides

Checked against `actions/runner-images` for Ubuntu 24.04 on 2026-09-24. The
four databases fall into two groups, and the difference decides how each one is
started.

SQLite3 and DuckDB are embedded. There is no server and no container. SQLite3
is preinstalled at 3.45.1, and DuckDB arrives as a Go driver. Both are free to
test.

PostgreSQL and MySQL are servers. Both are preinstalled and both have their
service disabled, so a job starts them with `sudo systemctl start
postgresql.service` or `sudo systemctl start mysql.service`.

Two facts about the preinstalled versions matter.

The preinstalled PostgreSQL is 16.15, not 18. The latest release is not what
the runner gives you. A job that must test the newest PostgreSQL needs a
service container or the upstream apt repository. Starting the preinstalled
service tests release 16.

The preinstalled MySQL is MySQL 8.0.46, not MariaDB. There is no MariaDB on the
runner. D14 makes MariaDB the reference product for the `mysql` driver, so the
preinstalled server is the flavor rather than the reference.

That second point is useful rather than a problem. CI exercises MySQL while
local testing exercises MariaDB, so the flavor axis of D14 gets covered on
every push at no extra cost. Write it down as intentional, because someone will
otherwise "fix" it by installing MariaDB in CI and lose the coverage.

## The risk this accepts, and what reduces it

A version regression now reaches the main branch unless somebody runs the local
matrix. The `pg_attrdef.adsrc` class of fault, where a newer server removes a
catalog column, is exactly the kind that the CI job on the latest version can
miss for an older supported version.

Three things keep that risk small. None is optional.

1. One command runs the full local matrix. If running it takes research, it
   will not be run.
2. The local matrix runs before a release, and the result is recorded in the
   release notes. A release that has not passed it does not go out.
3. The support table in `README.md` states which versions CI covers and which
   are covered only locally. Do not claim in public that a version is tested
   when only a person's machine tests it.
