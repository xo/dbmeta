# D1. The module centralizes database metadata

Status: Decided.

`dbmeta` is the single home for database metadata queries. `usql` and `dbtpl`
consume it instead of each keeping their own copy.
