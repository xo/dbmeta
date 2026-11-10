package postgres

import (
	"math"
	"strconv"
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
		Range:              rangeType,
		TriggerState:       triggerState,
		EventTrigger:       eventTrigger,
		Policies:           policies,
		SequenceValue:      sequenceValue,
		Role:               role,
		Membership:         membership,
		Grant:              grant,
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

// rangeType writes CREATE TYPE AS RANGE with the parts that are not the default.
// The multirange type is made with the range, so it is named here and not made.
func rangeType(quoted string, r dump.RangeType, _ func(string) string) string {
	parts := []string{"subtype = " + r.Subtype}
	for _, p := range [][2]string{{"subtype_opclass", r.OpClass}, {"collation", r.Collation},
		{"canonical", r.Canonical}, {"subtype_diff", r.SubtypeDiff}, {"multirange_type_name", r.Multirange}} {
		if p[1] != "" {
			parts = append(parts, p[0]+" = "+p[1])
		}
	}
	return "CREATE TYPE " + quoted + " AS RANGE (" + strings.Join(parts, ", ") + ")"
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

// triggerState switches a trigger off, or sets it for the replica role.
func triggerState(_, quotedTable, quotedTrigger, state string) string {
	// An event trigger has no table.
	if quotedTable == "" {
		switch state {
		case "disabled":
			return "ALTER EVENT TRIGGER " + quotedTrigger + " DISABLE"
		case "replica":
			return "ALTER EVENT TRIGGER " + quotedTrigger + " ENABLE REPLICA"
		case "always":
			return "ALTER EVENT TRIGGER " + quotedTrigger + " ENABLE ALWAYS"
		}
		return ""
	}
	switch state {
	case "disabled":
		return "ALTER TABLE " + quotedTable + " DISABLE TRIGGER " + quotedTrigger
	case "replica":
		return "ALTER TABLE " + quotedTable + " ENABLE REPLICA TRIGGER " + quotedTrigger
	case "always":
		return "ALTER TABLE " + quotedTable + " ENABLE ALWAYS TRIGGER " + quotedTrigger
	}
	return ""
}

// eventTrigger writes CREATE EVENT TRIGGER from the event, the tags and the function.
func eventTrigger(e dump.EventTriggerDef, q func(string) string, lit func(string) string) string {
	sql := "CREATE EVENT TRIGGER " + q(e.Name) + " ON " + e.Event
	if e.Tags != "" {
		tags := strings.Split(e.Tags, ", ")
		for i, tag := range tags {
			tags[i] = lit(tag)
		}
		sql += " WHEN TAG IN (" + strings.Join(tags, ", ") + ")"
	}
	return sql + " EXECUTE FUNCTION " + e.Function + "()"
}

// policies turns row level security on, and writes each policy of a table.
func policies(schema string, t dump.Table, _ map[string]bool, q func(string) string, _ func(string) string) []string {
	table := q(schema) + "." + q(t.Name)
	var out []string
	if t.RowSecurity {
		out = append(out, "ALTER TABLE "+table+" ENABLE ROW LEVEL SECURITY")
	}
	for _, p := range t.Policies {
		sql := "CREATE POLICY " + q(p.Name) + " ON " + table
		if !p.Permissive {
			sql += " AS RESTRICTIVE"
		}
		sql += " FOR " + strings.ToUpper(p.Command)
		if p.Roles != "" {
			roles := strings.Split(p.Roles, ",")
			for i, r := range roles {
				roles[i] = q(r)
			}
			sql += " TO " + strings.Join(roles, ", ")
		}
		if p.Using != "" {
			sql += " USING (" + p.Using + ")"
		}
		if p.WithCheck != "" {
			sql += " WITH CHECK (" + p.WithCheck + ")"
		}
		out = append(out, sql)
	}
	return out
}

// sequenceValue sets the next value of a sequence, after the last one that it gave.
func sequenceValue(quoted string, s dump.Sequence, lit func(string) string) string {
	return "SELECT pg_catalog.setval(" + lit(quoted) + ", " + s.Last + ", true)"
}

// role makes a role when it is not there, with the attributes that it had and with no
// password, because the catalog does not give one to a person who is no superuser.
func role(r dump.RoleDef, q func(string) string, lit func(string) string) string {
	attrs := []string{}
	if r.Superuser {
		attrs = append(attrs, "SUPERUSER")
	}
	if r.CreateDB {
		attrs = append(attrs, "CREATEDB")
	}
	if r.CreateRole {
		attrs = append(attrs, "CREATEROLE")
	}
	if r.Replication {
		attrs = append(attrs, "REPLICATION")
	}
	if r.BypassRLS {
		attrs = append(attrs, "BYPASSRLS")
	}
	if r.NoInherit {
		attrs = append(attrs, "NOINHERIT")
	}
	if r.CanLogin {
		attrs = append(attrs, "LOGIN")
	} else {
		attrs = append(attrs, "NOLOGIN")
	}
	if r.ConnLimit > 0 {
		attrs = append(attrs, "CONNECTION LIMIT "+strconv.FormatInt(r.ConnLimit, 10))
	}
	sql := "DO $$ BEGIN IF NOT EXISTS (SELECT FROM pg_catalog.pg_roles WHERE rolname = " + lit(r.Name) +
		") THEN CREATE ROLE " + q(r.Name) + " " + strings.Join(attrs, " ") + "; END IF; END $$"
	return sql
}

// membership makes a role a member of another.
func membership(role, member string, q func(string) string) string {
	return "GRANT " + q(role) + " TO " + q(member)
}

// grant writes the grants of one table, view or sequence. A grant with a column names
// the column, and the privileges of a grantee that it can pass on are given apart.
func grant(schema, object, kind, _ string, grants []dump.GrantDef, q func(string) string) []string {
	var target string
	switch kind {
	case "sequence":
		target = "SEQUENCE " + q(schema) + "." + q(object)
	case "schema":
		target = "SCHEMA " + q(schema)
	case "function", "aggregate":
		target = "FUNCTION " + q(schema) + "." + q(object) + "(" + grants[0].Arguments + ")"
	case "procedure":
		target = "PROCEDURE " + q(schema) + "." + q(object) + "(" + grants[0].Arguments + ")"
	case "type":
		target = "TYPE " + q(schema) + "." + q(object)
	default:
		target = "TABLE " + q(schema) + "." + q(object)
	}
	type who struct {
		grantee string
		option  bool
	}
	var order []who
	whole := map[who][]string{}
	columns := map[who]map[string][]string{}
	for _, gr := range grants {
		w := who{gr.Grantee, gr.Option}
		if _, ok := whole[w]; !ok && columns[w] == nil {
			order = append(order, w)
		}
		if gr.Column == "" {
			whole[w] = append(whole[w], gr.Privilege)
			continue
		}
		if columns[w] == nil {
			columns[w] = map[string][]string{}
		}
		columns[w][gr.Privilege] = append(columns[w][gr.Privilege], q(gr.Column))
	}
	var out []string
	for _, w := range order {
		to := "PUBLIC"
		if w.grantee != "" {
			to = q(w.grantee)
		}
		tail := " ON " + target + " TO " + to
		if w.option {
			tail += " WITH GRANT OPTION"
		}
		if privs := whole[w]; len(privs) > 0 {
			out = append(out, "GRANT "+strings.Join(privs, ", ")+tail)
		}
		if cols := columns[w]; len(cols) > 0 {
			var parts []string
			for _, priv := range []string{"SELECT", "INSERT", "UPDATE", "REFERENCES"} {
				if list := cols[priv]; len(list) > 0 {
					parts = append(parts, priv+" ("+strings.Join(list, ", ")+")")
				}
			}
			out = append(out, "GRANT "+strings.Join(parts, ", ")+tail)
		}
	}
	return out
}
