# D4. Keep the object coverage, drop the Reader naming

Status: Decided.

The 15 one method interfaces in `usql/drivers/metadata/metadata.go` are useful
for what they cover, which is one interface per kind of object. They are not
useful as a naming scheme. D5 drops the `Reader` and `Writer` concept, so
`TableReader` and `ColumnReader` do not carry over under those names.

Take the object coverage. Leave the names and the composition.

The name `Reader` carried information in `usql` because a `Writer` sat beside
it. In `dbmeta` everything reads, so `Reader` says nothing and repeats the
package. `dbmeta.TableReader` with a `Tables` method is three words for one
idea.

## The further question is answered: there are no per object interfaces

This decision once asked whether `dbmeta` exports those fifteen as
interfaces at all, and said not to settle it before D13 delivered the models.
D13 delivered eight. The shape they produced is the one this decision hoped
for, and the code is now the record of it.

The root package exports two interfaces and neither is per object.
`Queryer` declares `QueryContext` and nothing else, which is D49.
`AnyQuery` is what lets a caller hold queries of different row types in one
list. There is no `TableReader`, no `ColumnReader` and no composition by type
assertion, which is what D18 rejected.

An object kind is a `Query` value rather than an interface. A caller writes
`dbmeta.Tables.All(ctx, m, db, args)` and gets an iterator, and the same four
arguments read every other kind. The interface a consumer wants is the one it
declares for itself, which is the rule in `CLAUDE.md` and the reason nothing
here declares it for them.

## Read on rather than designing from here

The design is settled and it is recorded further down, not here. This decision
is the earliest one and it predates every model. A reader who takes it as the
current shape of the API will be several years of decisions out of date.

D39 has how a query is listed, described and rendered. D33 has why a result
streams as an iterator rather than arriving as a slice. D34 has what happens
when a database cannot answer. D47 has the cost test for whether a field
belongs here at all, and why a child of an object is its own kind with flat
rows rather than a slice on the parent. D49 has `Queryer` and why the
interface is documentation rather than a seam for a mock.
