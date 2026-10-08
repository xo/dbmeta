package elasticsearch

import (
	"context"
	"database/sql"
	"iter"
	"strings"

	"github.com/xo/dbmeta"
)

// noSchema describes the schema field of every row.
const noSchema = "always empty: Elasticsearch has no schema, and SYS TABLES and SYS COLUMNS hold NULL"

// noComment describes the comment of an index. The mapping can hold one in
// _meta, and no SQL statement reports it: REMARKS is empty for a table and
// NULL for a column.
const noComment = "always absent: SQL shows no comment, and the _meta of a mapping is not in it"

// system is the parameter that adds the hidden indices. Elasticsearch marks an
// index hidden with a setting that SQL does not report, and it names every
// hidden index it makes itself with a dot first, such as the backing index
// .ds-dbmeta_stream-2026.10.07-000001 of a data stream. A user can name an
// index with a dot too, and the model cannot tell it from the others.
var system = dbmeta.Param{
	Name:    "with_system",
	Desc:    "include the indices whose names start with a dot, which are hidden indices such as the backing index of a data stream",
	Default: false,
}

var (
	// tableParams are the parameters of Tables.
	tableParams = []dbmeta.Param{
		{Name: "schema", Desc: "schema name pattern. Elasticsearch has no schema, so only the empty pattern matches", Default: ""},
		{Name: "name", Desc: "table name pattern, empty for every table", Default: ""},
		system,
		dbmeta.TypesParam(),
	}
	// viewParams are the parameters of Views.
	viewParams = []dbmeta.Param{
		{Name: "schema", Desc: "schema name pattern. Elasticsearch has no schema, so only the empty pattern matches", Default: ""},
		{Name: "name", Desc: "view name pattern, empty for every view", Default: ""},
		system,
	}
)

// sysTable is one row of SYS TABLES.
type sysTable struct {
	catalog, name, typ string
}

// readTables yields the rows of SYS TABLES whose schema, name and type match
// the arguments. SYS TABLES lists the indices the user can read, and the
// aliases and the data streams as views. It never lists a hidden index.
//
// It is one statement, and the server answers it from the cluster state with
// no read of any document. The walk is one request for any number of indices.
func readTables[T any](ctx context.Context, db dbmeta.Queryer, args map[string]any,
	types string, build func(sysTable) T,
) iter.Seq2[T, error] {
	return readAll(ctx, db, `SYS TABLES`, func(rows *sql.Rows) (T, bool, error) {
		var (
			t       sysTable
			schema  any
			remarks sql.Null[string]
			unused  [5]any
		)
		var zero T
		if err := rows.Scan(&t.catalog, &schema, &t.name, &t.typ, &remarks,
			&unused[0], &unused[1], &unused[2], &unused[3], &unused[4]); err != nil {
			return zero, false, err
		}
		t.typ = strings.ToLower(t.typ)
		keep := visible(args, t.name) &&
			dbmeta.Like(arg(args, "schema"), "") &&
			dbmeta.Like(arg(args, "name"), t.name) &&
			dbmeta.ListHas(types, t.typ)
		return build(t), keep, nil
	})
}

func registerRelations() {
	// \dt. SYS TABLES has one row for each index and each alias, with the type
	// TABLE or VIEW. One statement.
	dbmeta.Tables.Register(dbmeta.Elasticsearch, &dbmeta.Binding[dbmeta.Table]{
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "the name of the cluster, such as docker-cluster"},
			{Name: "schema", Desc: noSchema},
			{Name: "name", Desc: "the index, or the alias, or the data stream"},
			{Name: "type", Desc: "table for an index, and view for an alias or a data stream"},
			{Name: "comment", Desc: noComment},
		},
		Params: tableParams,
		Walk: func(ctx context.Context, db dbmeta.Queryer, args map[string]any) iter.Seq2[dbmeta.Table, error] {
			return readTables(ctx, db, args, arg(args, "types"), func(t sysTable) dbmeta.Table {
				return dbmeta.Table{Catalog: t.catalog, Name: t.name, Type: t.typ}
			})
		},
	})

	// \dv. An alias is a view, and so is a data stream. The statement that
	// makes it, and the filter it holds, are in the alias API and not in SQL,
	// so the view has a name and nothing more. One statement.
	dbmeta.Views.Register(dbmeta.Elasticsearch, &dbmeta.Binding[dbmeta.View]{
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "the name of the cluster, such as docker-cluster"},
			{Name: "schema", Desc: noSchema},
			{Name: "name", Desc: "the alias, or the data stream"},
			{Name: "definition", Desc: "always absent: the indices and the filter of an alias are in the alias API, and no SQL statement reports them"},
			{Name: "check_option", Desc: "always absent: an alias has no check option"},
			{Name: "updatable", Desc: "always absent: SQL cannot write, so the question has no answer"},
			{Name: "insertable", Desc: "always absent: SQL cannot write, so the question has no answer"},
			{Name: "comment", Desc: noComment},
		},
		Params: viewParams,
		Walk: func(ctx context.Context, db dbmeta.Queryer, args map[string]any) iter.Seq2[dbmeta.View, error] {
			return readTables(ctx, db, args, "view", func(t sysTable) dbmeta.View {
				return dbmeta.View{Catalog: t.catalog, Name: t.name}
			})
		},
	})

	// \d NAME. SYS COLUMNS has one row for each field of the mapping of each
	// index, in the order of the table and then the ordinal. It answers 1000
	// rows to a page, and the driver follows the cursor. One statement for
	// the whole cluster, and one more request for each page. The server reads
	// the mappings in the cluster state and never a document, which D177
	// measured on a cluster of 2000 indices.
	dbmeta.Columns.Register(dbmeta.Elasticsearch, &dbmeta.Binding[dbmeta.Column]{
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "the name of the cluster, such as docker-cluster"},
			{Name: "schema", Desc: noSchema},
			{Name: "table", Desc: "the index, or the alias"},
			{Name: "name", Desc: "the field. A subfield is a column of its own, written with a dot, such as the multi-field name.raw and the object field dims.h"},
			{Name: "ordinal", Desc: "the position as SYS COLUMNS reports it, which follows the fields sorted by name. A field SQL does not list still has a position, so a number can be missing: an object, a nested field and a field of a type SQL cannot read, such as dense_vector and flattened"},
			{Name: "data_type", Desc: "the type of the mapping as SQL names it, in upper case, such as LONG, TEXT, KEYWORD and DATETIME. A constant_keyword and a wildcard are KEYWORD"},
			{Name: "nullable", Desc: "always true: SYS COLUMNS reports every field as nullable, because a document can leave any field out"},
			{Name: "default", Desc: "always absent: a mapping has no default"},
			{Name: "primary_key", Desc: "always false: Elasticsearch has no primary key, and the _id of a document is not a field SQL can read"},
			{Name: "identity", Desc: "always empty: SYS COLUMNS reports IS_AUTOINCREMENT as NO for every field"},
			{Name: "generated", Desc: "always empty: SYS COLUMNS reports IS_GENERATEDCOLUMN as NO for every field"},
			{Name: "comment", Desc: "always absent: SYS COLUMNS holds NULL, and a mapping has no comment of a field in SQL"},
			{Name: "collation", Desc: "always absent: Elasticsearch has no collation"},
		},
		Params: []dbmeta.Param{
			{Name: "schema", Desc: "schema name pattern. Elasticsearch has no schema, so only the empty pattern matches", Default: ""},
			{Name: "parent", Desc: "table name pattern, empty for every table", Default: ""},
			{Name: "name", Desc: "column name pattern, empty for every column", Default: ""},
			system,
		},
		Walk: func(ctx context.Context, db dbmeta.Queryer, args map[string]any) iter.Seq2[dbmeta.Column, error] {
			return readAll(ctx, db, `SYS COLUMNS`, func(rows *sql.Rows) (dbmeta.Column, bool, error) {
				return scanColumn(rows, args)
			})
		},
	})

	// \l. SHOW CATALOGS lists the cluster of the connection, and a remote
	// cluster that is set up for cross cluster search. One statement.
	dbmeta.Databases.Register(dbmeta.Elasticsearch, &dbmeta.Binding[dbmeta.Database]{
		Fields: []dbmeta.Field{
			{Name: "name", Desc: "the name of a cluster. A remote cluster of cross cluster search is listed beside the local one"},
			{Name: "owner", Desc: "always empty: a cluster has no owner"},
			{Name: "encoding", Desc: "always empty: Elasticsearch has no encoding to report"},
			{Name: "collate", Desc: "always empty: Elasticsearch has no collation"},
			{Name: "ctype", Desc: "always empty: Elasticsearch has no collation"},
			{Name: "access", Desc: "always absent: SQL reports no grant on a cluster"},
			{Name: "tablespace", Desc: "always absent: Elasticsearch has no tablespace"},
			{Name: "size", Desc: "always absent: SHOW CATALOGS reports no size"},
			{Name: "comment", Desc: "always absent: SHOW CATALOGS reports local or remote in its second column, which no field here holds"},
		},
		Params: []dbmeta.Param{
			{Name: "name", Desc: "cluster name pattern, empty for every cluster", Default: ""},
		},
		Walk: func(ctx context.Context, db dbmeta.Queryer, args map[string]any) iter.Seq2[dbmeta.Database, error] {
			return readAll(ctx, db, `SHOW CATALOGS`, func(rows *sql.Rows) (dbmeta.Database, bool, error) {
				var name, kind string
				if err := rows.Scan(&name, &kind); err != nil {
					return dbmeta.Database{}, false, err
				}
				return dbmeta.Database{Name: name}, dbmeta.Like(arg(args, "name"), name), nil
			})
		},
	})
}

// sysColumns is how many columns SYS COLUMNS returns, which are the 24 of
// JDBC's getColumns.
const sysColumns = 24

// scanColumn reads one row of SYS COLUMNS, and says whether the arguments
// keep it. The positions are those of JDBC's getColumns: 1 TABLE_CAT,
// 2 TABLE_SCHEM, 3 TABLE_NAME, 4 COLUMN_NAME, 6 TYPE_NAME, 11 NULLABLE,
// 12 REMARKS, 13 COLUMN_DEF and 17 ORDINAL_POSITION. The rest repeat the type
// or are NULL.
func scanColumn(rows *sql.Rows, args map[string]any) (dbmeta.Column, bool, error) {
	var (
		c        dbmeta.Column
		nullable int64
		remarks  sql.Null[string]
		def      sql.Null[string]
	)
	dest := make([]any, sysColumns)
	for i := range dest {
		dest[i] = new(any)
	}
	dest[0], dest[2], dest[3], dest[5] = &c.Catalog, &c.Table, &c.Name, &c.DataType
	dest[10], dest[11], dest[12], dest[16] = &nullable, &remarks, &def, &c.Ordinal
	if err := rows.Scan(dest...); err != nil {
		return dbmeta.Column{}, false, err
	}
	c.Nullable = nullable != 0
	c.Default = def
	c.Comment = remarks
	c.Identity = sql.Null[string]{Valid: true}
	c.Generated = sql.Null[string]{Valid: true}
	keep := visible(args, c.Table) &&
		dbmeta.Like(arg(args, "schema"), "") &&
		dbmeta.Like(arg(args, "parent"), c.Table) &&
		dbmeta.Like(arg(args, "name"), c.Name)
	return c, keep, nil
}
