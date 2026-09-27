# D43. Ask several models before a dialect is declared finished

Status: Decided.

When a new dialect is implemented, consult at least two independent AI models,
such as Gemini, DeepSeek and Astra, about the queries that the first pass could
not answer. Ask each one to sort them into three groups: absent from the
product, present under another name, and derivable from several catalog reads
or one complex statement. Then verify every claim against a running server.

## Why

A first pass finds the objects that the source product names the same way. It
misses the objects that the target product keeps under a different name, and it
misses the ones that no single catalog table holds. MariaDB is the case that
proved it. A first pass answered 19 of the 48 queries. Asking Gemini about the
other 29 found `information_schema.TABLESPACES` for tablespaces, `mysql.servers`
for foreign servers, `mysql.func` for aggregates, and an engine test on
`information_schema.TABLES` for foreign tables, and it named
`information_schema.PERIODS` and `EVENTS` as catalogs the model was not reading
at all.

One model is not enough, because each one has its own gaps. Two models that
agree on an analogue raise the confidence that it is real. Two that disagree
mark the place to check on a server.

## The rule

A model's answer is a lead, never a result. Every analogue it names is run
against the oldest and the newest supported release before it ships. An
analogue that is a stretch rather than a true match is left unsupported:
`ErrNotSupported` is an honest answer, and a column filled with something that
resembles the answer is not. Record the stretches that were rejected in
`COVERAGE.md`, with the reason, so that the next person does not find them
again and reach the other conclusion.
