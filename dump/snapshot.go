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

	// Extensions are the extensions that the database has installed, which a
	// script installs first. An object that an extension owns is not in the
	// snapshot, because the extension makes it again.
	Extensions []Extension `json:"extensions,omitempty"`
	// Roles are the roles that the grants of the snapshot name, and the roles that
	// those are members of. A script makes each one that is not there, with the
	// attributes that it had, and with no password. Grants are the privileges on the
	// tables, the views and the sequences, and on columns, for a product that has them.
	// See D297.
	Roles  []RoleDef  `json:"roles,omitempty"`
	Grants []GrantDef `json:"grants,omitempty"`
	// EventTriggers are the triggers that fire on a DDL event, which belong to the
	// database or to a schema, and not to a table. See D296.
	EventTriggers []EventTriggerDef `json:"event_triggers,omitempty"`
	// PartitionFunctions and PartitionSchemes are what a product that partitions by a
	// scheme keeps apart from its tables. A script writes them before the tables.
	PartitionFunctions []PartitionFunction `json:"partition_functions,omitempty"`
	PartitionSchemes   []PartitionScheme   `json:"partition_schemes,omitempty"`
	Schemas            []Schema            `json:"schemas"`
	// ViewDependencies is true when the source answered which relations each view
	// reads, so that an empty View.DependsOn means that the view reads no other
	// view. When it is false the order of the views is not known.
	ViewDependencies bool `json:"view_dependencies,omitempty"`
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
	Domains   []Domain   `json:"domains,omitempty"`
	// Ranges are the range types that a person made. The multirange type of each
	// is made with it.
	Ranges []RangeType `json:"ranges,omitempty"`
	// Composites are the composite types that a person made, which are not the row
	// type of a table.
	Composites []Composite `json:"composites,omitempty"`
	// Logs are the materialized view logs of the tables of the schema, for a product
	// that has them. See D296.
	Logs []MaterializedViewLog `json:"logs,omitempty"`
	// XMLSchemas are the XML schema collections of the schema, for a product that has
	// them.
	XMLSchemas []XMLSchema `json:"xml_schemas,omitempty"`
	// Events are the scheduled events of a database, for a product that has them.
	Events []ScheduledEvent `json:"events,omitempty"`
	// Synonyms are other names for objects, for a product that has them.
	Synonyms []Synonym `json:"synonyms,omitempty"`
	// Collections are the collection types of Oracle: a varray or a nested table
	// of one element type.
	Collections []Collection `json:"collections,omitempty"`
	// TableTypes are the table types of SQL Server, which a routine takes as a
	// parameter. A table type has columns, keys and indexes like a table, and it
	// holds no rows.
	TableTypes []Table   `json:"table_types,omitempty"`
	Routines   []Routine `json:"routines,omitempty"`
}

// empty reports whether the schema holds no object.
func (s Schema) empty() bool {
	return len(s.Tables)+len(s.Views)+len(s.Sequences)+len(s.Enums)+len(s.Domains)+len(s.Ranges)+len(s.Composites)+
		len(s.Synonyms)+len(s.Events)+len(s.Logs)+len(s.XMLSchemas)+len(s.Collections)+len(s.TableTypes)+len(s.Routines) == 0
}

// Extension is an installed extension.
type Extension struct {
	Name string `json:"name"`
	// Schema is the schema that holds the objects of the extension.
	Schema  string `json:"schema,omitempty"`
	Version string `json:"version,omitempty"`
	Comment string `json:"comment,omitempty"`
}

// Domain is a type that is another type with a default and constraints.
type Domain struct {
	Name string `json:"name"`
	// Type is the type that the domain is built on, as the source writes it.
	Type      string `json:"type"`
	Collation string `json:"collation,omitempty"`
	NotNull   bool   `json:"not_null,omitempty"`
	// Default is the expression of the default, and nil when the domain has none.
	Default *string `json:"default,omitempty"`
	// Constraints are the checks as the source writes them, such as CHECK (VALUE > 0),
	// one after the other.
	Constraints string `json:"constraints,omitempty"`
	Comment     string `json:"comment,omitempty"`
}

// Composite is a composite type with its attributes in order.
type Composite struct {
	Name    string   `json:"name"`
	Comment string   `json:"comment,omitempty"`
	Columns []Column `json:"columns"`
}

// Synonym is another name for an object. The object need not exist.
type Synonym struct {
	Name string `json:"name"`
	// TargetSchema and Target name the object. The database and the server are empty
	// for an object of this database and server.
	TargetSchema   string `json:"target_schema,omitempty"`
	Target         string `json:"target"`
	TargetDatabase string `json:"target_database,omitempty"`
	TargetServer   string `json:"target_server,omitempty"`
}

// Collection is a collection type, a varray or a nested table, with the name of
// its element type. A client that moves a value of such a type needs the element.
type Collection struct {
	Name string `json:"name"`
	// Element is the name of the element type, as the source writes it.
	Element string `json:"element"`
}

// Table is a table with what belongs to it.
type Table struct {
	Name    string `json:"name"`
	Comment string `json:"comment,omitempty"`

	// Partitioning says how the table is split, and is nil for a table that is
	// not. For PostgreSQL the partitions are tables of their own, which name this
	// one in PartitionOf. For MySQL they live inside the table, in Parts.
	Partitioning *Partitioning `json:"partitioning,omitempty"`
	// PartitionOf is set for a table that is a partition of another. The rows of
	// such a table belong to its parent, and a client does not read them again.
	// See [Table.IsPartition].
	PartitionOf *PartitionOf `json:"partition_of,omitempty"`
	// Versioning is set for a system versioned table, and is nil for any other.
	// The history table is a table of the snapshot, and a client loads its rows
	// like those of any table. See [Versioning].
	Versioning *Versioning `json:"versioning,omitempty"`

	// Engine, Collation and Options are the storage engine, the collation and the
	// create options of a table of MySQL or MariaDB. See D296.
	Engine    string `json:"engine,omitempty"`
	Collation string `json:"collation,omitempty"`
	Options   string `json:"options,omitempty"`
	// Definition is the statement of the source that makes the table, for a
	// product that keeps it, so that a script of the same product writes it as it
	// was. See D293.
	Definition string `json:"definition,omitempty"`
	// OfType is the object type that an object table is made of, as owner.type.
	OfType string `json:"of_type,omitempty"`
	// Organization is index for a table that keeps its rows in its primary key.
	Organization string `json:"organization,omitempty"`

	// RowSecurity says that row level security is on for the table. Policies are its
	// row level security policies, or the security policies of SQL Server. See D296.
	RowSecurity bool        `json:"row_security,omitempty"`
	Policies    []PolicyDef `json:"policies,omitempty"`

	Columns     []Column     `json:"columns"`
	Constraints []Constraint `json:"constraints,omitempty"`
	Indexes     []Index      `json:"indexes,omitempty"`
	Triggers    []Trigger    `json:"triggers,omitempty"`
}

// IsPartition reports whether the table is a partition of another table of the
// snapshot. The parent holds its rows, and a client that reads the rows of every
// table leaves this one out, because the rows of the parent already include them.
func (t Table) IsPartition() bool { return t.PartitionOf != nil }

// Versioning says that a table keeps its history, as a system versioned table of
// SQL Server does. The server moves a row that an update or a delete replaced to
// the history table, and the period columns say when the row held.
type Versioning struct {
	// Schema and Table name the history table.
	Schema string `json:"schema"`
	Table  string `json:"table"`
	// Start and End name the period columns, which hold the instants that a row
	// began and ended. The history table has columns of the same names.
	Start string `json:"start"`
	End   string `json:"end"`
}

// Partitioning is how a table is split into partitions.
type Partitioning struct {
	// Strategy is the method in lower case, such as range, list, hash or key.
	Strategy string `json:"strategy"`
	// Expression is the key, as the source writes it between the brackets of
	// PARTITION BY, such as happened_at or year(hired).
	Expression string `json:"expression"`
	// Interval is the expression of an interval partitioned table, Reference the foreign
	// key of a table of reference partitioning, and SubStrategy, SubKey and SubTemplate
	// say how each partition is split again. They are empty for a table that has none.
	// See D297.
	Interval    string `json:"interval,omitempty"`
	Reference   string `json:"reference,omitempty"`
	SubStrategy string `json:"sub_strategy,omitempty"`
	SubKey      string `json:"sub_key,omitempty"`
	SubTemplate string `json:"sub_template,omitempty"`
	// Scheme is the partition scheme that lays the table out, for a product that keeps
	// them apart from its tables. Expression holds the columns then.
	Scheme string `json:"scheme,omitempty"`
	// Parts are the partitions that live inside the table, for a product whose
	// partitions are not tables. It is empty for PostgreSQL.
	Parts []Part `json:"parts,omitempty"`
}

// Part is a partition that lives inside its table.
type Part struct {
	Name string `json:"name"`
	// Bound is the limit of a range partition, or the values of a list one, as the
	// source writes it. It is empty for a hash or a key partition.
	Bound string `json:"bound,omitempty"`
	// Subs are the subpartitions that a person named, each written as its own clause,
	// for a table that has no template.
	Subs []string `json:"subs,omitempty"`
}

// PartitionOf names the table that a partition belongs to.
type PartitionOf struct {
	Schema string `json:"schema"`
	Table  string `json:"table"`
	// Bound is what follows the name in CREATE TABLE ... PARTITION OF, such as
	// FOR VALUES FROM ('2025-01-01') TO ('2026-01-01'), or DEFAULT.
	Bound string `json:"bound"`
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
	// Invisible says that the column is hidden from SELECT * and from an INSERT
	// that names no columns. See D294.
	Invisible bool `json:"invisible,omitempty"`
	// OnUpdate is the expression that the column takes when its row is updated.
	OnUpdate string `json:"on_update,omitempty"`
	// DefaultName is the name that a person gave the default constraint.
	DefaultName string `json:"default_name,omitempty"`
	// Storage is the table that stores a nested table column.
	Storage string `json:"storage,omitempty"`
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
	// Invisible says that the optimizer ignores the index. See D296.
	Invisible bool `json:"invisible,omitempty"`
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
	// Prefix is the length of the prefix of the column that the index holds, and
	// zero when it holds the whole column. See D296.
	Prefix int `json:"prefix,omitempty"`
}

// Trigger is a trigger. Its definition is the text of the source and is not
// translated.
type Trigger struct {
	Name       string `json:"name"`
	Enabled    string `json:"enabled,omitempty"`
	Definition string `json:"definition"`
	Comment    string `json:"comment,omitempty"`
	// Order is the position among the triggers of the same table, event and timing,
	// from one, for a product that orders them. See D296.
	Order int `json:"order,omitempty"`
	// SQLMode is the sql_mode that the trigger was made under. It is a pointer,
	// because an empty mode is a mode.
	SQLMode *string `json:"sql_mode,omitempty"`
	// Definer is the account that the trigger runs as, as user@host.
	Definer string `json:"definer,omitempty"`
	// Placement says which events the trigger is first or last for, as text such as
	// first INSERT, last UPDATE.
	Placement string `json:"placement,omitempty"`
	// Follows is the name of the trigger that this one follows, which the script
	// works out from Order, and the snapshot does not hold.
	Follows string `json:"-"`
}

// View is a view. Its definition is the text of the source and is not
// translated.
type View struct {
	Name string `json:"name"`
	// DependsOn lists the views that this view reads, as schema.name, in order.
	DependsOn   []string `json:"depends_on,omitempty"`
	Definition  string   `json:"definition,omitempty"`
	CheckOption string   `json:"check_option,omitempty"`
	Comment     string   `json:"comment,omitempty"`
	// Materialized says that the view keeps its rows, as a materialized view of
	// PostgreSQL does. It is built after the rows of its tables are loaded.
	Materialized bool `json:"materialized,omitempty"`
	// Options are the clauses that follow the name of a materialized view, such as
	// BUILD IMMEDIATE REFRESH COMPLETE ON DEMAND, for a product that has them.
	Options string `json:"options,omitempty"`
	// Triggers are the triggers of the view, which are the INSTEAD OF ones.
	Triggers []Trigger `json:"triggers,omitempty"`
	// Indexes are the indexes of a materialized view, which a plain view has none
	// of.
	Indexes []Index `json:"indexes,omitempty"`
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
	// Last is the last value that the sequence gave, and empty when it gave none or the
	// product does not say. A script sets the next value from it. See D296.
	Last string `json:"last,omitempty"`
}

// Enum is an enumerated type with its labels in order.
type Enum struct {
	Name   string   `json:"name"`
	Values []string `json:"values"`
}

// RangeType is a range type. The parts after the subtype are empty when the type
// takes the default. See D292.
type RangeType struct {
	Name        string `json:"name"`
	Subtype     string `json:"subtype"`
	OpClass     string `json:"opclass,omitempty"`
	Collation   string `json:"collation,omitempty"`
	Canonical   string `json:"canonical,omitempty"`
	SubtypeDiff string `json:"subtype_diff,omitempty"`
	Multirange  string `json:"multirange,omitempty"`
	Comment     string `json:"comment,omitempty"`
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
	// SQLMode is the sql_mode that a MySQL or MariaDB routine was made under.
	// It is a pointer, because an empty mode is a mode.
	SQLMode *string `json:"sql_mode,omitempty"`
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

// ScheduledEvent is a statement that the server runs on a schedule.
type ScheduledEvent struct {
	Name    string `json:"name"`
	Definer string `json:"definer,omitempty"`
	// Kind is one time or recurring.
	Kind          string  `json:"kind"`
	ExecuteAt     string  `json:"execute_at,omitempty"`
	IntervalValue string  `json:"interval_value,omitempty"`
	IntervalField string  `json:"interval_field,omitempty"`
	Starts        string  `json:"starts,omitempty"`
	Ends          string  `json:"ends,omitempty"`
	Status        string  `json:"status"`
	OnCompletion  string  `json:"on_completion"`
	Body          string  `json:"body"`
	Comment       string  `json:"comment,omitempty"`
	SQLMode       *string `json:"sql_mode,omitempty"`
}

// EventTriggerDef is a trigger that fires on a DDL event. Definition is the statement
// that makes it, for a product that keeps one, and the other fields make it for a
// product that does not.
type EventTriggerDef struct {
	Name       string `json:"name"`
	Schema     string `json:"schema,omitempty"`
	Event      string `json:"event,omitempty"`
	Tags       string `json:"tags,omitempty"`
	Enabled    string `json:"enabled,omitempty"`
	Function   string `json:"function,omitempty"`
	Definition string `json:"definition,omitempty"`
	Comment    string `json:"comment,omitempty"`
}

// PolicyDef is a row level security policy of a table, or the predicate of a security
// policy of SQL Server on a table.
type PolicyDef struct {
	Name string `json:"name"`
	// Schema is the schema that holds the policy, where it is not the schema of the
	// table.
	Schema     string `json:"schema,omitempty"`
	Command    string `json:"command"`
	Permissive bool   `json:"permissive,omitempty"`
	Roles      string `json:"roles,omitempty"`
	Using      string `json:"using,omitempty"`
	WithCheck  string `json:"with_check,omitempty"`
	Disabled   bool   `json:"disabled,omitempty"`
	Comment    string `json:"comment,omitempty"`
}

// PartitionFunction says how the values of a column map to partitions.
type PartitionFunction struct {
	Name       string `json:"name"`
	DataType   string `json:"data_type"`
	Range      string `json:"range"`
	Boundaries string `json:"boundaries,omitempty"`
}

// PartitionScheme puts the partitions of a function on storage.
type PartitionScheme struct {
	Name       string `json:"name"`
	Function   string `json:"function"`
	Filegroups string `json:"filegroups,omitempty"`
}

// XMLSchema is an XML schema collection.
type XMLSchema struct {
	Name       string `json:"name"`
	Definition string `json:"definition,omitempty"`
	Comment    string `json:"comment,omitempty"`
}

// MaterializedViewLog is the log of the changes to a table. LogTable is the table that
// holds it, which the snapshot leaves out because the log makes it.
type MaterializedViewLog struct {
	Master     string `json:"master"`
	LogTable   string `json:"log_table"`
	PrimaryKey bool   `json:"primary_key,omitempty"`
	RowID      bool   `json:"row_id,omitempty"`
	Sequence   bool   `json:"sequence,omitempty"`
	ObjectID   bool   `json:"object_id,omitempty"`
	IncludeNew bool   `json:"include_new,omitempty"`
	// Columns are the columns that a person named, quoted and joined by commas.
	Columns string `json:"columns,omitempty"`
}

// RoleDef is a role that a grant names. MemberOf lists the roles that it is a member of.
type RoleDef struct {
	Name        string   `json:"name"`
	CanLogin    bool     `json:"can_login,omitempty"`
	Superuser   bool     `json:"superuser,omitempty"`
	CreateDB    bool     `json:"create_db,omitempty"`
	CreateRole  bool     `json:"create_role,omitempty"`
	Replication bool     `json:"replication,omitempty"`
	BypassRLS   bool     `json:"bypass_rls,omitempty"`
	NoInherit   bool     `json:"no_inherit,omitempty"`
	ConnLimit   int64    `json:"conn_limit,omitempty"`
	MemberOf    []string `json:"member_of,omitempty"`
	Comment     string   `json:"comment,omitempty"`
}

// GrantDef is one privilege on an object, or on one column of it, given to a role or an
// account. An empty Grantee is every role. Option says that the grantee can pass the
// privilege on, and Denied that the entry takes the privilege away.
type GrantDef struct {
	Schema    string `json:"schema"`
	Object    string `json:"object"`
	Kind      string `json:"kind"`
	Arguments string `json:"arguments,omitempty"`
	Grantee   string `json:"grantee,omitempty"`
	// GranteeType is user or role, for a product that tells them apart.
	GranteeType string `json:"grantee_type,omitempty"`
	Privilege   string `json:"privilege"`
	Column      string `json:"column,omitempty"`
	Option      bool   `json:"option,omitempty"`
	Denied      bool   `json:"denied,omitempty"`
}
