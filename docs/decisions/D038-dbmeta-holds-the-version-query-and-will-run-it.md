# D38. dbmeta holds the version query, and will run it on request

Status: Amended in place.

`dbmeta` holds the version query for every database, because that knowledge
belongs with the metadata queries.

## The original reasoning was wrong

The first version of this decision said `dbmeta` "cannot run one, because D26
gives it no driver and D36 gives it no connection", and exposed only the two
step form. Writing the examples showed that premise is false, and it is
corrected here rather than quietly dropped.

Running a query needs no driver, only the `DB` interface, and the caller hands
one over for every `Query.All` call. `dbmeta` already runs queries against a
caller supplied connection. There was never a reason the version query was
different.

What D36 actually requires is that `dbmeta` decides nothing. It must not detect
a version behind the caller's back and must let the caller override. A method
the caller chooses to call satisfies that. A constructor that silently probes
the server does not.

## Compare it against usql, every time

Adding or changing a dialect means comparing its version query against the one
`usql` runs for the same product, and recording the comparison in
`docs/USQL.md`. This sits alongside the other requirements for a dialect: the
fixture in rule 9, the second opinion in D43, the driver in D52 and the
principals in D61.

Find `usql`'s side in `usql/drivers/<driver>/<driver>.go`, in the `Version`
field of the registered `drivers.Driver`. A driver that declares none falls
through to `drivers.Version`, which runs the generic `SELECT version();`, and
that fallback is part of the comparison rather than an absence of one.

Record one row per model in the statements table in `docs/USQL.md`, saying
what each side runs and whether the answers agree. Three outcomes are normal
and each is written differently. The same statement is the common case. A
different statement with the same answer is fine and is left alone, with the
reason the two differ. A different answer is a decision: say which side reads
more and why this one does what it does.

The reason this is a rule is that it was not done, and the gap was invisible
from the other end. `docs/USQL.md` already compared the printed version lines
across 26 servers and called the coverage complete. It compared output rather
than statements, and it predated three models, so nobody noticed that `usql`
declares no `Version` function for Oracle at all. It falls through to
`SELECT version();`, Oracle answers `ORA-00904: "VERSION": invalid
identifier`, `drivers.Version` discards the error, and `usql` prints
`<unknown>`. Comparing the two statements finds that in a minute. Comparing
the two printed lines finds it only if somebody notices Oracle is missing from
the table.

`TestEveryModelIsInTheVersionTable` holds it: a model with no row in that
table fails.

## Both forms exist

The one step form is what nearly every caller wants:

```go
// Version runs the version statement against db and parses the result.
func (d Dialect) Version(ctx context.Context, db DB) (VersionSet, error)
```

A method named `Version` coexists with the `Version` type. A method name lives
in the method set of its receiver, not in the package scope, so there is no
collision. `Dialect.Version`, `Dialect.VersionQuery` and `Dialect.ParseVersion`
then read as one group.

The two step form stays, for a caller that wants the statement without running
it. `usql` prints statements in its trace output and needs this:

```go
// VersionQuery returns the statement and how many columns it returns.
func (d Dialect) VersionQuery() (sql string, cols int, ok bool)

// ParseVersion parses the columns of the first row.
func (d Dialect) ParseVersion(cols []string) (VersionSet, error)
```

The one step form exists because the two step form made every caller build a
slice of pointers into a slice of strings to scan into. That is the same nine
lines in every consumer, which is a sign the package drew the line in the wrong
place.

Neither form takes override away. A caller that wants to force a version builds
a `VersionSet` and passes it to `New`, and never calls either.

```go
// VersionQuery returns the query that reads the server version. The second
// result is false when the database reports no version.
func (d Dialect) VersionQuery() (VersionQuery, bool)

// ParseVersion parses the columns of the first row.
func (d Dialect) ParseVersion(cols []any) (VersionSet, error)

type VersionQuery struct {
	SQL     string
	Columns []Column
}
```

The column count is part of the two step form and the client must be told it,
because two databases return more than one column. SQL Server returns five: the
product name, the version, the level, the update level and the edition.
Cassandra returns three independent versions.

Parsing produces both things the client needs from one call. `VersionSet` gates
the queries. `Display` is the line `usql` prints, and it is built where the
shape is known rather than by the client guessing. For SQL Server that is

	Microsoft SQL Server 2022 16.0.4295.3, RTM-CU27, Developer Edition (64-bit)

Only the version gates anything. The rest is for the person reading it, and
each part is left out when the server did not report it, so a release with no
update level reads `RTM` rather than `RTM-`.

Four of the five are server properties and the product name is not. No
`SERVERPROPERTY` returns the name the product is sold under: `ProductMajorVersion`
says 16 and nothing says 2022. Only `@@VERSION` carries it, in its first words,
so the name is cut from there. The alternative is a table mapping a major
number to a year, which needs an edit for every release Microsoft ships, and
reading it from the server does not.

A column can arrive NULL, and `productupdatelevel` does on a release older than
the one that added it. `Dialect.Version` scans every version column as nullable
for that reason, and hands the parser empty text. That is the one place
flattening a NULL is right, because a version line is a single string shown to
a person, and a property that is absent and a property that is empty both mean
there is nothing to print. `docs/NULLS.md` governs a catalog column, where the
two are different facts.

A database with no version returns false, and the client uses an unknown
version, which D36 treats as newest.
