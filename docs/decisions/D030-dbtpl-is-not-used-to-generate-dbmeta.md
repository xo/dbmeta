# D30. dbtpl is not used to generate dbmeta

Status: Amended by D71.

`dbtpl` is not used and `dbmeta` does not pin it. That half is right and it
holds.

The other half said generation would be driven by ordinary Go code in a
sub-package. There is no such sub-package and there is no generation at all.
D71 records what actually happened, which is that the models were written.

Ken decided this. Both reviews questioned the stated reason and their objection
is recorded here, because a later reader will ask.

Neither review accepted that the bootstrap was a cycle. Go modules pin
versions, so `dbtpl` at one version can generate `dbmeta` at the next, the same
way a compiler bootstraps from an older build of itself. Both called it an
ordering problem rather than a module cycle. My earlier note in this file said
the same thing.

Both also warned about the cost. A hand written generator has to reimplement
what `dbtpl query` already does: connect per driver, introspect the result
columns of a statement, map database types to Go types, decide nullability,
handle arrays and enums, and emit structs and scan code. DeepSeek put it as
"most of `dbtpl`".

Two things reduce that cost and they are worth stating, because they are the
reason the decision is reasonable despite the warnings.

`dbmeta` needs far less than `dbtpl` does. `dbtpl` generates models for an
arbitrary user schema it has never seen. `dbmeta` generates models for a fixed,
known set of metadata queries that the project itself writes. The column types
are known in advance because the author wrote the query, so the hardest part of
`dbtpl`, which is inferring a type from an arbitrary result, is mostly not
needed.

D8 also removes the reason `dbtpl` had to introspect at all. Under the padding
rule every fragment returns the same columns for every version, so the result
shape of a query is fixed and can be declared rather than discovered.

If the generator starts growing type inference, stop and reconsider. That is
the signal that it is turning into `dbtpl`.
