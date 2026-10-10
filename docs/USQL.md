# What usql Answers Today, and What dbmeta Changes

`usql` is the reason the object model here follows `psql`. This measures what
`usql` can answer today, per database, and what changes if it reads `dbmeta`
instead of its own readers.

Everything below was measured rather than read. A program built `usql` with the
`most` build tag, constructed every driver's metadata reader, and asked each
one which reader interfaces it satisfies. The counts come from that.

Measured against `usql` at commit `382e1da` on `main`, with 47 drivers built.
`usql` has changed since. `ef8bf17` removed mymysql, adodb, sapase and
ignite, `2cc28e2` made pgx the PostgreSQL driver and moved lib/pq, `8407785`
replaced n1ql with dbimp's Couchbase driver, and `54c12a4` added SurrealDB.
The counts and the driver names below are the measurement at `382e1da`, and
they have not been measured again.

## How usql reads metadata today

`usql` defines 14 leaf reader interfaces in `drivers/metadata/metadata.go`,
one per object kind, aggregated by `ExtendedReader`. The file declares 19
interfaces in total and 16 of those end in `Reader`, because two are composites
and three are `Writer`, `CatalogProvider` and `Result`, so counting with grep
gives a different and less useful number. `BasicReader` embeds 3 of the 14. A driver implements the ones it can. The writer asks for a reader
by type assertion and leaves out the section when the driver does not have one,
so a command degrades rather than failing.

The 14 kinds: Catalogs, Schemas, Tables, Columns, ColumnStats, Indexes,
IndexColumns, Triggers, Constraints, ConstraintColumns, Functions,
FunctionColumns, Sequences and PrivilegeSummaries.

Eleven metacommands register in the Describe family in `metacmd/descs.go` and
all of them dispatch through `Describe`:

	\d \da \df \di \dm \dn \dp \ds \dt \dv \l

Seven readers decide whether a command runs at all: TableReader, ColumnReader,
FunctionReader, IndexReader, SchemaReader, PrivilegeSummaryReader and
CatalogReader. The other seven leaf readers decide how much a command prints
once it does run. SequenceReader, IndexColumnReader, TriggerReader,
ConstraintReader and ConstraintColumnReader add sections inside
`DescribeTableDetails`, FunctionColumnReader inside `DescribeFunctions`, and
ColumnStatReader serves `\ss` alone. `\l` needs Catalogs. `\dn` needs Schemas. `\dt`, `\dv`, `\dm`,
`\ds` and bare `\d` need Tables. `\d NAME` needs Tables and Columns to print
anything, and adds a section per further reader. `\di` needs Indexes. `\df`
and `\da` need Functions. `\dp` needs PrivilegeSummaries. `\ss` needs
ColumnStats and is outside the Describe family. `usql` has no `\z`.

Two earlier versions of this section were wrong in the same way, and both
errors were a right count of the wrong thing. It said eight commands, which
counted readers. Then it said eight gating readers, which counted seven gating
readers plus one that is not. The `usql` session measured both.

## Which databases answer what

Counting needs a unit and a build, and this is the unit: registered names,
which includes aliases, because `Register` puts an alias in the same map as its
own key.

	51 registered names, 21 with a reader, 30 without    built with -tags all
	13 registered names, 12 with a reader, 1 without     the default build

A package is a different count and reconciles with neither. There are 46
packages under `drivers/`, 42 calls to `drivers.Register` with a literal
scheme, which undercounts because Oracle and godror go through
`orshared.Register`, and 15 packages that define a reader of their own. More
schemes answer than that, because cockroachdb, redshift, tidb, vitess and
memsql have no package and register against another driver's reader: the
first two through `drivers/postgres` and the last three through
`drivers/mysql`.

An earlier version of this paragraph put `nzgo` in that list and it is not in
it. Netezza has its own package, `drivers/netezza`, its own driver,
`github.com/IBM/nzgo/v12`, and a reader it configures itself. The scheme and
the package are named differently, which is what made it look like an alias.
The `dburl` registry settles this kind of question now: a scheme that borrows
another scheme's driver has no `GoPackage`, and `nzgo` has one. See D80.

An earlier version of this section said 47, 20 and 27 without saying which
build, so nobody was able to reproduce them. The figures here were measured by the
`usql` session with a program that asserts each registered reader against the
interfaces the dispatch requires, rather than by reading code.

A driver with no reader answers no metadata command: `\dt` on it prints
nothing useful.

| Driver | Commands | Missing |
| --- | --- | --- |
| postgres, pgx | 11/11 | none |
| cockroachdb, redshift | 11/11 | none |
| sqlserver | 11/11 | none, and it reads `information_schema` with sequences and constraints off |
| duckdb | 11/11 | none |
| trino | 11/11 | none |
| mysql, mymysql | 10/11 | `\l`, no `CatalogReader` |
| memsql, tidb, vitess | 10/11 | `\l` |
| snowflake, databend, nzgo | 10/11 | `\l` |
| oracle, godror | 10/11 | `\dp`, no `PrivilegeSummaryReader` |
| sqlite3, moderncsqlite | 9/11 | `\dp` and `\l` |
| clickhouse | 8/11 | `\di`, `\dp` and `\l` |
| impala | 6/11 | `\da`, `\df`, `\di`, `\dp` and `\l` |

The floor is higher than a bare count suggests, and the gaps are two features
rather than a long tail. Every driver that fails fails on the same two
commands. `\l` is missing across the whole MySQL family and Snowflake,
Databend and Netezza, because none implements `CatalogReader`. `\dp` is
missing on Oracle, godror, SQLite, ClickHouse and Impala, because none
implements `PrivilegeSummaryReader`. Nothing is missing `\dt`, `\dn` or `\d`.

At `382e1da`, the 30 with no reader at all included avatica, awsathena,
bigquery, cassandra, chai, cosmos, csvq, databricks, exasol, firebirdsql, flightsql,
h2, hdb, hive, ignite, maxcompute, n1ql, ots, presto, ql, spanner, tds,
vertica, voltdb and ydb.

The seven at full marks get there the same way: they are PostgreSQL, or wire
compatible with it and reusing its reader, or they read `information_schema`.

One shape worth knowing, because D67 turns on it. Impala's reader returns a
value satisfying nothing when its handle is not a `*sql.DB`, so it degrades to
no metadata at all rather than reporting anything. It is the only driver here
that does that, and it is deliberate.

## What dbmeta changes

### It does not raise the command count for PostgreSQL

PostgreSQL already answers 11 of 11. What changes there is the number of object
kinds: 14 against 65. `COMMANDS.md` maps every `psql` metadata command to the
Go value that answers it, and 37 of them have no reader interface in `usql`
today. Tablespaces, types, domains, operators, text search, publications,
extensions, roles, access methods and the rest are all readable from `dbmeta`
and have nowhere to go in `usql` yet.

So for PostgreSQL the gain is not a command that starts working. It is that
`usql` can implement four times as many commands without writing SQL.

It also gains facts `psql` does not print. `Column.PrimaryKey` is the example
the policy was written around: `usql` reads it per column today and `dbmeta`
now returns it in the same row. D47 says when that is allowed, and the short
version is that the fact must come out of one statement.

### Which databases answer everything usql asks

`usql` dispatches eleven metadata commands through `Describe`, and `\d NAME`
adds a section per further reader, of which there are five. That is the unit
below, and an earlier version of this section used a different one.

It said eight commands and four sections, which was a right count of the wrong
thing twice over: eight counted the gating readers rather than the commands,
and four missed one of the five section readers. The eleven and the five are
in the first section of this document and this table now agrees with them. The
`usql` session measured the commands and put the same correction to this
table, which is how it was found.

This is what each `dbmeta` model answers, counted by asking every registered
query whether it supports the newest release of its dialect rather than by
reading code.

| Database | Commands | Sections | What it cannot answer |
| --- | --- | --- | --- |
| PostgreSQL | 11/11 | 5/5 | nothing |
| MariaDB | 11/11 | 5/5 | nothing |
| SQL Server | 11/11 | 5/5 | nothing |
| SAP HANA | 11/11 | 5/5 | nothing |
| CockroachDB | 11/11 | 5/5 | nothing. It shares the postgres model's statements for all of them |
| CrateDB | 11/11 | 3/5 | the sequence and trigger sections: CrateDB has neither |
| QuestDB | 9/11 | 0/5 | `\di`, because a symbol index is listed only one table at a time, and `\dp`, because the open source edition has no privileges. Every section, because QuestDB has no index, constraint, trigger or sequence it can list |
| TiDB | 9/11 | 4/5 | `\df` and `\da`, and the trigger section: TiDB has no stored function, procedure or trigger. `\dp` needs 8.5, where the view of table grants has rows |
| Vitess | 10/11 | 4/5 | `\dp`, because privileges list the accounts of the tablet's MySQL, which no client of vtgate logs in as. The trigger section, because vtgate refuses CREATE TRIGGER. A schema is the keyspace (D135) |
| Databend | 10/11 | 4/5 | `\dp`, because a grant is listed only by show_grants for one role or user at a time, and the trigger section: Databend has no trigger |
| SingleStore | 11/11 | 3/5 | the sequence and trigger sections: SingleStore has no sequence and refuses CREATE TRIGGER |
| Snowflake | 10/11 | 3/5 | measured on 2026-10-08 (D190) and again on 2026-10-09 (D203). `\di`: Snowflake has no index outside a hybrid table, which a trial account refuses. The index column and trigger sections: Snowflake has no trigger. The constraint column section is answered, by SHOW read through the pipe operator |
| Amazon Redshift | 10/11 | 2/5 | `\di`: Redshift has no index. Every section but constraints and constraint columns. A privilege to a role is not in `\dp`, because only an SVV view shows it. Measured on 2026-10-09 (D182, D204) |
| Apache Impala | 9/11 | 0/5 | `\di`: Impala has no index, and `\dp`: authorization is off in the image. Every section: SHOW lists no key, constraint, trigger or sequence |
| MySQL | 11/11 | 4/5 | the sequence section: MySQL has no sequence |
| ClickHouse | 11/11 | 2/5 | the sequence, trigger and constraint column sections |
| Couchbase | 11/11 | 2/5 | the trigger, constraint and constraint column sections. `\d NAME` also needs Columns to print anything, and a collection has no columns, so it prints nothing |
| Oracle | 10/11 | 5/5 | `\l`: Oracle has one database per instance and no list to read |
| Firebird | 10/11 | 5/5 | `\dn`: Firebird has no schemas before 6.0 |
| DuckDB | 10/11 | 3/5 | `\dp`, and the index column and trigger sections |
| GizmoSQL | 10/11 | 3/5 | the same as DuckDB, whose statements it shares. usql's own gizmosql driver is in its bad group and every statement fails with "No session ID in request context" (D118, D187). Not run with usql, and the numbers are DuckDB's |
| SQLite | 10/11 | 4/5 | `\dp`, and the sequence section |
| rqlite | 10/11 | 4/5 | the same as SQLite, whose statements it shares (D148) |
| libSQL | 10/11 | 4/5 | the same as SQLite, whose statements it shares (D160) |
| InfluxDB 3 | 8/11 | 0/5 | `\l`, because a query names its database and no SQL lists them, `\di`, because InfluxDB 3 has no index, and `\dp`, because Core has one token. Every section but triggers: no key, constraint or sequence (D152, D170) |
| Neo4j | 11/11 | 3/5 | the trigger and sequence sections: Neo4j has neither. `\d NAME` also needs Columns to print anything, and a label has no column catalog, so it prints nothing. On 5.26 `\df` and `\da` lose the procedures, and the index column and constraint column sections need 2026.05 (D162) |
| YDB | 8/11 | 0/5 | `\di`, `\df` and `\da`, and every section: the columns, indexes and keys of a table are in its schema, which only a gRPC call per table reads, and YQL has no function list. `\d NAME` also needs Columns to print anything. Every command answers only for an administrator (D161) |
| ArangoDB | 8/11 | 1/5 | `\l`, `\di` and `\dp`: AQL lists no database, index or user. `\dn` lists the database of the connection, which is the schema (D168). A collection's columns are the properties of its schema rule, so `\d NAME` prints nothing for a collection with no rule. Every section but constraints, because a rule is a check with no columns behind it (D163) |
| Apache Druid | 8/11 | 0/5 | `\l`: Druid has one catalog and no list of databases, `\di`: it has no index, and `\dp`: its users and roles are in an HTTP API. Every section: Druid has no key, constraint, trigger or sequence. The version and settings need an administrator (D171) |
| Apache Drill | 9/11 | 0/5 | `\di`: Drill has no index, and `\dp`: it has no privilege catalog. Every section: Drill has no key, constraint, trigger or sequence. `\l` lists the one catalog DRILL. A file table is listed only when the Metastore is on (D178) |
| Elasticsearch | 8/11 | 0/5 | `\dn`: Elasticsearch has no schema, `\di`: its SQL lists no index, and `\dp`: its users and roles are in the security API. Every section: no key, constraint, trigger or sequence. `\d NAME` lists a field with a dot, such as `name.raw`, as a column of its own (D177) |
| OpenSearch | 6/11 | 0/5 | `\dn`: OpenSearch has no schema, `\df` and `\da`: its SQL has no function list, `\di`: it has no index, and `\dp`: its users and roles are in the security plugin. Every section: no key, constraint, trigger or sequence. `\dv` lists no view, because SQL cannot tell an alias from an index. `\d NAME` prints nothing on 2.19.6, because dbimp's driver cannot read DESCRIBE there (D181) |
| Apache Avatica | 11/11 | 5/5 | nothing. The standalone server runs HSQLDB. `\df` lists the routines of the user and no built in function. An ordinary user sees only what it can access (D186) |
| Apache Solr | 6/11 | 0/5 | `\l`: Solr has one catalog and no list of databases, `\di`: it has no index, `\dp`: its users and roles are in security.json, and `\df` and `\da`: Calcite lists no function. Every section: Solr has no key, constraint, trigger or sequence. A column reads nullable, with no key, and the version needs an administrator (D179) |
| InfluxQL | 8/11 | 0/5 | `\df` and `\da`, because no InfluxQL statement lists a function, and `\di`, because InfluxDB has no index. `\dp` on InfluxDB 1 alone, because InfluxDB 2 and 3 have no SHOW GRANTS. Every section: no key, constraint, trigger or sequence (D165) |
| Cassandra | 10/11 | 4/5 | `\l`, and the sequence section |
| ScyllaDB | 10/11 | 4/5 | the same as Cassandra |
| Trino | 8/11 | 0/5 | `\df`, `\da`, `\di` and every section: a query engine has no index, no constraint and no table valued function list |
| Presto | 8/11 | 0/5 | the same as Trino |
| Apache Hive | 9/11 | 2/5 | `\l` and `\di`, and the sequence, index column and trigger sections |
| Exasol | 11/11 | 3/5 | the sequence and trigger sections |
| Spanner | 10/11 | 4/5 | `\l`: no function returns the name of the database. The trigger section: Spanner has no trigger. Measured on Spanner Omni 2026.r4-lts (D216) |
| Vertica | 11/11 | 5/5 | nothing on 25.1. Below 25.1 there is no trigger section |
| SurrealDB | 11/11 | 5/5 | nothing on 3.x, where an INFO statement is a value. 2.7 answers none of them, because a 2.x statement cannot read INFO as a value, and the model reports each as too old (D164) |

Seven answer every command and every section: PostgreSQL, MariaDB, SQL
Server, SAP HANA, CockroachDB, Vertica, and SurrealDB from 3.0. Almost every gap in the table is the
product having no such object rather than the model being unfinished. There
are two exceptions. The Redshift model cannot show a privilege that was granted
to a role, because only an SVV view lists it and that view shows a user only
its own rows, and on Impala `\dp` fails because authorization is off in the
image. Each gap says so with `NotSupported` rather than returning no rows.

The `usql` side of the comparison is not in this table, because it is
measured in `usql` and not here, and a copy of somebody else's measurement is
the thing that goes stale. One figure is worth stating because the `usql`
session measured it and it is the argument for moving MariaDB first: MariaDB
answers 10 of the 11 today and lacks only `CatalogReader`, so it fails only
`\l`, and `dbmeta` closes exactly that gap.

The wider `usql` figure is that 21 of 51 registered names have a reader at
all, so 30 answer no metadata command whatever.

The count is per dialect, and MySQL and MariaDB share one. A query the model
registers for both is counted for both, so a difference between the two
products shows up in `docs/COVERAGE.md` rather than here, at 29 kinds for
MariaDB and 26 for MySQL.

### It offers something to the 30 with nothing

`models/informationschema` answers 12 kinds for any database with a standard
`information_schema`, which is enough for `\dn`, `\dt`, `\d NAME`, `\df` and
`\dp`. A driver registers a profile saying how it differs from the standard,
which is tens of lines rather than a reader.

That is the cheapest coverage in this project, and for that reason it moves
first. It is one adapter rather than one per driver, and it lands on a set of
names rather than on one product.

Which of the 30 have a usable `information_schema` is not measured here and
must not be guessed. Each one needs the D43 treatment: ask two models, then run
the queries against a real server.

## The gap, and what closed it

`usql` read three kinds that `dbmeta` did not answer for any database:
ColumnStats, ConstraintColumns and FunctionColumns. At that time, a migration
lost all three. D46 named them, D47 set the policy for adding them, and all three
now exist.

`dbmeta.ConstraintColumns` is the column level detail of a constraint: which
column, in what position, and for a foreign key which column of which table it
points at. Every model answers it but ClickHouse, Trino, Presto, Couchbase,
QuestDB, Snowflake, Redshift, Impala, InfluxDB 3 and ArangoDB, which have no
such constraint to read, and YDB, which keeps its primary key where no SELECT
reaches.
SQLite answers it.

`dbmeta.RoutineParameters` is `usql`'s FunctionColumns: the name, position,
direction and type of each parameter. PostgreSQL, the MySQL dialect, SQL
Server, Oracle, DuckDB, Firebird, SAP HANA, Couchbase, CockroachDB, Vitess,
SingleStore, InfluxDB 3, Neo4j from 2026.05 and the shared model answer it. SQLite cannot, because a function there is compiled C with no
named parameters, and `COVERAGE.md` says why each of the others cannot.

`dbmeta.ColumnStats` backs `\ss`. PostgreSQL, MariaDB, SQL Server, Oracle, SAP
HANA, Apache Hive, CrateDB, Databend, SingleStore and Impala answer it. MySQL,
SQLite, DuckDB and Trino cannot, and say so rather than returning rows that
are almost all absent. `usql` implements `\ss` through its PostgreSQL reader,
for postgres, pgx, cockroachdb and redshift, and through its DuckDB and Trino
readers. DuckDB, Trino, CockroachDB and Redshift are not covered: `usql` prints
statistics there that `dbmeta` does not give. DuckDB and Trino keep no catalog
of them that the models read, and the CockroachDB and Redshift models do not
answer ColumnStats.

### What is still missing for a lossless migration

Nothing, for the databases `dbmeta` models, except `\ss` on DuckDB, Trino,
CockroachDB and Redshift. A migration of those four loses `\ss`, because none
of their models answers it.

Two `usql` reader kinds have no `dbmeta` equivalent by design.
ConstraintColumns replaces both ConstraintColumns and the column part of
Constraints, and FunctionColumns became RoutineParameters. The names differ and
the facts do not.

## How usql can use dbmeta

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

`dbmeta.Args` holds six filters: `Catalog`, `Schema`, `Parent`, `Name`,
`Types` and `WithSystem`. The example copies three of them from
`metadata.Filter`. `dbmeta.Queryer` is one method, so whatever `usql` already
holds satisfies it.

Two things the writer gets for free. `Query.Support(m)` says whether a database
can answer at all, so `usql` can tell "this database has no such object" from
"there are none", which today it cannot. And `Field.Min` and `Field.Present`
say whether a NULL means the value is null or the server is too old to have the
column, which `usql` has no way to express.

Do not move the writer. `dbmeta` only reads, and `tblfmt` renders. That
division is D5 and it does not change.

The order that loses nothing: move one driver, then the rest. PostgreSQL is the
wrong one to move first because it already works. MariaDB is the right one,
because it goes from 10 commands of 11 to all 11, and the difference is visible
the moment it lands.

The three kinds that blocked this are done. What is left is `usql`'s own work:
a reader per driver, and whatever filtering each command needs so that the
output still matches `psql`.

## The version line, measured

`usql` prints one line on connecting, and `dbmeta` builds the same line in
`VersionSet.Display`. Both were read from the same connection on 26 servers,
which was every release in `container.All()` at the time, plus the two SQLite
drivers and DuckDB. `usql`'s side is its own per driver `Version` function, or
`SELECT version()` where a driver declares none.

| Product | Releases | Result |
| --- | --- | --- |
| PostgreSQL | 9.6 to 18, all ten | identical, including the Debian build suffix |
| DuckDB | 1.5.5 | identical |
| MariaDB | 10.6 to 13.0, all six | `dbmeta` names the product, `usql` prints a bare number |
| MySQL | 8.4, 9.7, 26.7 | the same |
| SQL Server | 2017, 2019, 2022, 2025 | `dbmeta` adds the release year and the cumulative update |
| SQLite | one build, two drivers | the two disagree about the name |

Eleven lines match exactly. Thirteen are `dbmeta` reporting strictly more. Two
are the SQLite naming, which is the only real disagreement.

That table predates every model after `models/sqlserver`, and it compares
what the two print rather than what they run. The statements are compared below, because comparing only the output hid
a case where `usql` has no answer at all.

### The statements, compared

| Product | dbmeta runs | usql runs | |
| --- | --- | --- | --- |
| PostgreSQL | `SHOW server_version` | the same | same |
| SQLite | `SELECT sqlite_version()` | the same | same |
| ArangoDB | `RETURN VERSION()` | the same, in the `Version` of usql's arangodb driver, through dbimp's driver | the same statement, and both print the word ArangoDB before the release. dbmeta's was measured on 3.12.12 on 2026-10-01, and usql's was read from its source |
| InfluxDB 3 | `SELECT version()` | `GET /ping`, through the driver's raw connection, which no SQL statement reaches | different answers. `version()` names the release of DataFusion, such as 51.0.0, which the statements depend on, and only `/ping` names InfluxDB's. Measured on 3.11.5 on 2026-10-01 |
| YDB | `SELECT version()` | `SELECT '<unknown>' AS version`, a literal | different answers. `version()` returns the release, such as 26.3.1.17, and usql prints "YDB <unknown>" for every server. Measured on 26.2.1.14 and 26.3.1.17 on 2026-10-01 |
| Apache Druid | `SELECT version FROM sys.servers WHERE server_type = 'broker' LIMIT 1` | `SELECT version();`, the generic fallback, because usql's druid driver declares no `Version`. Not measured on Druid | the two differ. Only an administrator can read `sys.servers`, so the ordinary user gets HTTP 403 with Insufficient permission to view servers. Measured on 37.0.0 and 38.0.0 on 2026-10-07 (D171) |
| Apache Drill | `SELECT version FROM sys.version` | the same, in the `Version` of usql's drill driver, through dbimp's driver | the same statement. Both print Apache Drill before the release, and every user can read `sys.version`, so there is no refusal. dbmeta's was measured on 1.21.2 and 1.22.0 on 2026-10-08, and usql's was read from its source (D178) |
| Elasticsearch | `SELECT version()`, which dbimp's driver answers from `GET /` | the literal `Elasticsearch`, with no release, because its SQL has no function for it | different answers. usql prints the product and no release for every server. dbmeta prints Elasticsearch and the release, such as 9.5.3, to every user, because the role of the ordinary user holds `cluster:monitor/main`. Measured on 8.19.22, 9.4.6 and 9.5.3 on 2026-10-08 (D177, D191, D192) |
| OpenSearch | `SELECT version()`, which dbimp's driver answers from `GET /`, and on 3.9.0 from the header `X-OpenSearch-Version` | the literal `OpenSearch`, with no release, because `SELECT VERSION()` fails on the server itself | different answers. usql prints the product and no release for every server. dbmeta prints OpenSearch and the release to every user on both releases. The role of the ordinary user holds `cluster:monitor/main`, which 2.19.6 needs and 3.9.0 does not. Measured on 2.19.6 and 3.9.0 on 2026-10-08 (D181, D191, D192) |
| Apache Avatica | `VALUES (DATABASE_VERSION())` | the same statement, in the `Version` of usql's avatica driver, which prefixes `Avatica, HSQLDB ` and prints `Avatica <unknown>` when the statement fails | the same statement, and the same line. Both give the release of HSQLDB and not of Avatica, which no catalog holds. Phoenix refuses the statement, so usql prints unknown there and dbmeta has no model for it. Measured on 1.28.0 and 1.29.0 on 2026-10-08 (D186) |
| Apache Solr | `SELECT version()`, which dbimp's driver answers from `lucene.solr-spec-version` of `GET /solr/admin/info/system` | `Version` returns the word Solr and runs no statement, because no statement of its SQL gives the version | different answers. usql prints the word Solr and no release. dbmeta prints Apache Solr and the release to every user, because security.json lets the role of the ordinary user read that path. Measured on 9.9.0, 9.10.1 and 10.0.0 on 2026-10-08 (D179, D191, D192) |
| InfluxQL | none: no InfluxQL statement names the release for every user on every release, so `Dialect.Version` reports an unknown version, and `ParseVersion` takes the release a caller reads from `GET /ping` | `GET /ping`, through the driver's raw connection, which is the same function as for InfluxDB 3 | the same source, read by usql. dbmeta has no statement to run, because SHOW DIAGNOSTICS names the release only to an administrator on InfluxDB 1, and InfluxDB 2 and 3 do not have it. Measured on 1.13.1, 2.9.1 and 3.11.5 on 2026-10-01 (D165) |
| rqlite | `SELECT sqlite_version()` | none of its own: the driver declares no `Version`, and usql reads dbmeta's | the same statement. `dbmeta` reads the SQLite release the server runs, and no SQL statement names the rqlite release, which only the HTTP API reports. Measured on 9.4.5 and 10.3.6 on 2026-10-01 |
| GizmoSQL | `SELECT version()` | `SELECT version();`, the generic fallback, because usql's gizmosql driver declares no `Version` | same answer, when the driver works. Both read the release of DuckDB that the server runs, and no SQL statement names the GizmoSQL release. The driver fails before it reaches the statement, with "No session ID in request context", so usql prints nothing. dbmeta's was measured on 1.40.0 and 1.41.0 on 2026-10-08, with a session the test opened (D187) |
| libSQL | `SELECT sqlite_version()` | the same, which the driver declares as its `Version` | same. Both read the SQLite release that sqld runs, because only `GET /version` names the sqld release and no SQL statement reaches it. `dbmeta` prints "libSQL, which runs SQLite 3.45.1" and usql prints "libSQL, SQLite 3.45.1". Measured on 0.24.33 on 2026-10-01 |
| Cassandra | `SELECT JSON * FROM system.local WHERE key = 'local'`, the whole row as one text | three columns from `system.local` | different statement, same answer |
| ScyllaDB | the same statement as Cassandra | the same three columns | `usql` names the wrong product. It prints "Cassandra 3.0.8", which is the Cassandra release that ScyllaDB keeps compatible with. `dbmeta` finds ScyllaDB by the `supported_features` column, then runs `SELECT version FROM system.versions WHERE key = 'local'` for the ScyllaDB release, and prints both. A caller that runs the statements itself asks `Dialect.FollowUpQuery` for the second one. See D92. Measured on 2025.1 and 2026.3 on 2026-09-27 |
| MariaDB | `SELECT VERSION()` | no function, so the generic `SELECT version();` | same answer |
| MySQL | `SELECT VERSION()` | no function, so the generic `SELECT version();` | same answer |
| ClickHouse | `SELECT version()` | no function, so the generic `SELECT version();` | same answer |
| DuckDB | `SELECT version()` | `SELECT library_version FROM pragma_version()` | different statement, same answer |
| Trino | `SELECT version()` | `SELECT node_version FROM system.runtime.nodes LIMIT 1` | different statement, same answer |
| Presto | `SELECT node_version FROM system.runtime.nodes WHERE coordinator = true LIMIT 1` | the same, without the coordinator filter | same answer, and on a cluster `usql` can read a worker |
| Firebird | `SELECT rdb$get_context('SYSTEM', 'ENGINE_VERSION') FROM rdb$database` | the same statement | the same answer, and `usql` prefixes the word Firebird |
| Apache Hive | `SELECT version()` | no function, so the generic `SELECT version();` | the same statement, and Hive has the function, so the fallback works |
| SAP HANA | `SELECT VERSION FROM SYS.M_DATABASE` | the same statement, lower cased | the same answer, and `usql` prefixes the words SAP HANA |
| Vertica | `SELECT version()` | the same | same, measured on 7.2.1, 9.1.0, 10.1.1 and 25.1.0 on 2026-09-27 |
| Neo4j | `CALL dbms.components() YIELD name, versions, edition WHERE name = 'Neo4j Kernel' RETURN versions[0] AS version, edition` | the same, with no name for the first column | the same answer, and both print Neo4j, the release and the edition. The ordinary user reads it too. Measured on 5.26.31 and 2026.09.0 on 2026-10-01 |
| Couchbase | `SELECT RAW ds_version()` | the same, through the dbimp driver since `usql` commit 8407785 | the same answer, and both print the word Couchbase before it, measured on 7.6.12 and 8.0.3 on 2026-09-27 |
| Exasol | `SELECT PARAM_VALUE FROM EXA_METADATA WHERE PARAM_NAME = 'databaseProductVersion'` | the same statement, lower cased | the same answer, and `usql` prefixes the word Exasol. A user granted nothing but `CREATE SESSION` reads it on 2025.2.1 and 2026.2.0, measured on 2026-09-27 |
| Spanner | `SELECT CAST(MAX(version) AS STRING) AS version FROM spanner_sys.supported_optimizer_versions` | `SELECT version();`, the generic fallback, because usql's spanner driver declares no `Version` | different answers. Spanner has no `version()`, so the statement of usql fails with "Function not found: VERSION" and usql prints no version. dbmeta reads the highest optimizer version, such as 9, because it is the one number that SQL reads and that moves with the service. Measured on Spanner Omni 2026.r4-lts on 2026-10-10 (D216) |
| CockroachDB | `SELECT pg_catalog.version(), pg_catalog.current_setting('server_version')` | `SELECT version()`, cut at the first bracket | the same release. `dbmeta` also reads the PostgreSQL release that CockroachDB claims, 13.0.0 or 18.0.0, and gates the statements it shares with the postgres model on it (D123). Measured on 24.3.36, 26.2.7 and 26.3.2 on 2026-09-29 |
| CrateDB | `SELECT pg_catalog.version(), pg_catalog.current_setting('server_version')` | `SELECT version()`, cut before the first bracket | the same release. `dbmeta` also reads the PostgreSQL release that CrateDB claims, 14.0 on both releases, and gates the statements it shares with the postgres model on it (D123). `usql`'s cratedb driver has no metadata reader, because CrateDB refuses the pgx reader's queries. Measured on 6.3.7 and 6.4.5 on 2026-09-29 |
| QuestDB | `SELECT build()` | no function, so the generic `SELECT version();` | different answers. `version()` gives PostgreSQL 12.3 on every release, so usql shows "PostgreSQL 12.3 (questdb)", and only `build()` names the QuestDB release. usql reads the version itself until it takes dbmeta's (D126). Measured on 9.4.3 and 10.0.1 on 2026-09-29 |
| TiDB | `SELECT VERSION()` | no function, so the generic `SELECT version();` | the same statement. It answers `8.0.11-TiDB-v8.5.8`, and `dbmeta` reads both the MySQL release TiDB claims and TiDB's own, and gates the statements it shares with the mysql model on the first (D133). Measured on 7.5.8, 8.1.2 and 8.5.8 on 2026-09-29 |
| Vitess | `SELECT VERSION(), @@version_comment` | no function, so the generic `SELECT version();` | different answers. `version()` gives `8.4.6-Vitess`, which names the MySQL release Vitess claims and not Vitess's own. Only `@@version_comment` names it, as `Version: 24.0.3`, and `dbmeta` reads both and gates the statements it shares with the mysql model on the first (D135). Measured on 23.0.6 and 24.0.3 on 2026-09-30 |
| Databend | `SELECT version()` | no function, so the generic `SELECT version();` | the same statement. It answers `Databend Query v1.2.948-nightly-...`, and `dbmeta` reads the release after the v. Measured on 1.2.881 and 1.2.948 on 2026-09-30 |
| SingleStore | `SELECT VERSION(), @@memsql_version` | no function, so the generic `SELECT version();` | different answers. `version()` gives `5.7.32`, the MySQL release SingleStore claims, and only `@@memsql_version` names SingleStore's own, as `9.1.1`. `dbmeta` reads both and gates the statements it shares with the mysql model on the first (D141). Measured on 9.0 and 9.1 on 2026-09-30 |
| Snowflake | `SELECT CURRENT_VERSION()` | no function, so the generic `SELECT version();` | different answers, as far as Snowflake documents. CURRENT_VERSION() answers `10.36.101` and `dbmeta` reads it whole. The generic statement of usql was not run, because Snowflake documents no version(). Measured on a trial account on 2026-10-08 (D190) |
| Amazon Redshift | `SELECT version()` | no function, so the generic `SELECT version();` | the same statement. `dbmeta` reads the Redshift release after the word Redshift in the banner. Measured on Redshift Serverless on 2026-10-08. The statement answers `PostgreSQL 8.0.2 on i686-pc-linux-gnu, ... Redshift 1.0.434008`, and the release is the number after the word Redshift (D182) |
| Apache Impala | `SELECT version()` | no function, so the generic `SELECT version();` | the same statement. It answers `impalad version 4.5.2-RELEASE ...`, and `dbmeta` reads the release after the word version. Measured on 4.4.1 and 4.5.2 on 2026-09-30 |
| SurrealDB | `SELECT version()`, which dbimp's driver answers with the RPC method `version` | the RPC method `version`, through `surrealdb.Version` and the driver's raw connection, which no SurrealQL statement reaches | the same answer. `SELECT version()` gives one row with one column named version, such as `surrealdb-3.3.0`, and `ParseVersion` reads it as SurrealDB 3.3.0, which is what `usql` prints. Both principals read it on every release. Measured on 2.7.0, 3.1.6, 3.2.4 and 3.3.0 on 2026-10-08 (D164, D192) |
| SQL Server | the `@@VERSION` banner and four `SERVERPROPERTY` values | three `SERVERPROPERTY` values | `dbmeta` reads more |
| Oracle | `SELECT banner FROM v$version WHERE ROWNUM = 1` | `SELECT version FROM v$instance` | same answer for an administrator, and `usql` fails for everybody else |

Each row that gives no date of its own, and does not say it was not measured,
was measured on a live server on 2026-09-26.

#### Oracle, where usql reads a view an ordinary user cannot

An earlier version of this section said `usql` declares no `Version` function
for Oracle and falls through to the generic `SELECT version();`. That is
wrong, it was written here first, and it reached `usql`'s migration plan and
a commit message before the `usql` session caught it.

`usql` does declare one. It is at `drivers/oracle/orshared/orshared.go` and
both the `oracle` and `godror` drivers get it, because neither calls
`drivers.Register` itself: they both go through `orshared.Register`. Anything
that greps this tree for `drivers.Register(` misses both, which is the same
indirection that produced a wrong scheme count twice before. Grep for the
field rather than the call.

The statement is `SELECT version FROM v$instance`, and the defect is real but
different. `v$instance` needs a privilege an ordinary user does not have.
Measured on Oracle 26ai, as a user granted nothing but `CREATE SESSION`:

```
SELECT version FROM v$instance                -> ORA-00942: table or view
                                                 "SYS"."V_$INSTANCE" does not exist
SELECT banner FROM v$version WHERE ROWNUM = 1 -> Oracle AI Database 26ai Free ...
```

So `usql` prints a version for an administrator and prints none for anybody
else. `dbmeta` reads `v$version`, which every user can read, and it is the
only `v$` view the Oracle model touches: every other source is an `all_`
view, which is D61's doing.

The predicate is load bearing rather than tidiness. `v$version` returns five
rows on Oracle 11g, one each for the database, PL/SQL, CORE, TNS and NLSRTL,
and became a single row in 18c. Measured on 11.2.0.2, `ROWNUM = 1` returns
the database banner, which is the one that is wanted. A version query written
against `v$version` without a predicate is nondeterministic on an older
server.

The comparison still did its job. It found that Oracle's version handling is
wrong in `usql`, and it found it because the statements were compared rather
than the printed output. It found the wrong reason, which is what the first
paragraph is for.

#### DuckDB and Trino, where the statement differs and the answer does not

DuckDB returns `v1.5.5` from both `version()` and `library_version` in
`pragma_version()`. Trino returns `483` from both `version()` and
`node_version` in `system.runtime.nodes`, byte for byte.

Neither is a divergence worth closing, and the scalar function is the better of
each pair. `pragma_version()` is a table function where `version()` is a plain
call. `system.runtime.nodes` has one row per node, so `usql`'s `LIMIT 1` with
no `ORDER BY` picks an arbitrary one, which on a cluster mid upgrade can be a
worker rather than the coordinator that parses the SQL.

### MySQL and MariaDB, where usql prints no product at all

The `mysql` driver declares no `Version` function, so `usql` falls through to
the generic `SELECT version()` and prints what comes back. On MySQL that is the
whole line:

```
usql     8.4.11
dbmeta   MySQL 8.4.11
```

On MariaDB the suffix carries the product, so `usql` reads
`11.8.9-MariaDB-ubu2404` and is at least identifiable, by accident rather than
by design. `dbmeta` reads the same string and names the product from the same
suffix its queries gate on, so the two cannot drift apart. See D44.

This is the one place where `usql` gains from the move without a new query.

### SQL Server, where usql is thinner than its own specification

```
usql     Microsoft SQL Server 16.0.4295.3, RTM, Developer Edition (64-bit)
dbmeta   Microsoft SQL Server 2022 16.0.4295.3, RTM-CU27, Developer Edition (64-bit)
```

`usql` selects `productversion`, `productlevel` and `edition`. `dbmeta` selects
those, plus `productupdatelevel` for the `CU27`, plus the release year cut from
`@@VERSION`, which is the only place the server states the name the product is
sold under. D38 always specified the longer form.

### SQLite, which is a disagreement rather than a gap

```
usql     SQLite3 3.53.4          with mattn/go-sqlite3
usql     ModernC SQLite 3.53.4   with modernc.org/sqlite
dbmeta   SQLite 3.53.4           with either
```

`usql` names the driver, so one SQLite build reports two different products.
`dbmeta` names the product, because it holds a dialect and never sees the
driver, and under D47 the consumer decides what to show. A consumer that wants
to name its driver prepends that itself.

This is the one line that a move changes, so it is written down rather than
discovered later.

`TestTheDisplayLineNamesTheProduct` in the `test` module pins the shape per
product against a live server, so a model cannot quietly lose its product name
and start printing the bare number `usql` prints today.

## Changing a password

`usql` changes a password in seven drivers and escapes nothing. Each one
concatenates the new password into the statement, so a password holding a quote
or a backslash breaks the statement or sets something other than what was
asked.

`Dialect.ChangePassword` builds the statement instead and returns the text
for `usql` to run. It takes no database, so `dbmeta` still executes only reads.
The escaping needs the server, because whether a backslash escapes inside a
string literal is `sql_mode` on MySQL and MariaDB and
`standard_conforming_strings` on PostgreSQL, and `Dialect.Quoting` reads it.

That makes this the second thing that a move fixes rather than merely relocates,
alongside the version line for MySQL. See D56.

Vertica's is moved, and it fixes the same fault. `usql`'s Vertica driver
builds `ALTER USER ... IDENTIFIED BY '...'` by concatenating the password with
no escaping, so a quote in it broke the statement. `dbmeta` quotes it as a
literal and reads `standard_conforming_strings`, the way it does for
PostgreSQL. D88 has the rest, including a fault in `vertica-sql-go` itself:
it splits a statement at every semicolon, so no SQL function can be created
through it.

Exasol's is new rather than moved, because `usql`'s Exasol driver declares no
`ChangePassword` at all. Exasol takes a password as a quoted identifier rather
than as a string literal, so the only escaping is a doubled double quote and
there is no session state to read. D87 has the rest.

Every product whose driver in `usql` changes a password has a statement here.
Netezza was the one other, and it is out of scope (D132). Each one is tested by setting every
password in `hostilePasswords` on a real server and logging in with it (D127).
Snowflake is tested on a user that the test makes, and not on the account's own login (D193). Redshift is tested on a user that the test makes as well (D204). Its documentation
forbids a quote, a double quote, a backslash, a slash, an at sign and a space in a
password, and the server accepts all of them. Every hostile password sets and logs in. Vitess has no statement, because vtgate refuses ALTER USER.

| Product | Statement | Quoting |
| --- | --- | --- |
| PostgreSQL, CockroachDB, CrateDB | `ALTER USER "<user>" PASSWORD '<password>'`, and `SET (password = ...)` on CrateDB | a string literal, with a backslash doubled when `standard_conforming_strings` is off |
| MySQL, MariaDB, TiDB and SingleStore | `ALTER USER '<user>'@'<host>' IDENTIFIED BY '<password>'` | a string literal, with a backslash doubled unless `sql_mode` has `NO_BACKSLASH_ESCAPES` |
| SQL Server | `ALTER LOGIN [<login>] WITH PASSWORD = N'<password>'`, and `OLD_PASSWORD` when given | a string literal, and `]` doubled in the login |
| Oracle | `ALTER USER "<USER>" IDENTIFIED BY "<password>"`, and `REPLACE` when given | a quoted identifier. A plain name folds to upper case, and a double quote is refused |
| Vertica | `ALTER USER "<user>" IDENTIFIED BY '<password>'`, and `REPLACE` when given | as PostgreSQL |
| Exasol | `ALTER USER "<user>" IDENTIFIED BY "<password>"`, and `REPLACE` when given | a quoted identifier, with a double quote doubled |
| ClickHouse | ``ALTER USER `<user>` IDENTIFIED BY '<password>'`` | a backslash always doubled, in the literal and in the name |
| Cassandra and ScyllaDB | `ALTER ROLE "<role>" WITH PASSWORD = '<password>'` | a string literal with no backslash escape |
| Amazon Redshift | `ALTER USER "<user>" PASSWORD '<password>'` | a string literal, with a backslash always doubled. The server wants 8 characters with an upper case letter, a lower case letter and a digit |
| Snowflake | `ALTER USER "<user>" SET PASSWORD = '<password>'` | a string literal, with a backslash always doubled |
| Databend | `ALTER USER '<user>' IDENTIFIED BY '<password>'` | a string literal for the user and the password, with a backslash always doubled |
