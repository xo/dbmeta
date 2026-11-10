# D300. A SQLite time column is copied as the text that it holds

Status: Decided.

SQLite has no type for a date or a time. A column that is declared date, time, datetime or
timestamp holds any text that it was given. The driver `mattn/go-sqlite3` turns that text
into a `time.Time` when dburl asks for `_loc=auto`, and it reads text with no zone as UTC and
then moves the value to the zone of the machine. A dump that wrote the `time.Time` wrote the
wall clock of the machine. The copy stored that text, the next read took it as UTC, and each
copy added the offset again. On a machine in Asia/Jakarta, `1948-12-08` became
`1948-12-08 08:00:00`, and then `1948-12-08 16:00:00`. On a machine in UTC the text still
changed from `1948-12-08` to `1948-12-08 00:00:00`.

## Decision

- The SQLite style reads a column of a time kind as `CAST(column AS TEXT)`.
- A copy to SQLite writes that text as it is.
- A copy to another product reads the text, as it always did for a string, and a text that
  the loader cannot read as a time stops the dump with `ErrBadValue`.

## Tests

`TestDumpFixtureRoundTripsInOtherZones` runs the SQLite round trip with the local zone set to
Asia/Jakarta and to America/New_York, and the test opens SQLite with `_loc=auto`, as dburl
does. The SQLite fixture has a table `stamps` with a date, a date and a time with no zone, a
zone, a number and a date from before the offsets of today. The MySQL fixture has a table
`stamps` with a value in the hour that New York skips, and the round trip with `loc=Local`
in three zones finds no change there.
