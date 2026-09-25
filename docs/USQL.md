# What usql Answers Today, and What dbmeta Changes

`usql` is the reason the object model here follows `psql`. This measures what
`usql` can answer today, per database, and what changes if it reads `dbmeta`
instead of its own readers.

Everything below was measured rather than read. A program built `usql` with the
`most` build tag, constructed every driver's metadata reader, and asked each
one which reader interfaces it satisfies. The counts come from that.

Measured against `usql` at commit `382e1da` on `main`, with 47 drivers built.

## How usql reads metadata today

`usql` defines 14 reader interfaces in `drivers/metadata/metadata.go`, one per
object kind. A driver implements the ones it can. The writer asks for a reader
by type assertion and leaves out the section when the driver does not have one,
so a command degrades rather than failing.

The 14 kinds: Catalogs, Schemas, Tables, Columns, ColumnStats, Indexes,
IndexColumns, Triggers, Constraints, ConstraintColumns, Functions,
FunctionColumns, Sequences and PrivilegeSummaries.

Eight commands read them. `\l` needs Catalogs. `\dn` needs Schemas. `\dt`,
`\dv`, `\dm`, `\ds` and bare `\d` need Tables. `\d NAME` needs Tables and
Columns to print anything, and adds a section per further reader. `\di` needs
Indexes. `\df` and `\da` need Functions. `\dp` and `\z` need
PrivilegeSummaries. `\ss` needs ColumnStats.

## Which databases answer what

Of 47 drivers built, 20 have a metadata reader and 27 have none at all. A
driver with no reader answers no metadata command: `\dt` on it prints nothing
useful.

| Driver | Commands | `\d NAME` sections | Kinds it cannot read |
| --- | --- | --- | --- |
| postgres, pgx | 8/8 | 4/4 | none |
| cockroachdb | 8/8 | 4/4 | none |
| redshift | 8/8 | 4/4 | none |
| sqlserver | 8/8 | 4/4 | none, and it reads `information_schema` with sequences and constraints off |
| duckdb | 8/8 | 4/4 | none |
| trino | 8/8 | 4/4 | none |
| mysql, mymysql | 6/8 | 3/4 | Catalogs, ColumnStats, Triggers |
| memsql, tidb, vitess | 6/8 | 3/4 | Catalogs, ColumnStats, Triggers |
| snowflake | 6/8 | 3/4 | Catalogs, ColumnStats, Triggers |
| databend | 6/8 | 3/4 | Catalogs, ColumnStats, Triggers |
| nzgo (Netezza) | 6/8 | 3/4 | Catalogs, ColumnStats, Triggers |
| oracle | 6/8 | 1/4 | ColumnStats, Triggers, Constraints, ConstraintColumns, Sequences, PrivilegeSummaries |
| sqlite3, moderncsqlite | 5/8 | 1/4 | Catalogs, ColumnStats, Triggers, Constraints, ConstraintColumns, Sequences, PrivilegeSummaries |
| clickhouse | 4/8 | 0/4 | everything but Schemas, Tables, Columns, Functions |
| impala | 0/8 | 0/4 | its reader satisfies no interface |

The 27 with no reader at all: avatica, awsathena, bigquery, chai, cosmos, cql,
csvq, databricks, exasol, firebirdsql, flightsql, gocosmos, godynamo, h2, hdb,
hive, ignite, maxcompute, n1ql, ots, presto, ql, spanner, tds, vertica, voltdb,
ydb.

Most of the seven at 8/8 get there the same way: they are PostgreSQL, or they
are wire compatible with it and reuse its reader.

## What dbmeta changes

### It does not raise the command count for PostgreSQL

PostgreSQL already answers 8 of 8. What changes there is the number of object
kinds: 14 against 54. `COMMANDS.md` maps every `psql` metadata command to the
Go value that answers it, and 37 of them have no reader interface in `usql`
today. Tablespaces, types, domains, operators, text search, publications,
extensions, roles, access methods and the rest are all readable from `dbmeta`
and have nowhere to go in `usql` yet.

So for PostgreSQL the gain is not a command that starts working. It is that
`usql` could implement four times as many commands without writing SQL.

It also gains facts `psql` does not print. `Column.PrimaryKey` is the example
the policy was written around: `usql` reads it per column today and `dbmeta`
now returns it in the same row. D47 says when that is allowed, and the short
version is that the fact must come out of one statement.

### It raises the count for MariaDB, MySQL and SQLite

| Database | usql today | with dbmeta | What it gains |
| --- | --- | --- | --- |
| MariaDB | 6/8, 3/4 sections | 8/8, 4/4 | `\l` from Databases, `\ss` from ColumnStats, and the trigger section |
| MySQL | 6/8, 3/4 sections | 7/8, 4/4 | `\l` and the trigger section. MySQL cannot answer `\ss` |
| SQLite | 5/8, 1/4 sections | 6/8, 3/4 | `\l`, the constraint section and the trigger section |

MariaDB now reaches 8 of 8, because `ColumnStats` exists and MariaDB answers
it. MySQL and SQLite stay at 7 and 6, because neither can answer it and both
say so, which is a true answer rather than a missing one.

### It offers something to the 27 with nothing

`models/informationschema` answers 7 kinds for any database with a standard
`information_schema`, which is enough for `\dn`, `\dt`, `\d NAME`, `\df` and
`\dp`. A driver registers a profile saying how it differs from the standard,
which is tens of lines rather than a reader.

Which of the 27 have a usable `information_schema` is not measured here and
must not be guessed. Each one needs the D43 treatment: ask two models, then run
the queries against a real server.

## The gap, and what closed it

`usql` read three kinds that `dbmeta` could not answer for any database:
ColumnStats, ConstraintColumns and FunctionColumns. A migration would have lost
all three. D46 named them, D47 set the policy for adding them, and all three
now exist.

`dbmeta.ConstraintColumns` is the column level detail of a constraint: which
column, in what position, and for a foreign key which column of which table it
points at. Every model answers it, including SQLite.

`dbmeta.RoutineParameters` is `usql`'s FunctionColumns: the name, position,
direction and type of each parameter. Every model except SQLite answers it,
and SQLite cannot, because a function there is compiled C with no named
parameters.

`dbmeta.ColumnStats` backs `\ss`. PostgreSQL, MariaDB and SQL Server answer it.
MySQL, SQLite and DuckDB cannot, and say so rather than returning rows that are
almost all absent. `usql` implements `\ss` today for PostgreSQL, DuckDB, Trino,
CockroachDB, Redshift and SQL Server. Two of those six are covered, and DuckDB
is not: `usql` prints statistics there that `dbmeta` refuses, because DuckDB
has no catalog of them.

### What is still missing for a lossless migration

Nothing, for the databases `dbmeta` models. `usql` implements `\ss` for six
drivers and `dbmeta` answers it for two of them, so a migration of the other
four waits on a model for Trino and on what CockroachDB and Redshift answer
through the PostgreSQL model, rather than on a missing kind.

Two `usql` reader kinds have no `dbmeta` equivalent by design.
ConstraintColumns replaces both ConstraintColumns and the column part of
Constraints, and FunctionColumns became RoutineParameters. The names differ and
the facts do not.

## How usql would use dbmeta

The shape is already there. `usql`'s `Reader` interfaces and `dbmeta`'s
`Query[T]` values line up one for one, so the move is per driver and reversible.

Read the version once, when the connection opens, and build a `Meta`:

```go
versions, err := dbmeta.PostgreSQL.Version(ctx, db)
if err != nil {
    return err
}
m, err := dbmeta.New(dbmeta.PostgreSQL, versions)
```

Then one `usql` reader method becomes one loop:

```go
func (r *reader) Tables(f metadata.Filter) (*metadata.TableSet, error) {
    args := dbmeta.Args{Schema: f.Schema, Name: f.Name, WithSystem: f.WithSystem}.Map()
    var out []metadata.Table
    for v, err := range dbmeta.Tables.All(r.ctx, r.meta, r.db, args) {
        if err != nil {
            return nil, err
        }
        out = append(out, metadata.Table{Schema: v.Schema, Name: v.Name, Type: v.Type})
    }
    return metadata.NewTableSet(out), nil
}
```

`dbmeta.Args` is already the same four filters `metadata.Filter` carries, and
`dbmeta.Querier` is one method, so whatever `usql` already holds satisfies it.

Two things the writer gets for free. `Query.Support(m)` says whether a database
can answer at all, so `usql` can tell "this database has no such object" from
"there are none", which today it cannot. And `Field.Min` and `Field.Present`
say whether a NULL means the value is null or the server is too old to have the
column, which `usql` has no way to express.

Do not move the writer. `dbmeta` only reads, and `tblfmt` renders. That
division is D5 and it does not change.

The order that loses nothing: move one driver, then the rest. PostgreSQL is the
wrong one to move first because it already works. MariaDB is the right one,
because it goes from 6 commands of 8 to all 8, and the difference is visible
the moment it lands.

The three kinds that blocked this are done. What is left is `usql`'s own work:
a reader per driver, and whatever filtering each command needs so that the
output still matches `psql`.
