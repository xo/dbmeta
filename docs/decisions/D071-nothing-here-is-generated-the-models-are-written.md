# D71. Nothing here is generated. The models are written

Status: Amends D2, D12 and D30, supersedes D11.

No generator runs in this repository. There is no `tool` directive in either
`go.mod`, no `go:generate` anywhere, and no file carries a generated header.
Every line under `models/` was written, and the queries in it were written by
an agent working against a running server, checking each statement as it went.

`dbtpl` generates nothing here and never did. It is a consumer of `dbmeta`,
the same as `usql`, and that is the only relationship between the two projects.

## Why this needed a decision of its own

Because the plan said otherwise in three places and the instructions repeated
it. D2 said `dbtpl` generates the model code. D11 pinned `dbtpl` with the
`tool` directive so that two agents on two machines would produce the same Go
from the same SQL. D30 corrected half of it, saying `dbtpl` is not used, and
then put generation in a sub-package that was never built.

`CLAUDE.md` carried the consequence. It told a reader that `models/<driver>`
holds generated files, that they must not be edited, and that a change goes
into the SQL and is generated again through `go tool dbtpl`. All three are
wrong, and the first two are worse than wrong: they tell somebody not to touch
the only files there are to touch.

`docs/EVALUATION.md` carried it too, requiring a pinned image digest beside
each model so that generation would be reproducible. Nothing is reproduced,
so nothing needs the digest.

## What replaces it

A model is written, read and edited like any other Go. A query is written
against a live server, `dbrun` starts that server, and rule 9 makes the fixture
part of the model rather than something a generator would emit.

What D2 decided about layout stands. One package per driver under `models/`,
one package covering every supported release, with the version differences held
as data inside it, which is D8. Only the claim that a tool produced it is gone.

What D11 reasoned about build dependencies stands too, and it is D26 that
carries it: a driver does not belong in the root module. There is simply no
build dependency left to place.

D6a went the same way and is marked superseded by this. It put the NULL scan
fix in a generator's flags. The fix is in the code, the rule is D6 and
`docs/NULLS.md`, and `TestNoCoalesceOnCatalogColumns` guards it.

## What this does not mean

It is not a rule against generating code here later. If a generator is written,
it will be ordinary Go in this repository, it will mark what it emits, and it
will get a decision of its own. Until then, a file under `models/` is a file
somebody wrote, and the honest thing is to say so.
