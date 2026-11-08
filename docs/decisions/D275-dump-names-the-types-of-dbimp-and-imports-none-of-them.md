# D275. dump names the types of dbimp and imports none of them

Status: Decided.

D274 puts the dump in a package of the root module. The root module has no
dependency (hard rule 1). The date, time, interval, UUID and decimal kinds need
Go types that the standard library lacks, and `dbimp` already holds them in
`civil.go`, with `uuid.UUID` and `*apd.Decimal` for the other two. `dbimp`
requires `apd`. Ken asked on 2026-11-08 whether `dump` can use them, and decided
that it takes no dependency.

## Decision

The `dump` package names the Go type of a kind as a string, in `GoType{Import,
Name}`, and imports none of them. The neutral kinds use the kind names of
`dbimp` (`docs/TYPES.md` of dbimp), so that dbmeta and dbimp give one answer for
a type that both name.

The data side reads a value by the kind of its column. It accepts a
`time.Time`, text, or any `driver.Valuer` whose text is ISO 8601, which every
`dbimp` civil type is. It has a small parser for the forms that databases write.
It does not call the `dbimp` parser. A year before 0001 or after 9999 is an error
unless the parser reads the signed forms that `dbimp` writes, and the plan says
which columns it affects.

## Rejected

A separate module for `dump` that requires `dbimp`. It keeps the root module
free of dependencies but contradicts D274, which chose a package in the root
module.

A `require` of `dbimp` in the root module. It breaks hard rule 1.

## Consequence

The parser is a second implementation of the ISO 8601 forms. Its tests read the
text that each `dbimp` civil type writes, so a difference between the two shows
as a failed test.
