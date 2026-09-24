# Metadata Query Survey

This document surveys the two sources that D9 names as models. It lists what
`psql` describes, what `information_schema` describes, and where the two meet.

Read it before designing the object set. It exists so that the design starts
from what the sources actually contain rather than from the 15 object types
that `usql` happens to model today.

## How the numbers were produced

Everything here was measured from the PostgreSQL source, not recalled. The
checkout is at `/home/ken/src/postgres`, at `REL_19_BETA1-1062-gd9de60c5e47` on
`master`, which is release 20 under development.

The `psql` side is `src/bin/psql/describe.c`, 7400 lines. The
`information_schema` side is `src/backend/catalog/information_schema.sql`, 3011
lines.

An earlier version of this document measured a checkout at
`REL_15_BETA2-740-ge59a67fb8f`. Every number changed. See the section on what
release 20 removed, because the change is not only a drift in counts.

Count the version gates:

```bash
grep -oE 'pset\.sversion *(<|>=) *[0-9]+' src/bin/psql/describe.c | sort | uniq -c
```

List the views:

```bash
grep -oE '^CREATE VIEW [a-z_]+' src/backend/catalog/information_schema.sql
```

Rerun both when the checkout moves to a newer release. The numbers below will
change.

## Part 1. What psql describes

`describe.c` holds 54 function definitions. Five are helpers, so there are 49
entry points. Each one backs a backslash command.

One entry point is new since the release 15 tree: `describeRoleGrants`, which
backs `\drg`. Nothing was removed.

### Tables and their contents

| Command | Function | Gates | Reads |
| --- | --- | --- | --- |
| `\d name` | describeOneTableDetails | 21 | pg_class, pg_attribute, pg_attrdef, pg_constraint, pg_index, pg_am, pg_collation |
| `\d` `\dt` `\di` `\dv` `\dm` `\ds` | listTables | 2 | pg_class, pg_namespace, pg_index, pg_am |
| `\dP` | listPartitionedTables | 1 | pg_class, pg_inherits, pg_partition_tree |
| `\det` | listForeignTables | 0 | pg_foreign_table, pg_foreign_server, pg_class |
| `\dX` | listExtendedStats | 2 | pg_statistic_ext, pg_attribute |

### Routines and types

| Command | Function | Gates | Reads |
| --- | --- | --- | --- |
| `\df` | describeFunctions | 8 | pg_proc, pg_language, pg_get_function_arguments |
| `\da` | describeAggregates | 1 | pg_proc, pg_namespace |
| `\dT` | describeTypes | 0 | pg_type, pg_enum, pg_class |
| `\dD` | listDomains | 0 | pg_type, pg_constraint, pg_collation |
| `\do` | describeOperators | 0 | pg_operator, pg_type |
| `\dC` | listCasts | 0 | pg_cast, pg_proc, pg_type |
| `\dL` | listLanguages | 0 | pg_language |

### Namespaces, databases and storage

| Command | Function | Gates | Reads |
| --- | --- | --- | --- |
| `\dn` | listSchemas | 1 | pg_namespace, pg_publication_namespace |
| `\l` | listAllDbs | 4 | pg_database, pg_tablespace |
| `\db` | describeTablespaces | 0 | pg_tablespace |
| `\dA` | describeAccessMethods | 0 | pg_am |
| `\dO` | listCollations | 4 | pg_collation |
| `\dc` | listConversions | 0 | pg_conversion |
| `\dl` | listLargeObjects | 0 | pg_largeobject_metadata |

### Users and privileges

| Command | Function | Gates | Reads |
| --- | --- | --- | --- |
| `\z` `\dp` | permissionsList | 0 | pg_class, pg_policy, pg_roles, pg_attribute |
| `\du` `\dg` | describeRoles | 0 | pg_roles, pg_auth_members |
| `\drg` | describeRoleGrants | 1 | pg_auth_members, pg_roles |
| `\ddp` | listDefaultACLs | 0 | pg_default_acl |
| `\drds` | listDbRoleSettings | 0 | pg_db_role_setting |

### Foreign data

| Command | Function | Gates | Reads |
| --- | --- | --- | --- |
| `\dew` | listForeignDataWrappers | 0 | pg_foreign_data_wrapper |
| `\des` | listForeignServers | 0 | pg_foreign_server |
| `\deu` | listUserMappings | 0 | pg_user_mappings |

### Replication

| Command | Function | Gates | Reads |
| --- | --- | --- | --- |
| `\dRp` | listPublications | 4 | pg_publication |
| `\dRp+` | describePublications | 9 | pg_publication, pg_publication_namespace, pg_class |
| `\dRs` | describeSubscriptions | 8 | pg_subscription |

### Text search

| Command | Function | Gates | Reads |
| --- | --- | --- | --- |
| `\dFp` | listTSParsers, listTSParsersVerbose, describeOneTSParser | 0 | pg_ts_parser |
| `\dFd` | listTSDictionaries | 0 | pg_ts_dict, pg_ts_template |
| `\dFt` | listTSTemplates | 0 | pg_ts_template |
| `\dF` | listTSConfigs, listTSConfigsVerbose, describeOneTSConfig | 0 | pg_ts_config, pg_ts_config_map |

### Operator families

| Command | Function | Gates | Reads |
| --- | --- | --- | --- |
| `\dAc` | listOperatorClasses | 0 | pg_opclass, pg_am |
| `\dAf` | listOperatorFamilies | 0 | pg_opfamily, pg_am |
| `\dAo` | listOpFamilyOperators | 0 | pg_amop, pg_opfamily |
| `\dAp` | listOpFamilyFunctions | 0 | pg_amproc, pg_opfamily |

### Everything else

| Command | Function | Gates | Reads |
| --- | --- | --- | --- |
| `\dx` | listExtensions | 0 | pg_extension |
| `\dx+` | listExtensionContents, listOneExtensionContents | 0 | pg_extension, pg_depend |
| `\dy` | listEventTriggers | 0 | pg_event_trigger |
| `\dconfig` | describeConfigurationParameters | 2 | pg_settings, pg_parameter_acl |
| `\dd` | objectDescription | 0 | pg_description |

## Part 2. What release 20 removed, and why it matters

Read this part before the numbers in it, because one upstream change reframes
D20.

On 2026-07-02, commit `831bec45924`, titled "Remove psql support for pre-v10
servers", deleted every code path in `psql` for a server older than release 10.
Its message states the policy:

> Our current policy is to support at least 10 previous major versions, so this
> bumps the minimum to v10 for the v20 release.

The effect on `describe.c` is visible in the gate values. There is no longer a
single gate below 110000. The oldest is `< 110000` and the newest is `>=
190000`, so the gates now span release 11 to release 19. In the release 15 tree
they spanned 9.3 to 16.

Three consequences for this project.

The current source cannot tell you what release 9.6 needs. D20 sets the
PostgreSQL floor at 9.6 and phase 1 says to translate from the PostgreSQL
source. Those two instructions no longer meet in one tree. The 9.6 code paths
were deleted in July and live only in a release 15 or older checkout.

Supporting 9.6 now means supporting releases that `psql` itself does not.
D20's reason for 9.6 was compatibility with `psql`, and `psql` has moved.
Whatever dbmeta does below release 10 is dbmeta's own work, translated from an
older tree, and it cannot be checked against current upstream behavior.

PostgreSQL has a written policy that dbmeta can adopt. Ten previous major
versions is mechanical, it is what upstream follows, and D21 already prefers a
rule to a judgment. Against release 20 it gives a floor of release 10.

This does not overturn D20. Ken decided 9.6 knowing it cost roughly five times
the version work, and the argument that PostgreSQL is the specification still
holds. What has changed is that the cost is now also a fidelity cost, not only
an effort cost. That is new information and it belongs in front of him.

## Part 3. Where the version cost sits

There are 68 version gates, down from 76 in the release 15 tree. They are not
spread evenly. 35 of the 49 entry points have none at all.

| Function | Gates | Share |
| --- | --- | --- |
| describeOneTableDetails | 21 | 31% |
| describePublications | 9 | 13% |
| describeSubscriptions | 8 | 12% |
| describeFunctions | 8 | 12% |
| listPublications | 4 | 6% |
| listCollations | 4 | 6% |
| listAllDbs | 4 | 6% |
| listTables | 2 | 3% |
| listExtendedStats | 2 | 3% |
| describeConfigurationParameters | 2 | 3% |
| listSchemas, listPartitionedTables, describeRoleGrants, describeAggregates | 1 each | 6% |
| thirty five others | 0 | 0% |

`describeOneTableDetails` is still the hardest single piece at 31%, though less
dominant than the 39% it held in the release 15 tree. The top four now hold
68%.

Replication moved up sharply. `describePublications` and
`describeSubscriptions` together went from 10 gates to 17, which is what four
releases of logical replication work looks like. Plan for that family to keep
moving.

### Live gates by floor

Measured against the current tree, so these counts describe the release 11 to
19 range only. A floor below 11 needs an older checkout to measure at all.

| Floor | Live gates | Collapse |
| --- | --- | --- |
| 10 | 68 | 0 |
| 11 | 55 | 13 |
| 12 | 44 | 24 |
| 13 | 40 | 28 |
| 14 | 34 | 34 |
| 15 | 20 | 48 |
| 16 | 15 | 53 |
| 17 | 12 | 56 |
| 18 | 9 | 59 |

### What follows for the plan

First, `\d name` is still the hard one. It is one entry point, it reads seven
catalogs, and it carries 21 gates. Schedule it as its own piece of work.

Second, most of the work is not version work. 35 entry points need no fragments
at all, so for those the D8 machinery reduces to a single alternative and
`Always` covers them.

Third, build a zero gate entry point first, such as `\dn` or `\db`, to prove
the pipeline end to end with nothing else in the way. Then build `\d name`,
because everything hard is in it.

## Part 4. What information_schema describes

`information_schema.sql` defines 65 views. Five are internal helpers whose
names begin with an underscore, so 60 are public.

Column counts for the core views, which show how much detail the standard
carries:

| View | Columns |
| --- | --- |
| routines | 82 |
| columns | 44 |
| parameters | 36 |
| domains | 27 |
| table_constraints | 22 |
| triggers | 17 |
| key_column_usage | 15 |
| tables | 12 |
| sequences | 12 |
| views | 10 |
| referential_constraints | 9 |
| table_privileges | 8 |
| column_privileges | 8 |
| character_sets | 8 |
| check_constraints | 8 |
| schemata | 7 |
| collations | 4 |

The full list of public views, by family:

Tables and columns: `tables`, `columns`, `views`, `attributes`,
`column_options`, `column_udt_usage`, `column_domain_usage`,
`column_column_usage`, `element_types`.

Constraints: `table_constraints`, `key_column_usage`, `check_constraints`,
`referential_constraints`, `constraint_column_usage`, `constraint_table_usage`,
`domain_constraints`, `check_constraint_routine_usage`,
`triggered_update_columns`.

Routines: `routines`, `parameters`, `routine_column_usage`,
`routine_routine_usage`, `routine_sequence_usage`, `routine_table_usage`.

Types and domains: `domains`, `domain_udt_usage`, `user_defined_types`,
`data_type_privileges`, `transforms`.

Schemas and catalogs: `schemata`, `information_schema_catalog_name`,
`sequences`, `character_sets`, `collations`,
`collation_character_set_applicability`.

Privileges: `table_privileges`, `column_privileges`, `routine_privileges`,
`usage_privileges`, `udt_privileges`, `role_table_grants`,
`role_column_grants`, `role_routine_grants`, `role_usage_grants`,
`role_udt_grants`, `applicable_roles`, `enabled_roles`,
`administrable_role_authorizations`.

Foreign data: `foreign_data_wrappers`, `foreign_data_wrapper_options`,
`foreign_servers`, `foreign_server_options`, `foreign_tables`,
`foreign_table_options`, `user_mappings`, `user_mapping_options`.

Views and usage: `view_column_usage`, `view_routine_usage`,
`view_table_usage`.

Triggers: `triggers`.

## Part 5. The two side by side

### psql commands that information_schema can answer

Nine, and none of them completely.

| psql | information_schema | What is missing |
| --- | --- | --- |
| `\dt` list tables | `tables` | No size, no owner, no access method, no index listing. `table_type` has no matview. |
| `\d name` columns | `columns` | No storage, no statistics target, no index, no trigger, no partition bound. |
| `\d name` constraints | `table_constraints`, `key_column_usage`, `check_constraints`, `referential_constraints` | No exclusion constraint, and no constraint expression beyond a check clause. |
| `\dn` schemas | `schemata` | Shows only schemas the user owns on some databases. No comment. |
| `\df` functions | `routines`, `parameters` | No aggregate, no window function, no procedural language detail. |
| `\dD` domains | `domains`, `domain_constraints` | No comment. |
| `\z` privileges | `table_privileges`, `column_privileges`, `routine_privileges`, `usage_privileges` | No row level policy, no default ACL, one row per grant rather than the psql summary. |
| `\dO` collations | `collations` | Four columns against psql's provider, encoding and determinism. |
| `\des` `\dew` `\deu` `\det` foreign data | `foreign_servers`, `foreign_data_wrappers`, `user_mappings`, `foreign_tables` and their option views | Close to complete. This is the best covered family. |

### psql commands with no information_schema equivalent at all

Around thirty. Every one describes something the SQL standard does not define.

Extensions `\dx`. Publications and subscriptions `\dRp` `\dRs`. Operators
`\do`. Operator classes and families `\dAc` `\dAf` `\dAo` `\dAp`. Access
methods `\dA`. Text search, all four commands. Casts `\dC`. Languages `\dL`.
Conversions `\dc`. Tablespaces `\db`. Large objects `\dl`. Event triggers
`\dy`. Extended statistics `\dX`. Default ACLs `\ddp`. Role settings `\drds`.
Configuration parameters `\dconfig`. Comments `\dd`. Roles `\du` in any useful
form. Databases `\l`, since `information_schema_catalog_name` names only the
current one.

### information_schema views with no psql command

A handful, and they are not nothing.

`triggers`. psql shows triggers inside `\d name` rather than as a command of
its own, so there is no list form.

`referential_constraints`. psql shows foreign keys inside `\d name`, and the
match, update and delete rules are in the constraint definition text rather
than in columns.

`character_sets`, `transforms`, `attributes`, and the `*_usage` views that
record which view or routine depends on what.

## Part 6. What this means for a generic implementation

Five conclusions follow from the tables above.

### The two models do not overlap, they stack

`information_schema` answers the standard relational core: tables, columns,
constraints, routines, privileges, domains, views, foreign data. That is nine
of forty eight psql commands and it is the part every database has.

`psql` answers all of that plus everything PostgreSQL adds. The extra is not a
long tail of detail, it is thirty whole object kinds.

So the API is not a compromise between the two. It is the psql object set,
where `information_schema` is one way to answer the nine that overlap. D9
already ranks them this way and the survey supports it.

### psql never reads information_schema

Worth stating because it is easy to assume otherwise. `describe.c` mentions
`information_schema` seventeen times, and every one is
`n.nspname <> 'information_schema'`, excluding it from results as a system
schema. psql reads `pg_catalog` and nothing else.

A database that offers both will usually have a better native answer, and the
survey is the reason to prefer it. `information_schema` is the fallback for
databases that have nothing else, which is what D9 says.

### Most databases will answer a small part of the object set

Expect a table with many empty cells, and design for it rather than being
surprised. A database with only `information_schema` answers nine of forty
eight. This is exactly the case D34 is for, and it is why an empty result must
never stand in for "cannot ask".

It also confirms the narrowing of D9. If the ranking required every database to
answer all forty eight, almost every cell would be an error.

### The column detail is in information_schema, not in psql

The direction is the opposite of what the object counts suggest.
`information_schema.routines` has 82 columns and `columns` has 44, which is far
more per object than psql prints, because psql prints for a person and the
standard records for a program.

`dbtpl` is a program and needs that detail. `usql` prints for a person. The two
consumers want opposite things from the same object, and the API must carry the
detail and let `usql` select from it.

### Start at both ends

D13 says models before the API. The survey says which models.

Start with a zero gate command that both models can answer, such as `\dn`
schemas, to prove the pipeline with nothing hard in it.

Then do `\d name`, which is 31% of the version work, reads seven catalogs, and
has the weakest `information_schema` coverage of anything that matters. If the
design survives `\d name` at the oldest supported release and at the newest,
it will survive the rest.

Do not start in the middle. The middle is thirty five entry points that are
mostly a single query with no gates, and they will teach you nothing about
whether the design works.

### Read the right tree for the right release

This is new and it is easy to get wrong. The PostgreSQL source describes only
the releases the current `psql` still supports. Part 2 explains why.

Translating a query for release 9.6 or 10 means checking out a release 15 or
older tree and reading `describe.c` there. Translating a query for release 18
or 19 means the current tree. A single checkout cannot answer both.

Record which tree each fragment was translated from, next to the fragment. A
reader who cannot tell which source a gate came from cannot check it.
