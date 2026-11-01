# D250. ClickHouse reads backticks and three kinds of comment

Status: Amends D143.

usql asked whether the `Syntax` of ClickHouse was complete, because its
splitter ends a statement at a semicolon inside a name between backticks or
inside a comment. The model held only block comments, which was usql's old
lexer list and never a measurement.

## What was measured

Each form was sent to ClickHouse 26.9 with a semicolon inside it, and the
server accepted the statement:

- A name between backticks.
- A comment between `/*` and `*/`.
- A comment from `//` to the end of the line.
- A comment from `# ` or `#!` to the end of the line.

A hash with no space or exclamation mark after it is a syntax error, and so is
a hash at the end of a line. The `HashComments` flag is therefore true and is
narrower on the server than it is in a client that reads every hash as a
comment. The wider client reading is harmless, because the text it hides is
a syntax error on the server anyway.

## Decision

`Syntax` of ClickHouse is `BlockComments`, `SlashComments`, `HashComments` and
`Backticks`. `TestClickHouseLexicalFormsAreMeasured` in the `test` module holds
it to a server.
