# D79. A dialect that cannot bind renders its values

Status: Decided.

[dbmeta.Info.Literal] renders a parameter value as a SQL literal. A dialect
that sets it gets its statements with the values in them and no bind
arguments. Apache Hive is the only one, because it is the only product here
whose protocol has no parameter channel at all. See D78.

Ken decided this. The alternative was to leave Hive unsupported, which is
where D78 stood before.

## Why it is safe enough, and what that argument does not cover

The statements are written in this repository. No caller supplies one, and
nothing a caller passes becomes part of the statement's structure. What gets
rendered is a filter value for a parameter this project declared.

The value does come from outside. In `usql` it is a pattern somebody typed
and in `dbtpl` it is a schema name from a configuration, and in both the
person supplying it already has full SQL access through the same session, so
there is no privilege boundary for an injection to cross. That is Ken's
argument and it holds for both consumers.

It does not hold for every consumer. `dbmeta` is a library, and something
that put an untrusted name into a filter and ran it against Hive would have
a boundary to cross. So the escaping is written as though it mattered,
because for somebody it will:

`TestLiteral` checks the break out shapes directly, and
`TestHiveEscapingHoldsOnTheServer` asks a real Hive for tables named
`x' OR '1'='1` and three others, and fails if any of them matches more than
nothing. A unit test can show the string looks right. Only the server shows
what it means.

## The dialect supplies the function rather than setting a flag

The first design was a boolean and a shared escaper. Hive killed it.

[dbmeta.QuoteLiteral] doubles the quote, which is the standard's rule and
right for every other product here. Hive does not accept a doubled quote.
Measured on 4.2.1:

	SELECT 'a''b'  ->  ab
	SELECT 'a\'b'  ->  a'b

The first is read as two literals written next to each other and joined, so
a doubled quote loses the quote and returns a wrong answer rather than an
error. Hive needs C style backslash escapes.

So escaping is per product knowledge and cannot be a flag, which is the same
conclusion D56 reached for `ChangePassword`: the escaping is the product's
and the value cannot be bound. This is the second instance of that rule
rather than a new exception to anything.

## What it refuses

An implementation returns [dbmeta.ErrInvalidParam] rather than guessing. The
Hive one refuses a type it does not know, because every parameter this
project declares is a string or a boolean and an unknown type means a caller
passed something a query did not declare. It refuses a NUL for the reason
`ChangePassword` does: it can end a string early in a layer below this.

## What it is not

It is not a general literal mode and there must not be one. A caller cannot
reach it, a dialect that can bind must leave it nil, and `Query.Build`
returns no argument values when it is set, so a dialect cannot half use it.
