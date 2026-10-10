# psql Commands and the Go API

This maps every `psql` metadata command to the `dbmeta` Go value that answers
it. It exists so that wiring `usql` up later is a lookup rather than a reading
exercise.

The mapping was taken from `exec_command_d` in `src/bin/psql/command.c`, which
is the dispatcher `psql` itself uses, so the command spellings and the grouping
are `psql`'s rather than a reconstruction.

## How to read it

Each row gives a command, the Go value that answers it, and the type that value
yields. Every value is a `*dbmeta.Query[T]`, so the call is the same shape
whichever row you are on:

```go
for v, err := range dbmeta.Tables.All(ctx, m, db, args) {
	// v is a dbmeta.Table
}
```

A caller that wants the statement rather than the rows calls `Build` on the same
value, and `Fields`, `Params` and `Support` describe it. See the package
documentation.

`usql today` says whether `usql` currently implements that command at all. It
implements eleven of them, so most of this table is capability that exists in
`dbmeta` and has no consumer yet.

## Relations

| Command | Go value | Yields | usql today |
| --- | --- | --- | --- |
| `\d` | `dbmeta.Tables` | `Table` | yes |
| `\dt` | `dbmeta.Tables` | `Table` | yes |
| `\dv` | `dbmeta.Tables` | `Table` | yes |
| `\dm` | `dbmeta.Tables` | `Table` | yes |
| `\ds` | `dbmeta.Sequences` | `Sequence` | yes |
| `\di` | `dbmeta.Indexes` | `Index` | yes |
| `\dE` | `dbmeta.ForeignTables` | `ForeignTable` | no |
| `\dP` | `dbmeta.PartitionedTables` | `PartitionedTable` | no |
| `\dn` | `dbmeta.Schemas` | `Schema` | yes |
| `\l` | `dbmeta.Databases` | `Database` | yes |

`\dt`, `\dv` and `\dm` are one query in `psql` and one here. Narrow with the
`types` argument rather than with a different value: a table, a view and a
materialized view differ by `Table.Type`.

## Describing one relation

`\d NAME` is not one query. `psql` assembles it from several, in
`describeOneTableDetails`, which is the largest entry point in `describe.c` and
carries 21 of its 68 version gates. `dbmeta` keeps them separate, because a
caller usually wants one of them.

To reproduce `\d NAME`, read these with the same `Args{Schema, Parent}`:

| Part of the output | Go value | Yields |
| --- | --- | --- |
| the column list | `dbmeta.Columns` | `Column` |
| the index footer | `dbmeta.Indexes` | `Index` |
| the columns of each index | `dbmeta.IndexColumns` | `IndexColumn` |
| the constraint footers | `dbmeta.Constraints` | `Constraint` |
| the trigger footer | `dbmeta.Triggers` | `Trigger` |
| the sequence detail, for a sequence | `dbmeta.Sequences` | `Sequence` |
| the partition detail | `dbmeta.PartitionedTables` | `PartitionedTable` |

The sections that `\d+ NAME` adds are read the same way. Each takes the
parent filter, so one table costs one small statement. D199 holds the types
and the cost.

| Part of the output | Go value | Yields |
| --- | --- | --- |
| Partition key | `dbmeta.PartitionedTables`, the `Expression` field | `PartitionedTable` |
| Partitions, and Partition of with its bound and constraint | `dbmeta.Partitions` | `Partition` |
| Inherits and Child tables | `dbmeta.Inherits` | `Inherit` |
| Policies | `dbmeta.Policies` | `Policy` |
| Row security on and forced | the fields `RowSecurity` and `RowSecurityForced` of `dbmeta.Tables` | `Table` |
| Publications | `dbmeta.PublicationTables` with `schema` and `parent` | `PublicationTable` |
| Statistics objects | `dbmeta.ExtendedStats` with `parent` | `ExtendedStat` |
| Rules | `dbmeta.Rules` | `Rule` |
| Options of a table, a view or an index | the field `Options` of `dbmeta.Tables` and `dbmeta.Indexes` | `Table`, `Index` |
| View definition | `dbmeta.Views` | `View` |
| Server and FDW options of a foreign table | `dbmeta.ForeignTables` | `ForeignTable` |
| A composite type | `dbmeta.Tables` with `types=composite type`, and `dbmeta.Columns` | `Table`, `Column` |
| Not-null constraints, from release 18 | `dbmeta.NotNulls` | `NotNull` |
| the INCLUDE columns of an index | the field `Include` of `dbmeta.IndexColumns` | `IndexColumn` |
| the index text of the footer, such as `btree (label COLLATE "C" text_pattern_ops)` | the field `Using` of `dbmeta.Indexes`. `Definition` is the whole statement | `Index` |
| the constraint kind of an index, and the text of an exclusion constraint | the fields `ConstraintType`, `ConstraintDefinition` and `ConstraintPeriod` of `dbmeta.Indexes` | `Index` |
| the Cache of a sequence | the field `CacheSize` of `dbmeta.Sequences` | `Sequence` |
| the names that psql prints with no schema | the `Visible` fields of `Partition`, `Inherit`, `PartitionedTable`, `ExtendedStat` and `Index`. D201 lists them | |

## Routines and types

| Command | Go value | Yields | usql today |
| --- | --- | --- | --- |
| `\df` | `dbmeta.Functions` | `Function` | yes |
| `\dfa` | `dbmeta.Aggregates` | `Function` | no |
| `\da` | `dbmeta.Aggregates` | `Function` | yes |
| `\dT` | `dbmeta.Types` | `Type` | no |
| `\dD` | `dbmeta.Domains` | `Domain` | no |
| `\do` | `dbmeta.Operators` | `Operator` | no |
| `\dC` | `dbmeta.Casts` | `Cast` | no |
| `\dL` | `dbmeta.Languages` | `Language` | no |

`\dfn`, `\dfp`, `\dft` and `\dfw` narrow `\df` to normal functions, procedures,
triggers and window functions. They are the same query narrowed by
`Function.Kind`, not separate values.

## Users and privileges

| Command | Go value | Yields | usql today |
| --- | --- | --- | --- |
| `\du` | `dbmeta.Roles` | `Role` | no |
| `\dg` | `dbmeta.Roles` | `Role` | no |
| `\drds` | `dbmeta.RoleSettings` | `RoleSetting` | no |
| `\drg` | `dbmeta.RoleGrants` | `RoleGrant` | no |
| `\dp` | `dbmeta.Privileges` | `Privilege` | yes |
| `\z` | `dbmeta.Privileges` | `Privilege` | no |
| `\ddp` | `dbmeta.DefaultACLs` | `DefaultACL` | no |

`\z` and `\dp` are the same command in `psql`.

The two parts of `\dp` that `psql` prints as text are rows too. D201 holds the
reasons.

| Part of the output | Go value | Yields |
| --- | --- | --- |
| Column privileges, one line for each entry | `dbmeta.ColumnPrivileges` with `Args{Schema, Parent}` | `ColumnPrivilege` |
| Policies, with the command, the roles, the expressions and whether the policy is enabled | `dbmeta.Policies` with `Args{Schema, Parent}` | `Policy` |

## Storage and the server

| Command | Go value | Yields | usql today |
| --- | --- | --- | --- |
| `\db` | `dbmeta.Tablespaces` | `Tablespace` | no |
| `\dA` | `dbmeta.AccessMethods` | `AccessMethod` | no |
| `\dO` | `dbmeta.Collations` | `Collation` | no |
| `\dc` | `dbmeta.Conversions` | `Conversion` | no |
| `\dl` | `dbmeta.LargeObjects` | `LargeObject` | no |
| `\dconfig` | `dbmeta.Settings` | `Setting` | no |
| `\dd` | `dbmeta.Comments` | `Comment` | no |
| `\dy` | `dbmeta.EventTriggers` | `EventTrigger` | no |
| `\dX` | `dbmeta.ExtendedStats` | `ExtendedStat` | no |

## Foreign data

| Command | Go value | Yields | usql today |
| --- | --- | --- | --- |
| `\dew` | `dbmeta.ForeignDataWrappers` | `ForeignDataWrapper` | no |
| `\des` | `dbmeta.ForeignServers` | `ForeignServer` | no |
| `\deu` | `dbmeta.UserMappings` | `UserMapping` | no |
| `\det` | `dbmeta.ForeignTables` | `ForeignTable` | no |
| the options of `\dew+`, `\des+`, `\deu+` and `\det+`, and of a foreign table in `\d+` | `dbmeta.ForeignOptions` | `ForeignOption` | no |

`ForeignOption.Quoted` is the option as psql prints it inside the parentheses.
`Options` of each kind is the catalog text and keeps its meaning.

## Replication

| Command | Go value | Yields | usql today |
| --- | --- | --- | --- |
| `\dRp` | `dbmeta.Publications` | `Publication` | no |
| `\dRp+` | `dbmeta.PublicationTables` | `PublicationTable` | no |
| `\dRs` | `dbmeta.Subscriptions` | `Subscription` | no |
| `\dRs+`, the Conninfo column | `dbmeta.SubscriptionConnections` | `SubscriptionConnection` | no |

The other columns of `\dRs+` are fields of `Subscription`. Only a superuser can
read the connection string, so it is a kind of its own. D201 says why.

## Text search

| Command | Go value | Yields | usql today |
| --- | --- | --- | --- |
| `\dF` | `dbmeta.TextSearchConfigs` | `TextSearchConfig` | no |
| `\dF+` | `dbmeta.TextSearchConfigMaps` | `TextSearchConfigMap` | no |
| `\dFp` | `dbmeta.TextSearchParsers` | `TextSearchParser` | no |
| `\dFp+`, the Method and Function rows | `dbmeta.TextSearchParserFunctions` | `TextSearchParserFunction` | no |
| `\dFd` | `dbmeta.TextSearchDictionaries` | `TextSearchDictionary` | no |
| `\dFt` | `dbmeta.TextSearchTemplates` | `TextSearchTemplate` | no |

## Operator families

| Command | Go value | Yields | usql today |
| --- | --- | --- | --- |
| `\dAc` | `dbmeta.OperatorClasses` | `OperatorClass` | no |
| `\dAf` | `dbmeta.OperatorFamilies` | `OperatorFamily` | no |
| `\dAo` | `dbmeta.OperatorFamilyOperators` | `OperatorFamilyOperator` | no |
| `\dAp` | `dbmeta.OperatorFamilyFunctions` | `OperatorFamilyFunction` | no |

## Extensions

| Command | Go value | Yields | usql today |
| --- | --- | --- | --- |
| `\dx` | `dbmeta.Extensions` | `Extension` | no |
| `\dx+` | `dbmeta.ExtensionObjects` | `ExtensionObject` | no |

## What has no mapping

Two gaps, in opposite directions. One is now closed.

`\ss` is a `usql` command with no `psql` equivalent. It shows column statistics
values, and `psql` has never had such a command: no `"ss"` has ever appeared in
`command.c` and `describe.c` has never read `pg_stats`. Jan Was added it to
`usql` in 2021, implementing it for PostgreSQL from `pg_stats` and for Trino
from `SHOW STATS FOR`, and DuckDB gained it later with the same Trino syntax.

`dbmeta.ColumnStats` now answers it. The decision it needed was which shape to
take, because the databases disagree: PostgreSQL reports `null_frac`,
`n_distinct` and `most_common_vals`, Trino and DuckDB return the result of
`SHOW STATS FOR`, MariaDB keeps `mysql.column_stats` with its own columns, and
MySQL 8 keeps a JSON histogram under a view named like MariaDB's and holding
something else. D9 does not settle it, because PostgreSQL's shape is not
obviously right for a histogram.

The shape taken is PostgreSQL's, with every field nullable. A database that
computes something reports it and a database that does not reports absent,
which is the padding rule applied to a kind rather than to a release.
PostgreSQL, MariaDB, SQL Server, Oracle, SAP HANA, Apache Hive, CrateDB,
Databend, SingleStore and Impala answer. MySQL, SQLite and the other models
report `ErrNotSupported`, because none of them has a width, a null fraction or
a distinct count to give. A row of absences is worse than no row. See
COVERAGE.md.

`\sf` and `\sv` show the source of a function or a view. They live outside
`describe.c` and are not part of the 49, so they are not in this table.
`Function.Definition` answers the first, and `dbmeta.Views` answers the
second. `Function.Source` holds the body where a product keeps one apart from
the statement (D147).

## The kinds with no psql command

Seven object kinds here answer no `psql` command. They exist because `usql` and
`dbtpl` were measured and needed them, and D47 allows them: `psql` sets the
object model and does not set the column set.

| Go value | Yields | Why psql has no command |
| --- | --- | --- |
| `dbmeta.ConstraintColumns` | `ConstraintColumn` | `psql` prints a constraint as one line of text, which `Constraint.Definition` still holds |
| `dbmeta.RoutineParameters` | `RoutineParameter` | `psql` prints a signature as one line, which `Function.ArgTypes` still holds |
| `dbmeta.EnumValues` | `EnumValue` | `\dT+` prints the labels joined into one string, which `Type.Elements` still holds |
| `dbmeta.Views` | `View` | `\d name` on a view prints the definition, and `\sv` shows it outside `describe.c` |
| `dbmeta.ColumnStats` | `ColumnStat` | `psql` has no such command. `usql` added `\ss` |
| `dbmeta.CurrentSchema` | `Schema` | session state rather than an object. `dbtpl` reads it in every loader |
| `dbmeta.CurrentUser` | `User` | session state rather than an object. It came from auditing `usql` (D55) |

In each of the first three the prose and the parts are both available. The
parts are authoritative and the prose is what `psql` prints, so a client
matching `psql` output does not have to rebuild the string for every dialect.

Two fields follow the same rule. `Column.PrimaryKey` is in the row MySQL and
SQLite already select and costs PostgreSQL one join. `Function.ID` identifies a
routine where the name does not, and `RoutineParameters` carries the same value
so that the join is one expression everywhere.

## Wiring usql up

An agent doing that work needs three things beyond this table.

The caller supplies the dialect and version. `usql` already reads a version
string per driver, and `dbmeta.Dialect.Version` replaces that. The dialect is
`dburl.URL.Dialect`, which dburl sets when it parses a URL, from v0.32.0.

Read `URL.Dialect` and never `URL.Driver`. `dburl` registers a scheme per Go
driver, so `URL.Driver` names the driver: `pgx` or `moderncsqlite`,
and from dburl's D22 `pgx` for `postgres://` too. None of those is a dialect
here. `URL.Dialect` is `postgres` or `sqlite3` for each of them,
because dburl holds that taxonomy and hard rule 1 keeps it out of `dbmeta`.
D99 in [`decisions/`](decisions/README.md) has it.

The flavor needs nothing. `dbmeta` reads it from the server rather than from
the URL: `models/mysql` sets the `mariadb` key when `SELECT VERSION()` carries
the suffix, so a caller passes the `mysql` dialect for both products and never
has to say which. A product that speaks another product's protocol is not a
flavor. From dburl v0.36.0, `cockroachdb`, `cratedb`, `redshift`, `memsql`,
`tidb` and `vitess` each arrive with a dialect of their own, and a model for
one of them shares the statements of the model it imitates where they answer
(D123, D125).

`Query.Support` decides whether to offer a command. It returns `NotBuilt` when
the model was left out of the binary by a build tag, `NotSupported` when the
database has no such object, and `TooOld` when the product has the object and
this release of it does not. Those are three different messages to a person.

Rendering stays in `usql`. D5 keeps the `tblfmt` writer there, so the loop is
to read rows from a `Query` and hand them to the existing writer. The old
`Reader` and `Writer` interfaces do not carry over. `usql` writes no new
reader until it reads `dbmeta` (usql D3).
