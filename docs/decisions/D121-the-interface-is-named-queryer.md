# D121. The interface is named Queryer

Status: Amends D49.

D49 named the one method interface `Querier`. The spelling is correct English
and wrong for Go. Go's own packages spell the word Queryer:
`database/sql/driver` names the interface of this same method
`QueryerContext`, and its older `Queryer`. A Go programmer who searches for
the interface by that name finds nothing, and one who reads `Querier` beside
`driver.QueryerContext` sees two spellings of one idea.

Ken asked on 2026-09-29 for the name to follow Go's. The interface is
`Queryer`. Its method and everything D49 decided about it are unchanged. The
test that holds the one method was renamed with it, and is
`TestQueryerIsOneMethod`.

Every mention of the old name was changed, in the code, the tests and the
documents, D4 and D49 among them, so that a search for either spelling finds
one answer. The old name is kept in two places only: this decision, and the
doc comment of `Queryer`, which says what it was called until now.

No consumer imports dbmeta yet, so the rename breaks no caller. usql and
dbtpl will use the new name from the start (`docs/USQL.md`, `docs/DBTPL.md`).
