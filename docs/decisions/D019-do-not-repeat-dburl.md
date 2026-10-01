# D19. Do not repeat dburl

Status: Half decided, half overtaken by the code, amended by D125.

The half that stands: `dbmeta` must not carry its own list of schemes, its own
aliases, its own flavor table, or its own connection string parser. That
taxonomy belongs to `github.com/xo/dburl`, and a second copy of it is how two
copies start disagreeing. Everything below about what `dburl` records is still
how a consumer reads a URL.

The half that was wrong: this decision made `dburl` a direct dependency, and it
never became one. `dbmeta` imports the standard library and nothing else, and
its `go.mod` has no `require` block at all.

## Why the dependency never happened

The API took the shape that made it unnecessary. A caller opens its own
connection, passes a `DB`, and names a `Dialect`. No URL ever reaches this
module, so there is nothing here to parse and nothing to look up. D36 decided
that the client drives, and this is a consequence of D36 that nobody noticed
until the code was written.

A zero dependency library is the better answer, and it is strictly stronger
than the rule it replaces: a `go.mod` with no `require` block cannot acquire a
transitive dependency, and `depguard` now refuses one at lint time. A consumer
that has a URL imports `dburl` itself, which it was going to do anyway.

Do not add `dburl` back to reach the taxonomy below. Read it from a consumer.

## What dburl is still for

Every package that uses `dbmeta` is expected to use `dburl` as well, and that
is where a connection string is handled.

Do not repeat any part of it. `dbmeta` must not carry its own list of schemes,
its own aliases, its own flavor table, or its own connection string parser. If
`dbmeta` needs to know something about a database URL, `dburl` answers it.

An earlier draft of the testing plan said to copy the flavor taxonomy out of
`dburl` as data to protect D7. That instruction is withdrawn. Import
`dburl` and read it.

## dburl already draws the distinction that D14 needs

D14 separates a family from a flavor. `dburl` already separates them and it
exposes both on a parsed URL. Read `dburl.go:172` to see it:

```go
u.Driver, u.UnaliasedDriver = scheme.Driver, scheme.Driver
if scheme.Override != "" {
    u.Driver = scheme.Override
}
```

Three fields matter to `dbmeta`.

`URL.Driver` names the family, and it selects the model package. For
`cockroach://` it is `postgres`, because the `cockroachdb` scheme sets
`Override` to `postgres`.

`URL.UnaliasedDriver` names the wire compatible product. For `cockroach://` it
is `cockroachdb`. This is the flavor for a product that has its own scheme, and
`dburl` records five of them: `cockroachdb` and `redshift` over `postgres`, and
`memsql`, `tidb` and `vitess` over `mysql`.

`URL.OriginalScheme` holds what the caller typed. This is the flavor for a
product that `dburl` treats as an alias rather than a scheme. The `mysql`
scheme carries the aliases `mariadb`, `maria`, `percona` and `aurora`, so
`mariadb://` parses with `Driver` and `UnaliasedDriver` both set to `mysql`,
and only `OriginalScheme` records that MariaDB was asked for.

Note the consequence. The two kinds of flavor arrive on different fields. A
wire compatible shows up in `UnaliasedDriver`. An alias shows up only in
`OriginalScheme`. Code that reports the flavor must read both, and a test that
covers only one will miss half the matrix that the testing plan describes.

`SchemeDriverAndAliases` at `scheme.go:512` resolves a scheme name to its driver
and its aliases, and applies `Override` the same way. Use it rather than
reading the scheme table.
