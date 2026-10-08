package dbmeta

import "database/sql"

// This file declares the object kinds and the query for each one.
//
// There is one exported Query value per kind. A caller names the value, so the
// result type is inferred and a type the package does not know cannot be
// asked for. A model registers what its dialect provides for each value.
//
// A field a database can report as NULL is declared sql.Null[T] and never a
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
// The object set comes from psql, which describes 49 kinds, and later
// decisions such as D46 and D55 added the kinds psql has no command for. D13
// built the models before the API, so the shape of each kind follows from
// what the models answer. See docs/QUERIES.md.

// Table is a table, a view, a materialized view or a sequence.
type Table struct {
	Catalog string
	Schema  string
	Name    string
	Type    string
	Comment sql.Null[string]
	// Owner is the role that owns the relation. It is absent where the
	// product has no owner of a relation. See D198.
	Owner sql.Null[string]
	// Persistence is permanent, unlogged or temporary, as psql's \dt+ says
	// it. It is absent where the product has no such choice. See D198.
	Persistence sql.Null[string]
	// AccessMethod is the table access method, such as heap, and absent for
	// a relation that has none, such as a view, or on a server that records
	// none. PostgreSQL has it from release 12. See D198.
	AccessMethod sql.Null[string]
	// Size is the bytes the relation takes on disk, as pg_table_size counts
	// them: the main data, the free space map, the visibility map and the
	// TOAST table, and not the indexes. It is a number and not the text psql
	// prints, because a caller can format a number and cannot add up text.
	// A view and a foreign table have no file here, so PostgreSQL answers 0
	// for them. It is absent where the relation was dropped while the
	// statement ran, and where the product keeps no size. See D198.
	Size sql.Null[int64]
	// Rows is the planner's estimate of the rows, which is the number
	// ANALYZE and VACUUM last stored. It is not a count. PostgreSQL before
	// release 14 stores 0 for a relation that was never analyzed, and from
	// 14 stores -1, so that 0 means an empty relation. See D198.
	Rows sql.Null[int64]
	// Options are the storage parameters set on the relation, such as
	// fillfactor=70 or security_barrier=true, joined by a comma and a space.
	// PostgreSQL adds the parameters of the TOAST table with a prefix of
	// toast, as psql does. It is absent when none is set. See D199.
	Options sql.Null[string]
	// RowSecurity reports whether row level security is on for the table.
	// It is absent where the product has none. See D199.
	RowSecurity sql.Null[bool]
	// RowSecurityForced reports whether row level security also applies to
	// the owner of the table. See D199.
	RowSecurityForced sql.Null[bool]
}

// Schema is a namespace within a catalog.
type Schema struct {
	Catalog string
	Name    string
	Owner   string
	Comment sql.Null[string]
	// Access is the access privileges of the schema, one per line, as psql's
	// \dn+ prints them. It is absent when the privileges are the default.
	// See D201.
	Access sql.Null[string]
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
	// PostgreSQL gained it in release 11, so an older server reports it absent
	// under the padding rule, and [Field.Present] says which is which.
	Identity sql.Null[string]
	// Generated is the generated kind, empty when the column is not
	// generated. PostgreSQL gained it in release 12.
	Generated sql.Null[string]
	Comment   sql.Null[string]
	// Collation is the name of the column's collation, and absent for a
	// type that has none or a product that records none. It is here under
	// D47, because psql prints it in \d and it is a column of the same
	// catalog row. psql prints it only where it differs from the type's
	// default, and a caller decides that. See D139.
	Collation sql.Null[string]
	// Storage is how PostgreSQL stores a value that is too large for a page:
	// plain, main, external or extended. It is absent where the product has
	// no such choice. See D198.
	Storage sql.Null[string]
	// Compression is the compression method that is set on the column, such
	// as pglz or lz4. It is absent when the column uses the server default,
	// and on a server that has no such setting, which is every release before
	// 14. [Field.Present] says which. See D198.
	Compression sql.Null[string]
	// StatsTarget is the statistics target that was set on the column with
	// ALTER TABLE, and absent when the column uses the default. See D198.
	StatsTarget sql.Null[int64]
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
	// Owner is the role that owns the index. See D198.
	Owner sql.Null[string]
	// Persistence is permanent, unlogged or temporary. See D198.
	Persistence sql.Null[string]
	// Size is the bytes the index takes on disk, as pg_table_size counts
	// them. It is absent if the index was dropped while the statement ran.
	// See D198.
	Size sql.Null[int64]
	// Predicate is the WHERE clause of a partial index, as pg_get_expr writes
	// it, and absent for an index that covers every row. See D198.
	Predicate sql.Null[string]
	// Valid is false while an index is built with CONCURRENTLY, and after
	// that build failed. The planner does not use an index that is not valid.
	Valid sql.Null[bool]
	// Clustered reports whether the table was last clustered on this index.
	Clustered sql.Null[bool]
	// ReplicaIdentity reports whether this index is the table's replica
	// identity for logical replication.
	ReplicaIdentity sql.Null[bool]
	// Deferrable and InitiallyDeferred describe the constraint that owns the
	// index, and are absent for an index no constraint owns. See D198.
	Deferrable        sql.Null[bool]
	InitiallyDeferred sql.Null[bool]
	// Options are the storage parameters set on the index, such as
	// fillfactor=70, joined by a comma and a space. See D199.
	Options sql.Null[string]
	// Definition is the whole CREATE INDEX statement, as pg_get_indexdef
	// writes it with the pretty flag, and Using is the part of it that follows
	// the first " USING ", which is what psql prints in the Indexes footer of
	// \d name. Both keep the operator class and the collation of a key column
	// that is not the default. See D201.
	Definition sql.Null[string]
	Using      sql.Null[string]
	// ConstraintType is p, u or x for an index that a primary key, a unique
	// constraint or an exclusion constraint owns, and absent for any other
	// index. ConstraintDefinition is the text of that constraint, as
	// pg_get_constraintdef writes it, such as EXCLUDE USING gist (...). psql
	// prints it in place of the index text for an exclusion constraint and for
	// a constraint WITHOUT OVERLAPS, which ConstraintPeriod reports. It is
	// false below release 18, which has none. See D201.
	ConstraintType       sql.Null[string]
	ConstraintDefinition sql.Null[string]
	ConstraintPeriod     sql.Null[bool]
	// TableVisible reports that the table is on the search path of the
	// session, so that psql prints its name with no schema. See D201.
	TableVisible sql.Null[bool]
}

// Queries. One value per object kind.
var (
	// Tables lists tables and the relations that behave like them.
	Tables = NewQuery[Table]("tables")
	// Schemas lists namespaces.
	Schemas = NewQuery[Schema]("schemas")
	// Columns lists the columns of a table.
	Columns = NewQuery[Column]("columns")
	// Indexes lists the indexes of a table.
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
	Size       sql.Null[string]
	Comment    sql.Null[string]
	// LocaleProvider is libc, icu or builtin. It is libc below release 15,
	// which had no other provider. Locale is the ICU or builtin locale, and
	// absent for libc. ICURules are the tailoring rules of an ICU locale, from
	// release 16. See D201.
	LocaleProvider sql.Null[string]
	Locale         sql.Null[string]
	ICURules       sql.Null[string]
}

// Tablespace is a location the server stores data in. psql lists them with \db.
type Tablespace struct {
	Name     string
	Owner    sql.Null[string]
	Location sql.Null[string]
	Options  sql.Null[string]
	Size     sql.Null[string]
	Access   sql.Null[string]
	Comment  sql.Null[string]
}

// AccessMethod is an index or table access method. psql lists them with \dA.
type AccessMethod struct {
	Name    string
	Type    string
	Handler sql.Null[string]
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
	Function  sql.Null[string]
	Implicit  string
	LeakProof sql.Null[bool]
	Comment   sql.Null[string]
}

// Collation is a sorting rule. psql lists them with \dO.
type Collation struct {
	Schema        string
	Name          string
	Provider      sql.Null[string]
	Collate       sql.Null[string]
	CType         sql.Null[string]
	Locale        sql.Null[string]
	Deterministic sql.Null[bool]
	Comment       sql.Null[string]
	// Rules is the tailoring rules of an ICU collation, as psql's \dO+
	// prints them. PostgreSQL 16 added them. See D147.
	Rules sql.Null[string]
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
	// Display is the value as the server shows it, with its unit, such as
	// 128MB where Value holds 16384. psql's \dconfig prints it. See D147.
	Display sql.Null[string]
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
	ResultType sql.Null[string]
	ArgTypes   sql.Null[string]
	Volatility string
	Parallel   string
	Owner      sql.Null[string]
	Security   string
	Access     sql.Null[string]
	Language   string
	// Source is what the product keeps apart from the whole statement: the
	// body, or for a PostgreSQL function in C or internal code, the name of
	// the C function it runs. It is absent
	// where the product keeps only the whole statement, which is Definition.
	// See D147.
	Source  sql.Null[string]
	Comment sql.Null[string]
	// Definition is the whole statement that makes the routine, such as
	// pg_get_functiondef returns, which psql's \sf prints. It is absent where
	// the product keeps only the body, which is Source, or keeps nothing.
	// See D147.
	Definition sql.Null[string]
	// Leakproof reports whether the planner can trust the function not to
	// reveal its arguments in an error. It is false where the product has no
	// such property. See D198.
	Leakproof bool
	// Prosrc is the text PostgreSQL stores in pg_proc.prosrc for every
	// language: the body for SQL, PL/pgSQL and the other languages, and the
	// C function name or the link symbol for internal and C. Source holds it
	// only for the last two. It is absent where the product keeps no such
	// column. See D198.
	Prosrc sql.Null[string]
}

// Type is a data type. psql lists them with \dT.
type Type struct {
	Catalog  string
	Schema   string
	Name     string
	Internal string
	Kind     string
	Elements string
	Owner    sql.Null[string]
	Access   sql.Null[string]
	Comment  sql.Null[string]
	// Size is the internal length of a value in bytes, var for a type whose
	// values vary in length, and tuple for a composite type, as psql's \dT+
	// prints it. See D147.
	Size sql.Null[string]
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
	Function   sql.Null[string]
	Comment    sql.Null[string]
	// Leakproof reports that the function behind the operator is leakproof.
	// It is false for an operator with no function. See D201.
	Leakproof bool
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
	Role     sql.Null[string]
	Database sql.Null[string]
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
	Schema       sql.Null[string]
	Name         string
	Type         string
	Access       sql.Null[string]
	ColumnAccess sql.Null[string]
	Policies     sql.Null[string]
}

// DefaultACL is a default grant applied to objects created later. psql lists
// them with \ddp.
type DefaultACL struct {
	Owner  string
	Schema sql.Null[string]
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

// ForeignOption is one option of a foreign data wrapper, a foreign server, a
// user mapping or a foreign table. The Options field of each of them is the
// catalog text, name=value joined by a comma and a space, which cannot be
// split back when a value holds that text. This is the same fact in parts, one
// row for each option, as D47 asks. See D201.
//
// Kind is foreign data wrapper, foreign server, user mapping or foreign table.
// Name is the name of the object, and for a user mapping it is the name of the
// local role, public for every role. Schema is set for a foreign table only.
// Server is set for a user mapping and a foreign table. Ordinal is the
// position of the option in the list the catalog holds. Quoted is the option
// as psql prints it in the parentheses, such as "user" 'u', with the name
// quoted when it needs it and the value always quoted.
type ForeignOption struct {
	Kind    string
	Schema  sql.Null[string]
	Name    sql.Null[string]
	Server  sql.Null[string]
	Ordinal int64
	Option  string
	Value   string
	Quoted  string
}

// ColumnPrivilege is one entry of the access privileges of a column, which
// psql prints under its name in the Column privileges of \dp. It is one row
// for each entry, in the order the catalog holds them, so a column with two
// grantees is two rows. Access is the entry as psql prints it, such as
// pd_writer=w/postgres. Grantee is absent for public. Privileges are the
// letters of the entry, such as arw*. See D201.
type ColumnPrivilege struct {
	Schema     string
	Table      string
	Column     string
	Ordinal    int64
	Access     string
	Grantee    sql.Null[string]
	Grantor    sql.Null[string]
	Privileges string
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
	// ForeignOptions lists the options of the foreign data wrappers, the
	// foreign servers, the user mappings and the foreign tables, one row for
	// each option. See D201.
	ForeignOptions = NewQuery[ForeignOption]("foreign_options")
	// ColumnPrivileges lists the access privileges of the columns, one row for
	// each entry. See D201.
	ColumnPrivileges = NewQuery[ColumnPrivilege]("column_privileges")
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
	// GeneratedColumns is none or stored, which says whether the publication
	// sends generated columns. It is none below release 18, which cannot send them.
	// See D201.
	GeneratedColumns sql.Null[string]
}

// PublicationTable is one table a publication offers. psql shows them with
// \dRp+.
type PublicationTable struct {
	Publication string
	Schema      string
	Name        string
	Columns     string
	Where       sql.Null[string]
	// Via says how the table is in the publication: table when the
	// publication names it, schema when it names the schema of the table, and
	// all tables when it names every table. Only table is read unless the
	// caller asks for the others. See D199.
	Via sql.Null[string]
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
	// The columns below are the ones psql's \dRs+ adds. Each is absent on a
	// release that has no such property, because a subscription of that
	// release did not choose. Binary arrives in release 14, DisableOnError,
	// TwoPhase and SkipLSN in 15, Origin, PasswordRequired and RunAsOwner in
	// 16, and Failover in 17. Streaming arrives in 14 as on or off and in 16
	// as on, off or parallel. TwoPhase is disabled, pending or enabled. See
	// D201.
	Binary           sql.Null[bool]
	Streaming        sql.Null[string]
	TwoPhase         sql.Null[string]
	DisableOnError   sql.Null[bool]
	Origin           sql.Null[string]
	PasswordRequired sql.Null[bool]
	RunAsOwner       sql.Null[bool]
	Failover         sql.Null[bool]
	SkipLSN          sql.Null[string]
}

// SubscriptionConnection is the connection string of a subscription, which
// psql's \dRs+ prints in Conninfo. It is a kind of its own because only a
// superuser can read the column, and a statement that names it is refused for
// every other role. Subscription is refused with it. See D201.
type SubscriptionConnection struct {
	Name     string
	Conninfo string
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
	// TemplateSchema is the schema of Template. psql's \dFd+ prints the two
	// joined, such as pg_catalog.simple. See D201.
	TemplateSchema sql.Null[string]
}

// TextSearchParserFunction is one of the five functions of a text search
// parser, which psql's \dFp+ prints as Method and Function. Method is start,
// token, end, headline or lextype, in the order psql prints them. Function is
// the name as regproc writes it, and Comment the comment on the function. See
// D201.
type TextSearchParserFunction struct {
	Schema   string
	Parser   string
	Method   string
	Ordinal  int64
	Function string
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
	// ParserSchema is the schema of Parser. psql's \dF+ prints the two
	// joined, such as pg_catalog.default. See D147.
	ParserSchema sql.Null[string]
}

// TextSearchConfigMap is one dictionary that a configuration consults for one
// kind of token. A configuration consults the dictionaries for a token in the
// order of Position, and psql's \dF+ joins them into one line. See D147.
type TextSearchConfigMap struct {
	Schema           string
	Config           string
	Token            string
	Position         int64
	DictionarySchema string
	Dictionary       string
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
	// StorageType is the type the index stores, where it differs from the
	// input type, as psql's \dAc+ prints it. See D147.
	StorageType sql.Null[string]
	// Visible and FamilyVisible report that the class and its family are on
	// the search path of the session, so that psql prints their names with no
	// schema. FamilySchema is the schema of Family. See D201.
	Visible       sql.Null[bool]
	FamilyVisible sql.Null[bool]
	FamilySchema  string
}

// OperatorFamily groups operator classes. psql lists them with \dAf.
type OperatorFamily struct {
	AccessMethod string
	Schema       string
	Name         string
	Owner        string
	// AppliesTo is the input types of the family's operator classes, joined
	// by commas, as psql's \dAf prints them. See D147.
	AppliesTo sql.Null[string]
	// Visible reports that the family is on the search path of the session,
	// so that psql prints its name with no schema. See D201.
	Visible sql.Null[bool]
}

// OperatorFamilyOperator is one operator of a family. psql lists them with
// \dAo.
type OperatorFamilyOperator struct {
	AccessMethod string
	Family       string
	Operator     string
	Strategy     int64
	Purpose      string
	// LeftType and RightType are the registered types, which with Strategy
	// name one operator of the family, and by which psql orders the rows.
	LeftType  string
	RightType string
	// SortFamily is the operator family an ordering operator sorts by, and
	// absent for a search operator. Leakproof reports that the function of the
	// operator is leakproof. psql's \dAo+ prints both. See D201.
	SortFamily sql.Null[string]
	Leakproof  bool
	// Visible reports that the family is on the search path of the session,
	// so that psql prints its name with no schema. FamilySchema is the schema
	// of Family. See D201.
	Visible      sql.Null[bool]
	FamilySchema string
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
	// FunctionName is the function with no argument types, as psql's \dAp
	// prints it. Function is the form with them, which \dAp+ prints. See
	// D201.
	FunctionName string
	// Visible reports that the family is on the search path of the session.
	// FamilySchema is the schema of Family. See D201.
	Visible      sql.Null[bool]
	FamilySchema string
}

// Extension is an installed extension. psql lists them with \dx.
type Extension struct {
	Name    string
	Version string
	Schema  string
	Comment sql.Null[string]
	// DefaultVersion is the version the server installs by default, from the
	// control file of the extension. It is absent when the files of the
	// extension are not on the server. psql's \dx prints it. See D201.
	DefaultVersion sql.Null[string]
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
	Name    sql.Null[string]
	Owner   sql.Null[string]
	Table   string
	Kinds   string
	Comment sql.Null[string]
	// Definition is the columns and expressions the statistics cover and
	// their table, as psql's \dX prints it. See D147.
	Definition sql.Null[string]
	// Ndistinct, Dependencies and MCV say which of the three kinds the object
	// was made with, as the columns of psql's \dX do. Kinds holds the same
	// letters, except on SAP HANA, where it holds the type of the statistic in
	// lower case. MCV arrived in PostgreSQL 12.
	Ndistinct    bool
	Dependencies bool
	MCV          bool
	// StatsTarget is the statistics target of the object, and absent for the
	// default. PostgreSQL has it from release 13. See D199.
	StatsTarget sql.Null[int64]
	// TableVisible reports that the table is on the search path of the
	// session, so that psql prints its name with no schema. See D201.
	TableVisible sql.Null[bool]
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
	// SubscriptionConnections lists the connection string of each
	// subscription. Only a superuser can read it. See D201.
	SubscriptionConnections = NewQuery[SubscriptionConnection]("subscription_connections")
	// TextSearchParsers lists the parsers that split text into tokens.
	TextSearchParsers = NewQuery[TextSearchParser]("text_search_parsers")
	// TextSearchParserFunctions lists the five functions of each text search
	// parser. See D201.
	TextSearchParserFunctions = NewQuery[TextSearchParserFunction]("text_search_parser_functions")
	// TextSearchDictionaries lists the dictionaries that normalize tokens.
	TextSearchDictionaries = NewQuery[TextSearchDictionary]("text_search_dictionaries")
	// TextSearchTemplates lists the code behind the dictionaries.
	TextSearchTemplates = NewQuery[TextSearchTemplate]("text_search_templates")
	// TextSearchConfigs lists the configurations.
	TextSearchConfigs = NewQuery[TextSearchConfig]("text_search_configs")
	// TextSearchConfigMaps lists the dictionaries each configuration consults
	// for each kind of token.
	TextSearchConfigMaps = NewQuery[TextSearchConfigMap]("text_search_config_maps")
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
	Name       sql.Null[string]
	Ordinal    int64
	Expression sql.Null[string]
	Descending sql.Null[bool]
	// Include reports that the column is an INCLUDE column and not a key
	// column of the index. It is false below PostgreSQL 11, which has no such
	// column, and that is a real false. Every other model leaves it false.
	// See D199.
	Include bool
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
//
// Start, Minimum, Maximum and Increment are decimal text, because an Oracle
// sequence is bounded by 28 digits and an int64 holds 19. The default
// maximum of an Oracle sequence is 9999999999999999999999999999.
type Sequence struct {
	Schema    string
	Name      string
	DataType  sql.Null[string]
	Start     sql.Null[string]
	Minimum   sql.Null[string]
	Maximum   sql.Null[string]
	Increment sql.Null[string]
	Cycles    sql.Null[bool]
	OwnedBy   string
	Comment   sql.Null[string]
	// CacheSize is the number of values a session takes at once. It is absent
	// below release 10, as the bounds are. See D201.
	CacheSize sql.Null[int64]
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
	// Table is the table a partitioned index is on, and absent for a table.
	// AccessMethod is the access method of the index, and absent for a table
	// on a release below 12. See D199.
	Table        sql.Null[string]
	AccessMethod sql.Null[string]
	// DirectSize is the bytes that the partitions one level below take, and
	// TotalSize the bytes that every partition below takes, both as
	// pg_table_size counts them. They are absent below release 12, which has
	// no pg_partition_tree. See D199.
	DirectSize sql.Null[int64]
	TotalSize  sql.Null[int64]
	// ParentVisible and TableVisible report that the parent, and the table of
	// a partitioned index, are on the search path of the session, so that
	// psql prints their names with no schema. See D201.
	ParentVisible sql.Null[bool]
	TableVisible  sql.Null[bool]
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
	// Partitions lists the partitions of each partitioned table.
	Partitions = NewQuery[Partition]("partitions")
	// Inherits lists the inheritance of one table from another.
	Inherits = NewQuery[Inherit]("inherits")
	// Policies lists the row level security policies of a table.
	Policies = NewQuery[Policy]("policies")
	// Rules lists the rewrite rules of a table.
	Rules = NewQuery[Rule]("rules")
	// NotNulls lists the NOT NULL constraints of a table, as PostgreSQL
	// 18 records them.
	NotNulls = NewQuery[NotNull]("not_nulls")
)

// Partition is one partition of a partitioned table. psql prints it in the
// Partitions list of \d+ name, and in Partition of for the partition itself.
//
// It is one row for each partition, so a partitioned table of many partitions
// is many rows. A caller that wants the partitions of one table passes the
// table as Parent. A partition that is itself partitioned is a row of its own
// where it is the parent. See D199.
type Partition struct {
	// Schema and Table name the parent. PartitionSchema and Partition name the
	// partition.
	Schema          string
	Table           string
	PartitionSchema string
	Partition       string
	// Type is the relation type of the partition, as Table.Type spells it. A
	// partitioned table and a foreign table are partitions too.
	Type string
	// Bound is what follows FOR VALUES, or DEFAULT, as pg_get_expr writes it.
	Bound sql.Null[string]
	// Constraint is the implicit constraint the partition bound makes, as
	// pg_get_partition_constraintdef writes it.
	Constraint sql.Null[string]
	// Partitioned reports that the partition has partitions of its own.
	Partitioned bool
	// DetachPending reports a DETACH CONCURRENTLY that has not finished.
	// Releases below 14 have none, and that is a real false.
	DetachPending bool
	// TableVisible and PartitionVisible report that the parent and the
	// partition are on the search path of the session, so that psql prints
	// their names with no schema. See D201.
	TableVisible     sql.Null[bool]
	PartitionVisible sql.Null[bool]
}

// Inherit is one parent of a table by inheritance. psql prints the parents in
// Inherits and the children in Child tables of \d+ name.
//
// A partition is in PostgreSQL's catalog as a child too. Partition says so, so
// that a caller can leave it out the way psql does.
type Inherit struct {
	// Schema and Name name the child table. Type is its relation type.
	Schema string
	Name   string
	Type   string
	// ParentSchema and Parent name the table it inherits from.
	ParentSchema string
	Parent       string
	// Ordinal is the position of the parent among the parents of the child.
	Ordinal int64
	// Partition reports that the child is a partition of the parent. Releases
	// below 10 have no partitions, and that is a real false.
	Partition bool
	// Visible and ParentVisible report that the child and the parent are on
	// the search path of the session, so that psql prints their names with no
	// schema. See D201.
	Visible       sql.Null[bool]
	ParentVisible sql.Null[bool]
}

// Policy is a row level security policy. psql prints them in Policies of \d+
// name.
type Policy struct {
	Schema string
	Table  string
	Name   string
	// Command is all, select, insert, update or delete.
	Command string
	// Permissive is false for a restrictive policy. A release below 10 has
	// only permissive policies, and that is a real true.
	Permissive bool
	// Roles are the roles the policy applies to, joined by a comma, and absent
	// for public.
	Roles sql.Null[string]
	// Using and WithCheck are the expressions, as pg_get_expr writes them, and
	// absent when the policy has none.
	Using     sql.Null[string]
	WithCheck sql.Null[string]
	Comment   sql.Null[string]
}

// Rule is a rewrite rule of a table. psql prints them in Rules of \d+ name.
//
// The rule that makes a view is not here. It is named _RETURN and [View] holds
// its definition.
type Rule struct {
	Schema string
	Table  string
	Name   string
	// Event is select, update, insert or delete.
	Event string
	// Enabled is enabled, disabled, replica or always.
	Enabled string
	// Instead reports INSTEAD and not ALSO.
	Instead bool
	// Definition is the CREATE RULE statement, as pg_get_ruledef writes it,
	// without the final semicolon.
	Definition string
	Comment    sql.Null[string]
}

// NotNull is a NOT NULL constraint that has a name, which PostgreSQL 18
// records in pg_constraint. psql 18 prints them in Not-null constraints of
// \d+ name. [Constraints] does not hold them, and [Column.Nullable] still
// answers on every release. See D49 and D199.
type NotNull struct {
	Schema string
	Table  string
	Name   string
	Column string
	// NoInherit reports NO INHERIT.
	NoInherit bool
	// Local reports that the constraint was written on this table, and not
	// only inherited.
	Local bool
	// Inherited reports that a parent table also holds it.
	Inherited bool
	// Validated is false for a constraint that was added NOT VALID.
	Validated bool
}

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
// MySQL has no enum type, only an enum column, and the mysql model does not
// answer this kind.
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
	Definition sql.Null[string]
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
	// It is session dependent. It answers one row and, with [CurrentUser], it
	// describes the connection rather than the database. A caller reads it
	// with [First].
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
