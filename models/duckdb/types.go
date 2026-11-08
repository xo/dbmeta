package duckdb

import (
	"strconv"
	"strings"

	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/dump"
)

// The type functions of the dump. See D274 and docs/DUMP.md.

func init() {
	dump.RegisterTypes(dbmeta.DuckDB, dump.Types{Parse: parseType, Render: renderType, Text: "VARCHAR"})
}

// maxDecimal is the most digits of a DuckDB decimal.
const maxDecimal = 38

// parseType reads what DuckDB gives, such as DECIMAL(10,2), TIMESTAMP WITH TIME
// ZONE or INTEGER[]. A STRUCT, a MAP or a UNION is a type that DuckDB builds from
// others, and it is not read here.
func parseType(text string) (dump.Type, error) {
	trimmed := strings.TrimSpace(text)
	upper := strings.ToUpper(trimmed)
	switch {
	case strings.HasPrefix(upper, "STRUCT("), strings.HasPrefix(upper, "UNION("):
		return dump.Type{Kind: dump.Tuple}, nil
	case strings.HasPrefix(upper, "MAP("):
		return dump.Type{Kind: dump.Map}, nil
	case strings.HasPrefix(upper, "ENUM("):
		return dump.Type{Kind: dump.String}, nil
	}
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
	case "boolean", "bool", "logical":
		return dump.Type{Kind: dump.Boolean}
	case "tinyint", "int1":
		return dump.Type{Kind: dump.Integer, Bits: 8}
	case "smallint", "int2", "short":
		return dump.Type{Kind: dump.Integer, Bits: 16}
	case "integer", "int", "int4", "signed":
		return dump.Type{Kind: dump.Integer, Bits: 32}
	case "bigint", "int8", "long":
		return dump.Type{Kind: dump.Integer, Bits: 64}
	case "utinyint":
		return dump.Type{Kind: dump.Integer, Bits: 8, Unsigned: true}
	case "usmallint":
		return dump.Type{Kind: dump.Integer, Bits: 16, Unsigned: true}
	case "uinteger":
		return dump.Type{Kind: dump.Integer, Bits: 32, Unsigned: true}
	case "ubigint":
		return dump.Type{Kind: dump.Integer, Bits: 64, Unsigned: true}
	case "hugeint", "uhugeint":
		return dump.Type{Kind: dump.BigInteger, Unsigned: s.Name == "uhugeint"}
	case "float", "float4", "real":
		return dump.Type{Kind: dump.Float, Bits: 32}
	case "double", "float8":
		return dump.Type{Kind: dump.Float, Bits: 64}
	case "decimal", "numeric":
		return dump.Type{Kind: dump.Decimal, Precision: arg(0), Scale: arg(1)}
	case "varchar", "char", "bpchar", "text", "string", "nvarchar":
		return dump.Type{Kind: dump.String, Length: arg(0)}
	case "blob", "bytea", "binary", "varbinary":
		return dump.Type{Kind: dump.Binary}
	case "date":
		return dump.Type{Kind: dump.Date}
	case "time":
		return dump.Type{Kind: dump.TimeOfDay}
	case "time with time zone", "timetz":
		return dump.Type{Kind: dump.TimeOfDayOffset}
	case "timestamp", "datetime", "timestamp_us":
		return dump.Type{Kind: dump.LocalTimestamp}
	case "timestamp_s":
		return dump.Type{Kind: dump.LocalTimestamp}
	case "timestamp_ms":
		return dump.Type{Kind: dump.LocalTimestamp, Precision: 3}
	case "timestamp_ns":
		return dump.Type{Kind: dump.LocalTimestamp, Precision: 9}
	case "timestamp with time zone", "timestamptz":
		return dump.Type{Kind: dump.Timestamp}
	case "interval":
		return dump.Type{Kind: dump.Interval}
	case "uuid":
		return dump.Type{Kind: dump.UUID}
	case "json":
		return dump.Type{Kind: dump.JSON}
	}
	return dump.Type{Kind: dump.Other}
}

// renderType writes a type in DuckDB.
func renderType(t dump.Type) (dump.Rendering, error) {
	exact := func(sql string) (dump.Rendering, error) {
		return dump.Rendering{SQL: sql}, nil
	}
	switch t.Kind {
	case dump.Boolean:
		return exact("BOOLEAN")
	case dump.Integer:
		return exact(integerType(t))
	case dump.BigInteger:
		if t.Unsigned {
			return exact("UHUGEINT")
		}
		return exact("HUGEINT")
	case dump.Float:
		if t.Bits == 32 {
			return exact("FLOAT")
		}
		return exact("DOUBLE")
	case dump.Decimal:
		return decimalType(t)
	case dump.String:
		return exact("VARCHAR")
	case dump.Binary:
		return exact("BLOB")
	case dump.Date:
		return exact("DATE")
	case dump.TimeOfDay:
		return exact("TIME")
	case dump.TimeOfDayOffset:
		return exact("TIME WITH TIME ZONE")
	case dump.LocalTimestamp:
		return timestampType(t, "TIMESTAMP")
	case dump.Timestamp:
		return timestampType(t, "TIMESTAMP WITH TIME ZONE")
	case dump.Interval:
		return exact("INTERVAL")
	case dump.UUID:
		return exact("UUID")
	case dump.JSON:
		return exact("JSON")
	case dump.Array:
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
	return dump.Rendering{}, dump.ErrNoEquivalent
}

// integerType names the DuckDB integer with the bits and the sign.
func integerType(t dump.Type) string {
	bits := t.Bits
	if bits == 0 {
		bits = 32
	}
	names := map[int]string{8: "TINYINT", 16: "SMALLINT", 32: "INTEGER", 64: "BIGINT"}
	if t.Unsigned {
		names = map[int]string{8: "UTINYINT", 16: "USMALLINT", 32: "UINTEGER", 64: "UBIGINT"}
	}
	return names[bits]
}

// decimalType writes DECIMAL(p,s). A decimal of DuckDB has at most 38 digits,
// and a decimal with no limit has no equivalent.
func decimalType(t dump.Type) (dump.Rendering, error) {
	switch {
	case t.Precision == 0:
		return dump.Rendering{}, dump.ErrNoEquivalent
	case t.Precision > maxDecimal:
		return dump.Rendering{
			SQL:   "DECIMAL(" + strconv.Itoa(maxDecimal) + "," + strconv.Itoa(min(t.Scale, maxDecimal)) + ")",
			Level: dump.Lossy,
			Note:  "a decimal of DuckDB holds at most 38 digits and the source has " + strconv.Itoa(t.Precision),
		}, nil
	}
	return dump.Rendering{SQL: "DECIMAL(" + strconv.Itoa(t.Precision) + "," + strconv.Itoa(t.Scale) + ")"}, nil
}

// timestampType picks the DuckDB timestamp that keeps the digits of a second.
// DuckDB has seconds, milliseconds, microseconds and nanoseconds, and a zone
// only at the microsecond.
func timestampType(t dump.Type, base string) (dump.Rendering, error) {
	zone := base != "TIMESTAMP"
	switch {
	case t.Precision > 6 && !zone:
		return dump.Rendering{SQL: "TIMESTAMP_NS"}, nil
	case t.Precision > 6:
		return dump.Rendering{SQL: base, Level: dump.Lossy,
			Note: "DuckDB keeps 6 digits of a second with a zone and the source has " + strconv.Itoa(t.Precision)}, nil
	}
	return dump.Rendering{SQL: base}, nil
}
