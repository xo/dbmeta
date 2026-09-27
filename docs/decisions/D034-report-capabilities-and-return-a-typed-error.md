# D34. Report capabilities, and return a typed error when asked anyway

Status: Decided.

A caller can ask what a database supports before querying it. If it asks for
something unsupported regardless, it gets `ErrNotSupported` and not an empty
result.

Both external reviews raised this independently and both stressed the same
rule: an empty result means the database has no such object. It never means
that `dbmeta` cannot ask. Returning an empty slice for an unsupported object is
the failure mode to design against, because a caller cannot tell it from a real
answer.

The same mechanism answers a second question. D8 pads a missing column with
`NULL AS name` when the server is too old to have it, which makes "this version
has no such field" look exactly like "this value is null". The capability
report is where that is resolved, by recording which fields are valid at the
detected version. These are one mechanism, not two.
