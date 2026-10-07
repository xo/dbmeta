# D174. The ClickHouse tests use dbimp's driver

Status: Decided.

## The decision

D154 says a test driver is the package that dburl names. dburl v0.44.0 names
`github.com/xo/dbimp/clickhouse` for the clickhouse scheme, in place of
`github.com/ClickHouse/clickhouse-go/v2`, and dbimp v0.13.0 holds it. Ken asked
on 2026-10-07 to switch, so the test module did.

- `clickhouse-go` is out of the test module and out of the lint allow list.
  The driver name is `clickhouse` for both, so the two cannot share a binary.
- dbimp's driver speaks the HTTP interface, which the entry publishes on the
  second host port (D124). So the DSN and the URL of the entry are now
  `clickhouse://user:password@host:port/default` on that port, as D167 asks.
  The `api` is the same host and port with an `http://` scheme. The native
  port, 9000, is still published, and nothing here uses it.
- The model binds by position with a question mark, as before. The driver
  writes each value as a typed server parameter.

The four releases the entry runs, 25.3, 25.8, 26.8 and 26.9, pass the test
module on the new driver. The privilege parity test needed one change. Its
file records the text of each refusal, and the new driver writes it as
`clickhouse: ACCESS_DENIED (497): ...` where `clickhouse-go` wrote
`code: 497, message: ...`. The same eight queries are refused for the same
reason on every release, so only the wording in the file changed.
