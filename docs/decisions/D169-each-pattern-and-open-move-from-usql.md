# D169. Each, Pattern and Open move from usql

Status: Decided.

## The decision

Ken wants usql's metadata layer as thin as possible over dbmeta, with what
other projects can use moved here. usql named three general pieces on
2026-10-02, and Ken chose to move all three.

`Query.Each` runs a query from an `Args` and passes only the arguments the
query takes. `All` refuses an argument a query does not take, because that
catches a misspelled name in a map, and it keeps that. Every caller that asks
every kind of object with one set of arguments wrote the same loop over
`Query.Params`, and `Each` is that loop. `Args` gains `AccessMethod`,
`Server` and `Database`, the three parameters a few PostgreSQL, CockroachDB
and CrateDB queries take.

`Dialect.Pattern` splits a psql pattern, such as `public.film*`, into a schema
pattern and a name pattern. It splits at the first dot outside double quotes,
folds the text outside them with `FoldIdentifier`, keeps the text inside them
as written, and turns `*` into `%` and `?` into `_`. It depends on the fold and
the quoting of each product, which dbmeta owns (D127, D143).

`dbmeta.Open` reads the version and calls `New`. If the version cannot be
read, it returns the error and no `Meta`. usql falls back to an empty
`VersionSet`, which takes the newest fragments, and that can give wrong
statements on an old server. That choice stays with the caller, which calls
`New` with an empty `VersionSet` itself, because dbmeta does not hide a
failure.

usql's `Queryer` that logs each statement, or runs nothing for a dry run,
stays in usql. No other user is known, and the interface has one method, so
the wrapper is a few lines for any caller that wants one.
