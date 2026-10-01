package dbmeta_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// proseRule is one rule of the simple-english skill that a regular expression
// can check. The skill has more rules than these. Sentence length, the verb
// after a comma, the voice and one word for one meaning need a person, because
// a regular expression cannot find a sentence or a part of speech. Gemini and
// DeepSeek were asked which rules a machine can check, and both drew the line
// here.
type proseRule struct {
	name string
	re   *regexp.Regexp
	fix  string
}

// proseRules are the rules TestProseIsSimpleEnglish checks.
var proseRules = []proseRule{{
	name: "modal",
	re:   regexp.MustCompile(`(?i)\b(?:should|would|may|might|could)\b`),
	fix:  "use can, will or must, or state the fact in the present tense",
}, {
	name: "semicolon",
	re:   regexp.MustCompile(`;`),
	fix:  "write two sentences, or name the relation",
}, {
	name: "dash",
	re:   regexp.MustCompile(`\x{2014}|\x{2013}|\s--\s`),
	fix:  "write two sentences, or name the relation",
}, {
	name: "contraction",
	re: regexp.MustCompile(`(?i)\b[a-z]+n't\b|\b(?:it|that|there|here|what|let|he|she|who)'s\b` +
		`|\b(?:i|you|we|they|it|that|there|who)'(?:re|ve|ll|d|m)\b`),
	fix: "write the words in full",
}, {
	name: "bold",
	re:   regexp.MustCompile(`\*\*|__[A-Za-z]`),
	fix:  "remove the bold, or make the text a heading",
}, {
	name: "perfect",
	re:   regexp.MustCompile(`(?i)\b(?:has|have) been\b`),
	fix:  "use the simple past or the present",
}, {
	name: "spelling",
	re: regexp.MustCompile(`(?i)\b(?:licence[sd]?|behaviours?|colours?|favour\w*|honour\w*|centres?` +
		`|metres?|judgements?|catalogue[ds]?|cancell(?:ed|ing)|travell(?:ed|ing)|modell(?:ed|ing)` +
		`|labell(?:ed|ing)|signall(?:ed|ing)|artefacts?|grey|whilst|amongst|learnt|spelt|towards` +
		`|analys(?:e|ed|es|ing)|(?:un)?(?:organi|recogni|normali|generali|capitali|parameteri|initiali` +
		`|seriali|materiali|customi|optimi|prioriti|authori|summari|finali|utili|standardi|synchroni` +
		`|minimi|maximi|categori|emphasi|reali|speciali|visuali|tokeni|saniti)s(?:e|es|ed|ing|ations?))\b`),
	fix: "use the American spelling",
}, {
	name: "filler",
	re: regexp.MustCompile(`(?i)\b(?:simply|seamless(?:ly)?|robust|powerful|comprehensive|leverag(?:e|es|ed|ing)` +
		`|crucial|in order to|it is worth noting|note that|please)\b`),
	fix: "delete the word, or state the fact it stands for",
}, {
	name: "abbreviation",
	re:   regexp.MustCompile(`(?i)\b(?:e\.g|i\.e|etc)\.`),
	fix:  "write for example, that is, or name the rest",
}}

var (
	// codeSpan is a Markdown or Go doc comment code span.
	codeSpan = regexp.MustCompile("`[^`]*`")
	// quoted is text in double quotes, which is a quotation, a server's
	// message or a value, and is not ours to rewrite.
	quoted = regexp.MustCompile(`"[^"]*"|\x{201c}[^\x{201d}]*\x{201d}`)
	// linkTarget is the target of a Markdown link. The link text is prose and
	// stays.
	linkTarget = regexp.MustCompile(`\]\([^)]*\)`)
	// webAddress is a URL, which holds a semicolon or any word at all.
	webAddress = regexp.MustCompile(`\b[a-z][a-z0-9+.-]*://\S+`)
	// entity is an HTML character reference, which ends in a semicolon.
	entity = regexp.MustCompile(`&#?[A-Za-z0-9]+;`)
	// htmlComment is a comment in Markdown, which nobody reads rendered.
	htmlComment = regexp.MustCompile(`(?s)<!--.*?-->`)
	// month matches the name of the fifth month, which is not a modal.
	month = regexp.MustCompile(`\b(?:in|of|since|until|by|to) May\b|\bMay [0-9]{1,4}\b|\b[0-9]{1,2} May\b`)
	// fence opens or closes a fenced code block in Markdown.
	fence = regexp.MustCompile("^\\s*(?:```|~~~)")
)

// proseLine is one line of text a person reads, with what is not prose
// already masked out of it.
type proseLine struct {
	pos  string
	text string
}

// mask removes what the rules do not apply to: code, quotations, URLs and
// markup. It keeps every line break, so that a line of the result is the same
// line of the input.
func mask(s string) string {
	blank := func(m string) string {
		return strings.Map(func(r rune) rune {
			if r == '\n' {
				return r
			}
			return ' '
		}, m)
	}
	for _, re := range []*regexp.Regexp{codeSpan, htmlComment, linkTarget, webAddress, quoted, entity, month} {
		s = re.ReplaceAllStringFunc(s, blank)
	}
	return s
}

// paragraph collects the lines of one paragraph, so that a code span or a
// quotation that breaks across two lines is masked as one. A span never
// crosses a paragraph, so an unmatched backtick or quote masks no more than
// the rest of its own paragraph.
type paragraph struct {
	path  string
	first int
	lines []string
}

// add appends a line, which is line n of the file.
func (p *paragraph) add(n int, line string) {
	if len(p.lines) == 0 {
		p.first = n
	}
	p.lines = append(p.lines, line)
}

// flush masks the paragraph, appends its lines to out, and empties it.
func (p *paragraph) flush(out []proseLine) []proseLine {
	if len(p.lines) == 0 {
		return out
	}
	for i, line := range strings.Split(mask(strings.Join(p.lines, "\n")), "\n") {
		out = append(out, proseLine{p.path + ":" + strconv.Itoa(p.first+i), line})
	}
	p.lines = p.lines[:0]
	return out
}

// markdownProse returns the prose lines of a Markdown file. A fenced code
// block is code, and an indented one does not occur here.
func markdownProse(path, text string) []proseLine {
	var out []proseLine
	para := paragraph{path: path}
	inFence := false
	for i, line := range strings.Split(text, "\n") {
		switch {
		case fence.MatchString(line):
			out = para.flush(out)
			inFence = !inFence
		case inFence:
		case strings.TrimSpace(line) == "":
			out = para.flush(out)
		default:
			para.add(i+1, line)
		}
	}
	return para.flush(out)
}

// lineComments are the files whose prose is in line comments, by the end of
// the file name, with the marker that opens a comment in each.
var lineComments = []struct {
	suffix string
	marker *regexp.Regexp
}{
	{".yml", regexp.MustCompile(`^\s*#`)},
	{"Containerfile", regexp.MustCompile(`^\s*#`)},
	{".gitignore", regexp.MustCompile(`^\s*#`)},
	{".gitattributes", regexp.MustCompile(`^\s*#`)},
	{".ini", regexp.MustCompile(`^\s*;`)},
	{".bat", regexp.MustCompile(`(?i)^\s*@?(?:rem\b|::)`)},
}

// directive is a line comment that a tool reads, such as the syntax line of a
// Containerfile.
var directive = regexp.MustCompile(`^\s*(?:syntax|escape|check)=`)

// commentProse returns the prose of a file whose comments open with marker
// and run to the end of the line. A line that is not a comment ends a
// paragraph.
func commentProse(path, text string, marker *regexp.Regexp) []proseLine {
	var out []proseLine
	para := paragraph{path: path}
	for i, line := range strings.Split(text, "\n") {
		loc := marker.FindStringIndex(line)
		if loc == nil {
			out = para.flush(out)
			continue
		}
		body := line[loc[1]:]
		if strings.TrimSpace(body) == "" || directive.MatchString(body) {
			out = para.flush(out)
			continue
		}
		para.add(i+1, body)
	}
	return para.flush(out)
}

// goProse returns the prose of a Go file: its comments, and the messages it
// gives a person, which are the text of an error and of a test failure.
func goProse(t *testing.T, path, text string) []proseLine {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, text, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("parsing %s: %v", path, err)
	}
	var out []proseLine
	para := paragraph{path: path}
	for _, group := range f.Comments {
		for _, c := range group.List {
			line := fset.Position(c.Pos()).Line
			body, ok := strings.CutPrefix(c.Text, "//")
			if !ok {
				body = strings.TrimSuffix(strings.TrimPrefix(c.Text, "/*"), "*/")
			}
			for j, s := range strings.Split(body, "\n") {
				// A directive is for a tool, and a line indented past the
				// comment's own space is a code example. Either one ends a
				// paragraph, and so does a blank line.
				switch {
				case j == 0 && ok && (strings.HasPrefix(s, "go:") || strings.HasPrefix(s, "nolint") ||
					strings.HasPrefix(s, "line ") || strings.HasPrefix(s, "export ")),
					strings.HasPrefix(s, "\t") || strings.HasPrefix(s, " \t") || strings.HasPrefix(s, "  "),
					strings.TrimSpace(s) == "":
					out = para.flush(out)
				default:
					if len(para.lines) != 0 && para.first+len(para.lines) != line+j {
						out = para.flush(out)
					}
					para.add(line+j, s)
				}
			}
		}
		out = para.flush(out)
	}
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || len(call.Args) == 0 || !isMessage(call.Fun) {
			return true
		}
		ast.Inspect(call.Args[0], func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			s, err := strconv.Unquote(lit.Value)
			if err != nil {
				return true
			}
			pos := path + ":" + strconv.Itoa(fset.Position(lit.Pos()).Line)
			out = append(out, proseLine{pos, mask(s)})
			return true
		})
		return false
	})
	return out
}

// isMessage reports whether a call gives a person a message as its first
// argument: an error, or a test that fails or skips.
func isMessage(fun ast.Expr) bool {
	sel, ok := fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	x, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}
	switch x.Name {
	case "errors":
		return sel.Sel.Name == "New"
	case "fmt":
		return sel.Sel.Name == "Errorf"
	case "t", "b", "tb":
		switch sel.Sel.Name {
		case "Error", "Errorf", "Fatal", "Fatalf", "Skip", "Skipf", "Log", "Logf":
			return true
		}
	}
	return false
}

// TestProseIsSimpleEnglish checks the rules of the simple-english skill that a
// machine can check, over every document, every comment, and every message
// an error or a test gives a person. A comment is one in Go, or a line
// comment in a YAML file, a Containerfile, a batch file, an INI file or a git
// file. AGENTS.md says to follow the skill for
// all of that text, and D89 says why. The skill itself, under .claude and
// .agents, comes from elsewhere and is not checked.
//
// Text in backticks or in double quotes is not checked, because it is code,
// a value, or somebody else's words. There is no other exception. If a rule
// reports a sentence, rewrite the sentence.
func TestProseIsSimpleEnglish(t *testing.T) {
	var lines []proseLine
	for _, path := range repoFiles(t, ".md") {
		lines = append(lines, markdownProse(path, read(t, path))...)
	}
	for _, path := range repoFiles(t, ".go") {
		lines = append(lines, goProse(t, path, read(t, path))...)
	}
	for _, c := range lineComments {
		for _, path := range repoFiles(t, c.suffix) {
			lines = append(lines, commentProse(path, read(t, path), c.marker)...)
		}
	}
	for _, rule := range proseRules {
		t.Run(rule.name, func(t *testing.T) {
			var found int
			for _, line := range lines {
				for _, m := range rule.re.FindAllString(line.text, -1) {
					t.Errorf("%s: %q: %s", line.pos, m, rule.fix)
					found++
				}
			}
			if found != 0 {
				t.Logf("%d found", found)
			}
		})
	}
}

// TestProseMasksWhatIsNotProse checks the scanner against text whose findings
// are known, so that a mask that grows too wide cannot hide every fault in
// the repository and pass.
func TestProseMasksWhatIsNotProse(t *testing.T) {
	md := strings.Join([]string{
		"A query would block.",
		"",
		"The server says \"it may not",
		"be read\", and `x; y",
		"z` is code.",
		"",
		"```",
		"we should not look here",
		"```",
		"",
		"A <!-- would --> comment, a [link](a;b), and May 2026.",
		"Semicolons; split it.",
	}, "\n")
	src := strings.Join([]string{
		"package p",
		"",
		"import \"errors\"",
		"",
		"// F might fail.",
		"//",
		"//\tcode(); could",
		"func F() error {",
		"\treturn errors.New(\"reading it would fail\")",
		"}",
	}, "\n")
	lines := append(markdownProse("a.md", md), goProse(t, "a.go", src)...)
	var got []string
	for _, rule := range proseRules {
		for _, line := range lines {
			for _, m := range rule.re.FindAllString(line.text, -1) {
				got = append(got, line.pos+" "+rule.name+" "+m)
			}
		}
	}
	want := []string{
		"a.md:1 modal would",
		"a.go:5 modal might",
		"a.go:9 modal would",
		"a.md:12 semicolon ;",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("found:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}
