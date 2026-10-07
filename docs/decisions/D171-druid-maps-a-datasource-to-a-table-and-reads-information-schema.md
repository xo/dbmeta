# D171. Druid maps a datasource to a table and reads INFORMATION_SCHEMA

Status: Decided.

## The decision

Ken asked on 2026-10-07 for an Apache Druid dialect. `models/druid` answers 7
of the 56 on 37.0.0 and 38.0.0, through dbimp's druid driver at v0.11.0, which
dburl v0.42.0 names. The dialect is `druid`, which dbimp settled in its D154,
and `dialect.go` holds it now. Ken reviews the mapping below.

Druid answers SQL on the Router with Apache Calcite. It has an
INFORMATION_SCHEMA with four tables, SCHEMATA, TABLES, COLUMNS and ROUTINES,
and a sys schema with the segments, the servers, the server properties, the
server segments, the supervisors and the tasks. Every kind is one statement
over one of them. Druid has no DDL, so the product has no index, constraint,
trigger, sequence, type, domain or comment to list.

## The mapping

| Druid | Kind | Why |
| --- | --- | --- |
| a datasource | table | it has columns and rows, and SCHEMATA and TABLES name it. Every datasource is in the schema `druid` |
| a column of a datasource | column | COLUMNS. `__time` is first and the only column that is NOT NULL |
| the schemas `druid`, `lookup` and `view` | schemas | SCHEMATA |
| INFORMATION_SCHEMA and sys | schemas, system | listed only with with_system, as in every other model |
| a table of INFORMATION_SCHEMA or sys | table of the type `system table` | TABLES has the type SYSTEM_TABLE |
| the schema `druid` | current schema | an unqualified name resolves there. No function returns it, so SCHEMATA names it |
| a Druid SQL function | function, or aggregate when IS_AGGREGATOR is YES | ROUTINES, which has one row for each name. All are built in, so with_system lists them |
| a signature in ROUTINES | `Function.ArgTypes` | the text of every overload, one on each line. It is not parsed |
| a runtime property of a service | setting | `sys.server_properties`, with the service as the context |
| a datasource that has no segment yet | not a table | Druid lists a datasource only when it has a segment |

The catalog is always `druid`.

## No kind is a walk

D146 and D159 allow a walk for Impala and InfluxQL alone, and Ken did not
allow one for Druid. Each kind is one statement or it is
unanswered. A kind whose only source scans the data and not a catalog is
unsupported, which D162 decided for Neo4j: the same rule applies here. So the
types that a column uses, read with SELECT DISTINCT over COLUMNS, are not a
type catalog, and the segments of a datasource are not a partition clause.

## The version, and who can read it

Druid has no function that returns the release. `sys.servers.version` holds
the release of each service, and `GET /status` holds it too. Only an
administrator can read either. The permission is STATE, which the role of the
ordinary user does not hold, and the server answers HTTP 403 with
Insufficient permission to view servers. dbimp measured the same on 36.0.0 and
37.0.0, and this model measured it on 37.0.0 and 38.0.0.

So the version query is `SELECT version FROM sys.servers WHERE server_type =
'broker' LIMIT 1`, and it works for an administrator. A user without STATE
gets the refusal of the server as the error, which is a clear answer and not
an unknown version. Ken can choose another rule. The two others that D164 and
D166 used are a probe of the major release, as for SurrealDB, and no
statement at all, as for InfluxQL. Neither is possible here, because no
statement that a lesser principal can run tells one release from another.
`TestDruidVersionRefusedToAnOrdinaryUser` asserts the refusal. Parity runs
with the metadata that the administrator built, so it does not read the
version as a lesser principal.

Settings read `sys.server_properties`, which needs STATE too. The settings
query is refused to the ordinary user and to the reader, and parity records
both refusals in `testdata/parity.txt`.

## The driver, and the pin

The test module pins `github.com/xo/dbimp` at v0.11.0, the first tag that holds
the Druid driver. dburl names `github.com/xo/dbimp/druid` for the scheme
druid, with the alias dr, from v0.42.0, and calls it provisional until the
driver tag, so D154 is met. The driver reads INFORMATION_SCHEMA and sys with
plain SQL and needs no change. It reads the type of each column from the
header rows and decodes by it.

One case needed a change here. A string that holds a JSON array of two or
more strings, such as the property druid.extensions.loadList, is a multi-value
string to the driver, and it arrives as a list. The model scans the value as
text, and writes a list back as compact JSON.

## The fixture

Druid has no DDL, and dbimp's driver reads and never writes (dbimp D163). So
`models/druid/fixture` holds a REPLACE statement for each of the four core
datasources of D53, and the test sends each one to
`POST /druid/v2/sql/task` and waits for the task to end, as the ArangoDB fixture
sends its requests. A task takes from 5 to 12 seconds and the nano quickstart
has two slots, so the steps run one after the other, and a step is skipped when
its datasource already answers. A column that holds a timestamp, other than
`__time`, is stored as a BIGINT, so the book column `published` is a string.
The fixture has no teardown, because Druid has no DROP in SQL.

## The principals

Parity runs two. The ordinary user that the entry makes, who can READ every
datasource, and a reader that the test makes through the security API of the
Coordinator, who can READ the datasource author alone. Druid filters
INFORMATION_SCHEMA by permission, so the reader sees one datasource and its
columns, and the user sees what the administrator sees. Both are refused the
settings.

## What 38.0.0 changed

38.0.0 answers the same as 37.0.0 for all seven kinds and for both principals.
It adds one column, error_message, to `sys.server_properties`, which the
model does not read, so the number of columns that `COLUMNS` lists grows by one.
dbimp had measured 36.0.0 and 37.0.0 only, so this is the first measurement of
the driver on 38.0.0.

## The tier

Both releases were Staged with the cadence Tested. They are Tested now, and
`container/druid.go` calls `add` instead of `staged` (D119, D120). A Druid
container takes 2.7 GB, so a test starts one release at a time.
