# D186. Avatica reads the HSQLDB catalog, and Phoenix has no model

Status: Decided.

## The decision

Ken asked on 2026-10-08 for an Avatica dialect, and chose on the same day that
it covers the standalone Avatica server alone. `models/avatica` answers 24 of
the 56 on 1.28.0 and 1.29.0 through dbimp's avatica driver at v0.14.0, which
dburl v0.46.0 names. The dialect is `avatica` and `dialect.go` holds it. Ken
reviews the mapping below.

Avatica is the wire protocol of Apache Calcite. A client sends the calls of JDBC
to a server over HTTP, and the server runs each one on a database behind it. The
protocol has no catalog of its own. The standalone server of the Avatica
project, the image `docker.io/apache/calcite-avatica-hypersql`, runs HSQLDB
2.4.1 in memory, so the model reads the catalog of HSQLDB. Both releases of the
server hold the same HSQLDB and answered every statement the same way.

HSQLDB has an INFORMATION_SCHEMA of the SQL standard with 64 views, and 27 more
views whose names begin with SYSTEM_, which hold the metadata of JDBC. Every
kind is one statement over those, and no kind is a walk (D146).

## The mapping

| HSQLDB | Kind | Why |
| --- | --- | --- |
| the catalog PUBLIC | database | INFORMATION_SCHEMA_CATALOG_NAME names it. A server holds one database |
| a schema | schema | SYSTEM_SCHEMAS lists every schema the user can see. SCHEMATA lists only the schemas the user owns, so it supplies the owner |
| a table, a global temporary table, a text table | table | SYSTEM_TABLES. The type says where the rows are kept, as `memory table`. REMARKS is the comment |
| a view | view, and a table of the type `view` | VIEWS has the query, the check option and the flags |
| a table of INFORMATION_SCHEMA | table of the type `system table` | listed only with with_system, as in every other model |
| a column | column | COLUMNS, with the key from SYSTEM_PRIMARYKEYS and the comment from SYSTEM_COMMENTS |
| an index | index, and index columns | SYSTEM_INDEXINFO. A key makes one named SYS_IDX_ and a number |
| a constraint | constraint | TABLE_CONSTRAINTS, with the text of a check from CHECK_CONSTRAINTS. A NOT NULL is a check named SYS_CT and a number |
| the columns of a key | constraint columns | KEY_COLUMN_USAGE, and for a foreign key REFERENTIAL_CONSTRAINTS reaches the key at the other end |
| a trigger | trigger | TRIGGERS. The definition is put together from its parts |
| a sequence | sequence | SEQUENCES. An identity column keeps its counter in the column, so it is no sequence |
| a function and a procedure | function | ROUTINES, and kind says which |
| an aggregate function | aggregate | ROUTINES. No column says it, and the text of the definition does |
| a parameter | routine parameter | PARAMETERS |
| CREATE TYPE ... AS | type, of the kind distinct | USER_DEFINED_TYPES. The types of the engine are SYSTEM_TYPEINFO, and with_system adds them |
| a domain | domain | DOMAINS, with its checks from DOMAIN_CONSTRAINTS |
| a collation | collation | COLLATIONS lists 97, which belong to no schema |
| a user and a role | role | AUTHORIZATIONS, with ADMIN from SYSTEM_USERS. The role DBA is the superuser |
| a role granted to a grantee | role grant | ROLE_AUTHORIZATION_DESCRIPTORS |
| a grant | privilege | TABLE_PRIVILEGES, COLUMN_PRIVILEGES, ROUTINE_PRIVILEGES and USAGE_PRIVILEGES, one row for each object |
| a property of the database | setting | SYSTEM_PROPERTIES, which every user can read |
| a comment on a table, a view or a column | comment | SYSTEM_COMMENTS. The server writes one on each of its own views, so with_system adds those |
| CURRENT_SCHEMA and CURRENT_USER | current schema, current user | a SELECT over VALUES |

The catalog is always PUBLIC.

## What a user sees

HSQLDB filters INFORMATION_SCHEMA by what the user can access. The ordinary user
that the entry makes, `dbmeta_user`, can read one table, DBMETA.READABLE. It sees
that table and its columns, the three schemas, and itself among the users and
roles. It sees no routine, no sequence and no constraint. SCHEMATA lists the
schemas a user owns, so the owner of a schema is empty for it. A grantee that
holds the role dbmeta_reader sees the table author, its columns, the role and
its sequence. The sections `avatica/same/user` and `avatica/same/grantee` of
`test/testdata/parity.txt` hold the differences.

## The version

`VALUES (DATABASE_VERSION())` returns the release of HSQLDB, which is 2.4.1 on
both releases, and not the release of Avatica. No catalog and no statement
holds the release of Avatica, which only a call of the protocol reports. usql
runs the same statement and prints `Avatica, HSQLDB 2.4.1`, and the model
displays the same line. So the version is the one of HSQLDB, and every user
can read it. A version gate in this model gates on the release of HSQLDB.

## The terminator

HSQLDB takes a trailing semicolon, so the model keeps the default terminator.
Its syntax has block comments and names in double quotes, and it folds a name
that is not quoted to upper case.

## Phoenix has no model

The Phoenix Query Server speaks the same protocol, in front of Phoenix on
HBase, and dburl names `phoenix` as an alias of the scheme avatica, so both
reach the one dialect. One model cannot serve both, and Ken chose to leave
Phoenix without one. dbmeta measured Phoenix 2.0-5.0 on 2026-10-08 with the same
driver.

- No statement is shared. Phoenix has no INFORMATION_SCHEMA. Its catalog is
  SYSTEM.CATALOG, one table of more than 60 columns that holds the tables, the
  columns, the views and the indexes as rows of different kinds. `SYSTEM."FUNCTION"`
  and `SYSTEM."SEQUENCE"` exist and were empty, and they need quotes.
- A model has one version statement, and none runs on both servers. Phoenix has
  no statement for its version. It refuses `VALUES`, `VERSION()` and
  `CURRENT_USER`. Nothing in the model detects which of the two answered, so a
  gate on a product key, which D44 requires, has no source.
- Phoenix refuses a trailing semicolon, and HSQLDB takes it. The terminator is
  one value for the dialect.
- Its SQL is a subset. It refuses a subquery in WHERE, an ordinal in GROUP BY,
  and an EXISTS in WHERE, which fails with a NullPointerException.
- It has no ordinary user. It checks users only through Kerberos, which the
  image does not configure (D155), so the parity test has nobody to be.

A Phoenix dialect needs four things. First, dburl must give it a scheme and a
dialect of its own, because the alias `phoenix` is a scheme of avatica today and
the constant `Dialect` is the dburl name. Second, its model must read
SYSTEM.CATALOG with the SQL subset of Phoenix. Third, it needs a version source,
which only a call of the protocol gives, so dbimp must expose one, or the caller
passes the release to `ParseVersion` as D183 allows. Fourth, it sets
`TerminatorStripped`. `container/phoenix.go` stays Staged with the cadence
Tested, and its entry keeps the `http://` dsn that dbimp's recorder reads.

## The fixture

HSQLDB takes DDL, and dbimp's avatica driver runs it through `ExecContext`,
which was measured on both releases. So `models/avatica/fixture` is SQL, like the
fixtures of the other relational models, and no HTTP call is needed. It builds
the four core tables of D53 and the view recent, and a table with an identity
column and a generated column, a global temporary table, a sequence, a domain, a
distinct type, a function, a procedure, an aggregate, a trigger, two roles with
grants to them, and the comments. The server keeps its database in memory and
starts empty, so a teardown runs first and its errors are ignored.

It cannot build a text table or a cached table, which need a file, a collation
of its own, because HSQLDB 2.4.1 has no CREATE COLLATION, or a comment on a
routine, a sequence or an index, because COMMENT ON takes a table, a view and a
column alone.

## What the model learned about HSQLDB

- A CASE of two string literals gives a CHAR as wide as the longer one, so
  `'volatile'` comes back as `'volatile '`. Every such CASE is wrapped in
  RTRIM.
- A result with the same constant twice, such as FALSE for two columns, gives
  both columns the label of the last alias. A scan reads by position and does not
  care, and a name check does. The model writes the flags that are constants as
  comparisons that differ, such as `(3 = 4)`.
- A mark that is only compared with a literal has no type, so it is cast.
- NULL needs a cast in a select list, and `CAST(NULL AS VARCHAR(1))` is the one
  the model writes.

## The cost check

D47 asks for a catalog with thousands of objects. The check made 3,000 tables with
a key, an index on every second one, a comment on every third and a foreign key
on every fifth, 500 views, and 300 each of functions, sequences and triggers,
which is 13,000 columns, 5,100 indexes and 6,600 constraints. Every query read it
as the administrator over one connection. The slowest first versions are in the
list because the fix is the finding.

| Query | Rows | First version | Final |
| --- | --- | --- | --- |
| tables | 3,500 | 0.01 s | 0.01 s |
| columns | 13,000 | 1.5 s | 0.07 s |
| indexes | 5,100 | 0.9 s | 0.02 s |
| privileges | 4,100 | 115 s | 0.2 s |
| constraints | 6,600 | 0.02 s | 0.02 s |
| constraint columns | 3,600 | 0.1 s | 0.1 s |
| every other query | 300 or fewer | under 0.02 s | under 0.02 s |

The first versions joined views, and HSQLDB computes a view each time and joins
it with a loop. The comments became a derived table, which HSQLDB indexes. The
primary key flag of an index reads SYSTEM_PRIMARYKEYS and not TABLE_CONSTRAINTS.
Privileges read TABLE_PRIVILEGES and COLUMN_PRIVILEGES once, grouped, where the
first version checked each of 79,000 column grants against the table grants. The
joins that remain are on SYSTEM_PRIMARYKEYS and the derived table of comments,
which HSQLDB indexes. The join of the constraint columns is the slowest of the
final versions, with 0.1 seconds for 3,600 rows.

## The principals

Parity runs two. The user that the entry makes, `dbmeta_user`, and a grantee that
the test makes, `dbmeta_grantee`, which holds the role dbmeta_reader and so
reaches objects through a role and not through its own grant. HSQLDB hides rows
and refuses no statement, so the golden file records fewer rows and no refusal.

## The test and the pin

The test module opens `avatica://` with `github.com/xo/dbimp/avatica`, which is
the package dburl names for the scheme (D154). The pin is dbimp v0.14.0, which
holds the driver. The URL is the dsn of the entry, as D167 asks, and the `http://`
address is its `api`. Before this change the dsn of the standalone entry was the
`http://` address. Phoenix keeps its `http://` dsn.

## The tier

Both releases were Staged with the cadence Tested. They are Tested now, and
`container/avatica.go` calls `add` instead of `staged` (D119, D120). Phoenix
stays Staged.
