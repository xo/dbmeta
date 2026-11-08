package dump

import (
	"strconv"
	"strings"
)

// Kind is a family of types that means the same thing in any database, such as
// an integer or a date. The names are the kinds in the types document of dbimp, so
// that dbmeta and dbimp give one answer for a type that both name. See
// D275.
type Kind string

// The kinds that a relational database has.
const (
	Boolean         Kind = "boolean"
	Integer         Kind = "integer"
	Float           Kind = "float"
	Decimal         Kind = "decimal"
	BigInteger      Kind = "big integer"
	String          Kind = "string"
	Binary          Kind = "binary"
	Date            Kind = "date"
	TimeOfDay       Kind = "time of day"
	TimeOfDayOffset Kind = "time of day with offset"
	LocalTimestamp  Kind = "local timestamp"
	Timestamp       Kind = "timestamp"
	Interval        Kind = "interval"
	UUID            Kind = "uuid"
	IPAddress       Kind = "ip address"
	JSON            Kind = "json"
	Array           Kind = "array"
	Set             Kind = "set"
	Map             Kind = "map"
	Tuple           Kind = "tuple"
	Vector          Kind = "vector"
	Geometry        Kind = "geometry"
	Range           Kind = "range"
	Other           Kind = "other"
)

// Type is a column type, read from the text of a product.
//
// Precision means the digits of a decimal, and for a time, a timestamp or an
// interval it means the digits of the fraction of a second. A field that the
// kind does not use is zero.
type Type struct {
	Kind Kind `json:"kind"`
	// Bits is 8, 16, 32 or 64 for an integer or a float, and zero when the text
	// does not say.
	Bits int `json:"bits,omitempty"`
	// Length is the length of a string or a binary value, in characters or in
	// bytes. Zero means that the type has no limit, or that the text names none.
	Length int `json:"length,omitempty"`
	// Fixed means a char and not a varchar, and a binary and not a varbinary.
	Fixed bool `json:"fixed,omitempty"`
	// Precision and Scale are the digits of a decimal.
	Precision int `json:"precision,omitempty"`
	Scale     int `json:"scale,omitempty"`
	// Unsigned means that an integer or a decimal has no negative values.
	Unsigned bool `json:"unsigned,omitempty"`
	// Elem is the type of the element of an array or a set.
	Elem *Type `json:"elem,omitempty"`
	// Values are the labels of an enum, which is a string with a list of values.
	Values []string `json:"values,omitempty"`

	// Raw is the text that the product wrote, kept so that a dump to the same
	// dialect writes the type back as it was.
	Raw string `json:"raw,omitempty"`
}

// Go returns the Go type that stands for the kind, as dbimp names it. The
// package is not imported, only named. See D275.
func (t Type) Go() GoType {
	switch t.Kind {
	case Boolean:
		return GoType{Name: "bool"}
	case Integer:
		if t.Unsigned && t.Bits == 64 {
			return GoType{Name: "uint64"}
		}
		return GoType{Name: "int64"}
	case Float:
		return GoType{Name: "float64"}
	case Decimal:
		return GoType{Import: "github.com/cockroachdb/apd/v3", Package: "apd", Name: "*Decimal"}
	case BigInteger:
		return GoType{Import: "math/big", Package: "big", Name: "*Int"}
	case String:
		return GoType{Name: "string"}
	case Binary:
		return GoType{Name: "[]byte"}
	case Date:
		return dbimp("Date")
	case TimeOfDay:
		return dbimp("LocalTime")
	case TimeOfDayOffset:
		return dbimp("OffsetTime")
	case LocalTimestamp:
		return dbimp("LocalDateTime")
	case Timestamp:
		return GoType{Import: "time", Package: "time", Name: "Time"}
	case Interval:
		return dbimp("Interval")
	case UUID:
		return GoType{Import: "github.com/google/uuid", Package: "uuid", Name: "UUID"}
	case IPAddress:
		return GoType{Import: "net/netip", Package: "netip", Name: "Addr"}
	case JSON, Map:
		return GoType{Name: "any"}
	case Array, Set, Tuple:
		return GoType{Name: "[]any"}
	}
	return GoType{Name: "string"}
}

// dbimp returns a type of the root package of dbimp.
func dbimp(name string) GoType {
	return GoType{Import: "github.com/xo/dbimp", Package: "dbimp", Name: name}
}

// GoType names a Go type without importing it, so that dump takes no
// dependency (D275).
type GoType struct {
	// Import is the import path, empty for a type that Go predeclares, such as
	// bool, string, int64 and []byte.
	Import string
	// Package is the name that the code writes, which is not always the last
	// element of the path: github.com/cockroachdb/apd/v3 is the package apd.
	Package string
	// Name is the name of the type, with a leading * or [] where it has one.
	Name string
}

// String writes the type as code, such as time.Time, *apd.Decimal or []byte.
func (g GoType) String() string {
	if g.Package == "" {
		return g.Name
	}
	i := strings.IndexFunc(g.Name, func(r rune) bool { return r != '*' && r != '[' && r != ']' })
	if i < 0 {
		i = len(g.Name)
	}
	return g.Name[:i] + g.Package + "." + g.Name[i:]
}

// Level says how well a type in the target stands for a type in the source.
type Level int

// The levels. A client allows each one on its own, and dbmeta never picks a
// level that the client did not allow.
const (
	// Exact means that the target type holds every value of the source type.
	Exact Level = iota
	// Lossy means that the target has a type of the same kind that holds less.
	Lossy
	// Text means that the target has no type of the kind, so the string type of
	// the target stands for it.
	Text
)

// String returns the name of the level.
func (l Level) String() string {
	switch l {
	case Exact:
		return "exact"
	case Lossy:
		return "lossy"
	case Text:
		return "text"
	}
	return "level " + strconv.Itoa(int(l))
}

// Rendering is the type of a column in a target dialect.
type Rendering struct {
	// SQL is the type as the target writes it, such as numeric(10,2).
	SQL string
	// Level is how well it stands for the source type.
	Level Level
	// Note says why the level is not Exact, and is empty for Exact.
	Note string
}
