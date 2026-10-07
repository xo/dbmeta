# D189. dbimp reads the OpenSearch 2.19.6 DESCRIBE row

Status: Amends D181.

## The decision

D181 recorded that dbimp v0.14.0 cannot read a row of `DESCRIBE TABLES` on
OpenSearch 2.19.6, because that release declares every column keyword and sends
numbers in some of them. So Columns had no answer there, and the tests skipped
it under the condition `describeReadable`.

dbimp v0.14.1 reads a number in a keyword column as the value that arrived,
an `int64`. On 2.19.6 the position counts from 0, `NUM_PREC_RADIX` is 10 and
`NULLABLE` is 2. The test module pins v0.14.1 and the condition is gone. Columns
answers on both releases, 3 of the 3 kinds.

## What changed

- `describeReadable` and the skip of the conformance target are removed.
- The conformance report of 2.19.6 has a section of its own, `opensearch@2`,
  because 2.19.6 lists the alias `recent` as a table and 3.9.0 lists none.
- The parity sections of `opensearch@2` record the same differences as 3.9.0.
- Both releases passed `dbrun test opensearch --releases` on 2026-10-08.
