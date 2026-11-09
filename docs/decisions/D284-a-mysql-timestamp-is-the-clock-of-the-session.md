# D284. A MySQL timestamp is the clock of the session

Status: Decided.

## Decision

The dump reads and writes a MySQL TIMESTAMP as the UTC clock of the session, and
the zone of the client does not change it. A MariaDB column that is declared
JSON is a JSON column in the snapshot.

The maintainers chose both on 2026-11-09. usql found them while it tested the dump on dbmeta
v0.39.0.

## The timestamp

The loader sets the session of MySQL to UTC, so the server sends the UTC clock.
The driver `go-sql-driver/mysql` reads that clock as a time in its own location,
which is the local zone when the DSN says `loc=Local`, as dburl writes it. The
loader then moved the time to UTC. In UTC+7 the clock 12:34:56 became 05:34:56,
and each copy moved it 7 hours more. A test did not see it, because CI runs in
UTC.

The driver also moves a `time.Time` that it writes into its own location. So a
date, a datetime or a timestamp that was bound as a `time.Time` changed too, and
a date can change its day in a zone that is behind UTC.

`Style.SessionClock` says that a product has this driver behavior. For a source
with it, the loader reads the clock of a timestamp as UTC, whatever the location
of the value. For a target with it, `Args` binds a date or a time as text, which
the driver sends unchanged. MySQL and MariaDB set it. The statements that
`Insert` writes did not depend on the location of a value once the source is
read this way.

The tests ran against mysql-26.7 and mariadb-13.0 with the client in
Asia/Bangkok, America/New_York and UTC, and with `loc=Local`.

## The JSON column

MariaDB reports a column that is declared JSON as `longtext` and adds a check
named for the column, `json_valid(...)`. The reader dropped the check, because
the type was meant to bring it back, but the type said `longtext`. The copy was
a column that took any text.

The reader now makes a string column a JSON column when it drops the
`json_valid` check of that column. A table that is written for MariaDB gets the
check again, and one that is written for MySQL gets a native JSON column.
