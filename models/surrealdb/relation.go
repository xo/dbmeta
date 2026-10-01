package surrealdb

import (
	"database/sql"

	"github.com/xo/dbmeta"
)

// The three kinds of table, as Table.Type spells them. A view is a table
// defined AS SELECT, and a relation is a table defined TYPE RELATION, which
// holds the edges of a graph.
const (
	tableType    = "table"
	viewType     = "view"
	relationType = "relation"
)

// typeOf is the kind of the table in $x.
const typeOf = "(IF $x.view != NONE THEN '" + viewType + "'" +
	" ELSE IF $x.kind.kind = 'RELATION' THEN '" + relationType + "' ELSE '" + tableType + "' END)"

// typesFilter is the condition that the table type in col is one of the types
// in @types, which are joined by commas, and that everything matches an empty
// list. It is dbmeta.InList written for SurrealQL, which has no LIKE.
func typesFilter(col string) string {
	return "(@types = '' OR " + col + " IN string::split(@types, ','))"
}

// nullable is whether the field in $x accepts NONE or NULL. A field with no
// TYPE takes any value. An optional type is written option<T> on 2.x and
// none | T on 3.x, and a union can name none or null on either side.
const nullable = "($x.kind = NONE OR $x.kind IN ['any', 'none', 'null']" +
	" OR string::starts_with($x.kind, 'option<')" +
	" OR string::starts_with($x.kind, 'none | ') OR string::starts_with($x.kind, 'null | ')" +
	" OR string::ends_with($x.kind, ' | none') OR string::ends_with($x.kind, ' | null'))"

// definitionOf is the text of a view, which INFO writes as AS SELECT ....
const definitionOf = "(IF string::starts_with($x.view, 'AS ') THEN string::slice($x.view, 3) ELSE $x.view END)"

func registerRelations() {
	// \l. A namespace is the top of the tree, so it is the database. Only
	// the root user reads INFO FOR ROOT.
	dbmeta.Databases.Register(dbmeta.SurrealDB, &dbmeta.Binding[dbmeta.Database]{
		Stmt: from3("SELECT NULL AS access, '' AS collate, comment, '' AS ctype, '' AS encoding" +
			", name, '' AS owner, NULL AS size, NULL AS tablespace" +
			" FROM (INFO FOR ROOT STRUCTURE).namespaces" +
			" WHERE " + like("name", "@name") +
			" ORDER BY name"),
		Fields: []dbmeta.Field{
			{Name: "access", Desc: "always absent: a namespace carries no grant, and a user holds a role on it"},
			{Name: "collate", Desc: "always empty: a namespace has no collation"},
			{Name: "comment"},
			{Name: "ctype", Desc: "always empty, for the same reason"},
			{Name: "encoding", Desc: "always empty: a namespace has no encoding, and a string is UTF-8"},
			{Name: "name", Desc: "the namespace"},
			{Name: "owner", Desc: "always empty: a namespace has no owner"},
			{Name: "size", Desc: "always absent: INFO reports no size"},
			{Name: "tablespace", Desc: "always absent: a namespace has no tablespace"},
		},
		Params: []dbmeta.Param{{Name: "name", Desc: "namespace name pattern, empty for every namespace", Default: ""}},
		Scan: func(rows *sql.Rows) (dbmeta.Database, error) {
			var v dbmeta.Database
			err := rows.Scan(&v.Access, &v.Collate, &v.Comment, &v.CType, &v.Encoding,
				&v.Name, &v.Owner, &v.Size, &v.Tablespace)
			return v, err
		},
	})

	// \dn. A database is the schema, and its namespace is its catalog. Every
	// other kind reads only the database of the connection, so Schemas
	// lists that one alone, and a database whose tables are never returned
	// is not listed (D168). INFO FOR NS holds its comment, and a user
	// defined on a database is refused INFO FOR NS.
	dbmeta.Schemas.Register(dbmeta.SurrealDB, &dbmeta.Binding[dbmeta.Schema]{
		Stmt: from3("SELECT " + currentNS + " AS catalog, comment, name, '' AS owner" +
			" FROM (INFO FOR NS STRUCTURE).databases" +
			" WHERE name = " + currentDB + " AND " + like("name", "@name") +
			" ORDER BY name"),
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "the namespace the connection is in"},
			{Name: "comment"},
			{Name: "name", Desc: "the database the connection is in, which is the only one the other kinds read"},
			{Name: "owner", Desc: "always empty: a database has no owner"},
		},
		Params: []dbmeta.Param{{Name: "name", Desc: "database name pattern, empty for the database of the connection", Default: ""}},
		Scan: func(rows *sql.Rows) (dbmeta.Schema, error) {
			var v dbmeta.Schema
			err := rows.Scan(&v.Catalog, &v.Comment, &v.Name, &v.Owner)
			return v, err
		},
	})

	// The database the connection is in. It needs no INFO, so 2.7 answers
	// it too.
	dbmeta.CurrentSchema.Register(dbmeta.SurrealDB, &dbmeta.Binding[dbmeta.Schema]{
		Stmt: dbmeta.Always("SELECT " + currentNS + " AS catalog, NULL AS comment, " + currentDB + " AS name" +
			", '' AS owner FROM ONLY {}"),
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "the namespace the connection is in"},
			{
				Name: "comment",
				Desc: "always absent: Schemas returns the comment, from INFO FOR NS," +
					" which a user defined on a database is refused",
			},
			{Name: "name", Desc: "the database the connection is in"},
			{Name: "owner", Desc: "always empty: a database has no owner"},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Schema, error) {
			var v dbmeta.Schema
			err := rows.Scan(&v.Catalog, &v.Comment, &v.Name, &v.Owner)
			return v, err
		},
	})

	// \dt. Every table of the database, with its kind.
	dbmeta.Tables.Register(dbmeta.SurrealDB, &dbmeta.Binding[dbmeta.Table]{
		Stmt: from3("SELECT " + currentNS + " AS catalog, comment, name, " + currentDB + " AS schema, type" +
			" FROM array::map(" + tables + ", |$x| {name: $x.name, comment: $x.comment, type: " + typeOf + "})" +
			" WHERE " + like(currentDB, "@schema") +
			" AND " + like("name", "@name") +
			" AND " + typesFilter("type") +
			" ORDER BY name"),
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "the namespace"},
			{Name: "comment"},
			{Name: "name"},
			{Name: "schema", Desc: "the database"},
			{Name: "type", Desc: "table, view for a table defined AS SELECT, or relation for a table defined TYPE RELATION"},
		},
		Params: []dbmeta.Param{
			{Name: "schema", Desc: "database name pattern, empty for the database of the connection", Default: ""},
			{Name: "name", Desc: "table name pattern, empty for every table", Default: ""},
			dbmeta.TypesParam(),
		},
		Scan: func(rows *sql.Rows) (dbmeta.Table, error) {
			var v dbmeta.Table
			err := rows.Scan(&v.Catalog, &v.Comment, &v.Name, &v.Schema, &v.Type)
			return v, err
		},
	})

	// \dv. A table defined AS SELECT. The server keeps its records up to
	// date from the source tables, and refuses a write to it.
	dbmeta.Views.Register(dbmeta.SurrealDB, &dbmeta.Binding[dbmeta.View]{
		Stmt: from3("SELECT " + currentNS + " AS catalog, NULL AS check_option, comment, definition" +
			", false AS insertable, name, " + currentDB + " AS schema, false AS updatable" +
			" FROM array::map(array::filter(" + tables + ", |$x| $x.view != NONE)," +
			" |$x| {name: $x.name, comment: $x.comment, definition: " + definitionOf + "})" +
			" WHERE " + like(currentDB, "@schema") +
			" AND " + like("name", "@name") +
			" ORDER BY name"),
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "the namespace"},
			{Name: "check_option", Desc: "always absent: a view takes no write, so it has nothing to check"},
			{Name: "comment"},
			{Name: "definition", Desc: "the SELECT the view is defined as"},
			{Name: "insertable", Desc: "always false: the server refuses a write to a view"},
			{Name: "name"},
			{Name: "schema", Desc: "the database"},
			{Name: "updatable", Desc: "always false, for the same reason"},
		},
		Params: []dbmeta.Param{
			{Name: "schema", Desc: "database name pattern, empty for the database of the connection", Default: ""},
			{Name: "name", Desc: "view name pattern, empty for every view", Default: ""},
		},
		Scan: func(rows *sql.Rows) (dbmeta.View, error) {
			var v dbmeta.View
			err := rows.Scan(&v.Catalog, &v.CheckOption, &v.Comment, &v.Definition,
				&v.Insertable, &v.Name, &v.Schema, &v.Updatable)
			return v, err
		},
	})

	// \d. A column is a DEFINE FIELD. A field of a schemaless table that no
	// DEFINE FIELD names is known only from the records, and is not here
	// (D47). A field of the items of an array, such as tags.*, is a column
	// of its own, as DEFINE FIELD names it.
	dbmeta.Columns.Register(dbmeta.SurrealDB, &dbmeta.Binding[dbmeta.Column]{
		Stmt: from3("SELECT " + currentNS + " AS catalog, NULL AS collation, comment, data_type, default" +
			", generated, '' AS identity, name, nullable, ordinal, name = 'id' AS primary_key" +
			", " + currentDB + " AS schema, table" +
			" FROM " + eachTable(parentTables, "fields", "{table: $t.name, name: $x.name, ordinal: $i + 1, comment: $x.comment,"+
			" data_type: (IF $x.kind = NONE THEN 'any' ELSE $x.kind END), nullable: "+nullable+", default: $x.default,"+
			" generated: (IF $x.computed != NONE THEN 'v' ELSE IF $x.value != NONE THEN 's' ELSE '' END)}") +
			" WHERE " + like(currentDB, "@schema") +
			" AND " + like("name", "@name") +
			" ORDER BY table, ordinal"),
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "the namespace"},
			{Name: "collation", Desc: "always absent: a field has no collation"},
			{Name: "comment"},
			{Name: "data_type", Desc: "the TYPE of the field as INFO writes it, and any for a field defined with no TYPE"},
			{Name: "default", Desc: "the DEFAULT expression of the field"},
			{
				Name: "generated",
				Desc: "s for a field with a VALUE clause, which the server computes on each write," +
					" v for a COMPUTED field, which it computes on each read, and empty otherwise",
			},
			{Name: "identity", Desc: "always empty: SurrealDB has no identity column, and a record id is the key"},
			{Name: "name", Desc: "the field, as DEFINE FIELD names it, such as tags.* for the items of an array"},
			{Name: "nullable", Desc: "whether the field takes NONE or NULL: a field with no TYPE, or a TYPE that names none or null"},
			{
				Name: "ordinal",
				Desc: "the position of the field in the order of the names, from 1." +
					" SurrealDB records no position, and INFO lists the fields by name",
			},
			{Name: "primary_key", Desc: "whether the field is id, the record id, which is the key of every table"},
			{Name: "schema", Desc: "the database"},
			{Name: "table"},
		},
		Params: []dbmeta.Param{
			{Name: "parent", Desc: "table name pattern, empty for every table", Default: ""},
			{Name: "schema", Desc: "database name pattern, empty for the database of the connection", Default: ""},
			{Name: "name", Desc: "field name pattern, empty for every field", Default: ""},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Column, error) {
			var v dbmeta.Column
			err := rows.Scan(&v.Catalog, &v.Collation, &v.Comment, &v.DataType, &v.Default,
				&v.Generated, &v.Identity, &v.Name, &v.Nullable, &v.Ordinal, &v.PrimaryKey,
				&v.Schema, &v.Table)
			return v, err
		},
	})

	// \di. The kind of an index is the word DEFINE INDEX writes after its
	// fields, such as UNIQUE, FULLTEXT or HNSW, and nothing for an ordinary
	// index.
	dbmeta.Indexes.Register(dbmeta.SurrealDB, &dbmeta.Binding[dbmeta.Index]{
		Stmt: from3("SELECT " + currentNS + " AS catalog, comment, name, false AS primary" +
			", " + currentDB + " AS schema, table, type, type = 'unique' AS unique" +
			" FROM " + eachTable(parentTables, "indexes", "{table: $t.name, name: $x.name, comment: $x.comment,"+
			" type: string::lowercase(string::split($x.index, ' ')[0])}") +
			" WHERE " + like(currentDB, "@schema") +
			" AND " + like("name", "@name") +
			" ORDER BY table, name"),
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "the namespace"},
			{Name: "comment"},
			{Name: "name"},
			{Name: "primary", Desc: "always false: the record id is the key, and no index holds it"},
			{Name: "schema", Desc: "the database"},
			{Name: "table"},
			{
				Name: "type",
				Desc: "the kind DEFINE INDEX names, in lower case, such as unique, fulltext, search," +
					" hnsw or count, and empty for an ordinary index",
			},
			{Name: "unique", Desc: "whether the index is UNIQUE"},
		},
		Params: []dbmeta.Param{
			{Name: "parent", Desc: "table name pattern, empty for every table", Default: ""},
			{Name: "schema", Desc: "database name pattern, empty for the database of the connection", Default: ""},
			{Name: "name", Desc: "index name pattern, empty for every index", Default: ""},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Index, error) {
			var v dbmeta.Index
			err := rows.Scan(&v.Catalog, &v.Comment, &v.Name, &v.Primary, &v.Schema,
				&v.Table, &v.Type, &v.Unique)
			return v, err
		},
	})

	// The fields of an index, in order. DEFINE INDEX takes fields and no
	// expression, and no field is descending.
	dbmeta.IndexColumns.Register(dbmeta.SurrealDB, &dbmeta.Binding[dbmeta.IndexColumn]{
		Stmt: from3("SELECT false AS descending, NULL AS expression, index, name, ordinal" +
			", " + currentDB + " AS schema, table" +
			" FROM array::flatten(" + eachTable(parentTables, "indexes", "array::map($x.cols, |$c, $n|"+
			" {table: $t.name, index: $x.name, name: $c, ordinal: $n + 1})") + ")" +
			" WHERE " + like(currentDB, "@schema") +
			" AND " + like("index", "@name") +
			" ORDER BY table, index, ordinal"),
		Fields: []dbmeta.Field{
			{Name: "descending", Desc: "always false: an index field has no order"},
			{Name: "expression", Desc: "always absent: DEFINE INDEX takes fields and no expression"},
			{Name: "index"},
			{Name: "name", Desc: "the field, such as tags.* for the items of an array"},
			{Name: "ordinal", Desc: "the position of the field in the index, from 1"},
			{Name: "schema", Desc: "the database"},
			{Name: "table"},
		},
		Params: []dbmeta.Param{
			{Name: "parent", Desc: "table name pattern, empty for every table", Default: ""},
			{Name: "schema", Desc: "database name pattern, empty for the database of the connection", Default: ""},
			{Name: "name", Desc: "index name pattern, empty for every index", Default: ""},
		},
		Scan: func(rows *sql.Rows) (dbmeta.IndexColumn, error) {
			var v dbmeta.IndexColumn
			err := rows.Scan(&v.Descending, &v.Expression, &v.Index, &v.Name, &v.Ordinal,
				&v.Schema, &v.Table)
			return v, err
		},
	})

	// Two kinds of constraint: a UNIQUE index, and the ASSERT of a field,
	// which is a check. An ASSERT has no name of its own, so it takes the
	// name of its field.
	dbmeta.Constraints.Register(dbmeta.SurrealDB, &dbmeta.Binding[dbmeta.Constraint]{
		Stmt: from3("SELECT comment, false AS deferrable, false AS deferred, definition, name" +
			", " + currentDB + " AS schema, table, type" +
			" FROM " + constraints +
			" WHERE " + like(currentDB, "@schema") +
			" AND " + like("table", "@parent") +
			" AND " + like("name", "@name") +
			" ORDER BY table, name"),
		Fields: []dbmeta.Field{
			{Name: "comment", Desc: "the comment of the index or the field"},
			{Name: "deferrable", Desc: "always false: the server checks a constraint at once"},
			{Name: "deferred", Desc: "always false, for the same reason"},
			{Name: "definition", Desc: "the ASSERT expression of a check, and the DEFINE INDEX statement of a unique index"},
			{Name: "name", Desc: "the index, or the field of a check"},
			{Name: "schema", Desc: "the database"},
			{Name: "table"},
			{Name: "type", Desc: "unique or check"},
		},
		Params: []dbmeta.Param{
			{Name: "parent", Desc: "table name pattern, empty for every table", Default: ""},
			{Name: "schema", Desc: "database name pattern, empty for the database of the connection", Default: ""},
			{Name: "name", Desc: "constraint name pattern, empty for every constraint", Default: ""},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Constraint, error) {
			var v dbmeta.Constraint
			err := rows.Scan(&v.Comment, &v.Deferrable, &v.Deferred, &v.Definition, &v.Name,
				&v.Schema, &v.Table, &v.Type)
			return v, err
		},
	})

	// The fields of each constraint: the fields of a unique index in order,
	// and the one field of a check.
	dbmeta.ConstraintColumns.Register(dbmeta.SurrealDB, &dbmeta.Binding[dbmeta.ConstraintColumn]{
		Stmt: from3("SELECT " + currentNS + " AS catalog, constraint" +
			", NULL AS foreign_catalog, NULL AS foreign_name, NULL AS foreign_schema, NULL AS foreign_table" +
			", name, ordinal, " + currentDB + " AS schema, table" +
			" FROM " + constraintColumns +
			" WHERE " + like(currentDB, "@schema") +
			" AND " + like("table", "@parent") +
			" AND " + like("constraint", "@name") +
			" ORDER BY table, constraint, ordinal"),
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "the namespace"},
			{Name: "constraint"},
			{Name: "foreign_catalog", Desc: "always absent: SurrealDB has no foreign key"},
			{Name: "foreign_name", Desc: "always absent, for the same reason"},
			{Name: "foreign_schema", Desc: "always absent, for the same reason"},
			{Name: "foreign_table", Desc: "always absent, for the same reason"},
			{Name: "name", Desc: "the field"},
			{Name: "ordinal", Desc: "the position of the field in the constraint, from 1"},
			{Name: "schema", Desc: "the database"},
			{Name: "table"},
		},
		Params: []dbmeta.Param{
			{Name: "parent", Desc: "table name pattern, empty for every table", Default: ""},
			{Name: "schema", Desc: "database name pattern, empty for the database of the connection", Default: ""},
			{Name: "name", Desc: "constraint name pattern, empty for every constraint", Default: ""},
		},
		Scan: func(rows *sql.Rows) (dbmeta.ConstraintColumn, error) {
			var v dbmeta.ConstraintColumn
			err := rows.Scan(&v.Catalog, &v.Constraint, &v.ForeignCatalog, &v.ForeignName,
				&v.ForeignSchema, &v.ForeignTable, &v.Name, &v.Ordinal, &v.Schema, &v.Table)
			return v, err
		},
	})

	// An event runs when a record of its table changes, which is a trigger.
	// The definition is the DEFINE EVENT statement that INFO FOR TABLE
	// returns without STRUCTURE.
	dbmeta.Triggers.Register(dbmeta.SurrealDB, &dbmeta.Binding[dbmeta.Trigger]{
		Stmt: from3("SELECT comment, definition, 'enabled' AS enabled, name, " + currentDB + " AS schema, table" +
			" FROM " + eachTable(parentTables, "events", "{table: $t.name, name: $x.name, comment: $x.comment,"+
			" definition: (INFO FOR TABLE $t.name).events[$x.name]}") +
			" WHERE " + like(currentDB, "@schema") +
			" AND " + like("name", "@name") +
			" ORDER BY table, name"),
		Fields: []dbmeta.Field{
			{Name: "comment"},
			{Name: "definition", Desc: "the DEFINE EVENT statement"},
			{Name: "enabled", Desc: "always enabled: SurrealQL has no clause that turns an event off"},
			{Name: "name", Desc: "the event"},
			{Name: "schema", Desc: "the database"},
			{Name: "table"},
		},
		Params: []dbmeta.Param{
			{Name: "parent", Desc: "table name pattern, empty for every table", Default: ""},
			{Name: "schema", Desc: "database name pattern, empty for the database of the connection", Default: ""},
			{Name: "name", Desc: "event name pattern, empty for every event", Default: ""},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Trigger, error) {
			var v dbmeta.Trigger
			err := rows.Scan(&v.Comment, &v.Definition, &v.Enabled, &v.Name, &v.Schema, &v.Table)
			return v, err
		},
	})

	// \ds. DEFINE SEQUENCE arrived in 3.0. A sequence hands out the next
	// integer, and keeps its start, the size of the batch a node takes and
	// a timeout.
	dbmeta.Sequences.Register(dbmeta.SurrealDB, &dbmeta.Binding[dbmeta.Sequence]{
		Stmt: from3("SELECT NULL AS comment, NULL AS cycles, 'int' AS data_type, '1' AS increment" +
			", NULL AS maximum, NULL AS minimum, name, '' AS owned_by, " + currentDB + " AS schema" +
			", <string> start AS start" +
			" FROM (INFO FOR DB STRUCTURE).sequences" +
			" WHERE " + like(currentDB, "@schema") +
			" AND " + like("name", "@name") +
			" ORDER BY name"),
		Fields: []dbmeta.Field{
			{Name: "comment", Desc: "always absent: a sequence carries no comment"},
			{Name: "cycles", Desc: "always absent: INFO says nothing of what follows the largest integer"},
			{Name: "data_type", Desc: "always int: a sequence hands out integers"},
			{Name: "increment", Desc: "always 1: sequence::nextval adds 1"},
			{Name: "maximum", Desc: "always absent: a sequence records no bound"},
			{Name: "minimum", Desc: "always absent, for the same reason"},
			{Name: "name"},
			{Name: "owned_by", Desc: "always empty: a sequence is owned by nothing"},
			{Name: "schema", Desc: "the database"},
			{Name: "start", Desc: "the START of the sequence"},
		},
		Params: []dbmeta.Param{
			{Name: "schema", Desc: "database name pattern, empty for the database of the connection", Default: ""},
			{Name: "name", Desc: "sequence name pattern, empty for every sequence", Default: ""},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Sequence, error) {
			var v dbmeta.Sequence
			err := rows.Scan(&v.Comment, &v.Cycles, &v.DataType, &v.Increment, &v.Maximum,
				&v.Minimum, &v.Name, &v.OwnedBy, &v.Schema, &v.Start)
			return v, err
		},
	})

	// \dd. Every object of the database that carries a COMMENT: a table, a
	// field, an index, an event, a function, a param, an analyzer and a user.
	// The namespaces and the databases carry one too, and Databases and
	// Schemas return it.
	dbmeta.Comments.Register(dbmeta.SurrealDB, &dbmeta.Binding[dbmeta.Comment]{
		Stmt: from3("SELECT comment, name, " + currentDB + " AS schema, type" +
			" FROM array::concat(" +
			"array::map(" + tables + ", |$x| {type: 'table', name: $x.name, comment: $x.comment})" +
			", " + eachTable(tables, "fields", "{type: 'column', name: $t.name + '.' + $x.name, comment: $x.comment}") +
			", " + eachTable(tables, "indexes", "{type: 'index', name: $x.name, comment: $x.comment}") +
			", " + eachTable(tables, "events", "{type: 'trigger', name: $x.name, comment: $x.comment}") +
			", array::map((INFO FOR DB STRUCTURE).functions, |$x| {type: 'function', name: $x.name, comment: $x.comment})" +
			", array::map((INFO FOR DB STRUCTURE).params, |$x| {type: 'param', name: $x.name, comment: $x.comment})" +
			", array::map((INFO FOR DB STRUCTURE).analyzers, |$x| {type: 'analyzer', name: $x.name, comment: $x.comment})" +
			", array::map((INFO FOR DB STRUCTURE).users, |$x| {type: 'user', name: $x.name, comment: $x.comment})" +
			")" +
			" WHERE comment != NONE" +
			" AND " + like(currentDB, "@schema") +
			" AND " + like("name", "@name") +
			" ORDER BY type, name"),
		Fields: []dbmeta.Field{
			{Name: "comment"},
			{Name: "name", Desc: "the object, and table.field for a field"},
			{Name: "schema", Desc: "the database"},
			{Name: "type", Desc: "table, column, index, trigger, function, param, analyzer or user"},
		},
		Params: []dbmeta.Param{
			{Name: "schema", Desc: "database name pattern, empty for the database of the connection", Default: ""},
			{Name: "name", Desc: "object name pattern, empty for every object", Default: ""},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Comment, error) {
			var v dbmeta.Comment
			err := rows.Scan(&v.Comment, &v.Name, &v.Schema, &v.Type)
			return v, err
		},
	})
}

// constraints is every constraint of the database: one for each UNIQUE index
// and one for each field with an ASSERT, each with its table, name, type,
// definition and comment.
//
// It and constraintColumns read every table and leave the pattern on the
// table to the WHERE, unlike the kinds that read parentTables. The server
// refuses a statement whose expressions nest too deep, and these two reached
// that limit with the pattern inside them.
var constraints = "array::concat(" +
	"array::filter(" + eachTable(tables, "indexes", "{table: $t.name, name: $x.name, type: 'unique',"+
	" comment: $x.comment, definition: (INFO FOR TABLE $t.name).indexes[$x.name], kind: $x.index}") +
	", |$k| $k.kind = 'UNIQUE')" +
	", array::filter(" + eachTable(tables, "fields", "{table: $t.name, name: $x.name, type: 'check',"+
	" comment: $x.comment, definition: $x.assert}") +
	", |$k| $k.definition != NONE))"

// constraintColumns is one row for each field of each constraint: the
// fields of a UNIQUE index in order, and the one field of an ASSERT.
var constraintColumns = "array::concat(" +
	"array::flatten(" + eachTable(tables, "indexes", "IF $x.index = 'UNIQUE' THEN array::map($x.cols, |$c, $n|"+
	" {table: $t.name, constraint: $x.name, name: $c, ordinal: $n + 1}) ELSE [] END") + ")" +
	", array::filter(" + eachTable(tables, "fields", "IF $x.assert != NONE THEN"+
	" {table: $t.name, constraint: $x.name, name: $x.name, ordinal: 1} END") +
	", |$k| $k != NONE))"
