# D125. Each wire compatible product has a dialect of its own

Status: Amends D19, D80, D99 and D117.

## The decision

Ken approved dburl's D37 on 2026-09-29, and dburl v0.36.0 carries it. It
removes `Scheme.Override`, which gave a product that speaks another product's
protocol the other product's dialect. So `memsql://`, `tidb://`, `vitess://`
and `redshift://` now report the dialects `memsql`, `tidb`, `vitess` and
`redshift`, and not `mysql` or `postgres`. It also removes
`URL.UnaliasedDriver` and `URL.GoDriver`. `URL.SchemeName` names the scheme,
and `URL.Driver` is always the name that `sql.Open` takes.

dbmeta follows it. The root package has the dialect constants `MemSQL`,
`TiDB`, `Vitess` and `Redshift`, beside `CockroachDB` and `CrateDB`, which
D123 added for the same reason. It also has `QuestDB` and `GizmoSQL`, which
dburl added as schemes in v0.36.0.

## What it changes

A model for one of these products is a package of its own, which shares the
statements of the model it imitates where they answer, as `models/cockroachdb`
shares the postgres model's (D123). No such model exists yet for SingleStore,
TiDB, Vitess or Redshift, so none of them is answered:

- TiDB and Vitess were planned as flavors that `models/mysql` was to detect by
  version key, as it detects MariaDB (D44). They get models of their own
  instead, and their container entries name their dialects and stay Staged.
- Redshift is a hosted service. It was Verified, because the postgres model
  read it (D117). Its dialect is its own now and no model reads it, so it is
  Staged.
- SingleStore has no entry, as D117 and D118 decided.

MariaDB is not affected. `mariadb` is an alias of the `mysql` scheme and not
a product with a protocol of its own, so it keeps the `mysql` dialect, and
`models/mysql` detects it.

## What it amends

D19 and D80 describe `URL.UnaliasedDriver` as where a wire compatible product
shows, and D99 describes a flavor that arrives there. Nothing arrives there
now, because the field is gone and the product has its own dialect. D117 made
Redshift Verified through the postgres model, and it is Staged now.

## Why

Override tied together two facts: which Go driver opens a URL, and which
catalog to read. The first belongs to the driver and the second to the
product. A product with a dialect of its own selects its model the way every
other product does, and the model decides what it shares. D123 had already
made that choice for CockroachDB and CrateDB after measuring that their
catalogs differ from PostgreSQL's.
