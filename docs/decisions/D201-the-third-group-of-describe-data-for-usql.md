# D201. The third group of describe data for usql

Status: Amends D147 and D199.

## The decision

usql brings its describe commands up to psql 18. D198 and D199 gave it the
fields of one row and the sections of `\d+`. usql then measured what was left
and asked for a third group. This decision adds it. It also settles the two
points that D199 left open.

The rule that decided each shape is D47. A fact that psql prints as text and
the models flatten is returned as parts next to the text, and the text keeps
its meaning. A part that belongs to a child of an object is a kind with flat
rows, never a slice on the parent. A part that is one more fact about the row
is a field. Only the PostgreSQL model fills them. Every other model leaves a
field NULL and leaves a new kind unsupported, and no other model changed
except where a field is a plain `bool`, which it leaves false.

I checked each item against psql 18.6, from the source of `REL_18_6`, which
holds the statements psql sends. I ran the new statements on PostgreSQL 9.6,
12, 15 and 18, and on CockroachDB 26.2.7 and 26.3.2.

PostgreSQL answers 65 kinds on 18, 64 on 10 to 17 and 57 on 9.6. Four kinds
are new.

## Text that psql prints, as parts

| Text psql prints | Carried by | Reason |
| --- | --- | --- |
| `(host 'x', "user" 'u')`, the options of a wrapper, a server, a user mapping and a foreign table | new kind `ForeignOptions` | a child of four objects, so one kind with flat rows |
| the labels of `\dT+`, one on each line | `EnumValues`, which exists since D46 | it has one row for each label, with its order |
| `name:` then one line for each entry, in `\dp` | new kind `ColumnPrivileges` | a child of a column |
| `rls_p (r):` then the expression and the roles, in `\dp` | `Policies`, which exists since D199 | it has the command, the roles and both expressions |

`ForeignOption` has `Kind` (foreign data wrapper, foreign server, user mapping
or foreign table), `Schema` (a foreign table only), `Name` (the object, and the
local role for a user mapping), `Server` (a user mapping and a foreign table),
`Ordinal`, `Option`, `Value` and `Quoted`. The parameters are `kind`, `schema`,
`name` and `server`. One kind and not four, because the four have one shape and
usql reads them all to print one list. The statement is one `UNION ALL` of four
reads, each of which applies its own filter before it splits the options, so a
filter on one object reads only that object.

`pg_options_to_table` splits the array in the server, so a value that holds a
comma and a space is one row. That is what D199 left out, and the fixture holds
such a value on a server and on a foreign table. `Quoted` is the option as psql
prints it, `quote_ident(name) || ' ' || quote_literal(value)`. A name that is a
keyword, such as `user`, is quoted, and the rule is the server's, so a caller
cannot copy it. D127 says every quoting rule lives here. `ForeignTable.Options`,
`ForeignServer.Options`, `ForeignDataWrapper.Options` and `UserMapping.Options`
keep the catalog text and the same meaning.

A role that is neither the mapped user nor the owner of the server reads a user
mapping with NULL options, because `pg_user_mappings` hides them. Such a
mapping gives no row of options. The privilege parity test asks with the schema
filter, which excludes user mappings, so it does not record this. I measured it
with a role that has no rights: 8009 option rows for the superuser and 7007 for
that role, and the 1002 that are missing are all user mappings.

`ColumnPrivilege` has `Schema`, `Table`, `Column`, `Ordinal` (the position of
the entry in the list of the column), `Access` (the entry as psql prints it,
such as `pd_writer=w/postgres`), `Grantee` (NULL for public), `Grantor` and
`Privileges` (the letters, such as `rw`). The parameters are the usual
`schema`, `parent` for the table, `name` for the column, and `with_system`. A
table with an entry for two roles is two rows. `aclexplode` names the grantee
and the grantor. The letters are cut from the text of the entry with a pattern
that skips a quoted role name. `Privilege.ColumnAccess` keeps its text.

The policies of `\dp` and the labels of `\dT+` need no new kind. `Policies` has
`Command`, `Permissive`, `Roles`, `Using` and `WithCheck`, and it takes the
table as a filter, so usql reads the policies of every relation in one call and
groups them by `Schema` and `Table`. `EnumValues` is the same for labels, and
D46 made it for dbtpl. A second kind for either is the same rows. The
test checks that the names in `Privilege.Policies` are the names that
`Policies` returns, and that `Type.Elements` is the labels of `EnumValues`
joined. Both texts keep their meaning.

## The values that psql prints and the models got wrong

| Item | What was wrong | What it is now |
| --- | --- | --- |
| `\dD` Check | `Domain.Constraints` held NOT NULL on release 17 and later, which stores the NOT NULL of a domain in `pg_constraint` with the type n | only the checks, as psql shows them. `Nullable` holds the NOT NULL |
| `\dFd+` Template | `TextSearchDictionary.Template` has no schema | new field `TemplateSchema`, as D147 did for `TextSearchConfig.Parser`. `Template` is unchanged |
| `\dAc` | one row for each class, which the model already returned | no change to the rows. See below |
| `\dAf` | `AppliesTo` was sorted by type name, and psql lists the types in the order of the catalog | `AppliesTo` is in the order of `pg_opclass` in the heap, which is what psql's statement returns. D147 said sorted, so D147 is amended |
| `\dAo` | the rows were ordered by strategy only | ordered as psql orders them. New `LeftType` and `RightType` |
| `\dAp` | `Function` has the argument types, and psql 18 prints the name alone for `\dAp` and the signature for `\dAp+` | new `FunctionName`. `Function` is unchanged |

`\dAc` returns every operator class, so gin has `jsonb_ops` and
`jsonb_path_ops` as two rows. I found no operator class that the statement
misses. 177 rows in the catalog of 18 are 177 rows, and the test counts them.
usql passed the type pattern in `name`, which matches the name of the class. The
missing row is the filter and not the statement. `OperatorClasses` and
`OperatorFamilies` now take `type`, a pattern for the input type that matches
the type name or `format_type`, as psql's second argument does.

Strategy and Purpose of `\dAo` and Number of `\dAp` are the same as psql
prints. Strategy is `amopstrategy`, Purpose is ordering or search, and Number is
`amprocnum`. The rows differed in order, which is fixed. `Operator` of `\dAo`
is `regoperator` in psql, which is the name with the argument types, and so it
is here. The sort of `\dAp` is by left type equal to right type first, then the
two types, then the number. The sort of `\dAo` is the same with the strategy.

The order of `AppliesTo` is the physical order of `pg_opclass`, written as
`ORDER BY c.ctid` in the aggregate. psql has no `ORDER BY` there, and the
server answers in that order for a catalog table of this size. The expression
states it and does not depend on the plan. CockroachDB has no `ctid`, and keeps
the order by type name.

## New fields on kinds that existed

| Kind | Field | Type | Source | Filled from |
| --- | --- | --- | --- | --- |
| Schemas | `Access` | `sql.Null[string]` | `nspacl`, one entry on each line | every release |
| Extensions | `DefaultVersion` | `sql.Null[string]` | `pg_available_extensions()`, as psql 18 reads it | every release. NULL when the control file is not on the server, and on CockroachDB |
| Databases | `LocaleProvider` | `sql.Null[string]` | `datlocprovider` as libc, icu or builtin | 15. libc below it |
| Databases | `Locale` | `sql.Null[string]` | `daticulocale`, and `datlocale` from 17 | 15. NULL for libc |
| Databases | `ICURules` | `sql.Null[string]` | `daticurules` | 16 |
| Operators | `Leakproof` | `bool` | `proleakproof` of `oprcode` | every release. False when there is no function |
| Casts | `LeakProof` | `sql.Null[bool]` | `proleakproof` of `castfunc` | held before this decision. See below |
| Publications | `GeneratedColumns` | `sql.Null[string]` | `pubgencols` as none or stored | 18. none below it |
| Subscriptions | `Binary`, `DisableOnError`, `PasswordRequired`, `RunAsOwner`, `Failover` | `sql.Null[bool]` | `subbinary` and the rest | 14, 15, 16, 16, 17 |
| Subscriptions | `Streaming`, `TwoPhase`, `Origin`, `SkipLSN` | `sql.Null[string]` | `substream`, `subtwophasestate`, `suborigin`, `subskiplsn` | 14, 15, 16, 15 |
| OperatorFamilyOperators | `SortFamily`, `Leakproof`, `LeftType`, `RightType`, `Visible`, `FamilySchema` | see the struct | `pg_amop` | every release |
| Sequences | `CacheSize` | `sql.Null[int64]` | `seqcache` | 10 |
| Indexes | `Definition`, `Using`, `ConstraintType`, `ConstraintDefinition`, `ConstraintPeriod`, `TableVisible` | see the struct | `pg_get_indexdef`, `pg_constraint` | see below |

Locale provider below 15 is libc and not NULL. The question the padding rule
asks is whether the value on the old release is unknown or is genuinely that
value. A database of release 14 has no other provider, so libc is the answer,
and psql prints it. The same holds for `GeneratedColumns` below 18, which
published none. The columns of a subscription are the other case. A
subscription of release 13 never chose binary or streaming, and the choice did
not exist, so those are NULL and `Field.Min` says from which release. psql
prints no such column below the release, and a NULL tells usql to print none.

`Streaming` is off, on or parallel for every release that has it. Release 14
and 15 hold a boolean and the statement spells it. `TwoPhase` is disabled,
pending or enabled, and psql prints the raw letter. The words are the ones that
the catalog documents for the letters.

`Cast.LeakProof` was never NULL for every row. It is NULL for a cast with no
function, which is a binary coercible cast and a cast with inout, and that is
69 rows of 235 on release 18. It is true for 51 and false for 115. psql prints
no for a NULL, because it joins `pg_proc` on the left. A caller that wants the
same reads the value as false when it is not valid. I did not change the type,
because the NULL says something that false hides. If usql saw all NULL, the
cause was not the model, and I cannot reproduce it.

## What `Index` carries

psql prints an index in the footer of `\d NAME` as the `pg_get_indexdef` text
after the first ` USING `, with a label for a primary key or a unique
constraint, and the text of `pg_get_constraintdef` for an exclusion constraint
and for a constraint WITHOUT OVERLAPS. The column `IndexColumn.Expression`
holds the text of one column and loses the operator class and the collation.

`Definition` is the whole statement, `pg_get_indexdef(oid, 0, true)`. `Using`
is what follows the first ` USING `, which is the text psql prints, so a caller
needs no search. The two are one call of a function that costs 75 microseconds,
because a lateral join makes it once for each index. `ConstraintType` is p, u
or x, and NULL for an index that no constraint owns. It comes from the join
that gives `Deferrable`, which D198 limits to those three types. A foreign key
names the unique index it reads, and the join leaves it out. `ConstraintDefinition`
is `pg_get_constraintdef` of the same constraint. `ConstraintPeriod` is
`conperiod` from release 18, and false below it for a constraint, because no
constraint was WITHOUT OVERLAPS. It is NULL for a plain index. usql prints the
constraint text when the type is x or the period is true, as psql does.

## The visibility flags

psql prints a relation as `name` when its schema is on the search path and as
`schema.name` when it is not. It does that through the `regclass` output
function and through `pg_*_is_visible`. These are the places in `describe.c`
of 18 that name a relation, an operator class or an operator family that way:
the parents of Inherits, the Child tables and the Partition of, the parent
and the table of `\dP`, the table of a statistics object, the classes and
families of `\dAc`, `\dAf`, `\dAo` and `\dAp`, and the tables of Referenced by.

The flag is `sql.Null[bool]`, from `pg_table_is_visible`, `pg_opclass_is_visible`
or `pg_opfamily_is_visible` of the object. It is on `Partition` (`TableVisible`,
`PartitionVisible`), `Inherit` (`Visible`, `ParentVisible`), `PartitionedTable`
(`ParentVisible`, `TableVisible`), `ExtendedStat` (`TableVisible`), `Index`
(`TableVisible`), `OperatorClass` (`Visible`, `FamilyVisible`),
`OperatorFamily` (`Visible`), and `OperatorFamilyOperator` and
`OperatorFamilyFunction` (`Visible`). The last two and `OperatorClass` also have
the schema of the family, which the rows did not name.

A flag and not a kind of search path, for three reasons. The flag answers for
each object, and a list of schemas cannot, because a table is not visible when
an earlier schema on the path holds a table of the same name, and the rule for
the temporary schema and for pg_catalog is the server's. It uses the search
path of the session that runs the statement, which is what psql does. It needs
no second statement, and D47 forbids one. A caller that wants the path itself
can read `SHOW search_path`.

The flags that I did not add are the ones psql does not print that way. The
foreign key text and the index text come from `pg_get_constraintdef` and
`pg_get_indexdef`, and the server decides the schema in them. A policy, a rule
and a publication name no other relation. The `Referenced by` section lists
constraints on other tables that name this one, and no kind reads it, because
`Constraints` filters by the table that owns the constraint. That is a gap,
and it is in `docs/BACKLOG.md`.

The flag is NULL where the server cannot say, such as the table of an index that
is not partitioned. CockroachDB answers NULL where a statement is shared and it
lacks the function.

## The parts that are refused to a role

`subconninfo` is the one column of `pg_subscription` that only a superuser
can read, because the table grants the others to public by column. A statement
that names it fails for every other role before it runs. A column of
`Subscriptions` makes the whole kind fail for a role that reads the
other columns today. So `SubscriptionConnections` is a kind of its own
with `Name` and `Conninfo`, and a role that cannot read it gets the error of
that kind alone. psql has the same limit, and its verbose `\dRs+` fails for such
a role. The privilege parity test records `subscription_connections refused`
for the owner and the grantee on every release that has subscriptions.

## Text search parser functions

`\dFp+` prints five rows of Method, Function and Description. They are five
columns of `pg_ts_parser`, and each function has a comment, so a field for each
is ten fields. A kind is shorter: `TextSearchParserFunction` has `Schema`,
`Parser`, `Method` (start, token, end, headline or lextype), `Ordinal`,
`Function` as `regproc` writes it, and `Comment`. The methods are keys and not
the words psql prints, because the words are translated by psql and are for
the consumer to choose. `\dFp+` also prints the token types of the parser. That
table is not in this decision, and `ts_token_type` is how to read it.

## Fixed in the types of the two decisions

`PartitionedTable.Type` was table or index and is now `partitioned table` or
`partitioned index`, the words of psql. `Privilege.Type` was table for a
partitioned table and is now `partitioned table`. These change the value of an
existing field, as D198 did for `Table.Type`. A caller that compared either
with `table` must compare with the new word. The partitioned index has no entry
in `Privilege`, because an index has no privileges.

## What D199 left open

`ForeignTable.Options` is settled by `ForeignOptions`, above.

`Args` now has `ParentSchema`, `PartitionSchema` and `WithImplicit`, which are
the parameters `parent_schema`, `partition_schema` and `with_implicit`. It was a
small change, three fields in `filter.go` and a test of the map. A test in the
PostgreSQL model builds the statements of Inherits, Partitions and
PublicationTables with them on every release that answers.

## The cost

Measured with `EXPLAIN (ANALYZE)` on PostgreSQL 15.19, best of three, in a
scratch database of 22002 relations in one schema, with 5000 tables, 8025
indexes, 2005 schemas, 2799 operators, 1000 operator classes and families, 1001
text search parsers, 2000 foreign tables, 500 servers with a user mapping each,
2002 column privilege entries, 200 publications and 200 subscriptions. The
times are milliseconds of execution.

| Statement | Unfiltered | With the filter |
| --- | --- | --- |
| Schemas, 2003 rows | 6.3 | 0.03 by name |
| Extensions | 0.24 | 0.23 by name |
| Operators, 2799 rows with the system | 28 | 0.10 for one schema |
| Publications, 204 rows | 0.57 | 0.03 |
| Subscriptions, 201 rows | 0.39 | 0.03 |
| SubscriptionConnections, 201 rows | 0.03 | |
| TextSearchParserFunctions, 5000 rows | 17 | 0.05 for one parser |
| Sequences, 5005 rows | 5966 | 0.19 by name |
| Indexes, 8025 rows | 730 | 0.31 for one table |
| Partitions, 1000 rows | 4.1 | 1.6 for one partition by name |
| Inherits, 2006 rows | 5.5 | 0.12 for one child, 2.4 for the 1000 children of one parent |
| PartitionedTables | 8.5 | 5.8 by name |
| ColumnPrivileges, 2002 rows | 15 | 0.03 for one table |
| ForeignOptions, 8009 rows | 13 | 0.03 for one foreign table, 0.01 for one server |
| OperatorClasses, 1177 rows | 1.5 | 0.08 for gin and jsonb |
| OperatorFamilies, 1146 rows | 37 | 0.05 for one |
| OperatorFamilyOperators, 1945 rows | 4.0 | 0.16 for one family |
| OperatorFamilyFunctions, 1696 rows | 2.6 | 0.06 for one family |

Every cost grows with the rows the statement returns, and a filter on the
object narrows the rows before any function runs. Nothing scans the whole
catalog for one object.

Two numbers need a word. `Indexes` costs about 75 microseconds more for each
row, because `pg_get_indexdef` is dear. A call of that function alone on all
12000 indexes of the catalog takes 920 ms. A caller that lists every index of
a large catalog pays it, and a caller that describes one table pays 0.3 ms.
`OperatorFamilies` pays the same kind of cost for the heap order of its
aggregate. The sequences are slow for a reason older than this decision. The
`owned_by` subselect reads `pg_depend` by `objid` alone, and the index of that
catalog starts with `classid`, so each sequence scans the table. It was like
that before. A fix is to add `d.classid = 'pg_class'::regclass` and
`d.refclassid = 'pg_class'::regclass` to it. I did not change it, because it
is outside this request, and I record it here for the next change.

## Parity and conformance

`TestPrivilegeParity` on PostgreSQL 9.6, 12, 15 and 18 records two new lines.
`subscription_connections refused` is for the owner and the grantee on every
release from 10, as above. `user_mappings the same rows with different values`
is for the owner and the grantee too, and it comes from the new fixture
mapping, whose options the two roles cannot read. That difference is a
property of `pg_user_mappings`, and `UserMappings` had it before. Every other
new kind and field gives the same rows to the administrator, the owner and the
grantee, because the catalog tables and the functions behind them check no
privilege. The conformance golden is unchanged. Running `-update` with one
server up also moves the Cassandra section of `test/testdata/parity.txt`,
which has nothing to do with this change, so I put it back.

## CockroachDB

CockroachDB shares the statements for Tables, Schemas, Indexes, Casts, Types,
Domains, Operators, Privileges, Extensions, Publications, Subscriptions, the
text search kinds that exist there, OperatorClasses, OperatorFamilies,
OperatorFamilyFunctions, Sequences and PartitionedTables, and so it shares the
fields I added to them. The four new kinds are not shared, so they are not
supported there, and neither is `OperatorFamilyOperators`.

I measured what the shared statements need on 26.3.2. `pg_table_is_visible`,
`pg_get_indexdef` and `pg_get_constraintdef` exist and are used as they are.
Three things are missing, and each statement gates on the CockroachDB key as
D199 did. `pg_available_extensions()` is missing, so `DefaultVersion` answers
NULL. `pg_opclass_is_visible` and `pg_opfamily_is_visible` are missing, so the
flags of the operator kinds answer NULL. A `ctid` is missing, so `AppliesTo`
keeps the order by type name that the statement had before this decision.

The fixture steps that build the new objects are in the left map of
`models/cockroachdb/fixture` with the reason "not measured on CockroachDB",
and the new tests skip on them. I ran the test module on 26.2.7 and 26.3.2, and
both pass.

## What was left out

The index footer of psql 18 also prints the tablespace of an index and orders
the indexes with the primary key first. `Index` holds neither. The first is one
more field, and the second is an order the caller can apply, because `Primary`
is a field.

`Subscriptions` does not narrow to the current database as psql 18 does. A
subscription belongs to one database and `pg_subscription` is a shared catalog,
so the kind lists those of every database. That is the way it was.

The token types of `\dFp+` and the Referenced by section of `\d` have no kind.
They are in `docs/BACKLOG.md`, with the tablespace of an index and the database
filter of subscriptions.

The fixture has no invalid index, no detached partition and no constraint
WITHOUT OVERLAPS, because the first needs a failed concurrent build, the second
needs a pending detach, and the third needs the `btree_gist` extension for a
key of an ordinary type. The tests read `ConstraintPeriod` as false.
