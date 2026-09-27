package couchbase

import (
	"database/sql"

	"github.com/xo/dbmeta"
)

// system is the scope Couchbase keeps for itself in every bucket, which holds
// collections such as _query and _mobile.
const system = "_system"

func registerRelations() {
	// \l. A bucket is the top of the tree a statement names, so it is the
	// database.
	dbmeta.Databases.Register(dbmeta.Couchbase, &dbmeta.Binding[dbmeta.Database]{
		Stmt: dbmeta.Stmt{
			from76("SELECT b.name AS `name`"),
			from76(", '' AS `owner`"),
			from76(", '' AS `encoding`"),
			from76(", '' AS `collate`"),
			from76(", '' AS `ctype`"),
			from76(", NULL AS `access`"),
			from76(", NULL AS `tablespace`"),
			from76(", '' AS `size`"),
			from76(", NULL AS `comment`"),
			from76("FROM system:buckets b"),
			from76("WHERE " + like("b.name", "@name")),
			from76("ORDER BY b.name"),
		},
		Fields: []dbmeta.Field{
			{Name: "name"},
			{Name: "owner", Desc: "always empty: a bucket has no owner"},
			{Name: "encoding", Desc: "always empty: a document is JSON, which is UTF-8"},
			{Name: "collate", Desc: "always empty: a bucket has no collation"},
			{Name: "ctype", Desc: "always empty, for the same reason"},
			{Name: "access", Desc: "always absent: privileges returns what is granted on a bucket"},
			{Name: "tablespace", Desc: "always absent: a bucket has no tablespace"},
			{Name: "size", Desc: "always empty: system:buckets carries no quota or size"},
			{Name: "comment", Desc: "always absent: a bucket carries no comment"},
		},
		Params: []dbmeta.Param{{Name: "name", Desc: "bucket name pattern, empty for every bucket", Default: ""}},
		Scan: func(rows *sql.Rows) (dbmeta.Database, error) {
			var v dbmeta.Database
			err := rows.Scan(&v.Name, &v.Owner, &v.Encoding, &v.Collate, &v.CType,
				&v.Access, &v.Tablespace, &v.Size, &v.Comment)
			return v, err
		},
	})

	// \dn. A scope is the schema, and its bucket is its catalog.
	dbmeta.Schemas.Register(dbmeta.Couchbase, &dbmeta.Binding[dbmeta.Schema]{
		Stmt: dbmeta.Stmt{
			from76("SELECT s.`bucket` AS `catalog`"),
			from76(", s.name AS `name`"),
			from76(", '' AS `owner`"),
			from76(", NULL AS `comment`"),
			from76("FROM system:all_scopes s"),
			from76("WHERE (@with_system OR s.name != '" + system + "')"),
			from76("AND " + like("s.name", "@name")),
			from76("ORDER BY s.`bucket`, s.name"),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "the bucket the scope is in"},
			{Name: "name"},
			{Name: "owner", Desc: "always empty: a scope has no owner"},
			{Name: "comment", Desc: "always absent: a scope carries no comment"},
		},
		Params: []dbmeta.Param{
			{Name: "name", Desc: "scope name pattern, empty for every scope", Default: ""},
			{Name: "with_system", Desc: "include _system, the scope Couchbase keeps in every bucket", Default: false},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Schema, error) {
			var v dbmeta.Schema
			err := rows.Scan(&v.Catalog, &v.Name, &v.Owner, &v.Comment)
			return v, err
		},
	})

	// \dt. A collection is the table. system:all_keyspaces lists the bucket
	// itself as well, as the keyspace a two part name reaches, and it is left
	// out, because its _default collection is in the list as a collection.
	dbmeta.Tables.Register(dbmeta.Couchbase, &dbmeta.Binding[dbmeta.Table]{
		Stmt: dbmeta.Stmt{
			from76("SELECT k.`bucket` AS `catalog`"),
			from76(", k.`scope` AS `schema`"),
			from76(", k.name AS `name`"),
			from76(", 'collection' AS `type`"),
			from76(", NULL AS `comment`"),
			from76("FROM system:all_keyspaces k"),
			from76("WHERE k.`scope` IS NOT MISSING"),
			from76("AND (@with_system OR k.`scope` != '" + system + "')"),
			from76("AND " + like("k.`scope`", "@schema")),
			from76("AND " + like("k.name", "@name")),
			from76("ORDER BY k.`bucket`, k.`scope`, k.name"),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "the bucket the collection is in"},
			{Name: "schema", Desc: "the scope the collection is in"},
			{Name: "name"},
			{Name: "type", Desc: "always collection: SQL++ has no view"},
			{Name: "comment", Desc: "always absent: a collection carries no comment"},
		},
		Params: []dbmeta.Param{
			{Name: "schema", Desc: "scope name pattern, empty for every scope", Default: ""},
			{Name: "name", Desc: "collection name pattern, empty for every collection", Default: ""},
			{Name: "with_system", Desc: "include the collections of _system", Default: false},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Table, error) {
			var v dbmeta.Table
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Name, &v.Type, &v.Comment)
			return v, err
		},
	})

	// \di. An index on a bucket's default collection names the bucket as
	// its keyspace and carries no bucket_id or scope_id. It is reported on
	// the collection _default in the scope _default, which is where it is.
	dbmeta.Indexes.Register(dbmeta.Couchbase, &dbmeta.Binding[dbmeta.Index]{
		Stmt: dbmeta.Stmt{
			from76("SELECT IFMISSING(i.bucket_id, i.keyspace_id) AS `catalog`"),
			from76(", " + indexSchema + " AS `schema`"),
			from76(", " + indexTable + " AS `table`"),
			from76(", i.name AS `name`"),
			from76(", i.`using` AS `type`"),
			from76(", false AS `unique`"),
			from76(", IFMISSING(i.is_primary, false) AS `primary`"),
			from76(", NULL AS `comment`"),
			from76("FROM system:indexes i"),
			from76("WHERE " + like(indexSchema, "@schema")),
			from76("AND " + like(indexTable, "@name")),
			from76("ORDER BY 1, 2, 3, 4"),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "the bucket"},
			{Name: "schema", Desc: "the scope"},
			{Name: "table", Desc: "the collection"},
			{Name: "name", Desc: "the index name. A primary index is #primary unless it was named"},
			{Name: "type", Desc: "the index service, which is gsi"},
			{Name: "unique", Desc: "always false: a Couchbase index enforces nothing"},
			{Name: "primary", Desc: "whether it is a primary index, which indexes every document key"},
			{Name: "comment", Desc: "always absent: an index carries no comment"},
		},
		Params: []dbmeta.Param{
			{Name: "schema", Desc: "scope name pattern, empty for every scope", Default: ""},
			{Name: "name", Desc: "collection name pattern, empty for every collection", Default: ""},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Index, error) {
			var v dbmeta.Index
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Table, &v.Name, &v.Type,
				&v.Unique, &v.Primary, &v.Comment)
			return v, err
		},
	})

	// The keys of an index, in order. index_key is a list of the key texts,
	// such as `author_id` or `title` DESC or an ARRAY expression. A key that
	// is one field has that field as its name and no expression. Any other
	// key has an empty name and its text as the expression, which is how
	// PostgreSQL reports an expression key. A primary index has no key.
	dbmeta.IndexColumns.Register(dbmeta.Couchbase, &dbmeta.Binding[dbmeta.IndexColumn]{
		Stmt: dbmeta.Stmt{
			from76("SELECT " + indexSchema + " AS `schema`"),
			from76(", " + indexTable + " AS `table`"),
			from76(", i.name AS `index`"),
			from76(", CASE WHEN " + plainKey + " THEN TRIM(" + keyText + ", '`') ELSE '' END AS `name`"),
			from76(", k.pos + 1 AS `ordinal`"),
			from76(", CASE WHEN " + plainKey + " THEN NULL ELSE " + keyText + " END AS `expression`"),
			from76(", REGEXP_CONTAINS(k.`key`, ' DESC$') AS `descending`"),
			from76("FROM system:indexes i"),
			from76("UNNEST ARRAY {\"pos\": p, \"key\": v} FOR p:v IN i.index_key END AS k"),
			from76("WHERE " + like(indexSchema, "@schema")),
			from76("AND " + like(indexTable, "@name")),
			from76("ORDER BY 1, 2, 3, 5"),
		},
		Fields: []dbmeta.Field{
			{Name: "schema", Desc: "the scope"},
			{Name: "table", Desc: "the collection"},
			{Name: "index"},
			{Name: "name", Desc: "the field a key indexes, and empty for a key that is an expression"},
			{Name: "ordinal", Desc: "the position of the key, from 1"},
			{Name: "expression", Desc: "the text of a key that is not one field, such as an ARRAY expression"},
			{Name: "descending", Desc: "whether the key is DESC"},
		},
		Params: []dbmeta.Param{
			{Name: "schema", Desc: "scope name pattern, empty for every scope", Default: ""},
			{Name: "name", Desc: "collection name pattern, empty for every collection", Default: ""},
		},
		Scan: func(rows *sql.Rows) (dbmeta.IndexColumn, error) {
			var v dbmeta.IndexColumn
			err := rows.Scan(&v.Schema, &v.Table, &v.Index, &v.Name, &v.Ordinal,
				&v.Expression, &v.Descending)
			return v, err
		},
	})

	// \ds. system:sequences arrived in 7.6, which is the floor, and it keeps
	// no start value, only the next block it hands out.
	dbmeta.Sequences.Register(dbmeta.Couchbase, &dbmeta.Binding[dbmeta.Sequence]{
		Stmt: dbmeta.Stmt{
			from76("SELECT s.scope_id AS `schema`"),
			from76(", s.name AS `name`"),
			from76(", NULL AS `data_type`"),
			from76(", NULL AS `start`"),
			from76(", s.`min` AS `minimum`"),
			from76(", s.`max` AS `maximum`"),
			from76(", s.`increment` AS `increment`"),
			from76(", s.`cycle` AS `cycles`"),
			from76(", '' AS `owned_by`"),
			from76(", NULL AS `comment`"),
			from76("FROM system:sequences s"),
			from76("WHERE " + like("s.scope_id", "@schema")),
			from76("AND " + like("s.name", "@name")),
			from76("ORDER BY s.`bucket`, s.scope_id, s.name"),
		},
		Fields: []dbmeta.Field{
			{
				Name: "schema",
				Desc: "the scope. A sequence belongs to a bucket too, and the row" +
					" has no field for it, so two scopes of one name in two buckets" +
					" look the same",
			},
			{Name: "name"},
			{Name: "data_type", Desc: "always absent: a sequence is a 64 bit integer and the catalog names no type"},
			{Name: "start", Desc: "always absent: the catalog keeps the next block and not the start"},
			{Name: "minimum"}, {Name: "maximum"}, {Name: "increment"},
			{Name: "cycles"},
			{Name: "owned_by", Desc: "always empty: a sequence is owned by nothing"},
			{Name: "comment", Desc: "always absent: a sequence carries no comment"},
		},
		Params: []dbmeta.Param{
			{Name: "schema", Desc: "scope name pattern, empty for every scope", Default: ""},
			{Name: "name", Desc: "sequence name pattern, empty for every sequence", Default: ""},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Sequence, error) {
			var v dbmeta.Sequence
			err := rows.Scan(&v.Schema, &v.Name, &v.DataType, &v.Start, &v.Minimum,
				&v.Maximum, &v.Increment, &v.Cycles, &v.OwnedBy, &v.Comment)
			return v, err
		},
	})
}

// The scope and the collection of an index. An index on a bucket's default
// collection has no scope_id, and its keyspace_id is the bucket.
const (
	indexSchema = "IFMISSING(i.scope_id, '_default')"
	indexTable  = "CASE WHEN i.bucket_id IS MISSING THEN '_default' ELSE i.keyspace_id END"
)

// The text of one index key, without ASC or DESC, and whether it is one field
// written in backticks. REGEXP_LIKE matches the whole string.
const (
	keyText  = "REGEXP_REPLACE(k.`key`, ' (ASC|DESC)$', '')"
	plainKey = "REGEXP_LIKE(k.`key`, '`[^`]+`( ASC| DESC)?')"
)
