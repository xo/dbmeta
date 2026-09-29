# D142. ql is removed

Status: Amends D116, D119 and D129.

Ken decided on 2026-09-30 that the ql driver is removed from usql, and that
dbmeta removes every entry for it. His rule is that a database few people use
must not force logic or architecture into usql. ql was the one product that
needed BatchAsTransaction, and D129 had it waiting for a model that no one
was going to write.

So the ql dialect is gone from `dialect.go`, dbrun's list of embedded
databases no longer has it, and the documents no longer name it. usql tells
dburl to drop the ql scheme. chai and csvq stay as D129 leaves them.
