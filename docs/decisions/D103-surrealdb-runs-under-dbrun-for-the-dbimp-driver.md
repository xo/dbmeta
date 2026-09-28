# D103. SurrealDB runs under dbrun, for the dbimp driver

Status: Amended by D119.

Ken agreed on 2026-09-27 to a `dbrun` entry for SurrealDB, which is the next
driver in dbimp. dbmeta has no SurrealDB model, and the entry is there for
that driver's tests, as the Couchbase entry was first (D94).

## The name and the URL

The dialect is `surrealdb`, which is the name dbimp's driver registers and the
dburl Driver of its scheme. dbimp's D47 and D48 settle the URL:
`surrealdb://user:password@host:port/<namespace>/<database>`, with the
namespace and the database as the two segments of the path. dburl registers
the scheme once the driver is tagged. The `dsn` field is the plain `http://`
address of the server, and the `url` field is that URL.

## The releases

`container/surrealdb.go` names 2.7.0 and 3.3.0 as Tested and 3.1.6 and 3.2.4
as Nightly, on the vendor's image. Step 2 of `EVALUATION.md` gives the range:
2.7.0 and 3.3.0 were rebuilt that week, and 1.5.6, 2.6.5 and 3.0.5 were not.
Both major lines are Tested, because dbimp found that their HTTP interfaces
differ. A tag carries a `v`, so the product gained a `tagPrefix`.

## The setup has no shell

The image is the `surreal` binary and nothing else. `Ready` runs
`surreal isready`, which needs no shell. The setup has to send SurrealQL, and
`surreal sql` exits 0 when a statement fails, so a failed setup reads as a
success. `surreal import` exits 1, and it reads a file, which `/dev/stdin`
is. So `Init` runs `surreal import` on `/dev/stdin`, and `dbrun` sends it the
statements through a new field, `Server.InitInput`, with `exec -i`. Import on
3.3.0 wants `OPTION IMPORT;` as the first statement, and 2.7.0 takes it too.

The statements make the namespace and the database `dbmeta` if they are
missing, and make `dbmeta_user` or reset its password, with the role
EDITOR on that database. The data lives in RocksDB under `/tmp/dbmeta`,
because the image's user cannot write `/data`.

## What was measured

On 2.7.0 and 3.3.0, each started fresh, then stopped and started again so
that `Init` ran twice:

- `Init` gives the same result on the second run.
- `dbmeta_user` signs in through `/signin`, creates and reads a record, and is
  refused defining a user.
- A record written before the stop is there after the start.
- Basic auth as `dbmeta_user` works only when the request sends
  `Surreal-Auth-NS` and `Surreal-Auth-DB`, which say where the user is
  defined, beside `Surreal-NS` and `Surreal-DB`, which choose where the query
  runs. With the last two alone it is refused on both releases. dbimp's
  driver depends on this, and dbimp was told.

3.1.6 and 3.2.4 started and served the ordinary user's sign in.

## The ordinary user's URL names its level

dbimp's D51, which Ken decided the same day, adds `?auth=database` to the URL
of `dbmeta_user`, so that its principal reads
`surrealdb://dbmeta_user:<password>@127.0.0.1:<port>/dbmeta/dbmeta?auth=database`.
A database user signs in only with `Surreal-Auth-NS` and `Surreal-Auth-DB`,
and root is refused with 401 when it sends them, so the driver is told the
level rather than trying both. The administrator's URL has no key, because
`auth=root` is the default.
