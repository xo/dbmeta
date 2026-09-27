package dbmeta_test

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// The documentation is about half the size of the code and it points at itself
// constantly: 553 references to a decision by number, and a link from most
// documents to most others. Moving eight files into docs/ broke none of them
// only because this ran afterwards.
//
// These tests read the repository rather than the package. They are here
// rather than in the test module because they need no database and no driver.

// markdownLink matches a relative link, and leaves an absolute one alone.
var markdownLink = regexp.MustCompile(`\]\((?:\./)?([^)#:]+\.md)(#[^)]*)?\)`)

// decisionRef matches a bare reference to a decision, such as D47.
var decisionRef = regexp.MustCompile(`\bD([1-9][0-9]*)\b`)

// bareMention matches a document named in running text, which is how a Go
// comment points at one, as in: See docs/NULLS.md for the rule.
var bareMention = regexp.MustCompile(`(?:^|[\s` + "`" + `(])((?:docs/)?[A-Z][A-Z_]*\.md)`)

// TestEveryMarkdownLinkResolves walks every document and every Go comment and
// checks that each file it names exists.
func TestEveryMarkdownLinkResolves(t *testing.T) {
	t.Parallel()
	for _, path := range repoFiles(t, ".md", ".go", ".yml") {
		body := read(t, path)
		dir := filepath.Dir(path)
		for _, m := range markdownLink.FindAllStringSubmatch(body, -1) {
			target := filepath.Join(dir, m[1])
			if _, err := os.Stat(target); err != nil {
				t.Errorf("%s: link to %s does not resolve to %s", path, m[1], target)
			}
		}
		// A bare mention, which is the only form a Go comment or a workflow
		// comment has. Markdown is covered by the link check above, which is
		// stronger, and prose in a decision record may name a file that was
		// deliberately deleted.
		if strings.HasSuffix(path, ".md") {
			continue
		}
		for _, m := range bareMention.FindAllStringSubmatch(body, -1) {
			if _, err := os.Stat(filepath.Join(".", m[1])); err != nil {
				t.Errorf("%s: names %s, which is not a file. Moving a document "+
					"means fixing the comments that point at it.", path, m[1])
			}
		}
	}
}

// TestEveryDecisionReferenceExists checks that a decision named by number is a
// decision that was written. A reference to a number nobody wrote is how a
// reader ends up trusting a rule that does not exist.
func TestEveryDecisionReferenceExists(t *testing.T) {
	t.Parallel()
	written := make(map[string]bool)
	for _, d := range decisions(t) {
		written[d.num] = true
	}
	if len(written) < 50 {
		t.Fatalf("expected at least 50 decisions, found %d", len(written))
	}
	for _, path := range repoFiles(t, ".md", ".go") {
		for _, m := range decisionRef.FindAllStringSubmatch(read(t, path), -1) {
			if !written[m[1]] {
				t.Errorf("%s: refers to D%s, which is not in docs/decisions", path, m[1])
			}
		}
	}
}

// TestEveryTestNameInTheDocsExists checks that a test the documentation names
// is a test that is written.
//
// A rule here is usually paired with the test that enforces it, and the pair
// is what makes the rule credible. Renaming the test breaks that silently:
// TestWorkflowReadsTheList
// was TestWorkflowMatchesTheList until D69 renamed it, and CLAUDE.md, now AGENTS.md, went on
// naming the old one.
func TestEveryTestNameInTheDocsExists(t *testing.T) {
	t.Parallel()
	written := make(map[string]bool)
	for _, path := range repoFiles(t, ".go") {
		for _, m := range regexp.MustCompile(`(?m)^func (Test[A-Za-z0-9]+)\(`).
			FindAllStringSubmatch(read(t, path), -1) {
			written[m[1]] = true
		}
	}
	if len(written) < 50 {
		t.Fatalf("expected at least 50 tests, found %d", len(written))
	}
	var named int
	for _, path := range repoFiles(t, ".md") {
		for _, m := range regexp.MustCompile(`\bTest[A-Z][A-Za-z0-9]*`).
			FindAllString(read(t, path), -1) {
			named++
			if !written[m] {
				t.Errorf("%s: names %s, which no test defines. Renaming a test "+
					"means fixing the documents that tell a reader to trust it.",
					path, m)
			}
		}
	}
	if named == 0 {
		t.Error("no document names a test any more, so this guards nothing")
	}
}

// decision is one file in docs/decisions.
type decision struct {
	num    string
	title  string
	status string
	file   string
}

// decisionFile names a decision file: D, the number in three digits, and the
// title in lower case words joined by hyphens.
var decisionFile = regexp.MustCompile(`^D(\d{3})-[a-z0-9-]+\.md$`)

// decisions reads every decision in docs/decisions, in order. D111 moved them
// there from one file, and each opens with its number, its title and its
// status:
//
//	# D89. Agent skills are committed as copies
//
//	Status: Decided.
func decisions(t *testing.T) []decision {
	t.Helper()
	dir := filepath.Join("docs", "decisions")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	head := regexp.MustCompile(`\A# D(\d+)\. (.+)\n\nStatus: (.+)\.\n`)
	var out []decision
	for _, e := range entries {
		if e.Name() == "README.md" {
			continue
		}
		name := decisionFile.FindStringSubmatch(e.Name())
		if name == nil {
			t.Errorf("%s: a decision file is named D, three digits, a hyphen and the"+
				" title in lower case words, such as D089-agent-skills.md", e.Name())
			continue
		}
		m := head.FindStringSubmatch(read(t, filepath.Join(dir, e.Name())))
		if m == nil {
			t.Errorf("%s: a decision opens with \"# D<n>. <title>\", a blank line and"+
				" \"Status: <status>.\"", e.Name())
			continue
		}
		if n, _ := strconv.Atoi(name[1]); strconv.Itoa(n) != m[1] {
			t.Errorf("%s holds D%s. The file name and the heading name one decision.", e.Name(), m[1])
		}
		out = append(out, decision{num: m[1], title: m[2], status: m[3], file: e.Name()})
	}
	if len(out) < 50 {
		t.Fatalf("expected at least 50 decisions in %s, found %d", dir, len(out))
	}
	return out
}

// TestTheDecisionIndexIsComplete checks the table in docs/decisions/README.md
// against the decision files. A reader finds a decision by its number in that
// table, so a missing row or a stale status there hides it.
func TestTheDecisionIndexIsComplete(t *testing.T) {
	t.Parallel()
	index := read(t, filepath.Join("docs", "decisions", "README.md"))
	rows := make(map[string]string)
	for _, m := range regexp.MustCompile(`(?m)^\| \[D(\d+)\]\(.*$`).FindAllStringSubmatch(index, -1) {
		rows[m[1]] = m[0]
	}
	written := make(map[string]bool)
	for _, d := range decisions(t) {
		written[d.num] = true
		want := fmt.Sprintf("| [D%s](%s) | %s | %s |", d.num, d.file, d.title, d.status)
		switch got, ok := rows[d.num]; {
		case !ok:
			t.Errorf("D%s has no row in docs/decisions/README.md. Add:\n%s", d.num, want)
		case got != want:
			t.Errorf("D%s: the row in docs/decisions/README.md is\n%s\nand the file says\n%s", d.num, got, want)
		}
	}
	for num := range rows {
		if !written[num] {
			t.Errorf("docs/decisions/README.md has a row for D%s, and no file holds it", num)
		}
	}
}

// TestTheCountsInProseAreRight checks every number the root documents quote
// for how many of something there is.
//
// A number written in prose goes stale the next time somebody adds one, and
// the decision count had gone stale in two documents at once. It is cheaper to
// check than to remember. There are two such numbers: how many decisions
// docs/decisions holds, and how many hard rules AGENTS.md holds.
func TestTheCountsInProseAreRight(t *testing.T) {
	t.Parallel()
	all := decisions(t)
	decisions := len(all)
	// A decision that amends or replaces an earlier one says so in its own
	// status, in the active voice. The one it names says it back, which
	// TestAnAmendmentPointsBothWays checks, so counting one side counts both.
	var amending int
	for _, d := range all {
		if regexp.MustCompile(`(?i)\b(?:amends|supersedes) D\d+`).MatchString(d.title + ". " + d.status) {
			amending++
		}
	}
	rules := len(regexp.MustCompile(`(?m)^(\d+)\. `).
		FindAllString(hardRules(t), -1))
	if decisions == 0 || rules == 0 || amending == 0 {
		t.Fatalf("found %d decisions, %d of them amending, and %d hard rules,"+
			" expected some of each", decisions, amending, rules)
	}
	for _, c := range []struct {
		what string
		want int
		// phrase captures the number as the documents write it. Each is
		// anchored on its own wording, because the two counts are a sentence
		// apart in CONTRIBUTING.md and a loose pattern reads one as the other.
		phrase *regexp.Regexp
	}{
		{"decisions", decisions, regexp.MustCompile(`a table of all (\d+)`)},
		{"decisions", decisions, regexp.MustCompile(`[Ee]very decision, (\d+) of them`)},
		{"decisions", decisions, regexp.MustCompile(`lists all (\d+) with their`)},
		{"hard rules", rules, regexp.MustCompile(`holds the rules: (\d+) of them`)},
		{"amending decisions", amending,
			regexp.MustCompile(`(\d+) (?:of them )?amend or replace`)},
	} {
		var found bool
		// The index quotes the amendment count in its own introduction. When
		// it was the top of docs/PLAN.md it was the one place this test did
		// not look, so that number went stale while the three it did look at
		// stayed right.
		for _, name := range []string{
			"README.md", "AGENTS.md", "CONTRIBUTING.md",
			filepath.Join("docs", "decisions", "README.md"),
		} {
			for _, m := range c.phrase.FindAllStringSubmatch(read(t, name), -1) {
				found = true
				if m[1] != strconv.Itoa(c.want) {
					t.Errorf("%s says there are %s %s and there are %d: %q",
						name, m[1], c.what, c.want, strings.TrimSpace(m[0]))
				}
			}
		}
		if !found {
			t.Errorf("no document matches %v any more. Fix the pattern or drop it, "+
				"because a guard that matches nothing guards nothing.", c.phrase)
		}
	}
}

// hardRules returns the numbered list of hard rules from AGENTS.md.
func hardRules(t *testing.T) string {
	t.Helper()
	_, rest, ok := strings.Cut(read(t, "AGENTS.md"), "\n## Hard rules\n")
	if !ok {
		t.Fatal("AGENTS.md has no Hard rules section")
	}
	// Everything up to the next heading, or the rest of the file.
	rules, _, _ := strings.Cut(rest, "\n## ")
	return rules
}

// TestTheRootHoldsFourDocuments holds the layout D50 decided and D110
// amended. A document that appears in the root is one nobody filed.
func TestTheRootHoldsFourDocuments(t *testing.T) {
	t.Parallel()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	allowed := map[string]bool{
		"README.md": true, "AGENTS.md": true, "CLAUDE.md": true, "CONTRIBUTING.md": true,
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		if !allowed[e.Name()] {
			t.Errorf("%s is in the repository root. Only README, AGENTS, CLAUDE and CONTRIBUTING belong there, "+
				"and everything else goes in docs/. See D50.", e.Name())
		}
	}
	for name := range allowed {
		if _, err := os.Stat(name); err != nil {
			t.Errorf("expected %s in the repository root", name)
		}
	}
}

// TestClaudeImportsAgents holds D110. AGENTS.md holds the rules, because
// Codex and the other agents read it, and CLAUDE.md imports it, so that
// Claude Code reads the same rules. A rule written in CLAUDE.md would reach
// Claude Code alone. A symbolic link would not do, because a Windows checkout
// writes a link as a small text file, as D89 found for the skills.
func TestClaudeImportsAgents(t *testing.T) {
	t.Parallel()
	info, err := os.Lstat("CLAUDE.md")
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Fatal("CLAUDE.md is a symbolic link. Make it a file that holds @AGENTS.md. See D110")
	}
	if got := strings.TrimSpace(read(t, "CLAUDE.md")); got != "@AGENTS.md" {
		t.Errorf("CLAUDE.md holds %q. It holds only @AGENTS.md, and the rules go in"+
			" AGENTS.md. See D110", got)
	}
}

// TestNoSectionHeadingIsRepeated checks that a level two heading appears once
// in the document that holds it.
//
// A deeper heading repeats on purpose, because docs/COVERAGE.md asks the same
// questions of every product and "### What it answers" is the answer to one
// of them. A level two heading is a section, and two sections with one name
// means a section landed in the wrong place. docs/COVERAGE.md had three
// called "Which answers depend on who is asking": the document wide one, and
// the SAP HANA and Trino ones, which had been left outside their products and
// after Apache Hive, where nobody reading about HANA would find them.
func TestNoSectionHeadingIsRepeated(t *testing.T) {
	t.Parallel()
	for _, path := range repoFiles(t, ".md") {
		seen := make(map[string]bool)
		for _, m := range regexp.MustCompile(`(?m)^## (.+)$`).
			FindAllStringSubmatch(read(t, path), -1) {
			if seen[m[1]] {
				t.Errorf("%s has two sections called %q. A repeated section "+
					"heading is how one lands under the wrong product.", path, m[1])
			}
			seen[m[1]] = true
		}
	}
}

// TestEveryDocumentIsInBothTables checks that a document in docs/ is named in
// the table at the top of AGENTS.md and in the one in README.md.
//
// Both documents say to do this and neither could tell you whether it had
// been done. AGENTS.md goes further and says a document that is not in that
// table does not exist, which is only true if something holds it. See D50.
func TestEveryDocumentIsInBothTables(t *testing.T) {
	t.Parallel()
	entries, err := os.ReadDir("docs")
	if err != nil {
		t.Fatal(err)
	}
	tables := map[string]string{
		"AGENTS.md": read(t, "AGENTS.md"),
		"README.md": read(t, "README.md"),
	}
	var found int
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		found++
		for name, body := range tables {
			if !strings.Contains(body, "docs/"+e.Name()) {
				t.Errorf("%s does not name docs/%s. A document nobody can find "+
					"is a document nobody reads. See D50.", name, e.Name())
			}
		}
	}
	if found == 0 {
		t.Error("docs/ holds no document, so this guards nothing")
	}
}

// TestAnAmendmentPointsBothWays is the guard that made splitting the decision
// log safe.
//
// D50 rejected splitting it because an amendment would live in a different
// file from the decision it amends, so a reader landing on the older one would
// get a rule that no longer holds. One file never fixed that by itself. The
// status of both decisions has to say so, and each file opens with its status,
// so the status is the first thing anyone reads. D111 split the log on that
// ground.
//
// The index found the first case the moment it existed: D48 said it amends
// D26, and D26 said "Decided".
func TestAnAmendmentPointsBothWays(t *testing.T) {
	t.Parallel()
	status := make(map[string]string)
	for _, d := range decisions(t) {
		status[d.num] = d.title + ". " + d.status
	}
	// "Amends D26" and "Superseded by D24" both name another decision, and
	// that decision has to name this one back.
	naming := regexp.MustCompile(`(?i)\b(?:amends|supersedes|superseded by|replaced by|amended by) D(\d+)`)
	for num, head := range status {
		for _, m := range naming.FindAllStringSubmatch(head, -1) {
			other := m[1]
			if _, ok := status[other]; !ok {
				t.Errorf("D%s names D%s, which is not a decision", num, other)
				continue
			}
			if !strings.Contains(status[other], "D"+num) {
				t.Errorf("D%s says %q, and D%s does not mention D%s.\n"+
					"An amendment has to be visible from both sides, or a reader "+
					"who finds the older decision gets a rule that no longer holds. See D50.",
					num, head, other, num)
			}
		}
	}
}

// repoFiles returns every file with one of the given extensions, skipping the
// directories a build leaves behind.
func repoFiles(t *testing.T, exts ...string) []string {
	t.Helper()
	var out []string
	err := filepath.WalkDir(".", func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if strings.HasPrefix(d.Name(), ".") && d.Name() != "." && d.Name() != ".github" {
				return filepath.SkipDir
			}
			// A state directory holds what a script fetched or built, such as
			// the virtual machine disks and Oracle's Dockerfiles. It is not
			// this repository's code and it is not checked in, so its
			// documentation is not ours to hold together. Every .gitignore
			// here names it.
			if d.Name() == "state" {
				return filepath.SkipDir
			}
			return nil
		}
		for _, ext := range exts {
			if strings.HasSuffix(path, ext) {
				out = append(out, path)
				return nil
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
