# D25. Test on amd64 only. No build tags and no platform gates

Status: Decided.

`dbmeta` tests on `linux/amd64` and on nothing else, both in CI and on a
development machine. This applies to every database, PostgreSQL included.

`dbmeta` contains no `//go:build` constraint on an operating system or an
architecture, and no code path that branches on either. `dburl` works this way
and `dbmeta` follows it.

Two clarifications, because both external reviews misread this decision in
different ways.

This is a testing policy, not a restriction on where the code runs. `dbmeta` is
pure Go under D29, so it builds and runs anywhere Go does. Nothing enforces
amd64 and nothing should. Gemini read the decision as needing a `//go:build
amd64` constraint to enforce itself, which would contradict the rule. It does
not, because nothing is being enforced. Only testing is limited.

The ban is on operating system and architecture constraints. It is not a ban on
build tags of every kind. D31 gates model registration with the feature tags
`none`, `base`, `most` and `all`, which say nothing about a platform. DeepSeek
read the two as contradictory. They are not, and the difference is the subject
of the tag.

## The assumption, stated plainly

The same version of the same database, given the same schema, answers a query
the same way on every platform.

This is an assumption, not a fact, and it is adopted on purpose. The database
vendor is responsible for its product behaving the same across the platforms it
ships on. `dbmeta` takes the vendor at its word rather than multiplying the
test matrix by the number of platforms.

Do not add a platform test because you suspect a difference. Report the
difference to the vendor. If a real one is found that `dbmeta` must work
around, bring it to Ken and this decision gets revisited. Do not quietly add a
build tag.

## Where the assumption is weakest

Recorded so that a future reader knows what was accepted, not as a reason to
act now.

Collations are the clearest case. PostgreSQL builds `pg_collation` from the
locale data of the host, so `\dO` can list different collations on two
machines. This is not even a platform difference in the usual sense, because
two amd64 Linux hosts with different C library versions can disagree.

D12 already neutralizes most of this without any extra work. Generation and
tests run against a pinned container image, and the container carries its own C
library and its own locale data. The result depends on the image, not on the
host, which is one more reason to pin a digest rather than a tag.

Other places where a platform difference is plausible: default character set
and encoding, values in `pg_settings` that contain a file path, and system
views that differ between SQL Server on Linux and SQL Server on Windows. None
is a reason to test more platforms today.

## Platform specific dependencies are allowed, inside the driver

The rule bans platform gates in `dbmeta`. It does not ban depending on a driver
that has them.

The DuckDB driver is the live example. `github.com/duckdb/duckdb-go/v2` pulls a
separate binding module per platform, and `usql` carries all five as indirect
dependencies: `lib/darwin-amd64`, `lib/darwin-arm64`, `lib/linux-amd64`,
`lib/linux-arm64` and `lib/windows-amd64`. The driver selects one with its own
build tags.

That is the driver's business. `dbmeta` writes none of those tags. Do not be
surprised when `go mod tidy` adds five platform modules, and do not try to trim
them.
