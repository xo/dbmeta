# D89. Agent skills are committed as copies, and one command installs them

Status: Amended by D110.

The repository carries two agent skills. A skill is a set of instructions that
a coding agent loads for a task. `simple-english` sets how prose is written,
and `go-pedantry` sets how Go is written. Commit 28ccdaa added both, with
`skills-lock.json`. Three facts about them were not written down: where they
come from, how a person updates them, and what a Windows checkout does with
them. Ken decided all three on 2026-09-27.

## Where they come from

`skills-lock.json` names the source of each skill. The `skills` command from
npm writes that file, and version 1.7.0 is the one measured here. It writes
each skill into two folders. Codex and the other agents read
`.agents/skills/<name>`, and Claude Code reads `.claude/skills/<name>`.
`CONTRIBUTING.md` holds the command, under Agent skills.

The command was run in an empty repository, once for each skill, with
`--agent codex claude-code --copy -y`. The result was the same `.agents`
folder and the same `skills-lock.json` that this repository holds, byte for
byte. `npx skills experimental_install` restores the skills from the lock
file, but it writes only `.agents/skills`. That is why the document names
`add` and not the restore command.

## A Windows checkout gets folders, not links

Until this decision, `.claude/skills/<name>` was a symbolic link to
`.agents/skills/<name>`. The `skills` command writes a link by default. Git
writes a symbolic link as a small text file that holds the target path when
`core.symlinks` is off, and it is off by default on Windows. Claude Code then
finds a file where it expects a folder. It loads no skill and it reports
nothing.

Both copies are now ordinary folders, and the command takes `--copy` so that
it writes them that way. Two copies can drift apart. `TestSkillsAreCopies`
in the root module stops that. It fails on a link, on a copy that is missing,
on two copies that differ, and on a skill folder that the lock file does not
name.

The alternative was to keep the links and to tell each Windows developer to
turn on `core.symlinks`, which needs Developer Mode or an administrator. Ken
wanted the repository to handle it. A folder works on every checkout with no
setting.

## Line endings

A Windows checkout changes one more thing. Git for Windows writes CRLF by
default, and some files here must keep the same bytes on every machine.
`.gitattributes` makes every text file LF on every checkout. Three kinds of
file need it:

1. The golden files in `test/testdata`. The reader splits them at LF, so a
   section header that ends in CR is not a header and the test stops.
2. The Containerfile that `dbrun` embeds. It goes to a Linux build.
3. The two `.bat` files that `dbrun` embeds. They go to a Windows machine,
   and every Windows machine here was provisioned with them as LF. LF is the
   tested form, so they stay LF.

Every file in the index was LF already, so the rule changed no file.
`.gitattributes` is not an ignore file, so D58 still holds: there is one
`.gitignore`.

## The file that belongs to one person

`.claude/settings.local.json` holds the Claude Code permissions of one
person. The root `.gitignore` now ignores it. The shared
`.claude/settings.json` is not ignored, because a setting that belongs to the
project is committed.

## Which skill is for what

Ken asked that `simple-english` apply to every text a person reads: a
document, a code comment, an error message and a commit message. `CLAUDE.md`
tells an agent to load it first. Its rules include the ones `CLAUDE.md`
already had, which are short sentences, the active voice, `can`, `will` and
`must`, no semicolons, no em dashes, and the condition before the command. It
adds more, such as no contractions and one word for one meaning.
