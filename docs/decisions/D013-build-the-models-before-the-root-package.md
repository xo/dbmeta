# D13. Build the models before the root package

Status: Decided.

Build the queries for the primary databases first. PostgreSQL comes first, then
MariaDB, then the other primary databases. Write the root `dbmeta` package
after them.

The order matters. A root API designed before any query exists is a guess. An
API written after several databases have answered the same questions is a
description of what they can actually do.

This inverts the order that an earlier draft of this file gave. Ignore that
draft.
