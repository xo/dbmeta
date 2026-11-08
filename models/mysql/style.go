package mysql

import (
	"regexp"
	"strings"

	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/dump"
)

// The statement style of the dump, for MySQL and MariaDB. See D274 and
// docs/DUMP.md.

func init() {
	dump.RegisterStyle(dbmeta.MySQL, dump.Style{
		Quote:         quoteIdentifier,
		Literal:       literal,
		DefaultSchema: "*",
		Column:        column,
		TableOptions:  tableOptions,
		Index:         index,
		Trigger:       trigger,
	})
}

// quoteIdentifier quotes a name with backticks.
func quoteIdentifier(name string) string {
	return dbmeta.QuoteIdentifier(name, "`", "`")
}

// literal writes a string literal. It escapes the backslash, as the default
// mode does, which a mode that turns the escape off reads as two backslashes.
// A script runs in a session that has the default mode.
func literal(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

// column writes a column in the order that MySQL reads: the type, the collation,
// a generation expression, NOT NULL, the default, AUTO_INCREMENT and the comment.
func column(c dump.Column, typeSQL string, q func(string) string, lit func(string) string) string {
	var b strings.Builder
	b.WriteString(q(c.Name) + " " + typeSQL)
	if c.Collation != "" {
		b.WriteString(" COLLATE " + c.Collation)
	}
	generated := c.Generated != "" && c.Default != nil
	if generated {
		kind := "VIRTUAL"
		if c.Generated == "s" {
			kind = "STORED"
		}
		b.WriteString(" GENERATED ALWAYS AS (" + *c.Default + ") " + kind)
	}
	if !c.Nullable {
		b.WriteString(" NOT NULL")
	}
	if !generated && c.Default != nil {
		if c.DefaultLiteral {
			b.WriteString(" DEFAULT " + lit(*c.Default))
		} else {
			b.WriteString(" DEFAULT " + *c.Default)
		}
	}
	if c.Identity != "" {
		b.WriteString(" AUTO_INCREMENT")
	}
	if c.Comment != "" {
		b.WriteString(" COMMENT " + lit(c.Comment))
	}
	return b.String()
}

// tableOptions writes the comment of the table after the closing bracket.
func tableOptions(t dump.Table, lit func(string) string) string {
	if t.Comment == "" {
		return ""
	}
	return " COMMENT=" + lit(t.Comment)
}

// index writes CREATE INDEX with the kind of index that MySQL has, which a plain
// index, a full text index and a spatial index are.
func index(table string, ix dump.Index, q func(string) string) string {
	cols := make([]string, len(ix.Columns))
	for i, c := range ix.Columns {
		cols[i] = q(c.Name)
		if c.Expression != "" {
			cols[i] = "(" + c.Expression + ")"
		}
		if c.Descending {
			cols[i] += " DESC"
		}
	}
	kind := ""
	switch strings.ToUpper(ix.Type) {
	case "FULLTEXT":
		kind = "FULLTEXT "
	case "SPATIAL":
		kind = "SPATIAL "
	default:
		if ix.Unique {
			kind = "UNIQUE "
		}
	}
	sql := "CREATE " + kind + "INDEX " + q(ix.Name) + " ON " + table + " (" + strings.Join(cols, ", ") + ")"
	if strings.EqualFold(ix.Type, "HASH") {
		sql += " USING HASH"
	}
	return sql
}

// triggerDefinition reads the definition that the model writes, which is the
// timing, the event and the body.
var triggerDefinition = regexp.MustCompile(`(?is)^\s*(BEFORE|AFTER)\s+(INSERT|UPDATE|DELETE)\s+(.*)$`)

// trigger writes CREATE TRIGGER from the timing, the event and the body.
func trigger(table string, tg dump.Trigger, q func(string) string) string {
	m := triggerDefinition.FindStringSubmatch(tg.Definition)
	if m == nil {
		return ""
	}
	return "CREATE TRIGGER " + q(tg.Name) + " " + strings.ToUpper(m[1]) + " " + strings.ToUpper(m[2]) +
		" ON " + table + " FOR EACH ROW " + strings.TrimSpace(m[3])
}
