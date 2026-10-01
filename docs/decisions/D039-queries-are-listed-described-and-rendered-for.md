# D39. Queries are listed, described, and rendered for the client to run

Status: Decided.

The client can ask what queries exist, what each one takes, and what it
returns, then run one itself.

```go
func (m *Meta) Queries() []QueryDef
func (m *Meta) QueryDef(name string) (QueryDef, error)

// Render resolves the query for the version held by m and writes its
// placeholders in the dialect's syntax. It returns SQL ready to execute and
// the arguments in order.
func (m *Meta) Render(name string, args map[string]any) (string, []any, error)

type QueryDef struct {
	Name        string
	Description string
	Params      []Param
	Columns     []Column
}
```

## Placeholders

Write every query once, with named parameters such as `@schema`. `Render`
translates them into what the dialect wants and returns the arguments in the
matching order: `$1` for PostgreSQL, `?` for MySQL, `:1` for Oracle, `@p1` for
SQL Server.

Named parameters are worth the small cost. A query with four parameters is
unreadable in positional form, and D39 exists so a person can read a query and
understand it.

## Columns are declared, and the SQL is not

This is where DeepSeek was wrong, and its advice contradicts D8. It said
fragments can change only the `WHERE`, `ORDER` and `LIMIT` text and never the
select list.

That is the opposite of D8. Fragments exist to change the select list. Padding
a missing column with `NULL AS "name"` is a change to the select list, and it
is the central mechanism.

The rule is that the column SET is fixed and the SQL is not. `QueryDef.Columns`
declares the columns once, and every fragment of every version produces exactly
those columns under exactly those names. Gemini stated this correctly.

A client therefore knows the result shape before it runs anything, and the same
scan code works against every supported version. That declared list is also
what the generator emits scan code from, which is D30's reason for not needing
to introspect.
