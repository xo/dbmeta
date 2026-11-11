package hana

import (
	"strconv"
	"strings"

	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/dump"
)

// The type functions of the dump. See D274, D304 and docs/DUMP.md.

func init() {
	dump.RegisterTypes(dbmeta.HANA, dump.Types{Parse: parseType, Render: renderType, Text: "NCLOB"})
}

// The limits of SAP HANA.
const (
	// maxDecimal is the most digits of a DECIMAL.
	maxDecimal = 38
	// maxNVarchar is the most characters of an NVARCHAR.
	maxNVarchar = 5000
	// maxNChar is the most characters of an NCHAR.
	maxNChar = 2000
	// maxVarbinary is the most bytes of a VARBINARY.
	maxVarbinary = 5000
)

// parseType reads what the model assembles, such as DECIMAL(10,2), NVARCHAR(20), INTEGER or
// TIMESTAMP. A TINYINT of HANA has no sign.
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
	case "TINYINT":
		return dump.Type{Kind: dump.Integer, Bits: 8, Unsigned: true}, nil
	case "SMALLINT":
		return dump.Type{Kind: dump.Integer, Bits: 16}, nil
	case "INTEGER":
		return dump.Type{Kind: dump.Integer, Bits: 32}, nil
	case "BIGINT":
		return dump.Type{Kind: dump.Integer, Bits: 64}, nil
	case "REAL":
		return dump.Type{Kind: dump.Float, Bits: 32}, nil
	case "DOUBLE", "FLOAT":
		return dump.Type{Kind: dump.Float, Bits: 64}, nil
	case "DECIMAL":
		return dump.Type{Kind: dump.Decimal, Precision: number(0), Scale: number(1)}, nil
	case "CHAR", "NCHAR":
		return dump.Type{Kind: dump.String, Fixed: true, Length: number(0)}, nil
	case "VARCHAR", "NVARCHAR", "ALPHANUM", "SHORTTEXT":
		return dump.Type{Kind: dump.String, Length: number(0)}, nil
	case "CLOB", "NCLOB", "TEXT":
		return dump.Type{Kind: dump.String}, nil
	case "VARBINARY":
		return dump.Type{Kind: dump.Binary, Length: number(0)}, nil
	case "BLOB":
		return dump.Type{Kind: dump.Binary}, nil
	case "DATE":
		return dump.Type{Kind: dump.Date}, nil
	case "TIME":
		return dump.Type{Kind: dump.TimeOfDay}, nil
	case "SECONDDATE":
		return dump.Type{Kind: dump.LocalTimestamp, Precision: 0}, nil
	case "TIMESTAMP":
		return dump.Type{Kind: dump.LocalTimestamp, Precision: 7}, nil
	}
	return dump.Type{Kind: dump.Other}, nil
}

// renderType writes a type in SAP HANA. A string is NVARCHAR, which holds Unicode. HANA has
// no timestamp with a zone, no interval, no UUID and no JSON type.
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
		switch {
		case t.Unsigned && bits == 8:
			return exact("TINYINT")
		case t.Unsigned && bits >= 64:
			return exact("DECIMAL(20, 0)")
		case t.Unsigned:
			bits *= 2
		}
		switch {
		case bits <= 16:
			return exact("SMALLINT")
		case bits <= 32:
			return exact("INTEGER")
		}
		return exact("BIGINT")
	case dump.Float:
		if t.Bits == 32 {
			return exact("REAL")
		}
		return exact("DOUBLE")
	case dump.Decimal:
		if t.Precision == 0 || t.Precision > maxDecimal {
			return dump.Rendering{}, dump.ErrNoEquivalent
		}
		return exact("DECIMAL(" + strconv.Itoa(t.Precision) + ", " + strconv.Itoa(t.Scale) + ")")
	case dump.String:
		switch {
		case t.Fixed && t.Length > 0 && t.Length <= maxNChar:
			return exact("NCHAR(" + strconv.Itoa(t.Length) + ")")
		case t.Length > 0 && t.Length <= maxNVarchar:
			return exact("NVARCHAR(" + strconv.Itoa(t.Length) + ")")
		}
		return exact("NCLOB")
	case dump.Binary:
		if t.Length > 0 && t.Length <= maxVarbinary {
			return exact("VARBINARY(" + strconv.Itoa(t.Length) + ")")
		}
		return exact("BLOB")
	case dump.Date:
		return exact("DATE")
	case dump.TimeOfDay:
		return exact("TIME")
	case dump.LocalTimestamp:
		return exact("TIMESTAMP")
	case dump.Timestamp:
		return dump.Rendering{SQL: "TIMESTAMP", Level: dump.Lossy,
			Note: "SAP HANA has no timestamp with a zone, so the column holds the time in UTC"}, nil
	case dump.UUID:
		return dump.Rendering{SQL: "NVARCHAR(36)", Level: dump.Lossy, Note: "the uuid is kept as its text"}, nil
	}
	return dump.Rendering{}, dump.ErrNoEquivalent
}
