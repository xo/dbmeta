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
			fixed("SELECT ", `(text)''`, "keyspace_name", "catalog"),
			always(`, keyspace_name AS "schema"`),
			always(`, table_name AS "table"`),
			always(`, index_name AS "name"`),
			always(`, kind AS "type"`),
			fixed(", ", `(boolean)false`, "keyspace_name", "unique"),
			fixed(", ", `(boolean)false`, "keyspace_name", "primary"),
			fixed(", ", `(text)NULL`, "keyspace_name", "comment"),
			always(`, options`),
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
			{
				Name: "options",
				Desc: "the options map. It feeds the options and the using fields and is not a field of its own",
			},
		},
		Params: childFilters("index"),
		Keep:   keep(func(v dbmeta.Index) string { return v.Schema }, func(v dbmeta.Index) string { return v.Table }, func(v dbmeta.Index) string { return v.Name }),
		Scan: func(rows *sql.Rows) (dbmeta.Index, error) {
			var (
				v       dbmeta.Index
				options any
			)
			err := rows.Scan(pad{}, &v.Schema, &v.Table, &v.Name, &v.Type,
				pad{}, pad{}, pad{}, &options)
			v.Using, v.Options = indexUsing(textMap(options))
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
			fixed(", ", `(bigint)1`, "keyspace_name", "ordinal"),
			always(`, options AS "expression"`),
			fixed(", ", `(boolean)false`, "keyspace_name", "descending"),
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
		Params: childFilters("index"),
		Keep:   keep(func(v dbmeta.IndexColumn) string { return v.Schema }, func(v dbmeta.IndexColumn) string { return v.Table }, func(v dbmeta.IndexColumn) string { return v.Index }),
		Scan: func(rows *sql.Rows) (dbmeta.IndexColumn, error) {
			// The options map is selected twice, once for each field it
			// feeds, so that the query returns as many columns as it
			// declares fields. One copy is read and the other is only there
			// to be consumed.
			var (
				v              dbmeta.IndexColumn
				options, spare any
			)
			err := rows.Scan(&v.Schema, &v.Table, &v.Index, &options, pad{},
				&spare, pad{})
			v.Ordinal = 1
			col, call := indexTarget(textMap(options))
			// A target that names no column leaves the name absent.
			v.Name = sql.Null[string]{V: col, Valid: col != ""}
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
	// or above zero on either release. kind reads better and 3.11
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
			fixed(", ", `(text)'primary key'`, "keyspace_name", "type"),
			always(`, column_name AS "definition"`),
			fixed(", ", `(boolean)false`, "keyspace_name", "deferrable"),
			fixed(", ", `(boolean)false`, "keyspace_name", "deferred"),
			fixed(", ", `(text)NULL`, "keyspace_name", "comment"),
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
		Params: childFilters("constraint"),
		Keep:   keep(func(v dbmeta.Constraint) string { return v.Schema }, func(v dbmeta.Constraint) string { return v.Table }, func(v dbmeta.Constraint) string { return v.Name }),
		Scan: func(rows *sql.Rows) (dbmeta.Constraint, error) {
			var v dbmeta.Constraint
			err := rows.Scan(&v.Schema, &v.Table, &v.Name, pad{}, &v.Definition,
				pad{}, pad{}, pad{})
			v.Type = "primary key"
			return v, err
		},
	})

	// The columns of the primary key, in key order.
	dbmeta.ConstraintColumns.Register(dbmeta.Cassandra,
		&dbmeta.Binding[dbmeta.ConstraintColumn]{
			Stmt: dbmeta.Stmt{
				fixed("SELECT ", `(text)''`, "keyspace_name", "catalog"),
				always(`, keyspace_name AS "schema"`),
				always(`, table_name AS "table"`),
				always(`, table_name AS "constraint"`),
				always(`, column_name AS "name"`),
				always(`, position AS "ordinal"`),
				fixed(", ", `(text)NULL`, "keyspace_name", "foreign_catalog"),
				fixed(", ", `(text)NULL`, "keyspace_name", "foreign_schema"),
				fixed(", ", `(text)NULL`, "keyspace_name", "foreign_table"),
				fixed(", ", `(text)NULL`, "keyspace_name", "foreign_name"),
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
			Params: childFilters("constraint"),
			Keep:   keep(func(v dbmeta.ConstraintColumn) string { return v.Schema }, func(v dbmeta.ConstraintColumn) string { return v.Table }, func(v dbmeta.ConstraintColumn) string { return v.Constraint }),
			Scan: func(rows *sql.Rows) (dbmeta.ConstraintColumn, error) {
				var v dbmeta.ConstraintColumn
				err := rows.Scan(pad{}, &v.Schema, &v.Table, &v.Constraint,
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
			fixed(", ", `(text)'enabled'`, "keyspace_name", "enabled"),
			always(`, options AS "definition"`),
			fixed(", ", `(text)NULL`, "keyspace_name", "comment"),
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
		Params: childFilters("trigger"),
		Keep:   keep(func(v dbmeta.Trigger) string { return v.Schema }, func(v dbmeta.Trigger) string { return v.Table }, func(v dbmeta.Trigger) string { return v.Name }),
		Scan: func(rows *sql.Rows) (dbmeta.Trigger, error) {
			var (
				v       dbmeta.Trigger
				options any
			)
			err := rows.Scan(&v.Schema, &v.Table, &v.Name, pad{}, &options, pad{})
			v.Enabled = "enabled"
			v.Definition = textList(options)
			return v, err
		},
	})

	// The comments, which Cassandra keeps on a table and nowhere else.
	dbmeta.Comments.Register(dbmeta.Cassandra, &dbmeta.Binding[dbmeta.Comment]{
		Stmt: dbmeta.Stmt{
			always(`SELECT keyspace_name AS "schema"`),
			always(`, table_name AS "name"`),
			fixed(", ", `(text)'table'`, "keyspace_name", "type"),
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
		Keep:   keep(func(v dbmeta.Comment) string { return v.Schema }, nil, func(v dbmeta.Comment) string { return v.Name }),
		Scan: func(rows *sql.Rows) (dbmeta.Comment, error) {
			var v dbmeta.Comment
			err := rows.Scan(&v.Schema, &v.Name, pad{}, &v.Comment)
			v.Type = "table"
			return v, err
		},
	})

	// \dT. A user defined type, with its fields as one text.
	dbmeta.Types.Register(dbmeta.Cassandra, &dbmeta.Binding[dbmeta.Type]{
		Stmt: dbmeta.Stmt{
			fixed("SELECT ", `(text)''`, "keyspace_name", "catalog"),
			always(`, keyspace_name AS "schema"`),
			always(`, type_name AS "name"`),
			always(`, type_name AS "internal"`),
			fixed(", ", `(text)'composite'`, "keyspace_name", "kind"),
			always(`, field_types AS "elements"`),
			fixed(", ", `(text)NULL`, "keyspace_name", "owner"),
			fixed(", ", `(text)NULL`, "keyspace_name", "access"),
			fixed(", ", `(text)NULL`, "keyspace_name", "comment"),
			fixed(", ", `(text)'tuple'`, "keyspace_name", "size"),
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
			{Name: "owner", Desc: "always absent: a type has no owner in the catalog"},
			{Name: "access", Desc: "always absent: a grant is on a keyspace or a table"},
			{Name: "comment", Desc: "always absent: a type carries no comment"},
			{Name: "size", Desc: "always tuple, the word psql uses for a composite type"},
		},
		Params: filters("type"),
		Keep:   keep(func(v dbmeta.Type) string { return v.Schema }, nil, func(v dbmeta.Type) string { return v.Name }),
		Scan: func(rows *sql.Rows) (dbmeta.Type, error) {
			var (
				v      dbmeta.Type
				fields any
			)
			err := rows.Scan(pad{}, &v.Schema, &v.Name, &v.Internal, pad{},
				&fields, pad{}, pad{}, pad{}, pad{})
			v.Kind = "composite"
			v.Size = sql.Null[string]{V: "tuple", Valid: true}
			v.Elements = textList(fields)
			return v, err
		},
	})
}
