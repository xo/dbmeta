# D299. A dump writes named subpartitions and more options

Status: Amends D298.

The maintainers asked to finish the items that D298 left open. This decision records each
one, and what stays out because the product keeps no record of it.

## Written now

- A new kind, `Subpartitions`, answers one row for each subpartition of an Oracle table. It
  raises the total from 79 to 80. HIGH_VALUE is a LONG, so the rows come apart from the
  partitions, the same way as in D297. A table with a template gets its subpartitions from
  the template. A table with none has each subpartition written inside its partition, with
  the name that a person gave it.
- An Oracle collection of more than 4000 characters is read. The elements go through
  `XMLAGG`, which makes a CLOB, and `DBMS_XMLGEN.CONVERT` turns the entities back into
  text. `LISTAGG` stopped at 4000 bytes.
- A materialized view log with columns that a person named is written with them. The view
  of the dictionary for these columns is for an administrator only, so the columns come
  from the log table, without the columns of the primary key of the master, which Oracle
  refuses in the list. `MaterializedViewLog` has the new field `Columns`.
- A DuckDB struct, union, map or list type is written. The catalog names only its kind, so
  the reader asks `typeof` for each one, and the type is written with its full text.
- A PostgreSQL aggregate is written with its moving aggregate options, the size of its
  state, its serial functions, and its parallel safety. Release 11 adds how a final function
  changes the state, and the text has it from there.

## Not written, because the product keeps no record

- A DuckDB macro keeps no default for a parameter in `duckdb_functions`. The text of a
  macro with `b := 2` reads back as a macro with no default, and DuckDB 2.0.0 has no
  column that holds it. A script cannot write what the catalog does not hold, so a macro
  default is lost in a dump. Only `EXPORT DATABASE` writes it, and that writes files on the
  server.
- MariaDB, SQLite and DuckDB keep no list of the relations that a view reads. DuckDB has
  `duckdb_dependencies`, and it holds nothing for a view. The dump orders their views by the
  names in the text of each, which is the answer for them.
