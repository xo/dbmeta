# D87. Exasol is a model, read from the EXA_ALL views

Status: Decided.

D85 decided how Exasol is tested and D86 decided how `dbrun` runs a machine.
This is what was decided building the rest, on 2026-09-27, when Ken asked for
the whole dialect. `models/exasol` answers 25 of the 55 on 2025.2.1 and
2026.2.0, identically. `docs/COVERAGE.md` is the record of what it answers
and why, and this holds only what had to be decided rather than measured.

## The two releases, and their tiers

The nano container, 2026.2.0, is Tested. It is the one Exasol that CI can
start, and it starts in under a minute, so it is a job on every push like
every other product's newest release. The tag pins the build, nano.5, because
each nano build replaces the one before it rather than being a release of its
own.

The Community Edition machine, 2025.2.1, is Verified, as D85 said. It
publishes the database on host port 51437 and its screen on 8110, beside the
SQL Server machines.

## The vendor's password on both

Both keep `sys` and `exasol`. The appliance cannot be handed a password, which
D85 recorded. The nano image reads its first password from a file mounted into
it, and a `container.Server` carries arguments and environment rather than
files, so it keeps its default too rather than growing a field for one
product. Certificate validation is off on both, because neither certificate is
signed by anything a host trusts.

## Readiness from the host

The nano image has no shell and no client, so nothing can be run inside it to
ask whether it is up. A container with no readiness command is now asked the
way D86 asks a machine: by connecting with the driver and running the version
query. `Server.ReadyArgs` returns nil for it.

The Exasol driver logs every failed connection to standard error as well as
returning it, which printed a line per attempt while `dbrun` waited. `dbrun`
gives it a logger that discards, and the error still arrives through the
return value.

## An empty string is NULL

Exasol reads `''` as NULL. A filter tests for NULL, and a plain string field
is scanned through a helper that reads NULL as empty. That is not the COALESCE
`docs/NULLS.md` forbids: a field typed `sql.Null` keeps its NULL, and a plain
string field is one the object model says is never absent. Three fields where
the empty string means something, identity, generated and a foreign key's
catalog, are restored from the row, so that they read present and empty
rather than absent.

## Two queries an ordinary user is refused

`RoleGrants` reads `EXA_DBA_ROLE_PRIVS` and `UserMappings` reads the
connection views, and both need `SELECT ANY DICTIONARY`. The alternative for
role grants was the views that filter themselves, and they list only the
grants the current user holds, which hides every other member's grants from an
administrator as well. That is withholding a fact, which hard rule 13 forbids,
so the refusal is recorded instead, the way D61 recorded MariaDB's.

## The analogues

A virtual schema is a foreign server, its adapter script the wrapper and its
virtual tables the foreign tables. A connection granted to a principal is a
user mapping. A set UDF that returns one value is an aggregate. An index is
named by its object id, because nothing else names it, and its columns are
read from `REMARKS` by matching the table's real columns rather than by
splitting the list.

Three were rejected. Parsing a routine's parameters out of its text was
rejected, because a type such as `DECIMAL(18, 0)` puts a comma inside one.
Computing column statistics from the data was rejected, because the scan grows
with the data. A consumer group as a role setting was rejected, because it
limits resources rather than setting anything.

## Changing a password

`Dialect.ChangePassword` builds `ALTER USER "name" IDENTIFIED BY "new"`, with
`REPLACE "old"` when the caller gives the current password. A user changing its
own password without `ALTER USER` has to give it, and the statement without it
is refused. Both were measured on both releases.

The password is a quoted identifier rather than a string literal, so the only
escaping is a doubled double quote. A single quote, a backslash and a semicolon
are ordinary characters inside it, and no session state changes that, so the
model reads no `Quoting`. The user is quoted too and matched exactly, so the
name to pass is the one the catalog records, which is upper case for a user
created without quotes.

The test logs in with every hostile password but one. The password containing
a semicolon is left out, because an Exasol DSN separates its pairs with a
semicolon and escapes one with a backslash, and that is the driver's quirk
rather than this model's. Ken decided that. The statement itself handles it.
