# D74. Firebird has no schemas, and none is invented

Status: Decided.

`models/firebird` answers 24 of the 55 against Firebird 3.0, 4.0 and 5.0.
`docs/COVERAGE.md` holds the measurements. Three things had to be decided
rather than discovered, and they are here.

## Schemas report NotSupported, and every object returns an empty one

Firebird 3.0 through 5.0 has no schemas. Every object lives in one namespace
and names are unique across the database. Firebird 6.0 adds SQL schemas and is
out of range.

The alternative was to invent one. SQLite reports `main` and ClickHouse
reports the database name, so there was a shape to copy, and a consumer that
wants to qualify a name has an easier time with something in the field.

It is rejected. `main` is a name SQLite itself uses and a ClickHouse database
is a real namespace, so neither model is inventing anything. Firebird has
nothing to report, and a value put there would be indistinguishable from a
real schema to a caller that cannot see the server. D34 already says an empty
result must never stand in for `NotSupported`, and the converse holds as
firmly: a fabricated row must never stand in for an absent level.

So `Schemas` and `CurrentSchema` report `NotSupported`, every other query
returns an empty schema with a field description saying why, and
`TestFirebirdSchemasAreNotSupported` holds both halves.

The cost is one of the nine reads `dbtpl` makes, which `docs/DBTPL.md`
records. A generator would have to be told there is no schema to qualify by.
That is a true statement about Firebird and the honest thing for it to be told.

## Roles keeps SEC$USERS, although the driver has a fault around it

Firebird splits what PostgreSQL keeps in one place. `SEC$USERS` holds users
and belongs to the server, `RDB$ROLES` holds roles and belongs to the
database, and `Roles` is one statement over both with `can_login` separating
them.

Keeping `SEC$USERS` was a decision because of what it costs. After a
`CREATE USER` on a connection, `nakagami/firebirdsql` answers a later read of
`SEC$USERS` on that same connection with EOF: the server drops the attachment
and the pool's next connection fails its handshake, so every query after it
reports a protocol error. A few statements in between make it reliable, which
is why it looked intermittent until it was pinned down. The parity test
reproduced it four times out of four and recorded between 18 and 22 differing
queries where the truth is three.

Dropping `SEC$USERS` would remove the fault and make the parity record stable
at once, which was measured: the same run then reported three differences
every time.

It is rejected, because the fault cannot reach a consumer. `dbmeta` issues no
user management statement at any time and the one statement it is allowed to
build, `Dialect.ChangePassword`, returns text and runs nothing. Only a caller
that runs `CREATE USER` itself, on the same connection it then reads metadata
on, can trigger it. The parity test is exactly such a caller, so it runs every
user management statement on a connection of its own and closes it, and the
record is stable with `SEC$USERS` in place.

D52 says a query that fails on the driver `usql` ships is a query that does
not work. This one does not fail on that driver. It fails after a statement
`usql` would have to be asked to run, and the reason is written down in
`docs/COVERAGE.md` so that the next person to see EOF from Firebird knows
within a minute what it is.

## Languages stays unsupported, as a stretch

`RDB$FUNCTIONS.RDB$ENGINE_NAME` and the same column on `RDB$PROCEDURES` name
the external engine a routine is written for, so the engines in use are
derivable in one statement.

That is a list of languages in use and not a catalog of languages installed,
and Firebird has no catalog of the second. An engine that is installed and
unused would be missing and a caller could not tell which kind of answer it
had. Rule 14 says to leave an analogue that is a stretch unsupported and to
record the reason, and this is that case. The fact itself is not lost:
`Function.Language` carries it per routine, which is where Firebird records it.

## What the measurement gave back

Firebird's conformance section is identical to PostgreSQL's except for three
lines, and on all three Firebird agrees with the other eight databases while
PostgreSQL is the outlier, because its fixture declares the keys `serial`.
That is the closest any model has come to the reference, and it is worth
saying after two query engines that could answer neither a constraint nor an
index.
