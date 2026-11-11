package surrealdb

import (
	"strings"

	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/dump"
	"github.com/xo/dbmeta/models/internal/typeparse"
)

// The type functions of the dump. SurrealDB is only a source, so no type has an equivalent.
// See D274, D305 and docs/DUMP.md.

func init() {
	dump.RegisterTypes(dbmeta.SurrealDB, dump.Types{Parse: parseType, Render: typeparse.NoRender, Text: "text"})
}

// parseType reads the TYPE of a field as INFO writes it, such as int, option<string>,
// array<int> or record<person>. An option holds the type inside it and a value that can
// be NONE.
func parseType(text string) (dump.Type, error) {
	text = strings.ToLower(strings.TrimSpace(text))
	if inner, ok := strings.CutPrefix(text, "option<"); ok && strings.HasSuffix(inner, ">") {
		return parseType(inner[:len(inner)-1])
	}
	for _, word := range []string{"array", "set"} {
		if inner, ok := strings.CutPrefix(text, word+"<"); ok && strings.HasSuffix(inner, ">") {
			elem := inner[:len(inner)-1]
			if i := strings.LastIndexByte(elem, ','); i >= 0 && !strings.Contains(elem[i:], ">") {
				elem = elem[:i]
			}
			t, err := parseType(elem)
			if err != nil {
				return dump.Type{}, err
			}
			kind := dump.Array
			if word == "set" {
				kind = dump.Set
			}
			return dump.Type{Kind: kind, Elem: &t}, nil
		}
	}
	switch text {
	case "bool":
		return dump.Type{Kind: dump.Boolean}, nil
	case "int":
		return dump.Type{Kind: dump.Integer, Bits: 64}, nil
	case "float":
		return dump.Type{Kind: dump.Float, Bits: 64}, nil
	case "decimal":
		return dump.Type{Kind: dump.Decimal}, nil
	case "string":
		return dump.Type{Kind: dump.String}, nil
	case "bytes":
		return dump.Type{Kind: dump.Binary}, nil
	case "datetime":
		return dump.Type{Kind: dump.Timestamp, Precision: 9}, nil
	case "uuid":
		return dump.Type{Kind: dump.UUID}, nil
	case "object":
		return dump.Type{Kind: dump.JSON}, nil
	}
	return dump.Type{Kind: dump.Other}, nil
}
