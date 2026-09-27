# D5. dbmeta only reads

Status: Amended by D56.

`dbmeta` reads metadata. It does not render it and it does not write it.

D56 amends the second half of that by a hair and is worth reading with this.
`dbmeta` builds one statement that writes, the one that sets a password, and
does not run it. Everything `dbmeta` executes is still a read.

The `tblfmt` based writer stays in `usql`. That file,
`usql/drivers/metadata/writer.go`, is 838 lines and it is the only file that
uses `tblfmt` and `usql/env`. Leaving it behind keeps both out of this module
and is part of how D7 is met. It also uses `dburl`, across the eight `Writer`
methods at lines 148 to 162, but that is no longer a reason either way, because
D19 makes `dburl` a direct dependency.

The `Reader` and `Writer` pair is a `usql` concept and it does not come here.
Ken will change those names in `usql` itself. Do not design `dbmeta` around
them and do not preserve them for the sake of an easier phase 5.

One consequence belongs to phase 5. The `usql` writer consumes the cursor types
that D18 removes, so adapting it is part of integrating, not a reason to keep
the old shape.
