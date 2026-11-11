// Package typeparse reads the type names of the products that a basic dump only reads from.
// A product with a source only dump (D305) keeps no statement of its own for a type, so the
// parsers are shared by the families of names: the names of Presto, of Hive, of the JDBC
// types, of the mappings of Elasticsearch, and of Apache Arrow.
package typeparse

import (
	"strconv"
	"strings"

	"github.com/xo/dbmeta/dump"
)

// NoRender is the Render function of a product that is only a source. It finds no type, so a
// column takes the Text level when the client allows it.
func NoRender(dump.Type) (dump.Rendering, error) {
	return dump.Rendering{}, dump.ErrNoEquivalent
}

// split returns the name before the first open bracket, the arguments inside the
// brackets, and the text after the closing one. The brackets are ( and ) or < and >.
func split(text string) (name string, args []string, rest string) {
	text = strings.TrimSpace(text)
	open := strings.IndexAny(text, "(<")
	if open < 0 {
		return text, nil, ""
	}
	closing := map[byte]byte{'(': ')', '<': '>'}[text[open]]
	depth, end := 0, -1
	for i := open; i < len(text) && end < 0; i++ {
		switch text[i] {
		case text[open]:
			depth++
		case closing:
			depth--
			if depth == 0 {
				end = i
			}
		}
	}
	if end < 0 {
		return text, nil, ""
	}
	name, rest = strings.TrimSpace(text[:open]), strings.TrimSpace(text[end+1:])
	depth, start := 0, open+1
	for i := open + 1; i <= end; i++ {
		switch {
		case text[i] == '(' || text[i] == '<':
			depth++
		case text[i] == ')' || text[i] == '>':
			if i == end {
				args = append(args, strings.TrimSpace(text[start:i]))
			}
			depth--
		case text[i] == ',' && depth == 0:
			args = append(args, strings.TrimSpace(text[start:i]))
			start = i + 1
		}
	}
	return name, args, rest
}

func number(args []string, i int) int {
	if i < len(args) {
		n, _ := strconv.Atoi(args[i])
		return n
	}
	return 0
}

// integer makes the type of a signed integer.
func integer(bits int) dump.Type { return dump.Type{Kind: dump.Integer, Bits: bits} }

// Presto reads a type of Presto, Trino or Athena, such as varchar(20), decimal(10, 2),
// timestamp(3) with time zone or array(varchar).
func Presto(text string) (dump.Type, error) {
	name, args, rest := split(strings.ToLower(text))
	zone := strings.Contains(rest, "with time zone") || strings.HasSuffix(name, "with time zone")
	name = strings.TrimSuffix(name, " with time zone")
	switch name {
	case "boolean":
		return dump.Type{Kind: dump.Boolean}, nil
	case "tinyint":
		return integer(8), nil
	case "smallint":
		return integer(16), nil
	case "integer", "int":
		return integer(32), nil
	case "bigint":
		return integer(64), nil
	case "real":
		return dump.Type{Kind: dump.Float, Bits: 32}, nil
	case "double":
		return dump.Type{Kind: dump.Float, Bits: 64}, nil
	case "decimal":
		return dump.Type{Kind: dump.Decimal, Precision: number(args, 0), Scale: number(args, 1)}, nil
	case "varchar":
		return dump.Type{Kind: dump.String, Length: number(args, 0)}, nil
	case "char":
		return dump.Type{Kind: dump.String, Fixed: true, Length: number(args, 0)}, nil
	case "varbinary":
		return dump.Type{Kind: dump.Binary}, nil
	case "date":
		return dump.Type{Kind: dump.Date}, nil
	case "time":
		if zone {
			return dump.Type{Kind: dump.TimeOfDayOffset, Precision: number(args, 0)}, nil
		}
		return dump.Type{Kind: dump.TimeOfDay, Precision: number(args, 0)}, nil
	case "timestamp":
		if zone {
			return dump.Type{Kind: dump.Timestamp, Precision: number(args, 0)}, nil
		}
		return dump.Type{Kind: dump.LocalTimestamp, Precision: number(args, 0)}, nil
	case "uuid":
		return dump.Type{Kind: dump.UUID}, nil
	case "ipaddress":
		return dump.Type{Kind: dump.IPAddress}, nil
	case "json":
		return dump.Type{Kind: dump.JSON}, nil
	case "array":
		if len(args) == 1 {
			elem, err := Presto(args[0])
			if err != nil {
				return dump.Type{}, err
			}
			return dump.Type{Kind: dump.Array, Elem: &elem}, nil
		}
	}
	return dump.Type{Kind: dump.Other}, nil
}

// Hive reads a type of Hive, Impala or MaxCompute, such as int, string, decimal(10,2),
// varchar(20) or array<string>.
func Hive(text string) (dump.Type, error) {
	name, args, _ := split(strings.ToLower(text))
	switch name {
	case "boolean":
		return dump.Type{Kind: dump.Boolean}, nil
	case "tinyint":
		return integer(8), nil
	case "smallint":
		return integer(16), nil
	case "int", "integer":
		return integer(32), nil
	case "bigint":
		return integer(64), nil
	case "float":
		return dump.Type{Kind: dump.Float, Bits: 32}, nil
	case "double", "double precision":
		return dump.Type{Kind: dump.Float, Bits: 64}, nil
	case "decimal", "numeric":
		return dump.Type{Kind: dump.Decimal, Precision: number(args, 0), Scale: number(args, 1)}, nil
	case "string":
		return dump.Type{Kind: dump.String}, nil
	case "varchar":
		return dump.Type{Kind: dump.String, Length: number(args, 0)}, nil
	case "char":
		return dump.Type{Kind: dump.String, Fixed: true, Length: number(args, 0)}, nil
	case "binary":
		return dump.Type{Kind: dump.Binary}, nil
	case "date":
		return dump.Type{Kind: dump.Date}, nil
	case "timestamp", "datetime", "timestamp_ntz":
		return dump.Type{Kind: dump.LocalTimestamp}, nil
	case "array":
		if len(args) == 1 {
			elem, err := Hive(args[0])
			if err != nil {
				return dump.Type{}, err
			}
			return dump.Type{Kind: dump.Array, Elem: &elem}, nil
		}
	}
	return dump.Type{Kind: dump.Other}, nil
}

// JDBC reads the type names of a JDBC driver, such as VARCHAR, CHARACTER VARYING, BIGINT,
// DOUBLE, TIMESTAMP and ANY. A name that no table of SQL has, such as ANY or OTHER, is the
// kind other.
func JDBC(text string) (dump.Type, error) {
	name, args, _ := split(strings.ToUpper(text))
	switch name {
	case "BOOLEAN", "BIT":
		return dump.Type{Kind: dump.Boolean}, nil
	case "TINYINT":
		return integer(8), nil
	case "SMALLINT":
		return integer(16), nil
	case "INTEGER", "INT":
		return integer(32), nil
	case "BIGINT":
		return integer(64), nil
	case "REAL", "FLOAT":
		return dump.Type{Kind: dump.Float, Bits: 32}, nil
	case "DOUBLE", "DOUBLE PRECISION":
		return dump.Type{Kind: dump.Float, Bits: 64}, nil
	case "DECIMAL", "NUMERIC":
		return dump.Type{Kind: dump.Decimal, Precision: number(args, 0), Scale: number(args, 1)}, nil
	case "VARCHAR", "CHARACTER VARYING", "LONGVARCHAR":
		return dump.Type{Kind: dump.String, Length: number(args, 0)}, nil
	case "CHAR", "CHARACTER":
		return dump.Type{Kind: dump.String, Fixed: true, Length: number(args, 0)}, nil
	case "VARBINARY", "BINARY", "BINARY VARYING":
		return dump.Type{Kind: dump.Binary}, nil
	case "DATE":
		return dump.Type{Kind: dump.Date}, nil
	case "TIME":
		return dump.Type{Kind: dump.TimeOfDay}, nil
	case "TIMESTAMP":
		return dump.Type{Kind: dump.LocalTimestamp}, nil
	case "TIMESTAMP WITH TIME ZONE", "TIMESTAMP_WITH_LOCAL_TIME_ZONE":
		return dump.Type{Kind: dump.Timestamp}, nil
	}
	return dump.Type{Kind: dump.Other}, nil
}

// Elastic reads the type of a mapping as the SQL of Elasticsearch and OpenSearch name it,
// such as LONG, KEYWORD, TEXT, DATETIME or IP. The case does not matter.
func Elastic(text string) (dump.Type, error) {
	switch strings.ToUpper(strings.TrimSpace(text)) {
	case "BOOLEAN":
		return dump.Type{Kind: dump.Boolean}, nil
	case "BYTE":
		return integer(8), nil
	case "SHORT":
		return integer(16), nil
	case "INTEGER":
		return integer(32), nil
	case "LONG":
		return integer(64), nil
	case "UNSIGNED_LONG":
		return dump.Type{Kind: dump.Integer, Bits: 64, Unsigned: true}, nil
	case "HALF_FLOAT", "FLOAT":
		return dump.Type{Kind: dump.Float, Bits: 32}, nil
	case "DOUBLE", "SCALED_FLOAT":
		return dump.Type{Kind: dump.Float, Bits: 64}, nil
	case "KEYWORD", "TEXT", "VERSION", "STRING":
		return dump.Type{Kind: dump.String}, nil
	case "DATETIME", "DATE", "TIMESTAMP", "DATE_NANOS":
		return dump.Type{Kind: dump.Timestamp, Precision: 3}, nil
	case "IP":
		return dump.Type{Kind: dump.IPAddress}, nil
	case "BINARY":
		return dump.Type{Kind: dump.Binary}, nil
	}
	return dump.Type{Kind: dump.Other}, nil
}

// Arrow reads a type of Apache Arrow as InfluxDB 3 names it, such as Int64, Float64, Utf8,
// Boolean, Timestamp(ns) and Dictionary(Int32, Utf8), which a tag is.
func Arrow(text string) (dump.Type, error) {
	name, args, _ := split(text)
	switch strings.ToLower(name) {
	case "boolean":
		return dump.Type{Kind: dump.Boolean}, nil
	case "int8":
		return integer(8), nil
	case "int16":
		return integer(16), nil
	case "int32":
		return integer(32), nil
	case "int64":
		return integer(64), nil
	case "uint8":
		return dump.Type{Kind: dump.Integer, Bits: 8, Unsigned: true}, nil
	case "uint16":
		return dump.Type{Kind: dump.Integer, Bits: 16, Unsigned: true}, nil
	case "uint32":
		return dump.Type{Kind: dump.Integer, Bits: 32, Unsigned: true}, nil
	case "uint64":
		return dump.Type{Kind: dump.Integer, Bits: 64, Unsigned: true}, nil
	case "float32":
		return dump.Type{Kind: dump.Float, Bits: 32}, nil
	case "float64":
		return dump.Type{Kind: dump.Float, Bits: 64}, nil
	case "utf8", "largeutf8":
		return dump.Type{Kind: dump.String}, nil
	case "binary", "largebinary":
		return dump.Type{Kind: dump.Binary}, nil
	case "date32", "date64":
		return dump.Type{Kind: dump.Date}, nil
	case "timestamp":
		p := map[string]int{"s": 0, "ms": 3, "us": 6, "ns": 9}[strings.ToLower(strings.TrimSpace(strings.SplitN(strings.Join(args, ","), ",", 2)[0]))]
		return dump.Type{Kind: dump.LocalTimestamp, Precision: p}, nil
	case "dictionary":
		if len(args) == 2 {
			return Arrow(args[1])
		}
	}
	return dump.Type{Kind: dump.Other}, nil
}
