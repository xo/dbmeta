# D80. The driver registry is dburl's, and reading it is not importing it

Status: Decided.

`dburl` v0.29.0 describes every scheme it registers. `dburl.Scheme` gained
`Desc`, `Home`, `GoPackage`, `DriverURL`, `RequiresCGO` and `Deployment`, so
the question step 3 of `docs/DIALECT.md` asks has an authoritative answer in
one place for the first time. Step 3 and hard rule 10 now name that registry.

Nothing in the root module changes and nothing is imported. Hard rule 1
forbids the dependency, D19 removed it once already, and none of this is a
reason to bring it back: `dbmeta` takes a `DB` and a `Dialect` and has no URL
to parse. The registry is a document here, read by a person adding a dialect
and by nothing at build time.

## Why the grep it replaces gave a wrong answer

Step 3 said `grep -rn "// DRIVER" ~/src/go/src/github.com/xo/usql`. That finds
an import that carries the comment. Oracle does not have one: `oracle` and
`godror` both register through `orshared.Register`, so the grep answers for
every product except the one this project spent a day getting wrong. The
registry names `github.com/sijms/go-ora/v3` for `oracle` and
`github.com/godror/godror` for `godror`, and puts `RequiresCGO: true` on the
second, which is D48's rule written as data rather than as prose here.

Keep the grep as a second look. It shows what `usql` actually imports, and the
registry shows what it says it imports. Where the two disagree, one of the two
projects has a defect and the disagreement is the finding.

## What the registry does not answer

The version. D52 requires the same package and allows a different version, and
the version lives in `usql`'s `go.mod`. Step 3 now reads two things: the
registry for the package, and that `go.mod` for the version `usql` pins.
Oracle is the standing example, because D59 holds `dbmeta` on `go-ora/v2`
while the registry and `usql` both name v3.

A scheme with no `GoPackage` is not a hole. It is how the registry says the
scheme borrows another scheme's driver, and it is the same fact
`URL.UnaliasedDriver` reports at parse time. Seven schemes are in that state
today: `cockroachdb`, `memsql`, `redshift`, `tidb`, `vitess`, `oleodbc` and
`file`.

## Scheme.Deployment and Info.Embedded both stay

`DeploymentEmbedded` and `dbmeta.Info.Embedded` say the same thing about the
same products and neither replaces the other. `dbmeta` cannot read the first,
which settles it, and the two do not even count the same objects: `dburl`
marks `sqlite3`, `moderncsqlite` and `duckdb`, which is three schemes, and
`dbmeta` marks two models, because one model covers both SQLite drivers.
Neither number is wrong and neither is derivable from the other.
