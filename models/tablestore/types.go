package tablestore

import (
	"strconv"
	"strings"

	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/dump"
	"github.com/xo/dbmeta/models/internal/typeparse"
)

// The type functions of the dump. Tablestore is only a source, so no type has an equivalent.
// Its SQL names the types as MySQL does. See D274, D305 and docs/DUMP.md.

func init() {
	dump.RegisterTypes(dbmeta.Tablestore, dump.Types{Parse: parseType, Render: typeparse.NoRender, Text: "text"})
}

// parseType reads what DESCRIBE writes, such as bigint(20), mediumtext or varchar(1024).
func parseType(text string) (dump.Type, error) {
	name, length := strings.ToLower(strings.TrimSpace(text)), 0
	if i := strings.IndexByte(name, '('); i >= 0 && strings.HasSuffix(name, ")") {
		length, _ = strconv.Atoi(strings.SplitN(name[i+1:len(name)-1], ",", 2)[0])
		name = strings.TrimSpace(name[:i])
	}
	switch name {
	case "bool", "boolean":
		return dump.Type{Kind: dump.Boolean}, nil
	case "tinyint":
		return dump.Type{Kind: dump.Integer, Bits: 8}, nil
	case "smallint":
		return dump.Type{Kind: dump.Integer, Bits: 16}, nil
	case "int", "integer", "mediumint":
		return dump.Type{Kind: dump.Integer, Bits: 32}, nil
	case "bigint":
		return dump.Type{Kind: dump.Integer, Bits: 64}, nil
	case "float":
		return dump.Type{Kind: dump.Float, Bits: 32}, nil
	case "double":
		return dump.Type{Kind: dump.Float, Bits: 64}, nil
	case "varchar", "char":
		return dump.Type{Kind: dump.String, Length: length}, nil
	case "tinytext", "text", "mediumtext", "longtext":
		return dump.Type{Kind: dump.String}, nil
	case "tinyblob", "blob", "mediumblob", "longblob", "varbinary", "binary":
		return dump.Type{Kind: dump.Binary}, nil
	case "date":
		return dump.Type{Kind: dump.Date}, nil
	case "datetime", "timestamp":
		return dump.Type{Kind: dump.LocalTimestamp}, nil
	}
	return dump.Type{Kind: dump.Other}, nil
}
