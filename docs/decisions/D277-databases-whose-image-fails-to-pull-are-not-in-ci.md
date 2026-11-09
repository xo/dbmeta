# D277. Databases whose image fails to pull are not in CI

Status: Amends D40, D42 and D119.

Four CI runs between 2026-10-31 and 2026-11-08 failed on a job that never ran a
test, because `dbrun` failed to pull the image: `couchbase:7.2.9`,
`elasticsearch:9.5.5`, `usql/vertica:7.2` and the ScyllaDB images. A rerun of one
passed with no change in the code. A failure of that kind fails the whole run and
says nothing about dbmeta. The maintainers decided on 2026-11-08 to take such a database out
of CI completely.

## Decision

Couchbase, Elasticsearch, Vertica and ScyllaDB are in the Verified tier for every
release, so no push and no nightly run starts them. A person runs them with
`dbrun test` before a release, as D40 says for that tier.

A database goes back into CI when its image pulls without a failure for a month,
or when `dbrun` retries a pull. Neither is done here.

## Consequence

The Tested and Nightly lists are shorter by 11 releases. The models of the four
products are not tested on every push, and a change to one has to be run by hand.
