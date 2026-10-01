# D61. Every dialect is measured against every principal the product has

Status: Amended in place.

A dialect is not finished until every query is asked as the administrator
and as each lesser kind of principal the product has, and the differences are
written down. `test/parity_test.go` does it and
`test/testdata/parity.txt` is the record.

## Why

D60 asked whether Oracle must read `DBA_` views, and the argument rested on
`ALL_` showing a caller only what the caller can see. Nobody had asked whether
the other products do the same thing. They do, and one of them is worse than
Oracle.

Measured against the fixture schema, with every principal given the same
rights over it, so that the only thing varying is what kind of principal it
is:

| Database | Principal | Queries that answer differently |
| --- | --- | --- |
| Oracle 26ai | local user | none |
| SQL Server 2022 | contained user | none |
| SQL Server 2022 | server login | `roles` |
| PostgreSQL 18 | schema owner | `settings`, `tablespaces` |
| PostgreSQL 18 | grantee | `settings`, `tablespaces` |
| Cassandra 5.0 | granted role | `privileges`, `role_grants`, `roles`, `settings` |
| ClickHouse 26.9 | granted user | `constraints`, `databases`, `foreign_servers`, `index_columns`, `indexes`, `privileges`, `role_grants`, `roles`, `tablespaces` |
| MySQL 8.4 | grantee | `foreign_servers`, `functions`, `role_grants`, `roles`, `user_mappings` |
| MariaDB 13.0 | grantee | `aggregates`, `column_stats`, `foreign_servers`, `role_grants`, `roles`, `user_mappings` |

`current_user` and `current_schema` are left out of that table and are in the
file. They are supposed to differ, because they answer a question about the
connection, and a run where they agree is the fault.

One release needed a section of its own. PostgreSQL 12 grants public SELECT on
six columns of `pg_subscription` and not on `subsynccommit`, so an ordinary
role is refused `Subscriptions` there and served from 13 on. A section can
therefore be written `product@major`, and that one wins for a server reporting
that major. The query is not gated for it: a superuser on 12 reads the column,
and padding it withholds a fact from the caller who is allowed it.

Cassandra behaves like the MySQL dialect and for the same reason: `roles`,
`role_grants` and `privileges` read `system_auth`, and `settings` reads
`system_views`, and a role with every permission on its own keyspace is
refused all four outright. It has no containment either, so a role belongs to
the cluster and a keyspace is only a grant scope.

Oracle is the cleanest of the five, which is the opposite of what D60 assumed.
A local user owning the objects gets the administrator's answer to every
query. The MySQL dialect is the worst: a user holding ALL PRIVILEGES on its
own database has queries refused outright, six on MariaDB and four on MySQL,
because they read `mysql.proc`, `mysql.column_stats`, `mysql.servers`,
`mysql.roles_mapping`, `mysql.role_edges` and `mysql.user`. Those are tables
in the `mysql` database rather than views that filter themselves, so the
server answers with error 1142 and the query fails. That is the same shape as
Oracle's `DBA_` problem and it was never recorded. PostgreSQL refuses
`tablespaces` on `pg_global` and hides parameters from `pg_settings`.

MariaDB and MySQL do not agree with each other, which is why the section is
named for the product rather than the dialect. `mysql.proc` was removed in
MySQL 8.0 and MariaDB still has it, so `aggregates` is refused on one and
answered on the other.

## A principal is not one thing

SQL Server has three and they are not interchangeable. A sysadmin. A server
login mapped to a database user, which is the ordinary model. And a contained
database user, whose password is in the database and which has no login at the
server, which needs `CONTAINMENT` set to `PARTIAL`.

Oracle has the same three from 12c. `SYSTEM` is the administrator. A common
user exists in the container database and in every pluggable database at once,
which is what a server login is. A local user authenticates against one
pluggable database and has nothing above it, which is what a contained
database user is.

PostgreSQL has no containment, because a role belongs to the cluster rather
than to a database. The nearest three are the superuser, the owner of the
objects, and a role holding only grants.

MySQL and MariaDB have no containment either. A user is a name and a host at
server level and a database is only a grant scope, so there are two.

SQLite and DuckDB have no user, no role and no grant. There is no second
connection to make, so the rule does not reach them and cannot.

## The rule

A dialect ships its queries, its fixture, its documentation and its parity
targets. Those are one deliverable and not four, the same way rule 9 makes the
fixture part of the queries. A dialect with queries and no parity target is not
nearly finished. It is one whose answers were measured for exactly one kind
of user.

`TestEveryDialectIsMeasuredForParity` holds it. Every dialect must have a
target or an entry in `parityExempt` giving the reason it has none, and one
with neither fails. `TestPrivilegeParity` cannot do this job: it skips a target
whose server is not running, and it says nothing at all about a target that was
never written. Without this check, a dialect added without one passes every
test here.

Only three are exempt. SQLite and DuckDB have no user to be, and
`infoschema_over_postgres` is a test registration of the shared model over a
PostgreSQL server rather than a product.

Add a dialect, add its principals to `parityTargets` in
`test/parity_test.go`, run `go test -run TestPrivilegeParity -update`, and
read the diff. A product with a kind of principal that no target covers is not
finished. Say in `docs/COVERAGE.md` which queries differ and why, because a
consumer choosing a connection needs to know which answers depend on who is
asking.

A difference is not a failure. A principal with no privilege on another schema
has no business seeing it. The file records the set so that a change in the
set is what fails, the same way `conformance.txt` works.

## Two principals nothing covers yet

An Oracle common user cannot be made from inside a pluggable database, and
every Oracle target now names a pluggable database. Covering it needs a
connection to `CDB$ROOT`, which `container/oracle.go` already records as a
target worth having and which is not written.

A SQL Server sysadmin that is not `sa` is not covered either. Nothing suggests
that it answers differently from `sa`, so it is not worth a target.

## A scene can be newer than the server

A contained database arrived in SQL Server 2012. On 2008 R2 `sp_configure` has
no `contained database authentication` option and refuses the name, so the
contained scene cannot be prepared there at all. A scene therefore carries a
`min`, and a server older than it is skipped with the reason, the same way a
fixture step the server is too old for is skipped rather than refused.

A kind of principal that a release does not have is not a gap in coverage. It
is the product, and recording it as a skip says so where a reader sees it.
