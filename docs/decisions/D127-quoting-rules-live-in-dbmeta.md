# D127. Quoting rules live in dbmeta

Status: Amends D56.

## The decision

Ken decided on 2026-09-29 that every rule for quoting a value into a
statement, and every helper that carries such a rule, lives in dbmeta. A rule
that two products share goes in the root package, as `QuoteLiteral` and
`QuoteIdentifier` do. A rule that one product has goes beside its model, as
each model's `changePassword` does. usql takes the statement from
`Dialect.ChangePassword` and holds no quoting of its own.

The same holds for a helper that is not about quoting. `NullAsEmpty`, which
reads NULL as the empty string for a product that cannot store one, was a copy
in the Exasol model and another in the Oracle model. It is in the root package
now, and both models use it.

## Why it came up

The usql session wrote a quoting fix for the nine drivers in usql that
changed a password with no escaping at all. Ken rejected it there, because
usql is meant to shed per product knowledge and take it from dbmeta or dbimp,
and the usql session passed its rules here. D56 had already moved the
statement for seven products, and these are the gaps the rules found:

- ClickHouse. A backslash escapes the next character inside backticks, as it
  does inside a string literal. `QuoteIdentifier` doubles only the closing
  character, so a user name that ended in a backslash escaped its own closing
  backtick. The ClickHouse model doubles both now.
- Oracle. The model built no statement. It builds `ALTER USER "<user>"
  IDENTIFIED BY "<password>"` now, with `REPLACE "<old>"` when the caller
  gives the current password. A plain name folds to upper case, as Oracle
  stores it, and a name given between double quotes is kept as written, which
  Ken decided for usql. A quoted identifier has no escape for a double quote,
  so a password or a name that holds one is refused with
  `ErrInvalidPassword`.

The rest already matched: the refusal of a NUL, PostgreSQL, Vertica,
Cassandra and SQL Server. Netezza and SAP ASE have no model. usql removed its
SAP ASE driver, and Netezza is the next dialect to build.

## What was measured

Each statement was run on a real server on 2026-09-29, and then a new
connection logged in with the password it set, for every password in
`hostilePasswords`. ClickHouse 26.9, also for a user named with a backtick and
a trailing backslash. Cassandra 5.0 and ScyllaDB 2026.3. Oracle 21c and 26ai,
for a plain name and a name in lower case, and for a user changing its own
password with the current one.

Two facts came out of it. Cassandra 5 refuses to change a role's password
within five seconds of the last change, counted from when the role was made.
And an Oracle login names a user the way a statement does, so a name in lower
case is between double quotes in the connection string too.
