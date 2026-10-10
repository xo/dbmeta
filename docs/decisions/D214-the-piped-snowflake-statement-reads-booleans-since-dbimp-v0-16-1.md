# D214. The piped Snowflake statement reads booleans since dbimp v0.16.1

Status: Amends D213.

## The decision

D213 cast the two booleans of the piped `Columns` statement to numbers, because
the SQL API sends a boolean of a piped statement as the text 0 or 1, and the
driver of dbimp read only true and false. dbimp v0.16.1 reads the text 1 and 0 as
a boolean in a column of the type boolean, so the cast is gone. `nullable` is
`c.is_nullable = 'YES'` and `primary_key` is `k."key_sequence" IS NOT NULL`
again, as before D213. The test module pins dbimp v0.16.1.

## What was measured

The whole Snowflake suite, parity and conformance included, ran against the
trial account through `dbrun test snowflake` on 2026-10-10, with the new driver
and without the cast. The results are in the commit that records this decision.
