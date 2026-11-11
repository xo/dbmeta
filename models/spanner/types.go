package spanner

import (
	"strconv"
	"strings"

	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/dump"
)

// The type functions of the dump. See D274, D303 and docs/DUMP.md.

func init() {
	dump.RegisterTypes(dbmeta.Spanner, dump.Types{Parse: parseType, Render: renderType, Text: "STRING(MAX)"})
}

// maxStringLength is the most characters of a STRING with a length. A longer one is MAX.
const maxStringLength = 2621440

// maxBytesLength is the most bytes of a BYTES with a length.
const maxBytesLength = 10485760

// parseType reads spanner_type, such as STRING(MAX), BYTES(20), NUMERIC or ARRAY<INT64>.
func parseType(text string) (dump.Type, error) {
	text = strings.TrimSpace(text)
	upper := strings.ToUpper(text)
	if strings.HasPrefix(upper, "ARRAY<") && strings.HasSuffix(text, ">") {
		elem, err := parseType(text[len("ARRAY<") : len(text)-1])
		if err != nil {
			return dump.Type{}, err
		}
		return dump.Type{Kind: dump.Array, Elem: &elem}, nil
	}
	name, arg := upper, ""
	if i := strings.IndexByte(upper, '('); i >= 0 && strings.HasSuffix(upper, ")") {
		name, arg = upper[:i], upper[i+1:len(upper)-1]
	}
	length := 0
	if n, err := strconv.Atoi(arg); err == nil {
		length = n
	}
	switch name {
	case "BOOL":
		return dump.Type{Kind: dump.Boolean}, nil
	case "INT64":
		return dump.Type{Kind: dump.Integer, Bits: 64}, nil
	case "FLOAT32":
		return dump.Type{Kind: dump.Float, Bits: 32}, nil
	case "FLOAT64":
		return dump.Type{Kind: dump.Float, Bits: 64}, nil
	case "NUMERIC":
		return dump.Type{Kind: dump.Decimal, Precision: 38, Scale: 9}, nil
	case "STRING":
		return dump.Type{Kind: dump.String, Length: length}, nil
	case "BYTES":
		return dump.Type{Kind: dump.Binary, Length: length}, nil
	case "DATE":
		return dump.Type{Kind: dump.Date}, nil
	case "TIMESTAMP":
		return dump.Type{Kind: dump.Timestamp, Precision: 9}, nil
	case "JSON":
		return dump.Type{Kind: dump.JSON}, nil
	case "UUID":
		return dump.Type{Kind: dump.UUID}, nil
	}
	return dump.Type{Kind: dump.Other}, nil
}

// renderType writes a type in Spanner. It has one integer, INT64, and a NUMERIC of 38
// digits with 9 after the point. A TIMESTAMP is an instant in UTC, so a timestamp with no
// zone is written as the instant that has the same clock in UTC. It has no time of day
// and no interval that every release reads, so those have no equivalent.
func renderType(t dump.Type) (dump.Rendering, error) {
	exact := func(sql string) (dump.Rendering, error) {
		return dump.Rendering{SQL: sql}, nil
	}
	switch t.Kind {
	case dump.Boolean:
		return exact("BOOL")
	case dump.Integer:
		if t.Unsigned && t.Bits >= 64 {
			return exact("NUMERIC")
		}
		return exact("INT64")
	case dump.Float:
		if t.Bits == 32 {
			return exact("FLOAT32")
		}
		return exact("FLOAT64")
	case dump.Decimal:
		if t.Precision == 0 || t.Scale > 9 || t.Precision-t.Scale > 29 {
			return dump.Rendering{}, dump.ErrNoEquivalent
		}
		return exact("NUMERIC")
	case dump.String:
		switch {
		case t.Fixed && t.Length > 0 && t.Length <= maxStringLength:
			return dump.Rendering{SQL: "STRING(" + strconv.Itoa(t.Length) + ")", Level: dump.Lossy,
				Note: "a fixed length string is written as a STRING, which does not pad"}, nil
		case t.Length > 0 && t.Length <= maxStringLength:
			return exact("STRING(" + strconv.Itoa(t.Length) + ")")
		}
		return exact("STRING(MAX)")
	case dump.Binary:
		if t.Length > 0 && t.Length <= maxBytesLength {
			return exact("BYTES(" + strconv.Itoa(t.Length) + ")")
		}
		return exact("BYTES(MAX)")
	case dump.Date:
		return exact("DATE")
	case dump.LocalTimestamp, dump.Timestamp:
		return exact("TIMESTAMP")
	case dump.JSON:
		return exact("JSON")
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
		r.SQL = "ARRAY<" + r.SQL + ">"
		return r, nil
	}
	return dump.Rendering{}, dump.ErrNoEquivalent
}
