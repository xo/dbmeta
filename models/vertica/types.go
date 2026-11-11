package vertica

import (
	"strconv"
	"strings"

	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/dump"
)

// The type functions of the dump. See D274, D304 and docs/DUMP.md.

func init() {
	dump.RegisterTypes(dbmeta.Vertica, dump.Types{Parse: parseType, Render: renderType, Text: "LONG VARCHAR"})
}

// The limits of Vertica.
const (
	// maxVarchar is the most bytes of a VARCHAR. A LONG VARCHAR holds more.
	maxVarchar = 65000
	// maxLongVarchar is the most bytes of a LONG VARCHAR.
	maxLongVarchar = 32000000
	// maxNumeric is the most digits of a NUMERIC.
	maxNumeric = 1024
)

// parseType reads what the catalog gives, such as int, varchar(20), numeric(10,2),
// timestamptz or array[int8](65000). Every integer of Vertica has eight bytes.
func parseType(text string) (dump.Type, error) {
	text = strings.ToLower(strings.TrimSpace(text))
	if inner, ok := strings.CutPrefix(text, "array["); ok {
		if i := strings.IndexByte(inner, ']'); i > 0 {
			elem, err := parseType(inner[:i])
			if err != nil {
				return dump.Type{}, err
			}
			return dump.Type{Kind: dump.Array, Elem: &elem}, nil
		}
	}
	name, args := text, []int(nil)
	if i := strings.IndexByte(text, '('); i >= 0 && strings.HasSuffix(text, ")") {
		name = strings.TrimSpace(text[:i])
		for a := range strings.SplitSeq(text[i+1:len(text)-1], ",") {
			n, _ := strconv.Atoi(strings.TrimSpace(a))
			args = append(args, n)
		}
	}
	number := func(i int) int {
		if i < len(args) {
			return args[i]
		}
		return 0
	}
	switch name {
	case "boolean":
		return dump.Type{Kind: dump.Boolean}, nil
	case "int", "int8", "integer", "bigint", "smallint", "tinyint":
		return dump.Type{Kind: dump.Integer, Bits: 64}, nil
	case "float", "float8", "double precision", "real":
		return dump.Type{Kind: dump.Float, Bits: 64}, nil
	case "numeric", "decimal", "number":
		return dump.Type{Kind: dump.Decimal, Precision: number(0), Scale: number(1)}, nil
	case "char":
		return dump.Type{Kind: dump.String, Fixed: true, Length: number(0)}, nil
	case "varchar", "long varchar":
		return dump.Type{Kind: dump.String, Length: number(0)}, nil
	case "binary":
		return dump.Type{Kind: dump.Binary, Fixed: true, Length: number(0)}, nil
	case "varbinary", "long varbinary":
		return dump.Type{Kind: dump.Binary, Length: number(0)}, nil
	case "date":
		return dump.Type{Kind: dump.Date}, nil
	case "time":
		return dump.Type{Kind: dump.TimeOfDay}, nil
	case "timetz", "time with time zone":
		return dump.Type{Kind: dump.TimeOfDayOffset}, nil
	case "timestamp", "datetime", "smalldatetime":
		return dump.Type{Kind: dump.LocalTimestamp}, nil
	case "timestamptz", "timestamp with time zone":
		return dump.Type{Kind: dump.Timestamp}, nil
	case "uuid":
		return dump.Type{Kind: dump.UUID}, nil
	}
	return dump.Type{Kind: dump.Other}, nil
}

// renderType writes a type in Vertica. The length of a string is in bytes, so a string
// gets four bytes for each character. Vertica has no unsigned integer, no interval that
// every other product shares and no JSON type.
func renderType(t dump.Type) (dump.Rendering, error) {
	exact := func(sql string) (dump.Rendering, error) {
		return dump.Rendering{SQL: sql}, nil
	}
	switch t.Kind {
	case dump.Boolean:
		return exact("BOOLEAN")
	case dump.Integer:
		if t.Unsigned && t.Bits >= 64 {
			return exact("NUMERIC(20, 0)")
		}
		return exact("INT")
	case dump.Float:
		return exact("FLOAT")
	case dump.Decimal:
		if t.Precision == 0 || t.Precision > maxNumeric {
			return dump.Rendering{}, dump.ErrNoEquivalent
		}
		return exact("NUMERIC(" + strconv.Itoa(t.Precision) + ", " + strconv.Itoa(t.Scale) + ")")
	case dump.String:
		word := "VARCHAR"
		if t.Fixed {
			word = "CHAR"
		}
		switch n := t.Length * 4; {
		case t.Length == 0:
			return exact("LONG VARCHAR")
		case n <= maxVarchar:
			return exact(word + "(" + strconv.Itoa(n) + ")")
		case n <= maxLongVarchar:
			return exact("LONG VARCHAR(" + strconv.Itoa(n) + ")")
		}
		return exact("LONG VARCHAR")
	case dump.Binary:
		if t.Length > 0 && t.Length <= maxVarchar {
			word := "VARBINARY"
			if t.Fixed {
				word = "BINARY"
			}
			return exact(word + "(" + strconv.Itoa(t.Length) + ")")
		}
		return exact("LONG VARBINARY")
	case dump.Date:
		return exact("DATE")
	case dump.TimeOfDay:
		return exact("TIME")
	case dump.TimeOfDayOffset:
		return exact("TIMETZ")
	case dump.LocalTimestamp:
		return exact("TIMESTAMP")
	case dump.Timestamp:
		return exact("TIMESTAMPTZ")
	case dump.UUID:
		return exact("UUID")
	case dump.Array:
		if t.Elem == nil {
			return dump.Rendering{}, dump.ErrNoEquivalent
		}
		r, err := renderType(*t.Elem)
		if err != nil {
			return dump.Rendering{}, err
		}
		r.SQL = "ARRAY[" + r.SQL + "]"
		return r, nil
	}
	return dump.Rendering{}, dump.ErrNoEquivalent
}
