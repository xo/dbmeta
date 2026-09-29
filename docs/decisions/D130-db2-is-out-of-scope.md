# D130. Db2 is out of scope

Status: Decided.

Ken decided on 2026-09-29 that Db2 is out of scope, because it is not
possible to test here, and that usql removes its support for Db2 too.

Db2 was the one database usql supported with no entry in dbrun. Its Go driver
needs IBM's client libraries, which is the reason D48 rules out godror: a
driver that needs a vendor's client library, and not only a C compiler,
cannot be built into the test module. docs/EVALUATION.md held the container
facts usql had for Db2, as a lead for an evaluation. They are removed, and no
entry is made.
