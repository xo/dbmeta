# D234. The Exasol tests use the driver of dbimp

Status: Amends D154 and D87.

## The decision

The tests of Exasol open `github.com/xo/dbimp/exasol`, which dburl v0.52.0 names for the
exasol scheme. `test/go.mod` pins dburl v0.52.0 and dbimp v0.20.0, which holds the driver.

`test/go.mod` no longer requires `exasol-driver-go` and the modules that only it
needed. The depguard list in `test/.golangci.yml` no longer allows it. `dbrun` no
longer turns off the logger of that driver, because the driver of dbimp does not log.

## The connection string

The driver of dbimp reads the URL that dburl writes. The key
`validateservercertificate=false` turns off the check of the certificate, which the
nano image and the Community Edition machine need because neither certificate is
signed by an authority. The old driver read `exa:host:port;user=...;password=...`.
So the `dsn` and the `url` of both Exasol entries in `container/exasol.go` are now
the same string, `exasol://sys:<password>@127.0.0.1:<port>?validateservercertificate=false`.

The tests make the string of a principal with `replaceUser`, the helper that the
other URL drivers use. `exasolUser` is gone.

## What changes in the tests

A password can now hold a semicolon, because the URL escapes it. D87 left that
password out of `TestChangePasswordExasol` for the semicolon of the old DSN, and
that exception is over. All seven hostile passwords log in.

The text of an error differs. The old driver wrote
`E-EGOD-11: execution failed with SQL error code '42500' and message '...'`. The driver
of dbimp writes `exasol: 42500: ...`. Two lines of `role_grants` and `user_mappings`
in each of the four Exasol sections of `test/testdata/parity.txt` changed to the
new text, and nothing else in a golden file changed. The model did not change.
The other tests passed on `exasol-2026.2.0`, including conformance, the scan and
the privilege parity, so the driver binds `?` and returns the types that the model
scans.

## What is not measured

The Community Edition 2025.2.1 machine is Verified and was not started for this
change. Its two parity sections carry the new error text by edit, because the same
driver writes it. Run `dbrun test exasol-2025.2.1` on a development machine at the
next release.

## At the bump

dbimp tagged v0.20.0 with the driver on 2026-10-11, and `test/go.mod` pins it with dburl
v0.52.0. `dbrun test exasol-2026.2.0` passed with no workspace.
