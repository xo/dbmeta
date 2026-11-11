package clickhouse

import (
	"strconv"
	"strings"

	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/dump"
)

// The type functions of the dump. See D274, D303 and docs/DUMP.md.

func init() {
	dump.RegisterTypes(dbmeta.ClickHouse, dump.Types{Parse: parseType, Render: renderType, Text: "String"})
}

// maxDecimalPrecision is the most digits of a Decimal.
const maxDecimalPrecision = 76

// maxTimePrecision is the most digits of a second of a DateTime64.
const maxTimePrecision = 9

// defaultTimePrecision is the digits that a timestamp with no precision gets. A
// timestamp of PostgreSQL keeps six.
const defaultTimePrecision = 6

// parseType reads what system.columns gives, such as Nullable(Decimal(10, 2)),
// LowCardinality(String), DateTime64(3, 'UTC') or Array(Nullable(String)). Nullable
// and LowCardinality describe how the column is stored and not what it holds, so
// they are taken off. A type that no other product has is the kind other.
func parseType(text string) (dump.Type, error) {
	text = strings.TrimSpace(text)
	if inner, ok := wrapped(text, "Nullable"); ok {
		return parseType(inner)
	}
	if inner, ok := wrapped(text, "LowCardinality"); ok {
		return parseType(inner)
	}
	if inner, ok := wrapped(text, "Array"); ok {
		elem, err := parseType(inner)
		if err != nil {
			return dump.Type{}, err
		}
		return dump.Type{Kind: dump.Array, Elem: &elem}, nil
	}
	name, args := text, []string(nil)
	if i := strings.IndexByte(text, '('); i >= 0 && strings.HasSuffix(text, ")") {
		name = text[:i]
		for a := range strings.SplitSeq(text[i+1:len(text)-1], ",") {
			args = append(args, strings.Trim(strings.TrimSpace(a), "'"))
		}
	}
	number := func(i int) int {
		if i < len(args) {
			n, _ := strconv.Atoi(args[i])
			return n
		}
		return 0
	}
	switch name {
	case "Bool":
		return dump.Type{Kind: dump.Boolean}, nil
	case "Int8":
		return dump.Type{Kind: dump.Integer, Bits: 8}, nil
	case "Int16":
		return dump.Type{Kind: dump.Integer, Bits: 16}, nil
	case "Int32":
		return dump.Type{Kind: dump.Integer, Bits: 32}, nil
	case "Int64":
		return dump.Type{Kind: dump.Integer, Bits: 64}, nil
	case "UInt8":
		return dump.Type{Kind: dump.Integer, Bits: 8, Unsigned: true}, nil
	case "UInt16":
		return dump.Type{Kind: dump.Integer, Bits: 16, Unsigned: true}, nil
	case "UInt32":
		return dump.Type{Kind: dump.Integer, Bits: 32, Unsigned: true}, nil
	case "UInt64":
		return dump.Type{Kind: dump.Integer, Bits: 64, Unsigned: true}, nil
	case "Float32":
		return dump.Type{Kind: dump.Float, Bits: 32}, nil
	case "Float64":
		return dump.Type{Kind: dump.Float, Bits: 64}, nil
	case "Decimal":
		return dump.Type{Kind: dump.Decimal, Precision: number(0), Scale: number(1)}, nil
	case "Decimal32":
		return dump.Type{Kind: dump.Decimal, Precision: 9, Scale: number(0)}, nil
	case "Decimal64":
		return dump.Type{Kind: dump.Decimal, Precision: 18, Scale: number(0)}, nil
	case "Decimal128":
		return dump.Type{Kind: dump.Decimal, Precision: 38, Scale: number(0)}, nil
	case "Decimal256":
		return dump.Type{Kind: dump.Decimal, Precision: 76, Scale: number(0)}, nil
	case "String":
		return dump.Type{Kind: dump.String}, nil
	case "FixedString":
		return dump.Type{Kind: dump.String, Fixed: true, Length: number(0)}, nil
	case "Date", "Date32":
		return dump.Type{Kind: dump.Date}, nil
	case "DateTime":
		if len(args) > 0 {
			return dump.Type{Kind: dump.Timestamp}, nil
		}
		return dump.Type{Kind: dump.LocalTimestamp}, nil
	case "DateTime64":
		if len(args) > 1 {
			return dump.Type{Kind: dump.Timestamp, Precision: number(0)}, nil
		}
		return dump.Type{Kind: dump.LocalTimestamp, Precision: number(0)}, nil
	case "UUID":
		return dump.Type{Kind: dump.UUID}, nil
	case "IPv4", "IPv6":
		return dump.Type{Kind: dump.IPAddress}, nil
	}
	return dump.Type{Kind: dump.Other}, nil
}

// wrapped returns the text inside name( and ), and whether text is that call.
func wrapped(text, name string) (string, bool) {
	if strings.HasPrefix(text, name+"(") && strings.HasSuffix(text, ")") {
		return text[len(name)+1 : len(text)-1], true
	}
	return "", false
}

// renderType writes a type in ClickHouse. It has no time of day, no interval and no
// JSON type that every release reads, and it has no binary type, so bytes go in a
// String, which is Lossy. Whether a column can be NULL is the business of the column
// and not of the type, so the style wraps the type in Nullable.
func renderType(t dump.Type) (dump.Rendering, error) {
	exact := func(sql string) (dump.Rendering, error) {
		return dump.Rendering{SQL: sql}, nil
	}
	switch t.Kind {
	case dump.Boolean:
		return exact("Bool")
	case dump.Integer:
		return exact(integerType(t))
	case dump.Float:
		if t.Bits == 32 {
			return exact("Float32")
		}
		return exact("Float64")
	case dump.Decimal:
		if t.Precision == 0 || t.Precision > maxDecimalPrecision {
			return dump.Rendering{}, dump.ErrNoEquivalent
		}
		return exact("Decimal(" + strconv.Itoa(t.Precision) + ", " + strconv.Itoa(t.Scale) + ")")
	case dump.String:
		if t.Fixed && t.Length > 0 {
			return exact("FixedString(" + strconv.Itoa(t.Length) + ")")
		}
		return exact("String")
	case dump.Binary:
		return dump.Rendering{SQL: "String", Level: dump.Lossy,
			Note: "ClickHouse has no binary type, so the bytes go in a String"}, nil
	case dump.Date:
		return exact("Date32")
	case dump.LocalTimestamp:
		return exact("DateTime64(" + strconv.Itoa(timePrecision(t.Precision)) + ")")
	case dump.Timestamp:
		return exact("DateTime64(" + strconv.Itoa(timePrecision(t.Precision)) + ", 'UTC')")
	case dump.UUID:
		return exact("UUID")
	case dump.Array:
		return arrayType(t)
	}
	return dump.Rendering{}, dump.ErrNoEquivalent
}

// timePrecision is the digits of a second that a DateTime64 gets for a source with
// the given digits.
func timePrecision(p int) int {
	switch {
	case p == 0:
		return defaultTimePrecision
	case p > maxTimePrecision:
		return maxTimePrecision
	}
	return p
}

// integerType names the type of the integer. ClickHouse has both signs at every size.
func integerType(t dump.Type) string {
	bits := t.Bits
	if bits == 0 {
		bits = 32
	}
	name := "Int"
	if t.Unsigned {
		name = "UInt"
	}
	return name + strconv.Itoa(bits)
}

// arrayType writes Array(element). The level is the level of the element.
func arrayType(t dump.Type) (dump.Rendering, error) {
	if t.Elem == nil {
		return dump.Rendering{}, dump.ErrNoEquivalent
	}
	r, err := renderType(*t.Elem)
	if err != nil {
		return dump.Rendering{}, err
	}
	r.SQL = "Array(" + r.SQL + ")"
	return r, nil
}
