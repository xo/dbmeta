# D110. Every xo repository is set up for coding agents the same way

Status: Amends D50 and D89.

Ken decided on 2026-09-27 that every repository in the `xo` namespace is set
up for coding agents the same way. dbmeta, dbimp and cql had the same setup,
the others differed, and one repository had its skills as links that a
Windows checkout breaks. This decision is the standard. dbmeta follows it, and
the other repositories adopt it in their own logs.

## The files every repository has

In the root:

- `README.md`, for a person who finds the project.
- `AGENTS.md`, which holds the rules for a coding agent. Codex and the other
  agents read this file.
- `CLAUDE.md`, which holds one line, `@AGENTS.md`. Claude Code reads
  `CLAUDE.md`, and the line imports `AGENTS.md`, so that every agent reads the
  same rules. It is a file and not a symbolic link, because a Windows checkout
  writes a link as a small text file, as D89 found for the skills.
- `CONTRIBUTING.md`, for a person who changes the project. It has an Agent
  skills section with the command that installs them.
- `skills-lock.json`, which names the source of each skill.
- `.gitignore`, which ignores `.claude/settings.local.json`, the Claude Code
  permissions of one person.
- `.gitattributes`, which holds `* text=auto eol=lf`.

In `docs/`:

- `PLAN.md`, which holds the plan and the open questions for Ken.
- The decisions, with an index. A large project keeps each decision in a file
  of its own under `docs/decisions/`, and a small library keeps them in
  `PLAN.md`. D111 says which is which.
- `BACKLOG.md`, which holds the work that is known and not done.

Every library has all of these, including a small one.

## The skills

Every repository commits `simple-english`. A Go repository also commits
`go-pedantry`. Each is an ordinary folder in `.agents/skills/<name>` and in
`.claude/skills/<name>`, installed with `--copy`, as D89 decided for dbmeta. A
test like `TestSkillsAreCopies` fails on a link, on a missing copy and on two
copies that differ.

## The standing rules

`AGENTS.md` holds three rules that are the same in every repository:

1. Stage changes for review. Commit and push only when Ken says so.
2. Load `simple-english` before writing any text that a person reads: project
   documentation, a code comment, an error message or a commit message.
3. In a Go project, load `go-pedantry` before writing or reviewing Go code. A
   rule of the project wins where the two conflict.

Until now each agent learned the first rule from Ken and kept it in its own
memory, so an agent new to a repository did not know it.

## What changed in dbmeta

`CLAUDE.md` moved to `AGENTS.md`, which now opens with the standing rules, and
`CLAUDE.md` holds the import. `TestClaudeImportsAgents` checks it. The root
holds four documents, not three, which amends D50, and
`TestTheRootHoldsFourDocuments` checks that. `docs/BACKLOG.md` is new. D89's
skills and test did not change, and the rule that names `go-pedantry` amends
it.

n1ql and cql are not part of the rollout. Ken decided the same day that a clean
driver in dbimp replaces each, so neither gets further work.
