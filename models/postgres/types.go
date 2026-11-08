package postgres

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/dump"
)

// The type functions of the dump. See D274 and docs/DUMP.md.

func init() {
	dump.RegisterTypes(dbmeta.PostgreSQL, dump.Types{Parse: parseType, Render: renderType, Text: "text"})
}

// maxTimePrecision is the most digits of a second that PostgreSQL keeps.
const maxTimePrecision = 6

// parseType reads what format_type gives, such as character varying(10),
// time(3) with time zone or integer[]. A type that is not built in, such as an
// enum, a domain or a composite, is the kind other, because the catalog gives
// its name and nothing about it.
func parseType(text string) (dump.Type, error) {
	s, err := dump.ParseSpec(text)
	if err != nil {
		return dump.Type{}, err
	}
	t := parseBase(s)
	for range s.Arrays {
		elem := t
		t = dump.Type{Kind: dump.Array, Elem: &elem}
	}
	return t, nil
}

// parseBase reads the type without its array brackets.
func parseBase(s dump.Spec) dump.Type {
	arg := func(i int) int {
		if i < len(s.Args) {
			return s.Args[i]
		}
		return 0
	}
	switch s.Key() {
	case "smallint", "int2":
		return dump.Type{Kind: dump.Integer, Bits: 16}
	case "integer", "int", "int4":
		return dump.Type{Kind: dump.Integer, Bits: 32}
	case "bigint", "int8":
		return dump.Type{Kind: dump.Integer, Bits: 64}
	case "oid":
		return dump.Type{Kind: dump.Integer, Bits: 32, Unsigned: true}
	case "real", "float4":
		return dump.Type{Kind: dump.Float, Bits: 32}
	case "double precision", "float8":
		return dump.Type{Kind: dump.Float, Bits: 64}
	case "numeric", "decimal":
		return dump.Type{Kind: dump.Decimal, Precision: arg(0), Scale: arg(1)}
	case "character", "char", "bpchar":
		return dump.Type{Kind: dump.String, Fixed: true, Length: arg(0)}
	case "character varying", "varchar":
		return dump.Type{Kind: dump.String, Length: arg(0)}
	case "text", "name", "citext":
		return dump.Type{Kind: dump.String}
	case "bytea":
		return dump.Type{Kind: dump.Binary}
	case "boolean", "bool":
		return dump.Type{Kind: dump.Boolean}
	case "date":
		return dump.Type{Kind: dump.Date}
	case "time", "time without time zone":
		return dump.Type{Kind: dump.TimeOfDay, Precision: arg(0)}
	case "time with time zone", "timetz":
		return dump.Type{Kind: dump.TimeOfDayOffset, Precision: arg(0)}
	case "timestamp", "timestamp without time zone":
		return dump.Type{Kind: dump.LocalTimestamp, Precision: arg(0)}
	case "timestamp with time zone", "timestamptz":
		return dump.Type{Kind: dump.Timestamp, Precision: arg(0)}
	case "uuid":
		return dump.Type{Kind: dump.UUID}
	case "json", "jsonb":
		return dump.Type{Kind: dump.JSON}
	case "inet", "cidr":
		return dump.Type{Kind: dump.IPAddress}
	case "int4range", "int8range", "numrange", "tsrange", "tstzrange", "daterange":
		return dump.Type{Kind: dump.Range}
	}
	if s.Name == "interval" || strings.HasPrefix(s.Name, "interval ") {
		return dump.Type{Kind: dump.Interval, Precision: arg(0)}
	}
	return dump.Type{Kind: dump.Other}
}

// renderType writes a type in PostgreSQL. The levels follow docs/DUMP.md: a type
// that holds every value is Exact, and a type that holds less is Lossy.
func renderType(t dump.Type) (dump.Rendering, error) {
	exact := func(sql string) (dump.Rendering, error) {
		return dump.Rendering{SQL: sql}, nil
	}
	switch t.Kind {
	case dump.Boolean:
		return exact("boolean")
	case dump.Integer:
		return exact(integerType(t))
	case dump.Float:
		if t.Bits == 32 {
			return exact("real")
		}
		return exact("double precision")
	case dump.Decimal:
		return decimalType(t.Precision, t.Scale)
	case dump.BigInteger:
		return exact("numeric")
	case dump.String:
		return stringType(t)
	case dump.Binary:
		return exact("bytea")
	case dump.Date:
		return exact("date")
	case dump.TimeOfDay:
		return timeType("time", " without time zone", t.Precision)
	case dump.TimeOfDayOffset:
		return timeType("time", " with time zone", t.Precision)
	case dump.LocalTimestamp:
		return timeType("timestamp", " without time zone", t.Precision)
	case dump.Timestamp:
		return timeType("timestamp", " with time zone", t.Precision)
	case dump.Interval:
		return timeType("interval", "", t.Precision)
	case dump.UUID:
		return exact("uuid")
	case dump.IPAddress:
		return exact("inet")
	case dump.JSON:
		return exact("json")
	case dump.Array:
		return arrayType(t)
	}
	return dump.Rendering{}, dump.ErrNoEquivalent
}

// integerType names the smallest PostgreSQL type that holds every value of the
// integer. PostgreSQL has no unsigned integer, so an unsigned one takes the next
// size up, and an unsigned 64 bit integer is a numeric of 20 digits.
func integerType(t dump.Type) string {
	bits := t.Bits
	if bits == 0 {
		bits = 32
	}
	if t.Unsigned {
		switch bits {
		case 8:
			return "smallint"
		case 16:
			return "integer"
		case 32:
			return "bigint"
		}
		return "numeric(20,0)"
	}
	switch bits {
	case 8, 16:
		return "smallint"
	case 32:
		return "integer"
	}
	return "bigint"
}

// maxNumericPrecision is the most digits that numeric(p,s) takes.
const maxNumericPrecision = 1000

// decimalType writes numeric(p,s). A decimal with no precision is a numeric
// with no limit.
func decimalType(precision, scale int) (dump.Rendering, error) {
	switch {
	case precision == 0:
		return dump.Rendering{SQL: "numeric"}, nil
	case precision > maxNumericPrecision:
		return dump.Rendering{
			SQL:   "numeric",
			Level: dump.Lossy,
			Note:  fmt.Sprintf("a numeric holds at most %d digits and the source has %d, so no limit is written", maxNumericPrecision, precision),
		}, nil
	}
	return dump.Rendering{SQL: "numeric(" + strconv.Itoa(precision) + "," + strconv.Itoa(scale) + ")"}, nil
}

// maxVarchar is the most characters that character varying(n) takes.
const maxVarchar = 10485760

// stringType writes character(n), character varying(n) or text.
func stringType(t dump.Type) (dump.Rendering, error) {
	switch {
	case t.Fixed && t.Length > 0 && t.Length <= maxVarchar:
		return dump.Rendering{SQL: "character(" + strconv.Itoa(t.Length) + ")"}, nil
	case t.Fixed:
		return dump.Rendering{SQL: "text", Level: dump.Lossy,
			Note: "a fixed length string with no usable length is written as text, which does not pad"}, nil
	case t.Length > 0 && t.Length <= maxVarchar:
		return dump.Rendering{SQL: "character varying(" + strconv.Itoa(t.Length) + ")"}, nil
	}
	return dump.Rendering{SQL: "text"}, nil
}

// timeType writes a time, a timestamp or an interval with the digits of the
// fraction of a second. PostgreSQL keeps six, so a source with more is Lossy.
func timeType(name, zone string, precision int) (dump.Rendering, error) {
	var r dump.Rendering
	switch {
	case precision == 0:
		r.SQL = name + zone
	case precision > maxTimePrecision:
		r.SQL = name + "(" + strconv.Itoa(maxTimePrecision) + ")" + zone
		r.Level = dump.Lossy
		r.Note = fmt.Sprintf("PostgreSQL keeps %d digits of a second and the source has %d", maxTimePrecision, precision)
	default:
		r.SQL = name + "(" + strconv.Itoa(precision) + ")" + zone
	}
	return r, nil
}

// arrayType writes the element type and the brackets. The level is the level of
// the element.
func arrayType(t dump.Type) (dump.Rendering, error) {
	if t.Elem == nil {
		return dump.Rendering{}, dump.ErrNoEquivalent
	}
	r, err := renderType(*t.Elem)
	if err != nil {
		return dump.Rendering{}, err
	}
	r.SQL += "[]"
	return r, nil
}
