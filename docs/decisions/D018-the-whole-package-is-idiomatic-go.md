# D18. The whole package is idiomatic Go

Status: Decided.

Write Go that a Go reviewer recognizes. `dbmeta` is a move of older code, and
the older code predates generics, iterators, and the context rules that D17
sets. Do not carry a pattern over because it is already written.

The conventions are in `CLAUDE.md`. This decision records the two judgments
that need explaining, because both reverse something in the source.

## The Set cursor types do not carry over

`usql` returns each list as a `*CatalogSet`, a `*TableSet`, and 13 more like
them. Read `metadata.go` around line 1048 before you port one. The type is a
cursor with `Next`, `Get`, `Reset`, `Len`, `SetColumns`, and `SetScanValues`
over a slice that is already in memory.

Three things are wrong with it for a new package.

1. The cursor buys nothing. `NewCatalogSet` takes a `[]Catalog` and the rows
   are already materialized. A cursor over a full slice is a cursor over
   nothing.
2. It throws the type away and takes it back by assertion. Every row is stored
   as `Result`, an interface holding `Values() []interface{}`, and `Get`
   recovers the value with `r.(CatalogProvider)`. That assertion is unchecked,
   so a wrong row type is a panic at run time rather than an error at compile
   time.
3. It exists to feed the writer. `SetColumns` and `SetScanValues` serve the
   `tblfmt` renderer. D5 leaves that writer in `usql`, so the reason for the
   shape leaves with it.

Return an iterator, `iter.Seq2[Catalog, error]`. D33 supersedes an earlier
version of this paragraph that preferred a typed slice. The package streams and
does not materialize a result.

Note the consequence for `usql`. Its writer consumes the cursor, so phase 5
must adapt the writer to a slice or an iterator. That work belongs to phase 5
and it is a reason to keep the writer in `usql` rather than an argument against
doing this.

## Compose readers explicitly, not by type assertion

`usql` builds a reader with `PluginReader`, which takes a list of readers and
type asserts each one against 15 interfaces, keeping the last that matches.
Read `reader.go` from line 31.

This is how the DuckDB fault happened. Nothing in the type system says which
reader answers which call, so a reader from the wrong driver satisfies the
interface and wins silently.

Name the parts instead. A driver states which reader answers each object, and a
missing one is a value that reads as missing rather than a failed assertion.
D4 keeps the interfaces, which are useful. It does not require this way of
combining them.

## The rest

Use generics instead of `interface{}` for a container of one type. Use
`errors.Is` and `errors.As` rather than comparing strings. Give every error a
wrapped cause with `%w`. Make the zero value useful where you can. Keep an
interface small and define it where it is consumed.

`gofmt` and `go vet` must both be clean. See `CLAUDE.md` for the command.
