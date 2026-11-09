package dump

import (
	"fmt"
	"sync"

	"github.com/xo/dbmeta"
)

// Style is what the generator needs to know about the SQL of a dialect. A model
// registers it from init, beside its type functions, because the syntax of a
// product lives with its model. A field that is nil means that the product has
// no such statement, or that the generic form is right.
type Style struct {
	// Quote quotes an identifier. It is required.
	Quote func(name string) string
	// Literal writes a string literal. It is required.
	Literal func(s string) string

	// DefaultSchema is the schema that a connection starts in. Names in it are
	// not qualified, unless QualifyAll is set. The other schemas are created and
	// qualified. The value * means that every schema is the one the connection
	// starts in, for a product whose schema is its database.
	DefaultSchema string
	QualifyAll    bool
	// ExistingSchemas are the schemas that exist in an empty database, so that
	// a dump does not create them.
	ExistingSchemas []string
	// CreateSchema writes the statement that creates a schema. It is nil for a
	// product whose schema is its database, which a dump does not create.
	CreateSchema func(quoted string) string

	// Preamble is the statements that begin a script, such as a setting that
	// lets a function name a table that is not there yet.
	Preamble []string
	// BeforeData and AfterData are the statements that go round the rows. SQLite
	// turns its foreign key checks off before the rows and checks at the end.
	BeforeData []string
	AfterData  []string
	// CheckAfterData is the statement in AfterData that returns a row for each
	// fault, and the dump fails when it returns one.
	CheckAfterData string
	// IdentityValue writes the statement that sets the next value of an identity
	// column from the rows that were loaded. It is nil where the product follows
	// the rows by itself, as MySQL and SQLite do.
	IdentityValue func(quotedTable, quotedColumn, column string, literal func(string) string) string

	// Column writes one column of a CREATE TABLE. It is nil for the generic form,
	// which writes the name, the type, the default and NOT NULL.
	Column func(c Column, typeSQL string, q func(string) string, literal func(string) string) string
	// InlineConstraints says that unique keys, checks and foreign keys go inside
	// the CREATE TABLE, because the product cannot add them to a table that
	// exists. The tables are then written so that a foreign key finds its table,
	// unless OrderFree says that the product does not mind.
	InlineConstraints bool
	OrderFree         bool
	// KeyType writes the type of a column that is in a key, when the product wants
	// another than the one it writes for the same column elsewhere. MySQL cannot
	// key a text or a blob without a length. The result is false when the type of
	// the column is fine in a key.
	KeyType func(t Type) (Rendering, bool)
	// Binary writes a binary value from its bytes in lower case hexadecimal, as
	// the product reads a binary literal. It is required for a dump of data.
	Binary func(hex string) string
	// ZoneInTimestamps says that the product reads an offset in the literal of a
	// timestamp, so an instant is written with +00:00. MySQL reads one only from
	// 8.0.19 and MariaDB not at all, so an instant is the wall clock in UTC.
	ZoneInTimestamps bool
	// SessionClock says that a driver of the product reads and writes a timestamp
	// as the wall clock of the session, and that the Location of the time.Time is
	// the zone of the driver and not of the value. MySQL is one: its driver reads
	// the clock that the server sends as a time in its own location, which is the
	// local zone unless the DSN says otherwise, and it writes the clock of the time
	// in that location. Both sessions are UTC (see [Style.Session]), so the clock
	// is the UTC one whatever the Location is. The loader reads the clock of a
	// value from such a source as UTC, and binds a date or a time for such a
	// target as text, which a driver sends unchanged.
	SessionClock bool
	// TypedNullBinary says that a NULL for a binary column is bound as a nil byte
	// slice and not as an untyped nil. The driver of SQL Server sends the second as
	// text, and the server refuses to convert it.
	TypedNullBinary bool
	// PrimaryKeyNotNull says that the product refuses a primary key over a column
	// that can be NULL, so the table writes NOT NULL for each such column. SQLite
	// accepts one, and a table that has one is written back the way it was read.
	PrimaryKeyNotNull bool
	// FloatSpecial writes NaN and the two infinities as a literal, for a product
	// that reads them. The result is false for a product that does not.
	FloatSpecial func(v float64) (string, bool)
	// Session is the statements that set the session that reads or writes rows, so
	// that a value means the same on both sides. MySQL reads and writes a timestamp
	// in the zone of the session, so both sessions are set to UTC. The client runs
	// them on the connection of the source before it reads and on the connection of
	// the target before it writes. [Style.Preamble] holds them for a script.
	Session []string
	// Boolean writes a boolean value, for a product that has no TRUE and FALSE
	// in a default or in a row, such as 1 and 0 in SQL Server. It is nil where the
	// words are right.
	Boolean func(v bool) string
	// IdentityInsert writes the statement that lets an INSERT give a value to an
	// identity column of a table, and the one that ends it, for a product that
	// has a session setting for it, such as SET IDENTITY_INSERT. Both run on the
	// connection that runs the INSERT. It is nil for a product with no such rule.
	IdentityInsert func(quotedTable string, on bool) string
	// OverridingIdentity is the clause that lets an INSERT give a value to an
	// identity column that the product always numbers, such as PostgreSQL's
	// OVERRIDING SYSTEM VALUE. It is empty for a product with no such rule.
	OverridingIdentity string
	// MaxRows is the most rows of one INSERT that the product takes. Zero means
	// that the default batch is right.
	MaxRows int
	// MaxParameters is the most parameters of one statement, for the INSERT with
	// placeholders. Zero means that no limit is known.
	MaxParameters int
	// NoForeignKeyActions says that the product has no ON DELETE and ON UPDATE for
	// a foreign key, so the rules are not written, and the table says so.
	NoForeignKeyActions bool
	// NoRestrict says that the product has no RESTRICT for a foreign key and that
	// NO ACTION is the same rule, as SQL Server has it.
	NoRestrict bool
	// AnonymousConstraints says that a constraint is written with no name, because
	// the product keeps no name that a person gave it, and the model makes one up.
	AnonymousConstraints bool
	// RefuseConstraint returns the reason that the product cannot hold a constraint
	// of a table, or the empty string when it can. A constraint that it refuses is
	// not written, and the script says why, so that the client can warn. SingleStore
	// has no foreign key and takes only a unique key that holds every column of its
	// primary key.
	RefuseConstraint func(t Table, c Constraint) string
	// TableOptions writes what follows the closing bracket of CREATE TABLE.
	TableOptions func(t Table, literal func(string) string) string
	// ImplicitIndex says that the product builds an index of its own for a
	// constraint, so the dump does not write it. A model leaves it nil when the
	// name of the index is the name of the constraint.
	ImplicitIndex func(t Table, ix Index) bool
	// Trigger writes a CREATE TRIGGER from what the source keeps.
	Trigger func(quotedTable string, tg Trigger, q func(string) string) string

	// Enum writes the statement for an enumerated type. It is nil when the
	// product has none as a type of its own.
	Enum func(quoted string, values []string, literal func(string) string) string
	// Extension writes the statement that installs an extension, from its quoted
	// name and its quoted schema. It is nil for a product with no extensions.
	Extension func(quotedName, quotedSchema string) string
	// Domain writes a CREATE DOMAIN. It is nil for a product with none.
	Domain func(quoted string, d Domain, q func(string) string, literal func(string) string) string
	// Composite writes a CREATE TYPE of a composite type. It is nil for a product
	// with none.
	Composite func(quoted string, c Composite, q func(string) string) string
	// Sequence writes a CREATE SEQUENCE. It is nil for a product with none.
	Sequence func(quoted string, s Sequence) string
	// OwnedBy writes the statement that ties a sequence to a column. It is nil
	// where the product has no such tie.
	OwnedBy func(quotedSequence, ownerColumn string) string

	// Index writes a CREATE INDEX. It is nil for the generic form.
	Index func(quotedTable string, ix Index, q func(string) string) string
	// View writes a CREATE VIEW from the definition that the source keeps.
	View func(quoted string, v View) string
	// MaterializedView writes a CREATE MATERIALIZED VIEW from the definition that
	// the source keeps. It is nil for a product with none, and a materialized view
	// is then written as a plain view and the script says so.
	MaterializedView func(quoted string, v View) string
	// PartitionBy writes what follows the closing bracket of CREATE TABLE for a
	// table that is split into partitions, such as PARTITION BY RANGE (key). It is
	// nil for a product that has none, and the table is then written whole.
	PartitionBy func(p Partitioning, q func(string) string, literal func(string) string) string
	// PartitionOf writes the CREATE TABLE of a table that is a partition of
	// another, from the quoted name of the partition, the quoted name of its parent
	// and its bound. It is nil for a product whose partitions are not tables.
	PartitionOf func(quoted, parent, bound string) string
	// Comment writes the statement that sets a comment, and is nil for a product
	// that takes it inside CREATE TABLE or has none. The target is TABLE or
	// COLUMN, and the name is quoted.
	Comment func(target, quotedName, comment string, literal func(string) string) string
	// ReferenceTable rewrites the definition of a foreign key so that the table
	// it points at carries its schema. It is nil where the definition already
	// does.
	ReferenceTable func(definition string, ref Reference, q func(string) string) string
}

var (
	styleMu sync.RWMutex
	styles  = map[dbmeta.Dialect]Style{}
)

// RegisterStyle registers the style of a dialect. Registering twice panics, as
// [dbmeta.RegisterDialect] does.
func RegisterStyle(d dbmeta.Dialect, s Style) {
	if s.Quote == nil || s.Literal == nil {
		panic("dump: the style of " + string(d) + " needs Quote and Literal")
	}
	styleMu.Lock()
	defer styleMu.Unlock()
	if _, ok := styles[d]; ok {
		panic("dump: style registered twice: " + string(d))
	}
	styles[d] = s
}

// styleOf returns the style of a dialect.
func styleOf(d dbmeta.Dialect) (Style, error) {
	styleMu.RLock()
	defer styleMu.RUnlock()
	s, ok := styles[d]
	if !ok {
		return Style{}, fmt.Errorf("writing statements for %s: %w", d, ErrNoStyle)
	}
	return s, nil
}
