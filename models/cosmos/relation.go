package cosmos

import (
	"database/sql"
	"strings"

	"github.com/xo/dbmeta"
)

// Databases, schemas, tables, indexes and partitioned tables.

// containerFields declares the columns of the statement that reads the
// containers of a database. Tables, indexes and partitioned tables read it, and
// each one takes the columns that its object has a field for.
func containerFields() []dbmeta.Field {
	return []dbmeta.Field{
		field("database", "the database that the URL names. It is the schema"),
		field("id", "the name of the container"),
		field("rid", "the resource id of the container. A permission names its resource by it"),
		field("link", "the self link of the container, which is rid based, such as dbs/lg9iAA==/colls/lg9iANYJ1UQ=/"),
		field("partition_key_paths", "the paths of the partition key"),
		field("partition_key_kind", "the kind of the partition key, Hash or MultiHash"),
		field("partition_key_version", "the version of the partition key. The service leaves it out for a container that was made at version 1"),
		field("indexing_mode", "the indexing mode, consistent or none"),
		field("indexing_automatic", "whether the container indexes a new document by itself"),
		field("indexing_included_paths", "the paths that the indexing policy includes"),
		field("indexing_excluded_paths", "the paths that the indexing policy excludes. The service adds the path of _etag to every container"),
		field("composite_indexes", "the composite indexes, each a list of paths and orders"),
		field("spatial_indexes", "the spatial indexes, each a path and its geometry types"),
		field("vector_indexes", "the vector indexes, each a path and a type. Absent on an account with no vector search"),
		field("indexing_policy", "the whole indexing policy as JSON text with sorted keys"),
		field("unique_keys", "the unique keys, each a list of paths"),
		field("default_ttl", "the time to live of a document in seconds, and -1 for none by default. Absent when time to live is off"),
		field("analytical_ttl", "the time to live of the analytical store in seconds. Absent on an account with no analytical store"),
		field("conflict_resolution_mode", "the conflict resolution mode, LastWriterWins or Custom"),
		field("conflict_resolution_path", "the path that last writer wins compares"),
		field("conflict_resolution_procedure", "the stored procedure that a custom policy runs"),
		field("change_feed_retention", "the minutes that the change feed keeps a version, from the change feed policy. Absent unless the account keeps all versions"),
		field("computed_properties", "the computed properties, each a name and a query"),
		field("vector_embedding_policy", "the vector embedding policy. Absent on an account with no vector search"),
		field("full_text_policy", "the full text policy"),
		field("geospatial_type", "Geography or Geometry"),
	}
}

// containerOptions collects the settings of a container that the Table fields
// have no place for.
func containerOptions(r row) sql.Null[string] {
	var o options
	o.add("rid", r.text("rid"))
	o.add("link", r.text("link"))
	o.add("partition_key", strings.Join(r.list("partition_key_paths"), "|"))
	o.add("partition_key_kind", r.text("partition_key_kind"))
	o.addNumber("partition_key_version", r.number("partition_key_version"))
	o.addNumber("default_ttl", r.number("default_ttl"))
	o.addNumber("analytical_ttl", r.number("analytical_ttl"))
	o.add("geospatial_type", r.text("geospatial_type"))
	o.add("conflict_resolution", r.text("conflict_resolution_mode"))
	o.add("conflict_resolution_path", r.text("conflict_resolution_path"))
	o.add("conflict_resolution_procedure", r.text("conflict_resolution_procedure"))
	o.addNumber("change_feed_retention", r.number("change_feed_retention"))
	o.add("unique_keys", uniqueKeys(r))
	o.add("computed_properties", computedNames(r))
	o.add("full_text", hasPolicy(r, "full_text_policy"))
	o.add("vector_embedding", hasPolicy(r, "vector_embedding_policy"))
	return o.value()
}

// uniqueKeys writes each unique key as its paths joined by a plus sign, and the
// keys joined by a vertical bar.
func uniqueKeys(r row) string {
	keys, _ := r["unique_keys"].([]any)
	out := make([]string, 0, len(keys))
	for _, key := range keys {
		var paths []string
		items, _ := key.([]any)
		for _, item := range items {
			if s, ok := item.(string); ok {
				paths = append(paths, s)
			}
		}
		out = append(out, strings.Join(paths, "+"))
	}
	return strings.Join(out, "|")
}

// computedNames lists the names of the computed properties.
func computedNames(r row) string {
	props, _ := r["computed_properties"].([]any)
	var names []string
	for _, p := range props {
		if m, ok := p.(map[string]any); ok {
			if s, ok := m["name"].(string); ok {
				names = append(names, s)
			}
		}
	}
	return strings.Join(names, "|")
}

// hasPolicy returns true when the policy in column name is set, and the empty
// string when it is not, so that options leaves it out.
func hasPolicy(r row, name string) string {
	if r[name] != nil {
		return "true"
	}
	return ""
}

// partitionKey writes the paths of the partition key, joined by a comma and a
// space.
func partitionKey(r row) string { return strings.Join(r.list("partition_key_paths"), ", ") }

// indexName is the name of the one index of a container, which is its indexing
// policy. A policy has no name of its own.
const indexName = "indexing_policy"

// containerType is the type of a container, as Table.Type spells it.
const containerType = "container"

func registerRelations() {
	// \l. A Cosmos DB database holds containers, users and permissions. The
	// feed of databases has no owner, no encoding, no collation and no size.
	dbmeta.Databases.Register(dbmeta.Cosmos, &dbmeta.Binding[dbmeta.Database]{
		Stmt: catalog("$databases"),
		Fields: []dbmeta.Field{
			field("id", "the name of the database"),
			field("rid", "the resource id of the database, which Schemas reports in its options"),
			field("link", "the self link of the database, such as dbs/lg9iAA==/, which a permission names"),
		},
		Params: accountFilters("database"),
		Keep:   keep(nil, nil, func(v dbmeta.Database) string { return v.Name }),
		Scan: func(rows *sql.Rows) (dbmeta.Database, error) {
			r, err := scanRow(rows)
			return dbmeta.Database{Name: r.text("id")}, err
		},
	})

	// \dn. The database that holds containers is the schema, as an ArangoDB
	// database is (D168). The statement is the one of Databases, and it lists
	// every database of the account.
	dbmeta.Schemas.Register(dbmeta.Cosmos, &dbmeta.Binding[dbmeta.Schema]{
		Stmt: catalog("$databases"),
		Fields: []dbmeta.Field{
			field("id", "the name of the database"),
			field("rid", "the resource id of the database, which Options holds"),
			field("link", "the self link of the database, which Options holds"),
		},
		Params: accountFilters("schema"),
		Keep:   keep(nil, nil, func(v dbmeta.Schema) string { return v.Name }),
		Scan: func(rows *sql.Rows) (dbmeta.Schema, error) {
			r, err := scanRow(rows)
			var o options
			o.add("rid", r.text("rid"))
			o.add("link", r.text("link"))
			return dbmeta.Schema{Name: r.text("id"), Options: o.value()}, err
		},
	})

	// \dt. A container is a table of the type container. The container has no
	// size, no row count and no owner that the feed reports. The options hold the
	// settings that a table has no field for.
	dbmeta.Tables.Register(dbmeta.Cosmos, &dbmeta.Binding[dbmeta.Table]{
		Stmt:   catalog("$containers"),
		Fields: containerFields(),
		Params: append(databaseFilters("container"), dbmeta.TypesParam()),
		Keep: func(v dbmeta.Table, args map[string]any) bool {
			return keep(func(v dbmeta.Table) string { return v.Schema }, nil, func(v dbmeta.Table) string { return v.Name })(v, args) &&
				dbmeta.ListHas(arg(args, "types"), v.Type)
		},
		Scan: func(rows *sql.Rows) (dbmeta.Table, error) {
			r, err := scanRow(rows)
			return dbmeta.Table{
				Schema:  r.text("database"),
				Name:    r.text("id"),
				Type:    containerType,
				Options: containerOptions(r),
			}, err
		},
	})

	// \di. The indexing policy is the one index of a container, because a
	// container has no named index. The statement returns one row for a container.
	dbmeta.Indexes.Register(dbmeta.Cosmos, &dbmeta.Binding[dbmeta.Index]{
		Stmt:   catalog("$containers"),
		Fields: containerFields(),
		Params: []dbmeta.Param{schemaParam(), parentParam("container"), nameParam("index")},
		Keep:   keep(func(v dbmeta.Index) string { return v.Schema }, func(v dbmeta.Index) string { return v.Table }, func(v dbmeta.Index) string { return v.Name }),
		Scan: func(rows *sql.Rows) (dbmeta.Index, error) {
			r, err := scanRow(rows)
			var o options
			if v, ok := r["indexing_automatic"].(bool); ok {
				if v {
					o.add("automatic", "true")
				} else {
					o.add("automatic", "false")
				}
			}
			o.add("included_paths", strings.Join(r.list("indexing_included_paths"), "|"))
			o.add("excluded_paths", strings.Join(r.list("indexing_excluded_paths"), "|"))
			return dbmeta.Index{
				Schema:     r.text("database"),
				Table:      r.text("id"),
				Name:       indexName,
				Type:       r.text("indexing_mode"),
				Options:    o.value(),
				Definition: r.nullText("indexing_policy"),
			}, err
		},
	})

	// \dP. Every container is split by the hash of its partition key, so every
	// container is a partitioned table, and the expression is the paths of the key.
	// The physical partitions are not in the feed.
	dbmeta.PartitionedTables.Register(dbmeta.Cosmos, &dbmeta.Binding[dbmeta.PartitionedTable]{
		Stmt:   catalog("$containers"),
		Fields: containerFields(),
		Params: databaseFilters("partitioned table"),
		Keep:   keep(func(v dbmeta.PartitionedTable) string { return v.Schema }, nil, func(v dbmeta.PartitionedTable) string { return v.Name }),
		Scan: func(rows *sql.Rows) (dbmeta.PartitionedTable, error) {
			r, err := scanRow(rows)
			return dbmeta.PartitionedTable{
				Schema:     r.text("database"),
				Name:       r.text("id"),
				Type:       containerType,
				Strategy:   "hash",
				Expression: partitionKey(r),
			}, err
		},
	})
}
