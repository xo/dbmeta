# What dbtpl Reads, and What dbmeta Would Have To Add

`dbtpl` generates Go code from a database schema. It reads metadata with
queries of its own, in `models/*.dbtpl.go`, dispatched through a `Loader` struct
of function fields in `loader/loader.go`.

This measures what `dbtpl` reads, how much of it `dbmeta` already answers, and
what `dbmeta` would have to add before `dbtpl` could stop carrying its own SQL.

Measured against `dbtpl` at commit `8366044`.

## What dbtpl reads

The `Loader` has 17 fields. Eleven read metadata, four write or drop a view,
and two are helpers.

| Loader field | What it returns | postgres | mysql | sqlite3 | sqlserver | oracle |
| --- | --- | --- | --- | --- | --- | --- |
| `Schema` | the name of the current schema | yes | yes | yes | yes | yes |
| `Tables` | name, type, and the definition of a view | yes | yes | yes | yes | yes |
| `TableColumns` | ordinal, name, type, not null, default, is primary key, comment | yes | yes | yes | yes | yes |
| `TableSequences` | which columns the database fills itself | yes | yes | yes | yes | yes |
| `TableForeignKeys` | key name, column, referenced table, referenced column, key id | yes | yes | yes | yes | yes |
| `TableIndexes` | name, unique, primary | yes | yes | yes | yes | yes |
| `IndexColumns` | position, column id, column name | yes | yes | yes | yes | yes |
| `Procs` | id, name, kind, return type, return name, body | yes | yes | no | yes | yes |
| `ProcParams` | parameter name and type | yes | yes | no | yes | yes |
| `Enums` | the enum types of a schema | yes | yes | no | no | no |
| `EnumValues` | each label and its sort order | yes | yes | no | no | no |

The four view operations, `ViewCreate`, `ViewSchema`, `ViewTruncate` and
`ViewDrop`, write rather than read. They stay in `dbtpl`. `dbmeta` only reads,
which is D5 and does not change.

## What dbmeta already answers

Nine of the eleven now map onto a `dbmeta` query. Six always did, and three
arrived with the kinds D47 added.

| dbtpl | dbmeta | Note |
| --- | --- | --- |
| `Tables` | `dbmeta.Tables` | the view definition is `dbmeta.Views`, a kind of its own |
| `TableColumns` | `dbmeta.Columns` | exact: `Column.PrimaryKey` is in the same row |
| `TableSequences` | `dbmeta.Columns` | `Column.Identity` says which column the database fills, which is what this asks |
| `TableIndexes` | `dbmeta.Indexes` | exact: name, unique and primary are all there |
| `IndexColumns` | `dbmeta.IndexColumns` | exact, and `dbmeta` adds the collation and the direction |
| `Procs` | `dbmeta.Functions` | exact: `Function.ID` is the oid `dbtpl` joins on |
| `ProcParams` | `dbmeta.RoutineParameters` | exact, except on SQLite, which has no named parameters |
| `TableForeignKeys` | `dbmeta.ConstraintColumns` | exact, including the referenced column and the position in a composite key |
| `Schema` | `dbmeta.CurrentSchema` | one row, read with `dbmeta.First` |

`dbmeta` also answers these for MariaDB and SQLite, which `dbtpl` supports, and
`dbtpl` would gain nothing new there because it already has them. What it would
gain is not having to maintain five dialects of the same query.

### Which databases answer everything dbtpl needs

The nine above are what `dbtpl` reads to generate code. This is how many of
them each model answers, counted from the queries the models register rather
than from memory.

| Database | Of the nine | What is missing, and why |
| --- | --- | --- |
| PostgreSQL | 9 | nothing |
| MariaDB and MySQL | 9 | nothing |
| SQL Server | 9 | nothing |
| Oracle | 9 | nothing |
| DuckDB | 8 | `IndexColumns`: DuckDB names an index and does not list the columns of it |
| SQLite | 8 | `RoutineParameters`: a SQLite function has no named parameters |
| ClickHouse | 7 | `ConstraintColumns` and `RoutineParameters`: a CHECK holds an expression rather than columns, and a function is overloaded across types with no signature recorded |
| Cassandra | 7 | `CurrentSchema` and `RoutineParameters`: CQL has no expression for the current keyspace, and arguments are two parallel lists on the function's own row |
| any `information_schema` | 7 | `Indexes` and `IndexColumns`: the standard has no index at all |

Four answer all nine: PostgreSQL, the MySQL dialect, SQL Server and Oracle.
Those are the four a `dbtpl` built on `dbmeta` could generate from with nothing
missing.

Every gap above is the product rather than the model. `dbtpl` supports
PostgreSQL, MySQL, SQL Server, Oracle and SQLite today, so the only one of its
own databases that is short is SQLite, by one query, for a reason `dbtpl`
already knows: it writes no parameter names for SQLite either.

## What dbmeta added

Five things, three of them shared with `usql`. All five now exist, under D47.
What follows is what each one became and what it still cannot do.

**Routine parameters** became `dbmeta.RoutineParameters`, with a name,
position, mode and type per parameter. PostgreSQL, MariaDB, MySQL and the
shared model answer it. SQLite cannot: a function there is compiled C with no
named parameters.

Group by `Routine` and, where the database overloads a name, by `RoutineID`.
`Function.ID` carries the same value, so the join is one expression for every
database. On PostgreSQL it is the oid, which is what `dbtpl` uses today.

**Constraint columns** became `dbmeta.ConstraintColumns`, with the column, its
one based position within the constraint, and for a foreign key the catalog,
schema, table and column it points at. Every model answers it, SQLite included.

This was the largest gap, and it is closed exactly the way `dbtpl` needs: a
composite key is several rows sharing a constraint name, ordered by `Ordinal`,
each paired with the column it references.

**Enum values as rows** became `dbmeta.EnumValues`, and only PostgreSQL answers
it. A label there is a row with a one based ordinal, which is what a generated
Go constant needs.

MariaDB and MySQL cannot, and this is the one place `dbtpl` gains nothing. They
have an enum column rather than an enum type, and the labels exist only inside
the `enum('red','green','blue')` text of `COLUMN_TYPE`. Splitting that
correctly means tracking quoting, because a label may hold a comma or an
escaped quote, and no portable SQL does that. `dbtpl` splits it in Go today and
has the same limitation, so nothing is lost by keeping that where it is.
`Columns.DataType` returns the text verbatim.

**The definition of a view** became `dbmeta.Views`, a kind of its own rather
than a field on `Table`. Reaching the definition costs a join or a function
call per row, and a caller listing tables should not pay it. Every model
answers it.

**The current schema** became `dbmeta.CurrentSchema`, which answers one row and
is read with `dbmeta.First`. It is session dependent and says so. Every model
answers it.

### The two smaller ones

`Column.PrimaryKey` is now a field on `Column`. It was the example the whole
policy was written around, and the answer is that it is free on MySQL and
SQLite, which already select it, and one join on PostgreSQL. `dbtpl` reads it
per column and no longer needs a second query.

`Function.ID` is now a field on `Function`, and `RoutineParameters` carries the
same value. PostgreSQL returns the oid, which is what `dbtpl` joins on today.
Everything else returns the name or the specific name, because nothing else
here overloads a routine.

## How dbtpl would use dbmeta

Unlike `usql`, `dbtpl` reads per schema and per table rather than listing, and
it already passes a context and a `DB` everywhere. The shape fits.

A `Loader` field becomes a closure over a `Meta`:

```go
TableIndexes: func(ctx context.Context, db models.DB, schema, table string) ([]*models.Index, error) {
    args := dbmeta.Args{Schema: schema, Parent: table}.Map()
    var out []*models.Index
    for v, err := range dbmeta.Indexes.All(ctx, m, db, args) {
        if err != nil {
            return nil, err
        }
        out = append(out, &models.Index{
            IndexName: v.Name, IsUnique: v.Unique, IsPrimary: v.Primary,
        })
    }
    return out, nil
},
```

Three things `dbtpl` gets that it does not have today.

`Query.Support(m)` says whether a database has an object kind at all. `dbtpl`
today sets a `Loader` field to nil for that, which means the knowledge lives in
five separate files rather than in the model.

Version gates. `dbtpl`'s queries have none, so a query written against a recent
PostgreSQL either works on 9.6 or does not, and nothing says which. `dbmeta`
carries the gates and the padding rule, so a column absent on an old release
arrives as NULL with `Field.Min` saying why.

The fixtures. `models/<driver>/fixture` builds a schema with one of every object
the queries read. `dbtpl` generates against a live database and needs one, and
that was the reason the fixtures are exported rather than kept in the tests.
`dbtpl` can generate against `fixture.Everything` instead of a schema someone
has to build by hand.

### The dependency runs one way

`dbmeta` must not import `dbtpl`, which is hard rule 7. There is no dependency
in either direction today: `dbmeta` does not use `dbtpl` for anything, and
nothing here is generated, which is D71. If `dbtpl` starts reading `dbmeta`,
the rule still holds with one arrow: `dbtpl` imports `dbmeta`, and `dbmeta`
keeps its zero dependencies.

### The order that loses nothing

The five kinds exist, so the order is about risk rather than blocking.

Move `TableIndexes` and `IndexColumns` first: they map exactly and a mistake
shows immediately in generated code. Then `Tables` and `TableColumns`, which
gains `PrimaryKey` in the same row. Then `TableForeignKeys` onto
`ConstraintColumns`, which is the one that could not be approximated and now
can be read directly. Then `Procs` and `ProcParams` together, joined on
`Function.ID`.

Leave `Enums` and `EnumValues` where they are for MariaDB and MySQL. `dbmeta`
answers them only for PostgreSQL, so moving them would mean two code paths
rather than one.
