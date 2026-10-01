package arangodb

import (
	"database/sql"

	"github.com/xo/dbmeta"
)

// system is the filter that leaves out the collections ArangoDB keeps for
// itself, such as _graphs and _aqlfunctions. Their names start with _, which
// a user collection's name cannot.
const system = `(@with_system OR NOT STARTS_WITH(c.name, '_'))`

// The type and the null test of one property of a schema rule, which is the
// pair p, the name and the subschema, that ENTRIES returns. A JSON schema
// type is a name, such as string, or a list of names. A property with no type
// takes any value, null among them.
const (
	propType  = `p[1].type`
	propNulls = `(` + propType + ` == null OR ` + propType + ` == 'null'` +
		` OR (IS_ARRAY(` + propType + `) AND 'null' IN ` + propType + `))`
)

func registerRelations() {
	// \dn. A database is the schema, and AQL reaches only the database of
	// the connection, so this is that database alone (D168).
	schemaFields := []dbmeta.Field{
		{Name: "catalog", Desc: catalogDesc},
		{Name: "name", Desc: "the database of the connection"},
		{Name: "owner", Desc: "always empty: AQL does not read the users of a database"},
		{Name: "comment", Desc: "always absent: a database carries no comment"},
	}
	scanSchema := func(rows *sql.Rows) (dbmeta.Schema, error) {
		var v dbmeta.Schema
		err := rows.Scan(&v.Catalog, &v.Name, &v.Owner, &v.Comment)
		return v, err
	}
	dbmeta.Schemas.Register(dbmeta.ArangoDB, &dbmeta.Binding[dbmeta.Schema]{
		Stmt: dbmeta.Stmt{
			always(`FILTER ` + noSchema),
			always(`RETURN {catalog: '', name: CURRENT_DATABASE(), owner: '', comment: null}`),
		},
		Fields: schemaFields,
		Params: []dbmeta.Param{schemaParam},
		Scan:   scanSchema,
	})
	dbmeta.CurrentSchema.Register(dbmeta.ArangoDB, &dbmeta.Binding[dbmeta.Schema]{
		Stmt: dbmeta.Stmt{
			always(`RETURN {catalog: '', name: CURRENT_DATABASE(), owner: '', comment: null}`),
		},
		Fields: schemaFields,
		Scan:   scanSchema,
	})

	// \dt. COLLECTIONS() names every collection of the database, the
	// system ones among them, and nothing else: it has no type, and no
	// view, because an ArangoSearch view is not a collection.
	dbmeta.Tables.Register(dbmeta.ArangoDB, &dbmeta.Binding[dbmeta.Table]{
		Stmt: dbmeta.Stmt{
			always(`FOR c IN COLLECTIONS()`),
			always(`FILTER ` + system),
			always(`FILTER ` + noSchema),
			always(`FILTER ` + like(`c.name`, `@name`)),
			always(`FILTER (@types == '' OR 'collection' IN SPLIT(@types, ','))`),
			always(`SORT c.name`),
			always(`RETURN {catalog: '', schema: CURRENT_DATABASE(), name: c.name, type: 'collection', comment: null}`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: catalogDesc},
			{Name: "schema", Desc: schemaDesc},
			{Name: "name"},
			{
				Name: "type",
				Desc: "always collection: COLLECTIONS() does not say whether a" +
					" collection holds documents or edges, and AQL has no view",
			},
			{Name: "comment", Desc: "always absent: a collection carries no comment"},
		},
		Params: []dbmeta.Param{
			schemaParam,
			{Name: "name", Desc: "collection name pattern, empty for every collection", Default: ""},
			{Name: "with_system", Desc: "include the system collections, whose names start with _", Default: false},
			dbmeta.TypesParam(),
		},
		Scan: func(rows *sql.Rows) (dbmeta.Table, error) {
			var v dbmeta.Table
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Name, &v.Type, &v.Comment)
			return v, err
		},
	})

	// \d name. A column is a top-level property of the collection's JSON
	// schema rule, in the order the rule lists them, which ENTRIES keeps.
	// A collection with no rule has no column, because a document has no
	// fixed shape and nothing else records one. SCHEMA_GET reads the rule
	// from the collection's properties and not from its documents.
	dbmeta.Columns.Register(dbmeta.ArangoDB, &dbmeta.Binding[dbmeta.Column]{
		Stmt: dbmeta.Stmt{
			always(`FOR c IN COLLECTIONS()`),
			always(`FILTER ` + noSchema),
			always(`FILTER ` + like(`c.name`, `@parent`)),
			always(`LET s = SCHEMA_GET(c.name)`),
			always(`FILTER s != null`),
			always(`LET ps = ENTRIES(s.rule.properties)`),
			always(`FILTER LENGTH(ps) > 0`),
			always(`FOR i IN 0..LENGTH(ps) - 1`),
			always(`LET p = ps[i]`),
			always(`FILTER ` + like(`p[0]`, `@name`)),
			always(`SORT c.name, i`),
			always(`RETURN {catalog: '', schema: CURRENT_DATABASE(), table: c.name, name: p[0], ordinal: i + 1,`),
			always(` data_type: IS_STRING(` + propType + `) ? ` + propType + ` : (IS_ARRAY(` + propType + `) ? CONCAT_SEPARATOR(', ', ` + propType + `) : ''),`),
			always(` nullable: NOT (p[0] IN s.rule.required) OR ` + propNulls + `,`),
			always(` default: null, primary_key: p[0] == '_key', identity: null, generated: null, comment: null, collation: null}`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: catalogDesc},
			{Name: "schema", Desc: schemaDesc},
			{Name: "table", Desc: "the collection"},
			{Name: "name", Desc: "the property, which is the name of a document attribute"},
			{Name: "ordinal", Desc: "the position of the property in the rule, from 1"},
			{
				Name: "data_type",
				Desc: "the JSON schema type, such as string or integer, and the" +
					" names joined by a comma where the rule lists several. Empty" +
					" where the property names no type, which takes any value",
			},
			{
				Name: "nullable",
				Desc: "false where the rule requires the attribute and its type" +
					" does not take null. The rule checks a document as its" +
					" level says, and a document stored before the rule was set" +
					" is not checked, so a stored document can still lack it",
			},
			{Name: "default", Desc: "always absent: ArangoDB fills no attribute from a schema rule"},
			{Name: "primary_key", Desc: "true for _key, which is the primary key of every collection, where the rule names it"},
			{Name: "identity", Desc: "always absent: AQL does not read a collection's key generator"},
			{Name: "generated", Desc: "always absent: ArangoDB has no generated attribute"},
			{Name: "comment", Desc: "always absent: a property's description is not read"},
			{Name: "collation", Desc: "always absent: ArangoDB keeps one collation for the server"},
		},
		Params: []dbmeta.Param{
			schemaParam,
			{Name: "parent", Desc: "collection name pattern, empty for every collection", Default: ""},
			{Name: "name", Desc: "property name pattern, empty for every property", Default: ""},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Column, error) {
			var v dbmeta.Column
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Table, &v.Name, &v.Ordinal,
				&v.DataType, &v.Nullable, &v.Default, &v.PrimaryKey, &v.Identity,
				&v.Generated, &v.Comment, &v.Collation)
			return v, err
		},
	})

	// A JSON schema rule is the one constraint ArangoDB checks on a write
	// of a document, and it is a check on the whole document. The server
	// refuses a document that breaks it with error 1620, measured on
	// 3.12.12. A rule has no name.
	dbmeta.Constraints.Register(dbmeta.ArangoDB, &dbmeta.Binding[dbmeta.Constraint]{
		Stmt: dbmeta.Stmt{
			always(`FOR c IN COLLECTIONS()`),
			always(`FILTER ` + noSchema),
			always(`FILTER ` + like(`c.name`, `@parent`)),
			always(`FILTER ` + like(`''`, `@name`)),
			always(`LET s = SCHEMA_GET(c.name)`),
			always(`FILTER s != null`),
			always(`SORT c.name`),
			always(`RETURN {schema: CURRENT_DATABASE(), table: c.name, name: '', type: 'check',`),
			always(` definition: JSON_STRINGIFY(s), deferrable: false, deferred: false, comment: null}`),
		},
		Fields: []dbmeta.Field{
			{Name: "schema", Desc: schemaDesc},
			{Name: "table", Desc: "the collection"},
			{Name: "name", Desc: "always empty: a collection has one schema rule, and it has no name"},
			{Name: "type", Desc: "always check: a schema rule is the only constraint AQL reads"},
			{
				Name: "definition",
				Desc: "the schema as JSON: the rule, the level that says which" +
					" writes it checks, the message of a refused write, and its type",
			},
			{Name: "deferrable", Desc: "always false: a rule is checked at each write"},
			{Name: "deferred", Desc: "always false, for the same reason"},
			{Name: "comment", Desc: "always absent: a rule carries no comment"},
		},
		Params: []dbmeta.Param{
			schemaParam,
			{Name: "parent", Desc: "collection name pattern, empty for every collection", Default: ""},
			{Name: "name", Desc: "constraint name pattern. A rule has no name, so only an empty value or a pattern matching the empty string returns rows", Default: ""},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Constraint, error) {
			var v dbmeta.Constraint
			err := rows.Scan(&v.Schema, &v.Table, &v.Name, &v.Type, &v.Definition,
				&v.Deferrable, &v.Deferred, &v.Comment)
			return v, err
		},
	})
}
