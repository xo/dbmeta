package dbmeta_test

import (
	"os"
	"path/filepath"
	"regexp"
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
var decisionRef = regexp.MustCompile(`\bD([1-9][0-9]?)\b`)

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
	plan := read(t, filepath.Join("docs", "PLAN.md"))
	written := make(map[string]bool)
	for _, m := range regexp.MustCompile(`(?m)^### D(\d+)\.`).FindAllStringSubmatch(plan, -1) {
		written[m[1]] = true
	}
	if len(written) < 50 {
		t.Fatalf("expected at least 50 decisions, found %d", len(written))
	}
	for _, path := range repoFiles(t, ".md", ".go") {
		for _, m := range decisionRef.FindAllStringSubmatch(read(t, path), -1) {
			if !written[m[1]] {
				t.Errorf("%s: refers to D%s, which is not in docs/PLAN.md", path, m[1])
			}
		}
	}
}

// TestTheDecisionIndexIsComplete checks the table at the top of docs/PLAN.md
// against the decisions below it. D50 keeps the log in one file on the
// condition that the index makes it navigable, so a missing entry undoes that.
func TestTheDecisionIndexIsComplete(t *testing.T) {
	t.Parallel()
	plan := read(t, filepath.Join("docs", "PLAN.md"))
	indexed := make(map[string]bool)
	for _, m := range regexp.MustCompile(`(?m)^\| \[D(\d+)\]\(#([^)]+)\)`).FindAllStringSubmatch(plan, -1) {
		indexed[m[1]] = true
		if !strings.HasPrefix(m[2], "d"+m[1]+"-") {
			t.Errorf("D%s: the index anchor %q does not point at it", m[1], m[2])
		}
	}
	for _, m := range regexp.MustCompile(`(?m)^### D(\d+)\.`).FindAllStringSubmatch(plan, -1) {
		if !indexed[m[1]] {
			t.Errorf("D%s is written and is not in the index at the top of docs/PLAN.md", m[1])
		}
	}
}

// TestTheRootHoldsThreeDocuments holds the layout D50 decided. A document that
// appears in the root is one nobody filed.
func TestTheRootHoldsThreeDocuments(t *testing.T) {
	t.Parallel()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	allowed := map[string]bool{"README.md": true, "CLAUDE.md": true, "CONTRIBUTING.md": true}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		if !allowed[e.Name()] {
			t.Errorf("%s is in the repository root. Only README, CLAUDE and CONTRIBUTING belong there, "+
				"and everything else goes in docs/. See D50.", e.Name())
		}
	}
	for name := range allowed {
		if _, err := os.Stat(name); err != nil {
			t.Errorf("expected %s in the repository root", name)
		}
	}
}

// TestAnAmendmentPointsBothWays is the guard that makes one file worth keeping.
//
// D50 rejected splitting the decision log because an amendment would live in a
// different file from the decision it amends, so a reader landing on the older
// one would get a rule that no longer holds. One file does not fix that by
// itself. The status of both decisions has to say so, and the index makes the
// status the first thing anyone reads.
//
// The index found the first case the moment it existed: D48 said it amends
// D26, and D26 said "Decided".
func TestAnAmendmentPointsBothWays(t *testing.T) {
	t.Parallel()
	plan := read(t, filepath.Join("docs", "PLAN.md"))
	status := make(map[string]string)
	for _, m := range regexp.MustCompile(`(?m)^### D(\d+)\. (.+)$`).FindAllStringSubmatch(plan, -1) {
		status[m[1]] = m[2]
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
