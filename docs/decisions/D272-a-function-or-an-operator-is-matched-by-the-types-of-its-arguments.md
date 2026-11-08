# D272. A function or an operator is matched by the types of its arguments

Status: Decided.

psql takes type patterns after the name in `\df` and `\do`. `\df abs int`
lists the functions called abs whose first argument is an integer, and a last
pattern of `-` means that there is no more argument. `\do OPPTRN [LEFT [RIGHT]]`
does the same for the operands. usql asked on 2026-11-08 where this belongs,
because the matching needs the aliases of the database, such as int for
integer. Ken accepted on 2026-11-08 that dbmeta does it.

## Decision

`Args.ArgTypes` is a list of name patterns, one for each argument in order.
Each pattern is matched against the internal name of the type and against the
name that `format_type` gives. A pattern can name a schema, as in
`pg_catalog.int%`. A pattern with no schema matches only a type on the search
path, which is what psql does. The pattern `-` matches a position that has no
argument, so `{"integer", "-"}` means one argument of type integer.

For an operator the first pattern is the left operand and the second is the
right one. A prefix operator has no left operand, so `{"-", "integer"}` finds
it. psql reads one pattern as the right operand, so a caller that copies psql
passes `-` first. A third pattern matches nothing, because an operator has two
operands.

The PostgreSQL model reads the aliases in the way psql does. It reads int,
float and decimal as integer, double precision and numeric, and it reads the
alias of an array type from the name before the brackets, so int2[] is
smallint[]. A pattern with a schema is not mapped. A caller folds the case of a
name that is not quoted before it calls, because the model does not.

The statement does the matching. It compares each pattern with the type at the
same position of `pg_proc.proargtypes` or of the two operand columns of
`pg_operator`, and the pattern list is a parameter, so one statement serves any
number of patterns. CockroachDB shares the statements and the same tests
pass on it. Its int is a 64 bit integer, so a test there names bigint.

`Query.Each` refuses `ArgTypes` with `ErrNotSupported` where the model does not
take it. Every other field that a query does not take is left out by `Each`,
and that is safe because it only widens a result. Leaving out a type filter
returns every function for a caller that asked for the ones that take an
integer, and the caller reads that as the answer. `Query.All` refuses the
unknown parameter as it did before.

Only the Functions, Aggregates and Operators of PostgreSQL and CockroachDB take
it. Every other model refuses it. Oracle has operators and its bindings have no
type that psql patterns can match, so it is not built.

## How it was checked

Each case in the test was run in psql 18 first, with `\set ECHO_HIDDEN on`, to
read the statement and the answer. The tests pass on PostgreSQL 9.6 and 18 with
both drivers, and on CockroachDB 26.3. On a catalog of about 3300 functions
the filter adds under a millisecond, and a call with no type pattern does not
reach it.

## Rejected

Returning the argument types in parts for usql to match. It needs the aliases
and `format_type` in usql, which is what this module holds for every other
statement.

A statement for each count of patterns. The list is a parameter, so one
statement serves them all.
