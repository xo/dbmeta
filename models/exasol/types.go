package exasol

import (
	"strconv"
	"strings"

	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/dump"
)

// The type functions of the dump. See D274, D304 and docs/DUMP.md.

func init() {
	dump.RegisterTypes(dbmeta.Exasol, dump.Types{Parse: parseType, Render: renderType, Text: "VARCHAR(2000000) UTF8"})
}

// The limits of Exasol.
const (
	// maxDecimal is the most digits of a DECIMAL.
	maxDecimal = 36
	// maxVarchar is the most characters of a VARCHAR.
	maxVarchar = 2000000
	// maxChar is the most characters of a CHAR.
	maxChar = 2000
)

// parseType reads what COLUMN_TYPE gives, such as DECIMAL(18,0), VARCHAR(128) UTF8, DOUBLE,
// TIMESTAMP or TIMESTAMP WITH LOCAL TIME ZONE. Exasol keeps every integer as a DECIMAL
// with no digits after the point.
func parseType(text string) (dump.Type, error) {
	text = strings.ToUpper(strings.TrimSpace(text))
	text = strings.TrimSuffix(strings.TrimSuffix(text, " UTF8"), " ASCII")
	name, args := text, []int(nil)
	if i := strings.IndexByte(text, '('); i >= 0 {
		if j := strings.IndexByte(text, ')'); j > i {
			name = strings.TrimSpace(text[:i]) + text[j+1:]
			for a := range strings.SplitSeq(text[i+1:j], ",") {
				n, _ := strconv.Atoi(strings.TrimSpace(a))
				args = append(args, n)
			}
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
	case "DECIMAL":
		return dump.Type{Kind: dump.Decimal, Precision: number(0), Scale: number(1)}, nil
	case "DOUBLE", "DOUBLE PRECISION":
		return dump.Type{Kind: dump.Float, Bits: 64}, nil
	case "CHAR":
		return dump.Type{Kind: dump.String, Fixed: true, Length: number(0)}, nil
	case "VARCHAR":
		return dump.Type{Kind: dump.String, Length: number(0)}, nil
	case "DATE":
		return dump.Type{Kind: dump.Date}, nil
	case "TIMESTAMP":
		return dump.Type{Kind: dump.LocalTimestamp, Precision: 3}, nil
	case "TIMESTAMP WITH LOCAL TIME ZONE":
		return dump.Type{Kind: dump.Timestamp, Precision: 3}, nil
	}
	return dump.Type{Kind: dump.Other}, nil
}

// renderType writes a type in Exasol. It has no integer type of its own, so an integer is a
// DECIMAL with no digits after the point. It has no binary type, no UUID and no array. A
// string is UTF8. A timestamp has no zone, and the session decides what a local one means,
// so a timestamp holds the time in UTC.
func renderType(t dump.Type) (dump.Rendering, error) {
	exact := func(sql string) (dump.Rendering, error) {
		return dump.Rendering{SQL: sql}, nil
	}
	switch t.Kind {
	case dump.Boolean:
		return exact("BOOLEAN")
	case dump.Integer:
		bits := t.Bits
		if bits == 0 {
			bits = 32
		}
		digits := map[int]int{8: 3, 16: 5, 32: 10, 64: 19}[bits]
		if t.Unsigned && bits == 64 {
			digits = 20
		}
		if digits == 0 {
			return dump.Rendering{}, dump.ErrNoEquivalent
		}
		return exact("DECIMAL(" + strconv.Itoa(digits) + ", 0)")
	case dump.Float:
		return exact("DOUBLE PRECISION")
	case dump.Decimal:
		if t.Precision == 0 || t.Precision > maxDecimal {
			return dump.Rendering{}, dump.ErrNoEquivalent
		}
		return exact("DECIMAL(" + strconv.Itoa(t.Precision) + ", " + strconv.Itoa(t.Scale) + ")")
	case dump.String:
		switch {
		case t.Fixed && t.Length > 0 && t.Length <= maxChar:
			return exact("CHAR(" + strconv.Itoa(t.Length) + ") UTF8")
		case t.Length > 0 && t.Length <= maxVarchar:
			return exact("VARCHAR(" + strconv.Itoa(t.Length) + ") UTF8")
		}
		return exact("VARCHAR(" + strconv.Itoa(maxVarchar) + ") UTF8")
	case dump.Date:
		return exact("DATE")
	case dump.LocalTimestamp:
		return exact("TIMESTAMP(" + strconv.Itoa(timePrecision(t.Precision)) + ")")
	case dump.Timestamp:
		return dump.Rendering{SQL: "TIMESTAMP(" + strconv.Itoa(timePrecision(t.Precision)) + ")", Level: dump.Lossy,
			Note: "Exasol reads a timestamp in the zone of the session, so the column holds the time in UTC"}, nil
	case dump.UUID:
		return dump.Rendering{SQL: "CHAR(36) UTF8", Level: dump.Lossy, Note: "the uuid is kept as its text"}, nil
	}
	return dump.Rendering{}, dump.ErrNoEquivalent
}

// timePrecision is the digits of a second that a TIMESTAMP gets. Exasol keeps up to nine.
func timePrecision(p int) int {
	switch {
	case p == 0:
		return 6
	case p > 9:
		return 9
	}
	return p
}
