package neo4j

import (
	"database/sql"

	"github.com/xo/dbmeta"
)

// The two kinds of table, as Table.Type spells them.
const (
	labelType        = "node label"
	relationshipType = "relationship type"
)

// typesFilter is the condition that the table type in col is one of the types
// in @types, which are joined by commas, and that everything matches an empty
// list. Cypher joins strings with +, so this is dbmeta.InList written for it.
func typesFilter(col string) string {
	return "(@types = '' OR (',' + @types + ',') CONTAINS (',' + " + col + " + ','))"
}

// The label or the type that an index or a constraint is on. A full text
// index can be on several labels, which Cypher writes joined by |, as in
// FOR (n:book|author). A token lookup index is on every node or every
// relationship and names none.
var onTable = "CASE WHEN labelsOrTypes IS NULL THEN '' ELSE " + join("labelsOrTypes", "|") + " END"

// onAnyTable is the condition that one of the labels or types an index or a
// constraint is on matches @parent. A token lookup index matches a pattern
// that matches the empty string.
func onAnyTable() string {
	return "(@parent = '' OR any(t IN coalesce(labelsOrTypes, ['']) WHERE t =~ " + regex("@parent") + "))"
}

// constraintType is the kind of a constraint, in the words the other models
// use where Neo4j has the same thing. A key is unique and present on every
// node or relationship, which is what a primary key is. Any other kind keeps
// the Neo4j name, lower case and with spaces.
const constraintType = "CASE" +
	" WHEN type IN ['NODE_KEY', 'RELATIONSHIP_KEY'] THEN 'primary key'" +
	" WHEN type IN ['UNIQUENESS', 'RELATIONSHIP_UNIQUENESS'," +
	" 'NODE_PROPERTY_UNIQUENESS', 'RELATIONSHIP_PROPERTY_UNIQUENESS'] THEN 'unique'" +
	" WHEN type IN ['NODE_PROPERTY_EXISTENCE', 'RELATIONSHIP_PROPERTY_EXISTENCE'] THEN 'not null'" +
	" ELSE toLower(replace(type, '_', ' ')) END"

// indexKey is whether an index belongs to a key constraint. The index of a
// constraint is created by its CREATE CONSTRAINT statement, and SHOW INDEXES
// returns that statement, which ends IS NODE KEY or IS RELATIONSHIP KEY on
// 5.26 and IS KEY on 2026.09. A uniqueness constraint ends IS UNIQUE.
const indexKey = "(owningConstraint IS NOT NULL AND" +
	" createStatement =~ '(?s).* IS (NODE |RELATIONSHIP )?KEY( OPTIONS .*)?')"

func registerRelations() {
	// \l. SHOW DATABASES returns one row for each database on each server
	// that hosts it, so DISTINCT keeps one for each database in a cluster.
	dbmeta.Databases.Register(dbmeta.Neo4j, &dbmeta.Binding[dbmeta.Database]{
		Stmt: both("SHOW DATABASES YIELD name" +
			" WHERE " + like("name", "@name") +
			" RETURN DISTINCT name AS `name`, '' AS `owner`, '' AS `encoding`, '' AS `collate`" +
			", '' AS `ctype`, NULL AS `access`, NULL AS `tablespace`, NULL AS `size`, NULL AS `comment`" +
			" ORDER BY `name`"),
		Fields: []dbmeta.Field{
			{Name: "name", Desc: "the database, which includes system, where Neo4j keeps users, roles and privileges"},
			{Name: "owner", Desc: "always empty: a database has no owner"},
			{Name: "encoding", Desc: "always empty: a database has no encoding, and a string is Unicode"},
			{Name: "collate", Desc: "always empty: a database has no collation"},
			{Name: "ctype", Desc: "always empty, for the same reason"},
			{Name: "access", Desc: "always absent: privileges returns what is granted on a database"},
			{Name: "tablespace", Desc: "always absent: a database has no tablespace"},
			{Name: "size", Desc: "always absent: SHOW DATABASES reports no size"},
			{Name: "comment", Desc: "always absent: a database carries no comment"},
		},
		Params: []dbmeta.Param{{Name: "name", Desc: "database name pattern, empty for every database", Default: ""}},
		Scan: func(rows *sql.Rows) (dbmeta.Database, error) {
			var v dbmeta.Database
			err := rows.Scan(&v.Name, &v.Owner, &v.Encoding, &v.Collate, &v.CType,
				&v.Access, &v.Tablespace, &v.Size, &v.Comment)
			return v, err
		},
	})

	// \dn. A database has no namespace inside it, so the schema is the
	// database the connection is in, and only that one. Every other
	// statement reads that database alone, and another database has no
	// tables to them.
	schemaFields := []dbmeta.Field{
		{Name: "catalog", Desc: "always empty: a database is the top of the tree a statement reads"},
		{Name: "name", Desc: "the database the connection is in"},
		{Name: "owner", Desc: "always empty: a database has no owner"},
		{Name: "comment", Desc: "always absent: a database carries no comment"},
	}
	scanSchema := func(rows *sql.Rows) (dbmeta.Schema, error) {
		var v dbmeta.Schema
		err := rows.Scan(&v.Catalog, &v.Name, &v.Owner, &v.Comment)
		return v, err
	}
	dbmeta.Schemas.Register(dbmeta.Neo4j, &dbmeta.Binding[dbmeta.Schema]{
		Stmt: both("CALL db.info() YIELD name" +
			" WHERE " + like("name", "@name") +
			" RETURN '' AS `catalog`, name AS `name`, '' AS `owner`, NULL AS `comment`"),
		Fields: schemaFields,
		Params: []dbmeta.Param{{Name: "name", Desc: "database name pattern, empty for the database of the connection", Default: ""}},
		Scan:   scanSchema,
	})
	dbmeta.CurrentSchema.Register(dbmeta.Neo4j, &dbmeta.Binding[dbmeta.Schema]{
		Stmt: both("CALL db.info() YIELD name" +
			" RETURN '' AS `catalog`, name AS `name`, '' AS `owner`, NULL AS `comment`"),
		Fields: schemaFields,
		Scan:   scanSchema,
	})

	// \dt. db.labels() lists the labels some node carries and
	// db.relationshipTypes() the types some relationship has, which Neo4j
	// reads from its count of each, not from the data. A label that no node
	// carries is not listed, even when an index or a constraint names it.
	dbmeta.Tables.Register(dbmeta.Neo4j, &dbmeta.Binding[dbmeta.Table]{
		Stmt: both("CALL db.info() YIELD name AS db" +
			" CALL () {" +
			" CALL db.labels() YIELD label RETURN label AS name, '" + labelType + "' AS type" +
			" UNION ALL" +
			" CALL db.relationshipTypes() YIELD relationshipType" +
			" RETURN relationshipType AS name, '" + relationshipType + "' AS type }" +
			" WITH db, name, type" +
			" WHERE " + like("db", "@schema") + " AND " + like("name", "@name") + " AND " + typesFilter("type") +
			" RETURN '' AS `catalog`, db AS `schema`, name AS `name`, type AS `type`, NULL AS `comment`" +
			" ORDER BY `type`, `name`"),
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "always empty: a database is the top of the tree a statement reads"},
			{Name: "schema", Desc: "the database the connection is in"},
			{Name: "name", Desc: "the label or the relationship type"},
			{Name: "type", Desc: labelType + " or " + relationshipType},
			{Name: "comment", Desc: "always absent: a label and a type carry no comment"},
		},
		Params: []dbmeta.Param{
			{Name: "schema", Desc: "database name pattern, empty for the database of the connection", Default: ""},
			{Name: "name", Desc: "label or relationship type name pattern, empty for every one", Default: ""},
			dbmeta.TypesParam(),
		},
		Scan: func(rows *sql.Rows) (dbmeta.Table, error) {
			var v dbmeta.Table
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Name, &v.Type, &v.Comment)
			return v, err
		},
	})

	// \di. Every index, including the index a constraint owns and the two
	// token lookup indexes a new database has.
	dbmeta.Indexes.Register(dbmeta.Neo4j, &dbmeta.Binding[dbmeta.Index]{
		Stmt: both("SHOW INDEXES YIELD name, type, labelsOrTypes, owningConstraint, createStatement" +
			" WHERE " + like(currentDB, "@schema") + " AND " + onAnyTable() + " AND " + like("name", "@name") +
			" RETURN '' AS `catalog`, " + currentDB + " AS `schema`, " + onTable + " AS `table`" +
			", name AS `name`, toLower(type) AS `type`, owningConstraint IS NOT NULL AS `unique`" +
			", " + indexKey + " AS `primary`, NULL AS `comment`" +
			" ORDER BY `table`, `name`"),
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "always empty: a database is the top of the tree a statement reads"},
			{Name: "schema", Desc: "the database the connection is in"},
			{
				Name: "table",
				Desc: "the label or the relationship type. A full text index on several reads them" +
					" joined by |, as Cypher writes them, and a token lookup index, which is on" +
					" every node or every relationship, reads empty",
			},
			{Name: "name"},
			{Name: "type", Desc: "range, text, point, fulltext, vector or lookup"},
			{Name: "unique", Desc: "whether a key or a uniqueness constraint owns the index, which is what makes it unique"},
			{Name: "primary", Desc: "whether a key constraint owns the index"},
			{Name: "comment", Desc: "always absent: an index carries no comment"},
		},
		Params: []dbmeta.Param{
			{Name: "schema", Desc: "database name pattern, empty for the database of the connection", Default: ""},
			{Name: "parent", Desc: "label or relationship type name pattern, empty for every one", Default: ""},
			{Name: "name", Desc: "index name pattern, empty for every index", Default: ""},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Index, error) {
			var v dbmeta.Index
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Table, &v.Name, &v.Type,
				&v.Unique, &v.Primary, &v.Comment)
			return v, err
		},
	})

	// The properties of an index, in order. A Neo4j index has no direction
	// and no expression, and a token lookup index has no property.
	dbmeta.IndexColumns.Register(dbmeta.Neo4j, &dbmeta.Binding[dbmeta.IndexColumn]{
		Stmt: composed("SHOW INDEXES YIELD name, labelsOrTypes, properties" +
			" WHERE properties IS NOT NULL" +
			" AND " + like(currentDB, "@schema") + " AND " + onAnyTable() + " AND " + like("name", "@name") +
			" UNWIND range(0, size(properties) - 1) AS i" +
			" RETURN " + currentDB + " AS `schema`, " + onTable + " AS `table`, name AS `index`" +
			", properties[i] AS `name`, i + 1 AS `ordinal`, NULL AS `expression`, false AS `descending`" +
			" ORDER BY `table`, `index`, `ordinal`"),
		Fields: []dbmeta.Field{
			{Name: "schema", Desc: "the database the connection is in"},
			{Name: "table", Desc: "the label or the relationship type, as Index.Table reads it"},
			{Name: "index"},
			{Name: "name", Desc: "the property"},
			{Name: "ordinal", Desc: "the position of the property, from 1"},
			{Name: "expression", Desc: "always absent: a Neo4j index is on properties and not on an expression"},
			{Name: "descending", Desc: "always false: a Neo4j index has no direction"},
		},
		Params: []dbmeta.Param{
			{Name: "schema", Desc: "database name pattern, empty for the database of the connection", Default: ""},
			{Name: "parent", Desc: "label or relationship type name pattern, empty for every one", Default: ""},
			{Name: "name", Desc: "index name pattern, empty for every index", Default: ""},
		},
		Scan: func(rows *sql.Rows) (dbmeta.IndexColumn, error) {
			var v dbmeta.IndexColumn
			err := rows.Scan(&v.Schema, &v.Table, &v.Index, &v.Name, &v.Ordinal,
				&v.Expression, &v.Descending)
			return v, err
		},
	})

	// Every constraint. The definition is the CREATE CONSTRAINT statement
	// that SHOW CONSTRAINTS returns.
	dbmeta.Constraints.Register(dbmeta.Neo4j, &dbmeta.Binding[dbmeta.Constraint]{
		Stmt: both("SHOW CONSTRAINTS YIELD name, type, labelsOrTypes, createStatement" +
			" WHERE " + like(currentDB, "@schema") + " AND " + onAnyTable() + " AND " + like("name", "@name") +
			" RETURN " + currentDB + " AS `schema`, " + onTable + " AS `table`, name AS `name`" +
			", " + constraintType + " AS `type`, createStatement AS `definition`" +
			", false AS `deferrable`, false AS `deferred`, NULL AS `comment`" +
			" ORDER BY `table`, `name`"),
		Fields: []dbmeta.Field{
			{Name: "schema", Desc: "the database the connection is in"},
			{Name: "table", Desc: "the label or the relationship type"},
			{Name: "name"},
			{
				Name: "type",
				Desc: "primary key for a node key or a relationship key, unique, not null for a" +
					" property existence constraint, and the Neo4j name in lower case for any" +
					" other, such as node property type",
			},
			{Name: "definition", Desc: "the CREATE CONSTRAINT statement that makes it"},
			{Name: "deferrable", Desc: "always false: Neo4j checks a constraint when a transaction commits, and nothing can defer it"},
			{Name: "deferred", Desc: "always false, for the same reason"},
			{Name: "comment", Desc: "always absent: a constraint carries no comment"},
		},
		Params: []dbmeta.Param{
			{Name: "schema", Desc: "database name pattern, empty for the database of the connection", Default: ""},
			{Name: "parent", Desc: "label or relationship type name pattern, empty for every one", Default: ""},
			{Name: "name", Desc: "constraint name pattern, empty for every constraint", Default: ""},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Constraint, error) {
			var v dbmeta.Constraint
			err := rows.Scan(&v.Schema, &v.Table, &v.Name, &v.Type, &v.Definition,
				&v.Deferrable, &v.Deferred, &v.Comment)
			return v, err
		},
	})

	// The properties of a constraint, in order. Neo4j has no foreign key, so
	// the foreign fields are always absent.
	dbmeta.ConstraintColumns.Register(dbmeta.Neo4j, &dbmeta.Binding[dbmeta.ConstraintColumn]{
		Stmt: composed("SHOW CONSTRAINTS YIELD name, labelsOrTypes, properties" +
			" WHERE properties IS NOT NULL" +
			" AND " + like(currentDB, "@schema") + " AND " + onAnyTable() + " AND " + like("name", "@name") +
			" UNWIND range(0, size(properties) - 1) AS i" +
			" RETURN '' AS `catalog`, " + currentDB + " AS `schema`, " + onTable + " AS `table`" +
			", name AS `constraint`, properties[i] AS `name`, i + 1 AS `ordinal`" +
			", NULL AS `foreign_catalog`, NULL AS `foreign_schema`, NULL AS `foreign_table`, NULL AS `foreign_name`" +
			" ORDER BY `table`, `constraint`, `ordinal`"),
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "always empty: a database is the top of the tree a statement reads"},
			{Name: "schema", Desc: "the database the connection is in"},
			{Name: "table", Desc: "the label or the relationship type"},
			{Name: "constraint"},
			{Name: "name", Desc: "the property"},
			{Name: "ordinal", Desc: "the position of the property, from 1"},
			{Name: "foreign_catalog", Desc: "always absent: Neo4j has no foreign key"},
			{Name: "foreign_schema", Desc: "always absent, for the same reason"},
			{Name: "foreign_table", Desc: "always absent, for the same reason"},
			{Name: "foreign_name", Desc: "always absent, for the same reason"},
		},
		Params: []dbmeta.Param{
			{Name: "schema", Desc: "database name pattern, empty for the database of the connection", Default: ""},
			{Name: "parent", Desc: "label or relationship type name pattern, empty for every one", Default: ""},
			{Name: "name", Desc: "constraint name pattern, empty for every constraint", Default: ""},
		},
		Scan: func(rows *sql.Rows) (dbmeta.ConstraintColumn, error) {
			var v dbmeta.ConstraintColumn
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Table, &v.Constraint, &v.Name, &v.Ordinal,
				&v.ForeignCatalog, &v.ForeignSchema, &v.ForeignTable, &v.ForeignName)
			return v, err
		},
	})
}
