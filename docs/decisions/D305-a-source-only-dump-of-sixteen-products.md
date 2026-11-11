# D305. A source only dump of sixteen products

Status: Decided.

D302 put Trino, Presto, Athena, Hive, Impala, Elasticsearch, OpenSearch, Solr, Druid, Drill, InfluxDB,
InfluxQL, MaxCompute, Tablestore, SurrealDB and ArangoDB in tier 4. A dump can read these products and
cannot write them, because a write needs a bulk interface or an ingestion spec that a list of statements cannot
hold. This decision gives each one type functions and a style, so that a client reads the columns and the rows of
a table and writes them to a product that takes them.

## What a source only product has

- Type functions with a `Parse` and a `Render` that finds no equivalent. The parsers for the families of type
  names live in `models/internal/typeparse`: the names of Presto, of Hive, of the JDBC types, of the mappings of
  Elasticsearch, and of Apache Arrow. SurrealDB, InfluxQL, ArangoDB and Tablestore read their own names.
- A style with `Quote`, `Literal` and the way a table is named, and no binary literal. `dump.Support` says that
  such a product is a source and not a target, because a target needs a binary literal.
- A column that PostgreSQL has no type for, such as an object or a list, is read as JSON text. A boolean or a
  number in a column that the catalog calls text is written as its text.

## Differences

- Elasticsearch, OpenSearch, Solr, SurrealDB, ArangoDB and InfluxQL name no schema in a statement, so the style gives
  every table the one database. Trino, Presto, Hive, Impala, Druid, Drill and InfluxDB name it.
- Solr lists the query and the score of a search as columns, and a select with no limit cannot read them. The
  field `Style.NestedColumn` leaves them out.
- Solr reads a backtick as the quote of a name, as do Hive, Impala, Drill, OpenSearch, Tablestore and SurrealDB.
- ArangoDB speaks AQL. The new field `Style.SelectAll` writes `FOR d IN collection RETURN {...}`.
- The driver of InfluxQL gives the name of the series before the columns of each row. The new field
  `Style.LeadingColumns` and `Loader.LeadingColumns` say how many values to drop.
- The reader knew the table kinds `table`, `base table` and a few others. It now knows `external table`,
  `managed table` and `foreign table` of Hive and Impala, and `collection` of ArangoDB, as it learned the kinds of
  HSQLDB and SAP HANA in D304.
- A table that has no column, such as an empty index of OpenSearch, has nothing to move, and `NewLoader` says so.

## What is tested

`TestASourceOnlyDumpReadsATable` builds the fixture of a product, reads every table of it with its rows, writes the
table and the rows into PostgreSQL, and requires the same number of rows. It ran against Trino 484, Presto 0.299,
Hive 4.2.1, Impala 4.5.3, Elasticsearch 9.5.5, OpenSearch 3.9.0, Solr 10.0.0, Druid 38.0.0, Drill 1.22.0,
InfluxDB 3.12.1, InfluxDB 1.13.1 for InfluxQL, SurrealDB 3.3.4 and ArangoDB 3.12.12.

The fixtures of the search engines, the warehouses and the graph and document stores hold a few rows, so the test
checks the number of rows and that the types read, and does not check the value of each cell. The fixture of the
memory connector of Trino and Presto has no rows, so the test makes a table with the types of a basic dump.

Athena, MaxCompute and Tablestore have no container. They have the same functions and the unit tests of their type
names, and the test of each skips until a credential exists. They are on `docs/BACKLOG.md`.
