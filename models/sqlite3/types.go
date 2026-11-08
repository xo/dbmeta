package sqlite3

import (
	"strconv"

	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/dump"
)

// The type functions of the dump. See D274 and docs/DUMP.md.
//
// A declared type in SQLite is free text. The text decides an affinity and
// nothing else, so a column can hold any value. This reads the names that people
// write, such as boolean, datetime and varchar(20), and a name it does not know
// is the kind other.

func init() {
	dump.RegisterTypes(dbmeta.SQLite3, dump.Types{Parse: parseType, Render: renderType, Text: "text"})
}

// parseType reads a declared type.
func parseType(text string) (dump.Type, error) {
	s, err := dump.ParseSpec(text)
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
	case "boolean", "bool":
		return dump.Type{Kind: dump.Boolean}, nil
	case "tinyint":
		return dump.Type{Kind: dump.Integer, Bits: 8}, nil
	case "smallint", "int2":
		return dump.Type{Kind: dump.Integer, Bits: 16}, nil
	case "mediumint":
		return dump.Type{Kind: dump.Integer, Bits: 32}, nil
	case "int", "integer", "bigint", "int8":
		return dump.Type{Kind: dump.Integer, Bits: 64}, nil
	case "unsigned big int":
		return dump.Type{Kind: dump.Integer, Bits: 64, Unsigned: true}, nil
	case "real", "double", "double precision", "float":
		return dump.Type{Kind: dump.Float, Bits: 64}, nil
	case "decimal", "numeric":
		return dump.Type{Kind: dump.Decimal, Precision: arg(0), Scale: arg(1)}, nil
	case "char", "character", "nchar", "native character":
		return dump.Type{Kind: dump.String, Fixed: true, Length: arg(0)}, nil
	case "varchar", "character varying", "nvarchar", "varying character":
		return dump.Type{Kind: dump.String, Length: arg(0)}, nil
	case "text", "clob":
		return dump.Type{Kind: dump.String}, nil
	case "blob":
		return dump.Type{Kind: dump.Binary}, nil
	case "date":
		return dump.Type{Kind: dump.Date}, nil
	case "time":
		return dump.Type{Kind: dump.TimeOfDay}, nil
	case "datetime":
		return dump.Type{Kind: dump.LocalTimestamp}, nil
	case "timestamp":
		return dump.Type{Kind: dump.Timestamp}, nil
	case "uuid":
		return dump.Type{Kind: dump.UUID}, nil
	case "json":
		return dump.Type{Kind: dump.JSON}, nil
	}
	return dump.Type{Kind: dump.Other}, nil
}

// renderType writes a declared type. SQLite stores what it is given, so a type
// that SQLite has no name for is refused, and a type that SQLite stores as a
// number where the source is exact is Lossy.
func renderType(t dump.Type) (dump.Rendering, error) {
	exact := func(sql string) (dump.Rendering, error) {
		return dump.Rendering{SQL: sql}, nil
	}
	switch t.Kind {
	case dump.Boolean:
		return exact("boolean")
	case dump.Integer:
		if t.Unsigned && t.Bits == 64 {
			return dump.Rendering{SQL: "integer", Level: dump.Lossy,
				Note: "an integer of SQLite holds 64 bits with a sign, and the source is unsigned"}, nil
		}
		return exact("integer")
	case dump.Float:
		return exact("real")
	case dump.Decimal:
		return dump.Rendering{SQL: "numeric", Level: dump.Lossy,
			Note: "SQLite stores a number with a fraction as a real, which is not exact"}, nil
	case dump.String:
		switch {
		case t.Fixed && t.Length > 0:
			return exact("char(" + strconv.Itoa(t.Length) + ")")
		case t.Length > 0:
			return exact("varchar(" + strconv.Itoa(t.Length) + ")")
		}
		return exact("text")
	case dump.Binary:
		return exact("blob")
	case dump.Date:
		return exact("date")
	case dump.TimeOfDay:
		return exact("time")
	case dump.LocalTimestamp:
		return exact("datetime")
	case dump.Timestamp:
		return exact("timestamp")
	case dump.JSON:
		return exact("json")
	}
	return dump.Rendering{}, dump.ErrNoEquivalent
}
