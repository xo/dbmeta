# D220. BigQuery gets a model that reads the hosted service

Status: Decided, amended by D226 and D229.

## The decision

Ken asked on 2026-10-10 for a model for Google BigQuery, in the GoogleSQL
dialect, and D218 made it one of the hosted services that get a model. The model
is `models/bigquery`. It answers 15 of the 65 kinds: tables, columns, indexes,
index columns, constraints, constraint columns, views, functions, aggregates,
routine parameters, comments, partitioned tables, partitions, the current user
and databases.

It reads the hosted service and nothing else. goccy's bigquery-emulator, which
`dbrun` can start, answers four views of INFORMATION_SCHEMA, so no model reads it
and its entry stays Staged (D118, D119).

## What it reads

The views of INFORMATION_SCHEMA that belong to one dataset, and the legacy
metatable `__TABLES__`, which holds the exact size and row count of every table
in the dataset. The statements name `INFORMATION_SCHEMA.TABLES` with no dataset
in front, and the query job carries a default dataset that the driver takes from
the last part of the URL. So a connection reads one dataset, and the catalog of a
row is the project. A statement cannot bind the name of a view, so the model has
no way to read a second dataset, and the walk of D146 is for products with no
catalog that a SELECT reads and is not allowed here.

The views of the project and of the region need a permission on the project. The
principals of the test account hold one on the dataset, so SCHEMATA,
SCHEMATA_OPTIONS and TABLE_STORAGE answer 403, and OBJECT_PRIVILEGES answers only
a query that names one object. `Schemas`, `CurrentSchema`, `Privileges` and the
rest that need them are not answered, and `docs/COVERAGE.md` and
`docs/BACKLOG.md` say so.

## The driver

dburl v0.49.0 names `gorm.io/driver/bigquery/driver` for the bigquery scheme, and
the test module uses v1.2.1 of it (D154). A later dburl names
`github.com/xo/dbimp/bigquery`, which is not tagged yet. The test module follows
dburl when a tag names it, as it did for rqlite (D148, D151), and `dbrun` and the
tests then change together. The driver takes a key file in the option
`credential_file`, so the parity test names the key of the reader there. The DSN
holds a path and no secret.

## The version

BigQuery is a service with no release, and no function or view reports one. The
model declares no version query, so the version is unknown. usql declares no
`Version` for its bigquery driver either, so it runs `SELECT version();`, which
BigQuery refuses. D216 chose a number that SQL reads for Spanner, and BigQuery has
none.

## The root package changes by one rule

A statement that reads a system variable, such as `@@project_id`, has two at
signs, and the code that rewrites `@name` into a placeholder read the second one as
a parameter. It now leaves `@@name` in the statement. MySQL and SQL Server also
have system variables, and no statement of theirs reads one today, so no answer
changes.

## The analogues

Each is a choice that Ken can reverse:

- A search index and a vector index are indexes.
- A user defined aggregate function is an aggregate.
- A partition is the part of a table that a date or a number selects, and a table
  decorator names it. A partitioned table is a table with PARTITION BY in its DDL.
  The strategy is range.
- A project is a database. `Databases` reports the one project that the session
  runs in, because BigQuery lists no other.

An external table is not a foreign table, because it has no server. Its format and
its files are in `Table.Options`.

## Parity

BigQuery has IAM and no user in SQL. The principals are the administrator, which
owns the dataset, and the reader, a service account with dataViewer on the dataset
and jobUser on the project. The reader gets the administrator's answer to every
query but `CurrentUser`, which reads its own name. No query is refused and none
reads fewer rows. `TestPrivilegeParity` records it.

## The cost

BigQuery bills the bytes a query reads, and each view of INFORMATION_SCHEMA is
billed at least 10 MB. A pass of the fifteen statements bills about 370 MB. The
fixture is small, and the free tier is 1 TB each month. A catalog with thousands
of tables was not built, because a table takes seconds to make.
