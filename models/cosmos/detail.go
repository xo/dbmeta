package cosmos

import (
	"database/sql"
	"strconv"
	"strings"

	"github.com/xo/dbmeta"
)

// Index columns, constraints, constraint columns, partitions and the current
// schema. The first four read a statement that needs a container, and the
// connection names it in the path of its URL, as it does for the functions.

// indexColumnFields declares the columns of the statement that reads the paths of
// an indexing policy.
func indexColumnFields() []dbmeta.Field {
	return []dbmeta.Field{
		field("database", "the database that the URL names. It is the schema"),
		field("container", "the container that the URL names"),
		field("path", "the path of the entry, such as /title/? or /*"),
		field("kind", "included, excluded, composite, spatial, vector or full_text, from the list of the policy that holds the path"),
		field("path_order", "asc or desc for a path of a composite index, and absent for the other kinds"),
		field("position", "the position of the path inside its composite index, from 0, and absent for the other kinds"),
		field("composite_group", "the number of the composite index in the policy, from 0, and absent for the other kinds"),
		field("spatial_types", "the geometry types of a spatial index. The hosted service lists all four for a path that names Point"),
		field("vector_index_type", "the type of a vector index, such as flat, quantizedFlat or diskANN"),
		field("dimensions", "the dimensions of the vector that has the same path in the embedding policy"),
		field("full_text_path", "the path of a full text index"),
	}
}

// indexEntry says what an entry of the policy is, in the words that
// IndexColumn.Expression holds.
func indexEntry(r row) string {
	kind := r.text("kind")
	switch kind {
	case "composite":
		if g := r.number("composite_group"); g.Valid {
			return kind + " " + strconv.FormatInt(g.V, 10)
		}
	case "spatial":
		if types := r.list("spatial_types"); len(types) > 0 {
			return kind + " " + strings.Join(types, ",")
		}
	case "vector":
		var parts []string
		parts = append(parts, kind)
		if t := r.text("vector_index_type"); t != "" {
			parts = append(parts, t)
		}
		if d := r.number("dimensions"); d.Valid {
			parts = append(parts, strconv.FormatInt(d.V, 10))
		}
		return strings.Join(parts, " ")
	}
	return kind
}

// keyName is the name of a unique key, which has none of its own. It is the
// position of the key in the policy, which the driver counts from 0.
func keyName(position int64) string { return "unique_key_" + strconv.FormatInt(position, 10) }

func registerDetails() {
	// The columns of an index. A container has one index in Indexes, which is its
	// indexing policy, so every row names it. Each row is one path of one entry of
	// the policy. Expression says which entry it is, because IndexColumn has no
	// field for the kind, and Descending is the order of a path in a composite
	// index. The kinds that have no order are absent. The statement reads the one
	// container that the URL names, with no filter on the name.
	dbmeta.IndexColumns.Register(dbmeta.Cosmos, &dbmeta.Binding[dbmeta.IndexColumn]{
		Stmt:   catalog("$index_paths"),
		Fields: indexColumnFields(),
		Params: containerFilters("index"),
		Keep: keep(func(v dbmeta.IndexColumn) string { return v.Schema }, func(v dbmeta.IndexColumn) string { return v.Table },
			func(v dbmeta.IndexColumn) string { return v.Index }),
		Scan: func(rows *sql.Rows) (dbmeta.IndexColumn, error) {
			r, err := scanRow(rows)
			v := dbmeta.IndexColumn{
				Schema:     r.text("database"),
				Table:      r.text("container"),
				Index:      indexName,
				Name:       r.nullText("path"),
				Ordinal:    1,
				Expression: sql.Null[string]{V: indexEntry(r), Valid: true},
			}
			if p := r.number("position"); p.Valid {
				v.Ordinal = p.V + 1
			}
			switch r.text("path_order") {
			case "asc":
				v.Descending = sql.Null[bool]{V: false, Valid: true}
			case "desc":
				v.Descending = sql.Null[bool]{V: true, Valid: true}
			}
			return v, err
		},
	})

	// A unique key is a unique constraint of the container. It has no name, so
	// the name is its position. The paths are in ConstraintColumns.
	dbmeta.Constraints.Register(dbmeta.Cosmos, &dbmeta.Binding[dbmeta.Constraint]{
		Stmt: catalog("$unique_keys"),
		Fields: []dbmeta.Field{
			field("database", "the database that the URL names. It is the schema"),
			field("container", "the container that the URL names"),
			field("position", "the position of the key in the unique key policy, from 0. The name of the constraint holds it"),
		},
		Params: containerFilters("constraint"),
		Keep: keep(func(v dbmeta.Constraint) string { return v.Schema }, func(v dbmeta.Constraint) string { return v.Table },
			func(v dbmeta.Constraint) string { return v.Name }),
		Scan: func(rows *sql.Rows) (dbmeta.Constraint, error) {
			r, err := scanRow(rows)
			return dbmeta.Constraint{
				Schema: r.text("database"),
				Table:  r.text("container"),
				Name:   keyName(r.number("position").V),
				Type:   "unique",
			}, err
		},
	})

	// The paths of a unique key, in key order. The ordinal counts from 1.
	dbmeta.ConstraintColumns.Register(dbmeta.Cosmos, &dbmeta.Binding[dbmeta.ConstraintColumn]{
		Stmt: catalog("$unique_key_paths"),
		Fields: []dbmeta.Field{
			field("database", "the database that the URL names. It is the schema"),
			field("container", "the container that the URL names"),
			field("key_position", "the position of the key in the unique key policy, from 0"),
			field("position", "the position of the path inside its key, from 0"),
			field("path", "the path"),
		},
		Params: containerFilters("constraint"),
		Keep: keep(func(v dbmeta.ConstraintColumn) string { return v.Schema }, func(v dbmeta.ConstraintColumn) string { return v.Table },
			func(v dbmeta.ConstraintColumn) string { return v.Constraint }),
		Scan: func(rows *sql.Rows) (dbmeta.ConstraintColumn, error) {
			r, err := scanRow(rows)
			return dbmeta.ConstraintColumn{
				Schema:     r.text("database"),
				Table:      r.text("container"),
				Constraint: keyName(r.number("key_position").V),
				Name:       r.text("path"),
				Ordinal:    r.number("position").V + 1,
			}, err
		},
	})

	// \d+ of a partitioned table. A physical partition is a range of the hash of
	// the partition key. Its name is the id that the service gives it, and its
	// bound is the two ends of the range, as hexadecimal text. The status and the
	// parents of a range have no field here.
	dbmeta.Partitions.Register(dbmeta.Cosmos, &dbmeta.Binding[dbmeta.Partition]{
		Stmt: catalog("$partition_key_ranges"),
		Fields: []dbmeta.Field{
			field("database", "the database that the URL names. It is the schema"),
			field("container", "the container that the URL names"),
			field("id", "the id of the range, such as 0"),
			field("min_inclusive", "the start of the range as hexadecimal text, empty for the first range"),
			field("max_exclusive", "the end of the range as hexadecimal text"),
			field("status", "the status of the range, such as online"),
			field("parents", "the ids of the ranges that this range split from"),
		},
		Params: []dbmeta.Param{
			schemaParam(),
			parentParam("container"),
			{Name: "partition_schema", Desc: "database name pattern of the partition, which is the database of the container", Default: ""},
			nameParam("partition"),
		},
		Keep: func(v dbmeta.Partition, args map[string]any) bool {
			return dbmeta.Like(arg(args, "schema"), v.Schema) &&
				dbmeta.Like(arg(args, "parent"), v.Table) &&
				dbmeta.Like(arg(args, "partition_schema"), v.PartitionSchema) &&
				dbmeta.Like(arg(args, "name"), v.Partition)
		},
		Scan: func(rows *sql.Rows) (dbmeta.Partition, error) {
			r, err := scanRow(rows)
			var q dbmeta.Quoting
			return dbmeta.Partition{
				Schema:          r.text("database"),
				Table:           r.text("container"),
				PartitionSchema: r.text("database"),
				Partition:       r.text("id"),
				Type:            "partition",
				Bound: sql.Null[string]{
					V:     "FROM (" + dbmeta.QuoteLiteral(r.text("min_inclusive"), q) + ") TO (" + dbmeta.QuoteLiteral(r.text("max_exclusive"), q) + ")",
					Valid: true,
				},
			}, err
		},
	})

	// The database that the URL names is the current schema. The statement sends
	// no request. It has no user and no kind of key, so the current user is not
	// answered.
	dbmeta.CurrentSchema.Register(dbmeta.Cosmos, &dbmeta.Binding[dbmeta.Schema]{
		Stmt: catalog("$session"),
		Fields: []dbmeta.Field{
			field("host", "the host of the URL"),
			field("port", "the port of the URL, or 443 when it has none"),
			field("account", "the first label of the host"),
			field("database", "the database that the URL names. It is the schema, and the name is empty when the URL names none"),
			field("container", "the container that the URL names, and absent when it names none"),
			field("tls", "whether the connection uses TLS"),
			field("partition_key", "the partition key that the URL names, and absent when it names none"),
		},
		Scan: func(rows *sql.Rows) (dbmeta.Schema, error) {
			r, err := scanRow(rows)
			return dbmeta.Schema{Name: r.text("database")}, err
		},
	})
}
