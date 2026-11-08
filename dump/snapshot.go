package dump

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
)

// SnapshotVersion is the version of the format that this package writes and
// reads. A change to the name, the tag or the meaning of a field needs a new
// number. See "The snapshot and its format" in docs/DUMP.md.
const SnapshotVersion = 1

// ErrUnknownVersion is the unknown version error. A reader refuses a file with a
// version it does not know, and the message names the one it found.
const ErrUnknownVersion Error = "unknown snapshot version"

// Snapshot is the schema of a database as plain values. It is the intermediate
// format: the `json` tags define it, and every encoder writes the same names.
// It holds no data and nothing that depends on a target or a policy.
//
// The order is stable. Schemas, tables, views and the other objects are in order
// of name, and columns, index columns and enum values are in order of position,
// so the same database gives the same file.
type Snapshot struct {
	Version int `json:"version"`
	// Dialect is the dialect of the source, such as postgres.
	Dialect string `json:"dialect"`
	// Server is the line that the server reports for its version.
	Server string `json:"server,omitempty"`

	Schemas []Schema `json:"schemas"`
	// Skipped lists the objects that the reader saw and did not read, with the
	// reason, so that nothing is left out without a record.
	Skipped []Skipped `json:"skipped,omitempty"`
}

// Skipped is an object that Read saw and did not put in the snapshot.
type Skipped struct {
	Schema string `json:"schema,omitempty"`
	Name   string `json:"name"`
	Kind   string `json:"kind"`
	Reason string `json:"reason"`
}

// Schema is a schema, or a database for a product that names one that way.
type Schema struct {
	Name    string `json:"name"`
	Comment string `json:"comment,omitempty"`

	Tables    []Table    `json:"tables,omitempty"`
	Views     []View     `json:"views,omitempty"`
	Sequences []Sequence `json:"sequences,omitempty"`
	Enums     []Enum     `json:"enums,omitempty"`
	Routines  []Routine  `json:"routines,omitempty"`
}

// Table is a table with what belongs to it.
type Table struct {
	Name    string `json:"name"`
	Comment string `json:"comment,omitempty"`

	Columns     []Column     `json:"columns"`
	Constraints []Constraint `json:"constraints,omitempty"`
	Indexes     []Index      `json:"indexes,omitempty"`
	Triggers    []Trigger    `json:"triggers,omitempty"`
}

// Column is a column.
type Column struct {
	Name string `json:"name"`
	Type Type   `json:"type"`
	// Nullable is false for a column that has NOT NULL. A product that records
	// nothing reports true.
	Nullable bool `json:"nullable"`
	// Default is the expression of the default, as the source writes it. It is
	// nil when the column has none, and a default of the empty string is "". For a
	// generated column it is the expression that computes the value.
	Default *string `json:"default,omitempty"`
	// DefaultLiteral says that Default is a value to quote, and not SQL.
	DefaultLiteral bool `json:"default_literal,omitempty"`
	// Identity says how the database numbers the column, such as always, by
	// default or rowid, and is empty for a column it does not number.
	Identity string `json:"identity,omitempty"`
	// Generated says that the column is computed, such as stored or virtual.
	Generated string `json:"generated,omitempty"`
	Collation string `json:"collation,omitempty"`
	Comment   string `json:"comment,omitempty"`
}

// Constraint is a primary key, a unique key, a foreign key or a check. Kind is
// in lower case.
type Constraint struct {
	Name string `json:"name"`
	// Kind is primary key, unique, foreign key, check or exclusion.
	Kind    string   `json:"kind"`
	Columns []string `json:"columns,omitempty"`
	// Ref is where a foreign key points.
	Ref *Reference `json:"ref,omitempty"`
	// Definition is the text of the source, such as CHECK (n > 0).
	Definition string `json:"definition,omitempty"`
	// OnUpdate and OnDelete are what a foreign key does, when the source says it
	// apart from Definition.
	OnUpdate   string `json:"on_update,omitempty"`
	OnDelete   string `json:"on_delete,omitempty"`
	Deferrable bool   `json:"deferrable,omitempty"`
	Deferred   bool   `json:"deferred,omitempty"`
	Comment    string `json:"comment,omitempty"`
}

// Reference is the table and the columns that a foreign key points at.
type Reference struct {
	Schema  string   `json:"schema,omitempty"`
	Table   string   `json:"table"`
	Columns []string `json:"columns"`
}

// Index is an index. A primary key and a unique constraint that the database
// builds an index for appear here and in Constraints.
type Index struct {
	Name    string `json:"name"`
	Type    string `json:"type,omitempty"`
	Unique  bool   `json:"unique,omitempty"`
	Primary bool   `json:"primary,omitempty"`
	// Columns is empty for a product that does not say which columns an index
	// has.
	Columns []IndexColumn `json:"columns,omitempty"`
	// Predicate is the WHERE clause of a partial index.
	Predicate string `json:"predicate,omitempty"`
	// Definition is the whole CREATE INDEX statement of the source, for a
	// product that keeps one. It holds what the columns cannot, such as an
	// operator class.
	Definition string `json:"definition,omitempty"`
	// Constraint is p, u or x for an index that a primary key, a unique
	// constraint or an exclusion constraint owns, and empty for any other. The
	// constraint writes the index, so the index is not written again.
	Constraint string `json:"constraint,omitempty"`
	Comment    string `json:"comment,omitempty"`
}

// IndexColumn is a column of an index, or an expression.
type IndexColumn struct {
	// Name is the column, and is empty for an expression.
	Name string `json:"name,omitempty"`
	// Expression is the text of an expression, and is empty for a column.
	Expression string `json:"expression,omitempty"`
	Descending bool   `json:"descending,omitempty"`
	// Include is true for a column that the index stores and does not sort by.
	Include bool `json:"include,omitempty"`
}

// Trigger is a trigger. Its definition is the text of the source and is not
// translated.
type Trigger struct {
	Name       string `json:"name"`
	Enabled    string `json:"enabled,omitempty"`
	Definition string `json:"definition"`
	Comment    string `json:"comment,omitempty"`
}

// View is a view. Its definition is the text of the source and is not
// translated.
type View struct {
	Name        string `json:"name"`
	Definition  string `json:"definition,omitempty"`
	CheckOption string `json:"check_option,omitempty"`
	Comment     string `json:"comment,omitempty"`
}

// Sequence is a sequence.
type Sequence struct {
	Name      string `json:"name"`
	DataType  string `json:"data_type,omitempty"`
	Start     string `json:"start,omitempty"`
	Minimum   string `json:"minimum,omitempty"`
	Maximum   string `json:"maximum,omitempty"`
	Increment string `json:"increment,omitempty"`
	Cycles    bool   `json:"cycles,omitempty"`
	// OwnedBy is the column that owns the sequence, as the source names it.
	OwnedBy string `json:"owned_by,omitempty"`
	// Identity is true for the sequence of an identity column.
	Identity bool   `json:"identity,omitempty"`
	Comment  string `json:"comment,omitempty"`
}

// Enum is an enumerated type with its labels in order.
type Enum struct {
	Name   string   `json:"name"`
	Values []string `json:"values"`
}

// Routine is a function, a procedure or an aggregate. Its definition is the text
// of the source and is not translated.
type Routine struct {
	Name       string `json:"name"`
	Kind       string `json:"kind"`
	Arguments  string `json:"arguments,omitempty"`
	ResultType string `json:"result_type,omitempty"`
	Language   string `json:"language,omitempty"`
	Definition string `json:"definition,omitempty"`
	Comment    string `json:"comment,omitempty"`
}

// JSON writes the snapshot as indented JSON with a newline at the end. The
// order of the fields is the order of the struct, so the same snapshot gives the
// same bytes.
func (s *Snapshot) JSON() ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(s); err != nil {
		return nil, fmt.Errorf("writing the snapshot: %w", err)
	}
	return buf.Bytes(), nil
}

// ParseSnapshot reads a snapshot from JSON. It returns [ErrUnknownVersion] for a
// version that this package does not know, and names the one it found.
func ParseSnapshot(data []byte) (*Snapshot, error) {
	var head struct {
		Version int `json:"version"`
	}
	if err := json.Unmarshal(data, &head); err != nil {
		return nil, fmt.Errorf("reading the snapshot version: %w", err)
	}
	if head.Version != SnapshotVersion {
		return nil, fmt.Errorf("version %s, and this package reads %d: %w", strconv.Itoa(head.Version), SnapshotVersion, ErrUnknownVersion)
	}
	var s Snapshot
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&s); err != nil {
		return nil, fmt.Errorf("reading the snapshot: %w", err)
	}
	return &s, nil
}
