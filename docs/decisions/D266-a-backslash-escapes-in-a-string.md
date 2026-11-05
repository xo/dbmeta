# D266. A backslash escapes in a string for some dialects

Status: Amends D143.

A client that splits text into statements must know where a string ends. In
MySQL a backslash inside `'...'` escapes the next character, so `'it\'s'` is
one string. In PostgreSQL the same text is the string `'it\'` followed by `s'`.
A client that reads the wrong rule either joins two statements or cuts one.

usql asked on 2026-11-05 whether the rule is for the MySQL family only. Ken
decided that dbmeta holds the answer for every dialect, from what each product
does, and that usql keeps no list of its own.

## Decision

`Syntax.BackslashEscapes` is true when a backslash in a string between single
or double quotes escapes the next character. The word is positive, as
`Syntax.Brackets` is (D264). `Dialect.Classify` reads it, so the text after an
escaped quote is still inside the string.

The probe is `select 'a\\b'` and `select 'a\'b'`. A product that escapes
answers `a\b` and `a'b`. One that does not refuses the second statement.

| How it was decided | Dialects |
| --- | --- |
| Run on a server | MySQL, and MariaDB, TiDB, SingleStore and Vitess with it, ClickHouse, Databend, Impala, YDB, Neo4j, ArangoDB, SurrealDB, Couchbase and the Spanner emulator |
| The parser accepted the escaped quote | InfluxQL |
| Gemini and DeepSeek agreed, and no server was available | Snowflake, BigQuery, Databricks, MaxCompute, Redshift, Hive and Cosmos DB |

Hive and the hosted services were not run here. Hive was answered by Gemini,
and DeepSeek timed out on it. Redshift is the one to check first if a report
disagrees.

Every other model reads a backslash as an ordinary character, as psql does.
