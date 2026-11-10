package dump

import (
	"strings"
	"unicode"
)

// sqliteChecks finds the CHECK constraints in the text of a CREATE TABLE of SQLite,
// which keeps them nowhere else. It reads the text one token at a time, so a CHECK
// inside a quoted name, a string or a comment is no constraint, and it takes each
// expression up to the bracket that closes it. A constraint that a person named has
// the name. See D296.
func sqliteChecks(def string) []Constraint {
	var out []Constraint
	var prev, prev2 string
	i := 0
	for i < len(def) {
		c := def[i]
		switch {
		case c == '\'' || c == '"' || c == '`':
			j := i + 1
			for j < len(def) {
				if def[j] == c {
					if j+1 < len(def) && def[j+1] == c {
						j += 2
						continue
					}
					break
				}
				j++
			}
			prev2, prev = prev, def[i:min(j+1, len(def))]
			i = j + 1
		case c == '[':
			j := strings.IndexByte(def[i:], ']')
			if j < 0 {
				return out
			}
			prev2, prev = prev, def[i:i+j+1]
			i += j + 1
		case c == '-' && strings.HasPrefix(def[i:], "--"):
			j := strings.IndexByte(def[i:], '\n')
			if j < 0 {
				return out
			}
			i += j + 1
		case c == '/' && strings.HasPrefix(def[i:], "/*"):
			j := strings.Index(def[i:], "*/")
			if j < 0 {
				return out
			}
			i += j + 2
		case unicode.IsLetter(rune(c)) || c == '_':
			j := i
			for j < len(def) && (unicode.IsLetter(rune(def[j])) || unicode.IsDigit(rune(def[j])) || def[j] == '_' || def[j] == '$') {
				j++
			}
			word := def[i:j]
			if strings.EqualFold(word, "check") {
				k := j
				for k < len(def) && unicode.IsSpace(rune(def[k])) {
					k++
				}
				if k < len(def) && def[k] == '(' {
					end := closingBracket(def, k)
					if end < 0 {
						return out
					}
					con := Constraint{Kind: "check", Definition: "CHECK " + def[k:end+1]}
					if strings.EqualFold(prev2, "constraint") {
						con.Name = unquoteName(prev)
					}
					out = append(out, con)
					prev2, prev = "", ""
					i = end + 1
					continue
				}
			}
			prev2, prev = prev, word
			i = j
		default:
			if !unicode.IsSpace(rune(c)) {
				prev2, prev = "", ""
			}
			i++
		}
	}
	return out
}

// closingBracket returns the index of the bracket that closes the one at start, or -1.
func closingBracket(s string, start int) int {
	depth := 0
	for i := start; i < len(s); i++ {
		switch s[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return i
			}
		case '\'', '"', '`':
			q := s[i]
			j := i + 1
			for j < len(s) {
				if s[j] == q {
					if j+1 < len(s) && s[j+1] == q {
						j += 2
						continue
					}
					break
				}
				j++
			}
			i = j
		}
	}
	return -1
}

// unquoteName takes the quotes off a name.
func unquoteName(s string) string {
	if len(s) >= 2 {
		switch s[0] {
		case '"', '`', '\'':
			return strings.ReplaceAll(s[1:len(s)-1], string(s[0])+string(s[0]), string(s[0]))
		case '[':
			return s[1 : len(s)-1]
		}
	}
	return s
}
