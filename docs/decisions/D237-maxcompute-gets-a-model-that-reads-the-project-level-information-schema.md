# D237. MaxCompute gets a model that reads the project level INFORMATION_SCHEMA

Status: Amends D194 and D218, amended by D238, D240 and D243.

## The decision

Ken asked on 2026-10-29 for a model for Alibaba Cloud MaxCompute. D218 said that
the model stays unbuilt for MaxCompute and Tablestore, which had no account. Ken
now has an account for MaxCompute, a project named dbmeta in the region
ap-southeast-5, with an administrator and a reader as `dbmeta-maxcompute` and the
files `maxcompute` and `maxcompute-reader` of D117. The model is
`models/maxcompute`. It answers 15 of the 65 kinds: databases, schemas, tables,
columns, views, partitioned tables, partitions, comments, roles, role grants,
privileges, column privileges, functions, the current schema and the current user.
The tier of the service is Verified, and CI never runs it.

## What the driver reads

The driver that dburl names is `github.com/aliyun/aliyun-odps-go-sdk/sqldriver`
v0.4.26, the version that usql uses (D154). It registers as `odps`, and dburl turns
the URL of the credential file into a DSN that starts with `https://`.

The driver runs each statement as an SQL task and reads the result through the
instance tunnel. So only a SELECT returns rows. SHOW, DESC, LIST FUNCTIONS and the
like fail with "it is not select statement", and ExecContext runs them and returns
no row. The D146 walk does not help, because even one statement of a walk is a
statement that the driver cannot read. The model reads the views of the project
level INFORMATION_SCHEMA, which a SELECT reads.

A person must run `install package Information_Schema.systables;` once as the
project owner or as a RAM user with the role super_administrator. The
documentation says that a new project does not have the package since 2024-03-01,
and that the project owner reads it by default and any other user needs a grant of
read on it. The views show a new object after about an hour. D240 says what that
means for the tests.

The tenant level views of `SYSTEM_CATALOG.INFORMATION_SCHEMA` refuse the
administrator with "You have NO privilege 'odps:Select'". They are not read.

A user also needs CreateInstance on the project, because every statement is an
instance. The reader has the role reader, which holds List, Describe and Select,
and the service refuses it "odps:CreateInstance" for any statement at all. A
grant to the reader is a change of the account, so the session that wrote this did
not make it, and it is an open question for Ken. D240 holds it.

## The two levels

A project that is not schema enabled has two levels, a project and its tables, and
`show schemas` says "Invalid database operations on two-tier model". The views
report the catalog `odps` for a table, a role and a function, and put the project
in the column for the schema. So `Databases` answers odps, `Schemas` and
`CurrentSchema` answer the project from `current_project()`, and a table has the
catalog odps and the project as its schema. USERS reports the catalog RAM.

## The version

`SELECT version();` answers one string of nine lines, from the SQL task, such as
`ODPS_BUILD_NAME: V59.group_cn.SQL_RC60_hf9`. Nothing in it is a release number,
and the number after the V is the sprint of the service. The model declares that
statement as its version query and reads the build name, so the display line is
`MaxCompute V59.group_cn.SQL_RC60_hf9` and the main version is 59. No fragment
gates on it, because no difference between two sprints is known, and Ken can
decide to leave the number out. usql declares no version query for its maxcompute
driver, so it runs the generic `SELECT version();`, which is the same statement.
`select current_user();` does not exist, and `select user();` does not either.

## What is not answered

The primary key of a transactional table is in DESC and in no view, and COLUMNS
has no key column. MaxCompute has no foreign key, no index, no sequence, no
trigger and no user defined type. TABLE_PRIVILEGES names users and never roles,
and a grant to a role shows nothing until a user holds the role. External tables
need a RAM role and an OSS bucket, so the model has no query for them. A function
has only a name, an owner and a time in UDFS. docs/COVERAGE.md holds the list and
the second opinion.
