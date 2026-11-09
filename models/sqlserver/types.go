package sqlserver

import (
	"strconv"
	"strings"

	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/dump"
)

// The type functions of the dump. See D274, D280 and docs/DUMP.md.

func init() {
	dump.RegisterTypes(dbmeta.SQLServer, dump.Types{Parse: parseType, Render: renderType, Text: "nvarchar(max)"})
}

// Limits of SQL Server.
const (
	// maxPrecision is the most digits of a decimal.
	maxPrecision = 38
	// maxVarchar is the most characters of a varchar that is not max.
	maxVarchar = 8000
	// maxNvarchar is the most characters of an nvarchar that is not max.
	maxNvarchar = 4000
	// maxFraction is the most digits of a second of a datetime2, a time or a
	// datetimeoffset.
	maxFraction = 7
)

// parseType reads the data_type that the model gives, such as nvarchar(max),
// decimal(10,2) or datetime2(3). The model leaves out the length of a type that
// has none and the fraction of a type that has the default.
func parseType(text string) (dump.Type, error) {
	unbounded := strings.Contains(strings.ToLower(text), "(max)")
	s, err := dump.ParseSpec(strings.ReplaceAll(strings.ToLower(text), "(max)", ""))
	if err != nil {
		return dump.Type{}, err
	}
	arg := func(i int) int {
		if i < len(s.Args) {
			return s.Args[i]
		}
		return 0
	}
	switch s.Key() {
	case "bit":
		return dump.Type{Kind: dump.Boolean}, nil
	case "tinyint":
		return dump.Type{Kind: dump.Integer, Bits: 8, Unsigned: true}, nil
	case "smallint":
		return dump.Type{Kind: dump.Integer, Bits: 16}, nil
	case "int":
		return dump.Type{Kind: dump.Integer, Bits: 32}, nil
	case "bigint":
		return dump.Type{Kind: dump.Integer, Bits: 64}, nil
	case "real":
		return dump.Type{Kind: dump.Float, Bits: 32}, nil
	case "float":
		return dump.Type{Kind: dump.Float, Bits: 64}, nil
	case "decimal", "numeric":
		return dump.Type{Kind: dump.Decimal, Precision: arg(0), Scale: arg(1)}, nil
	case "money":
		return dump.Type{Kind: dump.Decimal, Precision: 19, Scale: 4}, nil
	case "smallmoney":
		return dump.Type{Kind: dump.Decimal, Precision: 10, Scale: 4}, nil
	case "char", "nchar":
		return dump.Type{Kind: dump.String, Fixed: true, Length: arg(0)}, nil
	case "varchar", "nvarchar":
		if unbounded {
			return dump.Type{Kind: dump.String}, nil
		}
		return dump.Type{Kind: dump.String, Length: arg(0)}, nil
	case "text", "ntext", "xml", "sysname":
		return dump.Type{Kind: dump.String}, nil
	case "binary":
		return dump.Type{Kind: dump.Binary, Fixed: true, Length: arg(0)}, nil
	case "varbinary":
		if unbounded {
			return dump.Type{Kind: dump.Binary}, nil
		}
		return dump.Type{Kind: dump.Binary, Length: arg(0)}, nil
	case "image":
		return dump.Type{Kind: dump.Binary}, nil
	case "date":
		return dump.Type{Kind: dump.Date}, nil
	case "time":
		return dump.Type{Kind: dump.TimeOfDay, Precision: fraction(s, 7)}, nil
	case "datetime2":
		return dump.Type{Kind: dump.LocalTimestamp, Precision: fraction(s, 7)}, nil
	case "datetime":
		return dump.Type{Kind: dump.LocalTimestamp, Precision: 3}, nil
	case "smalldatetime":
		return dump.Type{Kind: dump.LocalTimestamp}, nil
	case "datetimeoffset":
		return dump.Type{Kind: dump.Timestamp, Precision: fraction(s, 7)}, nil
	case "uniqueidentifier":
		return dump.Type{Kind: dump.UUID}, nil
	case "geography", "geometry":
		return dump.Type{Kind: dump.Geometry}, nil
	case "hierarchyid":
		// The value is the binary form of the type. The server reads it back
		// through a cast, and another product keeps the bytes.
		return dump.Type{Kind: dump.Binary}, nil
	}
	return dump.Type{Kind: dump.Other}, nil
}

// fraction returns the digits of a second that a type names, or the default of
// the product when it names none.
func fraction(s dump.Spec, def int) int {
	if len(s.Args) > 0 {
		return s.Args[0]
	}
	return def
}

// renderType writes a type in SQL Server.
func renderType(t dump.Type) (dump.Rendering, error) {
	exact := func(sql string) (dump.Rendering, error) {
		return dump.Rendering{SQL: sql}, nil
	}
	switch t.Kind {
	case dump.Boolean:
		return exact("bit")
	case dump.Integer:
		return integerType(t)
	case dump.Float:
		if t.Bits == 32 {
			return exact("real")
		}
		return exact("float")
	case dump.Decimal:
		return decimalType(t)
	case dump.BigInteger:
		return exact("decimal(38,0)")
	case dump.String:
		return stringType(t)
	case dump.Binary:
		return binaryType(t)
	case dump.Date:
		return exact("date")
	case dump.TimeOfDay:
		return timeType("time", t.Precision)
	case dump.LocalTimestamp:
		return timeType("datetime2", t.Precision)
	case dump.Timestamp:
		return timeType("datetimeoffset", t.Precision)
	case dump.UUID:
		return exact("uniqueidentifier")
	case dump.JSON:
		// SQL Server has no JSON type before 2025 and keeps JSON as text.
		return dump.Rendering{SQL: "nvarchar(max)", Level: dump.Lossy,
			Note: "SQL Server has no json type that every release reads, so the document is written as nvarchar(max)"}, nil
	}
	return dump.Rendering{}, dump.ErrNoEquivalent
}

// integerType names the smallest SQL Server integer that holds every value. Only
// tinyint is unsigned, so the other unsigned types take the next size up.
func integerType(t dump.Type) (dump.Rendering, error) {
	bits := t.Bits
	if bits == 0 {
		bits = 32
	}
	if t.Unsigned {
		switch bits {
		case 8:
			return dump.Rendering{SQL: "tinyint"}, nil
		case 16:
			return dump.Rendering{SQL: "int"}, nil
		case 32:
			return dump.Rendering{SQL: "bigint"}, nil
		}
		return dump.Rendering{SQL: "decimal(20,0)"}, nil
	}
	switch bits {
	case 8, 16:
		return dump.Rendering{SQL: "smallint"}, nil
	case 32:
		return dump.Rendering{SQL: "int"}, nil
	}
	return dump.Rendering{SQL: "bigint"}, nil
}

// decimalType writes decimal(p,s). SQL Server has at most 38 digits, and a
// decimal with no limit has no equivalent.
func decimalType(t dump.Type) (dump.Rendering, error) {
	if t.Precision == 0 {
		return dump.Rendering{}, dump.ErrNoEquivalent
	}
	p, s := t.Precision, t.Scale
	var r dump.Rendering
	if p > maxPrecision {
		p, s = maxPrecision, min(s, maxPrecision)
		r.Level = dump.Lossy
		r.Note = "a decimal of SQL Server has at most " + strconv.Itoa(maxPrecision) + " digits and the source has " +
			strconv.Itoa(t.Precision)
	}
	r.SQL = "decimal(" + strconv.Itoa(p) + "," + strconv.Itoa(s) + ")"
	return r, nil
}

// stringType writes nchar, nvarchar or nvarchar(max). The n types keep every
// character of Unicode, which the char types of a code page do not.
func stringType(t dump.Type) (dump.Rendering, error) {
	switch {
	case t.Fixed && t.Length > 0 && t.Length <= maxNvarchar:
		return dump.Rendering{SQL: "nchar(" + strconv.Itoa(t.Length) + ")"}, nil
	case !t.Fixed && t.Length > 0 && t.Length <= maxNvarchar:
		return dump.Rendering{SQL: "nvarchar(" + strconv.Itoa(t.Length) + ")"}, nil
	}
	return dump.Rendering{SQL: "nvarchar(max)"}, nil
}

// binaryType writes binary, varbinary or varbinary(max).
func binaryType(t dump.Type) (dump.Rendering, error) {
	switch {
	case t.Fixed && t.Length > 0 && t.Length <= maxVarchar:
		return dump.Rendering{SQL: "binary(" + strconv.Itoa(t.Length) + ")"}, nil
	case !t.Fixed && t.Length > 0 && t.Length <= maxVarchar:
		return dump.Rendering{SQL: "varbinary(" + strconv.Itoa(t.Length) + ")"}, nil
	}
	return dump.Rendering{SQL: "varbinary(max)"}, nil
}

// timeType writes time, datetime2 or datetimeoffset with the digits of a second.
// SQL Server keeps seven. A precision of zero in a snapshot means that the source
// states none, so the type has the default of the product and keeps the digits.
func timeType(name string, digits int) (dump.Rendering, error) {
	switch {
	case digits == 0:
		return dump.Rendering{SQL: name}, nil
	case digits > maxFraction:
		return dump.Rendering{SQL: name + "(" + strconv.Itoa(maxFraction) + ")", Level: dump.Lossy,
			Note: "SQL Server keeps " + strconv.Itoa(maxFraction) + " digits of a second and the source has " + strconv.Itoa(digits)}, nil
	}
	return dump.Rendering{SQL: name + "(" + strconv.Itoa(digits) + ")"}, nil
}
