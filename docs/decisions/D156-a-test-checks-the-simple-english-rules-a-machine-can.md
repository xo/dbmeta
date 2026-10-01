# D156. A test checks the simple English rules that a machine can check

Status: Decided.

## The decision

D89 and D110 say to follow the simple-english skill for every text that a
person reads. Nothing checked that. On 2026-10-01 a sweep changed about 190
British spellings, and the first run of the test then found 338 banned modals,
34 bold markers and about 60 other faults. Ken asked for the rules to be
checked by a machine.

`TestProseIsSimpleEnglish`, in `prose_test.go` in the root module, checks
these rules:

- The modals "should", "would", "may", "might" and "could".
- A semicolon, an em dash, an en dash, or two hyphens between spaces.
- A contraction.
- Bold in Markdown.
- The present perfect with "has been" or "have been".
- A fixed list of British spellings.
- The filler words "simply", "seamless", "robust", "powerful",
  "comprehensive", "leverage", "crucial", "in order to", "it is worth noting",
  "note that" and "please".
- The abbreviations "e.g.", "i.e." and "etc.".

The test reads every Markdown file outside a fenced code block, every Go
comment, and the first argument of `errors.New`, `fmt.Errorf` and the
messages of a failing or skipped test. It also reads the line comments of the
YAML files, the Containerfiles, the batch files, the INI file and the git
files.

It does not read text in backticks or in double quotes, because that text is
code, a value, or somebody else's words. It masks that text a paragraph at a
time, so a code span or a quotation that breaks across two lines is still
masked. It does not read the skill itself, under `.claude` and `.agents`,
because the skill comes from elsewhere (D89).

There is no other exception and no marker that turns a rule off. If the test
reports a sentence, rewrite the sentence.

## What a person still checks

Sentence length, a verb in "-ing" after a comma, the active voice, and one word
for one meaning stay with the reader. A regular expression cannot find where a
sentence ends or which word is a verb. Gemini and DeepSeek were asked which of
the rules a machine can check, and both drew the line in the same place. A
present perfect without "been", such as "has shipped", is also left to the
reader, because a regular expression for it also matches an adjective.

## Why there is no marker

Both models suggested a marker that turns a rule off for one line, and DeepSeek
suggested that the test fail when a marker is stale. No sentence in the
repository needed one. A quotation goes in double quotes and a value goes in
backticks, and every other sentence can be written without the banned word.
If a sentence ever cannot, that is a new decision.
