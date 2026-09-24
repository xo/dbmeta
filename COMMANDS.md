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

A caller that wants the statement rather than the rows calls `SQL` on the same
value, and `Fields`, `Params` and `Support` describe it. See the package
documentation.

`usql today` says whether `usql` currently implements that command at all. It
implements twelve of them, so most of this table is capability that exists in
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

## Replication

| Command | Go value | Yields | usql today |
| --- | --- | --- | --- |
| `\dRp` | `dbmeta.Publications` | `Publication` | no |
| `\dRp+` | `dbmeta.PublicationTables` | `PublicationTable` | no |
| `\dRs` | `dbmeta.Subscriptions` | `Subscription` | no |

## Text search

| Command | Go value | Yields | usql today |
| --- | --- | --- | --- |
| `\dF` | `dbmeta.TextSearchConfigs` | `TextSearchConfig` | no |
| `\dFp` | `dbmeta.TextSearchParsers` | `TextSearchParser` | no |
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

Two gaps, in opposite directions.

`\ss` is a `usql` command with nothing behind it here. It shows column
statistics values, and `psql` has never had such a command: no `"ss"` has ever
appeared in `command.c` and `describe.c` has never read `pg_stats`. Jan Was
added it to `usql` in 2021, implementing it for PostgreSQL from `pg_stats` and
for Trino from `SHOW STATS FOR`, and DuckDB gained it later with the same
Trino syntax. `dbmeta` has no `ColumnStats` object kind.

Adding one is a decision rather than a translation, because the databases that
have column statistics disagree about their shape. PostgreSQL reports
`null_frac`, `n_distinct` and `most_common_vals`. Trino and DuckDB return the
result of `SHOW STATS FOR`. MariaDB has `information_schema.COLUMN_STATISTICS`
with its own columns, and MySQL 8 has a histogram under the same view name with
different contents again. Four shapes for one idea, and D9 does not settle it,
because PostgreSQL's shape is not obviously the right one for a histogram.

`\sf` and `\sv` show the source of a function or a view. They live outside
`describe.c` and are not part of the 49, so they are not in this table. The
text they print is available as `Function.Source` and through the view
definition.

## Wiring usql up

An agent doing that work needs three things beyond this table.

The caller supplies the dialect and version. `usql` already reads a version
string per driver, and `dbmeta.Dialect.Version` replaces that. The dialect is
`dburl.URL.Driver`, which `usql` already has.

`Query.Support` decides whether to offer a command. It returns `NotBuilt` when
the model was left out of the binary by a build tag and `NotSupported` when the
database has no such object, and those are different messages to a person.

Rendering stays in `usql`. D5 keeps the `tblfmt` writer there, so the loop is
to read rows from a `Query` and hand them to the existing writer. The old
`Reader` and `Writer` interfaces do not carry over, and their names are being
changed in `usql` itself.
