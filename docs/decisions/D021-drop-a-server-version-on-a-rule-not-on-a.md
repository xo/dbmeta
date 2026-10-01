# D21. Drop a server version on a rule, not on a judgment

Status: Decided.

The support matrix grows every year unless something removes from it. This
decision makes removal mechanical, so no release needs an argument about it.

Both external reviews were asked and both gave the same primary trigger.

## The rule

> A server major version is supported while its upstream vendor end of life
> date is in the future. Support is dropped in the first minor release of
> `dbmeta` published after that date passes.

Use upstream end of life, not the other two candidates. A container image
disappearing is an operational accident rather than a policy. Cloud provider
support is commercial, differs per cloud, and often runs longer than upstream
for customers who pay for it. Upstream dates are published years ahead and
anyone can check them.

DeepSeek proposed adding a six month margin, so that a version leaves before
its end of life rather than after. Prefer the plain rule. A margin means
dropping a version that upstream still supports, which item three below turns
into a breaking change.

## Notice

Give one minor release of notice, and at least three months.

Use three mechanisms and not a fourth.

1. A support table in `README.md` with a column for the planned removal
   release.
2. An entry in the release notes and the changelog when a version is deprecated
   and again when it is removed.
3. A function that a caller can ask, so an application can warn in its own
   voice.

Do not log a warning from the library. Go has no single logging convention, so
a library that writes to standard output corrupts the output of a command line
tool. `usql` is exactly such a tool. Gemini made this point and it is right for
this project in particular.

A compile time signal is impossible. The server version is discovered over a
network connection at run time.

## Semantic versioning

Dropping a version that is already past upstream end of life is a minor
release. Go module compatibility covers the exported Go API, and the exported
API does not change when a query set is removed.

Dropping a version that upstream still supports is a breaking change and needs
a major version. This is the reason to avoid the six month margin above.

Both reviews agreed on this and both cited `jackc/pgx` and
`go-sql-driver/mysql` as precedent. The specific claims about those projects
were not verified.

## Delete rather than freeze

Delete the queries for a dropped version. Tag the last release that supported
it, and say so in the support table, so that a user on an old server can pin
that tag.

Frozen queries rot. They stay in the tree, they stop being tested to save CI
time, and a version selection fault can silently fall back to one and return a
wrong answer rather than an error.

## Below the floor: refuse, with a way through

When the server is older than the floor, return an error and name the versions:

```
postgres 12 is not supported, the oldest supported version is 14
```

Offer one option that proceeds anyway with the oldest known query set, for a
caller who accepts the risk. Do not make that the default. A best effort
attempt against an old catalog fails with a confusing SQL error, or worse
returns a partial answer that looks complete.

## Above the ceiling: proceed

When the server is newer than anything `dbmeta` knows, use the newest known
query set and continue.

This is the common case, because people upgrade a server faster than they
upgrade a library. A library that refuses an unknown newer server breaks every
user on the day a new release ships.

It can fail, and the failure is acceptable because it is loud and searchable.
PostgreSQL 12 removed `pg_attrdef.adsrc`, so a query written for 11 failed on
12 with `column "adsrc" does not exist`. That is a clear error that produces a
bug report. A silent empty result does not.

Do not catch a catalog error and return an empty set. An empty set means the
database has no such object.
