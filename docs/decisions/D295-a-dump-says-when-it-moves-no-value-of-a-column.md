# D295. A dump says when it moves no value of a column

Status: Amends D294.

The maintainers decided that a dump warns when it skips or ignores something. An
Oracle REF column holds the identifier of a row of the source, so the dump moves no
value of it and the copy has NULL (D294). That loses data, so it needs a warning.

The statement that makes the table has a note with the name of each such column. A client
prints the Note of a table statement, so it needs no change. The style of a product
says which columns lose their values, with `Style.LostValues`. Oracle says it for a
column of a REF type.
