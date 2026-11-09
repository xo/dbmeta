package oracle

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/dump"
)

// The type functions of the dump. See D274, D280 and docs/DUMP.md.

func init() {
	dump.RegisterTypes(dbmeta.Oracle, dump.Types{Parse: parseType, Render: renderType, Text: "CLOB"})
}

// Limits of Oracle with the extended data types off, which is the default.
const (
	// maxVarchar is the most bytes of a VARCHAR2.
	maxVarchar = 4000
	// maxRaw is the most bytes of a RAW.
	maxRaw = 2000
	// maxPrecision is the most digits of a NUMBER.
	maxPrecision = 38
	// maxFraction is the most digits of a second of a TIMESTAMP.
	maxFraction = 9
)

var (
	// lengthType is a type with one length, such as VARCHAR2(200 CHAR).
	lengthType = regexp.MustCompile(`^([A-Z0-9 ]+?)\s*\(\s*(\d+)\s*(BYTE|CHAR)?\s*\)$`)
	// numberType is NUMBER, NUMBER(p), NUMBER(p,s) and NUMBER(*,s).
	numberType = regexp.MustCompile(`^NUMBER(?:\(\s*(\*|\d+)\s*(?:,\s*(-?\d+)\s*)?\))?$`)
	// timestampType is TIMESTAMP with a fraction and a zone.
	timestampType = regexp.MustCompile(`^TIMESTAMP(?:\((\d+)\))?(?: (WITH TIME ZONE|WITH LOCAL TIME ZONE))?$`)
	// intervalType is INTERVAL YEAR(p) TO MONTH or INTERVAL DAY(p) TO SECOND(s).
	intervalType = regexp.MustCompile(`^INTERVAL (YEAR|DAY)(?:\((\d+)\))? TO (MONTH|SECOND)(?:\((\d+)\))?$`)
)

// parseType reads the data_type that the model gives. It is the declared type,
// with the length, the precision and the scale, and a type that a person made
// carries its owner in quotes.
func parseType(text string) (dump.Type, error) {
	up := strings.ToUpper(strings.Join(strings.Fields(text), " "))
	atoi := func(s string) int {
		n, _ := strconv.Atoi(s)
		return n
	}
	if m := numberType.FindStringSubmatch(up); m != nil {
		return numberKind(m[1], m[2], strings.Contains(up, "("), strings.Contains(up, ",")), nil
	}
	if m := timestampType.FindStringSubmatch(up); m != nil {
		digits := 6
		if m[1] != "" {
			digits = atoi(m[1])
		}
		if m[2] != "" {
			return dump.Type{Kind: dump.Timestamp, Precision: digits}, nil
		}
		return dump.Type{Kind: dump.LocalTimestamp, Precision: digits}, nil
	}
	if m := intervalType.FindStringSubmatch(up); m != nil {
		return dump.Type{Kind: dump.Interval, Precision: atoi(m[4])}, nil
	}
	if m := lengthType.FindStringSubmatch(up); m != nil {
		n := atoi(m[2])
		switch m[1] {
		case "VARCHAR2", "NVARCHAR2", "VARCHAR":
			return dump.Type{Kind: dump.String, Length: n}, nil
		case "CHAR", "NCHAR":
			return dump.Type{Kind: dump.String, Fixed: true, Length: n}, nil
		case "RAW":
			return dump.Type{Kind: dump.Binary, Length: n}, nil
		case "FLOAT":
			if n <= 63 {
				return dump.Type{Kind: dump.Float, Bits: 32}, nil
			}
			return dump.Type{Kind: dump.Float, Bits: 64}, nil
		case "UROWID":
			return dump.Type{Kind: dump.String, Length: n}, nil
		}
	}
	switch up {
	case "VARCHAR2", "NVARCHAR2", "LONG", "CLOB", "NCLOB":
		return dump.Type{Kind: dump.String}, nil
	case "CHAR", "NCHAR":
		return dump.Type{Kind: dump.String, Fixed: true, Length: 1}, nil
	case "RAW", "LONG RAW", "BLOB":
		return dump.Type{Kind: dump.Binary}, nil
	case "FLOAT":
		return dump.Type{Kind: dump.Float, Bits: 64}, nil
	case "BINARY_FLOAT":
		return dump.Type{Kind: dump.Float, Bits: 32}, nil
	case "BINARY_DOUBLE":
		return dump.Type{Kind: dump.Float, Bits: 64}, nil
	case "DATE":
		return dump.Type{Kind: dump.LocalTimestamp}, nil
	case "BOOLEAN":
		return dump.Type{Kind: dump.Boolean}, nil
	case "JSON":
		return dump.Type{Kind: dump.JSON}, nil
	case "ROWID", "UROWID":
		return dump.Type{Kind: dump.String, Length: 18}, nil
	}
	return dump.Type{Kind: dump.Other}, nil
}

// numberKind reads NUMBER with its precision and scale. A NUMBER with no digits
// at all holds any value, which is a decimal with no limit.
func numberKind(precision, scale string, bracket, comma bool) dump.Type {
	if !bracket {
		return dump.Type{Kind: dump.Decimal}
	}
	p, _ := strconv.Atoi(precision)
	s, _ := strconv.Atoi(scale)
	if precision == "*" {
		p = 0
	}
	if !comma || s == 0 {
		switch {
		case p == 0:
			// NUMBER(*,0) is what INTEGER is.
			return dump.Type{Kind: dump.BigInteger}
		case p <= 4:
			return dump.Type{Kind: dump.Integer, Bits: 16}
		case p <= 9:
			return dump.Type{Kind: dump.Integer, Bits: 32}
		case p <= 18:
			return dump.Type{Kind: dump.Integer, Bits: 64}
		}
		return dump.Type{Kind: dump.BigInteger}
	}
	return dump.Type{Kind: dump.Decimal, Precision: p, Scale: s}
}

// renderType writes a type in Oracle.
func renderType(t dump.Type) (dump.Rendering, error) {
	exact := func(sql string) (dump.Rendering, error) {
		return dump.Rendering{SQL: sql}, nil
	}
	switch t.Kind {
	case dump.Boolean:
		return dump.Rendering{SQL: "NUMBER(1)", Level: dump.Lossy,
			Note: "a release of Oracle before 23ai has no boolean, so the column is a NUMBER(1) that holds 0 or 1"}, nil
	case dump.Integer:
		switch t.Bits {
		case 8, 16:
			return exact("NUMBER(5)")
		case 32, 0:
			return exact("NUMBER(10)")
		}
		return exact("NUMBER(19)")
	case dump.BigInteger:
		return exact("NUMBER(38)")
	case dump.Float:
		if t.Bits == 32 {
			return exact("BINARY_FLOAT")
		}
		return exact("BINARY_DOUBLE")
	case dump.Decimal:
		return decimalType(t)
	case dump.String:
		return stringType(t)
	case dump.Binary:
		if t.Length > 0 && t.Length <= maxRaw {
			return exact("RAW(" + strconv.Itoa(t.Length) + ")")
		}
		return exact("BLOB")
	case dump.Date:
		return exact("DATE")
	case dump.LocalTimestamp:
		return fractionType("TIMESTAMP", "", t.Precision)
	case dump.Timestamp:
		return fractionType("TIMESTAMP", " WITH TIME ZONE", t.Precision)
	case dump.UUID:
		return exact("RAW(16)")
	case dump.JSON:
		return dump.Rendering{SQL: "CLOB", Level: dump.Lossy,
			Note: "a release of Oracle before 21c has no json type, so the document is written as a CLOB"}, nil
	}
	return dump.Rendering{}, dump.ErrNoEquivalent
}

// decimalType writes NUMBER(p,s). A decimal with no limit is a NUMBER.
func decimalType(t dump.Type) (dump.Rendering, error) {
	if t.Precision == 0 {
		return dump.Rendering{SQL: "NUMBER"}, nil
	}
	p, s := t.Precision, t.Scale
	var r dump.Rendering
	if p > maxPrecision {
		p = maxPrecision
		s = min(s, maxPrecision)
		r.Level = dump.Lossy
		r.Note = "a NUMBER of Oracle has at most 38 digits, and the source has " + strconv.Itoa(t.Precision)
	}
	r.SQL = "NUMBER(" + strconv.Itoa(p) + "," + strconv.Itoa(s) + ")"
	return r, nil
}

// stringType writes CHAR(n), VARCHAR2(n) or CLOB. The length is in characters, as
// the source counts it.
func stringType(t dump.Type) (dump.Rendering, error) {
	if len(t.Values) > 0 {
		return dump.Rendering{SQL: "VARCHAR2(" + strconv.Itoa(longest(t.Values)) + " CHAR)", Level: dump.Lossy,
			Note: "Oracle has no enumerated type, so the column is a VARCHAR2"}, nil
	}
	switch {
	case t.Fixed && t.Length > 0 && t.Length <= 2000:
		return dump.Rendering{SQL: "CHAR(" + strconv.Itoa(t.Length) + " CHAR)"}, nil
	case !t.Fixed && t.Length > 0 && t.Length <= maxVarchar/4:
		return dump.Rendering{SQL: "VARCHAR2(" + strconv.Itoa(t.Length) + " CHAR)"}, nil
	}
	return dump.Rendering{SQL: "CLOB"}, nil
}

// longest is the length of the longest value.
func longest(values []string) int {
	n := 1
	for _, v := range values {
		n = max(n, len([]rune(v)))
	}
	return n
}

// fractionType writes TIMESTAMP with the digits of a second, which Oracle keeps up
// to nine.
func fractionType(name, zone string, digits int) (dump.Rendering, error) {
	if digits > maxFraction {
		return dump.Rendering{SQL: name + "(" + strconv.Itoa(maxFraction) + ")" + zone, Level: dump.Lossy,
			Note: "Oracle keeps nine digits of a second and the source has " + strconv.Itoa(digits)}, nil
	}
	return dump.Rendering{SQL: name + "(" + strconv.Itoa(digits) + ")" + zone}, nil
}
