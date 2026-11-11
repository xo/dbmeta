package databend

import (
	"strconv"
	"strings"

	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/dump"
)

// The type functions of the dump. See D274, D304 and docs/DUMP.md.

func init() {
	dump.RegisterTypes(dbmeta.Databend, dump.Types{Parse: parseType, Render: renderType, Text: "VARCHAR"})
}

// maxDecimalPrecision is the most digits of a DECIMAL.
const maxDecimalPrecision = 76

// parseType reads what system.columns gives, such as INT, DECIMAL(10, 2), TINYINT UNSIGNED or
// ARRAY(INT32). A trailing NULL says that the column can be NULL and is not part of the type.
func parseType(text string) (dump.Type, error) {
	text = strings.ToUpper(strings.TrimSpace(text))
	text = strings.TrimSpace(strings.TrimSuffix(text, " NULL"))
	if inner, ok := strings.CutPrefix(text, "ARRAY("); ok && strings.HasSuffix(inner, ")") {
		elem, err := parseType(inner[:len(inner)-1])
		if err != nil {
			return dump.Type{}, err
		}
		return dump.Type{Kind: dump.Array, Elem: &elem}, nil
	}
	unsigned := false
	if base, ok := strings.CutSuffix(text, " UNSIGNED"); ok {
		text, unsigned = base, true
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
	integer := func(bits int) (dump.Type, error) {
		return dump.Type{Kind: dump.Integer, Bits: bits, Unsigned: unsigned}, nil
	}
	switch name {
	case "BOOLEAN", "BOOL":
		return dump.Type{Kind: dump.Boolean}, nil
	case "TINYINT", "INT8":
		return integer(8)
	case "SMALLINT", "INT16":
		return integer(16)
	case "INT", "INTEGER", "INT32":
		return integer(32)
	case "BIGINT", "INT64":
		return integer(64)
	case "FLOAT", "FLOAT32":
		return dump.Type{Kind: dump.Float, Bits: 32}, nil
	case "DOUBLE", "FLOAT64":
		return dump.Type{Kind: dump.Float, Bits: 64}, nil
	case "DECIMAL":
		return dump.Type{Kind: dump.Decimal, Precision: number(0), Scale: number(1)}, nil
	case "VARCHAR", "STRING", "CHAR", "TEXT":
		return dump.Type{Kind: dump.String}, nil
	case "BINARY", "VARBINARY":
		return dump.Type{Kind: dump.Binary}, nil
	case "DATE":
		return dump.Type{Kind: dump.Date}, nil
	case "TIMESTAMP", "DATETIME", "TIMESTAMPTZ", "TIMESTAMP_TZ":
		return dump.Type{Kind: dump.Timestamp}, nil
	case "VARIANT", "JSON":
		return dump.Type{Kind: dump.JSON}, nil
	}
	return dump.Type{Kind: dump.Other}, nil
}

// renderType writes a type in Databend. Its TIMESTAMP is an instant with six digits of a
// second, so a timestamp with no zone is read as UTC. It has no time of day, no interval and
// no uuid.
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
		name := map[int]string{8: "TINYINT", 16: "SMALLINT", 32: "INT", 64: "BIGINT"}[bits]
		if name == "" {
			return dump.Rendering{}, dump.ErrNoEquivalent
		}
		if t.Unsigned {
			name += " UNSIGNED"
		}
		return exact(name)
	case dump.Float:
		if t.Bits == 32 {
			return exact("FLOAT")
		}
		return exact("DOUBLE")
	case dump.Decimal:
		if t.Precision == 0 || t.Precision > maxDecimalPrecision {
			return dump.Rendering{}, dump.ErrNoEquivalent
		}
		return exact("DECIMAL(" + strconv.Itoa(t.Precision) + ", " + strconv.Itoa(t.Scale) + ")")
	case dump.String:
		return exact("VARCHAR")
	case dump.Binary:
		return exact("BINARY")
	case dump.Date:
		return exact("DATE")
	case dump.LocalTimestamp:
		return dump.Rendering{SQL: "TIMESTAMP", Level: dump.Lossy,
			Note: "a TIMESTAMP of Databend is an instant, so a time with no zone is read as UTC"}, nil
	case dump.Timestamp:
		return exact("TIMESTAMP")
	case dump.JSON:
		return exact("VARIANT")
	case dump.Array:
		if t.Elem == nil {
			return dump.Rendering{}, dump.ErrNoEquivalent
		}
		r, err := renderType(*t.Elem)
		if err != nil {
			return dump.Rendering{}, err
		}
		r.SQL = "ARRAY(" + r.SQL + ")"
		return r, nil
	}
	return dump.Rendering{}, dump.ErrNoEquivalent
}
