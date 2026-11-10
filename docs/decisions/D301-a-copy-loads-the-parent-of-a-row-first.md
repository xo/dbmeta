# D301. A copy loads the parent of a row first

Status: Decided.

DuckDB checks each row of a statement against the rows that come before it, so a table with
a foreign key to itself loads one row in each INSERT (D294). The rows came in the order of
the key. A row with a lower key than the row that it names loaded first and failed, as in
`employees` of northwind, where employee 1 reports to employee 2.

## Decision

- For a product with `Style.RowPerSelfReference`, `Loader.SelectAll` reads a table that has
  a foreign key to itself, on one column, in levels. A row with no reference is level zero, and
  a row that names a row of level n is level n plus one. The read orders by level and then by
  the key. It uses a recursive query, and the recursion ends, because each row names one row.
- A row that no level reaches is part of a cycle, or names itself. It comes last, and the
  load refuses it as the product refuses the key. A cycle of rows has no order that a
  load can use, and the answer for it is to write the reference as NULL and update it after
  the rows, which no product that checks each row accepts for a key that others name.
- A key of more than one column is read in the order of the key, as before.
- The DuckDB fixture has a row with a lower key than the row that it names, and a row at a
  deeper level.
