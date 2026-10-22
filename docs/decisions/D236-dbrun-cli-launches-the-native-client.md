# D236. dbrun cli launches the native client

Status: Decided.

## The decision

Ken asked for a command that does for the client of a database what
`dbrun usql <name>` does for `usql`. `dbrun cli <name>` runs the native command
line client of the product, such as `psql`, `mysql`, `mariadb` or `sqlcmd`,
connected to the server that `dbrun` started.

The command follows `usql` where it can. It starts nothing and needs the server
to be running. It applies no owner rule, so it connects to a server that
another owner started. A bare product resolves to the newest release and says
which one first. It runs the client from the `PATH` of the host, attached to
the terminal, and it returns the failure of the client as a failure of the
command.

It differs from `usql` in four ways:

1. It names one server. A selector that expands to more than one is an error.
2. `--as` picks a principal by role or by name, as `dsn --json` prints them. The
   default is the administrator (D102).
3. Arguments after `--` go to the client, and flag parsing never reads them.
4. `--print` prints the command line with the password masked as `xxxxx` and the
   names of the environment variables without their values, and runs nothing.
   A test and a document use it.

If the client is not on the `PATH`, the error names the command and the product
and offers `dbrun usql <name>`.

## Where the knowledge lives

The `container` package stays data. Each product has a `client` field, which is
a `container.Client`. It holds the command, or the reason why the product has
none, and a function from a `container.Connection` to the arguments and the
environment variables. The `Connection` is read from the URL of the principal,
so the host, the port, the user, the password and the database come from what
the entry already declares. The package runs nothing and imports no client.

One field covers the products that share a protocol. TiDB, Vitess and
SingleStore use `mysql`. CrateDB and QuestDB use `psql`. CockroachDB uses
`cockroach sql`, because it is the client of that product. The URL of ClickHouse
names the HTTP port, so its client reads the first published port from the
`Connection`.

`TestEveryServerNamesAClientOrSaysWhy` fails when a product has neither a
command nor a reason, in the way `TestEveryProductIsEvaluated` fails.

The embedded databases are not in `container` (D42), so `dbrun` knows them:
`sqlite3` and `moderncsqlite` open their file with `sqlite3`, and `duckdb` opens
its file with `duckdb`. chai and csvq have no shell. A machine takes the client
of its product.

## The password

The password goes in an environment variable of the client where the client
reads one. It never goes in an argument then, because a process list shows
arguments to every user of the machine. A test checks every product that has a
variable.

Nine clients take the password only as an argument: `sqlplus`, `cqlsh`,
`mongosh`, `hdbsql`, `arangosh`, `rqlite`, `cbq`, `ksql` and `exaplus`. `cli`
prints a note about it before it runs one, and the two `argvPassword` lists in
the tests name them, so a new one cannot arrive without a decision. Ken chose to keep the argument and the note for
`cqlsh` and `sqlplus`, and to write no temporary file.

## What it does not do

The only hosted services reached are the ones whose client reads the password
from the environment, through the environment (Ken, 2026-10-22): Redshift and
Neon with `psql`, and PlanetScale with `mysql`. Each sets TLS to required where
the connection string names no mode. `hosted.Service` has a `Client` field like
the container products, and every other hosted service says why it has none
(Athena, BigQuery, Cosmos, Databricks, DynamoDB, MaxCompute, Snowflake,
Spanner and Tablestore). A test fails when a service has neither.

`cli` parses the secret connection string in memory. The host, port, user and
database are arguments, and the password is only in `PGPASSWORD` or
`MYSQL_PWD`. `ParseConnection` returns an error that names no part of the URL,
because the error of `net/url` holds the whole string. A test checks that the
arguments, the `--print` output and every error leave out the password.
`--as` is not supported for a hosted service, because `dbrun` resolves one
credential for each and no `<name>-reader` one. Only the parity tests read those.

It does not run the client inside the container when the host has none. Ken
chose host only for now.

## What was measured

`psql`, `mysql` and `mariadb` are installed on the development machine.
`dbrun cli postgres-18 -- -tAc 'select 1, current_user'` ran `psql` against the
shared server and printed the answer, and a failing statement made the command
exit 1. The forms of the other clients come from the options each documents and
were not run. A test checks their shape and that they hide the password.

## What was rejected

A table of clients in `test/cmd/dbrun`. It is a second list of products
that drifts from `container.All`, which is the reason `container` exists.

A flag to run a client in the container. It was out of scope, and the images do
not all hold one.
