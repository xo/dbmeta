# D51. There is no alias for a nullable type

Status: Decided.

A field a database may report as NULL is declared `sql.Null[T]` and never a
named alias of one. `Text` and `Int` are gone.

## What was inconsistent

Two of the four nullable kinds were aliased and two were not: 93 fields as
`Text`, 5 as `Int`, and five written out as `sql.Null[bool]` or
`sql.Null[float64]`. Two structs declared next to each other read differently
for no reason a caller could see.

## Why not alias all four instead

Both reviews rejected that and so did Ken, for the same reason, and the reason
is what a reader actually sees.

`go doc` in a terminal prints plain text. With an alias a reader of
`go doc dbmeta.Sequence` sees `Cycles Text` and has to run a second command to
learn that it can be absent. Without one they see `Cycles sql.Null[bool]` and
already know. A code review diff and an editor's field list behave the same
way, and pkg.go.dev's clickable link is the only place where the alias costs
nothing.

The other two would have had to be called `Bool` and `Float`, which read like
primitives and hide the single thing a caller has to know about the field.
Gemini's phrasing: a name like `Text` is dangerous for a nullable type, because
a reader assumes it behaves like a string and does not expect to check `Valid`.

Neither review thought 93 against 5 argued for keeping one alias. DeepSeek
called that status quo bias, and it is. Frequency makes a name dominant, not
clear.

## Defined types were never an option

`type Text sql.Null[string]` does not inherit the methods of its underlying
type, so it loses `Scan` and `Value` and every `rows.Scan(&v.Comment)` in all
every binding stops working. The aliases worked only because they were aliases.

## Where the lesson went

The `Text` doc comment held the canonical account of this project's most
expensive mistake, that collapsing a NULL access list into an empty string made
"the owner has full access" read identically to "nobody has any access". That
is in `NULLS.md` in full, where it always was, and the package documentation in
`object.go` now points there.

An invariant that governs the whole project should not have been hanging off a
type alias. Deleting the alias fixed that as a side effect.

## The shape of the change

98 field declarations in `object.go` and nothing in `models/`, because every
binding names the struct field rather than the type. It was mechanical, and it
was cheap only because no release is tagged: `Text` was exported.
