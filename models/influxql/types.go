package influxql

import (
	"strings"

	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/dump"
	"github.com/xo/dbmeta/models/internal/typeparse"
)

// The type functions of the dump. InfluxQL is only a source, so no type has an equivalent.
// See D274, D305 and docs/DUMP.md.

func init() {
	dump.RegisterTypes(dbmeta.InfluxQL, dump.Types{Parse: parseType, Render: typeparse.NoRender, Text: "text"})
}

// parseType reads the type that the model gives: timestamp for time, tag for a tag, and
// float, integer, unsigned, string or boolean for a field.
func parseType(text string) (dump.Type, error) {
	switch strings.ToLower(strings.TrimSpace(text)) {
	case "timestamp":
		return dump.Type{Kind: dump.LocalTimestamp, Precision: 9}, nil
	case "tag", "string":
		return dump.Type{Kind: dump.String}, nil
	case "float":
		return dump.Type{Kind: dump.Float, Bits: 64}, nil
	case "integer":
		return dump.Type{Kind: dump.Integer, Bits: 64}, nil
	case "unsigned":
		return dump.Type{Kind: dump.Integer, Bits: 64, Unsigned: true}, nil
	case "boolean":
		return dump.Type{Kind: dump.Boolean}, nil
	}
	return dump.Type{Kind: dump.Other}, nil
}
