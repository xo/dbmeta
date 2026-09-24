package dbmeta

// This file declares the object kinds and the query for each one.
//
// There is one exported Query value per kind. A caller names the value, so the
// result type is inferred and a type the package does not know cannot be
// asked for. A model registers what its dialect provides for each value.
//
// The object set comes from psql, which describes 49 kinds. Only the first few
// are declared here, because D13 builds the models before the API and the
// shape of the rest follows from them. See QUERIES.md.

// Table is a table, a view, a materialized view or a sequence.
type Table struct {
	Catalog string
	Schema  string
	Name    string
	Type    string
	Comment string
}

// Schema is a namespace within a catalog.
type Schema struct {
	Catalog string
	Name    string
	Owner   string
	Comment string
}

// Column is one column of a table.
type Column struct {
	Catalog  string
	Schema   string
	Table    string
	Name     string
	Ordinal  int
	DataType string
	Nullable bool
	Default  string
	// Identity is the identity kind, empty when the column is not an identity.
	// PostgreSQL gained it in release 11, so an older server reports empty
	// under the padding rule and [Field.Min] says which is which.
	Identity string
	// Generated is the generated kind, empty when the column is not
	// generated. PostgreSQL gained it in release 12.
	Generated string
	Comment   string
}

// Index is an index on a table.
type Index struct {
	Catalog string
	Schema  string
	Table   string
	Name    string
	Type    string
	Unique  bool
	Primary bool
	Comment string
}

// Queries. One value per object kind.
var (
	// Tables lists tables and the relations that behave like them.
	Tables = NewQuery[Table]("tables")
	// Schemas lists namespaces.
	Schemas = NewQuery[Schema]("schemas")
	// Columns lists the columns of a table.
	Columns = NewQuery[Column]("columns")
	// Indexes lists the indexes of a table. No model provides it yet.
	Indexes = NewQuery[Index]("indexes")
)
