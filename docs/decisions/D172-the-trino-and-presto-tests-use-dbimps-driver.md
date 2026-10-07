# D172. The Trino and Presto tests use dbimp's driver

Status: Decided.

## The decision

D154 says a test driver is the package that dburl names. dburl v0.43.0 names
`github.com/xo/dbimp/trino` for both the trino and the presto scheme, and
dbimp v0.12.0 holds it. So the test module moved, on 2026-10-07, from the
two vendor clients to that one package.

- `trinodb/trino-go-client` and `prestodb/presto-go-client` are out of the test
  module and out of the lint allow list.
- Both products open the driver name `trino`. The driver tells Presto from
  Trino by `GET /v1/info`, so no `flavor` key is set.
- The DSN of both entries is `trino://user@host:port/catalog/schema`, and the
  URL is the same string, as D167 asks. The test image takes no password, so
  neither entry has an ordinary user.
- The models bind by position with a question mark, as before. The new
  driver writes each value into the statement as the product writes a
  literal, where the vendor clients used PREPARE and EXECUTE. The comments in
  the two models say so.

Trino 476 and 483 and Presto 0.299 were run on the new driver on the date
above, as the next section records.
