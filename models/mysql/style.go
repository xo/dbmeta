package mysql

import (
	"regexp"
	"strconv"
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
		// A dump of several databases makes each one.
		CreateSchema: func(quoted string) string { return "CREATE DATABASE " + quoted },
		// A view, a routine and a trigger keep names that mean the database they
		// are in, so a script of several databases chooses it first.
		UseSchema:     func(quoted string) string { return "USE " + quoted },
		Column:        column,
		TableOptions:  tableOptions,
		PartitionBy:   partitionBy,
		Index:         index,
		KeyType:       keyType,
		Binary:        func(hex string) string { return "X'" + hex + "'" },
		MaxParameters: 65535,
		Trigger:       trigger,
		// A view can call a function, and a function can read a view.
		RoutinesAfterTables: true,
		// A routine keeps the mode that it was made under.
		SQLMode:      func(mode string) string { return "SET SESSION sql_mode = " + literal(mode) },
		ResetSQLMode: "SET SESSION sql_mode = DEFAULT",
		// A timestamp is read and written in the zone of the session, so both are UTC.
		Session: []string{"SET time_zone = '+00:00'"},
		// The driver gives the clock of that session in the zone of its own DSN.
		SessionClock: true,
		Preamble:     []string{"SET time_zone = '+00:00'"},
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
	// The collation of a binary type is the word binary, which is the type and not
	// a collation that a column can name.
	if c.Collation != "" && !strings.EqualFold(c.Collation, "binary") {
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
	} else if strings.HasPrefix(strings.ToLower(typeSQL), "timestamp") {
		// A timestamp that does not say NULL is NOT NULL on a server that has
		// explicit_defaults_for_timestamp off, which MariaDB before 10.10 has.
		b.WriteString(" NULL")
	}
	// A default of NULL is no default, and MariaDB reports it for a nullable column
	// that has none. A text column cannot take it as an expression in brackets.
	nullDefault := c.Default != nil && c.Nullable && !c.DefaultLiteral && strings.EqualFold(strings.TrimSpace(*c.Default), "NULL")
	if !generated && c.Default != nil && !nullDefault {
		text := *c.Default
		if c.DefaultLiteral {
			text = lit(text)
		}
		// A text, a blob and a JSON column take a default only as an expression,
		// which is in brackets.
		if lower := strings.ToLower(typeSQL); strings.Contains(lower, "text") || strings.Contains(lower, "blob") || lower == "json" {
			if !strings.HasPrefix(text, "(") {
				text = "(" + text + ")"
			}
		}
		b.WriteString(" DEFAULT " + text)
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

// partitionBy writes the partitions of a table after its options. A range or a
// list partition has its bound, and a hash or a key partition is counted.
func partitionBy(p dump.Partitioning, q func(string) string, _ func(string) string) string {
	method := strings.ToUpper(p.Strategy)
	sql := " PARTITION BY " + method + " (" + p.Expression + ")"
	var bound string
	switch {
	case strings.Contains(method, "RANGE"):
		bound = "LESS THAN"
	case strings.Contains(method, "LIST"):
		bound = "IN"
	default:
		if len(p.Parts) > 0 {
			sql += " PARTITIONS " + strconv.Itoa(len(p.Parts))
		}
		return sql
	}
	parts := make([]string, len(p.Parts))
	for i, part := range p.Parts {
		parts[i] = "PARTITION " + q(part.Name) + " VALUES " + bound + " (" + part.Bound + ")"
	}
	return sql + " (" + strings.Join(parts, ", ") + ")"
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

// keyType gives a text or a binary value of no length a length, because MySQL
// cannot key a column of the types that have none. The length is the longest that
// every storage engine keys.
func keyType(t dump.Type) (dump.Rendering, bool) {
	const length = "255"
	switch {
	case t.Kind == dump.String && len(t.Values) == 0 && t.Length == 0 && !t.Fixed:
		return dump.Rendering{SQL: "varchar(" + length + ")", Level: dump.Lossy,
			Note: "MySQL cannot key a text with no length, so the column is a varchar of " + length}, true
	case t.Kind == dump.Binary && t.Length == 0:
		return dump.Rendering{SQL: "varbinary(" + length + ")", Level: dump.Lossy,
			Note: "MySQL cannot key a blob with no length, so the column is a varbinary of " + length}, true
	}
	return dump.Rendering{}, false
}
