package cassandra

import (
	"strings"

	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/dump"
)

// The type functions of the dump. See D274, D304 and docs/DUMP.md.
//
// The model serves Apache Cassandra and ScyllaDB, which share the type names of CQL.

func init() {
	dump.RegisterTypes(dbmeta.Cassandra, dump.Types{Parse: parseType, Render: renderType, Text: "text"})
}

// parseType reads what system_schema.columns gives, such as int, list<int>, frozen<set<text>>
// or map<text, int>. A frozen type is the type that it holds.
func parseType(text string) (dump.Type, error) {
	text = strings.ToLower(strings.TrimSpace(text))
	if inner, ok := generic(text, "frozen"); ok {
		return parseType(inner)
	}
	for _, kind := range []dump.Kind{dump.Array, dump.Set} {
		name := map[dump.Kind]string{dump.Array: "list", dump.Set: "set"}[kind]
		if inner, ok := generic(text, name); ok {
			elem, err := parseType(inner)
			if err != nil {
				return dump.Type{}, err
			}
			return dump.Type{Kind: kind, Elem: &elem}, nil
		}
	}
	switch text {
	case "boolean":
		return dump.Type{Kind: dump.Boolean}, nil
	case "tinyint":
		return dump.Type{Kind: dump.Integer, Bits: 8}, nil
	case "smallint":
		return dump.Type{Kind: dump.Integer, Bits: 16}, nil
	case "int":
		return dump.Type{Kind: dump.Integer, Bits: 32}, nil
	case "bigint", "counter":
		return dump.Type{Kind: dump.Integer, Bits: 64}, nil
	case "varint":
		return dump.Type{Kind: dump.BigInteger}, nil
	case "float":
		return dump.Type{Kind: dump.Float, Bits: 32}, nil
	case "double":
		return dump.Type{Kind: dump.Float, Bits: 64}, nil
	case "decimal":
		return dump.Type{Kind: dump.Decimal}, nil
	case "text", "varchar", "ascii":
		return dump.Type{Kind: dump.String}, nil
	case "blob":
		return dump.Type{Kind: dump.Binary}, nil
	case "date":
		return dump.Type{Kind: dump.Date}, nil
	case "time":
		return dump.Type{Kind: dump.TimeOfDay}, nil
	case "timestamp":
		return dump.Type{Kind: dump.Timestamp, Precision: 3}, nil
	case "uuid", "timeuuid":
		return dump.Type{Kind: dump.UUID}, nil
	case "inet":
		return dump.Type{Kind: dump.IPAddress}, nil
	}
	return dump.Type{Kind: dump.Other}, nil
}

// generic returns the text inside name< and >, and whether text is that type.
func generic(text, name string) (string, bool) {
	if strings.HasPrefix(text, name+"<") && strings.HasSuffix(text, ">") {
		return text[len(name)+1 : len(text)-1], true
	}
	return "", false
}

// renderType writes a type in CQL. A timestamp is an instant with milliseconds, so a time
// with no zone is read as UTC. CQL has no interval that the other products share and no
// JSON type. A decimal has no digits to set, and holds any value of the source.
func renderType(t dump.Type) (dump.Rendering, error) {
	exact := func(sql string) (dump.Rendering, error) {
		return dump.Rendering{SQL: sql}, nil
	}
	switch t.Kind {
	case dump.Boolean:
		return exact("boolean")
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
			return exact("tinyint")
		case bits <= 16:
			return exact("smallint")
		case bits <= 32:
			return exact("int")
		case bits <= 64:
			return exact("bigint")
		}
		return exact("varint")
	case dump.BigInteger:
		return exact("varint")
	case dump.Float:
		if t.Bits == 32 {
			return exact("float")
		}
		return exact("double")
	case dump.Decimal:
		return exact("decimal")
	case dump.String:
		return exact("text")
	case dump.Binary:
		return exact("blob")
	case dump.Date:
		return exact("date")
	case dump.TimeOfDay:
		return exact("time")
	case dump.LocalTimestamp:
		return dump.Rendering{SQL: "timestamp", Level: dump.Lossy,
			Note: "a timestamp of CQL is an instant, so a time with no zone is read as UTC"}, nil
	case dump.Timestamp:
		if t.Precision > 3 {
			return dump.Rendering{SQL: "timestamp", Level: dump.Lossy, Note: "a timestamp of CQL keeps milliseconds"}, nil
		}
		return exact("timestamp")
	case dump.UUID:
		return exact("uuid")
	case dump.IPAddress:
		return exact("inet")
	case dump.Array, dump.Set:
		if t.Elem == nil {
			return dump.Rendering{}, dump.ErrNoEquivalent
		}
		r, err := renderType(*t.Elem)
		if err != nil {
			return dump.Rendering{}, err
		}
		word := "list"
		if t.Kind == dump.Set {
			word = "set"
		}
		r.SQL = word + "<" + r.SQL + ">"
		return r, nil
	}
	return dump.Rendering{}, dump.ErrNoEquivalent
}
