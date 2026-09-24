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

Six of the eleven map onto a `dbmeta` query with no work.

| dbtpl | dbmeta | Note |
| --- | --- | --- |
| `Tables` | `dbmeta.Tables` | except the view definition, below |
| `TableColumns` | `dbmeta.Columns` | except `IsPrimaryKey`, below |
| `TableSequences` | `dbmeta.Columns` | `Column.Identity` says which column the database fills, which is what this asks |
| `TableIndexes` | `dbmeta.Indexes` | exact: name, unique and primary are all there |
| `IndexColumns` | `dbmeta.IndexColumns` | exact, and `dbmeta` adds the collation and the direction |
| `Procs` | `dbmeta.Functions` | except the id, below |

`dbmeta` also answers these for MariaDB and SQLite, which `dbtpl` supports, and
`dbtpl` would gain nothing new there because it already has them. What it would
gain is not having to maintain five dialects of the same query.

## What dbmeta would have to add

Five things. Three of them are the same three `usql` needs, which is the
argument for adding them: two independent consumers want them, and neither can
migrate without them.

**Routine parameters.** `dbtpl` calls them `ProcParams` and `usql` calls them
`FunctionColumns`. `dbmeta` has `Function.ArgTypes`, one string, which a person
can read and a code generator cannot use. `dbtpl` needs a name and a type per
parameter, and `usql` needs a direction, a position and a size as well. One new
kind answers both.

**Constraint columns.** `dbtpl` calls them `TableForeignKeys` and needs the
column, the referenced table, the referenced column and a key id so that a
composite key stays together. `usql` calls them `ConstraintColumns`. `dbmeta`
has `Constraint.Definition`, one string. Again one new kind answers both.

This is the largest gap for `dbtpl`. A code generator cannot parse
`author_id -> author(author_id)` out of a string and be right about every
database.

**Enum values as rows.** `dbmeta.Types` reports `Kind` as `enum` and puts the
labels in `Type.Elements`, joined with commas. `dbtpl` needs a row per label
with its sort order, because it generates a Go constant per label. Splitting a
string is not good enough: a label can contain a comma.

MySQL is the awkward one and `dbtpl` already handles it. MySQL has no enum
type, only an enum column, so `dbtpl` reads
`information_schema.columns WHERE data_type = 'enum'` and takes the column name
as the enum name. Any `dbmeta` answer has to allow that shape.

**The definition of a view.** `dbtpl.Table.ViewDef` holds the SQL of a view, and
`dbmeta.Table` has no such field. `dbtpl` uses it to know what a view selects.

**The current schema.** One value, and every `dbtpl` loader reads it:
`CURRENT_SCHEMA()` on PostgreSQL, `DATABASE()` on MySQL, and so on. `dbmeta`
has the expression already, as `informationschema.CurrentSchema`, and no query
that returns it.

### Two smaller ones

`Column.IsPrimaryKey`. `dbtpl` reads it per column. `dbmeta` answers it through
`Constraints` or `Indexes`, which means a second query and a join in Go. A
caller can do that, and it is worth asking whether the column should carry it.

A stable identity for a routine. `dbtpl` reads `Procs` and then `ProcParams`
per routine, joined by `Proc.ProcID`, which is the PostgreSQL oid. `dbmeta` has
no identifier on `Function`, so a parameters query needs something to join on.
Overloading makes the name insufficient: PostgreSQL allows two functions with
one name and different arguments.

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

`dbmeta` must not import `dbtpl`, which is hard rule 7. Today `dbmeta` pins
`dbtpl` as a tool for generating models, and that is the only relationship. If
`dbtpl` starts reading `dbmeta`, that stays true: `dbtpl` imports `dbmeta`, and
`dbmeta` keeps its zero dependencies.

### The order that loses nothing

Add the five kinds. Move `TableIndexes` and `IndexColumns` first, because they
map exactly and a mistake shows immediately in generated code. Then `Tables`
and `TableColumns`. Leave `TableForeignKeys` until constraint columns exist,
because that is the one that cannot be approximated.
