# D90. A database qualifies when it is free to run for development and testing

Status: Decided.

Ken decided on 2026-09-27 that a database qualifies for `dbmeta` when a
person can get it and run it for development and testing without paying. An
open source license is not the test. A source available release, an
evaluation edition, a developer edition and a free community edition all
qualify.

No document said that a database must be open source. The question came up
because ScyllaDB stopped its open source edition after 6.2, and its releases
from 2025.1 are source available with a free tier. That was read as a reason
to stop at 6.2, and Ken said that the reading was too strict. The project
already worked the other way. SQL Server runs as the Developer and
evaluation editions, Oracle as Free and XE, SAP HANA as the express edition,
Exasol as the Community Edition, Vertica as the community edition, and Db2 as
the Community image. None of them is open source in the way that PostgreSQL
and MariaDB are.

Two things follow. If a release needs an account, a signup or an accepted
license, its decision records that, as D85 does for the Exasol signup and D76
does for the SAP license. A release that nobody can run without paying cannot
be tested, so step 2 of `EVALUATION.md` decides against it, and this decision
changes nothing there. `EVALUATION.md` holds the rule under Which databases
qualify.
