# D88. Vertica is a model, on four community images

Status: Amends D66, amended by D100.

D66 put Vertica fifth and then recorded that it could not be started: the
official image was withdrawn, the maintained one runs only under the
Kubernetes operator, and the download one-node-ce builds from went away. It
also rejected `saadmairaj/vertica:10.1.1` as five years old, built by a
stranger and impossible to reproduce. Ken decided on 2026-09-27 to go ahead
with community images, and `models/vertica` answers 26 of the 55.

## The images, and why D66's objection no longer decides it

| Release | Image | Built | Tier |
| --- | --- | --- | --- |
| 25.1.0 | `docker.io/ratiopbc/vertica-ce:v25.1.0-0` | 2024-12-17 | Tested |
| 10.1.1 | `docker.io/saadmairaj/vertica:10.1.1-RHEL6` | 2021-05-22 | Nightly |
| 9.1.0 | `docker.io/iamamr/vertica:9.1.0-0` | 2018-08-10 | Nightly |
| 7.2.1 | `docker.io/colemantw/vertica:latest` | 2016-01-21 | Nightly |

The 25.1 image is not a stranger's build of Vertica. Its layers are the steps
of Vertica's own `vertica-containers/one-node-ce` Dockerfile, its entrypoint
carries the Open Text copyright and Apache licence, and its binary reports
Vertica Analytic Database v25.1.0-0. It is a copy of the image the withdrawn
`vertica/vertica-ce` published, pushed by somebody else, and 25.1 is a
current release. That answers the part of D66's objection that mattered,
which was that nothing a consumer runs could be tested.

The three older images are a stranger's builds of real releases, and they
are there for a different reason. One release cannot exercise a version gate,
and Vertica's catalog grows at every release: `v_catalog` has 56 tables on
7.2, 73 on 9.1 and 89 on 10.1. With them the model's gates have servers on
both sides. They run nightly, because they are large and they are the gates'
floor rather than what a consumer connects to today.

Every image was pushed once and never rebuilt, so every entry pins its digest
as well as its tag. A push to the same tag would otherwise change what was
tested without a line changing here.

## How a password reaches each image

The 25.1 entrypoint creates `APP_DB_USER` with `PSEUDOSUPERUSER` and the
password it is given, which is how [Password] reaches it. `dbadmin`, the
superuser the database is created with, has no password and cannot be given
one from outside. The three older images share one entrypoint that takes
nothing from the environment, so `Server.Init` creates the same user, with
the same role, once the server answers, and checks first so that it is safe
on every start. Every Vertica here is then reached as `dbmeta` with
[Password].

The readiness check connects over TCP as that user. The first version asked
as `dbadmin` over the local socket, which answers as soon as the database
exists and before the entrypoint has loaded VMart and created the user, and
it reported a server up whose first connection was refused.

## The driver splits a statement at every semicolon

`vertica-sql-go`, the driver `usql` uses, splits a statement at each
semicolon before it sends it, and it does not know a SQL function's `BEGIN
... END` body. `CREATE FUNCTION f(n INT) RETURN INT AS BEGIN RETURN (n * 2);
END;` reaches the server in halves and is refused near EOL. `vsql` creates
it without complaint. The semicolon cannot be left out either, so no SQL
function can be created through the driver `usql` ships. That is a driver
fault and it is `usql`'s to know about. The fixture creates none, and
Functions reads the functions the packages Vertica installs provide, and the
PL/vSQL procedure, whose dollar quoted body survives the split.

## The gates

Each is the oldest release measured to have the thing, and the release
before it in the list lacks it:

| From | What |
| --- | --- |
| 9.1 | CHECK constraints, SET USING columns, a function's owner, a user's connection limit |
| 10.1 | `LISTAGG`, and a comment on a table column rather than on a projection column |
| 25.1 | PL/vSQL, a procedure's language, owner and security, triggers, `user_configuration_parameters` |

The 25.1 row is measured present on 25.1 and absent on 10.1, and no release
between is measured, so the gate is where the thing was seen. A server in
between is told Triggers and RoleSettings are too old. That can be wrong for a
release nobody has run here, and a measurement is what would move it.

Before 10.1 there is no string aggregate, so Privileges returns a row per
object and grantee rather than a row per object. The columns are the same,
which is what hard rule 3 requires, and the field says so.

## The analogues

A projection is an index: it is a stored, sorted copy of a table's columns
and it is what the optimizer chooses between. A storage location is a
tablespace. An HCatalog schema, which reaches a Hive metastore, is a foreign
server. An external table, which reads files through a COPY statement, is a
foreign table. A user's own parameter value is a role setting, and a role
carries none. A trigger keeps Vertica's word and names no table, because it
runs a procedure on a schedule.

Four were rejected. RoutineParameters, because a routine's arguments are one
comma separated list of types, and the named parameters a library function
declares are `USING PARAMETERS` options rather than arguments. DefaultACLs,
because a schema's inherited privileges are a flag that makes new objects take
the schema's grants, which is a stretch from a default privilege. Languages,
because nothing lists them, and DeepSeek's `user_libraries.language` does not
exist. ColumnStats, because `table_statistics` holds row counts and nothing
exposes a column's distribution.

## Conformance learned about releases

`TestConformance` assumed one answer holds for every release of a product.
Vertica is the first product whose fixture cannot build a core object on an
old release: 7.2 has no CHECK constraint, so it reports one line fewer. A
section named `product@major` now wins for a server reporting that release,
the same way parity's has since D61, and a release's section is left out of
the agreement ratchet, because it is the product again and not another
database.

## Parity

The administrator is `dbmeta`, which holds `PSEUDOSUPERUSER`, because
`dbadmin` has no password. The owner and the grantee are PostgreSQL's two.
7.2 and 9.1 have no `ALTER SCHEMA ... OWNER`, so there the owner is handed
each table, view and sequence and granted usage on the schema. 10.1 refuses a
lesser principal `access_policy` outright, which Privileges joins for its
policies, so the whole query is refused there, and 25.1 serves it.

## Changing a password

`ChangePassword` builds `ALTER USER ... IDENTIFIED BY`, with `REPLACE` for
the current password. The password is a string literal, so it reads
`standard_conforming_strings` the way PostgreSQL's does. `usql`'s Vertica
driver concatenated the password with no escaping at all, so this is a move
that fixes something, the same as D56's. Every hostile password logs in on
7.2 and 25.1 under both settings.
