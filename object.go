package dbmeta

import "database/sql"

// This file declares the object kinds and the query for each one.
//
// There is one exported Query value per kind. A caller names the value, so the
// result type is inferred and a type the package does not know cannot be
// asked for. A model registers what its dialect provides for each value.
//
// A field a database may report as NULL is declared sql.Null[T] and never a
// plain T. An absent value is not an empty one: a table with no comment and a
// table whose comment is the empty string are different facts, and collapsing
// them shipped a real bug once. Read docs/NULLS.md before writing a query.
// That document is short and it is the one that cost the most to learn.
//
// Read Null.V to print a value, which is the zero value when absent and so
// behaves the way a plain field did. Read Null.Valid where the difference
// matters, and read Field.Present to tell an absent value from a column the
// server is too old to have.
//
// The object set comes from psql, which describes 49 kinds. Only the first few
// are declared here, because D13 builds the models before the API and the
// shape of the rest follows from them. See docs/QUERIES.md.

// Table is a table, a view, a materialized view or a sequence.
type Table struct {
	Catalog string
	Schema  string
	Name    string
	Type    string
	Comment sql.Null[string]
}

// Schema is a namespace within a catalog.
type Schema struct {
	Catalog string
	Name    string
	Owner   string
	Comment sql.Null[string]
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
	Default  sql.Null[string]
	// PrimaryKey reports whether the column is part of the primary key.
	//
	// psql does not print this and it is here under D47, because two
	// consumers read it per column and every database can answer it without a
	// second statement: MySQL has COLUMN_KEY, SQLite has the pk column of
	// pragma_table_xinfo, and PostgreSQL reaches it with one more join.
	PrimaryKey bool
	// Identity is the identity kind, empty when the column is not an identity.
	// PostgreSQL gained it in release 11, so an older server reports empty
	// under the padding rule and [Field.Min] says which is which.
	Identity sql.Null[string]
	// Generated is the generated kind, empty when the column is not
	// generated. PostgreSQL gained it in release 12.
	Generated sql.Null[string]
	Comment   sql.Null[string]
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
	Comment sql.Null[string]
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

// Database is a database within a server. psql lists them with \l.
type Database struct {
	Name       string
	Owner      string
	Encoding   string
	Collate    string
	CType      string
	Access     sql.Null[string]
	Tablespace sql.Null[string]
	Size       string
	Comment    sql.Null[string]
}

// Tablespace is a location the server stores data in. psql lists them with \db.
type Tablespace struct {
	Name     string
	Owner    string
	Location string
	Options  sql.Null[string]
	Size     string
	Access   sql.Null[string]
	Comment  sql.Null[string]
}

// AccessMethod is an index or table access method. psql lists them with \dA.
type AccessMethod struct {
	Name    string
	Type    string
	Handler string
	Comment sql.Null[string]
}

// Language is a procedural language. psql lists them with \dL.
type Language struct {
	Name      string
	Owner     string
	Trusted   bool
	Internal  bool
	Handler   string
	Validator string
	Inline    string
	Access    sql.Null[string]
	Comment   sql.Null[string]
}

// Conversion is an encoding conversion. psql lists them with \dc.
type Conversion struct {
	Schema  string
	Name    string
	Source  string
	Target  string
	Default bool
	Comment sql.Null[string]
}

// Cast converts one type to another. psql lists them with \dC.
type Cast struct {
	Source    string
	Target    string
	Function  string
	Implicit  string
	LeakProof bool
	Comment   sql.Null[string]
}

// Collation is a sorting rule. psql lists them with \dO.
type Collation struct {
	Schema        string
	Name          string
	Provider      sql.Null[string]
	Collate       string
	CType         string
	Locale        sql.Null[string]
	Deterministic bool
	Comment       sql.Null[string]
}

// LargeObject is a large object. psql lists them with \dl.
type LargeObject struct {
	OID     int64
	Owner   string
	Access  sql.Null[string]
	Comment sql.Null[string]
}

// EventTrigger fires on a DDL event. psql lists them with \dy.
type EventTrigger struct {
	Name     string
	Event    string
	Owner    string
	Enabled  string
	Function string
	Tags     sql.Null[string]
	Comment  sql.Null[string]
}

// Setting is a configuration parameter. psql lists them with \dconfig.
type Setting struct {
	Name    string
	Value   sql.Null[string]
	Type    sql.Null[string]
	Context sql.Null[string]
	Access  sql.Null[string]
}

// Queries for the objects above.
var (
	// Databases lists the databases of a server.
	Databases = NewQuery[Database]("databases")
	// Tablespaces lists the places a server stores data.
	Tablespaces = NewQuery[Tablespace]("tablespaces")
	// AccessMethods lists the index and table access methods.
	AccessMethods = NewQuery[AccessMethod]("access_methods")
	// Languages lists the procedural languages.
	Languages = NewQuery[Language]("languages")
	// Conversions lists the encoding conversions.
	Conversions = NewQuery[Conversion]("conversions")
	// Casts lists the conversions between types.
	Casts = NewQuery[Cast]("casts")
	// Collations lists the sorting rules.
	Collations = NewQuery[Collation]("collations")
	// LargeObjects lists the large objects.
	LargeObjects = NewQuery[LargeObject]("large_objects")
	// EventTriggers lists the triggers that fire on a DDL event.
	EventTriggers = NewQuery[EventTrigger]("event_triggers")
	// Settings lists the configuration parameters.
	Settings = NewQuery[Setting]("settings")
)

// Function is a function, procedure, aggregate or window function. psql lists
// them with \df and aggregates alone with \da.
type Function struct {
	Catalog string
	Schema  string
	Name    string
	// ID identifies the routine where a name does not. PostgreSQL overloads a
	// name, so [RoutineParameters] joins on this rather than on Name. A
	// database that does not overload reports it absent.
	ID         sql.Null[string]
	Kind       string
	ResultType string
	ArgTypes   string
	Volatility string
	Parallel   string
	Owner      string
	Security   string
	Access     sql.Null[string]
	Language   string
	Source     sql.Null[string]
	Comment    sql.Null[string]
}

// Type is a data type. psql lists them with \dT.
type Type struct {
	Catalog  string
	Schema   string
	Name     string
	Internal string
	Kind     string
	Elements string
	Owner    string
	Access   sql.Null[string]
	Comment  sql.Null[string]
}

// Domain is a type with a constraint. psql lists them with \dD.
type Domain struct {
	Catalog     string
	Schema      string
	Name        string
	DataType    string
	Collation   string
	Nullable    bool
	Default     sql.Null[string]
	Constraints string
	Access      sql.Null[string]
	Comment     sql.Null[string]
}

// Operator is an operator. psql lists them with \do.
type Operator struct {
	Schema     string
	Name       string
	LeftType   string
	RightType  string
	ResultType string
	Function   string
	Comment    sql.Null[string]
}

// Queries for routines and types.
var (
	// Functions lists functions, procedures, aggregates and window functions.
	Functions = NewQuery[Function]("functions")
	// Aggregates lists aggregate functions alone.
	Aggregates = NewQuery[Function]("aggregates")
	// Types lists data types.
	Types = NewQuery[Type]("types")
	// Domains lists types that carry a constraint.
	Domains = NewQuery[Domain]("domains")
	// Operators lists operators.
	Operators = NewQuery[Operator]("operators")
)

// Role is a database role. psql lists them with \du and \dg.
type Role struct {
	Name        string
	Superuser   bool
	CreateRole  bool
	CreateDB    bool
	CanLogin    bool
	Replication bool
	BypassRLS   bool
	Inherit     bool
	ConnLimit   int64
	ValidUntil  sql.Null[string]
	MemberOf    string
	Comment     sql.Null[string]
}

// RoleSetting is a configuration value set for a role, optionally in one
// database. psql lists them with \drds.
type RoleSetting struct {
	Role     string
	Database string
	Settings sql.Null[string]
}

// RoleGrant is one role's membership of another. psql lists them with \drg.
type RoleGrant struct {
	Role     string
	MemberOf string
	Grantor  sql.Null[string]
	Admin    bool
	Inherit  bool
	Set      bool
}

// Privilege is the grants on one object. psql lists them with \z and \dp.
type Privilege struct {
	Schema       string
	Name         string
	Type         string
	Access       sql.Null[string]
	ColumnAccess string
	Policies     string
}

// DefaultACL is a default grant applied to objects created later. psql lists
// them with \ddp.
type DefaultACL struct {
	Owner  string
	Schema string
	Type   string
	Access sql.Null[string]
}

// ForeignDataWrapper reaches data outside the database. psql lists them with
// \dew.
type ForeignDataWrapper struct {
	Name      string
	Owner     string
	Handler   sql.Null[string]
	Validator sql.Null[string]
	Access    sql.Null[string]
	Options   sql.Null[string]
	Comment   sql.Null[string]
}

// ForeignServer is a server reached through a wrapper. psql lists them with
// \des.
type ForeignServer struct {
	Name    string
	Owner   string
	Wrapper string
	Type    sql.Null[string]
	Version sql.Null[string]
	Access  sql.Null[string]
	Options sql.Null[string]
	Comment sql.Null[string]
}

// User is who a connection is authenticated as.
//
// Most products answer two names rather than one. The effective user is who
// the session acts as now, and the session user is who it authenticated as.
// They differ after SET ROLE on PostgreSQL, and on SQL Server they are always
// two different things: a connection authenticates as a server login and acts
// as a database user, so sa acts as dbo.
//
// Both come from one statement on every product that has them, so both are
// reported. See D47.
type User struct {
	// Name is the effective user, which is what a client shows.
	Name string
	// Session is who the connection authenticated as, and is absent on a
	// product that does not separate the two.
	Session sql.Null[string]
}

// UserMapping maps a local role onto a foreign server. psql lists them with
// \deu.
type UserMapping struct {
	Server  string
	Name    sql.Null[string]
	Options sql.Null[string]
}

// ForeignTable is a table on a foreign server. psql lists them with \det.
type ForeignTable struct {
	Schema  string
	Name    string
	Server  string
	Options sql.Null[string]
	Comment sql.Null[string]
}

// Queries for roles, privileges and foreign data.
var (
	// Roles lists the database roles.
	Roles = NewQuery[Role]("roles")
	// RoleSettings lists the configuration set for a role.
	RoleSettings = NewQuery[RoleSetting]("role_settings")
	// RoleGrants lists role memberships.
	RoleGrants = NewQuery[RoleGrant]("role_grants")
	// Privileges lists the grants on tables and their columns.
	Privileges = NewQuery[Privilege]("privileges")
	// DefaultACLs lists the grants applied to objects created later.
	DefaultACLs = NewQuery[DefaultACL]("default_acls")
	// ForeignDataWrappers lists the wrappers that reach outside data.
	ForeignDataWrappers = NewQuery[ForeignDataWrapper]("foreign_data_wrappers")
	// ForeignServers lists the servers reached through a wrapper.
	ForeignServers = NewQuery[ForeignServer]("foreign_servers")
	// UserMappings lists the local roles mapped onto a foreign server.
	UserMappings = NewQuery[UserMapping]("user_mappings")
	// ForeignTables lists the tables on a foreign server.
	ForeignTables = NewQuery[ForeignTable]("foreign_tables")
)

// Publication is a set of changes offered for replication. psql lists them
// with \dRp.
type Publication struct {
	Name      string
	Owner     string
	AllTables bool
	Insert    bool
	Update    bool
	Delete    bool
	Truncate  bool
	ViaRoot   bool
	Comment   sql.Null[string]
}

// PublicationTable is one table a publication offers. psql shows them with
// \dRp+.
type PublicationTable struct {
	Publication string
	Schema      string
	Name        string
	Columns     string
	Where       sql.Null[string]
}

// Subscription receives changes from a publication. psql lists them with \dRs.
type Subscription struct {
	Name         string
	Owner        string
	Enabled      bool
	Publications string
	Synchronous  sql.Null[string]
	Slot         sql.Null[string]
	Comment      sql.Null[string]
}

// TextSearchParser splits text into tokens. psql lists them with \dFp.
type TextSearchParser struct {
	Schema  string
	Name    string
	Comment sql.Null[string]
}

// TextSearchDictionary normalizes tokens. psql lists them with \dFd.
type TextSearchDictionary struct {
	Schema   string
	Name     string
	Template string
	Options  sql.Null[string]
	Comment  sql.Null[string]
}

// TextSearchTemplate is the code behind a dictionary. psql lists them with
// \dFt.
type TextSearchTemplate struct {
	Schema  string
	Name    string
	Init    sql.Null[string]
	Lexize  string
	Comment sql.Null[string]
}

// TextSearchConfig ties a parser to dictionaries. psql lists them with \dF.
type TextSearchConfig struct {
	Schema  string
	Name    string
	Parser  string
	Comment sql.Null[string]
}

// OperatorClass tells an access method how to index a type. psql lists them
// with \dAc.
type OperatorClass struct {
	AccessMethod string
	Schema       string
	Name         string
	InputType    string
	Default      bool
	Family       string
	Owner        string
}

// OperatorFamily groups operator classes. psql lists them with \dAf.
type OperatorFamily struct {
	AccessMethod string
	Schema       string
	Name         string
	Owner        string
}

// OperatorFamilyOperator is one operator of a family. psql lists them with
// \dAo.
type OperatorFamilyOperator struct {
	AccessMethod string
	Family       string
	Operator     string
	Strategy     int64
	Purpose      string
}

// OperatorFamilyFunction is one support function of a family. psql lists them
// with \dAp.
type OperatorFamilyFunction struct {
	AccessMethod string
	Family       string
	LeftType     string
	RightType    string
	Number       int64
	Function     string
}

// Extension is an installed extension. psql lists them with \dx.
type Extension struct {
	Name    string
	Version string
	Schema  string
	Comment sql.Null[string]
}

// ExtensionObject is one object an extension owns. psql lists them with \dx+.
type ExtensionObject struct {
	Extension   string
	Description string
}

// ExtendedStat is a statistics object over several columns. psql lists them
// with \dX.
type ExtendedStat struct {
	Schema  string
	Name    string
	Owner   string
	Table   string
	Kinds   string
	Comment sql.Null[string]
}

// Comment is a comment on any object. psql shows them with \dd.
type Comment struct {
	Schema  string
	Name    string
	Type    string
	Comment string
}

// Queries for replication, text search, operator families and extensions.
var (
	// Publications lists the sets of changes offered for replication.
	Publications = NewQuery[Publication]("publications")
	// PublicationTables lists the tables each publication offers.
	PublicationTables = NewQuery[PublicationTable]("publication_tables")
	// Subscriptions lists the subscriptions to a publication.
	Subscriptions = NewQuery[Subscription]("subscriptions")
	// TextSearchParsers lists the parsers that split text into tokens.
	TextSearchParsers = NewQuery[TextSearchParser]("text_search_parsers")
	// TextSearchDictionaries lists the dictionaries that normalize tokens.
	TextSearchDictionaries = NewQuery[TextSearchDictionary]("text_search_dictionaries")
	// TextSearchTemplates lists the code behind the dictionaries.
	TextSearchTemplates = NewQuery[TextSearchTemplate]("text_search_templates")
	// TextSearchConfigs lists the configurations.
	TextSearchConfigs = NewQuery[TextSearchConfig]("text_search_configs")
	// OperatorClasses lists the operator classes.
	OperatorClasses = NewQuery[OperatorClass]("operator_classes")
	// OperatorFamilies lists the operator families.
	OperatorFamilies = NewQuery[OperatorFamily]("operator_families")
	// OperatorFamilyOperators lists the operators of each family.
	OperatorFamilyOperators = NewQuery[OperatorFamilyOperator]("operator_family_operators")
	// OperatorFamilyFunctions lists the support functions of each family.
	OperatorFamilyFunctions = NewQuery[OperatorFamilyFunction]("operator_family_functions")
	// Extensions lists the installed extensions.
	Extensions = NewQuery[Extension]("extensions")
	// ExtensionObjects lists the objects each extension owns.
	ExtensionObjects = NewQuery[ExtensionObject]("extension_objects")
	// ExtendedStats lists the statistics objects over several columns.
	ExtendedStats = NewQuery[ExtendedStat]("extended_stats")
	// Comments lists the comments on objects.
	Comments = NewQuery[Comment]("comments")
)

// IndexColumn is one column of an index, in index order.
type IndexColumn struct {
	Schema     string
	Table      string
	Index      string
	Name       string
	Ordinal    int64
	Expression sql.Null[string]
	Descending bool
}

// Constraint is a check, unique, primary key, foreign key or exclusion
// constraint. psql shows them inside \d name.
//
// Definition is absent where the database records no expression for the kind
// of constraint. Read through information_schema, only a check constraint has
// one, because check_clause is the only expression the standard records.
type Constraint struct {
	Schema     string
	Table      string
	Name       string
	Type       string
	Definition sql.Null[string]
	Deferrable bool
	Deferred   bool
	Comment    sql.Null[string]
}

// Trigger fires on a change to a table. psql shows them inside \d name.
type Trigger struct {
	Schema     string
	Table      string
	Name       string
	Enabled    string
	Definition string
	Comment    sql.Null[string]
}

// Sequence generates numbers. psql lists them with \ds and shows the detail
// inside \d name.
type Sequence struct {
	Schema    string
	Name      string
	DataType  sql.Null[string]
	Start     sql.Null[int64]
	Minimum   sql.Null[int64]
	Maximum   sql.Null[int64]
	Increment sql.Null[int64]
	Cycles    sql.Null[bool]
	OwnedBy   string
	Comment   sql.Null[string]
}

// PartitionedTable is a table split into partitions. psql lists them with \dP.
type PartitionedTable struct {
	Schema     string
	Name       string
	Owner      string
	Type       string
	Parent     string
	Strategy   string
	Expression string
	Comment    sql.Null[string]
}

// Queries for the detail of a table.
var (
	// IndexColumns lists the columns of each index, in index order.
	IndexColumns = NewQuery[IndexColumn]("index_columns")
	// Constraints lists the constraints on a table.
	Constraints = NewQuery[Constraint]("constraints")
	// Triggers lists the triggers on a table.
	Triggers = NewQuery[Trigger]("triggers")
	// Sequences lists sequences and their bounds.
	Sequences = NewQuery[Sequence]("sequences")
	// PartitionedTables lists the tables split into partitions.
	PartitionedTables = NewQuery[PartitionedTable]("partitioned_tables")
)

// The kinds below are not in psql. They exist because a consumer measured in
// D46 needs them and no psql command shows them, which D47 allows: psql sets
// the object model and does not set the column set.
//
// Each one says whether it is durable, session dependent or runtime, because a
// caller holding the value has to know how long it stays true.

// ConstraintColumn is one column of a constraint, in constraint order.
//
// It is durable. psql prints a constraint as one line of text, which
// [Constraint.Definition] still holds, and this is the same fact in parts. A
// code generator reads a foreign key from here, because parsing a rendered
// definition is not something to rely on.
//
// A composite key is several rows sharing Constraint, told apart by Ordinal.
// The Foreign fields are set only for a foreign key and name what the column
// points at.
type ConstraintColumn struct {
	Catalog    string
	Schema     string
	Table      string
	Constraint string
	Name       string
	Ordinal    int64

	ForeignCatalog sql.Null[string]
	ForeignSchema  sql.Null[string]
	ForeignTable   sql.Null[string]
	ForeignName    sql.Null[string]
}

// RoutineParameter is one parameter of a function or a procedure, in
// declaration order.
//
// It is durable. [Function.ArgTypes] still holds the signature psql prints,
// and this is the same fact in parts.
//
// Group by Routine and, where the database overloads a name, by RoutineID.
// PostgreSQL allows two functions with one name and different parameters, so
// the name alone does not identify one.
type RoutineParameter struct {
	Catalog   string
	Schema    string
	Routine   string
	RoutineID sql.Null[string]
	Name      sql.Null[string]
	Ordinal   int64
	// Mode is in, out, inout, variadic or table for a column of a returned
	// table. A return value reports mode return and ordinal zero where the
	// database records one.
	Mode     string
	DataType string
	Default  sql.Null[string]
}

// EnumValue is one label of an enumerated type, in sort order.
//
// It is durable. [Type.Elements] holds the labels joined into one string,
// which is what psql prints, and this is the same fact in rows. A label can
// contain a comma, so splitting that string is not a substitute.
//
// MySQL has no enum type, only an enum column, so a model there reports the
// column as the enum and its name is the column name.
type EnumValue struct {
	Catalog string
	Schema  string
	Enum    string
	Label   string
	Ordinal int64
}

// View is a view and the statement that defines it.
//
// It is durable. The definition is what the catalog stores rather than a
// rendering of it, so it is text and that is the structured answer.
//
// It is a separate kind rather than a field on [Table] on purpose. Reaching
// the definition costs a join or a function call per row, and a caller listing
// tables does not want to pay it.
type View struct {
	Catalog    string
	Schema     string
	Name       string
	Definition string
	// CheckOption is none, local or cascaded.
	CheckOption sql.Null[string]
	Updatable   sql.Null[bool]
	Insertable  sql.Null[bool]
	Comment     sql.Null[string]
}

// ColumnStat is what the planner knows about the values in a column.
//
// It is runtime and it can be stale. Every database here computes it when
// asked, by ANALYZE or its equivalent, and not when the data changes. A column
// never analyzed has no row.
//
// usql shows this with \ss, which psql does not have.
type ColumnStat struct {
	Catalog string
	Schema  string
	Table   string
	Name    string

	// AvgWidth is the average size of a value in bytes.
	AvgWidth sql.Null[int64]
	// NullFrac is the fraction of values that are null, from zero to one.
	NullFrac sql.Null[float64]
	// Distinct is the number of distinct values. A negative number is a
	// fraction of the row count, which is how PostgreSQL records a column
	// whose distinct count grows with the table.
	Distinct sql.Null[float64]

	Min  sql.Null[string]
	Max  sql.Null[string]
	Mean sql.Null[string]
	// TopN holds the most common values and TopNFreqs their frequencies, both
	// as text, one entry per line. They are the same length.
	TopN      sql.Null[string]
	TopNFreqs sql.Null[string]
}

// Queries for the kinds psql has no command for.
var (
	// ConstraintColumns lists the columns of each constraint, in order.
	ConstraintColumns = NewQuery[ConstraintColumn]("constraint_columns")
	// RoutineParameters lists the parameters of each function and procedure.
	RoutineParameters = NewQuery[RoutineParameter]("routine_parameters")
	// EnumValues lists the labels of each enumerated type, in sort order.
	EnumValues = NewQuery[EnumValue]("enum_values")
	// Views lists views and the statements that define them.
	Views = NewQuery[View]("views")
	// ColumnStats lists what the planner knows about a column's values.
	ColumnStats = NewQuery[ColumnStat]("column_stats")
	// CurrentSchema returns the schema an unqualified name resolves in.
	//
	// It is session dependent. It answers one row and it is the one kind here
	// that describes the connection rather than the database. A caller reads
	// it with [First].
	CurrentSchema = NewQuery[Schema]("current_schema")
	// CurrentUser returns who the connection is authenticated as.
	//
	// Like [CurrentSchema] it is session dependent, answers one row and
	// describes the connection rather than the database. A caller reads it
	// with [First].
	//
	// It is here because it is database specific SQL that a client otherwise
	// carries itself: usql writes SELECT current_user for most products and
	// SELECT user FROM dual for Oracle. See D55.
	CurrentUser = NewQuery[User]("current_user")
)
