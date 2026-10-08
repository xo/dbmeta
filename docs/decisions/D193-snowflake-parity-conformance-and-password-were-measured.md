# D193. Snowflake parity, conformance and the password statement were measured

Status: Amends D190, amended by D203.

## The decision

D190 left four things open for Snowflake: parity, a conformance target, the
`ALTER USER` statement of D56, and a second look at two answers that the model
passes through. Ken granted CREATE USER and CREATE ROLE on the account to
DBMETA_ROLE on 2026-10-08, and all four ran the same day against release
10.36.101. Snowflake leaves `parityExempt`.

## Principals

A Snowflake principal is a user that holds roles, and a role holds the grants.
The test makes a role and a user of the same name for each principal, and drops
both when the test ends, on failure too. A made user logs in with a key pair.
The test generates the key in memory and sends only the public half to the
server, so no secret is in a file, in a statement or in the golden file. The
connection string of the principal is the administrator's string with the
user, the role and the key replaced. gosnowflake parses and writes it, and the
test never prints it.

There are three principals:

1. The grantee holds USAGE on the database and the schema and SELECT on the
   table AUTHOR.
2. The visitor holds USAGE on the database and nothing in it.
3. The stranger holds no grant at all.

There is no owner, as Redshift has. Handing the schema and its objects to
another role puts the fixture out of reach of the administrator role if a run
stops part way.

A made role has no grant on the warehouse DBMETA_WH, and DBMETA_ROLE cannot
give one, because it holds USAGE without the grant option. Snowflake grants
USAGE on the warehouse SYSTEM$STREAMLIT_NOTEBOOK_WH to PUBLIC, which every
role has, so the made users run on that one. It is the same size and also
stops after 60 seconds. The server refused that warehouse as the
DEFAULT_WAREHOUSE of a user (error 092850), so the connection string names it.

## What differs per principal

The golden file records these. Every line is the server hiding what the role
holds no grant on, and none is a fault in a statement.

- The grantee reads fewer rows from columns, constraints, functions,
  privileges, sequences, tables and views, because it can use one table and
  the schema. It reads the schema itself and the comments of AUTHOR.
- The visitor reads the INFORMATION_SCHEMA of the database and the fixture
  schema is not in it, so schemas, comments, columns, constraints, tables,
  views, functions and sequences are all fewer.
- The stranger has no database to be in. The session opens without a current
  database, and every statement that reads INFORMATION_SCHEMA is refused with
  "This session does not have a current database". It cannot name the database
  either, because it holds no USAGE on it. Only the statements that read no
  catalog answer, and `current_user` answers with its own name.
- `current_user` and `role_grants` differ in value for every principal, as they
  must.

INFORMATION_SCHEMA in Snowflake filters by the role, the way Oracle's USER_
views filter by the user. A consumer that connects with a lesser role gets a
partial answer and no error, which is what `usql` wants and what `dbtpl` must
know.

## Conformance

The target reads the core schema and the golden file has a Snowflake section.
It disagrees with PostgreSQL on three facts, and all three are the product:

- No column reads as a primary key and there are no constraint lines.
  INFORMATION_SCHEMA has no KEY_COLUMN_USAGE, and SHOW PRIMARY KEYS is not a
  SELECT, so the columns of a key are not reachable in one statement.
- An AUTOINCREMENT column has no default in the catalog, so `author_id` reads
  no default where PostgreSQL's serial column reads one.
- A view column reads as not nullable where it comes from a NOT NULL table
  column, so the view `recent` has no nullable column where PostgreSQL
  reports all of them nullable.

So Snowflake is in `agreementExcluded` with those reasons. The agreement count
of the relational databases stays at 23.

## The password statement

`ALTER USER "<user>" SET PASSWORD = '<password>'` ran against a user that the
test made, with every password in `hostilePasswords`, and the test logged in
with each one. All seven passed. The statement escapes the backslash and the
quote, and Snowflake reads them back as written.

Snowflake has rules of its own for the password. The account refused a
password of 11 characters with "MIN_LENGTH" and took one of 21, so the test
puts a 13 character prefix before each hostile password and the hostile part
stays at the end. The test user has `TYPE = LEGACY_SERVICE`, which is
the kind that a password alone can log in, and `MUST_CHANGE_PASSWORD` of false.
A user of the default type was not tried. No character was refused.

A user cannot run the statement on itself. The same user, logged in, was
refused with "must have MODIFY granted on USER", with the name and without it.
So `ChangePassword` works for an administrator that can modify the user, and
there is no form of it for a user changing its own password. This was measured
once and is not a test, because the refusal belongs to the server's privilege
model and not to the statement.

## The two answers passed through

Both were measured again and both are the same.

- Every key reads IS_DEFERRABLE of NO, INITIALLY_DEFERRED of YES, ENFORCED of
  NO and RELY of NO. The constraints query reports deferrable false and
  deferred true. Snowflake records the key and checks nothing, so deferred
  here means only that the server says so. The statement does not change, by
  rule 13. A consumer that reads deferred must read ENFORCED first, which the
  model does not return.
- APPLICABLE_ROLES lists the grant of PUBLIC to the user twice. ENABLED_ROLES
  lists it once. The role grants query passes the duplicate through, by rule
  13.

## What changed

1. `test/snowflake_users_test.go` makes the principals and runs the password
   statement.
2. `parityTargets` has a Snowflake target, and Snowflake is out of
   `parityExempt`.
3. `conformTargets` has a Snowflake target, and `agreementExcluded` has its
   reason.
4. The golden files have the Snowflake sections.

## What stays open

- The columns of a key, the referenced table of a foreign key, tags, masking
  policies, stages, streams, tasks, pipes and dynamic tables are not read.
- The model returns no ENFORCED column for a constraint.
- The `primary_key` field of a column is the literal false, which D190 left
  as it is. A column that Snowflake does not report must be NULL under
  `docs/NULLS.md`, and the field is not nullable. Ken decides.

## Cost

The run used 13 dbrun commands: one to read the version, four to ask
statements, seven for single tests and one for the whole suite. Most of the
time was the warehouse starting after its 60 second suspend. Every made user
and role was dropped and SHOW USERS and SHOW ROLES showed only the two users
and the roles that were there before. The fixture schema was gone too.
