# D42. Four releases per push, every release nightly

Status: Supersedes D24, amended by D69.

CI runs the integration tests against PostgreSQL 9.6, 12, 15 and 18 on every
push, and against all ten releases on a nightly schedule.

## Why four, and why those four

Testing the newest release alone is not enough, and there is evidence rather
than intuition for that. Running the queries against real servers found six
real faults. Here is the release that exposed each:

| Fault | Exposed on |
| --- | --- |
| a column gated at 12 that arrived in 15 | 12, 13, 14 |
| `EXECUTE FUNCTION` being release 11 syntax | 9.6, 10 |
| a field padded with an empty string, not NULL | every release below 15 |
| four fields wrongly marked as padded | 10 to 14 |
| a NULL access list collapsed into an empty string | every release |
| a NULL `check_clause` scanned into a string | every release |

Four of the six were invisible at the newest release. Testing 18 alone
catches two.

The floor, the ceiling and one release on each side of the middle catch all
six. That was checked rather than assumed.

## Why not a smaller set, and why not all ten on a push

A covering set over the version gates is not the right idea, and it is worth
saying why, because it looks right. The gates sit at 10, 11, 12, 13, 15, 16 and
17, and `{9.6, 18}` covers every one of them, since 9.6 is below all of them
and 18 is above all of them. That pair catches two faults out of the six.

The reason is that these tests do not check that the gates work. They check
that the gates are correct. A wrong gate is only visible between the
release it claims and the release that is true, and nothing predicts that
window. `colliculocale` was gated at 12 and arrived at 15, so it was fine at
9.6 and fine at 18 and broken at 12, 13 and 14.

That argues for all ten, and all ten do run, nightly. Four is what a push
carries, because a push has to stay fast enough that a contributor does not
avoid it, and four demonstrably catches every fault found so far.

## The rule for the next database

Do not hand maintain a list per database. The model already declares every
`Field.Min` and every fragment minimum, so generate the matrix from the gates:
the floor, the ceiling, and a release on each side of the densest gates. For a
database whose gates cluster, that gives three rather than ten by itself.

Two adjustments the shape of a database forces:

An embedded database has no container and no server version. SQLite3 and DuckDB
version with the Go module, so they belong in the unit job with a pinned module
version rather than in a container matrix.

A flavor is a separate target, not a second dimension. MariaDB and MySQL share
a driver and are different products, so the matrix is a flat list of pairs such
as `mariadb:11.4` and `mysql:8.4`, and the rule above applies to each on its
own.

A slow database goes nightly from the start. Oracle and Cassandra take minutes
to become healthy, and a push must not wait for them.
