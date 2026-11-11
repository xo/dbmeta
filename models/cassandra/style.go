package cassandra

import (
	"strings"

	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/dump"
)

// The statement style of the dump. See D274, D304 and docs/DUMP.md.
//
// A keyspace is the schema. A table needs a primary key, and it has no default, no NOT NULL
// beside the key, no foreign key, no unique key and no check. An INSERT takes one row.

func init() {
	dump.RegisterStyle(dbmeta.Cassandra, dump.Style{
		Quote:             quoteIdentifier,
		Literal:           literal,
		QualifyAll:        true,
		CreateSchema:      createKeyspace,
		InlineConstraints: true,
		OrderFree:         true,
		// The key has no name in CQL.
		AnonymousConstraints: true,
		Column:               column,
		RefuseConstraint:     refuseConstraint,
		Binary:               func(hex string) string { return "0x" + hex },
		ZoneInTimestamps:     true,
		MaxRows:              1,
		// The driver takes a decimal as an inf.Dec only.
		NoBind: func(t dump.Type) bool { return t.Kind == dump.Decimal },
		FloatSpecial: func(v float64) (string, bool) {
			switch {
			case v != v:
				return "NaN", true
			case v > 0:
				return "Infinity", true
			}
			return "-Infinity", true
		},
	})
}

// quoteIdentifier quotes a name with double quotes. CQL keeps the case of a quoted name.
func quoteIdentifier(name string) string {
	return dbmeta.QuoteIdentifier(name, `"`, `"`)
}

// literal writes a string literal. CQL has no backslash escape.
func literal(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

// createKeyspace makes a keyspace on one node. The client changes the replication for a
// cluster of its own.
func createKeyspace(quoted string) string {
	return "CREATE KEYSPACE " + quoted + " WITH replication = {'class': 'SimpleStrategy', 'replication_factor': 1}"
}

// column writes the name and the type. A table of CQL has no default and no NOT NULL.
func column(c dump.Column, typeSQL string, q func(string) string, _ func(string) string) string {
	return q(c.Name) + " " + typeSQL
}

// refuseConstraint says why a constraint is not written. CQL has a primary key and nothing
// else.
func refuseConstraint(_ dump.Table, c dump.Constraint) string {
	if c.Kind == "primary key" {
		return ""
	}
	return "CQL has no " + c.Kind + ", so " + c.Name + " is not written"
}
