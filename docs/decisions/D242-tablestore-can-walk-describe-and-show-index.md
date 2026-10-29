# D242. Tablestore can walk DESCRIBE and SHOW INDEX

Status: Amends D146 and D241.

## The decision

D241 left Columns, Indexes and IndexColumns of Tablestore unanswered, because
D146 allows a walk for Impala, InfluxQL, Elasticsearch and OpenSearch only. It
asked Ken whether Tablestore gets a walk. Ken decided on 2026-10-29 that it does.

The reason is the one that D146, D159 and D175 give. The SQL feature of Tablestore
has no catalog that a SELECT reads. No INFORMATION_SCHEMA and no system table
exists, so no one statement lists the columns of every table. The walk is allowed
for that reason and for no other. Everything else in D146 holds. A walk is for a
kind that no single statement answers. A kind that one statement answers stays one
statement, so Tables stays on SHOW FULL TABLES.

`models/tablestore` answers 4 of the 65 kinds now: Tables, Columns, Indexes and
IndexColumns.

## What the walks run

Columns runs SHOW FULL TABLES, and then DESCRIBE for each table. Indexes and
IndexColumns run SHOW FULL TABLES, and then SHOW INDEX FROM for each table. Each
statement runs to its end before the next, so a walk holds one connection at a
time. The name of a table goes between backticks with `dbmeta.QuoteIdentifier`,
which doubles a backtick in the name. The model never quotes a name by hand
(D127). The service read the name `a``b` as a`b, which proves that the doubled
backtick arrives as one.

The parameters are schema, parent and name. Schema matches the empty text only,
because a table has no schema. Parent matches the table, and a table that it drops
costs no statement. Name matches the column for Columns, and the index for Indexes
and IndexColumns. Go matches the patterns with `dbmeta.Like`, because neither
statement takes a pattern.

## What the service returned

Every statement ran on the instance `dbmeta` in the region ap-southeast-5, as the
administrator and as the reader, against the four tables of the fixture.

DESCRIBE returns Field, Type, Null, Key, Default and Extra. SHOW COLUMNS FROM
returns the same rows, so the model reads only DESCRIBE. A key column is `NO` and
`PRI`, and every other column is `YES` with an empty Key. The types are
`bigint(20)`, `mediumtext` and `varchar(1024)`. Default and Extra are the empty
text in every row. The service returns no NULL in any of these statements, and it
writes an empty text where MySQL writes a NULL.

SHOW INDEX FROM returns 17 columns. A table with a key of one column returns the
row PRIMARY with Seq_in_index 1. The table region has a key of two columns and
returns two rows for PRIMARY, with Seq_in_index 1 and 2. A secondary index returns
one row for each column that it holds, and the last one is the key of the table,
which Tablestore adds. The index `dbmeta_fx_author_name` on name returns name at 1
and author_id at 2. Both rows say `NO` in Is_defined_column and `GlobalIndex` in
Index_type. The search index on title returns one row, with `KEYWORD` in
Search_type and `SearchIndex` in Index_type. The key has an empty Index_type.
Non_unique is 0 for PRIMARY and 1 for the others. Collation is `A` in every row.
Expression is the text `NULL` in every row. Cardinality, Sub_part, Comment and
Index_comment are 0 or empty.

A statement about a table that is not mapped answers `OTSParameterInvalid Table
'dbmeta.<name>' doesn't exist`, for DESCRIBE and for SHOW INDEX. The name is case
sensitive, so `DBMETA_FX_AUTHOR` does not exist.

The reader gets the same rows as the administrator from all three statements.

## The fields

Nothing is hidden, and every value is one that a statement returned.

| Kind | Field | Value |
| --- | --- | --- |
| Columns | Name, DataType | Field and Type of DESCRIBE, as written |
| Columns | Ordinal | the position in DESCRIBE, from 1 |
| Columns | Nullable | true when Null says YES |
| Columns | PrimaryKey | true when Key says PRI |
| Columns | Default | absent. DESCRIBE writes an empty text, and an empty text is a value that is not there. A text that is not empty passes through |
| Columns | Identity, Generated, Comment, Collation | absent |
| Indexes | Name | Key_name, so PRIMARY for the key |
| Indexes | Type | Index_type as written: GlobalIndex, SearchIndex, and empty for the key |
| Indexes | Unique | true when Non_unique is 0 |
| Indexes | Primary | true for PRIMARY |
| Indexes | Comment and every other field | absent |
| IndexColumns | Name, Ordinal | Column_name and Seq_in_index |
| IndexColumns | Descending | false when Collation says A, true for D, absent for any other text |
| IndexColumns | Expression | absent. The text `NULL` that the service writes is not an expression |
| IndexColumns | Include | false |

Collation is absent, because only SHOW CREATE TABLE names one, for the whole table
(`utf8mb4_bin`), and it is a third statement for each table. Identity is absent,
because Extra was empty in every row, and no table with an automatic key was made
to see what Extra says then. The model reads nothing from Extra and does not guess.

The Search_type of a search index, which is KEYWORD, has no field in `IndexColumn`,
so the model does not return it. Index_type tells a search index from a secondary
index. Ken decided that `IndexColumn` gets no field for it, because no other product
needs one. Ken also decided that Index.Type stays empty for the primary key, as the
service reports it.

The primary key is in two kinds. Columns says that a column is in the key, and
IndexColumns for PRIMARY says in which position.

## A table that is dropped during a walk

A table can be dropped between SHOW FULL TABLES and its DESCRIBE. The statement
then answers the error above. The walk does not read the text of the error. It
runs SHOW FULL TABLES again. If the table is not in the list, the walk skips it
and goes on. If the table is still listed, or the second list fails, the walk
returns the first error and stops, so a service that is down never looks like an
empty instance. The extra statement costs one call, and only when a statement
failed. `TestTablestoreWalkErrors` drops a mapping inside the walk and checks both
paths.

## The cost

A walk of Columns, Indexes or IndexColumns is one statement for the list and one
for each table that the parent pattern keeps. Each statement is one call to the
service. On the instance, from the development machine, a statement took 17 to 30
ms, and the first one on a connection took about 400 ms because it opens the
connection. So four tables cost five statements, which is about 0.15 s, and the
time grows with the number of tables by about 25 ms for each one.

The check that D47 asks for, a catalog with thousands of tables, was not run. The
instance has no such catalog, and one cannot be built without making thousands of
tables with the API. The statement count grows with the number of tables, as it
does for Impala, and the parent filter is the only way to bound it. A consumer
that wants one table passes it as the parent, and then the walk runs two
statements. Nothing was measured above four tables.

## What changes

The conformance file of Tablestore holds the 14 columns of the four tables now,
and the entry in `agreementExcluded` says what the section does not hold. The
parity section for the reader is still empty, which means that the three new kinds
answer the same to both principals. The count in the README, in COVERAGE.md, in
the package comment and in the test of the supported kinds is 4. The question in
BACKLOG.md is gone. dbtpl cannot generate from Tablestore yet. It reads the
columns and the keys now, and the service has no foreign key, so no relation
exists between the tables.

D146 names the products that can walk, and this decision adds Tablestore to
them. D241 said that the model answers one kind, and it answers four now. The
other decisions that say a walk is allowed for four products, such as D222 and
D224, were true when Ken made them about the product they cover, and they stay as
they are.
