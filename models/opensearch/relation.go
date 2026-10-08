package opensearch

import (
	"context"
	"iter"

	"github.com/xo/dbmeta"
)

// noSchema describes the schema field of every row.
const noSchema = "always empty: OpenSearch has no schema, and SHOW TABLES and DESCRIBE hold NULL"

// noComment describes the comment of an index. The mapping can hold one in
// _meta, and no SQL statement reports it: REMARKS is NULL for a table and for a
// column.
const noComment = "always absent: SQL shows no comment, and the _meta of a mapping is not in it"

// system is the parameter that adds the hidden indices. OpenSearch names the
// ones it builds in with a dot first, such as .opendistro_security. A user can
// name an index with a dot too, and the model cannot tell it from the others.
var system = dbmeta.Param{
	Name:    "with_system",
	Desc:    "include the indices whose names start with a dot, which are the ones OpenSearch builds in, such as .opendistro_security",
	Default: false,
}

func registerRelations() {
	// \dt. SHOW TABLES LIKE % has one row for each index, and on 2.19.6 for
	// each alias, which it also calls BASE TABLE. One statement.
	dbmeta.Tables.Register(dbmeta.OpenSearch, &dbmeta.Binding[dbmeta.Table]{
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "the name of the cluster, such as docker-cluster"},
			{Name: "schema", Desc: noSchema},
			{Name: "name", Desc: "the index. On 2.19.6 it can be an alias, which SQL lists as BASE TABLE and no row tells from an index. 3.9.0 lists no alias"},
			{Name: "type", Desc: "always table: SHOW TABLES says BASE TABLE for every name"},
			{Name: "comment", Desc: noComment},
		},
		Params: []dbmeta.Param{
			{Name: "schema", Desc: "schema name pattern. OpenSearch has no schema, so only the empty pattern matches", Default: ""},
			{Name: "name", Desc: "table name pattern, empty for every table", Default: ""},
			system,
			dbmeta.TypesParam(),
		},
		Walk: func(ctx context.Context, db dbmeta.Queryer, args map[string]any) iter.Seq2[dbmeta.Table, error] {
			return func(yield func(dbmeta.Table, error) bool) {
				if !dbmeta.Like(arg(args, "schema"), "") {
					return
				}
				for t, err := range showTables(ctx, db, func(t showTable) bool {
					return visible(args, t.name) && dbmeta.Like(arg(args, "name"), t.name) &&
						dbmeta.ListHas(arg(args, "types"), "table")
				}) {
					if err != nil {
						yield(dbmeta.Table{}, err)
						return
					}
					if !yield(dbmeta.Table{Catalog: t.catalog, Name: t.name, Type: "table"}, nil) {
						return
					}
				}
			}
		},
	})

	// \d NAME. DESCRIBE TABLES LIKE has one row for each field of the mapping
	// of the indices it matches. See columns for the walk and its cost.
	dbmeta.Columns.Register(dbmeta.OpenSearch, &dbmeta.Binding[dbmeta.Column]{
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "the name of the cluster, such as docker-cluster"},
			{Name: "schema", Desc: noSchema},
			{Name: "table", Desc: "the index"},
			{Name: "name", Desc: "the field. An object and a nested field are columns, with the types object and nested, and a subfield is a column of its own, written with a dot, such as dims.h. A multi-field, such as name.raw, and a field of a type SQL cannot read, such as integer_range, are not listed"},
			{Name: "ordinal", Desc: "the position as DESCRIBE reports it, which is not the order of the mapping, and which starts at 0 on both releases"},
			{Name: "data_type", Desc: "the type of the mapping as DESCRIBE names it, in lower case, such as long, text and keyword. A date is timestamp"},
			{Name: "nullable", Desc: "always true: DESCRIBE reports NULLABLE 2, which is unknown, because a document can leave any field out"},
			{Name: "default", Desc: "always absent: a mapping has no default"},
			{Name: "primary_key", Desc: "always false: OpenSearch has no primary key, and the _id of a document is not a field SQL can read"},
			{Name: "identity", Desc: "always empty: DESCRIBE reports IS_AUTOINCREMENT as NO for every field"},
			{Name: "generated", Desc: "always empty: DESCRIBE reports IS_GENERATEDCOLUMN as empty for every field"},
			{Name: "comment", Desc: "always absent: DESCRIBE holds NULL in REMARKS, and a mapping has no comment of a field in SQL"},
			{Name: "collation", Desc: "always absent: OpenSearch has no collation"},
		},
		Params: []dbmeta.Param{
			{Name: "schema", Desc: "schema name pattern. OpenSearch has no schema, so only the empty pattern matches", Default: ""},
			{Name: "parent", Desc: "table name pattern, empty for every table", Default: ""},
			{Name: "name", Desc: "column name pattern, empty for every column", Default: ""},
			system,
		},
		Walk: columns,
	})

	// \l. The cluster is the one database. SHOW TABLES LIKE % names it in each
	// row, and the walk yields it once. One statement.
	dbmeta.Databases.Register(dbmeta.OpenSearch, &dbmeta.Binding[dbmeta.Database]{
		Fields: []dbmeta.Field{
			{Name: "name", Desc: "the name of the cluster, which is TABLE_CAT of SHOW TABLES. No statement names a cluster that has no index the user can see"},
			{Name: "owner", Desc: "always empty: a cluster has no owner"},
			{Name: "encoding", Desc: "always empty: OpenSearch has no encoding to report"},
			{Name: "collate", Desc: "always empty: OpenSearch has no collation"},
			{Name: "ctype", Desc: "always empty: OpenSearch has no collation"},
			{Name: "access", Desc: "always absent: SQL reports no grant on a cluster"},
			{Name: "tablespace", Desc: "always absent: OpenSearch has no tablespace"},
			{Name: "size", Desc: "always absent: SHOW TABLES reports no size"},
			{Name: "comment", Desc: "always absent: SHOW TABLES reports no description of a cluster"},
		},
		Params: []dbmeta.Param{
			{Name: "name", Desc: "cluster name pattern, empty for every cluster", Default: ""},
		},
		Walk: func(ctx context.Context, db dbmeta.Queryer, args map[string]any) iter.Seq2[dbmeta.Database, error] {
			return func(yield func(dbmeta.Database, error) bool) {
				seen := ""
				for t, err := range showTables(ctx, db, func(t showTable) bool { return t.catalog != seen }) {
					if err != nil {
						yield(dbmeta.Database{}, err)
						return
					}
					seen = t.catalog
					if !dbmeta.Like(arg(args, "name"), t.catalog) {
						continue
					}
					if !yield(dbmeta.Database{Name: t.catalog}, nil) {
						return
					}
				}
			}
		},
	})
}
