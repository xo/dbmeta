package cratedb

import (
	"strconv"
	"strings"

	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/dump"
)

// The type functions of the dump. See D274 and docs/DUMP.md.

func init() {
	dump.RegisterTypes(dbmeta.CrateDB, dump.Types{Parse: parseType, Render: renderType, Text: "text"})
}

// maxTimePrecision is the digits of a second that CrateDB keeps. It stores a
// timestamp as milliseconds.
const maxTimePrecision = 3

// maxNumericPrecision is the most digits of a numeric. CrateDB refuses a numeric
// with no digits named, so a source with none has no exact type here.
const maxNumericPrecision = 38

// parseType reads the data_type of the model, which is what the catalog gives with
// the digits of a numeric and the length of a string added, such as numeric(10,2),
// character varying(5) or integer_array. A type that CrateDB has and that no other
// product has, such as an object or a geo point, is the kind other.
func parseType(text string) (dump.Type, error) {
	text = strings.TrimSpace(text)
	arrays := 0
	for strings.HasSuffix(text, "_array") {
		text = strings.TrimSuffix(text, "_array")
		arrays++
	}
	s, err := dump.ParseSpec(text)
	if err != nil {
		return dump.Type{}, err
	}
	t := parseBase(s)
	for range arrays + s.Arrays {
		elem := t
		t = dump.Type{Kind: dump.Array, Elem: &elem}
	}
	return t, nil
}

// parseBase reads the type without its array suffix.
func parseBase(s dump.Spec) dump.Type {
	arg := func(i int) int {
		if i < len(s.Args) {
			return s.Args[i]
		}
		return 0
	}
	switch s.Key() {
	case "byte", "char":
		return dump.Type{Kind: dump.Integer, Bits: 8}
	case "smallint", "int2":
		return dump.Type{Kind: dump.Integer, Bits: 16}
	case "integer", "int", "int4":
		return dump.Type{Kind: dump.Integer, Bits: 32}
	case "bigint", "int8":
		return dump.Type{Kind: dump.Integer, Bits: 64}
	case "real", "float4":
		return dump.Type{Kind: dump.Float, Bits: 32}
	case "double precision", "float8":
		return dump.Type{Kind: dump.Float, Bits: 64}
	case "numeric", "decimal":
		return dump.Type{Kind: dump.Decimal, Precision: arg(0), Scale: arg(1)}
	case "character":
		return dump.Type{Kind: dump.String, Fixed: true, Length: arg(0)}
	case "character varying", "varchar":
		return dump.Type{Kind: dump.String, Length: arg(0)}
	case "text":
		return dump.Type{Kind: dump.String}
	case "boolean", "bool":
		return dump.Type{Kind: dump.Boolean}
	case "timestamp with time zone", "timestamptz":
		return dump.Type{Kind: dump.Timestamp, Precision: maxTimePrecision}
	case "timestamp without time zone", "timestamp":
		return dump.Type{Kind: dump.LocalTimestamp, Precision: maxTimePrecision}
	case "uuid":
		return dump.Type{Kind: dump.UUID}
	case "ip":
		return dump.Type{Kind: dump.IPAddress}
	}
	return dump.Type{Kind: dump.Other}
}

// renderType writes a type in CrateDB. It has no date, no time, no interval, no
// binary type and no JSON type, so a column of those kinds has no equivalent and
// takes the Text level when the client allows it. A date is a timestamp at
// midnight, which is Lossy.
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
		if t.Precision == 0 || t.Precision > maxNumericPrecision {
			return dump.Rendering{}, dump.ErrNoEquivalent
		}
		return exact("numeric(" + strconv.Itoa(t.Precision) + "," + strconv.Itoa(t.Scale) + ")")
	case dump.String:
		switch {
		case t.Fixed && t.Length > 0:
			return exact("character(" + strconv.Itoa(t.Length) + ")")
		case t.Length > 0:
			return exact("varchar(" + strconv.Itoa(t.Length) + ")")
		}
		return exact("text")
	case dump.Date:
		return dump.Rendering{SQL: "timestamp without time zone", Level: dump.Lossy,
			Note: "CrateDB has no date type, so a date is a timestamp at midnight"}, nil
	case dump.LocalTimestamp:
		return timestampType("timestamp without time zone", t.Precision)
	case dump.Timestamp:
		return timestampType("timestamp with time zone", t.Precision)
	case dump.UUID:
		return exact("uuid")
	case dump.IPAddress:
		return exact("ip")
	case dump.Array:
		return arrayType(t)
	}
	return dump.Rendering{}, dump.ErrNoEquivalent
}

// integerType names the smallest type that holds every value of the integer.
// CrateDB has no unsigned integer, so an unsigned one takes the next size up, and
// an unsigned 64 bit integer is a numeric of 20 digits.
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
	case 8:
		return "byte"
	case 16:
		return "smallint"
	case 32:
		return "integer"
	}
	return "bigint"
}

// timestampType writes a timestamp. CrateDB keeps milliseconds and takes no
// precision, so a source with more digits is Lossy.
func timestampType(sql string, precision int) (dump.Rendering, error) {
	r := dump.Rendering{SQL: sql}
	if precision > maxTimePrecision {
		r.Level = dump.Lossy
		r.Note = "CrateDB keeps milliseconds and the source has " + strconv.Itoa(precision) + " digits of a second"
	}
	return r, nil
}

// arrayType writes array(element). The level is the level of the element.
func arrayType(t dump.Type) (dump.Rendering, error) {
	if t.Elem == nil {
		return dump.Rendering{}, dump.ErrNoEquivalent
	}
	r, err := renderType(*t.Elem)
	if err != nil {
		return dump.Rendering{}, err
	}
	r.SQL = "array(" + r.SQL + ")"
	return r, nil
}
