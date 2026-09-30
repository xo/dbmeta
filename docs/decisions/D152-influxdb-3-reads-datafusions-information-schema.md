# D152. InfluxDB 3 reads DataFusion's information_schema

Status: Decided.

## The decision

Ken asked on 2026-10-01 for dialects for Apache Pinot, InfluxDB 3 and Apache
Druid. InfluxDB 3 is built, and `models/influxdb` answers 8 of the 56 under
the dialect `influxdb`, which dburl already names. The other two wait, and
`docs/BACKLOG.md` holds each with the reason. Druid has no dburl scheme, and
Ken chose to wait until he and dbimp settle its name and driver. Pinot keeps
its catalog only in the Controller's REST API, and its driver answers no
metadata statement.

InfluxDB 3 answers SQL with Apache DataFusion, and DataFusion keeps an
information_schema. The model reads schemata, tables, columns, routines,
parameters and df_settings through dbimp's influxdb driver, which is what
usql uses. It answers schemas, the current schema, tables, columns,
functions, aggregates, routine parameters and settings. InfluxQL is the
dialect influxql, and no model reads it.

## The releases

InfluxDB 3 Core 3.9.13 and 3.11.5 are Tested and 3.10.6 is Nightly, the
cadences each recorded while it was Staged (D120). InfluxDB 1 and 2 answer
only InfluxQL, so they stay Staged. dbimp's driver takes only the
`influxdb://` form, so the InfluxDB 3 entry's dsn is its url.

## What the model does

A function is one row for each overload. `routines` has one row for each name
and return type, and `parameters` has one set of rows for each overload,
numbered by `rid`, with the return type as its OUT row. So the model reads a
function from `parameters`, with the id `name(rid)`, which routine parameters
carries too, and reads a function with no parameter rows from `routines`,
with no id. Every function is built into DataFusion, so they are listed only
with the system objects.

The version is the release of DataFusion, which `version()` returns, because
the statements depend on it and no SQL statement names InfluxDB's. usql reads
InfluxDB's release from `GET /ping`.

InfluxDB 3's SQL writes nothing. dbimp's driver takes INSERT followed by line
protocol, so the fixture writes its four measurements that way, and it has no
teardown, because SQL has no DROP.

InfluxDB 3 Core has one kind of token, the administrator's, so it is exempt
from parity.
