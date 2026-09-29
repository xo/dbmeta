# D126. A version query waits for the product's model

Status: Decided.

## The decision

Ken decided on 2026-09-29 that a product's version query arrives with its
model, and not before. A dialect with no model registers nothing, so it has
no version query, and usql reads the version of such a product itself until
the model exists.

## Why it came up

Ken wants each database's version query to live in dbmeta's dialects rather
than in usql. QuestDB is where that met a product with no model. Its
PostgreSQL interface answers `SHOW server_version` and `version()` with
PostgreSQL 12.3, and only `SELECT build()` names the QuestDB release, as the
usql session measured on 10.0.1. usql shows "PostgreSQL 12.3 (questdb)" for
now. `container/questdb.go` records the query, so that the model has it.

## Rejected

A dialect that registers a version query and no model. It would give usql the
version sooner. It would also make a release that no model reads look read,
because a release is Staged exactly when its dialect has no model (D119), and
a version query alone would need that rule to change.
