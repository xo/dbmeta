package duckdb

import (
	"reflect"
	"slices"
	"strconv"
	"strings"

	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/dump"
)

// The statement style of the dump. See D274 and docs/DUMP.md.

func init() {
	dump.RegisterStyle(dbmeta.DuckDB, dump.Style{
		Quote:                quoteIdentifier,
		Literal:              literal,
		DefaultSchema:        "main",
		ExistingSchemas:      []string{"main"},
		CreateSchema:         func(quoted string) string { return "CREATE SCHEMA " + quoted },
		InlineConstraints:    true,
		AnonymousConstraints: true,
		// DuckDB refuses ON DELETE and ON UPDATE other than the default.
		NoForeignKeyActions: true,
		Binary:              func(hex string) string { return "from_hex('" + hex + "')" },
		ZoneInTimestamps:    true,
		Sequence:            sequence,
		Comment:             comment,
		Column:              column,
		View:                definitionStatement,
		// A view can read a macro, and a macro can read a table or a view.
		RoutinesAfterTables: true,
		Enum:                enum,
		SequenceValue:       sequenceValue,
		Domain:              aliasType,
		ValueLiteral:        nestedLiteral,
		RowPerSelfReference: true,
		SameSchemaKeys:      true,
	})
}

// quoteIdentifier quotes a name with double quotes.
func quoteIdentifier(name string) string {
	return dbmeta.QuoteIdentifier(name, `"`, `"`)
}

// literal writes a string literal. DuckDB has no backslash escape.
func literal(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

// definitionStatement returns the statement of the source, because DuckDB keeps
// the whole CREATE VIEW and not only the query.
func definitionStatement(_ string, v dump.View) string {
	return strings.TrimRight(strings.TrimSpace(v.Definition), "; \n\t")
}

// sequence writes CREATE SEQUENCE with the bounds that the catalog gives.
func sequence(quoted string, s dump.Sequence) string {
	var b strings.Builder
	b.WriteString("CREATE SEQUENCE " + quoted)
	for _, f := range []struct{ word, value string }{
		{"INCREMENT BY", s.Increment}, {"MINVALUE", s.Minimum}, {"MAXVALUE", s.Maximum}, {"START WITH", s.Start},
	} {
		if f.value != "" {
			b.WriteString(" " + f.word + " " + f.value)
		}
	}
	if s.Cycles {
		b.WriteString(" CYCLE")
	}
	return b.String()
}

// comment writes COMMENT ON.
func comment(target, quotedName, text string, lit func(string) string) string {
	// A type that a person made from another type is a domain to the dump, and a type
	// to DuckDB.
	if target == "DOMAIN" {
		target = "TYPE"
	}
	return "COMMENT ON " + target + " " + quotedName + " IS " + lit(text)
}

// enum writes CREATE TYPE AS ENUM.
func enum(quoted string, values []string, lit func(string) string) string {
	quotedValues := make([]string, len(values))
	for i, v := range values {
		quotedValues[i] = lit(v)
	}
	return "CREATE TYPE " + quoted + " AS ENUM (" + strings.Join(quotedValues, ", ") + ")"
}

// nestedLiteral writes the literal of a nested value that the driver gives as a
// Go value: a list, an array, a struct as a map with text keys, a map and a
// union. This module cannot import the driver, so a map and a union are known by
// the shape that the driver gives them: the union is a struct with a Tag and a
// Value, and the map has any as its key. The literal is cast to the type of the
// column, so that a fixed array and a union come back as they were.
func nestedLiteral(t dump.Type, v any) (string, bool) {
	if v == nil {
		return "", false
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Slice, reflect.Map:
	case reflect.Struct:
		if rv.FieldByName("Tag").IsValid() || rv.FieldByName("keys").IsValid() || isInterval(rv) {
			break
		}
		return "", false
	default:
		return "", false
	}
	if rv.Kind() == reflect.Slice && rv.Type().Elem().Kind() == reflect.Uint8 {
		return "", false
	}
	text, ok := valueText(rv)
	if !ok {
		return "", false
	}
	if t.Raw != "" {
		text = "CAST(" + text + " AS " + t.Raw + ")"
	}
	return text, true
}

// isInterval says that a value is the interval of the driver, which keeps the months, the
// days and the microseconds apart.
func isInterval(rv reflect.Value) bool {
	return rv.Kind() == reflect.Struct && rv.NumField() == 3 &&
		rv.FieldByName("Days").IsValid() && rv.FieldByName("Months").IsValid() && rv.FieldByName("Micros").IsValid()
}

// intervalText writes an interval with its three parts, so that none of them changes.
func intervalText(rv reflect.Value) string {
	return "INTERVAL '" + strconv.FormatInt(rv.FieldByName("Months").Int(), 10) + " months " +
		strconv.FormatInt(rv.FieldByName("Days").Int(), 10) + " days " +
		strconv.FormatInt(rv.FieldByName("Micros").Int(), 10) + " microseconds'"
}

// valueText writes one value, and the values inside it.
func valueText(rv reflect.Value) (string, bool) {
	for rv.Kind() == reflect.Interface || rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return "NULL", true
		}
		rv = rv.Elem()
	}
	switch rv.Kind() {
	case reflect.Invalid:
		return "NULL", true
	case reflect.String:
		return literal(rv.String()), true
	case reflect.Bool:
		if rv.Bool() {
			return "TRUE", true
		}
		return "FALSE", true
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return strconv.FormatInt(rv.Int(), 10), true
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return strconv.FormatUint(rv.Uint(), 10), true
	case reflect.Float32, reflect.Float64:
		return strconv.FormatFloat(rv.Float(), 'g', -1, 64), true
	case reflect.Slice, reflect.Array:
		parts := make([]string, rv.Len())
		for i := range parts {
			text, ok := valueText(rv.Index(i))
			if !ok {
				return "", false
			}
			parts[i] = text
		}
		return "[" + strings.Join(parts, ", ") + "]", true
	case reflect.Map:
		type entry struct{ key, value string }
		var entries []entry
		for _, k := range rv.MapKeys() {
			kt, ok := valueText(k)
			if !ok {
				return "", false
			}
			vt, ok := valueText(rv.MapIndex(k))
			if !ok {
				return "", false
			}
			entries = append(entries, entry{kt, vt})
		}
		slices.SortFunc(entries, func(a, b entry) int { return strings.Compare(a.key, b.key) })
		parts := make([]string, len(entries))
		for i, e := range entries {
			parts[i] = e.key + ": " + e.value
		}
		// A map with text keys is a struct to the driver, and a map of any is a map.
		if rv.Type().Key().Kind() == reflect.String {
			return "{" + strings.Join(parts, ", ") + "}", true
		}
		return "MAP {" + strings.Join(parts, ", ") + "}", true
	case reflect.Struct:
		// An ordered map keeps its keys and its values in two lists.
		if keys, values := rv.FieldByName("keys"), rv.FieldByName("values"); keys.IsValid() && values.IsValid() && keys.Len() == values.Len() {
			parts := make([]string, keys.Len())
			for i := range parts {
				kt, ok := valueText(keys.Index(i))
				if !ok {
					return "", false
				}
				vt, ok := valueText(values.Index(i))
				if !ok {
					return "", false
				}
				parts[i] = kt + ": " + vt
			}
			return "MAP {" + strings.Join(parts, ", ") + "}", true
		}
		if isInterval(rv) {
			return intervalText(rv), true
		}
		tag := rv.FieldByName("Tag")
		value := rv.FieldByName("Value")
		if !tag.IsValid() || !value.IsValid() {
			return "", false
		}
		text, ok := valueText(value)
		if !ok {
			return "", false
		}
		return "union_value(" + quoteIdentifier(tag.String()) + " := " + text + ")", true
	}
	return "", false
}

// aliasType writes CREATE TYPE AS for a type that a person made from another type.
func aliasType(quoted string, d dump.Domain, _ func(string) string, _ func(string) string) string {
	return "CREATE TYPE " + quoted + " AS " + d.Type
}

// sequenceValue draws the values that the sequence gave, because DuckDB has no statement
// that sets a sequence. It gives up on a sequence that gave more than a script can draw.
func sequenceValue(quoted string, s dump.Sequence, lit func(string) string) string {
	last, err1 := strconv.ParseInt(s.Last, 10, 64)
	start, err2 := strconv.ParseInt(s.Start, 10, 64)
	step, err3 := strconv.ParseInt(s.Increment, 10, 64)
	if err1 != nil || err2 != nil || err3 != nil || step == 0 || (last-start)%step != 0 {
		return ""
	}
	n := (last-start)/step + 1
	if n < 1 || n > 100000 {
		return ""
	}
	return "SELECT count(nextval(" + lit(quoted) + ")) FROM range(" + strconv.FormatInt(n, 10) + ")"
}

// column writes a column with its default, or with the expression that computes it.
func column(c dump.Column, typeSQL string, q func(string) string, _ func(string) string) string {
	var b strings.Builder
	b.WriteString(q(c.Name) + " " + typeSQL)
	switch {
	case c.Generated != "" && c.Default != nil:
		b.WriteString(" GENERATED ALWAYS AS (" + *c.Default + ") VIRTUAL")
	case c.Default != nil:
		b.WriteString(" DEFAULT " + *c.Default)
	}
	if !c.Nullable {
		b.WriteString(" NOT NULL")
	}
	return b.String()
}
