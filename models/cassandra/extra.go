package cassandra

import (
	"database/sql"

	"github.com/xo/dbmeta"
)

func registerExtra() {
	// \di. A Cassandra index is secondary and never unique. The primary key
	// is not an index here: it is the storage layout of the table itself,
	// and it is returned by the constraints query instead.
	dbmeta.Indexes.Register(dbmeta.Cassandra, &dbmeta.Binding[dbmeta.Index]{
		Stmt: dbmeta.Stmt{
			always(`SELECT (text)'' AS "catalog"`),
			always(`, keyspace_name AS "schema"`),
			always(`, table_name AS "table"`),
			always(`, index_name AS "name"`),
			always(`, kind AS "type"`),
			always(`, (boolean)false AS "unique"`),
			always(`, (boolean)false AS "primary"`),
			always(`, (text)NULL AS "comment"`),
			always(`FROM system_schema.indexes`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "always empty: Cassandra has nothing above a keyspace"},
			{Name: "schema"}, {Name: "table"}, {Name: "name"},
			{Name: "type", Desc: "the index kind, COMPOSITES, KEYS or CUSTOM"},
			{Name: "unique", Desc: "always false: a Cassandra index enforces nothing"},
			{
				Name: "primary",
				Desc: "always false: the primary key is the storage layout of the" +
					" table rather than an index, and the constraints query returns it",
			},
			{Name: "comment", Desc: "always absent: an index carries no comment"},
		},
		Params: filters("table"),
		Scan: func(rows *sql.Rows) (dbmeta.Index, error) {
			var v dbmeta.Index
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Table, &v.Name, &v.Type,
				&v.Unique, &v.Primary, pad{})
			return v, err
		},
	})

	// The column an index is on.
	//
	// It is in the options map under target, which holds a bare column name
	// for an ordinary index and a call such as keys(m) for an index on a
	// collection. Scan reads both out of the one map, because CQL cannot
	// take a map apart in a statement.
	dbmeta.IndexColumns.Register(dbmeta.Cassandra, &dbmeta.Binding[dbmeta.IndexColumn]{
		Stmt: dbmeta.Stmt{
			always(`SELECT keyspace_name AS "schema"`),
			always(`, table_name AS "table"`),
			always(`, index_name AS "index"`),
			always(`, options AS "name"`),
			always(`, (bigint)1 AS "ordinal"`),
			always(`, options AS "expression"`),
			always(`, (boolean)false AS "descending"`),
			always(`FROM system_schema.indexes`),
		},
		Fields: []dbmeta.Field{
			{Name: "schema"}, {Name: "table"}, {Name: "index"},
			{Name: "name", Desc: "the column, read from the target entry of the options map"},
			{
				Name: "ordinal",
				Desc: "always 1: a Cassandra secondary index is on one column",
			},
			{
				Name: "expression",
				Desc: "the target as written when it is a call such as keys(m)," +
					" and absent for an index on a plain column",
			},
			{
				Name: "descending",
				Desc: "always false: an index has no direction, and the clustering" +
					" order that does is on the table",
			},
		},
		Params: filters("table"),
		Scan: func(rows *sql.Rows) (dbmeta.IndexColumn, error) {
			// The options map is selected twice, once for each field it
			// feeds, so that the query returns as many columns as it
			// declares fields. One copy is read and the other is only there
			// to be consumed.
			var (
				v              dbmeta.IndexColumn
				options, spare any
			)
			err := rows.Scan(&v.Schema, &v.Table, &v.Index, &options, &v.Ordinal,
				&spare, &v.Descending)
			col, call := indexTarget(textMap(options))
			v.Name = col
			if call != "" {
				v.Expression = sql.Null[string]{V: call, Valid: true}
			}
			return v, err
		},
	})

	// The primary key, as a constraint.
	//
	// Cassandra has one constraint and it is the primary key. There is no
	// foreign key, no unique constraint apart from the key itself, and no
	// check. D49 already says a NOT NULL is not a constraint row, and here
	// there is nothing else to confuse it with.
	//
	// The row is per column rather than per table, because the catalog has
	// no table level row to return and CQL cannot group. A caller reading
	// this sees the key once per column of it.
	//
	// The filter is fixed rather than optional, which is the one shape CQL
	// does allow, so a regular column is left out rather than returned as a
	// constraint it is not.
	//
	// It tests position and not kind. A key column carries its position
	// within the partition key or the clustering key, counting from zero,
	// and everything else carries -1, so the range is exact: on 5.0 it
	// selects the same 102 columns that kind IN ('partition_key',
	// 'clustering') does, and no regular or static column has a position at
	// or above zero on either release. kind would read better and 3.11
	// refuses it, with "IN predicates on non-primary-key columns (kind) is
	// not yet supported", because that arrived in 4.0. One form that works
	// everywhere beats a fragment that makes the same query mean two things.
	//
	// ALLOW FILTERING is the cost and it is bounded: system_schema.columns
	// is the catalog and every query here reads all of it.
	dbmeta.Constraints.Register(dbmeta.Cassandra, &dbmeta.Binding[dbmeta.Constraint]{
		Stmt: dbmeta.Stmt{
			always(`SELECT keyspace_name AS "schema"`),
			always(`, table_name AS "table"`),
			always(`, table_name AS "name"`),
			always(`, (text)'primary key' AS "type"`),
			always(`, column_name AS "definition"`),
			always(`, (boolean)false AS "deferrable"`),
			always(`, (boolean)false AS "deferred"`),
			always(`, (text)NULL AS "comment"`),
			always(`FROM system_schema.columns`),
			always(`WHERE position >= 0 ALLOW FILTERING`),
		},
		Fields: []dbmeta.Field{
			{Name: "schema"}, {Name: "table"},
			{
				Name: "name",
				Desc: "the table name: Cassandra does not name a primary key," +
					" so there is nothing else to call it",
			},
			{
				Name: "type",
				Desc: "always primary key: it is the only constraint Cassandra has",
			},
			{Name: "definition", Desc: "the column the row is for"},
			{Name: "deferrable", Desc: "always false: CQL has no deferrable constraint"},
			{Name: "deferred", Desc: "always false, for the same reason"},
			{Name: "comment", Desc: "always absent: a key carries no comment"},
		},
		Params: filters("table"),
		Scan: func(rows *sql.Rows) (dbmeta.Constraint, error) {
			var v dbmeta.Constraint
			err := rows.Scan(&v.Schema, &v.Table, &v.Name, &v.Type, &v.Definition,
				&v.Deferrable, &v.Deferred, pad{})
			return v, err
		},
	})

	// The columns of the primary key, in key order.
	dbmeta.ConstraintColumns.Register(dbmeta.Cassandra,
		&dbmeta.Binding[dbmeta.ConstraintColumn]{
			Stmt: dbmeta.Stmt{
				always(`SELECT (text)'' AS "catalog"`),
				always(`, keyspace_name AS "schema"`),
				always(`, table_name AS "table"`),
				always(`, table_name AS "constraint"`),
				always(`, column_name AS "name"`),
				always(`, position AS "ordinal"`),
				always(`, (text)NULL AS "foreign_catalog"`),
				always(`, (text)NULL AS "foreign_schema"`),
				always(`, (text)NULL AS "foreign_table"`),
				always(`, (text)NULL AS "foreign_name"`),
				always(`FROM system_schema.columns`),
				always(`WHERE position >= 0 ALLOW FILTERING`),
			},
			Fields: []dbmeta.Field{
				{Name: "catalog", Desc: "always empty: Cassandra has nothing above a keyspace"},
				{Name: "schema"}, {Name: "table"},
				{Name: "constraint", Desc: "the table name, because a key has no name"},
				{Name: "name"},
				{
					Name: "ordinal",
					Desc: "the position within the partition key or within the" +
						" clustering key, counted from zero. The two are numbered" +
						" separately, because Cassandra keeps them as two lists",
				},
				{
					Name: "foreign_catalog",
					Desc: "always absent: Cassandra has no foreign key",
				},
				{Name: "foreign_schema", Desc: "always absent, for the same reason"},
				{Name: "foreign_table", Desc: "always absent, for the same reason"},
				{Name: "foreign_name", Desc: "always absent, for the same reason"},
			},
			Params: filters("table"),
			Scan: func(rows *sql.Rows) (dbmeta.ConstraintColumn, error) {
				var v dbmeta.ConstraintColumn
				err := rows.Scan(&v.Catalog, &v.Schema, &v.Table, &v.Constraint,
					&v.Name, &v.Ordinal, pad{}, pad{}, pad{}, pad{})
				return v, err
			},
		})

	// The triggers on a table. A Cassandra trigger is a Java class named in
	// the options map, not a statement, so there is no definition to print.
	dbmeta.Triggers.Register(dbmeta.Cassandra, &dbmeta.Binding[dbmeta.Trigger]{
		Stmt: dbmeta.Stmt{
			always(`SELECT keyspace_name AS "schema"`),
			always(`, table_name AS "table"`),
			always(`, trigger_name AS "name"`),
			always(`, (text)'enabled' AS "enabled"`),
			always(`, options AS "definition"`),
			always(`, (text)NULL AS "comment"`),
			always(`FROM system_schema.triggers`),
		},
		Fields: []dbmeta.Field{
			{Name: "schema"}, {Name: "table"}, {Name: "name"},
			{
				Name: "enabled",
				Desc: "always enabled: Cassandra has no way to disable a trigger" +
					" short of dropping it",
			},
			{
				Name: "definition",
				Desc: "the options map, which names the Java class that runs." +
					" A Cassandra trigger is code rather than a statement",
			},
			{Name: "comment", Desc: "always absent: a trigger carries no comment"},
		},
		Params: filters("table"),
		Scan: func(rows *sql.Rows) (dbmeta.Trigger, error) {
			var (
				v       dbmeta.Trigger
				options any
			)
			err := rows.Scan(&v.Schema, &v.Table, &v.Name, &v.Enabled, &options, pad{})
			v.Definition = textList(options)
			return v, err
		},
	})

	// The comments, which Cassandra keeps on a table and nowhere else.
	dbmeta.Comments.Register(dbmeta.Cassandra, &dbmeta.Binding[dbmeta.Comment]{
		Stmt: dbmeta.Stmt{
			always(`SELECT keyspace_name AS "schema"`),
			always(`, table_name AS "name"`),
			always(`, (text)'table' AS "type"`),
			always(`, comment`),
			always(`FROM system_schema.tables`),
		},
		Fields: []dbmeta.Field{
			{Name: "schema"}, {Name: "name"},
			{
				Name: "type",
				Desc: "always table: a keyspace, a column and an index carry no" +
					" comment in Cassandra",
			},
			{
				Name: "comment",
				Desc: "empty rather than absent for a table nobody commented," +
					" which is what Cassandra stores",
			},
		},
		Params: filters("table"),
		Scan: func(rows *sql.Rows) (dbmeta.Comment, error) {
			var v dbmeta.Comment
			err := rows.Scan(&v.Schema, &v.Name, &v.Type, &v.Comment)
			return v, err
		},
	})

	// \dT. A user defined type, with its fields as one text.
	dbmeta.Types.Register(dbmeta.Cassandra, &dbmeta.Binding[dbmeta.Type]{
		Stmt: dbmeta.Stmt{
			always(`SELECT (text)'' AS "catalog"`),
			always(`, keyspace_name AS "schema"`),
			always(`, type_name AS "name"`),
			always(`, type_name AS "internal"`),
			always(`, (text)'composite' AS "kind"`),
			always(`, field_types AS "elements"`),
			always(`, (text)'' AS "owner"`),
			always(`, (text)NULL AS "access"`),
			always(`, (text)NULL AS "comment"`),
			always(`FROM system_schema.types`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "always empty: Cassandra has nothing above a keyspace"},
			{Name: "schema"}, {Name: "name"},
			{Name: "internal", Desc: "the same as the name: a type has one name here"},
			{
				Name: "kind",
				Desc: "always composite: a user defined type is the only kind" +
					" CQL lets anybody declare",
			},
			{Name: "elements", Desc: "the field types, in order, as one text"},
			{Name: "owner", Desc: "always empty: a type has no owner in the catalog"},
			{Name: "access", Desc: "always absent: a grant is on a keyspace or a table"},
			{Name: "comment", Desc: "always absent: a type carries no comment"},
		},
		Params: filters("type"),
		Scan: func(rows *sql.Rows) (dbmeta.Type, error) {
			var (
				v      dbmeta.Type
				fields any
			)
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Name, &v.Internal, &v.Kind,
				&fields, &v.Owner, pad{}, pad{})
			v.Elements = textList(fields)
			return v, err
		},
	})
}
