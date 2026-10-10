# D190. Snowflake ran against a trial account

Status: Amends D144, amended by D193, D203 and D213.

## The decision

The Snowflake model ran for the first time on 2026-10-08, against a trial
account, release 10.36.101. A person provisioned the account, the database
DBMETA, the role DBMETA_ROLE and the warehouse DBMETA_WH, and a connection
string that dbrun resolves (D117). D144 said to finish the model when such a
string existed, and this finishes it except for parity.

The role owns its database and uses the warehouse. It holds no grant on the
account. Snowflake stays exempt from parity, with a new reason (see below).

## What failed

One statement failed. The sequences statement read `s.increment`, and
INCREMENT is a keyword in Snowflake, so the server answered with a syntax
error at that word. The statement now writes `s."INCREMENT"`.

The fixture, the version query and every other statement ran as written. The
version query answers `10.36.101`, and `parseVersion` reads it whole.

## What changed

1. The sequences statement quotes INCREMENT.
2. The privileges statement named every object a table. It now joins
   information_schema.tables, so a grant on a view reads view and a grant on a
   transient table reads transient table.
3. `TestSnowflakeFixtureObjects` reads the fixture back through the typed API.
   It checks the type of a table and a view, the comment, the identity kind,
   the default, the collation, the count of each kind of key, the sequence, the
   function and the procedure, and the type of a grant on a view.

## What the server answered

- SNOWFLAKE.ACCOUNT_USAGE is refused to this role, so the choice to read only
  INFORMATION_SCHEMA holds.
- Every key reads IS_DEFERRABLE of NO and INITIALLY_DEFERRED of YES. The
  statement passes both through, so deferred is true for a key that cannot be
  deferred. The model does not change the answer.
- APPLICABLE_ROLES lists the grant of PUBLIC to the user twice, and the role
  grants query passes the duplicate through.
- Snowflake names a primary key that has no name SYS_CONSTRAINT_ and a UUID.

## Why parity is not measured

Rule 16 asks for a lesser principal. The role holds OWNERSHIP of its database
and USAGE and OPERATE on the warehouse, and nothing on the account. The server
refused CREATE ROLE and CREATE USER with "Insufficient privileges to operate
on account". The role can make a database role in DBMETA, but a database role
is not a login and cannot make a second connection. Using the PUBLIC role or
another role of the user was ruled out.

So `parityExempt` holds a reason that says the test cannot make a principal,
and not that Snowflake has none. Snowflake does have more than one kind. Its
INFORMATION_SCHEMA shows each role only the objects that the role holds a
privilege on, so a role with no grant is expected to read fewer rows. That was
not measured. If Ken grants CREATE USER and CREATE ROLE on the account to a
role the tests use, parity gets a target with an owner, a grantee and a
stranger, as Redshift has (D182).

## What stays open

- Parity, for the reason above. The item stays in docs/BACKLOG.md.
- No conformance target exists for Snowflake, so none ran.
- The ALTER USER statement of D56 was not run, because the only login is the
  account's own user, and the statement changes its password.
- The model does not read the columns of a key, the table that a foreign key
  points at, tags, masking policies, stages, streams, tasks, pipes or dynamic
  tables.
- Each test pass ran once, and the warehouse stopped by itself after 60
  seconds. The fixture schema is dropped when a test ends, and no user or role
  was made.
