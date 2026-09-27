# D59. Oracle is tested with go-ora v2 until v3 tags its fix

Status: Amends D52.

D52 says a test driver is the one `usql` uses or it is the wrong driver.
Oracle is the first exception, and it is not a preference.

`usql` pins `github.com/sijms/go-ora/v3 v3.0.1`. That version cannot connect to
Oracle 11g or 18c. It does not return an error. It panics:

```
panic: runtime error: slice bounds out of range [41:32]
	go-ora/v3/network.newAcceptPacketFromData
	go-ora/v3/network/accept_packet.go:61
```

The cause is in the source. `newAcceptPacketFromData` reads

```go
NegotiatedOptions2: binary.BigEndian.Uint32(packetData[41:]),
```

unconditionally, and the accept packet an older server negotiates is 32 bytes.
v2 talks to every release here. Measured against all six:

| Driver | 11g | 18c | 19c | 21c | 23ai | 26ai |
| --- | --- | --- | --- | --- | --- | --- |
| `go-ora/v2 v2.9.0` | yes | yes | yes | yes | yes | yes |
| `go-ora/v3 v3.0.1` | panic | panic | yes | yes | yes | yes |
| `go-ora/v3` at master | yes | yes | yes | yes | yes | yes |

The last row is the important one. It is fixed upstream and not yet tagged, in
sijms/go-ora issue 759, and the fix was checked here against all six servers at
v3.0.2-0.20260914154503-360b4b7ac9e9 rather than taken on trust.

## Why this does not weaken D52

D52 exists so that a query passing here cannot fail on what a user runs. That
reasoning is about the SQL, and it still holds: the statements are the same
whichever major version of the driver carries them, because the difference is
in the wire protocol negotiation and not in what the server parses.

What D52 could not anticipate is a driver that cannot reach the server at all.
Holding to v3 would not make the old releases work in `usql`. It would only
stop dbmeta from testing them, and then nobody would know they are broken.

## This reaches usql, and only until v3 tags

`usql` switched to v3 recently and pins v3.0.1, so a `usql` user who connects
to Oracle 11g or 18c today gets a Go panic rather than a message. A panic takes
the program down, which is worse than not supporting the release.

Ken is moving the `xo` database projects back to v2 for now. The fix being
already upstream makes that an interim measure rather than a direction.

## The driver to want, which now exists

`godror` was Oracle's sanctioned driver and hard rule 10 bans it, because it
needs the Oracle Instant Client rather than only a C compiler.

Oracle has since published a pure Go one: `github.com/oracle/go-oracledb`,
package `oracle`, registering the driver name `oracledb`. It depends on
`golang.org/x/crypto` and `golang.org/x/text` and nothing else, so hard rule 10
has no objection to it. It was tagged v0.0.1 on 2026-08-18, and the module also
carries v26.0.1-beta.

It is not adoptable yet and it is worth watching. Tried against these servers
it reached authentication on 11g, 19c and 26ai, so the protocol side works
across the range that go-ora v3 cannot, and it then refused the credentials
because its connection string is not the URL form the rest of this project
uses. That is a matter of reading its documentation rather than a fault, and it
was not chased further, because a v0.0.1 beta is not what a test suite should
depend on.

Revisit when it reaches a stable release. It is the only driver that is both
pure Go and Oracle's own, and if it reaches 11g it settles this decision and
hard rule 10's Oracle clause together.

## When this ends

When v3 tags the fix. Then move Oracle back to v3, match `usql` again, and mark
this superseded. There is nothing else to decide at that point.

Pinning v3 at the master commit instead was considered and not taken. It would
keep the same package `usql` uses, and a pseudo-version is tolerable in a test
module in a way it is not in a shipped binary. But v2.9.0 is a tagged release
that reaches every server here, and choosing a tagged release over an untagged
commit needs no explanation later.

This amendment covers Oracle and nothing else. Every other database still uses
the driver `usql` ships, which D52 requires and this does not change.
