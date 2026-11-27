# D306. DuckDB reads the index of a key and the columns of an index

Status: Amends D208.

The `usql` maintainers found four faults in the DuckDB describe output. Three of them were in `models/duckdb`.

A primary key did not show in `\d table`. DuckDB builds an index for a primary key and a unique constraint, and `duckdb_indexes` does not list it. The `Indexes` query now adds those two kinds from `duckdb_constraints`. The row takes the name of the constraint and has no definition, and `ConstraintType` is `p` or `u`.

`\d index` listed no columns. D208 and `docs/COVERAGE.md` said `IndexColumns` was not answerable, because `duckdb_indexes.expressions` is `VARCHAR` and `unnest` refuses it. A cast to `VARCHAR[]` reads it as a list and keeps an element whole when it holds a comma or a quote. `IndexColumns` is now answered. An element that starts with a parenthesis is an expression, and any other element is a column. The direction of a key is not published, so `Descending` is absent. This kind brings the count to 23.

`\dn` listed no schema for a database that had tables. DuckDB marks `main` internal in every database, and the filter on `internal` hid it. The filter now keeps `main` in every database except `system` and `temp`.

The fourth fault is not in `dbmeta`. `\dnS` printed `main` three times because `duckdb_schemas` has one row for each attached database and `usql` does not show the catalog. The `Catalog` field tells the rows apart, and rule 13 says `dbmeta` does not drop one to make output look right.
