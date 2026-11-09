# D282. Hive and Impala have HTTP entries for dbimp

Status: Decided, amended by D283.

## Decision

`container` has two more products, `hivehttp` and `impalahttp`. They start the
same images as `hive` and `impala`, and a client reaches them over the HTTP
transport of HiveServer2 and nowhere else. dbimp wrote its own driver for that
transport and the maintainers chose to use only the HTTP connection with it.

The names of the releases are `hivehttp-4.2.1`, `hivehttp-4.0.1`,
`impalahttp-4.5.3` and `impalahttp-4.4.1`. They are Verified, so CI never starts
them. The binary entries stay, because `models/hive`, `models/impala` and usql
read them.

## What was measured

Hive 4.2.1 and 4.0.1, and Impala 4.5.3, on 2026-11-09:

- Hive publishes HTTP on 10001 and serves the path `cliservice`. Any other path
  fails to connect. The binary port 10000 refuses connections.
- Impala publishes HTTP on 28000, which is the default of `hs2_http_port`. It
  serves every path, so `cliservice` is a convention here and not a rule.
- Both accept any user name and any password. A request with no Authorization
  header gets 401 from Hive, so a client must send a Basic header, which is what
  the PLAIN exchange of the HTTP mode sends. No user can be made, because
  neither image checks one, so there is no Init and no ordinary principal.
- `dbrun dsn --json` gives `api` as the `http://` address, which is also the
  `dsn`, and `url` as the form that a person types.

Impala 4.4.1 and Hive 4.2.1 on the same port and path were not each run. They
share the image and the flags with the releases that were.

## The hand built request

A request that was built by hand, with an OpenSession call in the Thrift binary
protocol, was closed by Impala without an answer on every path. The fault was in
that request. It put the client protocol version in field 2 of the open session
request, and that field is the user name. The version is field 1. dbimp sent the
call with the version in field 1, the user name and the password in fields 2 and
3, and a Basic header, and both servers answered 200. Hive answered with
protocol version V10 and Impala with V6.
