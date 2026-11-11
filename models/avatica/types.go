package avatica

import (
	"strconv"
	"strings"

	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/dump"
)

// The type functions of the dump. See D274, D304 and docs/DUMP.md.
//
// The standalone Avatica server stands in front of HSQLDB, so the types are the ones of
// HSQLDB.

func init() {
	dump.RegisterTypes(dbmeta.Avatica, dump.Types{Parse: parseType, Render: renderType, Text: "LONGVARCHAR"})
}

// maxDecimalPrecision is the most digits that this module writes for a DECIMAL.
const maxDecimalPrecision = 1000

// avaticaZone is why a time with a zone is written without one. The standalone server of
// Avatica cannot send a value of the types of Java that carry a zone, so a client cannot read
// the column back.
const avaticaZone = "the standalone Avatica server cannot send a time with a zone, so the column holds the time in UTC"

// parseType reads what the model gives, which it assembles from INFORMATION_SCHEMA, such as
// CHARACTER VARYING(20), DECIMAL(10,2), DOUBLE PRECISION or TIMESTAMP WITH TIME ZONE.
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
		return dump.Type{Kind: dump.Integer, Bits: 8}, nil
	case "SMALLINT":
		return dump.Type{Kind: dump.Integer, Bits: 16}, nil
	case "INTEGER":
		return dump.Type{Kind: dump.Integer, Bits: 32}, nil
	case "BIGINT":
		return dump.Type{Kind: dump.Integer, Bits: 64}, nil
	case "DOUBLE PRECISION", "DOUBLE":
		return dump.Type{Kind: dump.Float, Bits: 64}, nil
	case "REAL":
		return dump.Type{Kind: dump.Float, Bits: 32}, nil
	case "DECIMAL", "NUMERIC":
		return dump.Type{Kind: dump.Decimal, Precision: number(0), Scale: number(1)}, nil
	case "CHARACTER":
		return dump.Type{Kind: dump.String, Fixed: true, Length: number(0)}, nil
	case "CHARACTER VARYING":
		return dump.Type{Kind: dump.String, Length: number(0)}, nil
	case "CHARACTER LARGE OBJECT":
		return dump.Type{Kind: dump.String}, nil
	case "BINARY":
		return dump.Type{Kind: dump.Binary, Fixed: true, Length: number(0)}, nil
	case "BINARY VARYING", "VARBINARY":
		return dump.Type{Kind: dump.Binary, Length: number(0)}, nil
	case "BINARY LARGE OBJECT":
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
	case "UUID":
		return dump.Type{Kind: dump.UUID}, nil
	}
	return dump.Type{Kind: dump.Other}, nil
}

// renderType writes a type in HSQLDB. It has no unsigned integer, no JSON and no array that
// the catalog gives the element of, so an array is the kind other and has no equivalent.
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
		if t.Unsigned {
			bits *= 2
		}
		switch {
		case bits <= 8:
			return exact("TINYINT")
		case bits <= 16:
			return exact("SMALLINT")
		case bits <= 32:
			return exact("INTEGER")
		case bits <= 64:
			return exact("BIGINT")
		}
		return dump.Rendering{}, dump.ErrNoEquivalent
	case dump.Float:
		return exact("DOUBLE")
	case dump.Decimal:
		if t.Precision == 0 || t.Precision > maxDecimalPrecision {
			return dump.Rendering{}, dump.ErrNoEquivalent
		}
		return exact("DECIMAL(" + strconv.Itoa(t.Precision) + ", " + strconv.Itoa(t.Scale) + ")")
	case dump.String:
		switch {
		case t.Length == 0:
			return exact("LONGVARCHAR")
		case t.Fixed:
			return exact("CHAR(" + strconv.Itoa(t.Length) + ")")
		}
		return exact("VARCHAR(" + strconv.Itoa(t.Length) + ")")
	case dump.Binary:
		switch {
		case t.Length == 0:
			return exact("LONGVARBINARY")
		case t.Fixed:
			return exact("BINARY(" + strconv.Itoa(t.Length) + ")")
		}
		return exact("VARBINARY(" + strconv.Itoa(t.Length) + ")")
	case dump.Date:
		return exact("DATE")
	case dump.TimeOfDay:
		return exact("TIME")
	case dump.TimeOfDayOffset:
		return dump.Rendering{SQL: "TIME", Level: dump.Lossy, Note: avaticaZone}, nil
	case dump.LocalTimestamp:
		return exact("TIMESTAMP(6)")
	case dump.Timestamp:
		return dump.Rendering{SQL: "TIMESTAMP(6)", Level: dump.Lossy, Note: avaticaZone}, nil
	case dump.UUID:
		return exact("UUID")
	}
	return dump.Rendering{}, dump.ErrNoEquivalent
}
