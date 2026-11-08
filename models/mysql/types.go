package mysql

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/dump"
)

// The type functions of the dump, for MySQL and MariaDB, which share a dialect.
// See D274 and docs/DUMP.md.

func init() {
	dump.RegisterTypes(dbmeta.MySQL, dump.Types{Parse: parseType, Render: renderType, Text: "longtext"})
}

// Limits of the types of MySQL, in characters, bytes and digits.
const (
	maxChar       = 255
	maxVarchar    = 16383 // characters, with the 4 bytes of utf8mb4 in a row of 65535 bytes
	maxVarbinary  = 65535
	maxDecimal    = 65
	maxDecimalS   = 30
	maxTimeDigits = 6
)

// parseType reads COLUMN_TYPE, such as tinyint(1), bigint(20) unsigned,
// decimal(10,2), varchar(255) or enum('a','b').
//
// A tinyint(1) is the kind boolean, because the manual says that BOOL and
// BOOLEAN are tinyint(1). The catalog cannot tell a column that holds a small
// number from one that holds a boolean, so the data side checks the values.
// The same holds for bit(1). See docs/DUMP.md.
func parseType(text string) (dump.Type, error) {
	lower := strings.ToLower(strings.TrimSpace(text))
	for _, name := range []string{"enum", "set"} {
		if strings.HasPrefix(lower, name+"(") {
			values, err := quotedList(strings.TrimSpace(text)[len(name):])
			if err != nil {
				return dump.Type{}, err
			}
			if name == "set" {
				return dump.Type{Kind: dump.Set, Elem: &dump.Type{Kind: dump.String}, Values: values}, nil
			}
			return dump.Type{Kind: dump.String, Values: values}, nil
		}
	}
	s, err := dump.ParseSpec(text)
	if err != nil {
		return dump.Type{}, err
	}
	arg := func(i int) int {
		if i < len(s.Args) {
			return s.Args[i]
		}
		return 0
	}
	// The words after the type are in Rest when it has brackets and in Name
	// when it has none, so both are read from the key.
	key := s.Key()
	unsigned := strings.Contains(key, " unsigned")
	for _, word := range []string{" unsigned", " zerofill"} {
		key = strings.ReplaceAll(key, word, "")
	}
	switch key {
	case "tinyint":
		if len(s.Args) == 1 && s.Args[0] == 1 && !unsigned {
			return dump.Type{Kind: dump.Boolean}, nil
		}
		return dump.Type{Kind: dump.Integer, Bits: 8, Unsigned: unsigned}, nil
	case "smallint":
		return dump.Type{Kind: dump.Integer, Bits: 16, Unsigned: unsigned}, nil
	case "mediumint", "int", "integer":
		return dump.Type{Kind: dump.Integer, Bits: 32, Unsigned: unsigned}, nil
	case "bigint":
		return dump.Type{Kind: dump.Integer, Bits: 64, Unsigned: unsigned}, nil
	case "serial":
		return dump.Type{Kind: dump.Integer, Bits: 64, Unsigned: true}, nil
	case "year":
		return dump.Type{Kind: dump.Integer, Bits: 16, Unsigned: true}, nil
	case "float":
		return dump.Type{Kind: dump.Float, Bits: 32, Unsigned: unsigned}, nil
	case "double", "double precision", "real":
		return dump.Type{Kind: dump.Float, Bits: 64, Unsigned: unsigned}, nil
	case "decimal", "numeric", "dec", "fixed":
		return dump.Type{Kind: dump.Decimal, Precision: arg(0), Scale: arg(1), Unsigned: unsigned}, nil
	case "char":
		return dump.Type{Kind: dump.String, Fixed: true, Length: arg(0)}, nil
	case "varchar":
		return dump.Type{Kind: dump.String, Length: arg(0)}, nil
	case "tinytext", "text", "mediumtext", "longtext":
		return dump.Type{Kind: dump.String}, nil
	case "binary":
		return dump.Type{Kind: dump.Binary, Fixed: true, Length: arg(0)}, nil
	case "varbinary":
		return dump.Type{Kind: dump.Binary, Length: arg(0)}, nil
	case "tinyblob", "blob", "mediumblob", "longblob":
		return dump.Type{Kind: dump.Binary}, nil
	case "bit":
		if arg(0) == 1 {
			return dump.Type{Kind: dump.Boolean}, nil
		}
		return dump.Type{Kind: dump.Other}, nil
	case "date":
		return dump.Type{Kind: dump.Date}, nil
	case "time":
		return dump.Type{Kind: dump.TimeOfDay, Precision: arg(0)}, nil
	case "datetime":
		return dump.Type{Kind: dump.LocalTimestamp, Precision: arg(0)}, nil
	case "timestamp":
		return dump.Type{Kind: dump.Timestamp, Precision: arg(0)}, nil
	case "json":
		return dump.Type{Kind: dump.JSON}, nil
	case "uuid":
		return dump.Type{Kind: dump.UUID}, nil
	case "geometry", "point", "linestring", "polygon", "multipoint", "multilinestring",
		"multipolygon", "geometrycollection", "geomcollection":
		return dump.Type{Kind: dump.Geometry}, nil
	}
	return dump.Type{Kind: dump.Other}, nil
}

// quotedList reads ('a','b') as its values. A quote inside a value is doubled,
// and a backslash escapes the next character, as COLUMN_TYPE writes them.
func quotedList(text string) ([]string, error) {
	text = strings.TrimSpace(text)
	if !strings.HasPrefix(text, "(") || !strings.HasSuffix(text, ")") {
		return nil, fmt.Errorf("the list %q is not in brackets: %w", text, dump.ErrUnknownType)
	}
	text = text[1 : len(text)-1]
	var values []string
	for text != "" {
		if text[0] != '\'' {
			return nil, fmt.Errorf("the list %q has a value with no quote: %w", text, dump.ErrUnknownType)
		}
		var b strings.Builder
		i := 1
		closed := false
		for i < len(text) {
			c := text[i]
			switch {
			case c == '\\' && i+1 < len(text):
				b.WriteByte(text[i+1])
				i += 2
			case c == '\'' && i+1 < len(text) && text[i+1] == '\'':
				b.WriteByte('\'')
				i += 2
			case c == '\'':
				closed = true
				i++
			default:
				b.WriteByte(c)
				i++
			}
			if closed {
				break
			}
		}
		if !closed {
			return nil, fmt.Errorf("the list %q has a value that does not close: %w", text, dump.ErrUnknownType)
		}
		values = append(values, b.String())
		text = strings.TrimPrefix(text[i:], ",")
	}
	return values, nil
}

// renderType writes a type in MySQL.
func renderType(t dump.Type) (dump.Rendering, error) {
	exact := func(sql string) (dump.Rendering, error) {
		return dump.Rendering{SQL: sql}, nil
	}
	switch t.Kind {
	case dump.Boolean:
		return exact("tinyint(1)")
	case dump.Integer:
		return exact(integerType(t))
	case dump.Float:
		name := "double"
		if t.Bits == 32 {
			name = "float"
		}
		return exact(name + unsigned(t.Unsigned))
	case dump.Decimal:
		return decimalType(t)
	case dump.String:
		return stringType(t)
	case dump.Binary:
		return binaryType(t)
	case dump.Date:
		return exact("date")
	case dump.TimeOfDay:
		return fraction("time", t.Precision)
	case dump.LocalTimestamp:
		return fraction("datetime", t.Precision)
	case dump.Timestamp:
		r, err := fraction("datetime", t.Precision)
		if err != nil {
			return r, err
		}
		if r.Level == dump.Exact {
			r.Level = dump.Lossy
			r.Note = "MySQL has no type that keeps a zone, so the instant is written as a datetime in UTC"
		}
		return r, nil
	case dump.JSON:
		return exact("json")
	}
	return dump.Rendering{}, dump.ErrNoEquivalent
}

// unsigned returns the word that follows a numeric type.
func unsigned(u bool) string {
	if u {
		return " unsigned"
	}
	return ""
}

// integerType names the MySQL integer with the bits and the sign.
func integerType(t dump.Type) string {
	name := map[int]string{8: "tinyint", 16: "smallint", 32: "int", 64: "bigint"}[t.Bits]
	if name == "" {
		name = "int"
	}
	return name + unsigned(t.Unsigned)
}

// decimalType writes decimal(p,s). MySQL has at most 65 digits and a scale of
// 30, and a decimal with no limit has no equivalent.
func decimalType(t dump.Type) (dump.Rendering, error) {
	if t.Precision == 0 {
		return dump.Rendering{}, dump.ErrNoEquivalent
	}
	p, s := t.Precision, t.Scale
	var r dump.Rendering
	if p > maxDecimal || s > maxDecimalS {
		p, s = min(p, maxDecimal), min(s, maxDecimalS)
		r.Level = dump.Lossy
		r.Note = fmt.Sprintf("a decimal of MySQL has at most %d digits and a scale of %d, and the source has %d and %d",
			maxDecimal, maxDecimalS, t.Precision, t.Scale)
	}
	r.SQL = "decimal(" + strconv.Itoa(p) + "," + strconv.Itoa(s) + ")" + unsigned(t.Unsigned)
	return r, nil
}

// stringType writes char(n), varchar(n), an enum or longtext. A string with a
// list of values is an enum.
func stringType(t dump.Type) (dump.Rendering, error) {
	switch {
	case len(t.Values) > 0:
		quoted := make([]string, len(t.Values))
		for i, v := range t.Values {
			v = strings.ReplaceAll(strings.ReplaceAll(v, `\`, `\\`), `'`, `''`)
			quoted[i] = "'" + v + "'"
		}
		return dump.Rendering{SQL: "enum(" + strings.Join(quoted, ",") + ")"}, nil
	case t.Fixed && t.Length > 0 && t.Length <= maxChar:
		return dump.Rendering{SQL: "char(" + strconv.Itoa(t.Length) + ")"}, nil
	case !t.Fixed && t.Length > 0 && t.Length <= maxVarchar:
		return dump.Rendering{SQL: "varchar(" + strconv.Itoa(t.Length) + ")"}, nil
	}
	return dump.Rendering{SQL: "longtext"}, nil
}

// binaryType writes binary(n), varbinary(n) or longblob.
func binaryType(t dump.Type) (dump.Rendering, error) {
	switch {
	case t.Fixed && t.Length > 0 && t.Length <= maxChar:
		return dump.Rendering{SQL: "binary(" + strconv.Itoa(t.Length) + ")"}, nil
	case !t.Fixed && t.Length > 0 && t.Length <= maxVarbinary:
		return dump.Rendering{SQL: "varbinary(" + strconv.Itoa(t.Length) + ")"}, nil
	}
	return dump.Rendering{SQL: "longblob"}, nil
}

// fraction writes a time or a datetime with the digits of a second. MySQL keeps
// six.
func fraction(name string, digits int) (dump.Rendering, error) {
	switch {
	case digits == 0:
		return dump.Rendering{SQL: name}, nil
	case digits > maxTimeDigits:
		return dump.Rendering{
			SQL:   name + "(" + strconv.Itoa(maxTimeDigits) + ")",
			Level: dump.Lossy,
			Note:  fmt.Sprintf("MySQL keeps %d digits of a second and the source has %d", maxTimeDigits, digits),
		}, nil
	}
	return dump.Rendering{SQL: name + "(" + strconv.Itoa(digits) + ")"}, nil
}
