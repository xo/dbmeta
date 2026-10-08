# D202. Cassandra matches names ignoring case, and the models share two helpers

Status: Amends D200.

## The decision

D200 gave the Cassandra model a row filter, `Binding.Keep`, which matched names
with `dbmeta.Like`, so a pattern was case sensitive. Ken decided on 2026-10-09
that the match ignores case, because an unquoted CQL name is case insensitive.
usql folds an unquoted pattern to lower case before it asks, as psql does, and a
quoted name keeps its case, so a pattern that matches only by case now matches
more than its author wrote. Ken accepted that for Cassandra.

The root package has a new function, `LikeFold`, which is `Like` with the case
of letters ignored on both sides. The Cassandra model uses it for the `schema`,
`name` and `parent` filters. A model for a product with case sensitive names
keeps `Like`.

## The shared list helper

Four models held a copy of the same test, whether a parameter such as `types`,
a list of words joined by commas, is empty or names a word: Cassandra,
Elasticsearch, OpenSearch and Impala, and InfluxQL held it inline. The root
package has `ListHas` now, and the copies are gone. `InList`, which already
existed, builds the same test as SQL, so the new name says what it returns.
`ListHas` treats an empty list as every word, as every parameter here does.
Impala guards the empty list before it calls the test, so its behavior did not
change.

## What changed

- `LikeFold` and `ListHas` in `walk.go`, with tests.
- The Cassandra filter test expects a pattern in the wrong case to match.
- `docs/COVERAGE.md` says the match ignores case.
