# D73. Presto is its own dialect, and not a flavor of Trino

Status: Decided.

`models/presto` is a model of its own. Presto and Trino do not share a dialect
the way MariaDB and MySQL do.

D66 guessed the other way, saying Presto was "probably a flavor key on the
Trino model rather than a model". That guess was made before either existed.
The measurement changed it.

## What they share

`system.jdbc` exactly: all thirteen tables, the same columns, the same
meanings. The same SQL dialect, the same `array_agg` and `array_join`, the
same strict typing that refuses `boolean = integer`, and the same five
catalogs in the official image including a writable `memory`. They are the
same program forked in 2019, when the original authors left Facebook and
renamed PrestoSQL to Trino, and it shows.

## What they do not

Measured on Trino 483 and Presto 0.299, which was the newest of each.

| | Trino | Presto |
| --- | --- | --- |
| `version()` | answers | not registered |
| `node_version` | `483` | `0.299-7d50721` |
| `system.metadata.table_comments` | present | absent, and no table comment is readable at all |
| `current_catalog`, `current_schema` | both resolve | neither resolves |
| `system.jdbc.columns.remarks` | carries the comment | always NULL |
| `information_schema.columns` | 8 columns | 13, including `comment` |
| `memory` with `NOT NULL` | from 476 | refused |
| role views on `memory` | answer nothing | raise `NOT_SUPPORTED` |
| Go driver | `trinodb/trino-go-client` | `prestodb/presto-go-client` |
| DSN the driver takes | `http://` with query parameters | `presto://` with a path |
| `CREATE CATALOG` | in the grammar | not |

## Why that means two dialects and not one with gates

The version query settles it on its own. A dialect registers one
`VersionQuery`, and Trino's `version()` does not exist on Presto. The
numbering cannot be compared either: Presto is `0.299` and Trino is `483`, so
there is no version to gate on, only a product to branch on.

A fragment that branches on which product it is talking to is not a version
fragment. Two models asked independently and both said the same thing. Gemini
put it best: once you are branching on engine identity rather than engine
version, you no longer have one dialect with versioned fragments, you have two
dialects sharing a struct. DeepSeek added the maintenance case, that a reader
would have to hold both products in their head at every object.

MariaDB and MySQL are not the counter example they look like. They share
decades rather than six years, their catalogs still agree on nearly
everything, and they use the same Go driver. The fragments there are genuinely
about version, which is what D44 is for.

## What Presto answers

9 of the 55, against Trino's 13. The four it does not are absences in the
product rather than gaps in the model. Comments has no source. CurrentSchema
has no expression. Roles and RoleGrants read the standard views and the memory
connector raises rather than answering nothing, and D34 says a query dbmeta
offers must run, so they are not offered.

Its conformance section is its own, and that is the second reason a shared
dialect would not have worked. Presto's `memory` connector refuses `NOT NULL`
on the newest release there is, so every column in its fixture is nullable
where Trino's is not. One section cannot describe both.

## What this does not decide

Whether a future product that forks from one of them is a flavor. The test is
the one applied here: if telling the two apart needs a product branch rather
than a version gate, they are two dialects.
