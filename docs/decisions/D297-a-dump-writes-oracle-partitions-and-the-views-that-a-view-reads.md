# D297. A dump writes Oracle partitions, and orders the views by what they read

Status: Amends D296, amended by D298.

The maintainers asked on 2026-11-10 to finish the open items of the dump. These are the
ones that were finished. `docs/BACKLOG.md` lists what stays open.

## Oracle partitions

- An interval partitioned table is written with `INTERVAL (...)`. The partitions that the
  server made for an interval are not written, because the rows make them again.
  `Partition.Generated` says which they are.
- A table that is split again has its subpartitions written from the template, with
  `SUBPARTITION BY` and `SUBPARTITION TEMPLATE`. The template is a kind of its own,
  `SubpartitionTemplates`, because HIGH_BOUND is a LONG, and no expression can join a
  LONG to anything. The kind raises the total from 77 to 78.
- A table of reference partitioning is written with `PARTITION BY REFERENCE`, and its
  foreign key is in its own statement. The table follows the table that it points at.
  `PartitionedTable` has the new fields `Interval`, `Reference`, `SubStrategy` and
  `SubKeyColumns`.

## Other changes

- A PostgreSQL ordered set aggregate and a hypothetical set aggregate are written. A user
  function that a range type names as its canonical function or its difference function is
  written before the type, and the canonical function has the type as a shell first.
- An Oracle `TIMESTAMP WITH TIME ZONE` keeps its offset when the copy is from Oracle.
  `Style.KeepOffset` says it.
- A MySQL trigger keeps its DEFINER.
- Oracle and MySQL answer `ViewDependencies`, so their views are in order of what each
  one reads. MySQL answers from 8.0.13, and MariaDB keeps no such view.

## Roles and grants of PostgreSQL

- A PostgreSQL dump writes the privileges on tables, views and sequences, and on columns, and
  the roles that they name with the roles that those are members of. A script makes a role
  when it is not there, with its attributes and with no password, and then the memberships.
  An entry that the owner holds for itself is no grant. `Snapshot.Roles` and
  `Snapshot.Grants` hold them, and `Style.Role`, `Style.Membership` and `Style.Grant` write
  them. The other products do not dump grants yet.
