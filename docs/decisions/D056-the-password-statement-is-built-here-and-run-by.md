# D56. The password statement is built here and run by the caller

Status: Amends D5, amended by D127.

D55 audited `usql` for database specific SQL and found `ChangePassword`, and
refused to move it because `dbmeta` reads. That refusal is reversed. The
knowledge moves here and the authority does not.

[Dialect.ChangePassword] returns statement text. It takes a
[PasswordChange] and a [Quoting], and it takes no database, so there is no way
for it to run anything. Everything `dbmeta` executes is still a read, and a
consumer still hands it a read only connection.

## What changed the answer

Two things, and the second is the one that matters.

The first is that D55 weighed the wrong risk. It treated leaving the statement
in `usql` as the safe option. `usql` concatenates the password into the
statement with no escaping at all, in seven drivers, so the status quo was not
safe anywhere. The choice was never between safety here and safety there.

The second is that the escaping rule needs the server, and `dbmeta` is the
thing that reads the server. Whether a backslash escapes inside a string
literal is session state: `sql_mode` on MySQL and MariaDB, and
`standard_conforming_strings` on PostgreSQL. A consumer that writes this itself
has to learn to read those, per product. This module already reads settings.

Both reviews recommended against the move and both were answered by that
second point, which neither raised. Gemini's objection was the read only
invariant, and returning text keeps it. DeepSeek's was that a static escaper
cannot be correct without session state, which is an argument about where the
state is read rather than about whether the statement belongs here.

## Why it cannot be a parameter

It cannot be bound. PostgreSQL will not prepare the statement at all:

```
postgres=# PREPARE t AS ALTER USER postgres PASSWORD $1;
ERROR:  syntax error at or near "ALTER"
```

and SQL Server rejects `@p` in `ALTER LOGIN`. The password goes into the text,
so the escaping has to be right.

## What wrong escaping does, demonstrated

Quote doubling alone is not enough. Sent to a real MariaDB with the password
`x\`:

```
CREATE USER t3@'%' IDENTIFIED BY 'x\';
SELECT 'statement completed' AS result;
```

The backslash escaped the closing quote, the literal swallowed the semicolon
and the line after it, and the second statement never ran. That is not a
mangled password. That is the password consuming the rest of the statement.

[Quoting] carries the state, and a product that needs it and does not have it
returns [ErrQuotingUnknown] rather than guessing. SQL Server needs none,
because T-SQL has no such setting and a backslash is an ordinary character,
verified on 2022 where `LEN('a\b')` is 3.

## The shape

`Quoting` is read with [Dialect.Quoting], which runs a query and is a read like
any other here, or with [Dialect.QuotingQuery] and [Dialect.ParseQuoting] for a
caller that runs its own statements. That pair is the same shape as
[Dialect.VersionQuery] and [Dialect.ParseVersion], deliberately.

A password and a user name are quoted by different rules, because they are
different things: PostgreSQL takes a role as an identifier and a password as a
literal, SQL Server takes a login in brackets, and MySQL takes an account as
two literals joined by an at sign. [QuoteLiteral] and [QuoteIdentifier] are
exported, because the rule is the thing worth reviewing once and a model
outside this repository needs the same one.

MySQL is the one product here where this is an addition rather than a move.
`usql`'s mysql driver declares no `ChangePassword` at all, so it cannot change
a password today, and the product whose escaping is hardest is the one that
had none.

## What the caller still owns

The returned text contains the password in clear, because the server requires
that. A caller must keep it out of its logs, and `dbmeta` cannot do that for
it. This is the first code here that handles a secret, and it is worth saying
so rather than leaving it implied.

## How it is tested

A unit test does not catch the fault this exists for. So each case sets
a real password on a real server and then opens a new connection with it,
across seven passwords chosen to break a naive escaper.

Removing the backslash rule proves both failure modes. One password raises a
syntax error, and one sets the wrong password successfully and is caught only
by the login failing afterwards. The second is why the test connects rather
than comparing text.
