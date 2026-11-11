package arangodb

import (
	"strings"

	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/dump"
	"github.com/xo/dbmeta/models/internal/typeparse"
)

// The type functions of the dump. ArangoDB is only a source, so no type has an equivalent.
// See D274, D305 and docs/DUMP.md.

func init() {
	dump.RegisterTypes(dbmeta.ArangoDB, dump.Types{Parse: parseType, Render: typeparse.NoRender, Text: "text"})
}

// parseType reads the JSON schema type that the model gives, such as string or integer.
// Where a rule lists several types, the names are joined by a comma, and the column has
// no one type.
func parseType(text string) (dump.Type, error) {
	switch strings.ToLower(strings.TrimSpace(text)) {
	case "boolean":
		return dump.Type{Kind: dump.Boolean}, nil
	case "integer":
		return dump.Type{Kind: dump.Integer, Bits: 64}, nil
	case "number":
		return dump.Type{Kind: dump.Float, Bits: 64}, nil
	case "string":
		return dump.Type{Kind: dump.String}, nil
	case "array":
		return dump.Type{Kind: dump.Array, Elem: &dump.Type{Kind: dump.Other}}, nil
	case "object":
		return dump.Type{Kind: dump.JSON}, nil
	}
	return dump.Type{Kind: dump.Other}, nil
}
