# D184. DynamoDB has no catalog that a statement can reach

Status: Decided.

## The decision

Every DynamoDB kind is unanswered. There is no DynamoDB model, no `Binding`
and no change to the root package. The two DynamoDB Local releases stay Staged
with the cadence Tested (D119, D120). This is the same answer as D180 gave for
VoltDB, and it waits for Ken to confirm it.

## Why

PartiQL, which is the only SQL that DynamoDB speaks, reads and writes items in
a table that already exists. It has no relation that holds metadata. The
sources of metadata are the native calls `ListTables`, `DescribeTable`,
`DescribeTimeToLive` and their relatives. They are HTTP requests with AWS
Signature Version 4, and a `Queryer` cannot send one. The driver is the dbimp
driver `dynamodb`, and it sends one thing only, `ExecuteStatement`. dbimp
reads only (its D163), so it has no way to run a native call either.

## What was measured

Servers: `dynamodb-3.2.0` and `dynamodb-3.3.1`, one at a time, with the entry
in `container/dynamodb.go`, through the dbimp driver and the DSN the entry
prints. The fixture was one table, `author`, made with native `CreateTable` and
`PutItem` calls. Both releases gave the same answers.

| Statement | Answer |
| --- | --- |
| `SELECT * FROM author` | one row, and the row is one map column |
| `SELECT * FROM author;` | the same row. A trailing semicolon is accepted |
| `SELECT * FROM "author";` and `... WHERE id = 1;` | the same row |
| `SELECT * FROM information_schema.tables` | ResourceNotFoundException, a non-existent table |
| `SELECT * FROM sys.tables`, `"dynamodb"."tables"`, `"TABLES"`, `"SYSTEM.TABLES"`, `"_meta"` | the same refusal |
| `SELECT * FROM "$tables"` and `"author"."$table"` | ValidationException, a table name can hold only letters, digits and `-_.` |
| `SELECT * FROM author.idx` | ValidationException, the table has no index named idx. A name after a dot is an index |
| `LIST TABLES`, `SHOW TABLES`, `DESCRIBE TABLE author` | ValidationException, not well formed |
| `SELECT version()` and `SELECT 1` | ValidationException, not well formed |

So the semicolon result is this: DynamoDB Local does not refuse a trailing
semicolon on a SELECT of a fixture table. A model does not set
`TerminatorStripped`. dbimp recorded the same for a comment and for two
statements in one text, which fail.

A name given as a table is read as a table, so every candidate source fails as
a missing table. There is no name that reaches a catalog. The native calls that
do (`ListTables` and the others) answered over HTTP as expected, and DynamoDB
Local refuses `DescribeEndpoints`. dbimp's record in `docs/DYNAMODB.md` says the
same, and so does D66 of this project.

### The version

No statement gives it. `SELECT version()` is a syntax error. usql's driver sets
a `Version` that returns the fixed text "Amazon DynamoDB" and runs nothing. By
D183 a caller who needs a release passes it to `ParseVersion`, and DynamoDB
Local does not publish one at all.

### What DynamoDB Local cannot show

- DynamoDB Local checks no key, so there is no principal with fewer rights, and
  rule 16 has nothing to compare. The real service has IAM, which a PartiQL
  statement can be refused by, and a measurement here says nothing about it.
- Local ignores the region and keeps one store with `-sharedDb`. The real
  service has a store for each region and each account.
- Local does not enforce throughput, and it ignores several table settings
  that the real service reports, such as the table class and the backup state.
- The real service can refuse a statement that Local accepts. For example, a
  PartiQL SELECT that is not a query on the key is a scan on both, and the
  real one can cost money.

### The cost check

None. No query exists, so there is nothing to give `EXPLAIN`. For reference, a
walk over `ListTables` pages by 100 names, and a `DescribeTable` for each name
is one more call, so a model of native calls costs a round trip for each
table. Rule 13 forbids that shape.

### What two models said

Gemini (`gemini-3.8-flash`) and DeepSeek (`deepseek-flash`) were asked, in
five lines each, whether any PartiQL statement returns the table list, the key
schema, the indexes or the time to live. Both said no, and both named the
native calls `ListTables`, `DescribeTable` and `DescribeTimeToLive`. No lead
held beyond the measurements above.

### Alternator

Not built, as asked. It speaks the DynamoDB API, but it runs no PartiQL
(dbimp D163). So it is further from a catalog than DynamoDB Local is. Nothing
was run against it.

## What changes this

A way for a model to send a native call, or a catalog statement in PartiQL,
which AWS has not shipped. Then the model can answer tables, primary keys,
indexes and index columns, and the time to live as a setting, from
`DescribeTable`. Until then nothing here can be built.
