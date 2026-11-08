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
	// NoForeignKeyActions says that the product has no ON DELETE and ON UPDATE for
	// a foreign key, so the rules are not written, and the table says so.
	NoForeignKeyActions bool
	// AnonymousConstraints says that a constraint is written with no name, because
	// the product keeps no name that a person gave it, and the model makes one up.
	AnonymousConstraints bool
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
	// Sequence writes a CREATE SEQUENCE. It is nil for a product with none.
	Sequence func(quoted string, s Sequence) string
	// OwnedBy writes the statement that ties a sequence to a column. It is nil
	// where the product has no such tie.
	OwnedBy func(quotedSequence, ownerColumn string) string

	// Index writes a CREATE INDEX. It is nil for the generic form.
	Index func(quotedTable string, ix Index, q func(string) string) string
	// View writes a CREATE VIEW from the definition that the source keeps.
	View func(quoted string, v View) string
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
