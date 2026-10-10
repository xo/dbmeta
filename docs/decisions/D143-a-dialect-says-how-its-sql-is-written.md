# D143. A dialect says how its SQL is written

Status: Decided, amended by D230.

## What Ken decided

usql kept facts about each product's SQL on its per-driver struct,
`drivers.Driver`, so two drivers of one product were able to disagree, and a
product with a model in dbmeta had its knowledge in two places. Ken went
through every field in usql's session on 2026-09-30, after Gemini Pro and
DeepSeek had sorted them, and moved the ones that are the product's grammar
into dbmeta, keyed by dialect. usql keeps what belongs to a driver, a URL or
its own display.

## What moves

`dbmeta.Info` gains:

- `Syntax`, the lexical forms a client must know to split text into
  statements: dollar quoted strings, block comments, slash comments, hash
  comments and names between backticks. usql's lexer flags were the seed.
- `OldPassword`, which says ChangePassword needs the current password.
  SQL Server is the case.
- `Terminator`, what the product does with a semicolon at the end of a
  statement. Oracle refuses it unless the statement ends with `END;`, and
  Trino and Presto refuse it always.
- `Batches`, the statements that open and close a batch, such as CQL's
  BEGIN BATCH and APPLY BATCH.
- `Fold`, what the product does to the case of a name that is not quoted,
  with `Dialect.FoldIdentifier` to apply it. A name between quotes keeps its
  case.

## The fold is measured

A fold that is guessed sends a caller looking for the wrong name, which is
the fault usql found on Oracle: `\d usql_w21_child` found nothing, because
Oracle stores the name as USQL_W21_CHILD. So `scanEveryQuery` selects an
unquoted alias, DbMeta_Fold, on every product it runs against, and fails
when the column name the product returns is not what FoldIdentifier says.
A product reports an alias as it stores a name.

## What stays in usql

The name of the syntax highlighter, which usql maps from the dialect, and
every field that belongs to a driver: opening a connection, the parameters
of a URL, the error types, the value conversions and the rows affected. usql
derives the lower casing of column names for display from the fold.

## What is open

ODBC, csvq, Athena and Cosmos have rules of their own in usql and no model
here, and an Info registers only with a model. Ken decided on 2026-09-30
that they wait for a restructuring of usql, and docs/BACKLOG.md holds it.
