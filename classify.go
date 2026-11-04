package dbmeta

import (
	"slices"
	"strings"
	"unicode"
)

// Returning is how the INSERT, UPDATE, DELETE and MERGE statements of a
// product give back the rows that they changed. See D264.
type Returning int

// Returning values.
const (
	// ReturningNone is a product with no such clause, or one whose clause
	// puts the rows in variables, as Oracle's RETURNING INTO does. The
	// statement returns no rows.
	ReturningNone Returning = iota
	// ReturningRows is a RETURNING clause that sends the rows to the client.
	// PostgreSQL, CockroachDB, SQLite, DuckDB, MariaDB and Firebird have it.
	ReturningRows
	// ReturningOutput is the OUTPUT clause of SQL Server. It sends the rows to
	// the client unless it has an INTO.
	ReturningOutput
	// ReturningThenReturn is the THEN RETURN clause of Spanner.
	ReturningThenReturn
)

// Statements is what a product's statements return, which a client needs to
// choose between a query and an exec. See [Dialect.Classify] and D264.
type Statements struct {
	// Returning says how a statement gives back the rows that it changed.
	Returning Returning
	// SetListsSettings says that SET with nothing after it, or with ALL or -v,
	// lists the settings. True for Impala and Hive.
	SetListsSettings bool
	// ExecuteReturnsRows says that EXECUTE runs a procedure and returns its
	// rows, as EXEC does. True for SQL Server. In every other product EXECUTE
	// runs a prepared statement and returns no rows.
	ExecuteReturnsRows bool
}

// Classify says what a client does with a statement. It returns the command
// tag that a client prints after a statement that returns no rows, such as
// INSERT or CREATE TABLE, and whether the statement returns rows, so that the
// client passes it to a query and not to an exec.
//
// The decision reads the whole statement, outside of strings, quoted names
// and comments, by the lexical forms of the dialect ([Syntax]). It does not
// need the release, because a release that lacks a clause refuses the
// statement, and the client prints the error. A statement that has a RETURNING
// clause at its top level returns rows, and so does an OUTPUT clause of SQL
// Server that has no INTO. A statement of an unknown dialect is read with the
// same rules and none of the product's clauses. See D264.
func (d Dialect) Classify(stmt string) (tag string, rows bool) {
	info, _ := d.Info()
	if info == nil {
		info = new(Info)
	}
	tag, rows = classify(stmt, info)
	return tag, rows || info.EveryStatementIsAQuery
}

// token is a word, a mark or something that a word must not match, such as a
// string.
type token struct {
	kind  byte // 'w' for a word, 'p' for a mark, 'o' for anything else
	text  string
	depth int
}

// lex splits stmt into tokens. A string, a quoted name and a comment is one
// token of kind 'o' and a comment is none.
func lex(stmt string, syn Syntax) []token {
	var toks []token
	depth := 0
	rs := []rune(stmt)
	isWord := func(r rune) bool {
		return r == '_' || r == '$' || r == '@' || r == '#' && !syn.HashComments ||
			unicode.IsLetter(r) || unicode.IsDigit(r)
	}
	hasPrefix := func(i int, s string) bool {
		return strings.HasPrefix(string(rs[i:min(i+len(s), len(rs))]), s)
	}
	// skipTo returns the index after the closer, doubling the closer to
	// escape it when double is true.
	skipTo := func(i int, closer rune, double bool) int {
		for i++; i < len(rs); i++ {
			if rs[i] == closer {
				if double && i+1 < len(rs) && rs[i+1] == closer {
					i++
					continue
				}
				return i + 1
			}
		}
		return len(rs)
	}
	for i := 0; i < len(rs); {
		r := rs[i]
		switch {
		case unicode.IsSpace(r):
			i++
		case syn.DashComments && hasPrefix(i, "--"), syn.SlashComments && hasPrefix(i, "//"), syn.HashComments && r == '#':
			for i < len(rs) && rs[i] != '\n' {
				i++
			}
		case syn.BlockComments && hasPrefix(i, "/*"):
			i = indexRunes(rs, i+2, "*/")
			if i < len(rs) {
				i += 2
			}
		case r == '\'' || r == '"':
			i = skipTo(i, r, true)
			toks = append(toks, token{kind: 'o', depth: depth})
		case r == '`' && syn.Backticks:
			i = skipTo(i, '`', true)
			toks = append(toks, token{kind: 'o', depth: depth})
		case r == '[' && syn.Brackets:
			i = skipTo(i, ']', true)
			toks = append(toks, token{kind: 'o', depth: depth})
		case r == '$' && syn.DollarQuotes && dollarTag(rs, i) != "":
			tag := dollarTag(rs, i)
			i = indexRunes(rs, i+len(tag), tag)
			if i < len(rs) {
				i += len(tag)
			}
			toks = append(toks, token{kind: 'o', depth: depth})
		case isWord(r):
			j := i
			for j < len(rs) && isWord(rs[j]) {
				j++
			}
			toks = append(toks, token{kind: 'w', text: strings.ToUpper(string(rs[i:j])), depth: depth})
			i = j
		default:
			switch r {
			case '(':
				toks = append(toks, token{kind: 'p', text: "(", depth: depth})
				depth++
			case ')':
				depth = max(depth-1, 0)
				toks = append(toks, token{kind: 'p', text: ")", depth: depth})
			default:
				toks = append(toks, token{kind: 'p', text: string(r), depth: depth})
			}
			i++
		}
	}
	return toks
}

// indexRunes returns the index of the first sub at or after from, and
// len(rs) when there is none.
func indexRunes(rs []rune, from int, sub string) int {
	for i := from; i < len(rs); i++ {
		if strings.HasPrefix(string(rs[i:min(i+len(sub), len(rs))]), sub) {
			return i
		}
	}
	return len(rs)
}

// dollarTag returns the opening of a dollar quoted string that starts at
// rs[i], such as $$ or $body$, and an empty string when none starts there.
func dollarTag(rs []rune, i int) string {
	for j := i + 1; j < len(rs); j++ {
		switch r := rs[j]; {
		case r == '$':
			return string(rs[i : j+1])
		case r == '_' || unicode.IsLetter(r) || j > i+1 && unicode.IsDigit(r):
		default:
			return ""
		}
	}
	return ""
}

// topWord returns the index of the first word of the text at the top level
// of the statement, from index from on, and -1 when there is none.
func topWord(toks []token, from int, words ...string) int {
	for i := from; i < len(toks); i++ {
		if toks[i].kind != 'w' || toks[i].depth != 0 {
			continue
		}
		if slices.Contains(words, toks[i].text) {
			return i
		}
	}
	return -1
}

// dml are the words that start a statement that changes rows.
var dml = []string{"INSERT", "UPDATE", "DELETE", "MERGE", "REPLACE", "UPSERT"}

func classify(stmt string, info *Info) (string, bool) {
	toks := lex(stmt, info.Syntax)
	if len(toks) == 0 {
		return "EXEC", false
	}
	// The words at the start of the statement make the tag.
	var words []string
	for _, t := range toks {
		if t.kind != 'w' || len(words) == 12 {
			break
		}
		words = append(words, t.text)
	}
	if len(words) == 0 {
		// A statement that starts with a parenthesis is a query, as in
		// (SELECT 1) UNION (SELECT 2).
		if toks[0].text == "(" {
			return "SELECT", true
		}
		return "EXEC", false
	}
	first := words[0]
	if len(words) > 1 && queryMap[first+" "+words[1]] {
		return first + " " + words[1], true
	}
	switch first {
	case "SET":
		if info.Statements.SetListsSettings && listsSettings(toks) {
			return "SET", true
		}
	case "EXECUTE":
		if info.Statements.ExecuteReturnsRows {
			return "EXECUTE", true
		}
	case "WITH":
		return withTag(toks, info)
	case "SELECT":
		if topWord(toks, 0, "INTO") >= 0 {
			return "SELECT INTO", false
		}
		return "SELECT", true
	case "PRAGMA":
		for _, t := range toks {
			if t.kind == 'p' && t.text == "=" {
				return "PRAGMA", false
			}
		}
		return "PRAGMA", true
	}
	if slices.Contains(dml, first) {
		return first, returnsRows(toks, 0, info.Statements.Returning)
	}
	if queryMap[first] && first != "SET" && first != "EXECUTE" {
		return first, true
	}
	return execTag(words), false
}

// listsSettings says that the tokens of a SET statement hold nothing, or ALL,
// or -v, which lists the settings.
func listsSettings(toks []token) bool {
	rest := toks[1:]
	switch len(rest) {
	case 0:
		return true
	case 1:
		return rest[0].kind == 'w' && rest[0].text == "ALL" || rest[0].text == ";"
	case 2:
		return rest[0].text == "-" && rest[1].text == "V"
	}
	return false
}

// withTag classifies a WITH statement by the statement that follows the
// names. The names hold their queries between parentheses, so the first
// statement word at the top level is the one.
func withTag(toks []token, info *Info) (string, bool) {
	i := topWord(toks, 1, append([]string{"SELECT", "VALUES", "TABLE"}, dml...)...)
	if i < 0 {
		return "WITH", true
	}
	switch word := toks[i].text; word {
	case "SELECT", "VALUES", "TABLE":
		if word == "SELECT" && topWord(toks, i, "INTO") >= 0 {
			return "SELECT INTO", false
		}
		return "WITH", true
	default:
		return word, returnsRows(toks, i, info.Statements.Returning)
	}
}

// returnsRows says whether the changing statement that starts at toks[from]
// gives its rows back to the client.
func returnsRows(toks []token, from int, r Returning) bool {
	switch r {
	case ReturningRows:
		return topWord(toks, from, "RETURNING") >= 0
	case ReturningThenReturn:
		i := topWord(toks, from, "THEN")
		return i >= 0 && i+1 < len(toks) && toks[i+1].text == "RETURN"
	case ReturningOutput:
		for i := topWord(toks, from, "OUTPUT"); i >= 0; {
			next := topWord(toks, i+1, "OUTPUT")
			end := next
			if end < 0 {
				end = len(toks)
			}
			// An OUTPUT with an INTO puts the rows in a table.
			if topWord(toks[:end], i, "INTO") < 0 {
				return true
			}
			i = next
		}
	}
	return false
}

// execTag returns the command tag for the leading words of a statement that
// returns no rows. It is the longest run of the words that is a known tag.
func execTag(words []string) string {
	switch words[0] {
	case "CREATE":
		n := []string{"CREATE"}
		for _, w := range words[1:] {
			if !createIgnore[w] {
				n = append(n, w)
			}
		}
		words = n
	case "DROP":
		n := []string{"DROP"}
		for _, w := range words[1:] {
			if w != "PROCEDURAL" {
				n = append(n, w)
			}
		}
		words = n
	}
	for i := len(words); i > 0; i-- {
		if tag := strings.Join(words[:i], " "); execMap[tag] {
			return tag
		}
	}
	return words[0]
}
