# D9. There are two platonic models. PostgreSQL is the primary one

Status: Decided.

A platonic model is an idealized shape that a real database only approximates.
`dbmeta` implements two, and they rank.

The primary model is PostgreSQL itself, and more exactly the metadata commands
of `psql`, the PostgreSQL command line client. These are the `\d` family, such
as `\dt` for tables and `\df` for functions. `psql` implements them in one C
file named `describe.c`. That file is the reference for what a metadata query
must return and for how it must behave.

The second model is the generic `information_schema`. Standards bodies define
it, many databases provide it, and every database varies it.

Read the ranking narrowly. It decides the shape and the naming of an object
that two databases both have. When PostgreSQL and `information_schema`
describe the same thing differently, follow PostgreSQL. Use
`information_schema` for the databases that offer nothing better.

It is not a requirement that every database answer all 48 PostgreSQL objects.
Both external reviews read it that way and warned that most drivers then
return "not supported" for most calls. That reading is wrong, and the wording
above is narrowed to prevent it. A database answers for the objects it has, and
the capability mechanism D34 describes reports the rest. PostgreSQL sets the
shape of the answer, not the list of questions every database must answer.

## The PostgreSQL model and pgdesc

The repository `github.com/xo/pgdesc` already holds a machine translation of
`describe.c` into Go. Do not start this model from nothing. Read `pgdesc`
first.

Facts about `pgdesc` that an agent needs:

- `pgdesc.go` is 3786 lines and holds about 30 describe functions. `gen.go`,
  at 804 lines, is the translator that produced it.
- It covers far more objects than the 15 that `usql` models. It has access
  methods, aggregates, casts, collations, conversions, domains, event triggers,
  extensions, foreign data wrappers, foreign servers, foreign tables,
  languages, operators, publications, subscriptions, roles, default access
  control lists, and text search parsers and configurations.
- The last commit is from January 2019, so it reflects a PostgreSQL release
  around 11. Regenerate it against a current release before you trust it.
- Its `TODO` file lists seven known faults in the translation, including a
  broken ternary operator and queries that need splitting into separate
  functions.

The object list above sets the target for the root package. The 15 object types
in `usql` are a subset of it, not the goal.

## How psql handles versions, and how dbmeta differs

`psql` does not keep one query per release. It builds one query and switches
fragments on an integer server version. The generated Go shows the pattern as
`if d.version < 90600`, where 90600 means release 9.6.0. When the server is too
old for a feature, `psql` returns an error that names the version.

`dbmeta` takes the other path. D8 sets discrete fragments per version, because
each is a concrete SQL statement that ran against a concrete server.
An inline conditional has no single statement to check.

Two consequences follow. An agent that ports a query from `describe.c` or from
`pgdesc` must read the version conditionals and split them into one statement
per supported version. The integer form of the version, such as 90600 and
180000, is the natural key for selection, and `SHOW server_version_num` returns
it directly.

## The information_schema model and its variations

The variations are the point. A driver does not get its own copy of the
`information_schema` queries. It gets the shared queries plus a description of
how it differs. `usql` already works this way, and its
`informationschema.InformationSchema` type is the model to carry over. It
describes a database in four ways.

1. Eight feature flags say whether the database has a thing at all:
   `hasFunctions`, `hasSequences`, `hasIndexes`, `hasConstraints`,
   `hasCheckConstraints`, `hasTablePrivileges`, `hasColumnPrivileges`, and
   `hasUsagePrivileges`.
2. Fifteen named clauses replace one fragment of SQL each. The names are values
   of the `ClauseName` type, such as `columns.data_type` and
   `privileges.grantor`. MySQL, for example, replaces `data_type` with
   `column_type` and replaces the deferrable clauses with an empty string.
3. A placeholder function writes the bind parameter for the database, `$1` for
   postgres and `?` for MySQL.
4. Two lists name the system schemas to hide and the expression that gives the
   current schema.

Carry this mechanism into `dbmeta`, with one change that D8 forces. Today the
description is fixed per driver. It must become fixed per driver and version,
because a database gains and loses `information_schema` features between
releases.

Carry it with a second change that the shared instance hazard forces. See the
section on known defects. The description must be a value the caller owns, not
a package level variable that another driver can borrow.
