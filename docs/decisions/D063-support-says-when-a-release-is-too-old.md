# D63. Support says when a release is too old

Status: Amends D54.

`Query.Support` has a fourth value, `TooOld`. It means the model is present,
the product has the object, and this release of it does not. `Query.Build` then
returns `ErrVersionTooOld`, as it always did.

## What it replaces

D54 left this open. `Support` answered a question about the product and the
release was the error's business, so a query gated above the server reported
`Supported` and then refused to build. `TestWrongProductIsNotSupported` fixed
that on purpose and D54 recorded the doubt: a caller that trusts `Support`
walks into a query it cannot build.

Cassandra made it concrete. `Settings` reads `system_views`, which arrived in
4.0, so on 3.11 `Support` said yes and `SQL` said no. Every smoke test already
carried the same two step dance, catching `ErrVersionTooOld` after being told
the query was supported.

## Why a fourth value rather than folding it into NotSupported

Because they are different answers and a caller acts differently on them.
`NotSupported` means stop asking: no upgrade changes it. `TooOld` means this
server cannot and a newer one can, which is something a person can act on.
Folding them together loses that. A consumer showing a person what it can
offer then says "this database does not have roles" when the truth is "yours
is too old".

## The ordering

`TooOld` is last in the constant block rather than in order of how supported
each value is, so that `NotBuilt`, `NotSupported` and `Supported` keep their
numbers. They are compared by equality and never by rank, and nothing in the
tree orders them.
