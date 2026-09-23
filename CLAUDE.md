# dbmeta

`dbmeta` holds database metadata queries and models in one Go module. `usql`
and `dbtpl` consume it.

`dbmeta` only reads. It does not render metadata and it does not write it. The
`Reader` and `Writer` pair from `usql` is a `usql` concept and it does not
come here.

Read `PLAN.md` before you change anything. It records the architecture, the
decisions, the known defects, and the questions that nobody has answered yet.
Do not decide an open question on your own. Ask Ken.

## Hard rules

1. The root module depends on the standard library and `github.com/xo/dburl`,
   and on nothing else. Never add a database driver to the root `go.mod`, not
   even for a test. A consumer brings its own driver and picks its own version
   of it, and `dbmeta` must not constrain that. Anything needing a driver goes
   in the `test` module. Do not add any other third party package without
   asking Ken first.
   Do not repeat `dburl`. Never write a scheme list, an alias list, a flavor
   table, or a connection string parser in this module. Read `dburl` instead.
   `URL.Driver` names the family and selects the model package.
   `URL.UnaliasedDriver` names a wire compatible product such as `cockroachdb`.
   `URL.OriginalScheme` holds an alias such as `mariadb`. A flavor arrives on
   one of the last two, never on both, so read both.
2. PostgreSQL is the primary model, and `psql` defines it. When two databases
   describe the same object differently, follow `psql`. This decides the shape
   of an answer. It does not require every database to answer every question.
3. A query that must differ between releases is one query with version
   fragments, held as generated data in the one driver package. There is no
   package per release. A fragment must never change the set of columns a
   query returns: select the column as `NULL AS name` when the server is too
   old to have it.
4. Stay backward compatible within reason. An old database keeps working when
   support for a new one arrives.
5. Configuration is a value that the caller owns. Do not put a configured
   value in a package level variable. One driver borrowed another driver's
   configuration in `usql` and shipped a fault to users.
6. Do not import `usql` or `dbtpl`. The dependency runs the other way.
7. Every function that reads metadata takes `ctx context.Context` as its first
   parameter. There are no exceptions. Call `QueryContext` and its relatives,
   never `Query`, `Exec`, `QueryRow`, or `Prepare`. Never call
   `context.Background()` or `context.TODO()` inside this library.
8. This project is pure Go. No cgo, anywhere, including in the `test` module.
   Use the pure Go driver for every database: `jackc/pgx` or `lib/pq`,
   `go-sql-driver/mysql`, `modernc.org/sqlite`, `microsoft/go-mssqldb`,
   `sijms/go-ora`, `gocql/gocql`. Never `mattn/go-sqlite3` and never `godror`.
9. Never write a `//go:build` constraint on an operating system or an
   architecture, and never branch on either. Testing is `linux/amd64` only.
   The same database version is assumed to answer the same way everywhere.
   A driver may carry its own platform builds, which is the driver's business.
10. Write idiomatic Go. This code is a move of an older package, so a pattern
   being present in the source is not a reason to keep it. See D18 in
   `PLAN.md` for the two patterns that must not carry over.

## Layout

- `/` is the root package `dbmeta`. It holds the driver agnostic API: the
  common types, the reader interfaces, the `Filter` type, and the error values.
  External projects use this package.
- `models/<driver>` holds the code that `dbtpl` generates for one driver. One
  package covers every supported version of that database. For example,
  `models/sqlite3`. Do not edit generated files. Change the SQL and generate
  again.
- `internal/` holds one file per model, such as `internal/postgres.go`, gated
  by the build tags `none`, `base`, `most` and `all`, following `usql`. A
  `gen.go` generates those files and the model table in `README.md`. Say model,
  not driver: `dbmeta` never opens a connection.
- `test/` is a separate module with its own `go.mod`. It holds the integration
  tests, the database drivers, and the `tool` directive pinning `dbtpl`. None
  of that may appear in the root module.

Read `dbtpl/models` before you design anything. It shows what a working
generated metadata model looks like.

## Generating a model

`dbtpl` generates every file under `models/`. `dbmeta` pins `dbtpl` with the
`tool` directive in `go.mod`, so run it as a tool and not from the path:

```bash
go tool dbtpl query <url> -T <Type> -F <Func> --go-pkg <driver> -o models/<driver>
```

`dbtpl query` introspects a live connection. It creates a temporary view from
your statement, reads the column types, and drops the view. There is no offline
mode, so the database must be running before you generate.

Start the database from the container configuration in `usql/contrib`:

```bash
/home/ken/src/go/src/github.com/xo/usql/contrib/podman-run.sh postgres
```

`usql/contrib/config.yaml` holds the connection URL for each container. Phase 2
uses MariaDB, and its directory is named `mysql`, not `mariadb`.

Read `dbtpl/gen.sh` for the full pattern. It runs 60 such commands and shows
how to hold each SQL statement in a shell heredoc.

Two flags prevent the recurring NULL scan fault. Pass `-U` to make a field
nullable when the column allows NULL. Pass `-Z` to force the field type by hand
when introspection reports NOT NULL and the database still returns NULL.

## Go conventions

These apply to hand written code. Generated code follows the `dbtpl`
templates instead. See question 4 in `PLAN.md`.

Wrap every error with `%w`, never `%s` or `%v`:

```go
if err != nil {
    return nil, fmt.Errorf("reading columns for %s: %w", table, err)
}
```

Write error messages in lower case, starting with a gerund. Do not write
"failed to" or "error". Name the object that failed.

Declare sentinel errors as constants of a defined string type, never as
variables. A `var` declared with `errors.New` can be reassigned by any
importer. A constant cannot.

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

Keep the `Err` prefix and the PascalCase name. Compare with `errors.Is`.

Accept interfaces and return concrete structs. Keep an interface to three
methods or fewer. Define an interface where it is consumed, not where it is
implemented.

Do not name a type `Reader`. Everything in this module reads, so the word adds
nothing and repeats the package.

Return an iterator, `iter.Seq2[Table, error]`. Read the result one record at a
time and never materialize it into a slice. Do not write a cursor type. An
error can arrive part way through a result, so a caller must be able to tell
the end of a result from a failure in the middle of one.

Use generics rather than `interface{}` for a container of one type. Compare
errors with `errors.Is` and `errors.As`, never by string.

Put `context.Context` first in every parameter list and name it `ctx`. Never
store it in a struct.

Name a package with one short lower case word. Do not stutter: write
`postgres.Reader`, not `postgres.PostgresReader`.

Name a receiver with one or two lower case letters, and use the same name on
every method of the type. Never write `this` or `self`.

Group struct fields by purpose, with exported fields first and a blank line
between groups.

## Before you commit

Run these:

```bash
gofmt -l . && go vet ./... && go build ./... && go test ./...
```

`gofmt -l .` must print nothing.

That does not cover the `test` module. `./...` does not descend into a
directory that has its own `go.mod`, so run the integration tests separately:

```bash
cd test && go test ./...
```

A test in the root module never opens a database connection. Test fragment
merging, version parsing and model selection there, against golden files. A
test that needs a server belongs in the `test` module.

CI runs on GitHub Actions, on `ubuntu-latest` only. The queries are SQL and the
code is pure Go, so `dbmeta` does not need the runner matrix that the other
`xo` projects use. Keep the workflow small.

CI tests four databases at their latest version: PostgreSQL, MySQL, SQLite3 and
DuckDB. It does not test older versions and it does not test flavors. Those run
on a development machine, and they must run before a release. Do not add a
version matrix to the workflow. See D24 in `PLAN.md`.

Two facts about the runner. The preinstalled PostgreSQL is 16, not the latest
release, so testing the newest PostgreSQL needs a service container. The
preinstalled MySQL server is MySQL, not MariaDB, which is intentional: MariaDB
is the reference product and MySQL is the flavor, so CI covers the flavor for
free. Do not replace it with MariaDB.

## Writing documentation

Write plain English. Use short sentences and the active voice. Use `can`,
`will`, and `must`, and do not use `should`, `may`, or `might`. Do not use
semicolons or em dashes. Put the condition before the command: "If the query
returns no rows, return the empty set."
