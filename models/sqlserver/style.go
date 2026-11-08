package sqlserver

import (
	"regexp"
	"strings"

	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/dump"
)

// The statement style of the dump. See D274, D280 and docs/DUMP.md.

func init() {
	dump.RegisterStyle(dbmeta.SQLServer, dump.Style{
		Quote:             quoteIdentifier,
		Literal:           literal,
		DefaultSchema:     "dbo",
		ExistingSchemas:   []string{"dbo", "guest", "sys", "INFORMATION_SCHEMA"},
		CreateSchema:      func(quoted string) string { return "CREATE SCHEMA " + quoted },
		Column:            column,
		Sequence:          sequence,
		Index:             index,
		View:              view,
		KeyType:           keyType,
		PrimaryKeyNotNull: true,
		TypedNullBinary:   true,
		NoRestrict:        true,
		Binary:            func(hex string) string { return "0x" + hex },
		Boolean: func(v bool) string {
			if v {
				return "1"
			}
			return "0"
		},
		IdentityInsert: func(quotedTable string, on bool) string {
			if on {
				return "SET IDENTITY_INSERT " + quotedTable + " ON"
			}
			return "SET IDENTITY_INSERT " + quotedTable + " OFF"
		},
		ZoneInTimestamps: true,
		// A table value constructor takes at most 1000 rows, and a statement takes
		// at most 2100 parameters.
		MaxRows:       1000,
		MaxParameters: 2100,
	})
}

// quoteIdentifier quotes a name with brackets.
func quoteIdentifier(name string) string {
	return dbmeta.QuoteIdentifier(name, "[", "]")
}

// literal writes a string literal as a Unicode string, so that no character is
// lost to the code page of the database.
func literal(s string) string {
	return "N'" + strings.ReplaceAll(s, "'", "''") + "'"
}

// column writes a column in the order that SQL Server reads: the type, the
// collation, IDENTITY, the nullability and the default. A computed column has an
// expression and no type.
func column(c dump.Column, typeSQL string, q func(string) string, lit func(string) string) string {
	if c.Generated != "" && c.Default != nil {
		text := q(c.Name) + " AS (" + *c.Default + ")"
		if c.Generated == "stored" || c.Generated == "s" {
			text += " PERSISTED"
		}
		return text
	}
	var b strings.Builder
	b.WriteString(q(c.Name) + " " + typeSQL)
	if c.Collation != "" {
		b.WriteString(" COLLATE " + c.Collation)
	}
	if c.Identity != "" {
		b.WriteString(" IDENTITY(1,1)")
	}
	if c.Nullable {
		b.WriteString(" NULL")
	} else {
		b.WriteString(" NOT NULL")
	}
	if c.Default != nil {
		text := *c.Default
		if c.DefaultLiteral {
			text = lit(text)
		}
		b.WriteString(" DEFAULT " + text)
	}
	return b.String()
}

// sequence writes CREATE SEQUENCE with the bounds that the catalog gives.
func sequence(quoted string, s dump.Sequence) string {
	var b strings.Builder
	b.WriteString("CREATE SEQUENCE " + quoted)
	if s.DataType != "" {
		b.WriteString(" AS " + s.DataType)
	}
	for _, f := range []struct{ word, value string }{
		{"START WITH", s.Start}, {"INCREMENT BY", s.Increment}, {"MINVALUE", s.Minimum}, {"MAXVALUE", s.Maximum},
	} {
		if f.value != "" {
			b.WriteString(" " + f.word + " " + f.value)
		}
	}
	if s.Cycles {
		b.WriteString(" CYCLE")
	} else {
		b.WriteString(" NO CYCLE")
	}
	return b.String()
}

// index writes CREATE INDEX with the columns that are keys, the columns that are
// only included, and the filter.
func index(table string, ix dump.Index, q func(string) string) string {
	var keys, include []string
	for _, c := range ix.Columns {
		text := q(c.Name)
		if c.Descending {
			text += " DESC"
		}
		if c.Include {
			include = append(include, q(c.Name))
			continue
		}
		keys = append(keys, text)
	}
	sql := "CREATE "
	if ix.Unique {
		sql += "UNIQUE "
	}
	sql += "INDEX " + q(ix.Name) + " ON " + table + " (" + strings.Join(keys, ", ") + ")"
	if len(include) > 0 {
		sql += " INCLUDE (" + strings.Join(include, ", ") + ")"
	}
	if ix.Predicate != "" {
		sql += " WHERE " + ix.Predicate
	}
	return sql
}

// keyType gives a text or a binary value of no length a length, because SQL Server
// cannot key a column of type max. The length is the longest that a nonclustered
// key holds on every release, which is 900 bytes.
func keyType(t dump.Type) (dump.Rendering, bool) {
	switch {
	case t.Kind == dump.String && len(t.Values) == 0 && (t.Length == 0 || t.Length > 450):
		return dump.Rendering{SQL: "nvarchar(450)", Level: dump.Lossy,
			Note: "SQL Server cannot key a text longer than 450 characters, so the column is an nvarchar of 450"}, true
	case t.Kind == dump.Binary && (t.Length == 0 || t.Length > 900):
		return dump.Rendering{SQL: "varbinary(900)", Level: dump.Lossy,
			Note: "SQL Server cannot key a binary value longer than 900 bytes, so the column is a varbinary of 900"}, true
	}
	return dump.Rendering{}, false
}

// viewHead is what SQL Server keeps of a view before its query: CREATE VIEW, the
// name, a list of column names and the options. The catalog keeps the whole
// statement as it was written, with the check option inside it.
var viewHead = regexp.MustCompile(`(?is)^\s*CREATE\s+VIEW\s+(?:\[[^\]]+\]|\w+)(?:\s*\.\s*(?:\[[^\]]+\]|\w+))*` +
	`\s*(\([^)]*\))?\s*(?:WITH\s+[A-Za-z_, ]+?\s+)?AS\s+`)

// view writes CREATE VIEW with the name of the target and the rest of the
// statement that the source kept.
func view(quoted string, v dump.View) string {
	def := strings.TrimRight(strings.TrimSpace(v.Definition), "; \n\t")
	loc := viewHead.FindStringSubmatchIndex(def)
	if loc == nil {
		return "CREATE VIEW " + quoted + " AS " + def
	}
	columns := ""
	if loc[2] >= 0 {
		columns = " " + def[loc[2]:loc[3]]
	}
	return "CREATE VIEW " + quoted + columns + " AS " + def[loc[1]:]
}
