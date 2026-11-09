package postgres

import (
	"math"
	"strings"

	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/dump"
)

// The statement style of the dump. See D274 and docs/DUMP.md.

func init() {
	dump.RegisterStyle(dbmeta.PostgreSQL, dump.Style{
		Quote:           quoteIdentifier,
		Literal:         literal,
		DefaultSchema:   "public",
		QualifyAll:      true,
		ExistingSchemas: []string{"public"},
		CreateSchema:    func(quoted string) string { return "CREATE SCHEMA " + quoted },
		// A function can name a table that is not there yet, as pg_dump allows.
		Preamble:           []string{"SET check_function_bodies = false"},
		Column:             column,
		Enum:               enum,
		Sequence:           sequence,
		OwnedBy:            func(seq, owner string) string { return "ALTER SEQUENCE " + seq + " OWNED BY " + owner },
		Comment:            comment,
		View:               view,
		MaterializedView:   materializedView,
		Extension:          extension,
		Domain:             domain,
		Composite:          composite,
		PartitionBy:        partitionBy,
		PartitionOf:        partitionOf,
		IdentityValue:      identityValue,
		Binary:             func(hex string) string { return "decode('" + hex + "', 'hex')" },
		OverridingIdentity: "OVERRIDING SYSTEM VALUE",
		ZoneInTimestamps:   true,
		MaxParameters:      65535,
		ReferenceTable:     referenceTable,
		FloatSpecial: func(v float64) (string, bool) {
			switch {
			case math.IsNaN(v):
				return "'NaN'::double precision", true
			case math.IsInf(v, 1):
				return "'Infinity'::double precision", true
			}
			return "'-Infinity'::double precision", true
		},
	})
}

// quoteIdentifier quotes a name with double quotes.
func quoteIdentifier(name string) string {
	return dbmeta.QuoteIdentifier(name, `"`, `"`)
}

// literal writes a string literal. A backslash is plain when
// standard_conforming_strings is on, which is the default since release 9.1, and
// a literal with a backslash is written in the E form, which reads it the same
// whatever the setting is.
func literal(s string) string {
	s = strings.ReplaceAll(s, "'", "''")
	if strings.Contains(s, `\`) {
		return "E'" + strings.ReplaceAll(s, `\`, `\\`) + "'"
	}
	return "'" + s + "'"
}

// column writes a column with its identity or its generation expression.
func column(c dump.Column, typeSQL string, q func(string) string, _ func(string) string) string {
	var b strings.Builder
	b.WriteString(q(c.Name) + " " + typeSQL)
	if c.Collation != "" && c.Collation != "default" {
		b.WriteString(" COLLATE " + q(c.Collation))
	}
	switch {
	case c.Generated != "" && c.Default != nil:
		b.WriteString(" GENERATED ALWAYS AS (" + *c.Default + ") STORED")
	case c.Identity != "" && serialType(typeSQL, c.Identity) != "":
		// An identity from another product is a serial column, which every release
		// has, and release 9.6 has no identity column.
		return q(c.Name) + " " + serialType(typeSQL, c.Identity)
	case c.Identity != "":
		b.WriteString(" GENERATED " + identityKind(c.Identity) + " AS IDENTITY")
	case c.Default != nil:
		b.WriteString(" DEFAULT " + *c.Default)
	}
	if !c.Nullable {
		b.WriteString(" NOT NULL")
	}
	return b.String()
}

// identityKind reads the identity that the catalog reports, which is a letter or
// the words, and returns the words of the clause.
func identityKind(identity string) string {
	switch strings.ToLower(identity) {
	case "a", "always":
		return "ALWAYS"
	}
	return "BY DEFAULT"
}

// identityValue sets the next value of a serial or identity column from the rows
// that were loaded, as pg_dump does with setval. The sequence is the one that
// pg_get_serial_sequence finds for the column.
func identityValue(quotedTable, quotedColumn, column string, lit func(string) string) string {
	return "SELECT pg_catalog.setval(pg_catalog.pg_get_serial_sequence(" + lit(quotedTable) + ", " + lit(column) +
		"), COALESCE((SELECT MAX(" + quotedColumn + ") FROM " + quotedTable + "), 0) + 1, false)"
}

// enum writes CREATE TYPE ... AS ENUM.
func enum(quoted string, values []string, lit func(string) string) string {
	quotedValues := make([]string, len(values))
	for i, v := range values {
		quotedValues[i] = lit(v)
	}
	return "CREATE TYPE " + quoted + " AS ENUM (" + strings.Join(quotedValues, ", ") + ")"
}

// sequence writes CREATE SEQUENCE with the bounds that the catalog gives.
func sequence(quoted string, s dump.Sequence) string {
	var b strings.Builder
	b.WriteString("CREATE SEQUENCE " + quoted)
	// Release 9.6 has no AS, and every sequence of it is a bigint.
	if s.DataType != "" && !strings.EqualFold(s.DataType, "bigint") {
		b.WriteString(" AS " + s.DataType)
	}
	for _, f := range []struct{ word, value string }{
		{"INCREMENT BY", s.Increment}, {"MINVALUE", s.Minimum}, {"MAXVALUE", s.Maximum}, {"START WITH", s.Start},
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

// comment writes COMMENT ON.
func comment(target, quotedName, text string, lit func(string) string) string {
	return "COMMENT ON " + target + " " + quotedName + " IS " + lit(text)
}

// referenceTable puts the schema in front of the table that a foreign key
// points at. pg_get_constraintdef leaves it out for a table on the search path
// of the session that read it, and the script runs in another session.
func referenceTable(def string, ref dump.Reference, q func(string) string) string {
	if ref.Schema == "" {
		return def
	}
	const word = "REFERENCES "
	i := strings.Index(def, word)
	if i < 0 {
		return def
	}
	start := i + len(word)
	// The name of the table is quoted or runs to the bracket. A name that has a
	// dot is already qualified.
	end := start
	quoted := false
	hasDot := false
	for ; end < len(def); end++ {
		c := def[end]
		if c == '"' {
			quoted = !quoted
			continue
		}
		if !quoted && (c == '(' || c == ' ') {
			break
		}
		if !quoted && c == '.' {
			hasDot = true
		}
	}
	if hasDot {
		return def
	}
	return def[:start] + q(ref.Schema) + "." + def[start:]
}

// view writes CREATE VIEW from the query that the catalog keeps, and the check
// option that the view has.
func view(quoted string, v dump.View) string {
	sql := "CREATE VIEW " + quoted + " AS " + strings.TrimRight(strings.TrimSpace(v.Definition), "; \n\t")
	switch strings.ToLower(v.CheckOption) {
	case "cascaded":
		sql += " WITH CASCADED CHECK OPTION"
	case "local":
		sql += " WITH LOCAL CHECK OPTION"
	}
	return sql
}

// extension installs an extension in its schema, if it is not there. It names no
// version, as pg_dump does, so the server installs the one that it has.
func extension(quotedName, quotedSchema string) string {
	return "CREATE EXTENSION IF NOT EXISTS " + quotedName + " WITH SCHEMA " + quotedSchema
}

// domain writes CREATE DOMAIN with its default, NOT NULL and checks. The checks
// are the text of pg_get_constraintdef.
func domain(quoted string, d dump.Domain, q func(string) string, _ func(string) string) string {
	sql := "CREATE DOMAIN " + quoted + " AS " + d.Type
	if d.Collation != "" {
		sql += " COLLATE " + q(d.Collation)
	}
	if d.Default != nil {
		sql += " DEFAULT " + *d.Default
	}
	if d.NotNull {
		sql += " NOT NULL"
	}
	if d.Constraints != "" {
		sql += " " + d.Constraints
	}
	return sql
}

// composite writes CREATE TYPE AS with the attributes in order.
func composite(quoted string, c dump.Composite, q func(string) string) string {
	attrs := make([]string, len(c.Columns))
	for i, col := range c.Columns {
		attrs[i] = q(col.Name) + " " + col.Type.Raw
		if col.Collation != "" && col.Collation != "default" {
			attrs[i] += " COLLATE " + q(col.Collation)
		}
	}
	return "CREATE TYPE " + quoted + " AS (" + strings.Join(attrs, ", ") + ")"
}

// materializedView writes CREATE MATERIALIZED VIEW. A script makes it after the
// rows of its tables are loaded, so it is made with its rows.
func materializedView(quoted string, v dump.View) string {
	return "CREATE MATERIALIZED VIEW " + quoted + " AS " + strings.TrimRight(strings.TrimSpace(v.Definition), "; \n\t") + " WITH DATA"
}

// partitionBy writes the clause that splits a table into partitions.
func partitionBy(p dump.Partitioning, _ func(string) string, _ func(string) string) string {
	return " PARTITION BY " + strings.ToUpper(p.Strategy) + " (" + p.Expression + ")"
}

// partitionOf writes the statement that makes a partition. It takes its columns
// and its keys from its parent.
func partitionOf(quoted, parent, bound string) string {
	return "CREATE TABLE " + quoted + " PARTITION OF " + parent + " " + bound
}

// serialType returns the serial type that stands for an integer type with
// auto numbering from another product, or the empty string when the identity is
// the one of PostgreSQL, which has the letters a and d, or the type is not an
// integer.
func serialType(typeSQL, identity string) string {
	if identity != "by default" {
		return ""
	}
	switch typeSQL {
	case "smallint":
		return "smallserial"
	case "integer":
		return "serial"
	case "bigint":
		return "bigserial"
	}
	return ""
}
