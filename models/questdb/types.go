package questdb

import (
	"strconv"

	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/dump"
)

// The type functions of the dump. See D274, D302 and docs/DUMP.md.
//
// QuestDB sends the type of PostgreSQL for each column, so the model gives
// character varying for a STRING, a VARCHAR, a SYMBOL, a UUID and an IPv4, and
// timestamp without time zone for a DATE and a TIMESTAMP. A column of those types is
// read as the kind of the text or the timestamp that it arrives as.

func init() {
	dump.RegisterTypes(dbmeta.QuestDB, dump.Types{Parse: parseType, Render: renderType, Text: "varchar"})
}

// parseType reads the data_type of the model, which is the type that the wire
// protocol of PostgreSQL names.
func parseType(text string) (dump.Type, error) {
	s, err := dump.ParseSpec(text)
	if err != nil {
		return dump.Type{}, err
	}
	switch s.Key() {
	case "boolean":
		return dump.Type{Kind: dump.Boolean}, nil
	case "smallint":
		return dump.Type{Kind: dump.Integer, Bits: 16}, nil
	case "integer":
		return dump.Type{Kind: dump.Integer, Bits: 32}, nil
	case "bigint":
		return dump.Type{Kind: dump.Integer, Bits: 64}, nil
	case "real":
		return dump.Type{Kind: dump.Float, Bits: 32}, nil
	case "double precision":
		return dump.Type{Kind: dump.Float, Bits: 64}, nil
	case "character varying", "character", "varchar", "text":
		return dump.Type{Kind: dump.String}, nil
	case "timestamp without time zone", "timestamp":
		return dump.Type{Kind: dump.LocalTimestamp, Precision: 6}, nil
	case "bytea":
		return dump.Type{Kind: dump.Binary}, nil
	}
	return dump.Type{Kind: dump.Other}, nil
}

// renderType writes a type in QuestDB. Its BOOLEAN, BYTE and SHORT have no NULL, so a
// NULL is written as false or 0, and that is Lossy. Its INT and LONG use the least
// value as NULL, so that one value is NULL. It has no time, no interval, no JSON and
// no binary literal, so a column of those kinds has no equivalent and takes the Text
// level when the client allows it.
func renderType(t dump.Type) (dump.Rendering, error) {
	exact := func(sql string) (dump.Rendering, error) {
		return dump.Rendering{SQL: sql}, nil
	}
	lossy := func(sql, note string) (dump.Rendering, error) {
		return dump.Rendering{SQL: sql, Level: dump.Lossy, Note: note}, nil
	}
	switch t.Kind {
	case dump.Boolean:
		return lossy("boolean", "QuestDB has no NULL for a boolean, so a NULL is written as false")
	case dump.Integer:
		return integerType(t)
	case dump.Float:
		if t.Bits == 32 {
			return exact("float")
		}
		return exact("double")
	case dump.Decimal:
		if t.Precision == 0 || t.Precision > maxDecimalPrecision {
			return dump.Rendering{}, dump.ErrNoEquivalent
		}
		return exact("decimal(" + strconv.Itoa(t.Precision) + "," + strconv.Itoa(t.Scale) + ")")
	case dump.String:
		return exact("varchar")
	case dump.Date:
		return exact("date")
	case dump.LocalTimestamp, dump.Timestamp:
		if t.Precision > 6 {
			return exact("timestamp_ns")
		}
		return exact("timestamp")
	case dump.UUID:
		return exact("uuid")
	}
	return dump.Rendering{}, dump.ErrNoEquivalent
}

// maxDecimalPrecision is the most digits of a DECIMAL.
const maxDecimalPrecision = 76

// integerType names the smallest type that holds every value of the integer.
// QuestDB has no unsigned integer, so an unsigned one takes the next size up, and an
// unsigned 64 bit integer is a decimal of 20 digits.
func integerType(t dump.Type) (dump.Rendering, error) {
	bits := t.Bits
	if bits == 0 {
		bits = 32
	}
	short := dump.Rendering{SQL: "short", Level: dump.Lossy, Note: "QuestDB has no NULL for a short, so a NULL is written as 0"}
	if t.Unsigned {
		switch bits {
		case 8:
			return short, nil
		case 16:
			return dump.Rendering{SQL: "int"}, nil
		case 32:
			return dump.Rendering{SQL: "long"}, nil
		}
		return dump.Rendering{SQL: "decimal(20,0)"}, nil
	}
	switch bits {
	case 8:
		return dump.Rendering{SQL: "byte", Level: dump.Lossy, Note: "QuestDB has no NULL for a byte, so a NULL is written as 0"}, nil
	case 16:
		return short, nil
	case 32:
		return dump.Rendering{SQL: "int"}, nil
	}
	return dump.Rendering{SQL: "long"}, nil
}
