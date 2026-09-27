# D32. Errors are constants of a string type

Status: Decided.

Use the `xo` house pattern. Errors are untyped constants of a defined string
type, not package level variables:

```go
// Error is an error.
type Error string

// Error satisfies the error interface.
func (err Error) Error() string {
	return string(err)
}

// Error values.
const (
	// ErrNotSupported is the not supported error.
	ErrNotSupported Error = "not supported"
)
```

`dburl` does this at `dburl.go:352`, and so do `tblfmt` and `usql`.

The reason is immutability. A sentinel declared with `errors.New` is a package
level variable, and any importer can assign to it. A constant cannot be
reassigned, so no consumer can change what `dbmeta.ErrNotSupported` means for
every other consumer in the process.

Comparison still works with `errors.Is`, and wrapping still works with `%w`.

This amends the naming guidance in `CLAUDE.md`, which said to declare sentinels
with `errors.New`. The `Err` prefix and the PascalCase name are unchanged. Only
the declaration changes.

The taxonomy needs at least these four, because they are four different
answers and `usql` returns one undifferentiated error for all of them today:
not supported, permission denied, version too old, and no such object.
