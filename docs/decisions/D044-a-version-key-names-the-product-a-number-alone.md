# D44. A version key names the product. A number alone never does

Status: Decided.

Two products that share a dialect gate on a named version key, not on the
number. MariaDB records its version under the key `mariadb`, MySQL under
`mysql`, and a fragment written for one names that key. A server that does not
report the key does not meet the gate, however new its numbers are.

The proposal this replaces was to fold the product into the version itself,
with `V(Major(11), Minor(13), Patch(15), Variant("mariadb"))`. Gemini and
DeepSeek both rejected it, and for the same reason. A version exists to be
ordered, and a product does not order: `mariadb` is neither greater nor less
than `mysql`. Putting an incomparable thing inside the comparable type breaks
the one contract `Version` has, and it costs a signature change at 134 call
sites to do it. The named key is the same mechanism with none of that, and it
was already in the code for Cassandra, which reports its release, its CQL and
its protocol version separately.

## The three rules

A fragment naming a key the server did not report never applies.
`VersionSet.Get` returns an unknown version for a missing key, and unknown
sorts above every known version, so reading it alone makes an absent key look
like the newest possible server. `VersionSet.Has` answers the real question and
`Gate.Met` calls it.

Within one `Choice`, a named key beats the empty key, and among alternatives
sharing a key the highest `Min` wins. A fragment written for one product is
more specific than one written for the family.

Two alternatives naming different keys, both met, is `ErrAmbiguousFragment`. It
is a fault in the model. Nothing decides between them, and picking the higher
number compares releases that mean different things.

A parser sets a key only for a product it actually detected. Never set a key
speculatively, because the absence of a key is the fact everything above rests
on.

## Wrong product and old server are different answers

A `Choice` where the server cannot meet any alternative on any release
returns `ErrNotSupported`. A `Choice` where the server reports the key and sits
below the `Min` returns `ErrVersionTooOld`. The first is a fact about the
product and no upgrade changes it. The second is a fact about the release and
an upgrade fixes it. `Query.Support` renders the statement and reports
`NotSupported` for the first, so a caller listing what a server answers is told
the truth before it runs anything.

## The fault this fixes, which was already shipped

The MariaDB model gated its check constraint column at 10.2 with no key. MySQL
recorded check constraints from 8.0.16 and reports 8 or 9, both below 10.2, so
that column was padded with NULL on every MySQL server that had it. The comment
beside the gate said "MariaDB 10.2 and MySQL 8.0.16" and the code said neither.
The sequence query had the same shape: gated at 11.5, it reported MySQL as too
old for an object MySQL has never had at any release.

A numeric coincidence standing in for a product test is the pattern. It reads
correctly, it happens to work for one case, and it fails the case it was
written for.

## Forks, and what a key cannot do

CockroachDB and Redshift are the next case, and a key handles half of it.
CockroachDB has its own release scale and lies in `server_version`, so
`{Key: "cockroach", Min: V(23, 1)}` is right. Redshift is a fork of PostgreSQL
8.0 whose number is honest and useless: PostgreSQL 8.0 fragments mostly work,
PostgreSQL 14 fragments do not, and Redshift has features PostgreSQL 8.0 never
had. No version predicate expresses that. The key tags the product, and
answering what such a fork can do needs a capability probe, which is a separate
question and is not settled here.

## When to stop sharing a dialect

Count the fragments that need a key. If most of them do, the shared model is a
partition wearing a trenchcoat, and a partition belongs in the type system: two
`Dialect` constants, two packages, shared helpers. The MariaDB and MySQL model
needs a key on seven pieces out of several hundred, so one dialect is right
today. Revisit it when that ratio moves.

## How it is tested

`TestMySQLAgainstMariaDB` builds the same fixture on one server of each product
and compares the answer to every query that narrows to one schema, row by row
and column by column. Two kinds of difference are expected and recorded: an
object only one product has, and a column each product spells its own way.
Anything else fails. It found the fault where `external_language` is NULL on
MariaDB and `SQL` on MySQL. That NULL fails to scan into a field that is not
nullable.
