# D147. The fields psql prints that usql needed

Status: Decided, amended by D201.

## The decision

usql's W31 wires up every psql describe command in `COMMANDS.md`, and it
found facts that psql prints and dbmeta did not return. Ken chose on
2026-09-30 to add all of them, and to add one kind for the mappings that
`\dF+` prints.

These fields are new:

| Field | What it holds | psql prints it in |
| --- | --- | --- |
| `Function.Definition` | the whole statement that makes the routine | `\sf` |
| `Setting.Display` | the value with its unit, such as 128MB | `\dconfig` |
| `Type.Size` | the internal length in bytes, `var`, or `tuple` | `\dT+` |
| `OperatorClass.StorageType` | the type the index stores, where it differs from the input type | `\dAc+` |
| `OperatorFamily.AppliesTo` | the input types of the family's operator classes | `\dAf` |
| `Collation.Rules` | the tailoring rules of an ICU collation | `\dO+` |
| `ExtendedStat.Definition` | the columns and expressions, and their table | `\dX` |
| `ExtendedStat.Ndistinct`, `Dependencies` and `MCV` | which kinds the object was made with | `\dX` |
| `TextSearchConfig.ParserSchema` | the schema of the parser, which usql asked for after W31 | `\dF+` |

The new kind is `dbmeta.TextSearchConfigMaps`, which yields
`TextSearchConfigMap`. psql joins the dictionaries for one token into one
line. D47 makes a child of an object its own kind with flat rows, so this
kind returns one row for each dictionary, with its position. That makes 56
kinds, and 49 of them come from psql.

## Why each field is filled where it is

`Function.Definition` holds a statement and never a body. PostgreSQL fills it
from `pg_get_functiondef`. That function refuses an aggregate, and psql's
`\sf` refuses one too, so an aggregate has no definition. SQL Server,
ClickHouse, SAP HANA and Exasol keep the whole statement in a catalog column,
and those models returned it as `Source` before this field existed. Ken chose
on 2026-09-30 that they return it in `Definition` alone and pad `Source`, so
that `Source` means one thing in every model: what the product keeps apart
from the whole statement. usql reads `Source` only for the Source code column
of `\df+`, which is empty for those products in psql too. Every other model
keeps only a body, or nothing, and pads `Definition` with NULL. Oracle's
`DBMS_METADATA.GET_DDL` builds the statement, and it runs statements of its
own for each object, which is a second statement for each row. D47 does not
allow that, so Oracle pads. Snowflake's `GET_DDL` is not measured, because the
Snowflake model has not run (D144), so Snowflake pads until it runs.

`Setting.Display` is `current_setting` on PostgreSQL. Every other model has
one form of a value, and pads.

`Type.Size` follows psql's expression over `typlen` and `typrelid`. CrateDB
has the same catalog and uses the same expression. DuckDB records the size of
a value in memory, which is what it returns. A Cassandra type is always
composite, so its size is always `tuple`. Every other model records a largest
declared size or nothing, and pads, because a declared size is not an
internal length.

`Collation.Rules` arrived in PostgreSQL 16, and the field carries that gate.
No other product records tailoring rules.

`ExtendedStat.Definition` uses `pg_get_statisticsobjdef_columns` from
PostgreSQL 14, which added statistics on expressions. Below 14 it names the
columns from `stxkeys`, as psql does. The MCV kind arrived in 12. Below 12 no
object has it, so the field is false and not padded. SQL Server and SAP HANA
answer from their own catalogs. Their `Kinds` field held the columns before
this field existed, and Ken chose on 2026-09-30 that it holds the kind alone.
On SQL Server it is `d`, PostgreSQL's letter for ndistinct, which is the one
kind SQL Server builds over several columns. On HANA it is the statistics type
in lower case, such as `histogram`, because HANA's types do not match
PostgreSQL's letters one for one. `Ndistinct` and `MCV` say where one does.

## What CockroachDB lacks

CockroachDB has no `pg_get_statisticsobjdef_columns`, so it has its own
ExtendedStats statement, which names the columns from `stxkeys`. It has no
statistics on an expression, so the columns are the whole definition. It has
no `ts_token_type` and keeps `pg_ts_config` empty, so it does not answer
`TextSearchConfigMaps`. It answers 54 of the 56.

## The cost

Each new field is one column of the statement that already exists. Two are
correlated subqueries. `OperatorFamily.AppliesTo` reads `pg_opclass` for each
family, and `pg_opclass` holds the operator classes that are installed, which
does not grow with the tables. `ExtendedStat.Definition` below 14 reads
`pg_attribute` by its index on the relation and the column number, for each
statistics object. `pg_get_functiondef` reads the one `pg_proc` row it is
given and runs no statement of its own, so it stays inside the rule of D47.
