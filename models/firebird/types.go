package firebird

import (
	"strconv"
	"strings"

	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/dump"
)

// The type functions of the dump. See D274, D304 and docs/DUMP.md.

func init() {
	dump.RegisterTypes(dbmeta.Firebird, dump.Types{Parse: parseType, Render: renderType, Text: "BLOB SUB_TYPE TEXT CHARACTER SET UTF8"})
}

// maxVarchar is the most characters of a VARCHAR in the character set UTF8, which takes four
// bytes for each character, and a VARCHAR holds 32765 bytes.
const maxVarchar = 8191

// maxDecimalPrecision is the most digits of a DECIMAL. Releases before 4.0 stop at 18.
const maxDecimalPrecision = 38

// parseType reads what the model gives, which it assembles from RDB$FIELDS, such as
// VARCHAR(20), DECIMAL(10,2), BLOB SUB_TYPE 1 or TIMESTAMP WITH TIME ZONE.
func parseType(text string) (dump.Type, error) {
	text = strings.ToUpper(strings.TrimSpace(text))
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
	case "BOOLEAN":
		return dump.Type{Kind: dump.Boolean}, nil
	case "SMALLINT":
		return dump.Type{Kind: dump.Integer, Bits: 16}, nil
	case "INTEGER":
		return dump.Type{Kind: dump.Integer, Bits: 32}, nil
	case "BIGINT":
		return dump.Type{Kind: dump.Integer, Bits: 64}, nil
	case "INT128":
		return dump.Type{Kind: dump.Decimal, Precision: 38}, nil
	case "FLOAT":
		return dump.Type{Kind: dump.Float, Bits: 32}, nil
	case "DOUBLE PRECISION":
		return dump.Type{Kind: dump.Float, Bits: 64}, nil
	case "DECIMAL", "NUMERIC":
		return dump.Type{Kind: dump.Decimal, Precision: number(0), Scale: number(1)}, nil
	case "CHAR":
		return dump.Type{Kind: dump.String, Fixed: true, Length: number(0)}, nil
	case "VARCHAR":
		return dump.Type{Kind: dump.String, Length: number(0)}, nil
	case "BLOB SUB_TYPE 1":
		return dump.Type{Kind: dump.String}, nil
	case "BLOB SUB_TYPE 0":
		return dump.Type{Kind: dump.Binary}, nil
	case "DATE":
		return dump.Type{Kind: dump.Date}, nil
	case "TIME":
		return dump.Type{Kind: dump.TimeOfDay}, nil
	case "TIME WITH TIME ZONE":
		return dump.Type{Kind: dump.TimeOfDayOffset}, nil
	case "TIMESTAMP":
		return dump.Type{Kind: dump.LocalTimestamp}, nil
	case "TIMESTAMP WITH TIME ZONE":
		return dump.Type{Kind: dump.Timestamp}, nil
	}
	return dump.Type{Kind: dump.Other}, nil
}

// renderType writes a type in Firebird. A string is written in the character set UTF8, so
// that every character of the source has a place. Firebird has no unsigned integer, no
// interval, no JSON and no array that a table can hold, and no timestamp with a zone before
// release 4.0, so a timestamp holds the time in UTC.
func renderType(t dump.Type) (dump.Rendering, error) {
	exact := func(sql string) (dump.Rendering, error) {
		return dump.Rendering{SQL: sql}, nil
	}
	switch t.Kind {
	case dump.Boolean:
		return exact("BOOLEAN")
	case dump.Integer:
		// A signed type of twice the size holds every value of an unsigned one.
		bits := t.Bits
		if bits == 0 {
			bits = 32
		}
		if t.Unsigned {
			bits *= 2
		}
		switch {
		case bits <= 16:
			return exact("SMALLINT")
		case bits <= 32:
			return exact("INTEGER")
		case bits <= 64:
			return exact("BIGINT")
		}
		return dump.Rendering{}, dump.ErrNoEquivalent
	case dump.Float:
		if t.Bits == 32 {
			return exact("FLOAT")
		}
		return exact("DOUBLE PRECISION")
	case dump.Decimal:
		if t.Precision == 0 || t.Precision > maxDecimalPrecision {
			return dump.Rendering{}, dump.ErrNoEquivalent
		}
		return exact("DECIMAL(" + strconv.Itoa(t.Precision) + ", " + strconv.Itoa(t.Scale) + ")")
	case dump.String:
		if t.Length > 0 && t.Length <= maxVarchar {
			word := "VARCHAR"
			if t.Fixed {
				word = "CHAR"
			}
			return exact(word + "(" + strconv.Itoa(t.Length) + ") CHARACTER SET UTF8")
		}
		return exact("BLOB SUB_TYPE TEXT CHARACTER SET UTF8")
	case dump.Binary:
		return exact("BLOB SUB_TYPE BINARY")
	case dump.Date:
		return exact("DATE")
	case dump.TimeOfDay:
		return exact("TIME")
	case dump.LocalTimestamp:
		return exact("TIMESTAMP")
	case dump.Timestamp:
		return dump.Rendering{SQL: "TIMESTAMP", Level: dump.Lossy,
			Note: "Firebird before 4.0 has no timestamp with a zone, so the column holds the time in UTC"}, nil
	case dump.UUID:
		return dump.Rendering{SQL: "CHAR(36) CHARACTER SET ASCII", Level: dump.Lossy,
			Note: "the uuid is kept as its text"}, nil
	}
	return dump.Rendering{}, dump.ErrNoEquivalent
}
