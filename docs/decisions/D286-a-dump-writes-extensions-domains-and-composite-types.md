# D286. A dump writes extensions, domains and composite types

Status: Decided.

## Decision

The dump reads the installed extensions, the domains and the composite types of
PostgreSQL, and writes them back for the same product. Ken asked for this on
2026-11-09. A copy of the `hr` schema of the northwind fixture failed without
them, because a column named `hr.email` and `hr.postal_address` and a type
`citext` did not exist.

## What the snapshot holds

- `Snapshot.Extensions`, and the `Domains` and `Composites` of a schema.
- The format version stays 1. The fields are optional.

## Objects that an extension owns

An extension makes its own tables, types and functions again. The extension
`citext` alone owns more than 100 functions. If the dump wrote them, the script
fails on the first one, and it holds the C code of the extension.

`Table`, `Type` and `Function` get an `Extension` field. It is the name in
`pg_depend` with the dependency type `e`, found by one index probe on the oid of
the object, so it costs the same as the other joins and does not grow with the
catalog. The reader leaves out any object where it is set. A domain is left out
when its type is.

## What the script does

- It installs each extension after the schemas and before every type.
- It writes the domains and composite types after the enums and before the
  routines and the tables. Each follows the types that its text names, because a
  domain can be built on a domain.
- It writes the comments of the three kinds after the rows.
- For another product it writes none of them.

## Rejected

- A `Skipped` entry for each kind until it is written. usql asked for that as a
  stop gap. Writing them is cheaper than a warning.
- A version in `CREATE EXTENSION`. The server that runs the script has its own
  version of the files, and a version that it lacks fails the script.

## Not done

- The names of the checks of a domain, which the model joins with a space.
- A check that calls a function. The routines come after the types, so such a
  domain fails. pg_dump breaks the circle with a shell type.
- A column of a domain type is written with the domain for another product, and
  not with its base type.
- Range types, collations, casts, operators and the other kinds that pg_dump
  writes.
