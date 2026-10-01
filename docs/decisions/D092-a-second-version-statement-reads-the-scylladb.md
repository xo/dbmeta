# D92. A second version statement reads the ScyllaDB release

Status: Amends D91.

D91 found ScyllaDB from `system.local` and did not read its release, which is
only in `system.versions`. It left that as an open question, and Ken
answered it on 2026-09-27: read the release with a second statement.

## What changed in the API

`Info` has two new fields, `FollowUpQuery` and `ParseFollowUp`, and `Dialect`
has two methods with the same names. `FollowUpQuery` takes the set that the
first statement produced and returns a second statement, or none.
`ParseFollowUp` adds what that statement read. `Dialect.Version` runs both.
The cql model is the only one that sets them, and it asks the follow-up only
of a server that the first statement found to be ScyllaDB. Cassandra runs one
statement, as before.

A caller that runs the statements itself, as `usql` does, now asks for the
follow-up after the first statement. The answer depends on the server, so the
caller cannot decide it from the dialect alone. D38 already made the version
statements something a consumer runs and `dbmeta` does not.

The ScyllaDB release, such as 2026.3.1, is recorded under the `scylla` key,
and the display line names it. The main version stays the Cassandra release
from `system.local`. A fragment for one ScyllaDB release and newer can now
gate on `Gate{Key: Scylla, Min: ...}`, which hard rule 3 asks for. No query
needs one yet.

## A refusal is not an error

A role granted nothing reads `system.local` and is refused `system.versions`,
on 2025.1 and on 2026.3. If the follow-up failed the whole version read, that
role reads nothing at all, because every consumer reads the version first. So `Dialect.Version` keeps the first statement's answer when the
follow-up fails. The role still learns that it is talking to ScyllaDB, and
the release stays unknown. An unknown release meets every gate on the key, so
the role gets the newest fragments, which is what D21 does above the ceiling.

A canceled context is the one failure that is an error, because nothing
after it can run. A test in the root module cancels between the two
statements to hold that apart from a refusal.

This is the answer depending on who is asking, which D61 exists to find.
`usql` has the same shape for Oracle, where it reads `v$instance` and prints
nothing for an ordinary user. The difference is that here the ordinary user
still gets the product and every query, and loses only a number that no
query gates on today. `TestScyllaIsItsOwnProduct` connects as a role granted
nothing and checks exactly that.

## Parity files a release by the product's own number

`parityRelease` picked a section such as `product@major` by the main version.
For ScyllaDB that is Cassandra 3.0.8, so a section for one ScyllaDB release
was never found. It now reads the product's own key when the server
reports one, so a section named `scylla@2026` works. None exists yet, because
all four releases give the same answers.
