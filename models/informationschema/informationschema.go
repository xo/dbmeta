// Package informationschema builds metadata queries from the SQL standard's
// information_schema views.
//
// It is not a model of one database. It is the thing a model is built from. A
// database that has information_schema registers its own model by describing
// how it differs from the standard and asking this package for the queries:
//
//	func init() {
//		informationschema.Register(dbmeta.MySQL, informationschema.Profile{
//			Placeholder: func(int) string { return "?" },
//			Has:         informationschema.Standard().Without(Sequences, CheckConstraints),
//			Clauses: map[informationschema.Clause]string{
//				informationschema.ColumnDataType: "column_type",
//			},
//			SystemSchemas: []string{"mysql", "information_schema", "performance_schema", "sys"},
//		})
//	}
//
// D9 makes this the secondary model. Prefer a native model where one exists,
// because information_schema answers 12 of the 55 object kinds that psql
// describes and answers none of them completely. It has no size, owner or
// access method for a table, no storage or index detail for a column, no
// exclusion constraint, and no aggregate or window function. It is what a
// database with nothing better can still answer.
//
// Which databases this serves, taken from what `usql` builds on the same
// reader today: DuckDB, Microsoft SQL Server, MySQL and MariaDB, Snowflake,
// Trino, Databend, Netezza, and PostgreSQL for the parts its native model does
// not cover. Oracle, SQLite3 and Cassandra have no information_schema at all
// and need a native model.
package informationschema

import (
	"database/sql"
	"maps"
	"strings"

	"github.com/xo/dbmeta"
)

// Feature is something a database may or may not have in its
// information_schema. A database that lacks one does not answer for it, and
// D34 reports that rather than returning an empty result.
type Feature string

// Feature values. These are the eight that `usql` found it needed across seven
// databases, which is evidence that the list is close to complete.
const (
	// Functions is the routines and parameters views.
	Functions Feature = "functions"
	// Sequences is the sequences view.
	Sequences Feature = "sequences"
	// Indexes is an index view, which the standard does not define at all and
	// which a database that has one spells its own way.
	Indexes Feature = "indexes"
	// Constraints is the table_constraints and key_column_usage views.
	Constraints Feature = "constraints"
	// CheckConstraints is the check_constraints view.
	CheckConstraints Feature = "check_constraints"
	// TablePrivileges is the table_privileges view.
	TablePrivileges Feature = "table_privileges"
	// ColumnPrivileges is the column_privileges view.
	ColumnPrivileges Feature = "column_privileges"
	// UsagePrivileges is the usage_privileges view.
	UsagePrivileges Feature = "usage_privileges"
	// Views is the views view, which holds the statement a view selects.
	Views Feature = "views"
	// Parameters is the parameters view, which holds the parameters of a
	// routine. The standard defines it beside routines, and a database that
	// has one usually has the other.
	Parameters Feature = "parameters"
)

// Features is the set a database has.
type Features map[Feature]bool

// Standard returns the features the SQL standard defines, which is every one
// except Indexes. A database starts from this and removes what it lacks.
func Standard() Features {
	return Features{
		Functions:        true,
		Sequences:        true,
		Constraints:      true,
		CheckConstraints: true,
		TablePrivileges:  true,
		ColumnPrivileges: true,
		UsagePrivileges:  true,
		Views:            true,
		Parameters:       true,
		Indexes:          false,
	}
}

// With returns a copy of f with each named feature present.
func (f Features) With(names ...Feature) Features {
	out := f.clone()
	for _, name := range names {
		out[name] = true
	}
	return out
}

// Without returns a copy of f with each named feature absent.
func (f Features) Without(names ...Feature) Features {
	out := f.clone()
	for _, name := range names {
		out[name] = false
	}
	return out
}

// Has reports whether the database has the feature.
func (f Features) Has(name Feature) bool { return f[name] }

func (f Features) clone() Features {
	out := make(Features, len(f))
	maps.Copy(out, f)
	return out
}

// Clause names a piece of SQL that databases spell differently. The standard
// spelling is the default, and a profile replaces the ones its database gets
// wrong.
type Clause string

// Clause values. Each default is the spelling the standard uses.
const (
	// ColumnDataType is the column type. MySQL reports the declared type in
	// column_type and only the base type in data_type, so it overrides this.
	ColumnDataType Clause = "columns.data_type"
	// ColumnSize is the length or precision of a column.
	ColumnSize Clause = "columns.size"
	// ColumnScale is the scale of a numeric column.
	ColumnScale Clause = "columns.scale"
	// ColumnRadix is the radix a numeric precision is counted in.
	ColumnRadix Clause = "columns.radix"
	// ConstraintDeferrable is whether a constraint can be deferred. MySQL has
	// no deferred constraints and reports an empty string.
	ConstraintDeferrable Clause = "constraints.deferrable"
	// ConstraintDeferred is whether a constraint is deferred by default.
	ConstraintDeferred Clause = "constraints.deferred"
	// PrivilegeGrantor is the role that granted a privilege.
	PrivilegeGrantor Clause = "privileges.grantor"
	// CurrentSchema is the expression giving the schema in use. MySQL has no
	// schema separate from the database, so it uses DATABASE().
	CurrentSchema Clause = "current_schema"
	// CurrentUser is the expression giving the effective user. The standard
	// spells it CURRENT_USER and every database here accepts that.
	CurrentUser Clause = "current_user"
	// SessionUser is the expression giving the user the connection
	// authenticated as. A database that does not separate the two overrides
	// it with NULL.
	SessionUser Clause = "session_user"
	// ColumnPrimaryKey is whether a column is part of the primary key. The
	// standard has no such column, so the default reaches key_column_usage,
	// which is one more subquery. MySQL has column_key and overrides it with
	// a free expression, which is the case D47 describes.
	ColumnPrimaryKey Clause = "columns.primary_key"
)

// standardClauses is what a database gets when it overrides nothing.
func standardClauses() map[Clause]string {
	return map[Clause]string{
		ColumnDataType:       "c.data_type",
		ColumnSize:           "COALESCE(c.character_maximum_length, c.numeric_precision, c.datetime_precision, 0)",
		ColumnScale:          "COALESCE(c.numeric_scale, 0)",
		ColumnRadix:          "COALESCE(c.numeric_precision_radix, 10)",
		ConstraintDeferrable: "t.is_deferrable",
		ConstraintDeferred:   "t.initially_deferred",
		PrivilegeGrantor:     "p.grantor",
		CurrentSchema:        "CURRENT_SCHEMA",
		CurrentUser:          "CURRENT_USER",
		SessionUser:          "SESSION_USER",
		ColumnPrimaryKey: `EXISTS (SELECT 1 FROM information_schema.key_column_usage k` +
			` JOIN information_schema.table_constraints tc` +
			` ON tc.constraint_catalog = k.constraint_catalog` +
			` AND tc.constraint_schema = k.constraint_schema` +
			` AND tc.constraint_name = k.constraint_name` +
			` WHERE tc.constraint_type = 'PRIMARY KEY'` +
			` AND k.table_schema = c.table_schema AND k.table_name = c.table_name` +
			` AND k.column_name = c.column_name)`,
	}
}

// Profile describes how one database differs from the standard.
//
// A model fills one in and passes it to [Register]. Everything it does not set
// takes the standard value, so a database close to the standard writes very
// little.
type Profile struct {
	// Placeholder writes the bind parameter for position n, counting from 1.
	Placeholder func(n int) string
	// Has is the features the database provides. Leave it nil for the standard
	// set, and build from [Standard] otherwise.
	Has Features
	// Clauses replaces the SQL for the pieces the database spells its own way.
	// A clause left out keeps the standard spelling.
	Clauses map[Clause]string
	// SystemSchemas are the schemas hidden unless a caller asks for them.
	SystemSchemas []string
	// VersionQuery reads the server version, and VersionColumns says how many
	// columns it returns. Leave both zero when the database reports none.
	VersionQuery   string
	VersionColumns int
	// ParseVersion turns those columns into a version set.
	ParseVersion func(cols []string) (dbmeta.VersionSet, error)
}

// clause returns the SQL for a piece, the profile's spelling where it has one.
func (p Profile) clause(name Clause) string {
	if s, ok := p.Clauses[name]; ok {
		return s
	}
	return standardClauses()[name]
}

// has reports whether the database has a feature, defaulting to the standard.
func (p Profile) has(name Feature) bool {
	if p.Has == nil {
		return Standard().Has(name)
	}
	return p.Has.Has(name)
}

// systemSchemas returns the SQL list of schemas to hide.
func (p Profile) systemSchemas() string {
	names := p.SystemSchemas
	if len(names) == 0 {
		names = []string{"information_schema"}
	}
	quoted := make([]string, len(names))
	for i, name := range names {
		quoted[i] = "'" + strings.ReplaceAll(name, "'", "''") + "'"
	}
	return strings.Join(quoted, ", ")
}

// Register builds the queries for a dialect from its profile and registers
// them, along with what the dialect declares about itself.
//
// A model calls this from its init. It registers only the queries the profile
// says the database has, so [dbmeta.Query.Support] reports the rest as not
// supported rather than returning an empty result.
func Register(d dbmeta.Dialect, p Profile) {
	dbmeta.RegisterDialect(d, &dbmeta.Info{
		Placeholder:    p.Placeholder,
		VersionQuery:   p.VersionQuery,
		VersionColumns: p.VersionColumns,
		ParseVersion:   p.ParseVersion,
	})
	dbmeta.Schemas.Register(d, schemas(p))
	dbmeta.Tables.Register(d, tables(p))
	dbmeta.Columns.Register(d, columns(p))
	if p.has(Constraints) {
		dbmeta.Constraints.Register(d, constraints(p))
	}
	if p.has(Sequences) {
		dbmeta.Sequences.Register(d, sequences(p))
	}
	if p.has(Functions) {
		dbmeta.Functions.Register(d, functions(p))
	}
	if p.has(TablePrivileges) {
		dbmeta.Privileges.Register(d, privileges(p))
	}
	// The kinds added under D47. The standard defines every one of these
	// views, so a database close to the standard answers them all.
	if p.has(Constraints) {
		dbmeta.ConstraintColumns.Register(d, constraintColumns(p))
	}
	if p.has(Parameters) {
		dbmeta.RoutineParameters.Register(d, routineParameters(p))
	}
	if p.has(Views) {
		dbmeta.Views.Register(d, views(p))
	}
	dbmeta.CurrentSchema.Register(d, currentSchema(p))
	dbmeta.CurrentUser.Register(d, currentUser(p))
}

// schemas reads information_schema.schemata.
func schemas(p Profile) *dbmeta.Binding[dbmeta.Schema] {
	return &dbmeta.Binding[dbmeta.Schema]{
		Stmt: dbmeta.Stmt{
			{{Query: `SELECT s.catalog_name AS "catalog"`}},
			{{Query: `, s.schema_name AS "name"`}},
			{{Query: `, s.schema_owner AS "owner"`}},
			// the standard has no comment on a schema
			{{Query: `, NULL AS "comment"`}},
			{{Query: `FROM information_schema.schemata s`}},
			{{Query: `WHERE (@with_system OR s.schema_name NOT IN (` + p.systemSchemas() + `))`}},
			{{Query: `AND (@name = '' OR s.schema_name LIKE @name)`}},
			{{Query: `ORDER BY 1, 2`}},
		},
		Fields: dbmeta.Fields("catalog", "name", "owner", "comment"),
		Params: nameSystem("schema"),
		Scan: func(rows *sql.Rows) (dbmeta.Schema, error) {
			var v dbmeta.Schema
			err := rows.Scan(&v.Catalog, &v.Name, &v.Owner, &v.Comment)
			return v, err
		},
	}
}

// tables reads information_schema.tables.
//
// The standard has no size, no owner and no access method, which is most of
// what psql prints for \dt. It also has no materialized view, so a database
// that has them reports them as something else here.
func tables(p Profile) *dbmeta.Binding[dbmeta.Table] {
	return &dbmeta.Binding[dbmeta.Table]{
		Stmt: dbmeta.Stmt{
			{{Query: `SELECT t.table_catalog AS "catalog"`}},
			{{Query: `, t.table_schema AS "schema"`}},
			{{Query: `, t.table_name AS "name"`}},
			{{Query: `, LOWER(t.table_type) AS "type"`}},
			{{Query: `, NULL AS "comment"`}},
			{{Query: `FROM information_schema.tables t`}},
			{{Query: `WHERE (@with_system OR t.table_schema NOT IN (` + p.systemSchemas() + `))`}},
			{{Query: `AND (@schema = '' OR t.table_schema LIKE @schema)`}},
			{{Query: `AND (@name = '' OR t.table_name LIKE @name)`}},
			{{Query: `ORDER BY 2, 3`}},
		},
		Fields: dbmeta.Fields("catalog", "schema", "name", "type", "comment"),
		Params: schemaNameSystem("table"),
		Scan: func(rows *sql.Rows) (dbmeta.Table, error) {
			var v dbmeta.Table
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Name, &v.Type, &v.Comment)
			return v, err
		},
	}
}

// columns reads information_schema.columns, which is the view the standard
// describes best and the one a caller most often wants.
func columns(p Profile) *dbmeta.Binding[dbmeta.Column] {
	return &dbmeta.Binding[dbmeta.Column]{
		Stmt: dbmeta.Stmt{
			{{Query: `SELECT c.table_catalog AS "catalog"`}},
			{{Query: `, c.table_schema AS "schema"`}},
			{{Query: `, c.table_name AS "table"`}},
			{{Query: `, c.column_name AS "name"`}},
			{{Query: `, c.ordinal_position AS "ordinal"`}},
			{{Query: `, ` + p.clause(ColumnDataType) + ` AS "data_type"`}},
			{{Query: `, CASE WHEN c.is_nullable = 'YES' THEN true ELSE false END AS "nullable"`}},
			{{Query: `, c.column_default AS "default"`}},
			{{Query: `, ` + p.clause(ColumnPrimaryKey) + ` AS "primary_key"`}},
			// the standard has no identity kind before Query:2003 and no
			// generated kind that every database reports the same way
			{{Query: `, NULL AS "identity"`}},
			{{Query: `, NULL AS "generated"`}},
			{{Query: `, NULL AS "comment"`}},
			{{Query: `FROM information_schema.columns c`}},
			{{Query: `WHERE (@with_system OR c.table_schema NOT IN (` + p.systemSchemas() + `))`}},
			{{Query: `AND (@schema = '' OR c.table_schema LIKE @schema)`}},
			{{Query: `AND (@parent = '' OR c.table_name LIKE @parent)`}},
			{{Query: `AND (@name = '' OR c.column_name LIKE @name)`}},
			{{Query: `ORDER BY 2, 3, 5`}},
		},
		Fields: dbmeta.Fields("catalog", "schema", "table", "name", "ordinal",
			"data_type", "nullable", "default", "primary_key", "identity", "generated", "comment"),
		Params: schemaParentName("column"),
		Scan: func(rows *sql.Rows) (dbmeta.Column, error) {
			var v dbmeta.Column
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Table, &v.Name, &v.Ordinal,
				&v.DataType, &v.Nullable, &v.Default, &v.PrimaryKey, &v.Identity,
				&v.Generated, &v.Comment)
			return v, err
		},
	}
}

// constraints reads information_schema.table_constraints, joined to
// check_constraints for the expression where the database has that view.
func constraints(p Profile) *dbmeta.Binding[dbmeta.Constraint] {
	definition := `NULL`
	join := ``
	if p.has(CheckConstraints) {
		definition = `k.check_clause`
		join = `LEFT JOIN information_schema.check_constraints k` +
			` ON k.constraint_catalog = t.constraint_catalog` +
			` AND k.constraint_schema = t.constraint_schema` +
			` AND k.constraint_name = t.constraint_name`
	}
	return &dbmeta.Binding[dbmeta.Constraint]{
		Stmt: dbmeta.Stmt{
			{{Query: `SELECT t.table_schema AS "schema"`}},
			{{Query: `, t.table_name AS "table"`}},
			{{Query: `, t.constraint_name AS "name"`}},
			{{Query: `, LOWER(t.constraint_type) AS "type"`}},
			{{Query: `, ` + definition + ` AS "definition"`}},
			{{Query: `, CASE WHEN ` + p.clause(ConstraintDeferrable) + ` = 'YES' THEN true ELSE false END AS "deferrable"`}},
			{{Query: `, CASE WHEN ` + p.clause(ConstraintDeferred) + ` = 'YES' THEN true ELSE false END AS "deferred"`}},
			{{Query: `, NULL AS "comment"`}},
			{{Query: `FROM information_schema.table_constraints t`}},
			{{Query: join}},
			{{Query: `WHERE (@with_system OR t.table_schema NOT IN (` + p.systemSchemas() + `))`}},
			{{Query: `AND (@schema = '' OR t.table_schema LIKE @schema)`}},
			{{Query: `AND (@parent = '' OR t.table_name LIKE @parent)`}},
			{{Query: `AND (@name = '' OR t.constraint_name LIKE @name)`}},
			{{Query: `ORDER BY 1, 2, 3`}},
		},
		Fields: dbmeta.Fields("schema", "table", "name", "type", "definition",
			"deferrable", "deferred", "comment"),
		Params: schemaParentName("constraint"),
		Scan: func(rows *sql.Rows) (dbmeta.Constraint, error) {
			var v dbmeta.Constraint
			err := rows.Scan(&v.Schema, &v.Table, &v.Name, &v.Type, &v.Definition,
				&v.Deferrable, &v.Deferred, &v.Comment)
			return v, err
		},
	}
}

// sequences reads information_schema.sequences.
func sequences(p Profile) *dbmeta.Binding[dbmeta.Sequence] {
	return &dbmeta.Binding[dbmeta.Sequence]{
		Stmt: dbmeta.Stmt{
			{{Query: `SELECT s.sequence_schema AS "schema"`}},
			{{Query: `, s.sequence_name AS "name"`}},
			{{Query: `, s.data_type AS "data_type"`}},
			{{Query: `, CAST(s.start_value AS BIGINT) AS "start"`}},
			{{Query: `, CAST(s.minimum_value AS BIGINT) AS "minimum"`}},
			{{Query: `, CAST(s.maximum_value AS BIGINT) AS "maximum"`}},
			{{Query: `, CAST(s.increment AS BIGINT) AS "increment"`}},
			{{Query: `, CASE WHEN s.cycle_option = 'YES' THEN true ELSE false END AS "cycles"`}},
			// the standard does not record what owns a sequence
			{{Query: `, '' AS "owned_by"`}},
			{{Query: `, NULL AS "comment"`}},
			{{Query: `FROM information_schema.sequences s`}},
			{{Query: `WHERE (@with_system OR s.sequence_schema NOT IN (` + p.systemSchemas() + `))`}},
			{{Query: `AND (@schema = '' OR s.sequence_schema LIKE @schema)`}},
			{{Query: `AND (@name = '' OR s.sequence_name LIKE @name)`}},
			{{Query: `ORDER BY 1, 2`}},
		},
		Fields: dbmeta.Fields("schema", "name", "data_type", "start", "minimum",
			"maximum", "increment", "cycles", "owned_by", "comment"),
		Params: schemaNameSystem("sequence"),
		Scan: func(rows *sql.Rows) (dbmeta.Sequence, error) {
			var v dbmeta.Sequence
			err := rows.Scan(&v.Schema, &v.Name, &v.DataType, &v.Start, &v.Minimum,
				&v.Maximum, &v.Increment, &v.Cycles, &v.OwnedBy, &v.Comment)
			return v, err
		},
	}
}

// functions reads information_schema.routines.
//
// The standard has no aggregate and no window function, so everything here is
// a function or a procedure.
func functions(p Profile) *dbmeta.Binding[dbmeta.Function] {
	return &dbmeta.Binding[dbmeta.Function]{
		Stmt: dbmeta.Stmt{
			{{Query: `SELECT r.specific_catalog AS "catalog"`}},
			{{Query: `, r.routine_schema AS "schema"`}},
			{{Query: `, r.routine_name AS "name"`}},
			// the specific name, which the standard defines precisely so that
			// an overloaded routine can be identified
			{{Query: `, r.specific_name AS "id"`}},
			{{Query: `, LOWER(r.routine_type) AS "kind"`}},
			{{Query: `, r.data_type AS "result_type"`}},
			// the standard keeps parameters in their own view, so a caller
			// that wants them asks for the parameters of one routine
			{{Query: `, '' AS "arg_types"`}},
			{{Query: `, '' AS "volatility"`}},
			{{Query: `, '' AS "parallel"`}},
			{{Query: `, '' AS "owner"`}},
			{{Query: `, LOWER(r.security_type) AS "security"`}},
			{{Query: `, NULL AS "access"`}},
			{{Query: `, r.external_language AS "language"`}},
			{{Query: `, r.routine_definition AS "source"`}},
			{{Query: `, NULL AS "comment"`}},
			{{Query: `FROM information_schema.routines r`}},
			{{Query: `WHERE (@with_system OR r.routine_schema NOT IN (` + p.systemSchemas() + `))`}},
			{{Query: `AND (@schema = '' OR r.routine_schema LIKE @schema)`}},
			{{Query: `AND (@name = '' OR r.routine_name LIKE @name)`}},
			{{Query: `ORDER BY 2, 3`}},
		},
		Fields: dbmeta.Fields("catalog", "schema", "name", "id", "kind", "result_type",
			"arg_types", "volatility", "parallel", "owner", "security", "access",
			"language", "source", "comment"),
		Params: schemaNameSystem("routine"),
		Scan: func(rows *sql.Rows) (dbmeta.Function, error) {
			var v dbmeta.Function
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Name, &v.ID, &v.Kind, &v.ResultType,
				&v.ArgTypes, &v.Volatility, &v.Parallel, &v.Owner, &v.Security,
				&v.Access, &v.Language, &v.Source, &v.Comment)
			return v, err
		},
	}
}

// privileges reads information_schema.table_privileges.
//
// The standard returns one row per grant, where psql returns one row per
// object with the grants gathered into a column. This keeps the standard's
// shape and lets the caller gather, because gathering in SQL needs a string
// aggregate that the standard does not define.
func privileges(p Profile) *dbmeta.Binding[dbmeta.Privilege] {
	return &dbmeta.Binding[dbmeta.Privilege]{
		Stmt: dbmeta.Stmt{
			{{Query: `SELECT p.table_schema AS "schema"`}},
			{{Query: `, p.table_name AS "name"`}},
			{{Query: `, 'table' AS "type"`}},
			{{Query: `, p.grantee || '=' || p.privilege_type || '/' || ` + p.clause(PrivilegeGrantor) + ` AS "access"`}},
			{{Query: `, NULL AS "column_access"`}},
			{{Query: `, NULL AS "policies"`}},
			{{Query: `FROM information_schema.table_privileges p`}},
			{{Query: `WHERE (@with_system OR p.table_schema NOT IN (` + p.systemSchemas() + `))`}},
			{{Query: `AND (@schema = '' OR p.table_schema LIKE @schema)`}},
			{{Query: `AND (@name = '' OR p.table_name LIKE @name)`}},
			{{Query: `ORDER BY 1, 2`}},
		},
		Fields: dbmeta.Fields("schema", "name", "type", "access", "column_access", "policies"),
		Params: schemaNameSystem("table"),
		Scan: func(rows *sql.Rows) (dbmeta.Privilege, error) {
			var v dbmeta.Privilege
			err := rows.Scan(&v.Schema, &v.Name, &v.Type, &v.Access, &v.ColumnAccess, &v.Policies)
			return v, err
		},
	}
}

func nameSystem(kind string) []dbmeta.Param {
	return []dbmeta.Param{
		{Name: "name", Desc: kind + " name pattern, empty for every " + kind, Default: ""},
		{Name: "with_system", Desc: "include the system schemas", Default: false},
	}
}

func schemaNameSystem(kind string) []dbmeta.Param {
	return append([]dbmeta.Param{
		{Name: "schema", Desc: "schema name pattern, empty for every schema", Default: ""},
	}, nameSystem(kind)...)
}

func schemaParentName(kind string) []dbmeta.Param {
	return append([]dbmeta.Param{
		{Name: "schema", Desc: "schema name pattern, empty for every schema", Default: ""},
		{Name: "parent", Desc: "table name pattern, empty for every table", Default: ""},
	}, nameSystem(kind)...)
}

// The kinds added under D47. Every view below is in the SQL standard, which is
// why the shared model can answer them at all: psql renders these as text and
// the standard keeps them in tables.

// constraintColumns reads information_schema.key_column_usage, which carries
// both the column of a constraint and the column it points at.
func constraintColumns(p Profile) *dbmeta.Binding[dbmeta.ConstraintColumn] {
	return &dbmeta.Binding[dbmeta.ConstraintColumn]{
		Stmt: dbmeta.Stmt{
			{{Query: `SELECT k.constraint_catalog AS "catalog"`}},
			{{Query: `, k.table_schema AS "schema"`}},
			{{Query: `, k.table_name AS "table"`}},
			{{Query: `, k.constraint_name AS "constraint"`}},
			{{Query: `, k.column_name AS "name"`}},
			{{Query: `, k.ordinal_position AS "ordinal"`}},
			// The standard reaches the referenced side through
			// referential_constraints and the unique constraint it names,
			// which is two more joins. Everything that has key_column_usage
			// has referential_constraints, because the standard defines them
			// together.
			{{Query: `, r.unique_constraint_catalog AS "foreign_catalog"`}},
			{{Query: `, r.unique_constraint_schema AS "foreign_schema"`}},
			{{Query: `, fk.table_name AS "foreign_table"`}},
			{{Query: `, fk.column_name AS "foreign_name"`}},
			{{Query: `FROM information_schema.key_column_usage k`}},
			{{Query: `LEFT JOIN information_schema.referential_constraints r` +
				` ON r.constraint_catalog = k.constraint_catalog` +
				` AND r.constraint_schema = k.constraint_schema` +
				` AND r.constraint_name = k.constraint_name`}},
			{{Query: `LEFT JOIN information_schema.key_column_usage fk` +
				` ON fk.constraint_catalog = r.unique_constraint_catalog` +
				` AND fk.constraint_schema = r.unique_constraint_schema` +
				` AND fk.constraint_name = r.unique_constraint_name` +
				` AND fk.ordinal_position = k.position_in_unique_constraint`}},
			{{Query: `WHERE (@with_system OR k.table_schema NOT IN (` + p.systemSchemas() + `))`}},
			{{Query: `AND (@schema = '' OR k.table_schema LIKE @schema)`}},
			{{Query: `AND (@parent = '' OR k.table_name LIKE @parent)`}},
			{{Query: `AND (@name = '' OR k.constraint_name LIKE @name)`}},
			{{Query: `ORDER BY 2, 3, 4, 6`}},
		},
		Fields: dbmeta.Fields("catalog", "schema", "table", "constraint", "name",
			"ordinal", "foreign_catalog", "foreign_schema", "foreign_table", "foreign_name"),
		Params: schemaParentName("constraint"),
		Scan: func(rows *sql.Rows) (dbmeta.ConstraintColumn, error) {
			var v dbmeta.ConstraintColumn
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Table, &v.Constraint, &v.Name,
				&v.Ordinal, &v.ForeignCatalog, &v.ForeignSchema, &v.ForeignTable, &v.ForeignName)
			return v, err
		},
	}
}

// routineParameters reads information_schema.parameters.
func routineParameters(p Profile) *dbmeta.Binding[dbmeta.RoutineParameter] {
	return &dbmeta.Binding[dbmeta.RoutineParameter]{
		Stmt: dbmeta.Stmt{
			{{Query: `SELECT p.specific_catalog AS "catalog"`}},
			{{Query: `, p.specific_schema AS "schema"`}},
			{{Query: `, p.specific_name AS "routine"`}},
			{{Query: `, p.specific_name AS "routine_id"`}},
			{{Query: `, p.parameter_name AS "name"`}},
			{{Query: `, p.ordinal_position AS "ordinal"`}},
			{{Query: `, CASE WHEN p.ordinal_position = 0 THEN 'return'` +
				` ELSE LOWER(COALESCE(p.parameter_mode, 'IN')) END AS "mode"`}},
			{{Query: `, p.dtd_identifier AS "data_type"`}},
			{{Query: `, NULL AS "default"`}},
			{{Query: `FROM information_schema.parameters p`}},
			{{Query: `WHERE (@with_system OR p.specific_schema NOT IN (` + p.systemSchemas() + `))`}},
			{{Query: `AND (@schema = '' OR p.specific_schema LIKE @schema)`}},
			{{Query: `AND (@parent = '' OR p.specific_name LIKE @parent)`}},
			{{Query: `AND (@name = '' OR COALESCE(p.parameter_name, '') LIKE @name)`}},
			{{Query: `ORDER BY 2, 3, 6`}},
		},
		Fields: []dbmeta.Field{
			{Name: "catalog"}, {Name: "schema"}, {Name: "routine"},
			{
				Name: "routine_id",
				Desc: "the specific name, which the standard defines so that an overloaded routine can be told apart",
			},
			{Name: "name"}, {Name: "ordinal"}, {Name: "mode"}, {Name: "data_type"},
			{Name: "default", Desc: "always absent: the standard records no parameter default"},
		},
		Params: schemaParentName("parameter"),
		Scan: func(rows *sql.Rows) (dbmeta.RoutineParameter, error) {
			var v dbmeta.RoutineParameter
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Routine, &v.RoutineID, &v.Name,
				&v.Ordinal, &v.Mode, &v.DataType, &v.Default)
			return v, err
		},
	}
}

// views reads information_schema.views.
func views(p Profile) *dbmeta.Binding[dbmeta.View] {
	return &dbmeta.Binding[dbmeta.View]{
		Stmt: dbmeta.Stmt{
			{{Query: `SELECT v.table_catalog AS "catalog"`}},
			{{Query: `, v.table_schema AS "schema"`}},
			{{Query: `, v.table_name AS "name"`}},
			{{Query: `, v.view_definition AS "definition"`}},
			{{Query: `, LOWER(v.check_option) AS "check_option"`}},
			{{Query: `, v.is_updatable = 'YES' AS "updatable"`}},
			{{Query: `, NULL AS "insertable"`}},
			{{Query: `, NULL AS "comment"`}},
			{{Query: `FROM information_schema.views v`}},
			{{Query: `WHERE (@with_system OR v.table_schema NOT IN (` + p.systemSchemas() + `))`}},
			{{Query: `AND (@schema = '' OR v.table_schema LIKE @schema)`}},
			{{Query: `AND (@name = '' OR v.table_name LIKE @name)`}},
			{{Query: `ORDER BY 2, 3`}},
		},
		Fields: []dbmeta.Field{
			{Name: "catalog"}, {Name: "schema"}, {Name: "name"},
			{
				Name: "definition",
				Desc: "the statement the view selects. The standard allows a server to return an empty string where the caller may not read it",
			},
			{Name: "check_option"}, {Name: "updatable"},
			{Name: "insertable", Desc: "always absent: the standard has no such column"},
			{Name: "comment", Desc: "always absent: the standard has no comment on a view"},
		},
		Params: schemaNameSystem("view"),
		Scan: func(rows *sql.Rows) (dbmeta.View, error) {
			var v dbmeta.View
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Name, &v.Definition,
				&v.CheckOption, &v.Updatable, &v.Insertable, &v.Comment)
			return v, err
		},
	}
}

// currentUser reads who the connection is authenticated as.
//
// There is no information_schema view for this and there does not need to be:
// CURRENT_USER and SESSION_USER are standard SQL expressions, so this is the
// one binding here that reads no view at all. It raises the shared model by
// one kind for two lines of standard SQL. See D55.
func currentUser(p Profile) *dbmeta.Binding[dbmeta.User] {
	return &dbmeta.Binding[dbmeta.User]{
		Stmt: dbmeta.Stmt{
			{{Query: `SELECT ` + p.clause(CurrentUser) + ` AS "name"`}},
			{{Query: `, ` + p.clause(SessionUser) + ` AS "session"`}},
		},
		Fields: []dbmeta.Field{
			{Name: "name", Desc: "the effective user"},
			{Name: "session", Desc: "the user the connection authenticated as"},
		},
		Scan: func(rows *sql.Rows) (dbmeta.User, error) {
			var v dbmeta.User
			err := rows.Scan(&v.Name, &v.Session)
			return v, err
		},
	}
}

// currentSchema reads the one row of schemata that the session resolves in.
func currentSchema(p Profile) *dbmeta.Binding[dbmeta.Schema] {
	return &dbmeta.Binding[dbmeta.Schema]{
		Stmt: dbmeta.Stmt{
			{{Query: `SELECT s.catalog_name AS "catalog"`}},
			{{Query: `, s.schema_name AS "name"`}},
			{{Query: `, s.schema_owner AS "owner"`}},
			{{Query: `, NULL AS "comment"`}},
			{{Query: `FROM information_schema.schemata s`}},
			{{Query: `WHERE s.schema_name = ` + p.clause(CurrentSchema)}},
		},
		Fields: []dbmeta.Field{
			{Name: "catalog"},
			{Name: "name", Desc: "the schema an unqualified name resolves in"},
			{Name: "owner"},
			{Name: "comment", Desc: "always absent: the standard has no comment on a schema"},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Schema, error) {
			var v dbmeta.Schema
			err := rows.Scan(&v.Catalog, &v.Name, &v.Owner, &v.Comment)
			return v, err
		},
	}
}
