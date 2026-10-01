# D131. CrateDB answers no text search kind

Status: Decided.

Ken decided on 2026-09-29 that the CrateDB model leaves the four text search
kinds unanswered.

CrateDB's `information_schema.routines` lists analyzers, tokenizers and token
filters, which do the work of a text search configuration, a parser and a
dictionary. An analyzer has no schema, and 45 built-in analyzers record no
tokenizer, so `TextSearchConfig.Parser` has no source for them. Answering
needs a schema of `''` and a parser that can be absent. Hard
rule 14 leaves an analogue that is a stretch unsupported, and this is one.
docs/COVERAGE.md has the measurement.
