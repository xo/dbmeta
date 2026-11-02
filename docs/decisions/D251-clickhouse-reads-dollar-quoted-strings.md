# D251. ClickHouse reads dollar quoted strings

Status: Amends D143 and D250.

usql asked whether ClickHouse has heredoc strings, because transit found them
in its grammar and a semicolon inside one ends a statement in the splitter.

## What was measured

ClickHouse 26.9 accepted `select $x$a;b$x$` and `select $$a;b$$`, and each
returned the string `a;b`. It also accepted a tag that is a number, as in
`select $1$a$1$`. So it reads both the tagged form and the form with no tag.

## Decision

`Syntax.DollarQuotes` is true for ClickHouse. `TestClickHouseLexicalFormsAreMeasured`
holds both forms to a server.
