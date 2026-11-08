package dump

import (
	"regexp"
	"strings"

	"github.com/xo/dbmeta"
)

// The defaults that every product writes the same way. dbmeta translates no SQL
// (D274), so a default that is not one of these is not written for another
// dialect, and the plan lists it as untranslated.

var (
	numberDefault  = regexp.MustCompile(`^[+-]?(\d+(\.\d*)?|\.\d+)([eE][+-]?\d+)?$`)
	stringDefault  = regexp.MustCompile(`(?s)^'((?:[^']|'')*)'(::[A-Za-z_][A-Za-z0-9_ ]*(\(\d+(,\d+)?\))?(\[\])?)?$`)
	nowDefault     = regexp.MustCompile(`(?i)^(current_timestamp|localtimestamp|now|sysdate|getdate)(\(\d*\))?$`)
	dateDefault    = regexp.MustCompile(`(?i)^(current_date|curdate\(\))(\(\))?$`)
	timeDefault    = regexp.MustCompile(`(?i)^(current_time|curtime\(\))(\(\d*\))?$`)
	boolDefault    = regexp.MustCompile(`(?i)^(true|false)$`)
	nullDefault    = regexp.MustCompile(`(?i)^null(::[A-Za-z_ ]+)?$`)
	createViewHead = regexp.MustCompile(`(?is)^\s*CREATE\s+(?:OR\s+REPLACE\s+)?(?:TEMP(?:ORARY)?\s+)?VIEW\s+(?:IF\s+NOT\s+EXISTS\s+)?(?:"[^"]+"|` + "`[^`]+`" + `|\[[^\]]+\]|[\w.]+)(?:\s*\.\s*(?:"[^"]+"|` + "`[^`]+`" + `|\[[^\]]+\]|[\w]+))?\s+AS\s+`)
)

// portableDefault writes the default of a column for the target, when it is one
// that every product writes the same way: a number, a string, a boolean, NULL, or
// the current time or date. The second result is false for any other default.
func portableDefault(source, target dbmeta.Dialect, c Column) (string, bool) {
	text, ok := portableDefaultText(source, target, c)
	if !ok {
		return text, ok
	}
	if style, err := styleOf(target); err == nil && style.Boolean != nil {
		switch text {
		case "TRUE":
			return style.Boolean(true), true
		case "FALSE":
			return style.Boolean(false), true
		}
	}
	return text, ok
}

// portableDefaultText is portableDefault before the words of a boolean are chosen
// for the target.
func portableDefaultText(source, target dbmeta.Dialect, c Column) (string, bool) {
	if c.Default == nil {
		return "", false
	}
	style, err := styleOf(target)
	if err != nil {
		return "", false
	}
	text := strings.TrimSpace(*c.Default)
	if c.DefaultLiteral {
		return literalDefault(style, c, text)
	}
	// An expression that is one value in brackets is that value.
	for strings.HasPrefix(text, "(") && strings.HasSuffix(text, ")") && balanced(text[1:len(text)-1]) {
		text = strings.TrimSpace(text[1 : len(text)-1])
	}
	switch {
	case nullDefault.MatchString(text):
		return "NULL", true
	case numberDefault.MatchString(text):
		// A boolean that a product writes as 0 and 1 is FALSE and TRUE elsewhere.
		if c.Type.Kind == Boolean {
			return booleanWord(text)
		}
		return text, true
	case boolDefault.MatchString(text):
		if c.Type.Kind == Integer {
			return map[string]string{"true": "1", "false": "0"}[strings.ToLower(text)], true
		}
		return strings.ToUpper(text), true
	case nowDefault.MatchString(text):
		return "CURRENT_TIMESTAMP", true
	case dateDefault.MatchString(text):
		return wrapDefault(target, "CURRENT_DATE"), true
	case timeDefault.MatchString(text):
		return wrapDefault(target, "CURRENT_TIME"), true
	}
	if m := stringDefault.FindStringSubmatch(text); m != nil {
		return style.Literal(strings.ReplaceAll(m[1], "''", "'")), true
	}
	_ = source
	return "", false
}

// wrapDefault puts a default in brackets for the products that read a function
// default as an expression, which MySQL does for a date and a time.
func wrapDefault(target dbmeta.Dialect, text string) string {
	if target == dbmeta.MySQL {
		return "(" + text + ")"
	}
	return text
}

// literalDefault writes a default that the source gave as a value with no quotes,
// as MySQL does for a string.
func literalDefault(style Style, c Column, value string) (string, bool) {
	switch c.Type.Kind {
	case Integer, Float, Decimal, BigInteger:
		if numberDefault.MatchString(value) {
			return value, true
		}
		return "", false
	case Boolean:
		switch strings.ToLower(value) {
		case "1", "true", "b'1'":
			return "TRUE", true
		case "0", "false", "b'0'":
			return "FALSE", true
		}
		return "", false
	case String, Date, TimeOfDay, LocalTimestamp, UUID, Timestamp, TimeOfDayOffset:
		return style.Literal(value), true
	}
	return "", false
}

// balanced reports whether the brackets in s close in order, so that (a) + (b) is
// not read as one value in brackets.
func balanced(s string) bool {
	depth := 0
	inQuote := false
	for _, r := range s {
		switch {
		case r == '\'':
			inQuote = !inQuote
		case inQuote:
		case r == '(':
			depth++
		case r == ')':
			depth--
			if depth < 0 {
				return false
			}
		}
	}
	return depth == 0 && !inQuote
}

// viewQuery returns the query of a view for another dialect. A product that keeps
// the whole CREATE VIEW has the head taken off, because the target writes its own.
func viewQuery(definition string) string {
	def := strings.TrimSpace(definition)
	if loc := createViewHead.FindStringIndex(def); loc != nil {
		def = def[loc[1]:]
	}
	return strings.TrimRight(def, "; \n\t")
}

// foreignKeyActions reads ON DELETE and ON UPDATE from the text of a foreign
// key, for a source that keeps the whole definition and no separate rules.
var (
	onDelete = regexp.MustCompile(`(?i)\bON\s+DELETE\s+(NO\s+ACTION|RESTRICT|CASCADE|SET\s+NULL|SET\s+DEFAULT)`)
	onUpdate = regexp.MustCompile(`(?i)\bON\s+UPDATE\s+(NO\s+ACTION|RESTRICT|CASCADE|SET\s+NULL|SET\s+DEFAULT)`)
)

// foreignKeyActions returns the rules of a foreign key, from the fields when the
// source gave them and from the definition when it did not.
func foreignKeyActions(c Constraint) (del, upd string) {
	del, upd = c.OnDelete, c.OnUpdate
	if del == "" {
		if m := onDelete.FindStringSubmatch(c.Definition); m != nil {
			del = strings.ToUpper(strings.Join(strings.Fields(m[1]), " "))
		}
	}
	if upd == "" {
		if m := onUpdate.FindStringSubmatch(c.Definition); m != nil {
			upd = strings.ToUpper(strings.Join(strings.Fields(m[1]), " "))
		}
	}
	return del, upd
}

// booleanWord reads 1 and 0 as TRUE and FALSE.
func booleanWord(text string) (string, bool) {
	switch strings.TrimLeft(text, "+") {
	case "1":
		return "TRUE", true
	case "0":
		return "FALSE", true
	}
	return "", false
}

// plainColumn matches a name that is a column and not an expression.
var plainColumn = regexp.MustCompile(`^(?:"((?:[^"]|"")+)"|` + "`" + `((?:[^` + "`" + `]|` + "``" + `)+)` + "`" + `|([A-Za-z_][A-Za-z0-9_$]*))(?:\s+(?i:asc|desc))?$`)

// definitionColumns reads the columns of an index from the statement that made
// it, for a source that keeps the statement and no columns, as DuckDB does. It
// reads a list of plain column names and nothing else, and gives no columns for
// a list that has an expression, so that dbmeta does not parse SQL.
func definitionColumns(definition string) []IndexColumn {
	open := strings.Index(definition, "(")
	closing := strings.LastIndex(definition, ")")
	if open < 0 || closing < open {
		return nil
	}
	var cols []IndexColumn
	for part := range strings.SplitSeq(definition[open+1:closing], ",") {
		part = strings.TrimSpace(part)
		m := plainColumn.FindStringSubmatch(part)
		if m == nil {
			return nil
		}
		name := m[1]
		switch {
		case m[2] != "":
			name = m[2]
		case m[3] != "":
			name = m[3]
		}
		cols = append(cols, IndexColumn{Name: name, Descending: strings.HasSuffix(strings.ToLower(part), " desc")})
	}
	return cols
}
