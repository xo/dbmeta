# D37. A version is a list of numbers with a name, and there can be several

Status: Decided.

The current `Version` type in this repository has `Major`, `Minor` and `Patch`.
That is wrong and it must be replaced. Oracle reports five components and SQL
Server reports four.

The real shapes, taken from the version queries `usql` runs today:

| Database | Reported | Note |
| --- | --- | --- |
| Trino | `443` | one component |
| Presto | `0.287` | two |
| PostgreSQL | `16.2` | two, plus an integer form |
| SQLite3 | `3.45.1` | three |
| MariaDB | `11.4.2-MariaDB` | three, with a suffix |
| DuckDB | `v1.1.3` | three, with a leading letter |
| ClickHouse | `24.3.1.2672` | four |
| SQL Server | `16.0.4295.3` | four, in one of five columns |
| Oracle | `19.3.0.0.0` | five |
| Cassandra | `4.1.3`, `3.4.6`, `5` | three independent versions |
| YDB | `<unknown>` | none |
| Snowflake, BigQuery, Athena | none | serverless |

Two types, because a reported version and a minimum version are not the same
thing. A minimum is numbers only. A reported version also carries the text it
came from, a suffix, and whether it is known at all.

```go
// Version is one version.
type Version struct {
	Raw     string
	Parts   []uint32
	Suffix  string
	Unknown bool
}

// VersionSet is every version one server reports, keyed by name, with the
// empty name for the main one.
type VersionSet struct {
	Versions map[string]Version
	Display  string
}
```

Compare by padding the shorter list with zeros, so `16.2` equals `16.2.0` and
equals `16.2.0.0.0`. Compare left to right. Never compare version strings.

Ignore the suffix when comparing. `11.4.2-MariaDB` and `11.4.2` compare equal,
and D14 already says the flavor is a separate axis. The suffix is recorded, not
ranked.

Cassandra needs the set rather than one version, because its release version,
its CQL version and its protocol version move independently. A fragment names
which one it gates on. When a reported set lacks the name a minimum asks for,
the gate fails rather than passing by accident.
