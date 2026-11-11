# D304. A basic dump of seven more products

Status: Decided.

D302 put SAP HANA, Firebird, Exasol, Vertica, Databend, Avatica and Phoenix, and Cassandra and ScyllaDB
in tier 3. Each one with a model and a container that `dbrun` starts is built here. Apache Phoenix has no
model and no dialect, so it waits. ScyllaDB shares the model and the style of Cassandra.

## Firebird

- Firebird has no schema before release 6, so the style gives every table the one database. A name is
  quoted, because Firebird makes the case of a name that is not quoted upper.
- An INSERT takes one row, because Firebird has no list of rows after VALUES.
- A string is written in the character set UTF8. A VARCHAR holds 8191 characters at most in that set,
  so a longer string is a BLOB of the sub type text.
- Before release 4.0 there is no timestamp with a zone, so a timestamp holds the time in UTC and the
  script says so. The driver reads a timestamp as local time, so the test reads the wall clock.
- The model gives a default with the word DEFAULT in front of it, so the column hook takes it off.
- The driver commits and keeps the transaction open, so a table that a connection used cannot be
  dropped while that connection is idle. The test closes idle connections before it drops a table.

## Databend

- A database is the schema. Databend has no key, no unique key, no foreign key and no check, so the
  style refuses each one and the script says so.
- A string literal escapes the backslash. A TIMESTAMP is an instant, so a timestamp with no zone is
  read as UTC and is Lossy.

## Vertica

- The length of a VARCHAR is in bytes, so a string gets four bytes for each character.
- Every integer has eight bytes, so a parsed integer has 64 bits. A float is a FLOAT of eight bytes.
- Vertica builds its own indexes, so the style writes none.

## Avatica

The standalone server stands in front of HSQLDB, so the style is the SQL of HSQLDB. Phoenix speaks the same
protocol and needs its own model, so it has no style.

- HSQLDB lists each NOT NULL as a check constraint. The new field `Style.ImplicitConstraint` makes the reader
  leave it out.
- The reader did not know the kinds `memory table`, `cached table` and `text table`, so a table of HSQLDB was
  not in a snapshot. It does now. It knows `column table` and `row table` of SAP HANA for the same reason.
- The server cannot send a value of the Java types that carry a zone, so a time with a zone is written
  without one, and the script says so.

## Cassandra and ScyllaDB

- A keyspace is the schema, and every name carries it. A table needs a primary key and has no default, no
  NOT NULL and no other constraint. A source with no key cannot be written, so the pairs of QuestDB and
  Databend to Cassandra are skipped.
- An INSERT takes one row. CQL has no list of rows after VALUES.
- The model gives the columns of the key a position and every other column the position -1. The reader
  puts those last, in the order of the catalog. The key has the name of the table, which a copy has no use
  for, so the reader leaves the name out.
- The driver binds a decimal only as an `inf.Dec`, which this module cannot name. The new field
  `Style.NoBind` makes `Loader.Args` return `ErrNoEquivalent` for such a column, and the client writes the rows as
  literals. The test of the placeholder form skips such a table.
- A decimal has no digits to set, so a product that wants digits takes the text of the number.

## Exasol

- Exasol has no integer type of its own, so an integer is a DECIMAL with no digits after the point.
  It has no binary type, no UUID and no array.
- Exasol keeps an empty string as NULL, so a copy changes one into the other. The test treats them as equal
  for Exasol.
- The driver gives the connection a pool of one, so a client that holds a connection must give it back before
  it reads.

## SAP HANA

- An INSERT takes one row. HANA has no list of rows after VALUES.
- The driver gives a DECIMAL as a value of its own type, so the style reads it as text with `SelectColumn`.
- A TINYINT has no sign, so it is an unsigned integer of 8 bits.
- HANA has no timestamp with a zone, so a timestamp holds the time in UTC and the script says so.

## Test

`TestABasicDumpMovesATable` runs all seven against PostgreSQL, DuckDB and one another, in both forms. The pairs
with MySQL, CrateDB, QuestDB, ClickHouse and Spanner were not run again after these products came in. The
run is slow for Exasol and HANA, which need minutes to start.
