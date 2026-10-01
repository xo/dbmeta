# D41. Every model ships its fixtures beside its queries

Status: Decided.

A model is not finished when its queries are written. It is finished when a
real server has answered them, and that needs a schema containing one of every
object the queries read.

The fixture lives in `models/<driver>/fixture`, beside the model, for two
reasons.

The queries and the fixture change together. Adding a query for event triggers
means adding an event trigger to the fixture, and those belong in one change
rather than two repositories.

The fixture is useful to other projects, which is the stronger reason and the
one that makes it exported API rather than a test artifact. `dbtpl` can
generate against a schema that is known to exercise every object kind instead
of writing one of its own.

A fixture is only SQL text. It needs a driver to execute and none to define, so
it stays inside the root module and D26 holds. The `test` module imports it and
runs it.

## Versioned like a query, skipped unlike one

A fixture varies by release for the same reason a query does. `CREATE TRIGGER
... EXECUTE FUNCTION` is release 11 syntax, and a fixture that writes the
deprecated `EXECUTE PROCEDURE` everywhere tests syntax nobody writes on a
current server. Reuse `Fragment`, `Choice` and `Stmt` rather than inventing a
parallel type, because the fixture is public and a caller must not have to
learn two ways to say the same thing.

One behavior differs. A query with no applicable alternative is refused, and
`ErrVersionTooOld` is the right answer, because asking for publications on 9.6
is asking for something that is not there. A fixture step with no applicable
alternative is skipped, because there is nothing to create and the query that
reads it is refused anyway. Express that by checking `errors.Is` against the
same sentinel rather than by adding a second resolver.

## Additive

A fixture is additive. A later release can add an object to one and will not
rename or remove what is there, so code written against it keeps working. A
change that is not additive means a new fixture beside the old one.

The cost of exporting it is that the schema becomes part of the public
contract, not just the Go types. Renaming a table to suit a new test
breaks a consumer generating against it.

## The invariant this buys

With a fixture at every release, the expected output needs no golden file per
release. `Field.Min` already declares the release each column arrived in, so a
test asserts generically that a field the server is too old for is NULL in
every row. Ten releases times forty eight queries is four hundred and eighty
combinations, too many for anyone to maintain, and one assertion covers them.
