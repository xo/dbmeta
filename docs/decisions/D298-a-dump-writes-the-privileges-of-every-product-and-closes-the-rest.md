# D298. A dump writes the privileges of every product, and states what it leaves out

Status: Amends D297, amended by D299.

The maintainers asked on 2026-11-10 to clear the dump items of the backlog. This decision
records what was built and what was decided to stay out. Nothing that stays out is a fault
of the round trip. Each one has a reason, and the reason is here.

## Privileges

- A new kind, `Grants`, answers one row for each privilege on an object or on a column. It
  raises the total from 78 to 79. PostgreSQL, SQL Server, MySQL, MariaDB and Oracle answer
  it. The kind reads only. The statement that gives a privilege is built by the `Grant`
  hook of the dump style.
- A script makes each role or account that a grant names, and the script makes it so that
  nobody can log in with it: `NOLOGIN`, `WITHOUT LOGIN`, `ACCOUNT LOCK` and
  `NO AUTHENTICATION`. A script writes no password and creates no superuser. A person who
  wants a login sets a password after the copy.
- The owner of an object is not written. The copy is owned by the user that runs the script.
- Oracle grants of the system to PUBLIC on an object, such as `INHERIT PRIVILEGES`, are not
  rows, because the server makes them again.

## Left out on purpose

- PostgreSQL writes the privileges on tables, views, sequences, columns, schemas, functions
  and types. It does not write the privileges on the database, or the default privileges.
  The other products write the privileges on tables, views, routines and columns that
  their catalogs hold.
- SQL Server writes every partition on the filegroup `PRIMARY`, because a script makes no
  filegroup.
- MariaDB has the style of MySQL. The two differ only in rules that the model already
  gates by product, so a style of its own copies code and gains nothing.
- An identity column becomes `AUTO_INCREMENT` in MySQL and a serial in PostgreSQL. DuckDB
  has no identity, and SQLite numbers only an integer primary key. An enum is a string
  with its values in another dialect. An unbounded text in a MySQL key is a `varchar(255)`
  and the plan says Lossy. Each of these is a plain table plus a note, which is the rule
  for a script that crosses products.
- A foreign key across schemas is not written for DuckDB, which refuses it. The script has
  a note in place of the statement. `Style.SameSchemaKeys` says it.
- The client decides what to do with `Style.CheckAfterData`, the data check for a boolean
  by convention, and the keys that SingleStore leaves out. `dbmeta` runs no query, so the
  stop and the choice are the work of the client.
- The Oracle object column has no placeholder form, so a copy that binds values cannot move
  one. The statement of the table says so (D295).
- The Oracle copy needs 18c or later because of `NO AUTHENTICATION`. 11g is not a target.
- The round trip does not call a function, fire a trigger or compare the effect of an event.

## Still open

D299 finished the items that this section listed.
