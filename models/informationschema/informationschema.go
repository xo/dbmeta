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
// because information_schema answers about 9 of the 48 object kinds that psql
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
	// VersionSQL reads the server version, and VersionColumns says how many
	// columns it returns. Leave both zero when the database reports none.
	VersionSQL     string
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
		VersionSQL:     p.VersionSQL,
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
}

// schemas reads information_schema.schemata.
func schemas(p Profile) *dbmeta.Binding[dbmeta.Schema] {
	return &dbmeta.Binding[dbmeta.Schema]{
		Stmt: dbmeta.Stmt{
			{{SQL: `SELECT s.catalog_name AS "catalog"`}},
			{{SQL: `, s.schema_name AS "name"`}},
			{{SQL: `, s.schema_owner AS "owner"`}},
			// the standard has no comment on a schema
			{{SQL: `, NULL AS "comment"`}},
			{{SQL: `FROM information_schema.schemata s`}},
			{{SQL: `WHERE (@with_system OR s.schema_name NOT IN (` + p.systemSchemas() + `))`}},
			{{SQL: `AND (@name = '' OR s.schema_name LIKE @name)`}},
			{{SQL: `ORDER BY 1, 2`}},
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
			{{SQL: `SELECT t.table_catalog AS "catalog"`}},
			{{SQL: `, t.table_schema AS "schema"`}},
			{{SQL: `, t.table_name AS "name"`}},
			{{SQL: `, LOWER(t.table_type) AS "type"`}},
			{{SQL: `, NULL AS "comment"`}},
			{{SQL: `FROM information_schema.tables t`}},
			{{SQL: `WHERE (@with_system OR t.table_schema NOT IN (` + p.systemSchemas() + `))`}},
			{{SQL: `AND (@schema = '' OR t.table_schema LIKE @schema)`}},
			{{SQL: `AND (@name = '' OR t.table_name LIKE @name)`}},
			{{SQL: `ORDER BY 2, 3`}},
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
			{{SQL: `SELECT c.table_catalog AS "catalog"`}},
			{{SQL: `, c.table_schema AS "schema"`}},
			{{SQL: `, c.table_name AS "table"`}},
			{{SQL: `, c.column_name AS "name"`}},
			{{SQL: `, c.ordinal_position AS "ordinal"`}},
			{{SQL: `, ` + p.clause(ColumnDataType) + ` AS "data_type"`}},
			{{SQL: `, CASE WHEN c.is_nullable = 'YES' THEN true ELSE false END AS "nullable"`}},
			{{SQL: `, c.column_default AS "default"`}},
			// the standard has no identity kind before SQL:2003 and no
			// generated kind that every database reports the same way
			{{SQL: `, NULL AS "identity"`}},
			{{SQL: `, NULL AS "generated"`}},
			{{SQL: `, NULL AS "comment"`}},
			{{SQL: `FROM information_schema.columns c`}},
			{{SQL: `WHERE (@with_system OR c.table_schema NOT IN (` + p.systemSchemas() + `))`}},
			{{SQL: `AND (@schema = '' OR c.table_schema LIKE @schema)`}},
			{{SQL: `AND (@parent = '' OR c.table_name LIKE @parent)`}},
			{{SQL: `AND (@name = '' OR c.column_name LIKE @name)`}},
			{{SQL: `ORDER BY 2, 3, 5`}},
		},
		Fields: dbmeta.Fields("catalog", "schema", "table", "name", "ordinal",
			"data_type", "nullable", "default", "identity", "generated", "comment"),
		Params: schemaParentName("column"),
		Scan: func(rows *sql.Rows) (dbmeta.Column, error) {
			var v dbmeta.Column
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Table, &v.Name, &v.Ordinal,
				&v.DataType, &v.Nullable, &v.Default, &v.Identity, &v.Generated, &v.Comment)
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
			{{SQL: `SELECT t.table_schema AS "schema"`}},
			{{SQL: `, t.table_name AS "table"`}},
			{{SQL: `, t.constraint_name AS "name"`}},
			{{SQL: `, LOWER(t.constraint_type) AS "type"`}},
			{{SQL: `, ` + definition + ` AS "definition"`}},
			{{SQL: `, CASE WHEN ` + p.clause(ConstraintDeferrable) + ` = 'YES' THEN true ELSE false END AS "deferrable"`}},
			{{SQL: `, CASE WHEN ` + p.clause(ConstraintDeferred) + ` = 'YES' THEN true ELSE false END AS "deferred"`}},
			{{SQL: `, NULL AS "comment"`}},
			{{SQL: `FROM information_schema.table_constraints t`}},
			{{SQL: join}},
			{{SQL: `WHERE (@with_system OR t.table_schema NOT IN (` + p.systemSchemas() + `))`}},
			{{SQL: `AND (@schema = '' OR t.table_schema LIKE @schema)`}},
			{{SQL: `AND (@parent = '' OR t.table_name LIKE @parent)`}},
			{{SQL: `AND (@name = '' OR t.constraint_name LIKE @name)`}},
			{{SQL: `ORDER BY 1, 2, 3`}},
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
			{{SQL: `SELECT s.sequence_schema AS "schema"`}},
			{{SQL: `, s.sequence_name AS "name"`}},
			{{SQL: `, s.data_type AS "data_type"`}},
			{{SQL: `, CAST(s.start_value AS BIGINT) AS "start"`}},
			{{SQL: `, CAST(s.minimum_value AS BIGINT) AS "minimum"`}},
			{{SQL: `, CAST(s.maximum_value AS BIGINT) AS "maximum"`}},
			{{SQL: `, CAST(s.increment AS BIGINT) AS "increment"`}},
			{{SQL: `, CASE WHEN s.cycle_option = 'YES' THEN true ELSE false END AS "cycles"`}},
			// the standard does not record what owns a sequence
			{{SQL: `, '' AS "owned_by"`}},
			{{SQL: `, NULL AS "comment"`}},
			{{SQL: `FROM information_schema.sequences s`}},
			{{SQL: `WHERE (@with_system OR s.sequence_schema NOT IN (` + p.systemSchemas() + `))`}},
			{{SQL: `AND (@schema = '' OR s.sequence_schema LIKE @schema)`}},
			{{SQL: `AND (@name = '' OR s.sequence_name LIKE @name)`}},
			{{SQL: `ORDER BY 1, 2`}},
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
			{{SQL: `SELECT r.specific_catalog AS "catalog"`}},
			{{SQL: `, r.routine_schema AS "schema"`}},
			{{SQL: `, r.routine_name AS "name"`}},
			{{SQL: `, LOWER(r.routine_type) AS "kind"`}},
			{{SQL: `, r.data_type AS "result_type"`}},
			// the standard keeps parameters in their own view, so a caller
			// that wants them asks for the parameters of one routine
			{{SQL: `, '' AS "arg_types"`}},
			{{SQL: `, '' AS "volatility"`}},
			{{SQL: `, '' AS "parallel"`}},
			{{SQL: `, '' AS "owner"`}},
			{{SQL: `, LOWER(r.security_type) AS "security"`}},
			{{SQL: `, NULL AS "access"`}},
			{{SQL: `, r.external_language AS "language"`}},
			{{SQL: `, r.routine_definition AS "source"`}},
			{{SQL: `, NULL AS "comment"`}},
			{{SQL: `FROM information_schema.routines r`}},
			{{SQL: `WHERE (@with_system OR r.routine_schema NOT IN (` + p.systemSchemas() + `))`}},
			{{SQL: `AND (@schema = '' OR r.routine_schema LIKE @schema)`}},
			{{SQL: `AND (@name = '' OR r.routine_name LIKE @name)`}},
			{{SQL: `ORDER BY 2, 3`}},
		},
		Fields: dbmeta.Fields("catalog", "schema", "name", "kind", "result_type",
			"arg_types", "volatility", "parallel", "owner", "security", "access",
			"language", "source", "comment"),
		Params: schemaNameSystem("routine"),
		Scan: func(rows *sql.Rows) (dbmeta.Function, error) {
			var v dbmeta.Function
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Name, &v.Kind, &v.ResultType,
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
			{{SQL: `SELECT p.table_schema AS "schema"`}},
			{{SQL: `, p.table_name AS "name"`}},
			{{SQL: `, 'table' AS "type"`}},
			{{SQL: `, p.grantee || '=' || p.privilege_type || '/' || ` + p.clause(PrivilegeGrantor) + ` AS "access"`}},
			{{SQL: `, NULL AS "column_access"`}},
			{{SQL: `, NULL AS "policies"`}},
			{{SQL: `FROM information_schema.table_privileges p`}},
			{{SQL: `WHERE (@with_system OR p.table_schema NOT IN (` + p.systemSchemas() + `))`}},
			{{SQL: `AND (@schema = '' OR p.table_schema LIKE @schema)`}},
			{{SQL: `AND (@name = '' OR p.table_name LIKE @name)`}},
			{{SQL: `ORDER BY 1, 2`}},
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
