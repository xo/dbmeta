# D199. Describe sections for partitions, inheritance, policies and rules

Status: Decided.

## The decision

usql brings `\d+ NAME` up to psql 18 and asked for the sections that psql
prints after the column list. D198 added the fields of one row. This decision
adds the sections. A section that is a child of a table is a kind of its own
with flat rows, as D47 requires. A section that is one more fact about the
row is a field. Only the PostgreSQL model fills them. Every other model leaves
a field NULL, and leaves a new kind unsupported. No other model changed
except where a field is a plain `bool`, which it leaves false.

I checked each section against psql 18.6, with `psql -E` on a PostgreSQL 15
server, so the statements are the ones psql sends.

| Section of `\d+ NAME` | Carried by | Type |
| --- | --- | --- |
| Partition key | `PartitionedTable.Expression`, which held it already | `string` |
| Partitions, with the bound | new kind `Partitions`, field `Bound` | `sql.Null[string]` |
| Partition of, and Partition constraint | `Partitions` read by the name of the partition, fields `Table`, `Bound` and `Constraint` | `sql.Null[string]` |
| `\dP` Table, Access method | `PartitionedTable.Table` and `AccessMethod` | `sql.Null[string]` |
| `\dP` Total size | `PartitionedTable.TotalSize`, and `DirectSize` | `sql.Null[int64]` |
| `\dP` Parent name | `PartitionedTable.Parent`, which held it already | `string` |
| Inherits, Child tables | new kind `Inherits` | |
| Policies | new kind `Policies` | |
| Row security on and forced | `Table.RowSecurity` and `RowSecurityForced` | `sql.Null[bool]` |
| Publications of a table | `PublicationTable`, which gains a parent filter and `Via` | `sql.Null[string]` |
| The WHERE and the column list of a publication | `PublicationTable.Where` and `Columns`, which held them already | |
| Statistics objects | `ExtendedStat`, which gains a parent filter and `StatsTarget` | `sql.Null[int64]` |
| Rules | new kind `Rules` | |
| Options of a table, an index or a view | `Table.Options` and `Index.Options` | `sql.Null[string]` |
| View definition | `View.Definition`, which held it already | `sql.Null[string]` |
| Foreign table Server and FDW options | `ForeignTable.Server` and `Options`, which held them already | |
| Composite type | `Tables` with `types=composite type`, and `Columns` | |
| Exclusion constraint text | `Constraint.Definition`, which held it already | `sql.Null[string]` |
| Not-null constraints | new kind `NotNulls` | |
| INCLUDE column of an index | `IndexColumn.Include` | `bool` |

## The new kinds

`Partitions` has one row for each partition. `Schema` and `Table` name the
parent. `PartitionSchema` and `Partition` name the partition. The other
fields are `Type`, `Bound` (what follows FOR VALUES, or DEFAULT), `Constraint`
(`pg_get_partition_constraintdef`), `Partitioned` (the partition has
partitions of its own) and `DetachPending`. The parameters are `schema` and
`parent` for the parent, and `partition_schema` and `name` for the partition.
Read the partitions of a table with `parent`. Read the parent and the bound of
a partition with `name`. It needs release 10 and is refused below it.
`DetachPending` is a plain `bool` and is a real false below release 14, which
has no `inhdetachpending`.

`Inherits` has one row for each parent of each child, from `pg_inherits`.
`Schema`, `Name` and `Type` name the child. `ParentSchema` and `Parent` name
the table it inherits from. `Ordinal` is `inhseqno`. `Partition` is true when
the child is a partition, because PostgreSQL records a partition in
`pg_inherits` too and psql leaves it out of Inherits and Child tables. The
statement does not leave it out, because rule 13 forbids it. The parameters
`schema` and `name` read the parents of one child. The parameters
`parent_schema` and `parent` read the children of one parent. The word
parent here is the inheritance parent, which is why it has a schema
parameter of its own. An index is never a row, because a partitioned index
has children that psql does not call inheritance. `Partition` is a real false
below release 10.

`Policies` has `Command` (all, select, insert, update or delete),
`Permissive`, `Roles` (the names joined by a comma, NULL for public, as psql
prints them), `Using` and `WithCheck` (NULL when the policy has none) and
`Comment`. It takes the usual `schema` and `parent`. It needs release 9.5.
`Permissive` is a real true below release 10, which had only permissive
policies.

`Rules` has `Event`, `Enabled` (enabled, disabled, replica or always),
`Instead` and `Definition`, which is `pg_get_ruledef` without the final
semicolon, as psql prints it. The rule named `_RETURN` is not a row. It is how
a view stores its definition, psql leaves it out, and `Views` holds it.

`NotNulls` has `Name`, `Column`, `NoInherit`, `Local`, `Inherited` and
`Validated`, from the rows of `pg_constraint` with the type n. Release 18 is
the first that names a NOT NULL constraint, so the kind is refused below 18.
`Constraints` still leaves these rows out on every release (D49), so the same
schema answers the same way in `Constraints` everywhere. `Column.Nullable`
still answers on every release.

## The new fields

| Kind | Field | Source | Filled from |
| --- | --- | --- | --- |
| Tables | `Options` | `reloptions` of the relation, then `toast.` and each option of the TOAST table, joined by a comma and a space. NULL for none | every release |
| Tables | `RowSecurity`, `RowSecurityForced` | `relrowsecurity`, `relforcerowsecurity` | every release |
| Indexes | `Options` | `reloptions` | every release |
| IndexColumns | `Include` | `k.ordinality > indnkeyatts` | 11. A plain `bool`, false below 11 and for every other model |
| PartitionedTables | `Table` | the table of an index, as `schema.name`. NULL for a table | 10 |
| PartitionedTables | `AccessMethod` | `pg_am` through `relam` | 10. NULL for a relation that has none |
| PartitionedTables | `DirectSize`, `TotalSize` | the sum of `pg_table_size` over `pg_partition_tree`, for the leaves one level down and for every relation of the tree | 12 |
| PublicationTables | `Via` | table, schema or all tables | 10 for table and all tables, 15 for schema |
| ExtendedStats | `StatsTarget` | `stxstattarget`, NULL for the default | 13 |

`Options` is text in psql's order, so a caller that wants the pairs splits on
a comma and a space. A value that holds that text is not an option PostgreSQL
accepts. `Persistence` and the others from D198 are unchanged.

`PublicationTables` takes `schema` and `parent` now, to name the table. It
read only the publications that name a table. A publication of a schema
(release 15) or of every table offers a table without naming it, and psql
lists those too under Publications. A new parameter `with_implicit`, false by
default, adds them as rows with `Via` set to schema or all tables. It is off by
default because a publication of every table has one row for every table, and
the old reading of `\dRp+ NAME` must not change.

`ExtendedStats` takes `parent` now, the name of the table. psql 18 also
prints the statistics target of the object, which is `StatsTarget`.

`Tables` lists a composite type, with `Type` composite type, only when the
caller names it in `types`. Without it the list is what it was, because psql
lists none. `Columns` already read the attributes of a composite type, because
it never filtered on the kind of the relation. So `\d+ app.pair` needs
`types=table,view,composite type,...` in the lookup that finds the relation.

The exclusion constraint of the request needed no change. `Constraints`
already returns `pg_get_constraintdef`, so the `Definition` of an exclusion
constraint is `EXCLUDE USING gist (int4range(id, id + 1) WITH &&)`. The test
reads it back. The shorter text `gist (int4range(id, id + 1))` is what
`pg_get_indexdef` gives, so usql read the index and not the constraint.

`View.Definition` is `pg_get_viewdef(oid, true)`, the same call psql 18 makes,
for a view and for a materialized view. The Options of a view, such as
`security_barrier=true` and `check_option=local`, are in `Table.Options`.

## A fault found on the way

`PartitionedTables` scanned `pg_get_partkeydef` into a `string`. A partitioned
index has no key, so the function answers NULL and the scan failed on any
server with a partitioned index. The fixture had none. It does now. The two
fields stay plain strings, and the scan reads NULL as empty with
`dbmeta.NullAsEmpty`, because a changed type breaks every model that
fills them.

## What each release fills

| Release | What is NULL, false or refused |
| --- | --- |
| 9.6 | `Partitions` and `NotNulls` are refused, and so are the kinds that were refused before: publications, publication tables, subscriptions, statistics objects and partitioned tables. `IndexColumn.Include` is false |
| 10 and 11 | `DirectSize`, `TotalSize` and `StatsTarget` are NULL. `NotNulls` is refused. `Include` is false on 10 |
| 12 | `StatsTarget` is NULL. `NotNulls` is refused |
| 13 to 17 | `NotNulls` is refused. The rows of `Via` for a schema need 15 |
| 18 | nothing, except what the data leaves empty |

`Field.Min` agrees with each fragment and a test checks it, so every release
returns the same columns. PostgreSQL answers 61 kinds on 18, 60 on 10 to 17
and 54 on 9.6.

## The cost

Measured on PostgreSQL 12 with `EXPLAIN (ANALYZE)` in a scratch database
of 22397 relations. It had 5000 tables with a primary key, one partitioned
table of 1000 partitions, 1000 policies, 1000 rules on other tables, 1000
children of one table, 500 statistics objects, 200 publications and one
publication of every table. A table of 7000 rows is the largest answer here.
Times are execution times in milliseconds.

| Statement | Unfiltered | With the parent filter |
| --- | --- | --- |
| Partitions, 1000 rows | 9.8 | 6.0 for the 1000 partitions of one table, 0.28 for one partition by name |
| Inherits, 2000 rows | 5.3 | 2.3 for the 1000 children of one table, 0.03 for one child |
| Policies, 1000 rows | 7.5 | 0.08 |
| Rules, 1000 rows | 10.7 | 0.08 |
| ExtendedStats, 500 rows | 4.1 | 0.18 |
| PublicationTables, 200 rows | 2.1 | 0.03 |
| PublicationTables with `with_implicit`, 7201 rows | 8.5 | 0.05 |
| PartitionedTables, 1 table of 1000 partitions | 13 | 9.8 by name |
| Indexes, 5000 rows | 86 | 0.19 |
| IndexColumns, 5000 rows | 53 | 0.09 |
| Tables, 7002 rows | 82 with the new fields, 73 to 97 without | 0.14 by name |

Every unfiltered cost grows with the rows the statement returns and no other
way, which is what D47 allows. A filter on the table narrows the rows before
any function runs, and the answer for one table takes under a millisecond,
except where the table has many partitions. Nothing scans the whole catalog
for one table. The three new fields of Tables cost about 1.5 microseconds for
each relation, which is the join to the TOAST table and the array, and the
difference was inside the variation between two runs. `DirectSize` and
`TotalSize` call `pg_partition_tree` twice for each partitioned relation, and
the call walks the whole tree, so reading them for a table of 1000 partitions
takes about 10 ms. A caller that lists the partitioned tables of a large
catalog pays that for each of them. If it matters, the fix is one call with a
lateral join. It was not needed at this size.

`Partitions` and `Rules` call a function for each row
(`pg_get_partition_constraintdef`, `pg_get_ruledef`), and `Policies` runs one
subquery on `pg_roles` for each row. All three stay near 10 microseconds for a
row. usql reads them for `\d+ NAME`, which is one table, and a tool that
reads the whole catalog pays the 10 ms shown above.

## Parity and conformance

`TestPrivilegeParity` on PostgreSQL 9.6, 12, 15 and 18 gives the same
sections as before. The owner and the grantee read the same rows as the
administrator for every new kind and field. The catalog tables behind them
(`pg_policy`, `pg_rewrite`, `pg_inherits`, `pg_statistic_ext`,
`pg_publication_rel`) are readable by every role, and `pg_get_viewdef`,
`pg_get_ruledef`, `pg_get_constraintdef` and `pg_get_expr` check no privilege
on the object. The conformance golden is unchanged. The canonical projection
reads the columns and constraints of the core tables only.
Running `-update` with one server up also rewrites the order of the Cassandra
section in `test/testdata/parity.txt`, which has nothing to do with this
change, so that file was left as it was.

## CockroachDB

CockroachDB shares the statements for Tables, Indexes, PublicationTables and
PartitionedTables. Two functions are missing there: `pg_partition_tree` and
`pg_relation_is_publishable`. So the sizes of `PartitionedTables` and the
extra rows of `PublicationTables` use a fragment that gates on the
CockroachDB key. They answer NULL, and no row, on CockroachDB. The other new
columns of Tables exist there and are read. The new kinds are not shared, so
they are not supported on CockroachDB. The fixture steps that build the new
objects are in the left map of `models/cockroachdb/fixture` with the reason
"not measured on CockroachDB", and the tests skip on them. I ran the test
module against CockroachDB 26.2.7 and 26.3.2, and both pass.

## What was left out

`ForeignTable.Options` is the catalog text `name=value, name=value`. psql
prints each pair as the quoted name and the quoted value. A value that holds a
comma and a space cannot be split back. A flat kind of options fixes it. I
did not add one, because the request lists Server and FDW options as present.
It needs a decision.

`Args` has no field for `parent_schema`, `partition_schema` and
`with_implicit`. A caller passes a plain map for them, as the tests do.

The fixture has no table in `Partitions` that is a foreign table, and no
detached partition, so `Type` of a foreign partition and `DetachPending` true
are read in the statement and not in a test.
