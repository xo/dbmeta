# D238. A statement of MaxCompute needs its semicolon, and the driver writes values as they are

Status: Amends D143 and D237.

## The decision

MaxCompute refuses a statement that does not end with a semicolon. It answers
"Query must end with ';'". Every other product here either takes the semicolon
or refuses it, and `Info.Terminator` had a value for each of those two. It gets a
third, `TerminatorRequired`, for a product that refuses a statement with no
semicolon. A client adds one when the text has none.

The change in this module is one constant and its comment. The statements of
`models/maxcompute` end with a semicolon in the text of the model, so a caller of
`Query.All` does nothing. `usql` keeps a semicolon that a person types, because
`TerminatorKept` is the zero value and the case that works already. The new value
is for the statement that has no semicolon, such as one that a person sends with
`\g`. Teaching `usql` to add it is the work of the usql session, and this decision
does not change `usql`.

## Why the model writes its own literals

The driver of Alibaba, `github.com/aliyun/aliyun-odps-go-sdk/sqldriver`, takes a
parameter and writes its value into the statement with `fmt.Sprintf("%v")`. It
adds no quote. A position parameter replaces every question mark in the text, also
one inside a string, and a name parameter replaces every `@name`. So a string
value reaches the service without a quote, and a quote in a value ends the string
early. The dialect sets `Info.Literal`, as Hive does (D78), and the statements
carry each value as a literal, so the driver binds nothing.

MaxCompute reads two quotes in a row as two strings that it joins, so `'a''b'`
is `ab`. It escapes with a backslash. Measured on 2026-10-29, `'a\'b'` is `a'b`
and `'a\\b'` is `a\b`. The function that writes a literal escapes the backslash
first and the quote second, and it refuses a NUL and a type that it does not know.
A name in backticks that holds a backtick doubles it, and `` `a``b` `` is the
name a`b.
